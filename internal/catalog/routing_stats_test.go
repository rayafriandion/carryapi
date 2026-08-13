package catalog

import (
	"database/sql"
	"fmt"
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
// created_at 用 SQLite datetime('now', '-N minutes') 生成,与生产 CURRENT_TIMESTAMP
// 存储格式一致("YYYY-MM-DD HH:MM:SS" 文本,空格分隔,UTC),从而真正覆盖 C1 修复。
func insertLog(t *testing.T, d *sql.DB, providerID int64, upstreamModel string, statusCode int, errorType string, age time.Duration, durationMs, ttftMs int64) {
	t.Helper()
	minutes := int(age.Minutes())
	if minutes < 1 {
		minutes = 1
	}
	_, err := d.Exec(fmt.Sprintf(`INSERT INTO request_logs(request_id, user_id, api_key_id, custom_model, provider_id, upstream_model, protocol_in, protocol_out, duration_ms, status_code, error_type, stream, created_at, ttft_ms)
		VALUES(?, NULL, NULL, 'm', ?, ?, 'chat', 'chat', ?, ?, ?, 0, datetime('now', '-%d minutes'), ?)`, minutes),
		fmt.Sprintf("r%d", providerID), providerID, upstreamModel, durationMs, statusCode, errorType, ttftMs)
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

func TestBindingHealthNoData(t *testing.T) {
	rs, _ := newRoutingStatsFixture(t)
	now := time.Now()
	// 无任何日志 → no_data
	h, err := rs.BindingHealth(1, "gpt-4o", now)
	if err != nil {
		t.Fatalf("BindingHealth: %v", err)
	}
	if h != StatusNoData {
		t.Errorf("expected no_data, got %s", h)
	}
}

func TestBindingHealthHealthy(t *testing.T) {
	rs, d := newRoutingStatsFixture(t)
	now := time.Now()
	// 20 成功 + 1 失败 → 20/21 ≈ 95.2% → healthy(≥95%)
	for i := 0; i < 20; i++ {
		insertLog(t, d, 1, "gpt-4o", 200, "none", 1*time.Hour, 100, 50)
	}
	insertLog(t, d, 1, "gpt-4o", 500, "upstream", 1*time.Hour, 0, 0)
	h, err := rs.BindingHealth(1, "gpt-4o", now)
	if err != nil {
		t.Fatalf("BindingHealth: %v", err)
	}
	if h != StatusHealthy {
		t.Errorf("expected healthy (95.2%%), got %s", h)
	}
}

func TestBindingHealthUnhealthy(t *testing.T) {
	rs, d := newRoutingStatsFixture(t)
	now := time.Now()
	// 1 成功 + 4 失败 → 1/5 = 20% → unhealthy(<80%)
	insertLog(t, d, 1, "gpt-4o", 200, "none", 1*time.Hour, 100, 50)
	for i := 0; i < 4; i++ {
		insertLog(t, d, 1, "gpt-4o", 500, "upstream", 1*time.Hour, 0, 0)
	}
	h, err := rs.BindingHealth(1, "gpt-4o", now)
	if err != nil {
		t.Fatalf("BindingHealth: %v", err)
	}
	if h != StatusUnhealthy {
		t.Errorf("expected unhealthy (20%%), got %s", h)
	}
}

func TestBindingTimelineLastRequestAtPopulated(t *testing.T) {
	rs, d := newRoutingStatsFixture(t)
	now := time.Now()
	insertLog(t, d, 1, "gpt-4o", 200, "none", 1*time.Hour, 100, 50)
	tl, err := rs.BindingTimeline(1, "gpt-4o", now)
	if err != nil {
		t.Fatalf("BindingTimeline: %v", err)
	}
	if tl.LastRequestAt == nil {
		t.Fatalf("expected LastRequestAt to be non-nil when logs exist")
	}
	// 最近请求应在最近 2 小时内(插入时 age=1h)
	if age := time.Since(*tl.LastRequestAt); age > 2*time.Hour {
		t.Errorf("LastRequestAt too old: %v ago", age)
	}
}

func TestBindingTimelineLastRequestAtNil(t *testing.T) {
	rs, _ := newRoutingStatsFixture(t)
	now := time.Now()
	// 无日志 → LastRequestAt 应为 nil
	tl, err := rs.BindingTimeline(1, "gpt-4o", now)
	if err != nil {
		t.Fatalf("BindingTimeline: %v", err)
	}
	if tl.LastRequestAt != nil {
		t.Errorf("expected LastRequestAt nil when no logs exist, got %v", *tl.LastRequestAt)
	}
}
