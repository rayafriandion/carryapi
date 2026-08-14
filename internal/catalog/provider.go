package catalog

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"carryapi/internal/crypto"
)

type Provider struct {
	ID        int64
	Name      string
	BaseURL   string
	APIKey    string // 解密后
	Protocol  string
	Status    string
	CreatedAt time.Time
}

var validProtocols = map[string]bool{"openai_chat": true, "openai_responses": true, "anthropic": true}

type ProviderStore struct {
	db     *sql.DB
	cipher *crypto.Cipher
}

func NewProviderStore(db *sql.DB, cipher *crypto.Cipher) *ProviderStore {
	return &ProviderStore{db: db, cipher: cipher}
}

func (s *ProviderStore) Create(name, baseURL, apiKey, protocol string) (Provider, error) {
	if !validProtocols[protocol] {
		return Provider{}, fmt.Errorf("invalid protocol %q", protocol)
	}
	enc, err := s.cipher.Encrypt([]byte(apiKey))
	if err != nil {
		return Provider{}, fmt.Errorf("encrypt api key: %w", err)
	}
	res, err := s.db.Exec(
		`INSERT INTO upstream_providers(name, base_url, api_key, protocol, status) VALUES(?, ?, ?, ?, 'active')`,
		name, baseURL, enc, protocol)
	if err != nil {
		return Provider{}, fmt.Errorf("create provider: %w", err)
	}
	id, _ := res.LastInsertId()
	// 同时把该 key 落入多 key 池(便于后续追加更多 key)。
	if apiKey != "" {
		if _, err := s.AddKey(id, apiKey, ""); err != nil {
			return Provider{}, fmt.Errorf("seed provider api key pool: %w", err)
		}
	}
	return s.Get(id)
}

func (s *ProviderStore) Get(id int64) (Provider, error) {
	row := s.db.QueryRow(
		`SELECT id, name, base_url, api_key, protocol, status, created_at FROM upstream_providers WHERE id=?`, id)
	return s.scan(row)
}

func (s *ProviderStore) List() ([]Provider, error) {
	rows, err := s.db.Query(
		`SELECT id, name, base_url, api_key, protocol, status, created_at FROM upstream_providers ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()
	var out []Provider
	for rows.Next() {
		p, err := s.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *ProviderStore) Update(id int64, name, baseURL, apiKey, protocol, status string) error {
	if !validProtocols[protocol] {
		return fmt.Errorf("invalid protocol %q", protocol)
	}
	if status != "active" && status != "disabled" {
		return fmt.Errorf("invalid status %q", status)
	}
	var err error
	if apiKey != "" {
		var enc []byte
		enc, err = s.cipher.Encrypt([]byte(apiKey))
		if err != nil {
			return fmt.Errorf("encrypt api key: %w", err)
		}
		_, err = s.db.Exec(
			`UPDATE upstream_providers SET name=?, base_url=?, api_key=?, protocol=?, status=? WHERE id=?`,
			name, baseURL, enc, protocol, status, id)
		// 同步池中主 key(第一个 active key)为新值;若池为空则新增。
		if err == nil {
			if err = s.replacePrimaryKey(id, apiKey); err != nil {
				return err
			}
		}
	} else {
		_, err = s.db.Exec(
			`UPDATE upstream_providers SET name=?, base_url=?, protocol=?, status=? WHERE id=?`,
			name, baseURL, protocol, status, id)
	}
	return err
}

// replacePrimaryKey 把池中第一个 active key 更新为新值;若无 active key 则新增一条。
func (s *ProviderStore) replacePrimaryKey(providerID int64, apiKey string) error {
	var keyID int64
	err := s.db.QueryRow(
		`SELECT id FROM provider_api_keys WHERE provider_id = ? AND status = 'active' ORDER BY priority ASC, id ASC LIMIT 1`,
		providerID).Scan(&keyID)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = s.AddKey(providerID, apiKey, "")
		return err
	}
	if err != nil {
		return err
	}
	enc, err := s.cipher.Encrypt([]byte(apiKey))
	if err != nil {
		return fmt.Errorf("encrypt provider api key: %w", err)
	}
	if _, err := s.db.Exec(`UPDATE provider_api_keys SET key_enc = ? WHERE id = ?`, enc, keyID); err != nil {
		return fmt.Errorf("update primary provider api key: %w", err)
	}
	return s.logKeyEvent(keyID, providerID, KeyEventUpdated, "primary key replaced via provider edit")
}

func (s *ProviderStore) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM upstream_providers WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete provider: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("provider not found")
	}
	return nil
}

// rowScanner matches both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(...any) error
}

func (s *ProviderStore) scan(r rowScanner) (Provider, error) {
	var p Provider
	var enc []byte
	if err := r.Scan(&p.ID, &p.Name, &p.BaseURL, &enc, &p.Protocol, &p.Status, &p.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Provider{}, errors.New("provider not found")
		}
		return Provider{}, err
	}
	dec, err := s.cipher.Decrypt(enc)
	if err != nil {
		return Provider{}, fmt.Errorf("decrypt api key: %w", err)
	}
	p.APIKey = string(dec)
	return p, nil
}
