package ir

import (
	"encoding/json"
	"testing"
)

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
