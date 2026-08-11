package ir

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestDecodeChatRequestBasic(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "system", "content": "You are helpful."},
			{"role": "user", "content": "Hi there"},
			{"role": "assistant", "content": "Hello!"}
		],
		"temperature": 0.7,
		"max_tokens": 100,
		"stream": true
	}`)
	r, err := DecodeChatRequest(body)
	if err != nil {
		t.Fatalf("DecodeChatRequest: %v", err)
	}
	if r.Model != "gpt-4o" {
		t.Errorf("model = %q", r.Model)
	}
	if len(r.System) != 1 || r.System[0].Text != "You are helpful." {
		t.Errorf("system = %+v", r.System)
	}
	if len(r.Messages) != 2 || r.Messages[0].Role != "user" || r.Messages[1].Role != "assistant" {
		t.Errorf("messages = %+v", r.Messages)
	}
	if r.Temperature == nil || *r.Temperature != 0.7 {
		t.Error("temperature not decoded")
	}
	if r.MaxTokens == nil || *r.MaxTokens != 100 {
		t.Error("max_tokens not decoded")
	}
	if !r.Stream {
		t.Error("stream not decoded")
	}
}

func TestDecodeChatRequestTools(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"messages": [{"role": "user", "content": "weather?"}],
		"tools": [{"type": "function", "function": {"name": "get_weather", "description": "Get weather", "parameters": {"type": "object", "properties": {"city": {"type": "string"}}}}}],
		"tool_choice": {"type": "function", "function": {"name": "get_weather"}}
	}`)
	r, err := DecodeChatRequest(body)
	if err != nil {
		t.Fatalf("DecodeChatRequest: %v", err)
	}
	if len(r.Tools) != 1 || r.Tools[0].Name != "get_weather" || r.Tools[0].Type != "function" {
		t.Fatalf("tools = %+v", r.Tools)
	}
	if r.ToolChoice == nil || r.ToolChoice.Type != "function" || r.ToolChoice.Name != "get_weather" {
		t.Errorf("tool_choice = %+v", r.ToolChoice)
	}
}

func TestDecodeChatRequestToolMessages(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "assistant", "content": null, "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"beijing\"}"}}]},
			{"role": "tool", "tool_call_id": "call_1", "content": "sunny"}
		]
	}`)
	r, err := DecodeChatRequest(body)
	if err != nil {
		t.Fatalf("DecodeChatRequest: %v", err)
	}
	assistant := r.Messages[0]
	if len(assistant.Content) != 0 {
		t.Errorf("assistant content should be empty, got %+v", assistant.Content)
	}
	if len(assistant.ToolCalls) != 1 { // Message.ToolCalls 字段由本任务 Step 2(前置步骤)加到 types.go
		t.Errorf("assistant tool calls missing")
	}
	tool := r.Messages[1]
	if tool.Role != "tool" || tool.ToolCallID != "call_1" || tool.Content[0].Text != "sunny" {
		t.Errorf("tool message = %+v", tool)
	}
}

func TestEncodeChatRequestToolCallTypeDefault(t *testing.T) {
	// assistant ToolCall.Type 为空时,编码应为 "function"(OpenAI 要求)。
	r := &Request{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "assistant", ToolCalls: []ToolCall{
				{ID: "call_1", Name: "get_weather", Arguments: `{"city":"beijing"}`},
			}},
		},
	}
	data, err := EncodeChatRequest(r)
	if err != nil {
		t.Fatalf("EncodeChatRequest: %v", err)
	}
	if !bytes.Contains(data, []byte(`"type":"function"`)) {
		t.Errorf("tool_calls type not defaulted to function: %s", data)
	}
	var out struct {
		Messages []struct {
			Role      string `json:"role"`
			ToolCalls []struct {
				Type string `json:"type"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal encoded: %v", err)
	}
	if out.Messages[0].ToolCalls[0].Type != "function" {
		t.Errorf("tool call type = %q", out.Messages[0].ToolCalls[0].Type)
	}
}
