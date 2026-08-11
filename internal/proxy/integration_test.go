package proxy

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// 端到端:客户端 Responses 协议 <-> 上游 Chat 协议(非流式)
func TestEndToEndResponsesToChat(t *testing.T) {
	up := newUpstreamServer(t) // Chat 上游
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	// 但 provider protocol 是 openai_chat,客户端用 Responses
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{
		"model":        "my-gpt4",
		"instructions": "Be brief.",
		"input":        "hi",
	})
	req := httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["object"] != "response" {
		t.Errorf("object = %v", resp["object"])
	}
	output := resp["output"].([]any)
	if len(output) == 0 {
		t.Fatal("empty output")
	}
}
