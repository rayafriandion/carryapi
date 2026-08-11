package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"carryapi/internal/user"
)

// newStreamingUpstream 返回 Chat 流式 SSE。
func newStreamingUpstream(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hel\"}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\"}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":3,\"total_tokens\":8}}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
}

func TestStreamingChat(t *testing.T) {
	up := newStreamingUpstream(t)
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{
		"model":    "my-gpt4",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
		"stream":   true,
	})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	out := rec.Body.String()
	if !bytes.Contains([]byte(out), []byte(`"content":"Hel"`)) || !bytes.Contains([]byte(out), []byte(`"content":"lo"`)) {
		t.Errorf("missing content deltas:\n%s", out)
	}
	if !bytes.Contains([]byte(out), []byte(`"finish_reason":"stop"`)) {
		t.Errorf("missing finish reason:\n%s", out)
	}
	if !bytes.Contains([]byte(out), []byte("data: [DONE]")) {
		t.Errorf("missing [DONE]:\n%s", out)
	}
	// 统计应记录 token
	var input, output int
	p.deps.DB.QueryRow("SELECT input_tokens, output_tokens FROM request_logs").Scan(&input, &output)
	if input != 5 || output != 3 {
		t.Errorf("tokens = %d/%d, want 5/3", input, output)
	}
}

// 单个事件块内 JSON 被拆成多行 data:(重组时直接拼接,不带分隔符,与 ir.SplitSSE 一致)。
func TestStreamingChatMultiLineData(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"co\n"))
		w.Write([]byte("data: ntent\":\"X\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{"model": "my-gpt4", "messages": []map[string]string{{"role": "user", "content": "hi"}}, "stream": true})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"content":"X"`)) {
		t.Errorf("missing multi-line content delta:\n%s", rec.Body.String())
	}
}

// Anthropic 上游:event: + data: 块(流式)。下游 Chat。
func newStreamingAnthropicUpstream(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "sk-anthropic" {
			t.Errorf("anthropic auth = %q", r.Header.Get("x-api-key"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":4,\"cache_creation_input_tokens\":1,\"cache_read_input_tokens\":0}}}\n\n"))
		w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"Hel\"}}\n\n"))
		w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"lo\"}}\n\n"))
		w.Write([]byte("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\n"))
		w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
}

func TestStreamingAnthropicUpstream(t *testing.T) {
	up := newStreamingAnthropicUpstream(t)
	defer up.Close()
	p, u := newProxyWithProvider(t, up.URL, "sk-anthropic", "anthropic", "my-claude", "claude-3-5-sonnet-20241022")
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{
		"model":    "my-claude",
		"messages": []map[string]string{{"role": "user", "content": "salut"}},
		"stream":   true,
	})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	out := rec.Body.String()
	if !bytes.Contains([]byte(out), []byte(`"content":"Hel"`)) || !bytes.Contains([]byte(out), []byte(`"content":"lo"`)) {
		t.Errorf("missing anthropic content deltas:\n%s", out)
	}
	if !bytes.Contains([]byte(out), []byte(`"finish_reason":"stop"`)) {
		t.Errorf("missing finish reason:\n%s", out)
	}
	var input, output, cc int
	p.deps.DB.QueryRow("SELECT input_tokens, output_tokens, cache_creation_tokens FROM request_logs").Scan(&input, &output, &cc)
	if input != 4 || output != 2 {
		t.Errorf("tokens = %d/%d, want 4/2", input, output)
	}
	if cc != 1 {
		t.Errorf("cache_creation = %d, want 1", cc)
	}
}

// Responses 上游:event: response.output_text.delta(流式)。下游 Chat。
func newStreamingResponsesUpstream(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-upstream" {
			t.Errorf("responses auth = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"Hi\"}\n\n"))
		w.Write([]byte("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\" there\"}\n\n"))
		w.Write([]byte("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"r1\",\"usage\":{\"input_tokens\":5,\"output_tokens\":3,\"total_tokens\":8,\"input_tokens_details\":{\"cached_tokens\":2}}}}\n\n"))
	}))
}

func TestStreamingResponsesUpstream(t *testing.T) {
	up := newStreamingResponsesUpstream(t)
	defer up.Close()
	p, u := newProxyWithProvider(t, up.URL, "sk-upstream", "openai_responses", "my-resp", "gpt-4o")
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{
		"model":    "my-resp",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
		"stream":   true,
	})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	out := rec.Body.String()
	if !bytes.Contains([]byte(out), []byte(`"content":"Hi"`)) || !bytes.Contains([]byte(out), []byte(`"content":" there"`)) {
		t.Errorf("missing responses content deltas:\n%s", out)
	}
	if !bytes.Contains([]byte(out), []byte(`"finish_reason":"stop"`)) {
		t.Errorf("missing finish reason:\n%s", out)
	}
	var input, output, cr int
	p.deps.DB.QueryRow("SELECT input_tokens, output_tokens, cache_read_tokens FROM request_logs").Scan(&input, &output, &cr)
	if input != 5 || output != 3 {
		t.Errorf("tokens = %d/%d, want 5/3", input, output)
	}
	if cr != 2 {
		t.Errorf("cache_read = %d, want 2", cr)
	}
}

// CRLF 上游(\\r\\n\\r\\n 分隔)。
func TestStreamingChatCRLF(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"A\"}}]}\r\n\r\n"))
		w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"B\"}}]}\r\n\r\n"))
		w.Write([]byte("data: [DONE]\r\n\r\n"))
	}))
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{"model": "my-gpt4", "messages": []map[string]string{{"role": "user", "content": "hi"}}, "stream": true})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	out := rec.Body.String()
	if !bytes.Contains([]byte(out), []byte(`"content":"A"`)) || !bytes.Contains([]byte(out), []byte(`"content":"B"`)) {
		t.Errorf("missing CRLF content deltas:\n%s", out)
	}
}

// splitSSERecords 单元测试:CRLF 与 LF 分割。
func TestSplitSSERecordsCRLF(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"LF", "a\n\nb\n\n"},
		{"CRLF", "a\r\n\r\nb\r\n\r\n"},
		{"mixed", "a\r\n\r\nb\n\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var tokens []string
			data := []byte(c.in)
			for {
				adv, tok, err := splitSSERecords(data, false)
				if err != nil {
					t.Fatalf("err: %v", err)
				}
				if adv == 0 {
					break
				}
				if len(tok) > 0 {
					tokens = append(tokens, string(tok))
				}
				data = data[adv:]
			}
			if len(tokens) != 2 || tokens[0] != "a" || tokens[1] != "b" {
				t.Errorf("tokens = %#v", tokens)
			}
		})
	}
}

// 流式上游返回 429 -> 状态码透传(而非 502)。
func TestStreamingUpstream429Mapped(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"message":"Rate limit exceeded for this API key"}}`))
	}))
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{"model": "my-gpt4", "messages": []map[string]string{{"role": "user", "content": "hi"}}, "stream": true})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 429 {
		t.Errorf("code = %d, want 429 (streaming upstream status propagated)", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("Rate limit exceeded")) {
		t.Errorf("body = %s, want upstream message", rec.Body.String())
	}
	var errType string
	p.deps.DB.QueryRow("SELECT error_type FROM request_logs").Scan(&errType)
	if errType != "rate_limit" {
		t.Errorf("error_type = %q, want rate_limit", errType)
	}
}

// 流中途 scanner 错误(超长行超过 1MB buffer)-> 记录错误类型,且不累加配额。
func TestStreamingScannerError(t *testing.T) {
	var lim int64 = 1000000
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// 一个有效块
		w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"}}]}\n\n"))
		// 超长行(>1MB)触发 bufio.ErrTooLong
		w.Write(bytes.Repeat([]byte("a"), 2*1024*1024))
		w.Write([]byte("\n\n"))
	}))
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	// 预置配额,验证失败流不累加
	p.deps.Users.SetQuota(user.Quota{Scope: "user", ScopeID: u.ID, Period: "total", LimitTokens: &lim})
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{"model": "my-gpt4", "messages": []map[string]string{{"role": "user", "content": "hi"}}, "stream": true})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	// 头已写 200,但中途失败
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var errType string
	p.deps.DB.QueryRow("SELECT error_type FROM request_logs").Scan(&errType)
	if errType != "upstream" {
		t.Errorf("error_type = %q, want upstream", errType)
	}
	// 失败流不应累加配额
	q, _ := p.deps.Users.GetQuotas("user", u.ID)
	if len(q) == 0 || q[0].UsedTokens != 0 {
		t.Errorf("quota used_tokens = %+v, want 0 (failed stream must not accumulate)", q)
	}
}

// 客户端提前断开:模拟慢上游,取消请求 context,应不 panic。
func TestStreamingClientDisconnect(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hel\"}}]}\n\n"))
		close(started)
		<-release // 阻塞,模拟慢上游
	}))
	defer up.Close()

	ctx, cancel := context.WithCancel(context.Background())
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{"model": "my-gpt4", "messages": []map[string]string{{"role": "user", "content": "hi"}}, "stream": true})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()

	go func() {
		<-started
		cancel() // 客户端断开
	}()

	// 不应 panic(断开可能产生部分数据或错误,均接受)
	p.ServeHTTP(rec, req)
	close(release)
	if strings.TrimSpace(rec.Body.String()) == "" {
		t.Log("client disconnected before any output (acceptable)")
	}
}
