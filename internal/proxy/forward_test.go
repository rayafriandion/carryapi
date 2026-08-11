package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"carryapi/internal/apikey"
	"carryapi/internal/catalog"
	"carryapi/internal/db"
	"carryapi/internal/user"
)

// newUpstreamServer 返回固定 Chat 响应。
func newUpstreamServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-upstream" {
			t.Errorf("upstream auth = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"chatcmpl-up","object":"chat.completion","model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"Hello from upstream"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`))
	}))
}

// newProxyWithProvider 建一个完整 proxy(内存 db + 用户/密钥 + provider/model/price),
// 供 openai 与 anthropic 上游场景复用。
func newProxyWithProvider(t *testing.T, upstreamURL, apiKey, protocol, modelName, upstreamModel string) (*Proxy, *user.User) {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	c := mustCipher(t)
	us := user.New(d, c)
	ks := apikey.New(d)
	ps := catalog.NewProviderStore(d, c)
	ms := catalog.NewModelStore(d)
	pr := catalog.NewPriceStore(d)
	p := NewProxy(Deps{DB: d, Keys: ks, Users: us, Models: ms, Providers: ps, Prices: pr})
	u, err := us.Create("proxy@x.com", "hash", "user")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	prov, err := ps.Create("Mock", upstreamURL, apiKey, protocol)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	m, err := ms.Create(modelName, prov.ID, upstreamModel)
	if err != nil {
		t.Fatalf("create model: %v", err)
	}
	pr.Set(m.ID, 5.0, 15.0, nil, nil)
	return p, &u
}

func newProxyWithUpstream(t *testing.T, upstreamURL string) (*Proxy, *user.User) {
	return newProxyWithProvider(t, upstreamURL, "sk-upstream", "openai_chat", "my-gpt4", "gpt-4o")
}

func newRateLimitUpstream(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"message":"Rate limit exceeded for this API key","type":"rate_limit_error","code":"rate_limit_exceeded"}}`))
	}))
}

func TestNonStreamingUpstream429Mapped(t *testing.T) {
	up := newRateLimitUpstream(t)
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{"model": "my-gpt4", "messages": []map[string]string{{"role": "user", "content": "hi"}}})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 429 {
		t.Errorf("code = %d, want 429 (upstream status propagated)", rec.Code)
	}
	// 错误体应含上游 message
	if !bytes.Contains(rec.Body.Bytes(), []byte("Rate limit exceeded")) {
		t.Errorf("body = %s, want upstream message", rec.Body.String())
	}
	// 日志 error_type=rate_limit
	var errType string
	p.deps.DB.QueryRow("SELECT error_type FROM request_logs").Scan(&errType)
	if errType != "rate_limit" {
		t.Errorf("error_type = %q, want rate_limit", errType)
	}
}

func TestNonStreamingChat(t *testing.T) {
	up := newUpstreamServer(t)
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")

	body, _ := json.Marshal(map[string]any{
		"model":    "my-gpt4",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Error("missing X-Request-Id header")
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	choices := resp["choices"].([]any)
	msg := choices[0].(map[string]any)["message"].(map[string]any)
	if msg["content"] != "Hello from upstream" {
		t.Errorf("content = %v", msg["content"])
	}
	// 统计已写入 request_logs
	var count int
	p.deps.DB.QueryRow("SELECT COUNT(*) FROM request_logs").Scan(&count)
	if count != 1 {
		t.Errorf("request_logs = %d, want 1", count)
	}
}

func TestNonStreamingAuthFailure(t *testing.T) {
	up := newUpstreamServer(t)
	defer up.Close()
	p, _ := newProxyWithUpstream(t, up.URL)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("code=%d, want 401", rec.Code)
	}
}

func TestNonStreamingModelNotFound(t *testing.T) {
	up := newUpstreamServer(t)
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{"model": "nope", "messages": []map[string]string{{"role": "user", "content": "hi"}}})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Errorf("code=%d, want 404", rec.Code)
	}
}

// anthropic 上游
func newAnthropicUpstream(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "sk-anthropic" {
			t.Errorf("anthropic auth = %q", r.Header.Get("x-api-key"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg_up","type":"message","role":"assistant","model":"claude-3-5-sonnet-20241022","content":[{"type":"text","text":"Bonjour"}],"stop_reason":"end_turn","usage":{"input_tokens":4,"output_tokens":2,"cache_creation_input_tokens":1,"cache_read_input_tokens":0}}`))
	}))
}

func newProxyWithUpstreamAnthropic(t *testing.T, upstreamURL string) (*Proxy, *user.User) {
	return newProxyWithProvider(t, upstreamURL, "sk-anthropic", "anthropic", "my-claude", "claude-3-5-sonnet-20241022")
}

func TestCrossProtocolChatToAnthropic(t *testing.T) {
	up := newAnthropicUpstream(t)
	defer up.Close()
	p, u := newProxyWithUpstreamAnthropic(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	// 客户端用 Chat 协议调用,上游是 Anthropic
	body, _ := json.Marshal(map[string]any{
		"model":    "my-claude",
		"messages": []map[string]string{{"role": "user", "content": "salut"}},
	})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	// 下游 Chat 格式
	choices := resp["choices"].([]any)
	msg := choices[0].(map[string]any)["message"].(map[string]any)
	if msg["content"] != "Bonjour" {
		t.Errorf("content = %v", msg["content"])
	}
	// 统计里 cache_creation 应保留
	var cc int
	p.deps.DB.QueryRow("SELECT cache_creation_tokens FROM request_logs").Scan(&cc)
	if cc != 1 {
		t.Errorf("cache_creation = %d, want 1", cc)
	}
}
