package ir

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestEncodeResponsesRequestToolResultPart(t *testing.T) {
	// I1 回归:tool_result part 经 Responses 边界输出为 function_call_output 纯文本。
	r := &Request{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "tool", ToolCallID: "call_1", Content: []ContentPart{{
				Type: "tool_result", ToolUseID: "call_1", IsError: true,
				ToolResultContent: []ContentPart{{Type: "text", Text: "boom"}},
			}}},
		},
	}
	out, err := EncodeResponsesRequest(r)
	if err != nil {
		t.Fatalf("EncodeResponsesRequest: %v", err)
	}
	var m map[string]any
	json.Unmarshal(out, &m)
	input := m["input"].([]any)
	fco := input[0].(map[string]any)
	if fco["type"] != "function_call_output" || fco["call_id"] != "call_1" || fco["output"] != "boom" {
		t.Errorf("function_call_output = %+v", fco)
	}
}

func TestDecodeResponsesRequestBasic(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"instructions": "Be concise.",
		"input": [
			{"role": "user", "content": [{"type": "input_text", "text": "hello"}]}
		],
		"temperature": 0.5,
		"max_output_tokens": 200
	}`)
	r, err := DecodeResponsesRequest(body)
	if err != nil {
		t.Fatalf("DecodeResponsesRequest: %v", err)
	}
	if r.Model != "gpt-4o" || len(r.System) != 1 || r.System[0].Text != "Be concise." {
		t.Errorf("model/system = %q/%+v", r.Model, r.System)
	}
	if len(r.Messages) != 1 || r.Messages[0].Role != "user" || r.Messages[0].Content[0].Text != "hello" {
		t.Errorf("messages = %+v", r.Messages)
	}
	if r.MaxTokens == nil || *r.MaxTokens != 200 {
		t.Error("max_output_tokens not decoded")
	}
}

func TestDecodeResponsesRequestStringInput(t *testing.T) {
	body := []byte(`{"model":"gpt-4o","input":"just a string"}`)
	r, _ := DecodeResponsesRequest(body)
	if len(r.Messages) != 1 || r.Messages[0].Role != "user" || r.Messages[0].Content[0].Text != "just a string" {
		t.Errorf("string input: %+v", r.Messages)
	}
}

func TestEncodeResponsesRequest(t *testing.T) {
	r := &Request{
		Model:  "gpt-4o",
		System: []ContentPart{{Type: "text", Text: "Be concise."}},
		Messages: []Message{
			{Role: "user", Content: []ContentPart{{Type: "text", Text: "hello"}}},
			{Role: "tool", ToolCallID: "fc_1", Content: []ContentPart{{Type: "text", Text: "sunny"}}},
		},
		MaxTokens: intPtr(100),
	}
	out, err := EncodeResponsesRequest(r)
	if err != nil {
		t.Fatalf("EncodeResponsesRequest: %v", err)
	}
	var m map[string]any
	json.Unmarshal(out, &m)
	if m["instructions"] != "Be concise." {
		t.Errorf("instructions = %v", m["instructions"])
	}
	input := m["input"].([]any)
	if len(input) != 2 {
		t.Fatalf("input = %+v", input)
	}
	toolMsg := input[1].(map[string]any)
	if toolMsg["type"] != "function_call_output" || toolMsg["call_id"] != "fc_1" || toolMsg["output"] != "sunny" {
		t.Errorf("tool msg = %+v", toolMsg)
	}
}

func TestEncodeResponsesRequestToolChoice(t *testing.T) {
	r := &Request{
		Model:      "gpt-4o",
		Messages:   []Message{{Role: "user", Content: []ContentPart{{Type: "text", Text: "hi"}}}},
		ToolChoice: &ToolChoice{Type: "function", Name: "get_weather"},
	}
	out, _ := EncodeResponsesRequest(r)
	var m map[string]any
	json.Unmarshal(out, &m)
	tc := m["tool_choice"].(map[string]any)
	if tc["type"] != "function" || tc["name"] != "get_weather" {
		t.Errorf("tool_choice = %+v", tc)
	}
}

func intPtr(v int) *int { return &v }

func TestDecodeResponsesResponse(t *testing.T) {
	body := []byte(`{
		"id": "resp_1",
		"model": "gpt-4o",
		"output": [
			{"type": "message", "id": "msg_1", "role": "assistant", "content": [{"type": "output_text", "text": "Weather is sunny.", "annotations": []}]},
			{"type": "function_call", "id": "fc_1", "call_id": "call_1", "name": "get_weather", "arguments": "{\"city\":\"beijing\"}"}
		],
		"usage": {"input_tokens": 10, "output_tokens": 15, "total_tokens": 25, "input_tokens_details": {"cached_tokens": 4}}
	}`)
	r, err := DecodeResponsesResponse(body)
	if err != nil {
		t.Fatalf("DecodeResponsesResponse: %v", err)
	}
	if r.ID != "resp_1" || r.Model != "gpt-4o" {
		t.Errorf("id/model = %q/%q", r.ID, r.Model)
	}
	if len(r.Choices) != 1 {
		t.Fatalf("choices = %d, want 1", len(r.Choices))
	}
	ch := r.Choices[0]
	if len(ch.Content) != 1 || ch.Content[0].Text != "Weather is sunny." {
		t.Errorf("content = %+v", ch.Content)
	}
	if len(ch.ToolCalls) != 1 || ch.ToolCalls[0].Name != "get_weather" {
		t.Errorf("tool calls = %+v", ch.ToolCalls)
	}
	if r.Usage.InputTokens != 10 || r.Usage.CacheReadTokens != 4 {
		t.Errorf("usage = %+v", r.Usage)
	}
}

func TestEncodeResponsesResponse(t *testing.T) {
	r := &Response{
		ID: "resp_1", Model: "gpt-4o",
		Choices: []Choice{{
			Role:      "assistant",
			Content:   []ContentPart{{Type: "text", Text: "hi"}},
			ToolCalls: []ToolCall{{ID: "call_1", Type: "function", Name: "get_weather", Arguments: `{"city":"beijing"}`}},
		}},
		Usage: Usage{InputTokens: 3, OutputTokens: 2, TotalTokens: 5},
	}
	out, err := EncodeResponsesResponse(r)
	if err != nil {
		t.Fatalf("EncodeResponsesResponse: %v", err)
	}
	var m map[string]any
	json.Unmarshal(out, &m)
	output := m["output"].([]any)
	if len(output) != 2 {
		t.Fatalf("output = %d items, want 2", len(output))
	}
	fc := output[1].(map[string]any)
	if fc["type"] != "function_call" || fc["name"] != "get_weather" {
		t.Errorf("fc = %+v", fc)
	}
}

func TestResponsesStreamDecoder(t *testing.T) {
	d := &ResponsesStreamDecoder{}
	// content delta
	evs, _ := d.DecodeLine([]byte(`{"type":"response.output_text.delta","delta":"Hel"}`))
	if len(evs) != 1 || evs[0].Type != EventContentDelta || evs[0].Delta != "Hel" {
		t.Fatalf("content delta: %+v", evs)
	}
	// tool args delta
	evs, _ = d.DecodeLine([]byte(`{"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\"ci"}`))
	if len(evs) != 1 || evs[0].Type != EventToolCallDelta {
		t.Fatalf("tool delta: %+v", evs)
	}
	// completed
	evs, _ = d.DecodeLine([]byte(`{"type":"response.completed","response":{"id":"resp_1","model":"gpt-4o","status":"completed","usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}`))
	hasDone, hasUsage := false, false
	for _, ev := range evs {
		if ev.Type == EventDone {
			hasDone = true
		}
		if ev.Type == EventUsage && ev.Usage != nil && ev.Usage.TotalTokens == 3 {
			hasUsage = true
		}
	}
	if !hasDone || !hasUsage {
		t.Errorf("completed events: %+v", evs)
	}
	// 未知事件忽略
	evs, _ = d.DecodeLine([]byte(`{"type":"response.created"}`))
	if len(evs) != 0 {
		t.Errorf("created should be ignored, got %+v", evs)
	}
}

func TestResponsesStreamEncoder(t *testing.T) {
	e := &ResponsesStreamEncoder{}
	lines, _ := e.Encode(Event{Type: EventContentDelta, Delta: "Hel"})
	if len(lines) != 1 || !bytes.Contains(lines[0], []byte(`"delta":"Hel"`)) {
		t.Errorf("delta line = %q", lines[0])
	}
	lines, _ = e.Encode(Event{Type: EventDone, Finish: "completed", Usage: &Usage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3}})
	if len(lines) != 1 {
		t.Fatalf("done should produce 1 line, got %d", len(lines))
	}
	if !bytes.Contains(lines[0], []byte(`"response.completed"`)) || !bytes.Contains(lines[0], []byte(`"total_tokens":3`)) {
		t.Errorf("completed line = %s", lines[0])
	}
}
