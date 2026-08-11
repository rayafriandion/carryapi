package ir

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ---- 请求:下游 Chat JSON -> IR ----

type chatRequestRaw struct {
	Model       string           `json:"model"`
	Messages    []chatMessageRaw `json:"messages"`
	Tools       []chatToolRaw    `json:"tools"`
	ToolChoice  json.RawMessage  `json:"tool_choice"`
	Stream      bool             `json:"stream"`
	Temperature *float64         `json:"temperature"`
	TopP        *float64         `json:"top_p"`
	MaxTokens   *int             `json:"max_tokens"`
	MaxCompTok  *int             `json:"max_completion_tokens"`
	Stop        json.RawMessage  `json:"stop"`
}

type chatMessageRaw struct {
	Role       string            `json:"role"`
	Content    json.RawMessage   `json:"content"` // string 或 []ContentPart-like
	ToolCalls  []chatToolCallRaw `json:"tool_calls"`
	ToolCallID string            `json:"tool_call_id"`
	Name       string            `json:"name"`
}

type chatToolCallRaw struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatToolRaw struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

// DecodeChatRequest 把 OpenAI Chat Completions 请求 JSON 解码为 IR Request。
// 处理:messages(含 string/array content、tool_calls、tool 消息)、tools、
// tool_choice、temperature/max_tokens/max_completion_tokens/top_p/stop/stream。
// role=system 的 message 抽出到 System;其余进 Messages。
func DecodeChatRequest(body []byte) (*Request, error) {
	var raw chatRequestRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode chat request: %w", err)
	}
	r := &Request{
		Model:       raw.Model,
		Stream:      raw.Stream,
		Temperature: raw.Temperature,
		TopP:        raw.TopP,
	}
	if raw.MaxCompTok != nil {
		r.MaxTokens = raw.MaxCompTok
	} else {
		r.MaxTokens = raw.MaxTokens
	}
	if len(raw.Stop) > 0 {
		stop, err := decodeStop(raw.Stop)
		if err != nil {
			return nil, err
		}
		r.Stop = stop
	}
	for _, m := range raw.Messages {
		msg := Message{Role: m.Role, ToolCallID: m.ToolCallID, Name: m.Name}
		if m.Role == "system" {
			parts, err := decodeChatContent(m.Content)
			if err != nil {
				return nil, err
			}
			r.System = append(r.System, parts...)
			continue
		}
		parts, err := decodeChatContent(m.Content)
		if err != nil {
			return nil, err
		}
		msg.Content = parts
		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID: tc.ID, Type: tc.Type, Name: tc.Function.Name, Arguments: tc.Function.Arguments,
			})
		}
		r.Messages = append(r.Messages, msg)
	}
	for _, t := range raw.Tools {
		r.Tools = append(r.Tools, Tool{
			Type: t.Type, Name: t.Function.Name,
			Description: t.Function.Description, Parameters: t.Function.Parameters,
		})
	}
	if len(raw.ToolChoice) > 0 {
		tc, err := decodeChatToolChoice(raw.ToolChoice)
		if err != nil {
			return nil, err
		}
		r.ToolChoice = tc
	}
	return r, nil
}

// decodeChatContent 解析 Chat content(string 或 content part 数组)。
func decodeChatContent(raw json.RawMessage) ([]ContentPart, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []ContentPart{{Type: "text", Text: s}}, nil
	}
	var parts []chatContentPartRaw
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, fmt.Errorf("decode content: %w", err)
	}
	out := make([]ContentPart, 0, len(parts))
	for _, p := range parts {
		cp := ContentPart{Type: p.Type, Text: p.Text}
		if p.Type == "image_url" && p.ImageURL != nil {
			cp.ImageURL = p.ImageURL.URL
		}
		out = append(out, cp)
	}
	return out, nil
}

type chatContentPartRaw struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	ImageURL *struct {
		URL string `json:"url"`
	} `json:"image_url"`
}

func decodeChatToolChoice(raw json.RawMessage) (*ToolChoice, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return &ToolChoice{Type: s}, nil
	}
	var obj struct {
		Type     string `json:"type"`
		Function *struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decode tool_choice: %w", err)
	}
	tc := &ToolChoice{Type: obj.Type}
	if obj.Function != nil {
		tc.Name = obj.Function.Name
	}
	return tc, nil
}

func decodeStop(raw json.RawMessage) ([]string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []string{s}, nil
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("decode stop: %w", err)
	}
	return arr, nil
}

// ---- 请求:IR -> 上游 Chat JSON ----

type chatRequestOut struct {
	Model       string           `json:"model"`
	Messages    []chatMessageOut `json:"messages"`
	Tools       []chatToolOut    `json:"tools,omitempty"`
	ToolChoice  json.RawMessage  `json:"tool_choice,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
	Temperature *float64         `json:"temperature,omitempty"`
	TopP        *float64         `json:"top_p,omitempty"`
	MaxTokens   *int             `json:"max_tokens,omitempty"`
	Stop        json.RawMessage  `json:"stop,omitempty"`
}

type chatMessageOut struct {
	Role       string            `json:"role"`
	Content    json.RawMessage   `json:"content,omitempty"`
	ToolCalls  []chatToolCallOut `json:"tool_calls,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
}

type chatToolCallOut struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatToolOut struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

// EncodeChatRequest 把 IR Request 编码为 OpenAI Chat Completions 请求 JSON。
// System 变为首位 system message;工具 arguments 字符串转 JSON 对象;
// ToolChoice{Type:"function",Name} 转 {"type":"function","function":{"name":...}}。
func EncodeChatRequest(r *Request) ([]byte, error) {
	out := chatRequestOut{
		Model: r.Model, Stream: r.Stream,
		Temperature: r.Temperature, TopP: r.TopP, MaxTokens: r.MaxTokens,
	}
	// system 变首位 system message
	if len(r.System) > 0 {
		content, err := encodeChatContent(r.System)
		if err != nil {
			return nil, err
		}
		out.Messages = append(out.Messages, chatMessageOut{Role: "system", Content: content})
	}
	for _, m := range r.Messages {
		content, err := encodeChatContent(m.Content)
		if err != nil {
			return nil, err
		}
		mo := chatMessageOut{Role: m.Role, Content: content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			tcType := tc.Type
			if tcType == "" {
				tcType = "function" // Chat 的 tool_calls 元素 type 恒为 "function"
			}
			fn := chatToolCallOut{ID: tc.ID, Type: tcType}
			fn.Function.Name = tc.Name
			fn.Function.Arguments = tc.Arguments
			mo.ToolCalls = append(mo.ToolCalls, fn)
		}
		out.Messages = append(out.Messages, mo)
	}
	for _, t := range r.Tools {
		to := chatToolOut{Type: t.Type}
		to.Function.Name = t.Name
		to.Function.Description = t.Description
		to.Function.Parameters = t.Parameters
		out.Tools = append(out.Tools, to)
	}
	if r.ToolChoice != nil {
		tc, err := encodeChatToolChoice(r.ToolChoice)
		if err != nil {
			return nil, err
		}
		out.ToolChoice = tc
	}
	if len(r.Stop) == 1 {
		out.Stop, _ = json.Marshal(r.Stop[0])
	} else if len(r.Stop) > 1 {
		out.Stop, _ = json.Marshal(r.Stop)
	}
	return json.Marshal(out)
}

func encodeChatContent(parts []ContentPart) (json.RawMessage, error) {
	if len(parts) == 0 {
		return json.RawMessage("null"), nil
	}
	if len(parts) == 1 && parts[0].Type == "text" {
		return json.Marshal(parts[0].Text)
	}
	arr := make([]map[string]any, 0, len(parts))
	for _, p := range parts {
		item := map[string]any{"type": p.Type}
		switch p.Type {
		case "text":
			item["text"] = p.Text
		case "image_url":
			item["image_url"] = map[string]string{"url": p.ImageURL}
		}
		arr = append(arr, item)
	}
	return json.Marshal(arr)
}

func encodeChatToolChoice(tc *ToolChoice) (json.RawMessage, error) {
	switch tc.Type {
	case "auto", "none", "required":
		return json.Marshal(tc.Type)
	case "function", "tool":
		return json.Marshal(map[string]any{
			"type":     "function",
			"function": map[string]string{"name": tc.Name},
		})
	default:
		return nil, fmt.Errorf("unsupported tool_choice type %q", tc.Type)
	}
}
