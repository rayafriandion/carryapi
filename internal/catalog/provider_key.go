package catalog

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// 上游 API key 池状态。
const (
	KeyStatusActive      = "active"
	KeyStatusCoolingDown = "cooling_down"
	KeyStatusDeleted     = "deleted"
)

// provider_api_key_events 事件类型(即"API key 调用日志"中的事件)。
const (
	KeyEventCreated       = "created"
	KeyEventUpdated       = "updated"
	KeyEventDegraded      = "degraded"
	KeyEventRetryStarted  = "retry_started"
	KeyEventRetrySuccess  = "retry_success"
	KeyEventRetryFailed   = "retry_failed"
	KeyEventRecovered     = "recovered"
	KeyEventDeleted       = "deleted"
	KeyEventDeletedManual = "deleted_manual"
)

// 上游 key 冷却时长与后台自测次数(重试 3 次仍失败则删除)。
const (
	KeyCooldownDuration   = 1 * time.Hour
	KeyRetryProbeAttempts = 3
)

// ProviderAPIKey 是供应商下的一个上游 API key(APIKey 为解密后的明文)。
type ProviderAPIKey struct {
	ID           int64
	ProviderID   int64
	APIKey       string
	Label        string
	Priority     int
	BasePriority int
	Status       string
	FailCount    int
	RetryAfter   *time.Time
	LastUsedAt   *time.Time
	CreatedAt    time.Time
	DeletedAt    *time.Time
}

// ProviderKeyEvent 是上游 key 生命周期日志行("API key 调用日志")。
type ProviderKeyEvent struct {
	ID         int64     `json:"id"`
	KeyID      int64     `json:"key_id"`
	ProviderID int64     `json:"provider_id"`
	Event      string    `json:"event"`
	Detail     string    `json:"detail"`
	CreatedAt  time.Time `json:"created_at"`
}

var (
	errNoActiveProviderKey = errors.New("no active provider api key")
	errProviderKeyNotFound = errors.New("provider api key not found")
)

// MaskKey 生成用于展示的掩码(不暴露完整 key)。
func MaskKey(plain string) string {
	if len(plain) <= 8 {
		return "***"
	}
	head := plain[:4]
	tail := plain[len(plain)-4:]
	return head + "..." + tail
}

// AddKey 为供应商新增一个上游 API key,追加到优先级末尾,记录 created 事件。
func (s *ProviderStore) AddKey(providerID int64, apiKey, label string) (ProviderAPIKey, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return ProviderAPIKey{}, fmt.Errorf("api key is required")
	}
	enc, err := s.cipher.Encrypt([]byte(apiKey))
	if err != nil {
		return ProviderAPIKey{}, fmt.Errorf("encrypt provider api key: %w", err)
	}
	// 新 key 放池尾:priority 取当前最大 priority+1(>=100),base_priority 记为基准 100。
	var nextPriority int
	_ = s.db.QueryRow(`SELECT COALESCE(MAX(priority), 99) + 1 FROM provider_api_keys WHERE provider_id = ?`, providerID).Scan(&nextPriority)
	res, err := s.db.Exec(
		`INSERT INTO provider_api_keys(provider_id, key_enc, label, priority, base_priority, status)
		 VALUES(?, ?, ?, ?, 100, 'active')`,
		providerID, enc, label, nextPriority)
	if err != nil {
		return ProviderAPIKey{}, fmt.Errorf("add provider api key: %w", err)
	}
	id, _ := res.LastInsertId()
	if err := s.logKeyEvent(id, providerID, KeyEventCreated, label); err != nil {
		return ProviderAPIKey{}, err
	}
	if err := s.syncProviderPrimaryKey(providerID); err != nil {
		return ProviderAPIKey{}, err
	}
	return s.GetKey(id)
}

// Keys 返回供应商的全部 key(含冷却/已删除),用于管理端展示。
func (s *ProviderStore) Keys(providerID int64) ([]ProviderAPIKey, error) {
	rows, err := s.db.Query(
		`SELECT id, provider_id, key_enc, label, priority, base_priority, status, fail_count, retry_after, last_used_at, created_at, deleted_at
		 FROM provider_api_keys WHERE provider_id = ? ORDER BY (status = 'deleted') ASC, priority ASC, id ASC`, providerID)
	if err != nil {
		return nil, fmt.Errorf("list provider api keys: %w", err)
	}
	defer rows.Close()
	return scanProviderKeys(s, rows)
}

// ActiveKeys 返回供应商的可用 key,按优先级升序。
func (s *ProviderStore) ActiveKeys(providerID int64) ([]ProviderAPIKey, error) {
	rows, err := s.db.Query(
		`SELECT id, provider_id, key_enc, label, priority, base_priority, status, fail_count, retry_after, last_used_at, created_at, deleted_at
		 FROM provider_api_keys WHERE provider_id = ? AND status = 'active' ORDER BY priority ASC, id ASC`, providerID)
	if err != nil {
		return nil, fmt.Errorf("list active provider api keys: %w", err)
	}
	defer rows.Close()
	return scanProviderKeys(s, rows)
}

// SelectKey 为某用户的请求选择上游 key:
// 优先该用户之前用过的 key(缓存命中),其余按优先级/ID 排序。
func (s *ProviderStore) SelectKey(providerID, userID int64) (ProviderAPIKey, error) {
	row := s.db.QueryRow(
		`SELECT k.id, k.provider_id, k.key_enc, k.label, k.priority, k.base_priority, k.status, k.fail_count, k.retry_after, k.last_used_at, k.created_at, k.deleted_at
		 FROM provider_api_keys k
		 LEFT JOIN provider_key_prefs pkp ON pkp.key_id = k.id AND pkp.user_id = ?
		 WHERE k.provider_id = ? AND k.status = 'active'
		 ORDER BY (pkp.last_used_at IS NULL) ASC, pkp.last_used_at DESC, k.priority ASC, k.id ASC
		 LIMIT 1`, userID, providerID)
	k, err := s.scanProviderKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ProviderAPIKey{}, errNoActiveProviderKey
	}
	if err != nil {
		return ProviderAPIKey{}, err
	}
	return k, nil
}

// GetKey 返回单个 key。
func (s *ProviderStore) GetKey(keyID int64) (ProviderAPIKey, error) {
	row := s.db.QueryRow(
		`SELECT id, provider_id, key_enc, label, priority, base_priority, status, fail_count, retry_after, last_used_at, created_at, deleted_at
		 FROM provider_api_keys WHERE id = ?`, keyID)
	k, err := s.scanProviderKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ProviderAPIKey{}, errProviderKeyNotFound
	}
	if err != nil {
		return ProviderAPIKey{}, err
	}
	return k, nil
}

// DegradeKey 将 key 降级:放到优先级末尾、进入 1 小时冷却、失败计数+1,并记录日志。
func (s *ProviderStore) DegradeKey(keyID int64, reason string) error {
	var providerID int64
	var priority int
	err := s.db.QueryRow(`SELECT provider_id FROM provider_api_keys WHERE id = ?`, keyID).Scan(&providerID)
	if err != nil {
		return err
	}
	_ = s.db.QueryRow(`SELECT COALESCE(MAX(priority), 99) + 1 FROM provider_api_keys WHERE provider_id = ? AND status != 'deleted'`, providerID).Scan(&priority)
	retryAfter := time.Now().Add(KeyCooldownDuration)
	if _, err := s.db.Exec(
		`UPDATE provider_api_keys SET status = 'cooling_down', priority = ?, fail_count = fail_count + 1, retry_after = ? WHERE id = ?`,
		priority, retryAfter, keyID); err != nil {
		return fmt.Errorf("degrade provider api key: %w", err)
	}
	// 若降级的是 legacy 主 key,同步为下一个 active key(保证探测/展示一致)。
	if err := s.syncProviderPrimaryKey(providerID); err != nil {
		return err
	}
	detail := fmt.Sprintf("moved to end of priority, cooldown until %s; reason: %s", retryAfter.Format(time.RFC3339), reason)
	return s.logKeyEvent(keyID, providerID, KeyEventDegraded, detail)
}

// RecoverKey 将 key 恢复为 active,优先级还原为基准值,并记录日志。
func (s *ProviderStore) RecoverKey(keyID int64, detail string) error {
	var providerID int64
	var basePriority int
	err := s.db.QueryRow(`SELECT provider_id, base_priority FROM provider_api_keys WHERE id = ?`, keyID).Scan(&providerID, &basePriority)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(
		`UPDATE provider_api_keys SET status = 'active', fail_count = 0, priority = ?, retry_after = NULL WHERE id = ?`,
		basePriority, keyID); err != nil {
		return fmt.Errorf("recover provider api key: %w", err)
	}
	if err := s.syncProviderPrimaryKey(providerID); err != nil {
		return err
	}
	return s.logKeyEvent(keyID, providerID, KeyEventRecovered, detail)
}

// DeleteKey 软删除 key(保留审计日志):manual=true 为管理员手动删除。
func (s *ProviderStore) DeleteKey(keyID int64, manual bool, reason string) error {
	var providerID int64
	err := s.db.QueryRow(`SELECT provider_id FROM provider_api_keys WHERE id = ?`, keyID).Scan(&providerID)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(
		`UPDATE provider_api_keys SET status = 'deleted', deleted_at = ? WHERE id = ?`,
		time.Now(), keyID); err != nil {
		return fmt.Errorf("delete provider api key: %w", err)
	}
	if err := s.syncProviderPrimaryKey(providerID); err != nil {
		return err
	}
	event := KeyEventDeleted
	if manual {
		event = KeyEventDeletedManual
	}
	return s.logKeyEvent(keyID, providerID, event, reason)
}

// UpdateKeyMeta 更新 key 的标签与基准优先级(active 时同步生效)。
func (s *ProviderStore) UpdateKeyMeta(keyID int64, label string, basePriority *int) error {
	if basePriority != nil {
		if *basePriority < 0 {
			return fmt.Errorf("priority must be non-negative")
		}
		if _, err := s.db.Exec(
			`UPDATE provider_api_keys SET label = ?, base_priority = ?, priority = CASE WHEN status = 'active' THEN ? ELSE priority END WHERE id = ?`,
			label, *basePriority, *basePriority, keyID); err != nil {
			return fmt.Errorf("update provider api key meta: %w", err)
		}
		return nil
	}
	if _, err := s.db.Exec(`UPDATE provider_api_keys SET label = ? WHERE id = ?`, label, keyID); err != nil {
		return fmt.Errorf("update provider api key label: %w", err)
	}
	return nil
}

// MarkUsed 记录 key 被使用(更新时间戳)并建立用户-上游 key 亲和(缓存命中)。
func (s *ProviderStore) MarkUsed(keyID, userID int64) error {
	if keyID <= 0 {
		return nil
	}
	now := time.Now()
	if _, err := s.db.Exec(`UPDATE provider_api_keys SET last_used_at = ? WHERE id = ?`, now, keyID); err != nil {
		return err
	}
	_, err := s.db.Exec(
		`INSERT INTO provider_key_prefs(user_id, key_id, last_used_at) VALUES(?, ?, ?)
		 ON CONFLICT(user_id, key_id) DO UPDATE SET last_used_at = excluded.last_used_at`,
		userID, keyID, now)
	return err
}

// RetryDueKeys 返回已到冷却截止、等待后台自测重试的 key。
func (s *ProviderStore) RetryDueKeys() ([]ProviderAPIKey, error) {
	rows, err := s.db.Query(
		`SELECT id, provider_id, key_enc, label, priority, base_priority, status, fail_count, retry_after, last_used_at, created_at, deleted_at
		 FROM provider_api_keys WHERE status = 'cooling_down' AND retry_after IS NOT NULL AND retry_after <= ?`,
		time.Now())
	if err != nil {
		return nil, fmt.Errorf("list due provider api keys: %w", err)
	}
	defer rows.Close()
	return scanProviderKeys(s, rows)
}

// KeyEvents 返回某 key 的生命周期日志("API key 调用日志")。
func (s *ProviderStore) KeyEvents(keyID int64) ([]ProviderKeyEvent, error) {
	rows, err := s.db.Query(
		`SELECT id, key_id, provider_id, event, detail, created_at
		 FROM provider_api_key_events WHERE key_id = ? ORDER BY id DESC, created_at DESC`, keyID)
	if err != nil {
		return nil, fmt.Errorf("list provider api key events: %w", err)
	}
	defer rows.Close()
	var out []ProviderKeyEvent
	for rows.Next() {
		var e ProviderKeyEvent
		if err := rows.Scan(&e.ID, &e.KeyID, &e.ProviderID, &e.Event, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// syncProviderPrimaryKey 让 legacy upstream_providers.api_key 始终等于池中第一个 active key,
// 保证探测(probe)/Provider.APIKey 等旧逻辑语义一致。
func (s *ProviderStore) syncProviderPrimaryKey(providerID int64) error {
	var enc []byte
	err := s.db.QueryRow(
		`SELECT key_enc FROM provider_api_keys WHERE provider_id = ? AND status = 'active' ORDER BY priority ASC, id ASC LIMIT 1`,
		providerID).Scan(&enc)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE upstream_providers SET api_key = ? WHERE id = ?`, enc, providerID)
	return err
}

func (s *ProviderStore) logKeyEvent(keyID, providerID int64, event, detail string) error {
	_, err := s.db.Exec(
		`INSERT INTO provider_api_key_events(key_id, provider_id, event, detail) VALUES(?, ?, ?, ?)`,
		keyID, providerID, event, detail)
	return err
}

type providerKeyScanner interface {
	Scan(...any) error
}

func (s *ProviderStore) scanProviderKey(r providerKeyScanner) (ProviderAPIKey, error) {
	var k ProviderAPIKey
	var enc []byte
	var retryAfter, lastUsedAt, deletedAt sql.NullTime
	if err := r.Scan(&k.ID, &k.ProviderID, &enc, &k.Label, &k.Priority, &k.BasePriority, &k.Status,
		&k.FailCount, &retryAfter, &lastUsedAt, &k.CreatedAt, &deletedAt); err != nil {
		return ProviderAPIKey{}, err
	}
	dec, err := s.cipher.Decrypt(enc)
	if err != nil {
		return ProviderAPIKey{}, fmt.Errorf("decrypt provider api key: %w", err)
	}
	k.APIKey = string(dec)
	if retryAfter.Valid {
		t := retryAfter.Time
		k.RetryAfter = &t
	}
	if lastUsedAt.Valid {
		t := lastUsedAt.Time
		k.LastUsedAt = &t
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		k.DeletedAt = &t
	}
	return k, nil
}

func scanProviderKeys(s *ProviderStore, rows *sql.Rows) ([]ProviderAPIKey, error) {
	var out []ProviderAPIKey
	for rows.Next() {
		k, err := s.scanProviderKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}
