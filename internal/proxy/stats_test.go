package proxy

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"carryapi/internal/catalog"
)

func TestComputeCost(t *testing.T) {
	var cr float64 = 0.5
	var cw float64 = 1.0
	price := &catalog.Price{InputPrice: 5.0, OutputPrice: 15.0, CacheReadPrice: &cr, CacheWritePrice: &cw}
	rc := &requestContext{
		inputTokens: 1000, outputTokens: 2000, cacheRead: 500, cacheCreation: 100,
	}
	// 5*1000/1e6 + 15*2000/1e6 + 0.5*500/1e6 + 1.0*100/1e6
	want := 5.0*1000/1e6 + 15.0*2000/1e6 + 0.5*500/1e6 + 1.0*100/1e6
	got := computeCost(price, rc)
	if got != want {
		t.Errorf("cost = %f, want %f", got, want)
	}
}

func TestComputeCostNoCachePrices(t *testing.T) {
	price := &catalog.Price{InputPrice: 2.0, OutputPrice: 8.0}
	rc := &requestContext{inputTokens: 1000, outputTokens: 500, cacheRead: 100, cacheCreation: 50}
	// 无 cache 价格时:cacheRead 用 input_price,cacheCreation 用 input_price
	want := 2.0*1000/1e6 + 8.0*500/1e6 + 2.0*(100+50)/1e6
	got := computeCost(price, rc)
	if got != want {
		t.Errorf("cost = %f, want %f", got, want)
	}
}

func TestRecordStatsWritesError(t *testing.T) {
	// 无鉴权请求 -> 401 -> request_logs 应有一条 error_type=authentication 的记录
	up := newUpstreamServer(t)
	defer up.Close()
	p, _ := newProxyWithUpstream(t, up.URL)
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
	var count int
	p.deps.DB.QueryRow("SELECT COUNT(*) FROM request_logs").Scan(&count)
	if count != 1 {
		t.Fatalf("request_logs = %d, want 1 (auth failure logged)", count)
	}
	var errType, errMsg string
	p.deps.DB.QueryRow("SELECT error_type, error_message FROM request_logs").Scan(&errType, &errMsg)
	if errType != "authentication" {
		t.Errorf("error_type = %q, want authentication", errType)
	}
	if errMsg == "" {
		t.Error("error_message should not be empty")
	}
}

func TestRecordStatsDuration(t *testing.T) {
	// 非流式成功请求 -> duration_ms >= 0, stream=false
	up := newUpstreamServer(t)
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{"model": "my-gpt4", "messages": []map[string]string{{"role": "user", "content": "hi"}}})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	var durationMs int64
	var stream bool
	if err := p.deps.DB.QueryRow("SELECT duration_ms, stream FROM request_logs").Scan(&durationMs, &stream); err != nil {
		t.Fatalf("request_logs row: %v", err)
	}
	if durationMs < 0 {
		t.Errorf("duration_ms = %d, want >= 0", durationMs)
	}
	if stream {
		t.Error("stream should be false for non-streaming request")
	}
}
