# carryAPI 子项目 5:统计与管理 API 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现统计查询层:汇总(summary)、时间趋势(trend)、费用核算(cost)、成功率(success-rate)、请求日志查询(logs)。所有查询基于子项目 4 已写入的 `request_logs` 表,加上 catalog/keys/users 做维度关联。权限:普通用户只看自己的日志/统计,admin 看全部。

**Architecture:** 新包 `internal/stats`。分两层:
- 查询层(`query.go`):纯 SQL 聚合函数,返回结构体,不依赖 HTTP。所有查询按用户 role 做 scope 过滤(admin 无过滤,普通用户 `WHERE user_id=?`)。
- Handler 层(`handler.go`):HTTP 端点,从 context 取用户,解析查询参数(时间范围/分组维度/分页),调查询层,返回 JSON。
- 路由挂载到 `/api/stats/*` 和 `/api/logs`,需登录(session 鉴权,复用子项目 2 的 SessionMiddleware + RequireLogin)。
- 普通用户只能查自己;admin 可查全部(通过 role 判断,不需要 ?user_id 参数——admin 视角默认全部)。

**Tech Stack:** Go 标准库(`database/sql`、`net/http`、`time`、`encoding/json`)+ `internal/user`(取当前用户)、`internal/middleware`(RequireLogin)、`internal/server`(Deps/路由)。无 CGO,无新依赖。

## Global Constraints

- Go 1.22+;无 CGO。
- 所有查询只读,不得修改数据。请求日志保留/清理仍由子项目 6 的定时任务负责(本子项目只查询)。
- 时间范围参数 `start`/`end`:RFC3339 格式(如 `2026-08-01T00:00:00Z`),缺省时 start=30 天前,end=now。
- 分组维度:模型=request_logs.custom_model;上游=request_logs.provider_id JOIN upstream_providers.name;Key=request_logs.api_key_id JOIN api_keys(prefix/label)。provider_id/api_key_id 为 NULL 的行(认证失败)不计入按维度分组,但计入总数。
- 权限:普通用户查询自动加 `AND user_id=?`(从 context 取);admin 不加。admin 可用 `?user_id=` 过滤指定用户。
- 成功率:成功 = status_code BETWEEN 200 AND 299 且 error_type='none'。成功率 = 成功/总数×100。
- 费用:`cost` 列已由子项目 4 按当时价格快照,直接 SUM 即可,不做历史重算。
- 分页:logs 端点 `?page=1&page_size=50`(默认),page_size 上限 200;返回 `{total, page, page_size, items}`。
- TDD:每个任务先写失败测试,再实现,再验证通过,再提交。
- Git 身份:`rayafriandion <amizhisa@outlook.com>`(本仓库已配置)。

---

## File Structure

```
carryAPI/
└── internal/
    ├── stats/
    │   ├── query.go            # 查询层:纯 SQL 聚合
    │   ├── query_test.go
    │   ├── handler.go          # HTTP handlers:summary/trend/cost/success-rate/logs
    │   ├── handler_test.go
    │   └── fixtures_test.go    # 测试数据 helper(插入 request_logs 样本)
    └── server/
        ├── router.go           # (修改)挂载 /api/stats/* 和 /api/logs
        └── server.go           # (修改)Deps 加 Stats
```

---

### Task 1: 查询层基础 + Summary

**Files:**
- Create: `internal/stats/query.go`
- Test: `internal/stats/query_test.go`

**Interfaces:**
- Consumes: `*sql.DB`
- Produces:

```go
// 查询参数:统一的时间范围 + 可选过滤。
type QueryParams struct {
    Start   time.Time
    End     time.Time
    UserID  *int64 // nil=全部(admin);非 nil=只看该用户
    Model   string // 可选:只统计某模型
}

// Summary 汇总结果。
type Summary struct {
    TotalRequests  int64
    SuccessCount   int64
    ErrorCount     int64
    TotalInputTok  int64
    TotalOutputTok int64
    TotalCacheRead int64
    TotalCost      float64
    AvgDurationMs  float64
    ByModel []ModelStat   // 按模型分组
    ByProvider []ProviderStat // 按上游分组
    ByKey    []KeyStat    // 按 Key 分组
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
    KeyID    int64
    KeyPrefix string
    Label    string
    Requests int64
    Cost     float64
}

// QuerySummary 返回时间段内汇总。
func QuerySummary(db *sql.DB, p QueryParams) (*Summary, error)
```

- [ ] **Step 1: 写失败测试**

`internal/stats/query_test.go`:

```go
package stats

import (
	"testing"
	"time"

	"carryapi/internal/db"
)

// fixture 建 in-memory db + 迁移 + 返回 db。
func newDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// seedLogs 插入样本 request_logs(直接 INSERT,绕过 proxy)。
func seedLogs(t *testing.T, d *sql.DB) {
	t.Helper()
	// user 1 的两条成功 + 一条失败;user 2 的一条成功
	// provider 1 / key 1 关联
	inserts := []struct {
		userID    int64
		keyID     int64
		model     string
		provID    *int64
		inTok     int64
		outTok    int64
		cacheRead int64
		cost      float64
		status    int
		errType   string
	}{
		{1, 1, "my-gpt4", int64Ptr(1), 100, 50, 10, 0.005, 200, "none"},
		{1, 1, "my-gpt4", int64Ptr(1), 200, 100, 20, 0.01, 200, "none"},
		{1, 2, "my-claude", int64Ptr(2), 50, 25, 0, 0.003, 400, "invalid_request"},
		{2, 3, "my-gpt4", int64Ptr(1), 300, 150, 30, 0.015, 200, "none"},
	}
	for _, ins := range inserts {
		_, err := d.Exec(
			`INSERT INTO request_logs(user_id, api_key_id, custom_model, provider_id, upstream_model,
			 protocol_in, protocol_out, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
			 cost, status_code, error_type, created_at)
			 VALUES(?, ?, ?, ?, 'm', 'chat', 'chat', ?, ?, ?, 0, ?, ?, ?, datetime('now'))`,
			ins.userID, ins.keyID, ins.model, ins.provID, ins.inTok, ins.outTok, ins.cacheRead, ins.cost, ins.status, ins.errType)
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

func int64Ptr(v int64) *int64 { return &v }

func TestQuerySummaryAll(t *testing.T) {
	d := newDB(t)
	seedLogs(t, d)
	s, err := QuerySummary(d, QueryParams{Start: time.Now().Add(-24 * time.Hour), End: time.Now()})
	if err != nil {
		t.Fatalf("QuerySummary: %v", err)
	}
	if s.TotalRequests != 4 {
		t.Errorf("TotalRequests = %d, want 4", s.TotalRequests)
	}
	if s.SuccessCount != 3 || s.ErrorCount != 1 {
		t.Errorf("success/error = %d/%d, want 3/1", s.SuccessCount, s.ErrorCount)
	}
	if s.TotalInputTok != 650 || s.TotalOutputTok != 325 {
		t.Errorf("tokens = %d/%d, want 650/325", s.TotalInputTok, s.TotalOutputTok)
	}
	if s.TotalCost != 0.033 {
		t.Errorf("cost = %f, want 0.033", s.TotalCost)
	}
	// ByModel: my-gpt4 x3, my-claude x1
	if len(s.ByModel) != 2 {
		t.Fatalf("ByModel = %d, want 2", len(s.ByModel))
	}
	// ByProvider: provider 1 x3, provider 2 x1
	if len(s.ByProvider) != 2 {
		t.Errorf("ByProvider = %d, want 2", len(s.ByProvider))
	}
}

func TestQuerySummaryUserFilter(t *testing.T) {
	d := newDB(t)
	seedLogs(t, d)
	uid := int64(1)
	s, err := QuerySummary(d, QueryParams{Start: time.Now().Add(-24 * time.Hour), End: time.Now(), UserID: &uid})
	if err != nil {
		t.Fatalf("QuerySummary: %v", err)
	}
	if s.TotalRequests != 3 {
		t.Errorf("TotalRequests = %d, want 3 (user 1 only)", s.TotalRequests)
	}
}

func TestQuerySummaryTimeRange(t *testing.T) {
	d := newDB(t)
	seedLogs(t, d)
	// 时间范围在过去(无数据)
	start := time.Now().Add(-48 * time.Hour)
	end := time.Now().Add(-25 * time.Hour)
	s, err := QuerySummary(d, QueryParams{Start: start, End: end})
	if err != nil {
		t.Fatalf("QuerySummary: %v", err)
	}
	if s.TotalRequests != 0 {
		t.Errorf("TotalRequests = %d, want 0 (outside range)", s.TotalRequests)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /d/Projects/carryAPI
go test ./internal/stats/ -v
```

预期:编译失败(找不到 stats 包)。

- [ ] **Step 3: 实现 query.go**

```go
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
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/stats/ -v
```

预期:3 个测试 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/stats/query.go internal/stats/query_test.go
git commit -m "feat(stats): summary query with model/provider/key grouping"
```

---

### Task 2: Trend(时间趋势)

**Files:**
- Modify: `internal/stats/query.go`
- Modify: `internal/stats/query_test.go`

**Interfaces:**
- Produces:

```go
// TrendPoint 一个时间桶的统计。
type TrendPoint struct {
    Bucket     string // "2026-08-10" 或 "2026-08-10T15"
    Requests   int64
    SuccessCount int64
    InputTok   int64
    OutputTok  int64
    Cost       float64
}

// TrendGranularity 粒度:day=按天, hour=按小时。
type TrendGranularity string
const (
    GranularityDay  TrendGranularity = "day"
    GranularityHour TrendGranularity = "hour"
)

// QueryTrend 按粒度返回时间段内各桶的统计。
// SQLite 用 strftime('%Y-%m-%d', created_at) 按天、strftime('%Y-%m-%dT%H', created_at) 按小时。
func QueryTrend(db *sql.DB, p QueryParams, g TrendGranularity) ([]TrendPoint, error)
```

- [ ] **Step 1: 写失败测试**

`internal/stats/query_test.go` 追加:

```go
func TestQueryTrendDay(t *testing.T) {
	d := newDB(t)
	seedLogs(t, d)
	points, err := QueryTrend(d, QueryParams{Start: time.Now().Add(-24 * time.Hour), End: time.Now()}, GranularityDay)
	if err != nil {
		t.Fatalf("QueryTrend: %v", err)
	}
	if len(points) == 0 {
		t.Fatal("no trend points")
	}
	// 所有样本都在今天 -> 应合并为 1 个桶(4 请求)
	if points[len(points)-1].Requests != 4 {
		t.Errorf("last bucket requests = %d, want 4", points[len(points)-1].Requests)
	}
}

func TestQueryTrendHour(t *testing.T) {
	d := newDB(t)
	seedLogs(t, d)
	points, err := QueryTrend(d, QueryParams{Start: time.Now().Add(-2 * time.Hour), End: time.Now()}, GranularityHour)
	if err != nil {
		t.Fatalf("QueryTrend: %v", err)
	}
	if len(points) == 0 {
		t.Fatal("no hour points")
	}
}
```

> 注:样本 created_at 都用 `datetime('now')`,同一时刻——按天合并成 1 桶、按小时也基本 1 桶(除非跨小时边界)。测试只断言桶数 > 0 和末桶请求数。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/stats/ -v -run Trend
```

预期:编译失败。

- [ ] **Step 3: 实现 QueryTrend**

在 `internal/stats/query.go` 追加:

```go
type TrendGranularity string

const (
	GranularityDay  TrendGranularity = "day"
	GranularityHour TrendGranularity = "hour"
)

type TrendPoint struct {
	Bucket       string
	Requests     int64
	SuccessCount int64
	InputTok     int64
	OutputTok    int64
	Cost         float64
}

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
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/stats/ -v
```

预期:全部 PASS(3 summary + 2 trend)。

- [ ] **Step 5: 提交**

```bash
git add internal/stats/query.go internal/stats/query_test.go
git commit -m "feat(stats): time trend query by day and hour"
```

---

### Task 3: Cost + SuccessRate

**Files:**
- Modify: `internal/stats/query.go`
- Modify: `internal/stats/query_test.go`

**Interfaces:**
- Produces:

```go
// CostGroup 费用分组维度。
type CostGroup string
const (
    CostByModel    CostGroup = "model"
    CostByKey      CostGroup = "key"
    CostByProvider CostGroup = "provider"
)

// CostRow 一行费用统计。
type CostRow struct {
    Group    string // 模型名 / key prefix+label / provider 名
    Requests int64
    TotalCost float64
}

// QueryCost 按维度返回费用统计。
func QueryCost(db *sql.DB, p QueryParams, group CostGroup) ([]CostRow, error)

// SuccessStat 成功率统计。
type SuccessStat struct {
    Group        string // 模型名 / provider 名 / key 标识
    Total        int64
    Success      int64
    Failed       int64
    SuccessRate  float64 // 0-100
    AvgDurationMs float64
}

// QuerySuccessRate 按维度返回成功率(总/成功/失败/率/平均耗时)。
// 维度:model / provider / key。
func QuerySuccessRate(db *sql.DB, p QueryParams, group string) ([]SuccessStat, error)
```

- [ ] **Step 1: 写失败测试**

`internal/stats/query_test.go` 追加:

```go
func TestQueryCostByModel(t *testing.T) {
	d := newDB(t)
	seedLogs(t, d)
	rows, err := QueryCost(d, QueryParams{Start: time.Now().Add(-24 * time.Hour), End: time.Now()}, CostByModel)
	if err != nil {
		t.Fatalf("QueryCost: %v", err)
	}
	// my-gpt4 cost=0.005+0.01+0.015=0.03; my-claude=0.003
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	for _, r := range rows {
		if r.Group == "my-gpt4" && r.TotalCost != 0.03 {
			t.Errorf("my-gpt4 cost = %f, want 0.03", r.TotalCost)
		}
	}
}

func TestQuerySuccessRateByModel(t *testing.T) {
	d := newDB(t)
	seedLogs(t, d)
	rows, err := QuerySuccessRate(d, QueryParams{Start: time.Now().Add(-24 * time.Hour), End: time.Now()}, "model")
	if err != nil {
		t.Fatalf("QuerySuccessRate: %v", err)
	}
	// my-gpt4: 3 成功 0 失败 -> 100%; my-claude: 0 成功 1 失败 -> 0%
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	for _, r := range rows {
		if r.Group == "my-gpt4" {
			if r.Total != 3 || r.Success != 3 || r.Failed != 0 || r.SuccessRate != 100.0 {
				t.Errorf("my-gpt4 stat = %+v", r)
			}
		}
		if r.Group == "my-claude" {
			if r.Total != 1 || r.Success != 0 || r.Failed != 1 || r.SuccessRate != 0.0 {
				t.Errorf("my-claude stat = %+v", r)
			}
		}
	}
}

func TestQuerySuccessRateByProvider(t *testing.T) {
	d := newDB(t)
	seedLogs(t, d)
	rows, err := QuerySuccessRate(d, QueryParams{Start: time.Now().Add(-24 * time.Hour), End: time.Now()}, "provider")
	if err != nil {
		t.Fatalf("QuerySuccessRate: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2 (providers 1,2)", len(rows))
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/stats/ -v -run "Cost|SuccessRate"
```

预期:编译失败。

- [ ] **Step 3: 实现 QueryCost + QuerySuccessRate**

在 `internal/stats/query.go` 追加:

```go
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
```

> 注:`QuerySuccessRate` 统一用 `WHERE rl.created_at >= ? AND rl.created_at <= ?` + userClause,参数 `[]any{p.Start, p.End}` + userArgs;model 维度也用 `rl.` 前缀(join 为空时 from 是 `FROM request_logs rl `)。

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/stats/ -v
```

预期:全部 PASS(3 summary + 2 trend + 3 cost/success)。

- [ ] **Step 5: 提交**

```bash
git add internal/stats/query.go internal/stats/query_test.go
git commit -m "feat(stats): cost and success-rate queries"
```

---

### Task 4: Logs 查询(分页 + 筛选)

**Files:**
- Modify: `internal/stats/query.go`
- Modify: `internal/stats/query_test.go`

**Interfaces:**
- Produces:

```go
// LogEntry 一条请求日志(含关联信息)。
type LogEntry struct {
    RequestID    string
    UserID       int64
    Email        string
    CustomModel  string
    ProviderName string // 关联 upstream_providers
    UpstreamModel string
    ProtocolIn   string
    ProtocolOut  string
    InputTokens  int64
    OutputTokens int64
    CacheRead    int64
    CacheCreation int64
    Cost         float64
    DurationMs   int64
    StatusCode   int
    ErrorType    string
    ErrorMessage string
    Stream       bool
    CreatedAt    time.Time
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

// QueryLogs 返回分页日志 + 总数。
func QueryLogs(db *sql.DB, f LogFilter) (total int64, items []LogEntry, err error)
```

- [ ] **Step 1: 写失败测试**

`internal/stats/query_test.go` 追加:

```go
func TestQueryLogsPagination(t *testing.T) {
	d := newDB(t)
	seedLogs(t, d)
	total, items, err := QueryLogs(d, LogFilter{
		Start: time.Now().Add(-24 * time.Hour), End: time.Now(),
		Page: 1, PageSize: 2,
	})
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}
	if len(items) != 2 {
		t.Errorf("items = %d, want 2 (page size)", len(items))
	}
	// 第二页
	_, items2, _ := QueryLogs(d, LogFilter{
		Start: time.Now().Add(-24 * time.Hour), End: time.Now(),
		Page: 2, PageSize: 2,
	})
	if len(items2) != 2 {
		t.Errorf("page 2 items = %d, want 2", len(items2))
	}
}

func TestQueryLogsFilterModel(t *testing.T) {
	d := newDB(t)
	seedLogs(t, d)
	total, _, err := QueryLogs(d, LogFilter{
		Start: time.Now().Add(-24 * time.Hour), End: time.Now(),
		Model: "my-claude", Page: 1, PageSize: 50,
	})
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1 (my-claude only)", total)
	}
}

func TestQueryLogsFilterErrorType(t *testing.T) {
	d := newDB(t)
	seedLogs(t, d)
	total, _, _ := QueryLogs(d, LogFilter{
		Start: time.Now().Add(-24 * time.Hour), End: time.Now(),
		ErrorType: "invalid_request", Page: 1, PageSize: 50,
	})
	if total != 1 {
		t.Errorf("total = %d, want 1 (invalid_request only)", total)
	}
}

func TestQueryLogsUserFilter(t *testing.T) {
	d := newDB(t)
	seedLogs(t, d)
	uid := int64(1)
	total, _, _ := QueryLogs(d, LogFilter{
		Start: time.Now().Add(-24 * time.Hour), End: time.Now(),
		UserID: &uid, Page: 1, PageSize: 50,
	})
	if total != 3 {
		t.Errorf("total = %d, want 3 (user 1)", total)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/stats/ -v -run Logs
```

预期:编译失败。

- [ ] **Step 3: 实现 QueryLogs**

在 `internal/stats/query.go` 追加:

```go
type LogEntry struct {
	RequestID     string
	UserID        int64
	Email         string
	CustomModel   string
	ProviderName  string
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

type LogFilter struct {
	Start      time.Time
	End        time.Time
	UserID     *int64
	Model      string
	StatusCode *int
	ErrorType  string
	RequestID  string
	Page       int
	PageSize   int
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
		COALESCE(up.name,''), rl.upstream_model, rl.protocol_in, rl.protocol_out,
		rl.input_tokens, rl.output_tokens, rl.cache_read_tokens, rl.cache_creation_tokens,
		rl.cost, rl.duration_ms, rl.status_code, rl.error_type, COALESCE(rl.error_message,''),
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
		if err := rows.Scan(&e.RequestID, &e.UserID, &e.Email, &e.CustomModel, &e.ProviderName,
			&e.UpstreamModel, &e.ProtocolIn, &e.ProtocolOut, &e.InputTokens, &e.OutputTokens,
			&e.CacheRead, &e.CacheCreation, &e.Cost, &e.DurationMs, &e.StatusCode, &e.ErrorType,
			&e.ErrorMessage, &e.Stream, &e.CreatedAt); err != nil {
			return 0, nil, err
		}
		items = append(items, e)
	}
	return total, items, rows.Err()
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/stats/ -v
```

预期:全部 PASS(3 summary + 2 trend + 3 cost/success + 4 logs)。

- [ ] **Step 5: 提交**

```bash
git add internal/stats/query.go internal/stats/query_test.go
git commit -m "feat(stats): paginated log query with filters"
```

---

### Task 5: HTTP handlers + 路由挂载 + 集成测试

**Files:**
- Create: `internal/stats/handler.go`
- Create: `internal/stats/handler_test.go`
- Modify: `internal/server/router.go`(挂载路由)
- Modify: `internal/server/server.go`(Deps 加 Stats)
- Modify: `cmd/carryapi/main.go`(wire stats)
- Test: `internal/stats/integration_test.go`

**Interfaces:**
- Consumes: `*sql.DB`、`internal/middleware`(UserFromContext、RequireLogin)、`internal/user`
- Produces:

```go
// Handler 统计 HTTP handlers。
type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler

// 端点(均需登录):
// GET /api/stats/summary       ?start=&end=&user_id=(admin)
// GET /api/stats/trend         ?start=&end=&granularity=day|hour&user_id=(admin)
// GET /api/stats/cost          ?start=&end=&group=model|key|provider&user_id=(admin)
// GET /api/stats/success-rate  ?start=&end=&group=model|provider|key&user_id=(admin)
// GET /api/logs                ?start=&end=&model=&status=&error_type=&request_id=&page=&page_size=&user_id=(admin)

// 权限:普通用户自动绑定自己的 user_id;admin 可选 ?user_id= 过滤。
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request)
func (h *Handler) Trend(w http.ResponseWriter, r *http.Request)
func (h *Handler) Cost(w http.ResponseWriter, r *http.Request)
func (h *Handler) SuccessRate(w http.ResponseWriter, r *http.Request)
func (h *Handler) Logs(w http.ResponseWriter, r *http.Request)
```

- [ ] **Step 1: 写失败测试**

`internal/stats/handler_test.go`:

```go
package stats

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"carryapi/internal/middleware"
	"carryapi/internal/user"
)

// ctxUser 注入用户到 context。
func ctxUser(uid int64, role string) context.Context {
	u := &user.User{ID: uid, Email: "u@x.com", Role: role, Status: "active"}
	return context.WithValue(context.Background(), middleware.UserKey{}, u)
}

func newHandler(t *testing.T) (*Handler, *sql.DB) {
	d := newDB(t)
	seedLogs(t, d)
	return NewHandler(d), d
}

func TestHandlerSummaryAdmin(t *testing.T) {
	h, _ := newHandler(t)
	req := httptest.NewRequest("GET", "/api/stats/summary", nil)
	req = req.WithContext(ctxUser(1, "admin"))
	rec := httptest.NewRecorder()
	h.Summary(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var s Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.TotalRequests != 4 {
		t.Errorf("TotalRequests = %d, want 4 (admin sees all)", s.TotalRequests)
	}
}

func TestHandlerSummaryUserScoped(t *testing.T) {
	h, _ := newHandler(t)
	req := httptest.NewRequest("GET", "/api/stats/summary", nil)
	req = req.WithContext(ctxUser(2, "user")) // 普通用户
	rec := httptest.NewRecorder()
	h.Summary(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
	var s Summary
	json.Unmarshal(rec.Body.Bytes(), &s)
	if s.TotalRequests != 1 {
		t.Errorf("TotalRequests = %d, want 1 (user 2 only)", s.TotalRequests)
	}
}

func TestHandlerTrend(t *testing.T) {
	h, _ := newHandler(t)
	req := httptest.NewRequest("GET", "/api/stats/trend?granularity=day", nil)
	req = req.WithContext(ctxUser(1, "admin"))
	rec := httptest.NewRecorder()
	h.Trend(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var pts []TrendPoint
	json.Unmarshal(rec.Body.Bytes(), &pts)
	if len(pts) == 0 {
		t.Error("no trend points")
	}
}

func TestHandlerCost(t *testing.T) {
	h, _ := newHandler(t)
	req := httptest.NewRequest("GET", "/api/stats/cost?group=model", nil)
	req = req.WithContext(ctxUser(1, "admin"))
	rec := httptest.NewRecorder()
	h.Cost(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestHandlerSuccessRate(t *testing.T) {
	h, _ := newHandler(t)
	req := httptest.NewRequest("GET", "/api/stats/success-rate?group=model", nil)
	req = req.WithContext(ctxUser(1, "admin"))
	rec := httptest.NewRecorder()
	h.SuccessRate(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
	var rows []SuccessStat
	json.Unmarshal(rec.Body.Bytes(), &rows)
	if len(rows) != 2 {
		t.Errorf("rows = %d, want 2", len(rows))
	}
}

func TestHandlerLogs(t *testing.T) {
	h, _ := newHandler(t)
	req := httptest.NewRequest("GET", "/api/logs?page=1&page_size=10", nil)
	req = req.WithContext(ctxUser(1, "admin"))
	rec := httptest.NewRecorder()
	h.Logs(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Total int64       `json:"total"`
		Items []LogEntry  `json:"items"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Total != 4 || len(resp.Items) != 4 {
		t.Errorf("total/items = %d/%d, want 4/4", resp.Total, len(resp.Items))
	}
}

func TestHandlerRequiresLogin(t *testing.T) {
	h, _ := newHandler(t)
	req := httptest.NewRequest("GET", "/api/stats/summary", nil)
	// 无用户 in context
	rec := httptest.NewRecorder()
	h.Summary(rec, req)
	if rec.Code != 401 {
		t.Errorf("code=%d, want 401", rec.Code)
	}
}
```

> 注:handler 内部自己检查 UserFromContext(缺失 -> 401),不依赖路由层 RequireLogin(路由层也加,双保险)。`seedLogs` 的样本用 `datetime('now')`——QueryParams 缺省时间范围需覆盖(handler 解析缺省 start=30 天前)。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/stats/ -v -run Handler
```

预期:编译失败。

- [ ] **Step 3: 实现 handler.go**

```go
package stats

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"carryapi/internal/middleware"
	"carryapi/internal/user"
)

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// currentUser 返回 context 中的用户;缺失返回 nil。
func currentUser(r *http.Request) *user.User {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		return nil
	}
	return u
}

// params 解析公共查询参数(start/end/user_id),按角色确定范围。
func (h *Handler) params(r *http.Request) (QueryParams, error) {
	u := currentUser(r)
	if u == nil {
		return QueryParams{}, fmt.Errorf("unauthorized")
	}
	start, end, err := parseTimeRange(r)
	if err != nil {
		return QueryParams{}, err
	}
	p := QueryParams{Start: start, End: end}
	if u.Role == "admin" {
		// admin 可选 user_id 过滤
		if v := r.URL.Query().Get("user_id"); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return QueryParams{}, fmt.Errorf("invalid user_id")
			}
			p.UserID = &id
		}
	} else {
		// 普通用户只看自己
		uid := u.ID
		p.UserID = &uid
	}
	return p, nil
}

func parseTimeRange(r *http.Request) (time.Time, time.Time, error) {
	now := time.Now()
	start := now.Add(-30 * 24 * time.Hour)
	end := now
	if v := r.URL.Query().Get("start"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid start (RFC3339): %v", err)
		}
		start = t
	}
	if v := r.URL.Query().Get("end"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end (RFC3339): %v", err)
		}
		end = t
	}
	return start, end, nil
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	p, err := h.params(r)
	if err != nil {
		writeErr(w, 401, "unauthorized")
		return
	}
	s, err := QuerySummary(h.db, p)
	if err != nil {
		writeErr(w, 500, "query failed")
		return
	}
	writeJSON(w, 200, s)
}

func (h *Handler) Trend(w http.ResponseWriter, r *http.Request) {
	p, err := h.params(r)
	if err != nil {
		writeErr(w, 401, "unauthorized")
		return
	}
	g := TrendGranularity(r.URL.Query().Get("granularity"))
	if g == "" {
		g = GranularityDay
	}
	if g != GranularityDay && g != GranularityHour {
		writeErr(w, 400, "granularity must be day or hour")
		return
	}
	pts, err := QueryTrend(h.db, p, g)
	if err != nil {
		writeErr(w, 500, "query failed")
		return
	}
	writeJSON(w, 200, pts)
}

func (h *Handler) Cost(w http.ResponseWriter, r *http.Request) {
	p, err := h.params(r)
	if err != nil {
		writeErr(w, 401, "unauthorized")
		return
	}
	group := CostGroup(r.URL.Query().Get("group"))
	if group == "" {
		group = CostByModel
	}
	rows, err := QueryCost(h.db, p, group)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, rows)
}

func (h *Handler) SuccessRate(w http.ResponseWriter, r *http.Request) {
	p, err := h.params(r)
	if err != nil {
		writeErr(w, 401, "unauthorized")
		return
	}
	group := r.URL.Query().Get("group")
	if group == "" {
		group = "model"
	}
	rows, err := QuerySuccessRate(h.db, p, group)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, rows)
}

func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeErr(w, 401, "unauthorized")
		return
	}
	start, end, err := parseTimeRange(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	f := LogFilter{Start: start, End: end}
	if u.Role == "admin" {
		if v := r.URL.Query().Get("user_id"); v != "" {
			id, _ := strconv.ParseInt(v, 10, 64)
			f.UserID = &id
		}
	} else {
		uid := u.ID
		f.UserID = &uid
	}
	f.Model = r.URL.Query().Get("model")
	if v := r.URL.Query().Get("status"); v != "" {
		if sc, err := strconv.Atoi(v); err == nil {
			f.StatusCode = &sc
		}
	}
	f.ErrorType = r.URL.Query().Get("error_type")
	f.RequestID = r.URL.Query().Get("request_id")
	if v := r.URL.Query().Get("page"); v != "" {
		f.Page, _ = strconv.Atoi(v)
	}
	if v := r.URL.Query().Get("page_size"); v != "" {
		f.PageSize, _ = strconv.Atoi(v)
	}
	total, items, err := QueryLogs(h.db, f)
	if err != nil {
		writeErr(w, 500, "query failed")
		return
	}
	writeJSON(w, 200, map[string]any{
		"total": total, "page": f.Page, "page_size": f.PageSize, "items": items,
	})
}
```

- [ ] **Step 4: 路由挂载 + main.go 接线**

`internal/server/router.go`:在 admin/已登录路由组内加(需登录 + CSRF,但 GET 只读可豁免 CSRF——CSRF 中间件对 GET 放行):

```go
// 已登录用户组(需 SessionMiddleware + RequireLogin;GET 天然豁免 CSRF)
r.Get("/api/stats/summary", s.statsH.Summary)
r.Get("/api/stats/trend", s.statsH.Trend)
r.Get("/api/stats/cost", s.statsH.Cost)
r.Get("/api/stats/success-rate", s.statsH.SuccessRate)
r.Get("/api/logs", s.statsH.Logs)
```

> 放 RequireLogin 组(不要求 admin——普通用户也要看自己的统计)。`Deps` 加 `Stats *stats.Handler`;main.go 构造 `stats.NewHandler(d)` 注入。

- [ ] **Step 5: 运行测试 + 集成测试**

`internal/stats/integration_test.go`(端到端:完整 server 路由):

```go
package stats

import (
	"net/http/httptest"
	"testing"

	"carryapi/internal/db"
	"carryapi/internal/user"
	"github.com/go-chi/chi/v5"
	"carryapi/internal/middleware"
)

// TestStatsRoutesMounted 用 chi 挂载 stats 路由 + 中间件,验证 401(未登录)与 200(已登录)。
func TestStatsRoutesMounted(t *testing.T) {
	d := newDB(t)
	seedLogs(t, d)
	h := NewHandler(d)

	r := chi.NewRouter()
	r.Get("/api/stats/summary", h.Summary)
	r.Get("/api/logs", h.Logs)

	// 未登录 -> 401(handler 内部检查)
	req := httptest.NewRequest("GET", "/api/stats/summary", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("unauth code=%d, want 401", rec.Code)
	}

	// 已登录(admin)
	req = httptest.NewRequest("GET", "/api/stats/summary", nil)
	req = req.WithContext(ctxUser(1, "admin"))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("auth code=%d, want 200", rec.Code)
	}
}
```

`go test ./internal/stats/ -v` 全部 PASS;`go test ./... -count=1` 全绿。

- [ ] **Step 6: 提交**

```bash
git add internal/stats/handler.go internal/stats/handler_test.go internal/stats/integration_test.go internal/server/router.go internal/server/server.go cmd/carryapi/main.go
git commit -m "feat(stats): http handlers for summary, trend, cost, success-rate, logs"
```

---

### Task 6: README + 全量验证

**Files:**
- Modify: `README.md`

**内容:**
- README 加"统计 API"章节:5 个端点 + 参数说明 + 权限说明(普通用户看自己,admin 看全部)。

- [ ] **Step 1: 更新 README**

加"统计 API"章节(在"代理端点"后):
- 端点列表:summary/trend/cost/success-rate/logs + 参数
- 时间范围 RFC3339,缺省 30 天
- 权限:登录后可查;普通用户仅自己的数据

- [ ] **Step 2: 全量验证**

```bash
go test ./... -count=1
```
预期:全部 PASS(新增 stats ~17 测试,全部包合计 220+)。

- [ ] **Step 3: 提交**

```bash
git add README.md
git commit -m "docs: stats api section in readme"
```

---

## 子项目 5 完成标准

- [ ] `go test ./...` 全绿(新增 stats ~17 测试,全部包合计 220+)
- [ ] 5 个统计端点可用:summary/trend/cost/success-rate/logs
- [ ] 普通用户只看自己的数据,admin 看全部(可选 user_id 过滤)
- [ ] 成功率计算正确(2xx 且无 error_type 才算成功)
- [ ] 费用核算基于 request_logs.cost 快照(按模型/Key/上游维度)
- [ ] 日志分页 + 筛选(model/status/error_type/request_id)可用
- [ ] 时间趋势按天/小时
- [ ] 交叉编译仍无 CGO
