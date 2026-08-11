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

func TestDecodeChatResponseBasic(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-123",
		"model": "gpt-4o",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": "Hello!"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}
	}`)
	r, err := DecodeChatResponse(body)
	if err != nil {
		t.Fatalf("DecodeChatResponse: %v", err)
	}
	if r.ID != "chatcmpl-123" || r.Model != "gpt-4o" {
		t.Errorf("id/model = %q/%q", r.ID, r.Model)
	}
	if len(r.Choices) != 1 || r.Choices[0].FinishReason != "stop" || r.Choices[0].Content[0].Text != "Hello!" {
		t.Errorf("choices = %+v", r.Choices)
	}
	if r.Usage.InputTokens != 10 || r.Usage.OutputTokens != 20 || r.Usage.TotalTokens != 30 {
		t.Errorf("usage = %+v", r.Usage)
	}
}

func TestDecodeChatResponseToolCall(t *testing.T) {
	body := []byte(`{
		"choices": [{"index": 0, "message": {"role": "assistant", "content": null, "tool_calls": [{"id": "call_9", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"beijing\"}"}}]}, "finish_reason": "tool_calls"}],
		"usage": {"prompt_tokens": 5, "completion_tokens": 8, "total_tokens": 13}
	}`)
	r, _ := DecodeChatResponse(body)
	ch := r.Choices[0]
	if ch.FinishReason != "tool_calls" || len(ch.ToolCalls) != 1 {
		t.Fatalf("choice = %+v", ch)
	}
	tc := ch.ToolCalls[0]
	if tc.Name != "get_weather" || tc.Arguments != `{"city":"beijing"}` {
		t.Errorf("tool call = %+v", tc)
	}
}

func TestDecodeChatResponseCachedTokens(t *testing.T) {
	body := []byte(`{
		"choices": [{"message": {"role": "assistant", "content": "x"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 100, "completion_tokens": 5, "total_tokens": 105, "prompt_tokens_details": {"cached_tokens": 60}}
	}`)
	r, _ := DecodeChatResponse(body)
	if r.Usage.CacheReadTokens != 60 {
		t.Errorf("cache read = %d, want 60", r.Usage.CacheReadTokens)
	}
}

func TestEncodeChatResponseRoundTrip(t *testing.T) {
	r := &Response{
		ID: "chatcmpl-1", Model: "gpt-4o",
		Choices: []Choice{{Index: 0, Role: "assistant", Content: []ContentPart{{Type: "text", Text: "hi"}}, FinishReason: "stop"}},
		Usage:   Usage{InputTokens: 5, OutputTokens: 3, TotalTokens: 8, CacheReadTokens: 2},
	}
	out, err := EncodeChatResponse(r)
	if err != nil {
		t.Fatalf("EncodeChatResponse: %v", err)
	}
	back, err := DecodeChatResponse(out)
	if err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if back.Choices[0].Content[0].Text != "hi" || back.Choices[0].FinishReason != "stop" {
		t.Errorf("round trip choice: %+v", back.Choices[0])
	}
	if back.Usage.CacheReadTokens != 2 {
		t.Errorf("round trip cache: %d", back.Usage.CacheReadTokens)
	}
}

func TestChatStreamDecoder(t *testing.T) {
	d := &ChatStreamDecoder{}
	// content delta
	evs, err := d.DecodeLine([]byte(`{"id":"x","choices":[{"index":0,"delta":{"content":"Hel"},"finish_reason":null}]}`))
	if err != nil || len(evs) != 1 || evs[0].Type != EventContentDelta || evs[0].Delta != "Hel" {
		t.Fatalf("content delta: %+v err %v", evs, err)
	}
	// tool call arguments delta
	evs, _ = d.DecodeLine([]byte(`{"id":"x","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"get_weather","arguments":"{\"city\":"}}]},"finish_reason":null}]}`))
	if len(evs) != 1 || evs[0].Type != EventToolCallDelta {
		t.Fatalf("tool delta: %+v", evs)
	}
	// finish + usage -> 恰好 2 个事件:EventUsage(独立)+ EventDone(不内嵌用量)
	evs, _ = d.DecodeLine([]byte(`{"id":"x","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
	if len(evs) != 2 {
		t.Fatalf("finish+usage should emit exactly 2 events, got %d: %+v", len(evs), evs)
	}
	if evs[0].Type != EventUsage || evs[0].Usage == nil || evs[0].Usage.TotalTokens != 3 {
		t.Errorf("first event should be EventUsage with total=3: %+v", evs[0])
	}
	if evs[1].Type != EventDone || evs[1].Finish != "stop" {
		t.Errorf("second event should be EventDone stop: %+v", evs[1])
	}
	if evs[1].Usage != nil {
		t.Errorf("EventDone must not embed usage (duplicate): %+v", evs[1])
	}
}

func TestChatStreamDecoderStandaloneUsage(t *testing.T) {
	d := &ChatStreamDecoder{}
	// 无 finish_reason 的独立 usage chunk -> 仅 EventUsage
	evs, err := d.DecodeLine([]byte(`{"id":"x","choices":[{"index":0,"delta":{},"finish_reason":null}],"usage":{"prompt_tokens":4,"completion_tokens":5,"total_tokens":9}}`))
	if err != nil {
		t.Fatalf("DecodeLine: %v", err)
	}
	if len(evs) != 1 || evs[0].Type != EventUsage || evs[0].Usage == nil || evs[0].Usage.TotalTokens != 9 {
		t.Errorf("standalone usage events = %+v", evs)
	}
}

func TestChatStreamEncoder(t *testing.T) {
	e := &ChatStreamEncoder{}
	lines, err := e.Encode(Event{Type: EventContentDelta, Delta: "Hel"})
	if err != nil || len(lines) != 1 {
		t.Fatalf("encode delta: %d lines err %v", len(lines), err)
	}
	if !bytes.Contains(lines[0], []byte(`"content":"Hel"`)) {
		t.Errorf("delta line = %s", lines[0])
	}
	// done with usage -> usage chunk + [DONE]
	u := &Usage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3}
	lines, _ = e.Encode(Event{Type: EventDone, Finish: "stop", Usage: u})
	if len(lines) != 2 {
		t.Fatalf("done should produce 2 lines, got %d", len(lines))
	}
	if !bytes.Contains(lines[0], []byte(`"finish_reason":"stop"`)) || !bytes.Contains(lines[0], []byte(`"total_tokens":3`)) {
		t.Errorf("usage chunk = %s", lines[0])
	}
	if string(lines[1]) != "data: [DONE]\n\n" {
		t.Errorf("done line = %q", lines[1])
	}
}

func TestChatStreamEncoderPendingUsageOnDone(t *testing.T) {
	// C2 回归:EventUsage 缓冲,EventDone 的 finish chunk 内嵌缓冲的 total_tokens。
	e := &ChatStreamEncoder{}
	// 独立 EventUsage 不产出任何行
	lines, err := e.Encode(Event{Type: EventUsage, Usage: &Usage{InputTokens: 10, OutputTokens: 20, TotalTokens: 30}})
	if err != nil {
		t.Fatalf("encode usage: %v", err)
	}
	if len(lines) != 0 {
		t.Fatalf("standalone EventUsage should emit nothing, got %d lines", len(lines))
	}
	lines, err = e.Encode(Event{Type: EventDone, Finish: "stop"})
	if err != nil {
		t.Fatalf("encode done: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("done should produce 2 lines, got %d", len(lines))
	}
	if !bytes.Contains(lines[0], []byte(`"finish_reason":"stop"`)) || !bytes.Contains(lines[0], []byte(`"total_tokens":30`)) {
		t.Errorf("finish chunk should carry buffered usage: %s", lines[0])
	}
	if string(lines[1]) != "data: [DONE]\n\n" {
		t.Errorf("done line = %q", lines[1])
	}
	// Reset 清空 pendingUsage
	e.Reset()
	lines, _ = e.Encode(Event{Type: EventDone, Finish: "stop"})
	if bytes.Contains(lines[0], []byte(`"total_tokens"`)) {
		t.Errorf("Reset should clear pendingUsage: %s", lines[0])
	}
}
