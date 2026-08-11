package ir

import (
	"bytes"
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

func TestDecodeAnthropicResponse(t *testing.T) {
	body := []byte(`{
		"id": "msg_1",
		"model": "claude-3-5-sonnet-20241022",
		"role": "assistant",
		"content": [
			{"type": "text", "text": "The weather is sunny."},
			{"type": "tool_use", "id": "tu_1", "name": "get_weather", "input": {"city": "beijing"}}
		],
		"stop_reason": "tool_use",
		"usage": {"input_tokens": 10, "output_tokens": 15, "cache_creation_input_tokens": 2, "cache_read_input_tokens": 4}
	}`)
	r, err := DecodeAnthropicResponse(body)
	if err != nil {
		t.Fatalf("DecodeAnthropicResponse: %v", err)
	}
	if r.ID != "msg_1" || r.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("id/model = %q/%q", r.ID, r.Model)
	}
	if len(r.Choices) != 1 {
		t.Fatalf("choices = %d", len(r.Choices))
	}
	ch := r.Choices[0]
	if len(ch.Content) != 1 || ch.Content[0].Text != "The weather is sunny." {
		t.Errorf("content = %+v", ch.Content)
	}
	if len(ch.ToolCalls) != 1 || ch.ToolCalls[0].Name != "get_weather" || ch.ToolCalls[0].Arguments != `{"city":"beijing"}` {
		t.Errorf("tool calls = %+v", ch.ToolCalls)
	}
	if ch.FinishReason != "tool_calls" {
		t.Errorf("finish = %q, want tool_calls", ch.FinishReason)
	}
	if r.Usage.InputTokens != 10 || r.Usage.CacheCreationTokens != 2 || r.Usage.CacheReadTokens != 4 {
		t.Errorf("usage = %+v", r.Usage)
	}
}

func TestEncodeAnthropicResponse(t *testing.T) {
	r := &Response{
		ID: "msg_1", Model: "claude-3-5-sonnet-20241022",
		Choices: []Choice{{
			Role:         "assistant",
			Content:      []ContentPart{{Type: "text", Text: "hi"}},
			ToolCalls:    []ToolCall{{ID: "tu_1", Type: "function", Name: "get_weather", Arguments: `{"city":"beijing"}`}},
			FinishReason: "tool_calls",
		}},
		Usage: Usage{InputTokens: 3, OutputTokens: 2, CacheCreationTokens: 1},
	}
	out, err := EncodeAnthropicResponse(r)
	if err != nil {
		t.Fatalf("EncodeAnthropicResponse: %v", err)
	}
	var m map[string]any
	json.Unmarshal(out, &m)
	if m["stop_reason"] != "tool_use" {
		t.Errorf("stop_reason = %v", m["stop_reason"])
	}
	content := m["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("content = %d blocks, want 2", len(content))
	}
	tu := content[1].(map[string]any)
	if tu["type"] != "tool_use" || tu["name"] != "get_weather" {
		t.Errorf("tool_use block = %+v", tu)
	}
	usage := m["usage"].(map[string]any)
	if usage["cache_creation_input_tokens"] != float64(1) {
		t.Errorf("cache_creation = %v", usage["cache_creation_input_tokens"])
	}
}

func TestAnthropicStreamDecoder(t *testing.T) {
	d := &AnthropicStreamDecoder{}
	// message_start(input_tokens 记录)
	evs, _ := d.DecodeLine([]byte(`{"type":"message_start","message":{"id":"msg_1","usage":{"input_tokens":10,"cache_creation_input_tokens":2,"cache_read_input_tokens":3}}}`))
	if len(evs) != 0 {
		t.Fatalf("message_start should emit nothing, got %+v", evs)
	}
	// content_block_delta text
	evs, _ = d.DecodeLine([]byte(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hel"}}`))
	if len(evs) != 1 || evs[0].Type != EventContentDelta || evs[0].Delta != "Hel" {
		t.Fatalf("text delta: %+v", evs)
	}
	// content_block_delta input_json
	evs, _ = d.DecodeLine([]byte(`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"ci"}}`))
	if len(evs) != 1 || evs[0].Type != EventToolCallDelta {
		t.Fatalf("input_json delta: %+v", evs)
	}
	// message_delta(output_tokens + stop_reason)
	evs, _ = d.DecodeLine([]byte(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`))
	if len(evs) != 0 {
		t.Fatalf("message_delta should emit nothing, got %+v", evs)
	}
	// message_stop -> usage + done
	evs, _ = d.DecodeLine([]byte(`{"type":"message_stop"}`))
	hasDone, hasUsage := false, false
	for _, ev := range evs {
		if ev.Type == EventDone && ev.Finish == "stop" {
			hasDone = true
		}
		if ev.Type == EventUsage && ev.Usage != nil && ev.Usage.InputTokens == 10 && ev.Usage.OutputTokens == 5 && ev.Usage.CacheCreationTokens == 2 {
			hasUsage = true
		}
	}
	if !hasDone || !hasUsage {
		t.Errorf("message_stop events: %+v", evs)
	}
}

func TestAnthropicStreamEncoder(t *testing.T) {
	e := &AnthropicStreamEncoder{}
	lines, _ := e.Encode(Event{Type: EventContentDelta, Delta: "Hel"})
	if len(lines) != 1 || !bytes.Contains(lines[0], []byte(`"text_delta"`)) || !bytes.Contains(lines[0], []byte(`"Hel"`)) {
		t.Errorf("delta line = %q", lines[0])
	}
	lines, _ = e.Encode(Event{Type: EventDone, Finish: "stop", Usage: &Usage{InputTokens: 10, OutputTokens: 5}})
	if len(lines) != 2 {
		t.Fatalf("done should produce 2 lines (message_delta + message_stop), got %d", len(lines))
	}
	if !bytes.Contains(lines[0], []byte(`"message_delta"`)) || !bytes.Contains(lines[0], []byte(`"output_tokens":5`)) {
		t.Errorf("message_delta line = %s", lines[0])
	}
	if !bytes.Contains(lines[1], []byte(`"message_stop"`)) {
		t.Errorf("message_stop line = %s", lines[1])
	}
}
