package ir

import (
	"encoding/json"
	"testing"
)

func TestDecodeAnthropicRequestBasic(t *testing.T) {
	body := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"system": "You are Claude.",
		"messages": [
			{"role": "user", "content": "Hello"},
			{"role": "assistant", "content": [{"type": "text", "text": "Hi!"}]}
		],
		"max_tokens": 100,
		"temperature": 0.3
	}`)
	r, err := DecodeAnthropicRequest(body)
	if err != nil {
		t.Fatalf("DecodeAnthropicRequest: %v", err)
	}
	if r.Model != "claude-3-5-sonnet-20241022" || len(r.System) != 1 || r.System[0].Text != "You are Claude." {
		t.Errorf("model/system = %q/%+v", r.Model, r.System)
	}
	if len(r.Messages) != 2 || r.Messages[0].Content[0].Text != "Hello" {
		t.Errorf("messages = %+v", r.Messages)
	}
	if r.MaxTokens == nil || *r.MaxTokens != 100 {
		t.Error("max_tokens not decoded")
	}
}

func TestDecodeAnthropicRequestToolUse(t *testing.T) {
	body := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"messages": [
			{"role": "assistant", "content": [{"type": "tool_use", "id": "tu_1", "name": "get_weather", "input": {"city": "beijing"}}]},
			{"role": "user", "content": [{"type": "tool_result", "tool_use_id": "tu_1", "content": "sunny"}]}
		],
		"tools": [{"name": "get_weather", "description": "Get weather", "input_schema": {"type": "object", "properties": {"city": {"type": "string"}}}}],
		"tool_choice": {"type": "tool", "name": "get_weather"},
		"max_tokens": 50
	}`)
	r, err := DecodeAnthropicRequest(body)
	if err != nil {
		t.Fatalf("DecodeAnthropicRequest: %v", err)
	}
	assistant := r.Messages[0]
	if len(assistant.ToolCalls) != 1 || assistant.ToolCalls[0].Name != "get_weather" {
		t.Fatalf("assistant tool calls = %+v", assistant.ToolCalls)
	}
	if assistant.ToolCalls[0].Arguments != `{"city":"beijing"}` {
		t.Errorf("arguments = %q", assistant.ToolCalls[0].Arguments)
	}
	toolMsg := r.Messages[1]
	if toolMsg.Role != "tool" || toolMsg.ToolCallID != "tu_1" || toolMsg.Content[0].Text != "sunny" {
		t.Errorf("tool message = %+v", toolMsg)
	}
	if len(r.Tools) != 1 || r.Tools[0].Name != "get_weather" {
		t.Fatalf("tools = %+v", r.Tools)
	}
	if r.ToolChoice == nil || r.ToolChoice.Type != "tool" || r.ToolChoice.Name != "get_weather" {
		t.Errorf("tool_choice = %+v", r.ToolChoice)
	}
}

func TestEncodeAnthropicRequest(t *testing.T) {
	r := &Request{
		Model:  "claude-3-5-sonnet-20241022",
		System: []ContentPart{{Type: "text", Text: "Be concise."}},
		Messages: []Message{
			{Role: "user", Content: []ContentPart{{Type: "text", Text: "hello"}}},
			{Role: "assistant", ToolCalls: []ToolCall{{ID: "tu_1", Type: "function", Name: "get_weather", Arguments: `{"city":"beijing"}`}}},
			{Role: "tool", ToolCallID: "tu_1", Content: []ContentPart{{Type: "text", Text: "sunny"}}},
		},
		MaxTokens:  intPtr(100),
		ToolChoice: &ToolChoice{Type: "tool", Name: "get_weather"},
	}
	out, err := EncodeAnthropicRequest(r)
	if err != nil {
		t.Fatalf("EncodeAnthropicRequest: %v", err)
	}
	var m map[string]any
	json.Unmarshal(out, &m)
	if m["system"] != "Be concise." {
		t.Errorf("system = %v", m["system"])
	}
	msgs := m["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("messages = %d, want 3", len(msgs))
	}
	// assistant tool_use block
	am := msgs[1].(map[string]any)
	content := am["content"].([]any)
	tu := content[0].(map[string]any)
	if tu["type"] != "tool_use" || tu["name"] != "get_weather" {
		t.Errorf("tool_use block = %+v", tu)
	}
	// tool_result block
	tm := msgs[2].(map[string]any)
	tr := tm["content"].([]any)[0].(map[string]any)
	if tr["type"] != "tool_result" || tr["tool_use_id"] != "tu_1" {
		t.Errorf("tool_result block = %+v", tr)
	}
	// tool_choice
	tc := m["tool_choice"].(map[string]any)
	if tc["type"] != "tool" || tc["name"] != "get_weather" {
		t.Errorf("tool_choice = %+v", tc)
	}
	if m["max_tokens"] != float64(100) {
		t.Errorf("max_tokens = %v", m["max_tokens"])
	}
}
