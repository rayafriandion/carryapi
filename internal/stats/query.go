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

// TrendGranularity 粒度:day=按天, hour=按小时。
type TrendGranularity string

const (
	GranularityDay  TrendGranularity = "day"
	GranularityHour TrendGranularity = "hour"
)

// TrendPoint 一个时间桶的统计。
type TrendPoint struct {
	Bucket       string // "2026-08-10" 或 "2026-08-10T15"
	Requests     int64
	SuccessCount int64
	InputTok     int64
	OutputTok    int64
	Cost         float64
}

// QueryTrend 按粒度返回时间段内各桶的统计。
// SQLite 用 strftime('%Y-%m-%d', created_at) 按天、strftime('%Y-%m-%dT%H', created_at) 按小时。
func QueryTrend(db *sql.DB, p QueryParams, g TrendGranularity) ([]TrendPoint, error) {
	format := "%Y-%m-%d"
	if g == GranularityHour {
		format = "%Y-%m-%dT%H"
	}
	clause, args := whereClause(p)
	query := `SELECT strftime('` + format + `', created_at) AS bucket,
		COUNT(*),
		COALESCE(SUM(CASE WHEN status_code BETWEEN 200 AND 299 AND error_type='none' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(input_tokens), 0),
		COALESCE(SUM(output_tokens), 0),
		COALESCE(SUM(cost), 0)
		FROM request_logs ` + clause + ` GROUP BY bucket ORDER BY bucket ASC`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query trend: %w", err)
	}
	defer rows.Close()
	var out []TrendPoint
	for rows.Next() {
		var tp TrendPoint
		if err := rows.Scan(&tp.Bucket, &tp.Requests, &tp.SuccessCount, &tp.InputTok, &tp.OutputTok, &tp.Cost); err != nil {
			return nil, err
		}
		out = append(out, tp)
	}
	return out, rows.Err()
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

type CostGroup string

const (
	CostByModel    CostGroup = "model"
	CostByKey      CostGroup = "key"
	CostByProvider CostGroup = "provider"
)

type CostRow struct {
	Group     string
	Requests  int64
	TotalCost float64
}

func QueryCost(db *sql.DB, p QueryParams, group CostGroup) ([]CostRow, error) {
	var selectExpr, groupExpr string
	switch group {
	case CostByModel:
		selectExpr, groupExpr = "custom_model", "custom_model"
	case CostByKey:
		selectExpr, groupExpr = "COALESCE(ak.key_prefix,'?') || ' ' || COALESCE(ak.label,'')", "rl.api_key_id"
		// 需要 join,单独处理
	case CostByProvider:
		selectExpr, groupExpr = "COALESCE(up.name,'unknown')", "rl.provider_id"
	default:
		return nil, fmt.Errorf("unknown cost group %q", group)
	}

	clause, args := whereClause(p)
	if group == CostByKey {
		query := `SELECT ` + selectExpr + `, COUNT(*), COALESCE(SUM(rl.cost),0)
			FROM request_logs rl LEFT JOIN api_keys ak ON rl.api_key_id = ak.id
			WHERE rl.created_at >= ? AND rl.created_at <= ?` + userClause(p) + `
			AND rl.api_key_id IS NOT NULL GROUP BY rl.api_key_id ORDER BY COUNT(*) DESC`
		args = append([]any{p.Start, p.End}, userArgs(p)...)
		return queryCostRows(db, query, args)
	}
	if group == CostByProvider {
		query := `SELECT ` + selectExpr + `, COUNT(*), COALESCE(SUM(rl.cost),0)
			FROM request_logs rl LEFT JOIN upstream_providers up ON rl.provider_id = up.id
			WHERE rl.created_at >= ? AND rl.created_at <= ?` + userClause(p) + `
			AND rl.provider_id IS NOT NULL GROUP BY rl.provider_id ORDER BY COUNT(*) DESC`
		args = append([]any{p.Start, p.End}, userArgs(p)...)
		return queryCostRows(db, query, args)
	}
	query := `SELECT ` + selectExpr + `, COUNT(*), COALESCE(SUM(cost),0)
		FROM request_logs ` + clause + ` GROUP BY ` + groupExpr + ` ORDER BY COUNT(*) DESC`
	return queryCostRows(db, query, args)
}

func queryCostRows(db *sql.DB, query string, args []any) ([]CostRow, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query cost: %w", err)
	}
	defer rows.Close()
	var out []CostRow
	for rows.Next() {
		var r CostRow
		if err := rows.Scan(&r.Group, &r.Requests, &r.TotalCost); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type SuccessStat struct {
	Group         string
	Total         int64
	Success       int64
	Failed        int64
	SuccessRate   float64
	AvgDurationMs float64
}

// QuerySuccessRate 按维度返回成功率。
// group 取值:"model" | "provider" | "key"。
func QuerySuccessRate(db *sql.DB, p QueryParams, group string) ([]SuccessStat, error) {
	var selectExpr, join, groupExpr string
	switch group {
	case "model":
		selectExpr, join, groupExpr = "rl.custom_model", "", "rl.custom_model"
	case "provider":
		selectExpr, join, groupExpr = "COALESCE(up.name,'unknown')", "LEFT JOIN upstream_providers up ON rl.provider_id = up.id", "rl.provider_id"
	case "key":
		selectExpr, join, groupExpr = "COALESCE(ak.key_prefix,'?') || ' ' || COALESCE(ak.label,'')", "LEFT JOIN api_keys ak ON rl.api_key_id = ak.id", "rl.api_key_id"
	default:
		return nil, fmt.Errorf("unknown success group %q", group)
	}

	// 统一参数:时间范围 + 可选 user 过滤(普通用户查自己)
	clause := "rl.created_at >= ? AND rl.created_at <= ?" + userClause(p)
	args := append([]any{p.Start, p.End}, userArgs(p)...)

	query := `SELECT ` + selectExpr + `, COUNT(*),
		COALESCE(SUM(CASE WHEN rl.status_code BETWEEN 200 AND 299 AND rl.error_type='none' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN rl.status_code < 200 OR rl.status_code >= 300 OR rl.error_type != 'none' THEN 1 ELSE 0 END),0),
		COALESCE(AVG(rl.duration_ms),0)
		FROM request_logs rl ` + join + ` WHERE ` + clause + ` GROUP BY ` + groupExpr + ` ORDER BY COUNT(*) DESC`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query success rate: %w", err)
	}
	defer rows.Close()
	var out []SuccessStat
	for rows.Next() {
		var s SuccessStat
		if err := rows.Scan(&s.Group, &s.Total, &s.Success, &s.Failed, &s.AvgDurationMs); err != nil {
			return nil, err
		}
		if s.Total > 0 {
			s.SuccessRate = float64(s.Success) / float64(s.Total) * 100
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// LogEntry 一条请求日志(含关联信息)。
type LogEntry struct {
	RequestID     string
	UserID        int64
	Email         string
	CustomModel   string
	ProviderName  string // 关联 upstream_providers
	UpstreamModel string
	ProtocolIn    string
	ProtocolOut   string
	InputTokens   int64
	OutputTokens  int64
	CacheRead     int64
	CacheCreation int64
	Cost          float64
	DurationMs    int64
	StatusCode    int
	ErrorType     string
	ErrorMessage  string
	Stream        bool
	CreatedAt     time.Time
}

// LogFilter 日志筛选条件。
type LogFilter struct {
	Start      time.Time
	End        time.Time
	UserID     *int64 // nil=全部
	Model      string
	StatusCode *int
	ErrorType  string
	RequestID  string
	Page       int // 1-based
	PageSize   int // 默认 50,上限 200
}

func (f *LogFilter) normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 50
	}
	if f.PageSize > 200 {
		f.PageSize = 200
	}
}

// logsWhere 构造日志查询的 WHERE 子句。
func (f *LogFilter) whereClause() (string, []any) {
	clause := "rl.created_at >= ? AND rl.created_at <= ?"
	args := []any{f.Start, f.End}
	if f.UserID != nil {
		clause += " AND rl.user_id = ?"
		args = append(args, *f.UserID)
	}
	if f.Model != "" {
		clause += " AND rl.custom_model = ?"
		args = append(args, f.Model)
	}
	if f.StatusCode != nil {
		clause += " AND rl.status_code = ?"
		args = append(args, *f.StatusCode)
	}
	if f.ErrorType != "" {
		clause += " AND rl.error_type = ?"
		args = append(args, f.ErrorType)
	}
	if f.RequestID != "" {
		clause += " AND rl.request_id = ?"
		args = append(args, f.RequestID)
	}
	return clause, args
}

// QueryLogs 返回分页日志 + 总数。
func QueryLogs(db *sql.DB, f LogFilter) (int64, []LogEntry, error) {
	f.normalize()
	clause, args := f.whereClause()

	var total int64
	err := db.QueryRow(`SELECT COUNT(*) FROM request_logs rl WHERE `+clause, args...).Scan(&total)
	if err != nil {
		return 0, nil, fmt.Errorf("count logs: %w", err)
	}

	offset := (f.Page - 1) * f.PageSize
	query := `SELECT rl.request_id, rl.user_id, COALESCE(u.email,''), rl.custom_model,
		COALESCE(up.name,''), COALESCE(rl.upstream_model,''), COALESCE(rl.protocol_in,''), COALESCE(rl.protocol_out,''),
		COALESCE(rl.input_tokens,0), COALESCE(rl.output_tokens,0), COALESCE(rl.cache_read_tokens,0), COALESCE(rl.cache_creation_tokens,0),
		COALESCE(rl.cost,0), COALESCE(rl.duration_ms,0), rl.status_code, COALESCE(rl.error_type,''), COALESCE(rl.error_message,''),
		COALESCE(rl.stream, 0), rl.created_at
		FROM request_logs rl
		LEFT JOIN users u ON rl.user_id = u.id
		LEFT JOIN upstream_providers up ON rl.provider_id = up.id
		WHERE ` + clause + ` ORDER BY rl.created_at DESC, rl.id DESC LIMIT ? OFFSET ?`
	args = append(args, f.PageSize, offset)
	rows, err := db.Query(query, args...)
	if err != nil {
		return 0, nil, fmt.Errorf("query logs: %w", err)
	}
	defer rows.Close()
	var items []LogEntry
	for rows.Next() {
		var e LogEntry
		// user_id 迁移后可为 NULL(鉴权失败行),cost 为 REAL(可为小数)。
		var nu sql.NullInt64
		var cost float64
		if err := rows.Scan(&e.RequestID, &nu, &e.Email, &e.CustomModel, &e.ProviderName,
			&e.UpstreamModel, &e.ProtocolIn, &e.ProtocolOut, &e.InputTokens, &e.OutputTokens,
			&e.CacheRead, &e.CacheCreation, &cost, &e.DurationMs, &e.StatusCode, &e.ErrorType,
			&e.ErrorMessage, &e.Stream, &e.CreatedAt); err != nil {
			return 0, nil, err
		}
		e.UserID = nu.Int64 // NULL -> 0
		e.Cost = cost
		items = append(items, e)
	}
	return total, items, rows.Err()
}
