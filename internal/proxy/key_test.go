package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newEchoAuthUpstream 回显请求携带的 Authorization,便于断言命中的上游 key。
func newEchoAuthUpstream(t *testing.T, auth *string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"chatcmpl-up","object":"chat.completion","model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
}

func TestProxySelectsUserAffinityKey(t *testing.T) {
	var lastAuth string
	up := newEchoAuthUpstream(t, &lastAuth)
	defer up.Close()
	p, u := newProxyWithProvider(t, up.URL, "sk-key-a", "openai_chat", "my-gpt4", "gpt-4o")
	// 追加第二个 key sk-key-b
	keyB, err := p.deps.Providers.AddKey(1, "sk-key-b", "b")
	if err != nil {
		t.Fatalf("AddKey: %v", err)
	}
	// 预置用户对 keyB 的亲和(模拟该用户此前一直用 keyB,缓存已建立)
	if err := p.deps.Providers.MarkUsed(keyB.ID, u.ID); err != nil {
		t.Fatalf("MarkUsed: %v", err)
	}
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{"model": "my-gpt4", "messages": []map[string]string{{"role": "user", "content": "hi"}}})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if lastAuth != "Bearer sk-key-b" {
		t.Errorf("upstream auth = %q, want Bearer sk-key-b (user affinity)", lastAuth)
	}
	// request_logs 应记录实际命中的上游 key id
	var kid int64
	err = p.deps.DB.QueryRow("SELECT provider_api_key_id FROM request_logs LIMIT 1").Scan(&kid)
	if err != nil || kid != keyB.ID {
		t.Errorf("request_logs.provider_api_key_id = %d (err=%v), want %d", kid, err, keyB.ID)
	}
}

func TestProxyDegradesKeyOn429InsufficientBalance(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"message":"Insufficient balance","type":"insufficient_quota","code":"insufficient_quota"}}`))
	}))
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{"model": "my-gpt4", "messages": []map[string]string{{"role": "user", "content": "hi"}}})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 429 {
		t.Fatalf("code=%d, want 429 propagated", rec.Code)
	}
	// key 应被降级为 cooling_down 并进入冷却
	var status string
	err := p.deps.DB.QueryRow("SELECT status FROM provider_api_keys WHERE id = 1").Scan(&status)
	if err != nil {
		t.Fatalf("query key status: %v", err)
	}
	if status != "cooling_down" {
		t.Errorf("key status = %q, want cooling_down (degraded on insufficient balance)", status)
	}
	// 事件日志应有 degraded
	var ev string
	err = p.deps.DB.QueryRow("SELECT event FROM provider_api_key_events WHERE key_id = 1 ORDER BY id DESC LIMIT 1").Scan(&ev)
	if err != nil || ev != "degraded" {
		t.Errorf("latest key event = %q (err=%v), want degraded", ev, err)
	}
}

func TestProxyRetriesThenDegradesOnTransient500(t *testing.T) {
	calls := 0
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write([]byte(`{"error":{"message":"internal server error"}}`))
	}))
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{"model": "my-gpt4", "messages": []map[string]string{{"role": "user", "content": "hi"}}})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 500 {
		t.Fatalf("code=%d, want 500 propagated", rec.Code)
	}
	// 重试 3 次(共 4 次尝试)后仍失败 -> 降级
	if calls != keyRetryLimit+1 {
		t.Errorf("upstream calls = %d, want %d (1 initial + %d retries)", calls, keyRetryLimit+1, keyRetryLimit)
	}
	var status string
	_ = p.deps.DB.QueryRow("SELECT status FROM provider_api_keys WHERE id = 1").Scan(&status)
	if status != "cooling_down" {
		t.Errorf("key status = %q, want cooling_down after 3 failed retries", status)
	}
}
