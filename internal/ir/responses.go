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

// ---- 响应:上游 Responses JSON -> IR ----

type responsesResponseRaw struct {
	ID     string             `json:"id"`
	Model  string             `json:"model"`
	Status string             `json:"status"`
	Output []responsesOutItem `json:"output"`
	Usage  responsesUsageRaw  `json:"usage"`
}

type responsesOutItem struct {
	Type      string          `json:"type"` // message / function_call
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	CallID    string          `json:"call_id"`
	Name      string          `json:"name"`
	Arguments string          `json:"arguments"`
}

type responsesUsageRaw struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
	InputDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"input_tokens_details"`
}

func DecodeResponsesResponse(body []byte) (*Response, error) {
	var raw responsesResponseRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode responses response: %w", err)
	}
	r := &Response{ID: raw.ID, Model: raw.Model}
	r.Usage = Usage{
		InputTokens: raw.Usage.InputTokens, OutputTokens: raw.Usage.OutputTokens,
		TotalTokens: raw.Usage.TotalTokens, CacheReadTokens: raw.Usage.InputDetails.CachedTokens,
	}
	for _, it := range raw.Output {
		switch it.Type {
		case "message":
			content, err := decodeResponsesContent(it.Content)
			if err != nil {
				return nil, err
			}
			ch := Choice{Index: len(r.Choices), Role: "assistant", Content: content}
			r.Choices = append(r.Choices, ch)
		case "function_call":
			tc := ToolCall{ID: it.CallID, Type: "function", Name: it.Name, Arguments: it.Arguments}
			if len(r.Choices) > 0 {
				r.Choices[len(r.Choices)-1].ToolCalls = append(r.Choices[len(r.Choices)-1].ToolCalls, tc)
			} else {
				r.Choices = append(r.Choices, Choice{Role: "assistant", ToolCalls: []ToolCall{tc}})
			}
		}
	}
	return r, nil
}

// ---- 响应:IR -> 下游 Responses JSON ----

func EncodeResponsesResponse(r *Response) ([]byte, error) {
	out := map[string]any{
		"id":     r.ID,
		"object": "response",
		"model":  r.Model,
		"status": "completed",
	}
	var output []any
	for _, ch := range r.Choices {
		if len(ch.Content) > 0 {
			var content []map[string]any
			for _, p := range ch.Content {
				if p.Type == "text" {
					content = append(content, map[string]any{"type": "output_text", "text": p.Text, "annotations": []any{}})
				}
			}
			output = append(output, map[string]any{
				"type": "message", "role": "assistant", "content": content,
			})
		}
		for _, tc := range ch.ToolCalls {
			output = append(output, map[string]any{
				"type": "function_call", "id": "fc_" + tc.ID, "call_id": tc.ID,
				"name": tc.Name, "arguments": tc.Arguments,
			})
		}
	}
	out["output"] = output
	usage := map[string]any{
		"input_tokens": r.Usage.InputTokens, "output_tokens": r.Usage.OutputTokens,
		"total_tokens": r.Usage.TotalTokens,
	}
	if r.Usage.CacheReadTokens > 0 {
		usage["input_tokens_details"] = map[string]int{"cached_tokens": r.Usage.CacheReadTokens}
	}
	out["usage"] = usage
	return json.Marshal(out)
}

// ---- 流式:上游 Responses SSE data -> []Event ----

type ResponsesStreamDecoder struct{}

func (d *ResponsesStreamDecoder) DecodeLine(data []byte) ([]Event, error) {
	evType := SSEEventType(data)
	switch evType {
	case "response.output_text.delta":
		var v struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, fmt.Errorf("decode output_text.delta: %w", err)
		}
		return []Event{{Type: EventContentDelta, Delta: v.Delta}}, nil
	case "response.function_call_arguments.delta":
		var v struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, fmt.Errorf("decode function_call_arguments.delta: %w", err)
		}
		return []Event{{Type: EventToolCallDelta, ToolCall: &ToolCall{Type: "function", Arguments: v.Delta}}}, nil
	case "response.completed":
		var v struct {
			Response responsesResponseRaw `json:"response"`
		}
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, fmt.Errorf("decode response.completed: %w", err)
		}
		usage := &Usage{
			InputTokens: v.Response.Usage.InputTokens, OutputTokens: v.Response.Usage.OutputTokens,
			TotalTokens:     v.Response.Usage.TotalTokens,
			CacheReadTokens: v.Response.Usage.InputDetails.CachedTokens,
		}
		return []Event{
			{Type: EventUsage, Usage: usage},
			{Type: EventDone, Finish: "stop"},
		}, nil
	default:
		return nil, nil
	}
}

// ---- 流式:[]Event -> 下游 Responses SSE 行 ----

type ResponsesStreamEncoder struct {
	toolItemID int
}

func (e *ResponsesStreamEncoder) Encode(ev Event) ([][]byte, error) {
	switch ev.Type {
	case EventContentDelta:
		line := map[string]any{"type": "response.output_text.delta", "delta": ev.Delta}
		return [][]byte{EncodeSSELine(mustJSON(line))}, nil
	case EventToolCallDelta:
		if ev.ToolCall == nil {
			return nil, fmt.Errorf("tool call delta without ToolCall")
		}
		e.toolItemID++
		line := map[string]any{
			"type":    "response.function_call_arguments.delta",
			"item_id": fmt.Sprintf("fc_%d", e.toolItemID),
			"delta":   ev.ToolCall.Arguments,
		}
		return [][]byte{EncodeSSELine(mustJSON(line))}, nil
	case EventDone:
		line := map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id": "resp_stream", "object": "response", "model": "",
				"status": "completed",
				"output": []any{},
				"usage":  usageToResponses(ev.Usage),
			},
		}
		return [][]byte{EncodeSSELine(mustJSON(line))}, nil
	}
	return nil, fmt.Errorf("unknown event type %d", ev.Type)
}

func (e *ResponsesStreamEncoder) Reset() { e.toolItemID = 0 }

func usageToResponses(u *Usage) map[string]any {
	if u == nil {
		return map[string]any{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0}
	}
	return map[string]any{
		"input_tokens": u.InputTokens, "output_tokens": u.OutputTokens, "total_tokens": u.TotalTokens,
	}
}
