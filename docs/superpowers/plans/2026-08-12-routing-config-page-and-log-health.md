# 路由配置页与基于日志的健康状态 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增独立路由配置页(以模型为主视图,展开管理多 binding),并将健康状态判定从内存连续失败计数改为基于 `request_logs` 日志的时间窗口成功率聚合(6 格 × 4h 时间轴 + 详情页性能指标)。

**Architecture:** 新增 `catalog/routing_stats.go`(日志聚合)和 `catalog/health_cache.go`(后台每 1 分钟预算缓存);`Router` 改读缓存而非旧 `health.go`;转发路径加 TTFT 计时 + 客户端断开判定;前端新增 `RoutingView.vue` 并移除旧的上游/路由弹窗和编辑页的 provider 字段。

**Tech Stack:** Go + SQLite + chi(v5)后端;Vue 3 + Naive UI 前端。测试用 `:memory:` SQLite + 内置 `httptest`。

## Global Constraints

- SQLite 数据库,`created_at` 存 UTC,SQL 聚合用 `'localtime'` 转本地时区分桶,Go 侧用 `now.Local()` 对齐。
- 现有 migration 用 `internal/db/migrations.go` 的 `migrations` 切片追加,版本号递增(当前最高 v3,新增 v4)。
- catalog 包测试共享 `catalogFixture`(`model_test.go:15-41`),用 `db.Open(":memory:")` + `SetMaxOpenConns(1)`。
- Handler 测试用 `adminCtx()`(`handler_test.go:14-17`)注入 admin context,挂 chi 路由。
- 路由注册在 `internal/server/router.go` 的 `r.With(middleware.RequireRole("admin"))` 块内(行 101-132)。
- 前端 Vue 视图在 `web/src/views/`,导航在 `web/src/layouts/AppLayout.vue`(或等价路由文件)。

---

## File Structure

| 文件 | 责任 | 新增/改 |
|------|------|---------|
| `internal/db/migrations.go` | 加 v4:`ttft_ms` 列 + 复合索引 | 改 |
| `internal/catalog/routing_stats.go` | 日志聚合:6 格时间轴 + 性能指标 + 单状态 | 新增 |
| `internal/catalog/routing_stats_test.go` | 聚合层单元测试 | 新增 |
| `internal/catalog/health_cache.go` | 后台预算缓存 + `Get` 只读 | 新增 |
| `internal/catalog/health_cache_test.go` | 缓存预算 + 并发读测试 | 新增 |
| `internal/catalog/router.go` | `activeBindings`/`healthSelect` 读 HealthCache | 改 |
| `internal/catalog/router_test.go` | 多 binding 路由测试 | 新增 |
| `internal/catalog/handler.go` | 加 `GetRoutingStatus` + `GetBindingMetrics` | 改 |
| `internal/catalog/handler.go` | `NewHandler` 加 `RoutingStats` 注入 | 改 |
| `internal/catalog/handler_test.go` | 两个新 API 测试 | 改 |
| `internal/catalog/model.go` | `createInTx`/`updateInTx` 不再双写 binding | 改 |
| `internal/proxy/proxy.go` | `requestContext` 加 `firstByteAt`;`Deps` 加 `HealthCache`;`getRouter` 注入 | 改 |
| `internal/proxy/forward.go` | 非流式 `firstByteAt` + 移除 4 处 `Record*` | 改 |
| `internal/proxy/stream.go` | 流式 `firstByteAt` + `isClientDisconnect` | 改 |
| `internal/proxy/stats.go` | 算 `ttftMs` + 写 `ttft_ms` 列 | 改 |
| `internal/proxy/health.go` | 废弃(移除 `bindingHealth`,`Router` 不再用) | 删 |
| `internal/server/router.go` | 两个新路由 | 改 |
| `cmd/carryapi/main.go` | 装配 `routingStats` + `healthCache` | 改 |
| `web/src/views/RoutingView.vue` | 路由配置页 | 新增 |
| `web/src/views/ModelsView.vue` | 移除上游/路由弹窗 | 改 |
| `web/src/views/ModelEditView.vue` | 移除 provider/upstream_model 字段 | 改 |
| 导航/路由表 | 加 `/routing` 入口 | 改 |

---

### Task 1: Migration v4 — ttft_ms 列 + 复合索引

**Files:**
- Modify: `internal/db/migrations.go` (在 `migrations` 切片末尾,v3 之后追加)

**Interfaces:**
- Produces: `request_logs.ttft_ms INTEGER` 列 + `idx_request_logs_provider_model ON request_logs(provider_id, upstream_model, created_at)` 索引。

- [ ] **Step 1: 写迁移语句**

在 `internal/db/migrations.go` 的 `migrations` 切片里,v3 条目之后追加:

```go
	{4, `
ALTER TABLE request_logs ADD COLUMN ttft_ms INTEGER;
CREATE INDEX IF NOT EXISTS idx_request_logs_provider_model
    ON request_logs(provider_id, upstream_model, created_at);
`},
```

- [ ] **Step 2: 写失败测试验证列存在**

在 `internal/db/migrations_test.go`(若无则创建)加测试:

```go
package db

import (
	"database/sql"
	"testing"
)

func TestMigrationV4AddsTtftColumn(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()
	if err := Migrate(d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	var cid string
	err = d.QueryRow(`SELECT name FROM pragma_table_info('request_logs') WHERE name='ttft_ms'`).Scan(&cid)
	if err == sql.ErrNoRows {
		t.Fatal("ttft_ms column not found")
	}
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// 验证索引
	var idxName string
	err = d.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_request_logs_provider_model'`).Scan(&idxName)
	if err != nil {
		t.Fatalf("index not found: %v", err)
	}
}
```

- [ ] **Step 3: 运行测试验证通过**

Run: `go test ./internal/db/ -run TestMigrationV4 -v`
Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add internal/db/migrations.go internal/db/migrations_test.go
git commit -m "feat(db): v4 migration adds ttft_ms column and provider_model index"
```

---

### Task 2: RoutingStats — 日志聚合统计层

**Files:**
- Create: `internal/catalog/routing_stats.go`
- Test: `internal/catalog/routing_stats_test.go`

**Interfaces:**
- Consumes: `*sql.DB`(查询 `request_logs` 表)
- Produces:
  - `type TimeBucket struct { BucketStart time.Time; Total int; Success int; Status string }`
  - `type BindingTimeline struct { ProviderID int64; UpstreamModel string; Buckets []TimeBucket; AvgLatencyMs int64; LastRequestAt *time.Time }`
  - `type BindingMetrics struct { AvgLatencyMs int64; AvgTtftMs int64; ThroughputPerHour float64; TotalRequests24h int; SuccessRate float64 }`
  - `func NewRoutingStats(db *sql.DB) *RoutingStats`
  - `func (s *RoutingStats) BindingTimeline(providerID int64, upstreamModel string, now time.Time) (*BindingTimeline, error)`
  - `func (s *RoutingStats) BindingHealth(providerID int64, upstreamModel string, now time.Time) (string, error)`
  - `func (s *RoutingStats) BindingMetrics(providerID int64, upstreamModel string, now time.Time) (*BindingMetrics, error)`

- [ ] **Step 1: 写失败测试 — 时间轴补齐 + 成功率映射**

```go
package catalog

import (
	"database/sql"
	"testing"
	"time"

	"carryapi/internal/db"
)

func newRoutingStatsFixture(t *testing.T) (*RoutingStats, *sql.DB) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	d.SetMaxOpenConns(1)
	t.Cleanup(func() { d.Close() })
	return NewRoutingStats(d), d
}

// insertLog 插入一条 request_logs,age 表示距 now 多久前。
func insertLog(t *testing.T, d *sql.DB, providerID int64, upstreamModel string, statusCode int, errorType string, age time.Duration, durationMs, ttftMs int64) {
	t.Helper()
	_, err := d.Exec(`INSERT INTO request_logs(request_id, user_id, api_key_id, custom_model, provider_id, upstream_model, protocol_in, protocol_out, duration_ms, status_code, error_type, stream, created_at, ttft_ms)
		VALUES(?, NULL, NULL, 'm', ?, ?, 'chat', 'chat', ?, ?, ?, 0, ?, ?)`,
		"r"+providerID, providerID, upstreamModel, durationMs, statusCode, errorType,
		time.Now().Add(-age).UTC().Format(time.RFC3339), ttftMs)
	if err != nil {
		t.Fatalf("insert log: %v", err)
	}
}

func TestBindingTimelineNoData(t *testing.T) {
	rs, _ := newRoutingStatsFixture(t)
	now := time.Now()
	tl, err := rs.BindingTimeline(1, "gpt-4o", now)
	if err != nil {
		t.Fatalf("BindingTimeline: %v", err)
	}
	if len(tl.Buckets) != 6 {
		t.Fatalf("expected 6 buckets, got %d", len(tl.Buckets))
	}
	for _, b := range tl.Buckets {
		if b.Status != "no_data" || b.Total != 0 {
			t.Errorf("expected no_data/0, got %s/%d", b.Status, b.Total)
		}
	}
}

func TestBindingTimelineHealthy(t *testing.T) {
	rs, d := newRoutingStatsFixture(t)
	now := time.Now()
	// 最近 4h 内:10 成功 + 1 失败 → 成功率 90.9% → warning(80-95%)
	for i := 0; i < 10; i++ {
		insertLog(t, d, 1, "gpt-4o", 200, "none", 1*time.Hour, 100, 50)
	}
	insertLog(t, d, 1, "gpt-4o", 500, "upstream", 1*time.Hour, 0, 0)
	tl, err := rs.BindingTimeline(1, "gpt-4o", now)
	if err != nil {
		t.Fatalf("BindingTimeline: %v", err)
	}
	last := tl.Buckets[5] // 最近 4h
	if last.Status != "warning" {
		t.Errorf("expected warning (90.9%%), got %s (total=%d success=%d)", last.Status, last.Total, last.Success)
	}
}

func TestBindingTimelineUnhealthyAllFail(t *testing.T) {
	rs, d := newRoutingStatsFixture(t)
	now := time.Now()
	// 全失败,低请求量
	insertLog(t, d, 1, "gpt-4o", 500, "upstream", 1*time.Hour, 0, 0)
	insertLog(t, d, 1, "gpt-4o", 500, "upstream", 1*time.Hour, 0, 0)
	tl, err := rs.BindingTimeline(1, "gpt-4o", now)
	if err != nil {
		t.Fatalf("BindingTimeline: %v", err)
	}
	last := tl.Buckets[5]
	if last.Status != "unhealthy" {
		t.Errorf("expected unhealthy (0%% success), got %s", last.Status)
	}
}

func TestBindingTimelineClientDisconnectExcluded(t *testing.T) {
	rs, d := newRoutingStatsFixture(t)
	now := time.Now()
	// 2 成功 + 1 客户端断开 → total=2(排除断开), success=2 → 100% → healthy
	insertLog(t, d, 1, "gpt-4o", 200, "none", 1*time.Hour, 100, 50)
	insertLog(t, d, 1, "gpt-4o", 200, "none", 1*time.Hour, 100, 50)
	insertLog(t, d, 1, "gpt-4o", 200, "client_disconnect", 1*time.Hour, 100, 50)
	tl, err := rs.BindingTimeline(1, "gpt-4o", now)
	if err != nil {
		t.Fatalf("BindingTimeline: %v", err)
	}
	last := tl.Buckets[5]
	if last.Total != 2 || last.Success != 2 {
		t.Errorf("expected total=2 success=2 (client_disconnect excluded), got total=%d success=%d", last.Total, last.Success)
	}
	if last.Status != "healthy" {
		t.Errorf("expected healthy (100%%), got %s", last.Status)
	}
}

func TestBindingMetrics(t *testing.T) {
	rs, d := newRoutingStatsFixture(t)
	now := time.Now()
	insertLog(t, d, 1, "gpt-4o", 200, "none", 1*time.Hour, 100, 50)
	insertLog(t, d, 1, "gpt-4o", 200, "none", 1*time.Hour, 200, 60)
	m, err := rs.BindingMetrics(1, "gpt-4o", now)
	if err != nil {
		t.Fatalf("BindingMetrics: %v", err)
	}
	if m.AvgLatencyMs != 150 {
		t.Errorf("avg latency: expected 150, got %d", m.AvgLatencyMs)
	}
	if m.AvgTtftMs != 55 {
		t.Errorf("avg ttft: expected 55, got %d", m.AvgTtftMs)
	}
	if m.TotalRequests24h != 2 {
		t.Errorf("total: expected 2, got %d", m.TotalRequests24h)
	}
	if m.SuccessRate != 1.0 {
		t.Errorf("success rate: expected 1.0, got %f", m.SuccessRate)
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run: `go test ./internal/catalog/ -run TestBindingTimeline -v`
Expected: FAIL — `RoutingStats` / `NewRoutingStats` 未定义。

- [ ] **Step 3: 实现 RoutingStats**

创建 `internal/catalog/routing_stats.go`:

```go
package catalog

import (
	"database/sql"
	"fmt"
	"time"
)

const (
	StatusHealthy   = "healthy"
	StatusWarning   = "warning"
	StatusUnhealthy = "unhealthy"
	StatusNoData    = "no_data"
)

type TimeBucket struct {
	BucketStart time.Time
	Total       int
	Success     int
	Status      string
}

type BindingTimeline struct {
	ProviderID    int64
	UpstreamModel string
	Buckets       []TimeBucket
	AvgLatencyMs  int64
	LastRequestAt *time.Time
}

type BindingMetrics struct {
	AvgLatencyMs      int64
	AvgTtftMs         int64
	ThroughputPerHour float64
	TotalRequests24h  int
	SuccessRate       float64
}

type RoutingStats struct {
	db *sql.DB
}

func NewRoutingStats(db *sql.DB) *RoutingStats {
	return &RoutingStats{db: db}
}

// statusFromRate 按成功率映射状态色。
func statusFromRate(success, total int) string {
	if total == 0 {
		return StatusNoData
	}
	rate := float64(success) / float64(total)
	switch {
	case rate >= 0.95:
		return StatusHealthy
	case rate >= 0.80:
		return StatusWarning
	default:
		return StatusUnhealthy
	}
}

// BindingTimeline 返回最近 24h 的 6 个 4 小时桶 + 24h 平均延迟。
func (s *RoutingStats) BindingTimeline(providerID int64, upstreamModel string, now time.Time) (*BindingTimeline, error) {
	localNow := now.Local()
	start := localNow.Add(-24 * time.Hour)

	rows, err := s.db.Query(`
		SELECT
		  strftime('%Y-%m-%d %H',
		    datetime(created_at, 'unixepoch', 'localtime',
		             '-' || (strftime('%H', created_at, 'unixepoch', 'localtime') % 4) || ' hours')
		  ) AS bucket,
		  COUNT(CASE WHEN error_type != 'client_disconnect' THEN 1 END) AS total,
		  SUM(CASE WHEN status_code = 200 AND error_type = 'none' THEN 1 ELSE 0 END) AS success
		FROM request_logs
		WHERE provider_id = ? AND upstream_model = ?
		  AND created_at >= ? AND created_at < ?
		GROUP BY bucket
		ORDER BY bucket`,
		providerID, upstreamModel,
		start.UTC().Format(time.RFC3339), localNow.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("query timeline: %w", err)
	}
	defer rows.Close()

	bucketMap := map[string]TimeBucket{}
	for rows.Next() {
		var bk string
		var total, success int
		if err := rows.Scan(&bk, &total, &success); err != nil {
			return nil, fmt.Errorf("scan timeline: %w", err)
		}
		bucketMap[bk] = TimeBucket{Total: total, Success: success}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 补齐 6 个 4 小时桶
	buckets := make([]TimeBucket, 6)
	for i := 0; i < 6; i++ {
		bStart := localNow.Add(-time.Duration(6-1-i) * 4 * time.Hour).Truncate(4 * time.Hour)
		key := bStart.Format("2006-01-02 15")
		b := bucketMap[key]
		b.BucketStart = bStart
		b.Status = statusFromRate(b.Success, b.Total)
		buckets[i] = b
	}

	// 24h 平均延迟
	var avgLatency sql.NullInt64
	_ = s.db.QueryRow(`SELECT AVG(duration_ms) FROM request_logs WHERE provider_id=? AND upstream_model=? AND created_at>=? AND created_at<? AND duration_ms IS NOT NULL`,
		providerID, upstreamModel, start.UTC().Format(time.RFC3339), localNow.UTC().Format(time.RFC3339)).Scan(&avgLatency)

	// 最近请求时间
	var lastReq sql.NullString
	_ = s.db.QueryRow(`SELECT created_at FROM request_logs WHERE provider_id=? AND upstream_model=? ORDER BY created_at DESC LIMIT 1`,
		providerID, upstreamModel).Scan(&lastReq)

	tl := &BindingTimeline{
		ProviderID:    providerID,
		UpstreamModel: upstreamModel,
		Buckets:       buckets,
		AvgLatencyMs:  avgLatency.Int64,
	}
	if lastReq.Valid {
		t, err := time.Parse(time.RFC3339, lastReq.String)
		if err == nil {
			tl.LastRequestAt = &t
		}
	}
	return tl, nil
}

// BindingHealth 返回最近 4 小时桶的状态(供 Router 用)。
func (s *RoutingStats) BindingHealth(providerID int64, upstreamModel string, now time.Time) (string, error) {
	localNow := now.Local()
	start := localNow.Add(-4 * time.Hour)
	var total, success int
	err := s.db.QueryRow(`
		SELECT
		  COUNT(CASE WHEN error_type != 'client_disconnect' THEN 1 END),
		  SUM(CASE WHEN status_code = 200 AND error_type = 'none' THEN 1 ELSE 0 END)
		FROM request_logs
		WHERE provider_id = ? AND upstream_model = ?
		  AND created_at >= ? AND created_at < ?`,
		providerID, upstreamModel,
		start.UTC().Format(time.RFC3339), localNow.UTC().Format(time.RFC3339)).Scan(&total, &success)
	if err != nil {
		return StatusNoData, fmt.Errorf("query health: %w", err)
	}
	return statusFromRate(success, total), nil
}

// BindingMetrics 返回 24h 性能详情。
func (s *RoutingStats) BindingMetrics(providerID int64, upstreamModel string, now time.Time) (*BindingMetrics, error) {
	localNow := now.Local()
	start := localNow.Add(-24 * time.Hour)
	var avgLatency, avgTtft sql.NullInt64
	var total, success int
	err := s.db.QueryRow(`
		SELECT
		  AVG(duration_ms), AVG(ttft_ms),
		  COUNT(*),
		  SUM(CASE WHEN status_code = 200 AND error_type = 'none' THEN 1 ELSE 0 END)
		FROM request_logs
		WHERE provider_id = ? AND upstream_model = ?
		  AND created_at >= ? AND created_at < ?`,
		providerID, upstreamModel,
		start.UTC().Format(time.RFC3339), localNow.UTC().Format(time.RFC3339)).Scan(&avgLatency, &avgTtft, &total, &success)
	if err != nil {
		return nil, fmt.Errorf("query metrics: %w", err)
	}
	m := &BindingMetrics{
		AvgLatencyMs:     avgLatency.Int64,
		AvgTtftMs:        avgTtft.Int64,
		TotalRequests24h: total,
	}
	if total > 0 {
		m.SuccessRate = float64(success) / float64(total)
		m.ThroughputPerHour = float64(total) / 24.0
	}
	return m, nil
}
```

- [ ] **Step 4: 运行测试验证通过**

Run: `go test ./internal/catalog/ -run "TestBindingTimeline|TestBindingMetrics" -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/catalog/routing_stats.go internal/catalog/routing_stats_test.go
git commit -m "feat(catalog): add RoutingStats log aggregation layer"
```

---

### Task 3: HealthCache — 后台预算缓存

**Files:**
- Create: `internal/catalog/health_cache.go`
- Test: `internal/catalog/health_cache_test.go`

**Interfaces:**
- Consumes: `*ModelBindingStore`(遍历 binding)、`*ProviderStore`(拿 active provider)、`*RoutingStats`(算健康)
- Produces:
  - `type HealthCache struct { ... }`
  - `func NewHealthCache(bindings *ModelBindingStore, providers *ProviderStore, stats *RoutingStats) *HealthCache`
  - `func (c *HealthCache) Start(ctx context.Context)` — 每 1 分钟预算
  - `func (c *HealthCache) Get(providerID int64, upstreamModel string) string` — 并发安全读,返回 `"healthy"|"warning"|"unhealthy"|"no_data"`
  - `func (c *HealthCache) Refresh(now time.Time)` — 同步预算一次(测试用)

- [ ] **Step 1: 写失败测试**

```go
package catalog

import (
	"context"
	"testing"
	"time"
)

func TestHealthCacheRefreshAndGet(t *testing.T) {
	f := newCatalogFixture(t)
	rs := NewRoutingStats(f.db)
	hc := NewHealthCache(f.bindingsStore(), f.providers, rs)

	// 插入 binding + 失败日志
	p, _ := f.providers.Create("p", "https://x.com", "k", "openai_chat")
	m, _ := f.models.Create("m", p.ID, "gpt-4o")
	if _, err := f.bindingsStore().Create(m.ID, p.ID, "gpt-4o", 100, 1, true); err != nil {
		t.Fatal(err)
	}
	insertLog(t, f.db, p.ID, "gpt-4o", 500, "upstream", 1*time.Hour, 0, 0)
	insertLog(t, f.db, p.ID, "gpt-4o", 500, "upstream", 1*time.Hour, 0, 0)

	hc.Refresh(time.Now())
	got := hc.Get(p.ID, "gpt-4o")
	if got != StatusUnhealthy {
		t.Errorf("expected unhealthy, got %s", got)
	}
}

func TestHealthCacheNoDataForUnknown(t *testing.T) {
	f := newCatalogFixture(t)
	rs := NewRoutingStats(f.db)
	hc := NewHealthCache(f.bindingsStore(), f.providers, rs)
	hc.Refresh(time.Now())
	got := hc.Get(999, "unknown")
	if got != StatusNoData {
		t.Errorf("expected no_data, got %s", got)
	}
}

func TestHealthCacheStartStopsOnCancel(t *testing.T) {
	f := newCatalogFixture(t)
	rs := NewRoutingStats(f.db)
	hc := NewHealthCache(f.bindingsStore(), f.providers, rs)
	ctx, cancel := context.WithCancel(context.Background())
	go hc.Start(ctx)
	cancel()
	time.Sleep(100 * time.Millisecond)
	// 不 panic 即通过
}
```

> 注:`catalogFixture` 需加 `bindingsStore()` 方法返回 `NewModelBindingStore(f.db)`。在 `model_test.go` 的 `catalogFixture` struct 加字段 `bindings *ModelBindingStore` 并在 `newCatalogFixture` 初始化。若已存在则跳过。

- [ ] **Step 2: 运行测试验证失败**

Run: `go test ./internal/catalog/ -run TestHealthCache -v`
Expected: FAIL — `HealthCache` 未定义。

- [ ] **Step 3: 实现 HealthCache**

创建 `internal/catalog/health_cache.go`:

```go
package catalog

import (
	"context"
	"strconv"
	"sync"
	"time"
)

type cachedHealth struct {
	status string
}

type HealthCache struct {
	bindings  *ModelBindingStore
	providers *ProviderStore
	stats     *RoutingStats
	mu        sync.RWMutex
	states    map[string]cachedHealth
}

func NewHealthCache(bindings *ModelBindingStore, providers *ProviderStore, stats *RoutingStats) *HealthCache {
	return &HealthCache{
		bindings:  bindings,
		providers: providers,
		stats:     stats,
		states:    make(map[string]cachedHealth),
	}
}

func healthCacheKey(providerID int64, upstreamModel string) string {
	return strconv.FormatInt(providerID, 10) + ":" + upstreamModel
}

// Get 并发安全读;未缓存返回 no_data。
func (c *HealthCache) Get(providerID int64, upstreamModel string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	st, ok := c.states[healthCacheKey(providerID, upstreamModel)]
	if !ok {
		return StatusNoData
	}
	return st.status
}

// Refresh 同步预算一次所有 active provider 的 binding。
func (c *HealthCache) Refresh(now time.Time) {
	providers, err := c.providers.List()
	if err != nil {
		return
	}
	newStates := make(map[string]cachedHealth)
	for _, p := range providers {
		if p.Status != "active" {
			continue
		}
		bindings, err := c.bindings.ListByProvider(p.ID)
		if err != nil {
			continue
		}
		for _, b := range bindings {
			if !b.Enabled {
				continue
			}
			status, err := c.stats.BindingHealth(p.ID, b.UpstreamModel, now)
			if err != nil {
				status = StatusNoData
			}
			newStates[healthCacheKey(p.ID, b.UpstreamModel)] = cachedHealth{status: status}
		}
	}
	c.mu.Lock()
	c.states = newStates
	c.mu.Unlock()
}

// Start 后台循环预算,每 1 分钟一次;ctx 取消时退出。
func (c *HealthCache) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	c.Refresh(time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.Refresh(time.Now())
		}
	}
}
```

> 注:需在 `ModelBindingStore` 加 `ListByProvider(providerID int64) ([]ModelBinding, error)` 方法。

- [ ] **Step 4: 给 ModelBindingStore 加 ListByProvider**

在 `internal/catalog/binding.go` 加:

```go
func (s *ModelBindingStore) ListByProvider(providerID int64) ([]ModelBinding, error) {
	rows, err := s.db.Query(
		`SELECT id, model_id, provider_id, upstream_model, priority, weight, enabled, created_at
		 FROM model_bindings WHERE provider_id=? ORDER BY id`, providerID)
	if err != nil {
		return nil, fmt.Errorf("list by provider: %w", err)
	}
	defer rows.Close()
	var out []ModelBinding
	for rows.Next() {
		b, err := scanModelBinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
```

- [ ] **Step 5: 给 catalogFixture 加 bindingsStore()**

在 `internal/catalog/model_test.go` 的 `catalogFixture` struct 加 `bindings *ModelBindingStore` 字段,`newCatalogFixture` 里初始化:

```go
// struct 加字段
type catalogFixture struct {
	db        *sql.DB
	providers *ProviderStore
	models    *ModelStore
	prices    *PriceStore
	bindings  *ModelBindingStore
}

// newCatalogFixture 内初始化
bindings: NewModelBindingStore(d),

// 加方法
func (f *catalogFixture) bindingsStore() *ModelBindingStore {
	return f.bindings
}
```

- [ ] **Step 6: 运行测试验证通过**

Run: `go test ./internal/catalog/ -run TestHealthCache -v`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add internal/catalog/health_cache.go internal/catalog/health_cache_test.go internal/catalog/binding.go internal/catalog/model_test.go
git commit -m "feat(catalog): add HealthCache background precompute"
```

---

### Task 4: Router 改读 HealthCache

**Files:**
- Modify: `internal/catalog/router.go:14-32, 79-101, 111-125`
- Create: `internal/catalog/router_test.go`

**Interfaces:**
- Consumes: `HealthCache.Get(providerID, upstreamModel) string`
- Produces: `NewRouter(providers, healthCache)` 替换旧 `NewRouter(providers, health)`;`BindingHealth` interface 改为读 `HealthCache`。

- [ ] **Step 1: 写失败测试 — 多 binding 健康过滤**

```go
package catalog

import (
	"testing"
)

func TestRouterExcludesUnhealthy(t *testing.T) {
	f := newCatalogFixture(t)
	rs := NewRoutingStats(f.db)
	hc := NewHealthCache(f.bindingsStore(), f.providers, rs)

	p1, _ := f.providers.Create("healthy-prov", "https://x.com", "k", "openai_chat")
	p2, _ := f.providers.Create("sick-prov", "https://y.com", "k", "openai_chat")
	// p2 active 但插入失败日志使其 unhealthy
	insertLog(t, f.db, p2.ID, "gpt-4o", 500, "upstream", 0, 0, 0)
	insertLog(t, f.db, p2.ID, "gpt-4o", 500, "upstream", 0, 0, 0)
	hc.Refresh(time.Now())

	m := Model{ID: 1, RoutingStrategy: RoutingStrategyAuto, AutoMode: AutoModeHealth}
	bindings := []ModelBinding{
		{ID: 1, ModelID: 1, ProviderID: p1.ID, UpstreamModel: "gpt-4o", Priority: 100, Weight: 1, Enabled: true},
		{ID: 2, ModelID: 1, ProviderID: p2.ID, UpstreamModel: "gpt-4o", Priority: 200, Weight: 1, Enabled: true},
	}
	r := NewRouter(f.providers, hc)
	sel, candidates, err := r.Select(m, bindings)
	if err != nil {
		t.Fatalf("Select: %v", err)
	}
	if sel.Provider.ID != p1.ID {
		t.Errorf("expected healthy provider %d, got %d", p1.ID, sel.Provider.ID)
	}
	// candidates 应含全部(healthy 在前)
	if len(candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(candidates))
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run: `go test ./internal/catalog/ -run TestRouterExcludesUnhealthy -v`
Expected: FAIL — `NewRouter` 签名不匹配。

- [ ] **Step 3: 改 Router 读 HealthCache**

在 `internal/catalog/router.go`:

1. 替换 `BindingHealth` interface 为 `HealthCacheReader`:
```go
type HealthCacheReader interface {
	Get(providerID int64, upstreamModel string) string
}
```

2. `Router` struct 字段 `health BindingHealth` → `health HealthCacheReader`。
3. `NewRouter` 签名:
```go
func NewRouter(providers *ProviderStore, health HealthCacheReader) *Router {
	return &Router{providers: providers, health: health}
}
```
4. `activeBindings` 加健康过滤(`unhealthy` 排除,其余保留):
```go
func (r *Router) activeBindings(bindings []ModelBinding) ([]ModelBinding, error) {
	out := make([]ModelBinding, 0, len(bindings))
	for _, b := range bindings {
		if !b.Enabled {
			continue
		}
		provider, err := r.providers.Get(b.ProviderID)
		if err != nil || provider.Status != "active" {
			continue
		}
		if r.health != nil {
			st := r.health.Get(b.ProviderID, b.UpstreamModel)
			if st == StatusUnhealthy {
				continue
			}
		}
		out = append(out, b)
	}
	if len(out) == 0 {
		return nil, errNoAvailableBinding
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}
```
5. `healthSelect` 改用 `Get`:
```go
func (r *Router) healthSelect(bindings []ModelBinding) (ModelBinding, []ModelBinding) {
	healthy := make([]ModelBinding, 0, len(bindings))
	unhealthy := make([]ModelBinding, 0)
	for _, b := range bindings {
		if r.health == nil || r.health.Get(b.ProviderID, b.UpstreamModel) != StatusUnhealthy {
			healthy = append(healthy, b)
		} else {
			unhealthy = append(unhealthy, b)
		}
	}
	if len(healthy) > 0 {
		return priorityRandom(globalRand, healthy), append(healthy, unhealthy...)
	}
	return priorityRandom(globalRand, bindings), bindings
}
```

- [ ] **Step 4: 运行测试验证通过**

Run: `go test ./internal/catalog/ -run TestRouter -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/catalog/router.go internal/catalog/router_test.go
git commit -m "feat(catalog): router reads HealthCache instead of in-memory failure counter"
```

---

### Task 5: 转发路径 — TTFT 计时 + 客户端断开判定

**Files:**
- Modify: `internal/proxy/proxy.go` (requestContext 加 firstByteAt)
- Modify: `internal/proxy/forward.go:104-123` (非流式 firstByteAt + 移除 Record* 调用)
- Modify: `internal/proxy/stream.go:50, 57, 87-90` (流式 firstByteAt + isClientDisconnect)
- Modify: `internal/proxy/stats.go:32-51` (算 ttftMs + 写 ttft_ms 列)

**Interfaces:**
- Produces: `request_logs.ttft_ms` 列被写入;`error_type='client_disconnect'` 用于聚合排除。

- [ ] **Step 1: requestContext 加 firstByteAt**

在 `internal/proxy/proxy.go` 的 `requestContext` struct 内,`start` 字段后加:

```go
	firstByteAt   time.Time // 新增:上游首字节到达时间(算 ttft_ms)
```

- [ ] **Step 2: forward.go — 非流式 firstByteAt + 移除 Record***

在 `internal/proxy/forward.go` 的 `forwardSelected` 函数:

1. 行 104 `upResp, err := p.deps.Client.Do(upReq)` 后,成功路径加:
```go
	rc.firstByteAt = time.Now()
```
(放在 `if err != nil { ... }` 块之后,`defer upResp.Body.Close()` 之前)

2. 移除 4 处 `p.health.Record*` 调用(行 106, 115, 120, 122)。这些行的健康判定已交给 HealthCache(后台从日志聚合),转发不再维护内存计数。

改后 `forwardSelected` 关键段:
```go
	upResp, err := p.deps.Client.Do(upReq)
	if err != nil {
		return p.failoverOrError(r, w, rc, irReq, downstream, selected, candidates,
			ir.NewError("upstream", "upstream_unreachable", "upstream request failed: "+err.Error(), 502))
	}
	defer upResp.Body.Close()
	rc.firstByteAt = time.Now()
	rc.stream = irReq.Stream
	if shouldFailoverStatus(upResp.StatusCode) && rc.model.RoutingStrategy == catalog.RoutingStrategyAuto && rc.model.AutoMode == catalog.AutoModeFailover {
		body, _ := io.ReadAll(upResp.Body)
		upResp.Body.Close()
		return p.failoverOrError(r, w, rc, irReq, downstream, selected, candidates,
			upstreamErrorFromStatus(upResp.StatusCode, upstreamErrorMessage(provider.Protocol, body)))
	}
	rc.provider = &provider
	rc.selected = &selected
```

- [ ] **Step 3: forwardNonStreaming 加 firstByteAt**

在 `forward.go` 的 `forwardNonStreaming` 开头(`body, err := io.ReadAll(upResp.Body)` 前)无改动——`firstByteAt` 已在 `forwardSelected` 设置。确认 `forwardNonStreaming` 不覆盖它。

- [ ] **Step 4: stream.go — 流式 firstByteAt + isClientDisconnect**

在 `internal/proxy/stream.go`:

1. 行 50 `rc.statusCode = 200` 后加(首字节到达保底——实际首次 Scan 时更准):
```go
	rc.firstByteAt = time.Now()
```

2. 行 57 `for scanner.Scan() {` 改为:
```go
	firstByte := true
	for scanner.Scan() {
		if firstByte {
			rc.firstByteAt = time.Now()
			firstByte = false
		}
```

3. 行 87-90 `scanner.Err()` 处改:
```go
	if err := scanner.Err(); err != nil {
		if isClientDisconnect(err) {
			rc.errorType = "client_disconnect"
		} else {
			rc.errorType = "upstream"
		}
		rc.errorMessage = "stream read error: " + err.Error()
	}
```

4. 在文件末尾加 helper:
```go
func isClientDisconnect(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, io.ErrClosedPipe) ||
		strings.Contains(err.Error(), "broken pipe")
}
```

5. import 加 `"context"`, `"errors"`, `"strings"`。

- [ ] **Step 5: stats.go — 算 ttftMs + 写 ttft_ms 列**

在 `internal/proxy/stats.go` 的 `recordStats`:

1. 行 32-35 附近,`durationMs` 计算后加:
```go
	var ttftMs any
	if !rc.firstByteAt.IsZero() && !rc.start.IsZero() {
		ttftMs = rc.firstByteAt.Sub(rc.start).Milliseconds()
	}
```

2. 行 44-51 的 INSERT 加 `ttft_ms` 列:
```go
	_, _ = p.deps.DB.Exec(
		`INSERT INTO request_logs(request_id, user_id, api_key_id, custom_model, provider_id, upstream_model,
		 protocol_in, protocol_out, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
		 cost, duration_ms, status_code, error_type, error_message, stream, ttft_ms)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rc.requestID, userID, apiKeyID, modelName, providerID, upstreamModel,
		rc.downstream, protocolOutName(rc.provider), rc.inputTokens, rc.outputTokens,
		rc.cacheRead, rc.cacheCreation, cost, durationMs, rc.statusCode, errorType, errorMessage, rc.stream, ttftMs)
```

- [ ] **Step 6: 运行现有测试验证不破坏**

Run: `go test ./internal/proxy/ -v`
Expected: PASS(现有测试不应因移除 Record* / 加 ttft_ms 而失败;若有测试断言 INSERT 列数需更新)。

- [ ] **Step 7: 提交**

```bash
git add internal/proxy/proxy.go internal/proxy/forward.go internal/proxy/stream.go internal/proxy/stats.go
git commit -m "feat(proxy): add TTFT timing, client_disconnect detection, drop in-memory health calls"
```

---

### Task 6: 移除 health.go + Proxy 装配 HealthCache

**Files:**
- Delete: `internal/proxy/health.go`
- Modify: `internal/proxy/proxy.go:15-48` (Deps 加 HealthCache,移除 health 字段,getRouter 注入)
- Modify: `cmd/carryapi/main.go:57-88` (装配 routingStats + healthCache)

**Interfaces:**
- Produces: `proxy.Deps.HealthCache catalog.HealthCacheReader`;`Proxy` 不再有 `health *bindingHealth`。

- [ ] **Step 1: 删除 health.go**

```bash
git rm internal/proxy/health.go
```

- [ ] **Step 2: 改 proxy.go**

1. `Deps` struct 加字段:
```go
	HealthCache catalog.HealthCacheReader
```

2. `Proxy` struct 移除 `health *bindingHealth` 字段:
```go
type Proxy struct {
	deps        Deps
	router      *catalog.Router
}
```

3. `NewProxy` 移除 `health := newBindingHealth()` 及赋值:
```go
func NewProxy(deps Deps) *Proxy {
	if deps.Client == nil {
		deps.Client = &http.Client{}
	}
	if deps.Bindings == nil {
		deps.Bindings = catalog.NewModelBindingStore(deps.DB)
	}
	return &Proxy{deps: deps, router: nil}
}
```

4. `getRouter` 注入 `HealthCache`:
```go
func (p *Proxy) getRouter() *catalog.Router {
	if p.router == nil {
		p.router = catalog.NewRouter(p.deps.Providers, p.deps.HealthCache)
	}
	return p.router
}
```

- [ ] **Step 3: 改 main.go 装配**

在 `cmd/carryapi/main.go`:

1. 行 62 `catalogH := catalog.NewHandler(d, catProv, catModel, catPrice)` 后加:
```go
	catBindings := catalog.NewModelBindingStore(d) // 已存在,确认
	routingStats := catalog.NewRoutingStats(d)
	healthCache := catalog.NewHealthCache(catBindings, catProv, routingStats)
```

2. 行 111 `stop` channel 之前加:
```go
	ctx, cancelCtx := context.WithCancel(context.Background())
	go healthCache.Start(ctx)
```

3. shutdown 信号处理块内(`go func() { <-stop ... }`)加:
```go
		cancelCtx()
```

4. `proxy.NewProxy(proxy.Deps{...})` 加 `HealthCache: healthCache`。

5. `catalog.NewHandler` 改签名传 `routingStats`(见 Task 7)。

- [ ] **Step 4: 运行编译 + 测试**

Run: `go build ./... && go test ./internal/proxy/ ./internal/catalog/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/proxy/proxy.go cmd/carryapi/main.go
git commit -m "refactor(proxy): remove health.go, inject HealthCache into Proxy"
```

---

### Task 7: Handler 加 GetRoutingStatus + GetBindingMetrics

**Files:**
- Modify: `internal/catalog/handler.go:13-31` (Handler struct + NewHandler 加 RoutingStats)
- Modify: `internal/catalog/handler.go` 末尾 (加两个 handler)
- Modify: `internal/catalog/handler_test.go` (加两个测试)
- Modify: `internal/server/router.go:114-132` (加两个路由)

**Interfaces:**
- Produces:
  - `GET /api/routing/status` → `Handler.GetRoutingStatus`
  - `GET /api/routing/bindings/{bindingID}/metrics` → `Handler.GetBindingMetrics`
  - `NewHandler(db, providers, models, prices, stats *RoutingStats)` 签名变更

- [ ] **Step 1: 改 NewHandler 签名加 RoutingStats**

在 `internal/catalog/handler.go`:

1. `Handler` struct 加字段 `stats *RoutingStats`。
2. `NewHandler` 改:
```go
func NewHandler(db *sql.DB, providers *ProviderStore, models *ModelStore, prices *PriceStore, stats *RoutingStats) *Handler {
	return &Handler{
		db:        db,
		providers: providers,
		models:    models,
		bindings:  NewModelBindingStore(db),
		prices:    prices,
		prober:    NewProber(nil),
		stats:     stats,
	}
}
```

- [ ] **Step 2: 写失败测试 — GetRoutingStatus**

在 `internal/catalog/handler_test.go` 加:

```go
func TestGetRoutingStatus(t *testing.T) {
	f := newCatalogFixture(t)
	rs := NewRoutingStats(f.db)
	h := NewHandler(f.db, f.providers, f.models, f.prices, rs)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Get("/api/routing/status", h.GetRoutingStatus)

	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	m, _ := f.models.Create("my-gpt4", p.ID, "gpt-4o")
	insertLog(t, f.db, p.ID, "gpt-4o", 200, "none", 1*time.Hour, 100, 50)

	req := httptest.NewRequest("GET", "/api/routing/status", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Models []struct {
			Name     string `json:"name"`
			Bindings []struct {
				ProviderID    int64    `json:"provider_id"`
				UpstreamModel string   `json:"upstream_model"`
				Timeline      []string `json:"timeline"`
				AvgLatencyMs  int64    `json:"avg_latency_ms"`
			} `json:"bindings"`
		} `json:"models"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Models) != 1 || resp.Models[0].Name != "my-gpt4" {
		t.Fatalf("unexpected models: %+v", resp.Models)
	}
	if len(resp.Models[0].Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(resp.Models[0].Bindings))
	}
	b := resp.Models[0].Bindings[0]
	if len(b.Timeline) != 6 {
		t.Errorf("expected 6 timeline buckets, got %d", len(b.Timeline))
	}
	if b.AvgLatencyMs != 100 {
		t.Errorf("expected avg latency 100, got %d", b.AvgLatencyMs)
	}
}
```

- [ ] **Step 3: 运行测试验证失败**

Run: `go test ./internal/catalog/ -run TestGetRoutingStatus -v`
Expected: FAIL — `GetRoutingStatus` 未定义。

- [ ] **Step 4: 实现 GetRoutingStatus**

在 `internal/catalog/handler.go` 末尾加:

```go
// GetRoutingStatus 返回所有 enabled model 的 bindings + 6 格时间轴 + 24h 平均延迟。
func (h *Handler) GetRoutingStatus(w http.ResponseWriter, r *http.Request) {
	models, err := h.models.ListEnabled()
	if err != nil {
		jsonErr(w, 500, "failed to list models")
		return
	}
	now := time.Now()
	type bindingOut struct {
		BindingID     int64    `json:"binding_id"`
		ProviderID   int64    `json:"provider_id"`
		ProviderName string   `json:"provider_name"`
		ProviderStatus string `json:"provider_status"`
		UpstreamModel string   `json:"upstream_model"`
		Priority      int      `json:"priority"`
		Weight        int      `json:"weight"`
		Enabled       bool     `json:"enabled"`
		Timeline      []string `json:"timeline"`
		AvgLatencyMs  int64    `json:"avg_latency_ms"`
		LastRequestAt *string  `json:"last_request_at"`
	}
	type modelOut struct {
		ModelID          int64        `json:"model_id"`
		Name             string       `json:"name"`
		Enabled          bool         `json:"enabled"`
		RoutingStrategy  string       `json:"routing_strategy"`
		AutoMode         string       `json:"auto_mode"`
		Bindings         []bindingOut `json:"bindings"`
	}
	out := struct {
		Models []modelOut `json:"models"`
	}{Models: []modelOut{}}

	for _, m := range models {
		bindings, err := h.bindings.ListByModel(m.ID)
		if err != nil {
			continue
		}
		mo := modelOut{
			ModelID:         m.ID,
			Name:            m.Name,
			Enabled:         m.Enabled,
			RoutingStrategy: m.RoutingStrategy,
			AutoMode:        m.AutoMode,
			Bindings:        []bindingOut{},
		}
		for _, b := range bindings {
			provider, err := h.providers.Get(b.ProviderID)
			if err != nil {
				continue
			}
			tl, err := h.stats.BindingTimeline(b.ProviderID, b.UpstreamModel, now)
			if err != nil {
				continue
			}
			bo := bindingOut{
				BindingID:      b.ID,
				ProviderID:     b.ProviderID,
				ProviderName:   provider.Name,
				ProviderStatus: provider.Status,
				UpstreamModel:  b.UpstreamModel,
				Priority:       b.Priority,
				Weight:         b.Weight,
				Enabled:        b.Enabled,
				AvgLatencyMs:   tl.AvgLatencyMs,
			}
			for _, bk := range tl.Buckets {
				bo.Timeline = append(bo.Timeline, bk.Status)
			}
			if tl.LastRequestAt != nil {
				s := tl.LastRequestAt.Format(time.RFC3339)
				bo.LastRequestAt = &s
			}
			mo.Bindings = append(mo.Bindings, bo)
		}
		out.Models = append(out.Models, mo)
	}
	jsonOut(w, 200, out)
}

// GetBindingMetrics 返回某 binding 的 24h 性能详情。
func (h *Handler) GetBindingMetrics(w http.ResponseWriter, r *http.Request) {
	bindingID, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	binding, err := h.bindings.Get(bindingID)
	if err != nil {
		jsonErr(w, 404, "binding not found")
		return
	}
	provider, err := h.providers.Get(binding.ProviderID)
	if err != nil {
		jsonErr(w, 404, "provider not found")
		return
	}
	m, err := h.stats.BindingMetrics(binding.ProviderID, binding.UpstreamModel, time.Now())
	if err != nil {
		jsonErr(w, 500, "failed to compute metrics")
		return
	}
	jsonOut(w, 200, map[string]any{
		"binding_id":          binding.ID,
		"provider_id":         binding.ProviderID,
		"provider_name":       provider.Name,
		"upstream_model":      binding.UpstreamModel,
		"avg_latency_ms":      m.AvgLatencyMs,
		"avg_ttft_ms":         m.AvgTtftMs,
		"throughput_per_hour": m.ThroughputPerHour,
		"total_requests_24h":  m.TotalRequests24h,
		"success_rate":        m.SuccessRate,
	})
}
```

> 注:`parseIDParam` 用 `chi.URLParam(r, "bindingID")`,需确认 router 注册用 `{bindingID}`。

- [ ] **Step 5: 写测试 — GetBindingMetrics**

在 `handler_test.go` 加:

```go
func TestGetBindingMetrics(t *testing.T) {
	f := newCatalogFixture(t)
	rs := NewRoutingStats(f.db)
	h := NewHandler(f.db, f.providers, f.models, f.prices, rs)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Get("/api/routing/bindings/{bindingID}/metrics", h.GetBindingMetrics)

	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	m, _ := f.models.Create("my-gpt4", p.ID, "gpt-4o")
	b, _ := f.bindings.Create(m.ID, p.ID, "gpt-4o", 100, 1, true)
	insertLog(t, f.db, p.ID, "gpt-4o", 200, "none", 1*time.Hour, 100, 50)

	req := httptest.NewRequest("GET", "/api/routing/bindings/"+strconv.FormatInt(b.ID, 10)+"/metrics", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		AvgLatencyMs int64 `json:"avg_latency_ms"`
		AvgTtftMs    int64 `json:"avg_ttft_ms"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.AvgLatencyMs != 100 {
		t.Errorf("avg latency: expected 100, got %d", resp.AvgLatencyMs)
	}
	if resp.AvgTtftMs != 50 {
		t.Errorf("avg ttft: expected 50, got %d", resp.AvgTtftMs)
	}
}
```

- [ ] **Step 6: 运行测试验证通过**

Run: `go test ./internal/catalog/ -run "TestGetRoutingStatus|TestGetBindingMetrics" -v`
Expected: PASS

- [ ] **Step 7: 注册路由**

在 `internal/server/router.go` 行 132 后(`PutModelPrice` 之后)加:

```go
					r.Get("/api/routing/status", deps.Catalog.GetRoutingStatus)
					r.Get("/api/routing/bindings/{bindingID}/metrics", deps.Catalog.GetBindingMetrics)
```

- [ ] **Step 8: 更新所有 NewHandler 调用点**

`cmd/carryapi/main.go` 行 62:
```go
	catalogH := catalog.NewHandler(d, catProv, catModel, catPrice, routingStats)
```

(确认 `routingStats` 在此前已声明,见 Task 6 Step 3。)

- [ ] **Step 9: 运行编译 + 全部测试**

Run: `go build ./... && go test ./... -v`
Expected: PASS

- [ ] **Step 10: 提交**

```bash
git add internal/catalog/handler.go internal/catalog/handler_test.go internal/server/router.go cmd/carryapi/main.go
git commit -m "feat(catalog): add GET /api/routing/status and bindings metrics API"
```

---

### Task 8: model.go — createInTx/updateInTx 不再双写 binding

**Files:**
- Modify: `internal/catalog/model.go:59-79, 147-184`

**Interfaces:**
- Produces: `Update(id, name, enabled bool)`(移除 providerID/upstreamModel 参数);`Create` 仍保留 providerID/upstreamModel 用于创建首条 binding(草稿导入依赖)。

> **注意**:此任务需谨慎,`createInTx` 被 `CreateDraft` 和 `ImportModels` 复用。这里只改 `updateInTx` 不再碰 binding(因为编辑页不再传 provider/upstream),`createInTx` 保留(创建时仍建首条 binding)。

- [ ] **Step 1: 写失败测试 — Update 不再改 binding**

在 `internal/catalog/model_test.go` 加:

```go
func TestUpdateDoesNotChangeBindings(t *testing.T) {
	f := newCatalogFixture(t)
	p1, _ := f.providers.Create("p1", "https://x.com", "k", "openai_chat")
	p2, _ := f.providers.Create("p2", "https://y.com", "k", "openai_chat")
	m, _ := f.models.Create("m", p1.ID, "gpt-4o")
	// 加第二条 binding
	_, _ = f.bindings.Create(m.ID, p2.ID, "claude", 200, 1, true)

	// Update 改 name + enabled,binding 不受影响
	err := f.models.Update(m.ID, "m-renamed", false)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	bindings, _ := f.bindings.ListByModel(m.ID)
	if len(bindings) != 2 {
		t.Fatalf("expected 2 bindings unchanged, got %d", len(bindings))
	}
	// binding 内容不变
	if bindings[0].UpstreamModel != "gpt-4o" || bindings[1].UpstreamModel != "claude" {
		t.Errorf("bindings changed: %+v", bindings)
	}
}
```

- [ ] **Step 2: 运行测试验证失败**

Run: `go test ./internal/catalog/ -run TestUpdateDoesNotChangeBindings -v`
Expected: FAIL — `Update` 签名不匹配(当前需要 providerID/upstreamModel)。

- [ ] **Step 3: 改 updateInTx + Update 签名**

在 `internal/catalog/model.go`:

1. `Update` 改签名(移除 providerID/upstreamModel):
```go
func (s *ModelStore) Update(id int64, name string, enabled bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.updateInTx(tx, id, name, enabled); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *ModelStore) UpdateInTx(tx *sql.Tx, id int64, name string, enabled bool) error {
	return s.updateInTx(tx, id, name, enabled)
}

func (s *ModelStore) updateInTx(tx *sql.Tx, id int64, name string, enabled bool) error {
	_, err := tx.Exec(
		`UPDATE custom_models SET name=?, enabled=? WHERE id=?`,
		name, enabled, id)
	return err
}
```

- [ ] **Step 4: 更新调用点**

1. `handler.go` 的 `UpdateModel` handler:解析 body 时不再要求 provider_id/upstream_model,只取 name/enabled。找到 `UpdateModel` 函数,改调用为 `h.models.Update(id, name, enabled)`。
2. 全仓 grep `\.Update(` 在 catalog/proxy 测试里,更新所有 `models.Update(id, name, providerID, upstreamModel, enabled)` 调用为新签名。

- [ ] **Step 5: 运行测试验证通过**

Run: `go test ./internal/catalog/ ./internal/proxy/ -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add internal/catalog/model.go internal/catalog/handler.go internal/catalog/model_test.go
git commit -m "refactor(catalog): Update no longer touches bindings (managed via RoutingView)"
```

---

### Task 9: 前端 RoutingView.vue

**Files:**
- Create: `web/src/views/RoutingView.vue`
- Modify: 导航/路由表(加 `/routing` 路由 + 导航项)

**Interfaces:**
- Consumes: `GET /api/routing/status`、`GET /api/routing/bindings/{id}/metrics`、现有 bindings/routing CRUD API、`GET /api/providers`

- [ ] **Step 1: 创建 RoutingView.vue**

创建 `web/src/views/RoutingView.vue`,参照现有 `ModelsView.vue` 的结构(Naive UI 组件)。核心结构:

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NCard, NDataTable, NButton, NTag, NSpace, NModal, NForm, NFormItem, NInput, NInputNumber, NSelect, NSwitch, NPopconfirm, useMessage } from 'naive-ui'

interface Binding {
  binding_id: number
  provider_id: number
  provider_name: string
  provider_status: string
  upstream_model: string
  priority: number
  weight: number
  enabled: boolean
  timeline: string[]
  avg_latency_ms: number
  last_request_at: string | null
}
interface RouteModel {
  model_id: number
  name: string
  enabled: boolean
  routing_strategy: string
  auto_mode: string
  bindings: Binding[]
}
interface Provider { id: number; name: string; status: string }

const message = useMessage()
const models = ref<RouteModel[]>([])
const providers = ref<Provider[]>([])
const expandedRowKeys = ref<number[]>([])
const metricsModal = ref<{ show: boolean; data: any }>({ show: false, data: null })
const editModal = ref<{ show: boolean; binding: any }>({ show: false, binding: null })
const addModal = ref<{ show: boolean; modelId: number }>({ show: false, modelId: 0 })

const statusColor: Record<string, string> = {
  healthy: '#52c41a', warning: '#faad14', unhealthy: '#ff4d4f', no_data: '#d9d9d9'
}

async function loadStatus() {
  const res = await fetch('/api/routing/status')
  models.value = (await res.json()).models || []
}
async function loadProviders() {
  const res = await fetch('/api/providers')
  providers.value = (await res.json()) || []
}
async function loadMetrics(bindingId: number) {
  const res = await fetch(`/api/routing/bindings/${bindingId}/metrics`)
  metricsModal.value = { show: true, data: await res.json() }
}
onMounted(() => { loadStatus(); loadProviders() })
</script>
```

模板部分包含:刷新按钮、模型列表(NDataTable,行展开显示 binding 表)、每 binding 的 6 格时间轴(`v-for` 渲染色块 div)、延迟显示、详情按钮、编辑/删除按钮、添加 binding 弹窗、路由策略弹窗。

> 完整 Vue 模板代码较长,实现时参照 `ModelsView.vue` 的弹窗模式(`436-582`)。时间轴渲染:
> ```html
> <div v-for="(st, i) in binding.timeline" :key="i"
>      :style="{ background: statusColor[st], width: '16px', height: '16px', display: 'inline-block', marginRight: '2px' }"
>      :title="`Bucket ${i+1}/6: ${st}`" />
> ```

- [ ] **Step 2: 注册路由**

在 `web/src/router/index.ts`(或等价)加:

```ts
{ path: '/routing', name: 'routing', component: () => import('@/views/RoutingView.vue'), meta: { requiresAuth: true, admin: true } }
```

- [ ] **Step 3: 加导航项**

在 `web/src/layouts/AppLayout.vue`(或导航组件)的菜单项里加"路由配置"→ `/routing`。

- [ ] **Step 4: 构建验证**

Run: `cd web && npm run build`
Expected: 构建成功。

- [ ] **Step 5: 手动验证(如可运行)**

启动后端,访问 `/routing`,确认:列表加载、时间轴渲染、binding CRUD、详情指标。

- [ ] **Step 6: 提交**

```bash
git add web/src/views/RoutingView.vue web/src/router/index.ts web/src/layouts/AppLayout.vue
git commit -m "feat(web): add routing config page with 24h status timeline"
```

---

### Task 10: 移除旧入口 — ModelsView 弹窗 + ModelEditView 字段

**Files:**
- Modify: `web/src/views/ModelsView.vue` (移除"上游"弹窗组件 + "路由"按钮 + 逻辑)
- Modify: `web/src/views/ModelEditView.vue` (移除 provider_id + upstream_model 字段)

- [ ] **Step 1: ModelsView 移除上游/路由弹窗**

在 `web/src/views/ModelsView.vue`:
1. 删除"上游"弹窗模板(约 `54-83` 行的 `<n-modal>` 上游绑定弹窗)。
2. 删除"路由"按钮模板(约 `86-101` 行)。
3. 删除对应 `<script>` 里的状态、函数(`showBindings`/`showRouting`/相关 API 调用)。
4. 模型列表行只保留:名称、启用状态、编辑、删除、从供应商导入。

- [ ] **Step 2: ModelEditView 移除 provider/upstream 字段**

在 `web/src/views/ModelEditView.vue`:
1. 删除 `provider_id`(NSelect)表单项。
2. 删除 `upstream_model`(NInput)表单项。
3. 表单只保留:name、enabled、定价区(currency/input_price/output_price/cache_read_price/cache_write_price)。
4. 提交 `PUT /api/models/{id}` 时只发 `{ name, enabled }`(后端 Task 8 已适配)。

- [ ] **Step 3: 构建验证**

Run: `cd web && npm run build`
Expected: 构建成功。

- [ ] **Step 4: 提交**

```bash
git add web/src/views/ModelsView.vue web/src/views/ModelEditView.vue
git commit -m "refactor(web): remove legacy binding/routing modals from ModelsView and provider fields from ModelEditView"
```

---

### Task 11: 端到端验证 + 收尾

**Files:** 无新文件,全量测试 + 文档同步。

- [ ] **Step 1: 全量后端测试**

Run: `go test ./... -v`
Expected: PASS

- [ ] **Step 2: 前端构建**

Run: `cd web && npm run build`
Expected: 构建成功。

- [ ] **Step 3: 启动服务手动验证**

启动后端,登录 admin,访问 `/routing`:
1. 看到模型列表,展开看 binding + 6 格时间轴。
2. 添加/编辑/删除 binding 生效。
3. 切路由策略生效。
4. 点详情看平均时延/TTFT/吞吐量。
5. 发几个代理请求,刷新看时间轴色块更新。

- [ ] **Step 4: 同步 MANUAL.md / README(如涉及路由功能描述)**

检查 `MANUAL.md` / `README.md` 是否有路由配置/健康状态相关描述需更新(按 [[keep-manual-in-sync-on-feature-changes]] 规则)。

- [ ] **Step 5: 最终提交**

```bash
git add -A
git commit -m "feat: routing config page with log-based health and TTFT metrics"
```

---

## Self-Review

**1. Spec coverage:**
- ✅ 独立路由配置页(模型为主视图)→ Task 9
- ✅ 按 binding 粒度状态 → Task 2/4
- ✅ 基于 request_logs 日志成功率聚合 → Task 2
- ✅ 6 格 × 4h 时间轴 → Task 2
- ✅ 状态映射(≥95 绿/80-95 黄/<80 红/无数据灰)→ Task 2
- ✅ 客户端断开不计入失败 → Task 5 (isClientDisconnect)
- ✅ TTFT → Task 1(migration)+ Task 5(计时)+ Task 2(统计)
- ✅ 详情页性能指标 → Task 7 (GetBindingMetrics)
- ✅ Router 改读 HealthCache → Task 4
- ✅ 后台预算缓存(每 1 分钟)→ Task 3
- ✅ 废弃 health.go → Task 6
- ✅ 移除旧入口 → Task 10
- ✅ model.go 不再双写 binding → Task 8

**2. Placeholder scan:** Task 9 前端 Vue 模板代码未完整展开(注明"参照 ModelsView.vue 弹窗模式"),因前端模板冗长且应参照现有代码风格。这是合理的——实现时需阅读现有 `ModelsView.vue` 匹配风格。其余任务均有完整代码。

**3. Type consistency:**
- `NewHandler` 签名:Task 7 定义 `(db, providers, models, prices, stats)`,Task 6/8 调用一致 ✅
- `NewRouter` 签名:Task 4 定义 `(providers, HealthCacheReader)`,Task 6 调用 `NewRouter(p.deps.Providers, p.deps.HealthCache)` ✅
- `Model.Update` 签名:Task 8 定义 `(id, name, enabled)`,Task 8 Step 4 更新调用点 ✅
- `HealthCache.Get` 返回 `string`,Task 4 `activeBindings` 比较 `StatusUnhealthy` 常量 ✅
- `RoutingStats.BindingTimeline/Metrics/BindingHealth` 签名在 Task 2 定义,Task 3/4/7 调用一致 ✅
