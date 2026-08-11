package ir

import (
	"encoding/json"
	"fmt"
)

// ---- 请求:下游 Responses JSON -> IR ----

type responsesRequestRaw struct {
	Model         string             `json:"model"`
	Instructions  string             `json:"instructions"`
	Input         json.RawMessage    `json:"input"`
	Tools         []responsesToolRaw `json:"tools"`
	ToolChoice    json.RawMessage    `json:"tool_choice"`
	Stream        bool               `json:"stream"`
	Temperature   *float64           `json:"temperature"`
	TopP          *float64           `json:"top_p"`
	MaxOutputToks *int               `json:"max_output_tokens"`
}

type responsesToolRaw struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type responsesInputItem struct {
	Type      string          `json:"type"` // message / function_call / function_call_output
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	CallID    string          `json:"call_id"`
	Name      string          `json:"name"`
	Arguments string          `json:"arguments"`
	Output    string          `json:"output"`
}

func DecodeResponsesRequest(body []byte) (*Request, error) {
	var raw responsesRequestRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode responses request: %w", err)
	}
	r := &Request{
		Model: raw.Model, Stream: raw.Stream,
		Temperature: raw.Temperature, TopP: raw.TopP, MaxTokens: raw.MaxOutputToks,
	}
	if raw.Instructions != "" {
		r.System = append(r.System, ContentPart{Type: "text", Text: raw.Instructions})
	}
	msgs, err := decodeResponsesInput(raw.Input)
	if err != nil {
		return nil, err
	}
	r.Messages = msgs
	for _, t := range raw.Tools {
		r.Tools = append(r.Tools, Tool{Type: t.Type, Name: t.Name, Description: t.Description, Parameters: t.Parameters})
	}
	if len(raw.ToolChoice) > 0 {
		tc, err := decodeResponsesToolChoice(raw.ToolChoice)
		if err != nil {
			return nil, err
		}
		r.ToolChoice = tc
	}
	return r, nil
}

func decodeResponsesInput(raw json.RawMessage) ([]Message, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	// 纯字符串 -> 单条 user 消息
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []Message{{Role: "user", Content: []ContentPart{{Type: "text", Text: s}}}}, nil
	}
	var items []responsesInputItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("decode responses input: %w", err)
	}
	var msgs []Message
	for _, it := range items {
		switch it.Type {
		case "function_call":
			// 属于 assistant 的工具调用;若上一条不是 assistant 则开一条
			if len(msgs) == 0 || msgs[len(msgs)-1].Role != "assistant" {
				msgs = append(msgs, Message{Role: "assistant"})
			}
			last := &msgs[len(msgs)-1]
			last.ToolCalls = append(last.ToolCalls, ToolCall{
				ID: it.CallID, Type: "function", Name: it.Name, Arguments: it.Arguments,
			})
		case "function_call_output":
			msgs = append(msgs, Message{
				Role: "tool", ToolCallID: it.CallID,
				Content: []ContentPart{{Type: "text", Text: it.Output}},
			})
		default: // message
			role := it.Role
			if role == "" {
				role = "user"
			}
			content, err := decodeResponsesContent(it.Content)
			if err != nil {
				return nil, err
			}
			msgs = append(msgs, Message{Role: role, Content: content})
		}
	}
	return msgs, nil
}

func decodeResponsesContent(raw json.RawMessage) ([]ContentPart, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []ContentPart{{Type: "text", Text: s}}, nil
	}
	var items []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ImageURL string `json:"image_url"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("decode responses content: %w", err)
	}
	var parts []ContentPart
	for _, it := range items {
		switch it.Type {
		case "input_text", "output_text":
			parts = append(parts, ContentPart{Type: "text", Text: it.Text})
		case "input_image":
			parts = append(parts, ContentPart{Type: "image_url", ImageURL: it.ImageURL})
		default:
			parts = append(parts, ContentPart{Type: it.Type, Text: it.Text})
		}
	}
	return parts, nil
}

func decodeResponsesToolChoice(raw json.RawMessage) (*ToolChoice, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return &ToolChoice{Type: s}, nil
	}
	var obj struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decode responses tool_choice: %w", err)
	}
	return &ToolChoice{Type: obj.Type, Name: obj.Name}, nil
}

// ---- 请求:IR -> 下游 Responses JSON ----

func EncodeResponsesRequest(r *Request) ([]byte, error) {
	out := map[string]any{
		"model":  r.Model,
		"stream": r.Stream,
	}
	if len(r.System) > 0 {
		out["instructions"] = joinText(r.System)
	}
	out["input"] = encodeResponsesInput(r.Messages)
	if len(r.Tools) > 0 {
		var tools []map[string]any
		for _, t := range r.Tools {
			tools = append(tools, map[string]any{
				"type":        "function",
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			})
		}
		out["tools"] = tools
	}
	if r.ToolChoice != nil {
		out["tool_choice"] = encodeResponsesToolChoice(r.ToolChoice)
	}
	if r.Temperature != nil {
		out["temperature"] = *r.Temperature
	}
	if r.TopP != nil {
		out["top_p"] = *r.TopP
	}
	if r.MaxTokens != nil {
		out["max_output_tokens"] = *r.MaxTokens
	}
	return json.Marshal(out)
}

func encodeResponsesInput(msgs []Message) []any {
	var items []any
	for _, m := range msgs {
		switch m.Role {
		case "tool":
			items = append(items, map[string]any{
				"type": "function_call_output", "call_id": m.ToolCallID,
				"output": joinText(m.Content),
			})
		case "assistant":
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					items = append(items, map[string]any{
						"type": "function_call", "call_id": tc.ID,
						"name": tc.Name, "arguments": tc.Arguments,
					})
				}
			}
			if len(m.Content) > 0 {
				items = append(items, map[string]any{
					"type": "message", "role": "assistant",
					"content": encodeResponsesContent(m.Content),
				})
			}
		default:
			items = append(items, map[string]any{
				"type": "message", "role": m.Role,
				"content": encodeResponsesContent(m.Content),
			})
		}
	}
	return items
}

func encodeResponsesContent(parts []ContentPart) any {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 && parts[0].Type == "text" {
		return parts[0].Text
	}
	var arr []map[string]any
	for _, p := range parts {
		if p.Type == "text" {
			arr = append(arr, map[string]any{"type": "input_text", "text": p.Text})
		} else if p.Type == "image_url" {
			arr = append(arr, map[string]any{"type": "input_image", "image_url": p.ImageURL})
		}
	}
	return arr
}

func encodeResponsesToolChoice(tc *ToolChoice) any {
	switch tc.Type {
	case "auto", "none", "required":
		return tc.Type
	default:
		return map[string]any{"type": tc.Type, "name": tc.Name}
	}
}

func joinText(parts []ContentPart) string {
	var s string
	for _, p := range parts {
		if p.Type == "text" {
			if s != "" {
				s += "\n"
			}
			s += p.Text
		}
	}
	return s
}
