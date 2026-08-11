package ir

import (
	"encoding/json"
	"testing"
)

func TestRequestJSONRoundTrip(t *testing.T) {
	// IR 类型本身要可 JSON 序列化(流式响应编码需要组装 Response JSON)
	req := Request{
		Model: "gpt-4o",
		Messages: []Message{
			{Role: "user", Content: []ContentPart{{Type: "text", Text: "hi"}}},
			{Role: "assistant", ToolCalls: []ToolCall{{ID: "call_1", Type: "function", Name: "get_weather", Arguments: `{"city":"beijing"}`}}},
		},
		System: []ContentPart{{Type: "text", Text: "be helpful"}},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back Request
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(back.Messages) != 2 || back.Messages[0].Content[0].Text != "hi" {
		t.Errorf("round trip mismatch: %+v", back)
	}
	if len(back.Messages[1].ToolCalls) != 1 || back.Messages[1].ToolCalls[0].Name != "get_weather" {
		t.Errorf("tool calls round trip mismatch: %+v", back.Messages[1].ToolCalls)
	}
	if len(back.System) != 1 || back.System[0].Text != "be helpful" {
		t.Errorf("system round trip mismatch: %+v", back.System)
	}
}

func TestToolCallRoundTrip(t *testing.T) {
	tc := ToolCall{ID: "call_1", Type: "function", Name: "get_weather", Arguments: `{"city":"beijing"}`}
	data, _ := json.Marshal(tc)
	var back ToolCall
	json.Unmarshal(data, &back)
	if back.Name != "get_weather" || back.Arguments != `{"city":"beijing"}` {
		t.Errorf("tool call round trip: %+v", back)
	}
}

func TestEventRoundTrip(t *testing.T) {
	ev := Event{Type: EventUsage, Usage: &Usage{InputTokens: 10, OutputTokens: 5, CacheReadTokens: 3}}
	data, _ := json.Marshal(ev)
	var back Event
	json.Unmarshal(data, &back)
	if back.Usage == nil || back.Usage.InputTokens != 10 || back.Usage.CacheReadTokens != 3 {
		t.Errorf("usage round trip: %+v", back.Usage)
	}
}

func TestContentPartToolUseRoundTrip(t *testing.T) {
	cp := ContentPart{Type: "tool_use", ToolUseID: "tu_1", ToolName: "get_weather", ToolInput: json.RawMessage(`{"city":"beijing"}`)}
	data, _ := json.Marshal(cp)
	var back ContentPart
	json.Unmarshal(data, &back)
	if back.ToolUseID != "tu_1" || string(back.ToolInput) != `{"city":"beijing"}` {
		t.Errorf("content part round trip: %+v", back)
	}
}
