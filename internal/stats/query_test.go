package stats

import (
	"database/sql"
	"fmt"
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
	// 父表行:FOREIGN KEY 约束开启,须先存在。
	// users 1..2, upstream_providers 1..2, api_keys 1..3(分属两用户)。
	parents := []string{
		`INSERT INTO users(id, email, role, status) VALUES(1, 'u1@test.dev', 'user', 'active'),
		 (2, 'u2@test.dev', 'user', 'active')`,
		`INSERT INTO upstream_providers(id, name, base_url, api_key, protocol, status)
		 VALUES(1, 'openai', 'https://openai.example', 'sk-1', 'openai', 'active'),
		 (2, 'anthropic', 'https://anthropic.example', 'sk-2', 'anthropic', 'active')`,
		`INSERT INTO api_keys(id, user_id, key_hash, key_prefix, label, status)
		 VALUES(1, 1, 'h1', 'pk-a', 'key one', 'active'),
		 (2, 1, 'h2', 'pk-b', 'key two', 'active'),
		 (3, 2, 'h3', 'pk-c', 'key three', 'active')`,
	}
	for _, stmt := range parents {
		if _, err := d.Exec(stmt); err != nil {
			t.Fatalf("seed parent: %v", err)
		}
	}
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
	for i, ins := range inserts {
		_, err := d.Exec(
			`INSERT INTO request_logs(request_id, user_id, api_key_id, custom_model, provider_id, upstream_model,
			 protocol_in, protocol_out, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
			 cost, status_code, error_type, created_at)
			 VALUES(?, ?, ?, ?, ?, 'm', 'chat', 'chat', ?, ?, ?, 0, ?, ?, ?, datetime('now'))`,
			fmt.Sprintf("seed-%d", i+1), ins.userID, ins.keyID, ins.model, ins.provID, ins.inTok, ins.outTok, ins.cacheRead, ins.cost, ins.status, ins.errType)
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

func TestQueryTrendDay(t *testing.T) {
	d := newDB(t)
	seedLogs(t, d)
	points, err := QueryTrend(d, QueryParams{Start: time.Now().UTC().Add(-24 * time.Hour), End: time.Now().UTC()}, GranularityDay)
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
	points, err := QueryTrend(d, QueryParams{Start: time.Now().UTC().Add(-2 * time.Hour), End: time.Now().UTC()}, GranularityHour)
	if err != nil {
		t.Fatalf("QueryTrend: %v", err)
	}
	if len(points) == 0 {
		t.Fatal("no hour points")
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

// TestQueryLogsNullUserAndFractionalCost 验证 NULL user 与小数 cost 的扫描。
func TestQueryLogsNullUserAndFractionalCost(t *testing.T) {
	d := newDB(t)
	// 插入一条 NULL user 的日志(模拟认证失败)+ 分数 cost
	_, err := d.Exec(`INSERT INTO request_logs(request_id, user_id, api_key_id, custom_model, protocol_in, protocol_out,
		input_tokens, output_tokens, cost, status_code, error_type, created_at)
		VALUES('null-user', NULL, NULL, 'my-gpt4', 'chat', 'chat', 0, 0, 0.1, 401, 'authentication', datetime('now'))`)
	if err != nil {
		t.Fatalf("insert null-user row: %v", err)
	}
	total, items, err := QueryLogs(d, LogFilter{
		Start: time.Now().Add(-24 * time.Hour).UTC(), End: time.Now().UTC(), Page: 1, PageSize: 50,
	})
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1 (only null-user row in this fresh db)", total)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].UserID != 0 {
		t.Errorf("UserID = %d, want 0 (NULL user)", items[0].UserID)
	}
	if items[0].Cost != 0.1 {
		t.Errorf("Cost = %v, want 0.1 (fractional)", items[0].Cost)
	}
}
