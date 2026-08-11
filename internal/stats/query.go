package stats

import (
	"database/sql"
	"fmt"
	"time"
)

type QueryParams struct {
	Start  time.Time
	End    time.Time
	UserID *int64 // nil=全部;非 nil=只看该用户
	Model  string // 可选模型过滤
}

type Summary struct {
	TotalRequests  int64
	SuccessCount   int64
	ErrorCount     int64
	TotalInputTok  int64
	TotalOutputTok int64
	TotalCacheRead int64
	TotalCost      float64
	AvgDurationMs  float64
	ByModel        []ModelStat
	ByProvider     []ProviderStat
	ByKey          []KeyStat
}

type ModelStat struct {
	Model        string
	Requests     int64
	InputTokens  int64
	OutputTokens int64
	Cost         float64
}

type ProviderStat struct {
	ProviderID   int64
	ProviderName string
	Requests     int64
	Cost         float64
}

type KeyStat struct {
	KeyID     int64
	KeyPrefix string
	Label     string
	Requests  int64
	Cost      float64
}

// whereClause 构造 WHERE 子句(含时间范围 + 可选过滤)。
// 返回 (clause, args)。
func whereClause(p QueryParams) (string, []any) {
	clause := "WHERE created_at >= ? AND created_at <= ?"
	args := []any{p.Start, p.End}
	if p.UserID != nil {
		clause += " AND user_id = ?"
		args = append(args, *p.UserID)
	}
	if p.Model != "" {
		clause += " AND custom_model = ?"
		args = append(args, p.Model)
	}
	return clause, args
}

func QuerySummary(db *sql.DB, p QueryParams) (*Summary, error) {
	clause, args := whereClause(p)
	s := &Summary{}

	err := db.QueryRow(`SELECT COUNT(*),
		COALESCE(SUM(CASE WHEN status_code BETWEEN 200 AND 299 AND error_type='none' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN status_code < 200 OR status_code >= 300 OR error_type != 'none' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(input_tokens), 0),
		COALESCE(SUM(output_tokens), 0),
		COALESCE(SUM(cache_read_tokens), 0),
		COALESCE(SUM(cost), 0),
		COALESCE(AVG(duration_ms), 0)
		FROM request_logs `+clause, args...).Scan(
		&s.TotalRequests, &s.SuccessCount, &s.ErrorCount,
		&s.TotalInputTok, &s.TotalOutputTok, &s.TotalCacheRead, &s.TotalCost, &s.AvgDurationMs)
	if err != nil {
		return nil, fmt.Errorf("summary totals: %w", err)
	}

	// ByModel
	rows, err := db.Query(`SELECT custom_model, COUNT(*), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0), COALESCE(SUM(cost),0)
		FROM request_logs `+clause+` GROUP BY custom_model ORDER BY COUNT(*) DESC`, args...)
	if err != nil {
		return nil, fmt.Errorf("summary by model: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m ModelStat
		if err := rows.Scan(&m.Model, &m.Requests, &m.InputTokens, &m.OutputTokens, &m.Cost); err != nil {
			return nil, err
		}
		s.ByModel = append(s.ByModel, m)
	}

	// ByProvider(provider_id 非空才分组)
	rows, err = db.Query(`SELECT rl.provider_id, COALESCE(up.name, 'unknown'), COUNT(*), COALESCE(SUM(rl.cost),0)
		FROM request_logs rl LEFT JOIN upstream_providers up ON rl.provider_id = up.id
		WHERE rl.created_at >= ? AND rl.created_at <= ?
		AND rl.provider_id IS NOT NULL`+userClause(p)+` GROUP BY rl.provider_id ORDER BY COUNT(*) DESC`,
		append([]any{p.Start, p.End}, userArgs(p)...)...)
	if err != nil {
		return nil, fmt.Errorf("summary by provider: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ps ProviderStat
		if err := rows.Scan(&ps.ProviderID, &ps.ProviderName, &ps.Requests, &ps.Cost); err != nil {
			return nil, err
		}
		s.ByProvider = append(s.ByProvider, ps)
	}

	// ByKey
	rows, err = db.Query(`SELECT rl.api_key_id, ak.key_prefix, COALESCE(ak.label, ''), COUNT(*), COALESCE(SUM(rl.cost),0)
		FROM request_logs rl LEFT JOIN api_keys ak ON rl.api_key_id = ak.id
		WHERE rl.created_at >= ? AND rl.created_at <= ?
		AND rl.api_key_id IS NOT NULL`+userClause(p)+` GROUP BY rl.api_key_id ORDER BY COUNT(*) DESC`,
		append([]any{p.Start, p.End}, userArgs(p)...)...)
	if err != nil {
		return nil, fmt.Errorf("summary by key: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ks KeyStat
		if err := rows.Scan(&ks.KeyID, &ks.KeyPrefix, &ks.Label, &ks.Requests, &ks.Cost); err != nil {
			return nil, err
		}
		s.ByKey = append(s.ByKey, ks)
	}

	return s, nil
}

// userClause 返回附加的 user 过滤子句(仅当 UserID 非空)。
func userClause(p QueryParams) string {
	if p.UserID != nil {
		return " AND rl.user_id = ?"
	}
	return ""
}

func userArgs(p QueryParams) []any {
	if p.UserID != nil {
		return []any{*p.UserID}
	}
	return nil
}
