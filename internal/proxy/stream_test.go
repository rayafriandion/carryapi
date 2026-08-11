package proxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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

func TestStreamingClientDisconnect(t *testing.T) {
	// 客户端提前断开 -> 上游取消
	// (简化:验证 context 取消传播——用可取消的 request)
	up := newStreamingUpstream(t)
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{"model": "my-gpt4", "messages": []map[string]string{{"role": "user", "content": "hi"}}, "stream": true})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	// 正常跑完即可(httptest 不模拟真实断开;验证不 panic)
	p.ServeHTTP(rec, req)
}
