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
