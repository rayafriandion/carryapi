package ir

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ---- 请求:下游 Anthropic JSON -> IR ----

type anthropicRequestRaw struct {
	Model       string                `json:"model"`
	System      json.RawMessage       `json:"system"`
	Messages    []anthropicMessageRaw `json:"messages"`
	Tools       []anthropicToolRaw    `json:"tools"`
	ToolChoice  *anthropicToolChoice  `json:"tool_choice"`
	MaxTokens   int                   `json:"max_tokens"`
	Temperature *float64              `json:"temperature"`
	TopP        *float64              `json:"top_p"`
	StopSeq     []string              `json:"stop_sequences"`
	Stream      bool                  `json:"stream"`
}

type anthropicMessageRaw struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string 或 blocks 数组
}

type anthropicToolRaw struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"input_schema"`
	CacheControl *anthropicCache `json:"cache_control"`
}

type anthropicCache struct {
	Type string `json:"type"`
}

type anthropicToolChoice struct {
	Type string `json:"type"` // auto / any / tool
	Name string `json:"name"`
}

type anthropicContentBlockRaw struct {
	Type         string          `json:"type"`
	Text         string          `json:"text"`
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Input        json.RawMessage `json:"input"`
	ToolUseID    string          `json:"tool_use_id"`
	Content      json.RawMessage `json:"content"` // tool_result 的内容(string 或 blocks)
	IsError      bool            `json:"is_error"`
	CacheControl *anthropicCache `json:"cache_control"`
}

// DecodeAnthropicRequest 把 Anthropic Messages 请求 JSON 解码为 IR Request。
// system(string 或 [{type:text,text,cache_control}]) -> System;
// messages 的 content(string 或 blocks:text/tool_use/tool_result) -> Messages/ToolCalls;
// tools({name,description,input_schema,cache_control}) -> Tools;tool_choice 映射;
// max_tokens 必填;stop_sequences -> Stop。
func DecodeAnthropicRequest(body []byte) (*Request, error) {
	var raw anthropicRequestRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode anthropic request: %w", err)
	}
	r := &Request{
		Model: raw.Model, Stream: raw.Stream,
		Temperature: raw.Temperature, TopP: raw.TopP, Stop: raw.StopSeq,
	}
	if raw.MaxTokens > 0 {
		r.MaxTokens = &raw.MaxTokens
	}
	if len(raw.System) > 0 && string(raw.System) != "null" {
		sys, err := decodeAnthropicSystem(raw.System)
		if err != nil {
			return nil, err
		}
		r.System = sys
	}
	for _, m := range raw.Messages {
		msg, err := decodeAnthropicMessage(m)
		if err != nil {
			return nil, err
		}
		r.Messages = append(r.Messages, msg)
	}
	for _, t := range raw.Tools {
		tool := Tool{Type: "function", Name: t.Name, Description: t.Description, Parameters: t.InputSchema}
		if t.CacheControl != nil {
			tool.CacheControl = &CacheControl{Type: t.CacheControl.Type}
		}
		r.Tools = append(r.Tools, tool)
	}
	if raw.ToolChoice != nil {
		r.ToolChoice = &ToolChoice{Type: raw.ToolChoice.Type, Name: raw.ToolChoice.Name}
	}
	return r, nil
}

func decodeAnthropicSystem(raw json.RawMessage) ([]ContentPart, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []ContentPart{{Type: "text", Text: s}}, nil
	}
	var blocks []anthropicContentBlockRaw
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, fmt.Errorf("decode anthropic system: %w", err)
	}
	var parts []ContentPart
	for _, b := range blocks {
		if b.Type == "text" {
			parts = append(parts, ContentPart{Type: "text", Text: b.Text})
		}
	}
	return parts, nil
}

func decodeAnthropicMessage(m anthropicMessageRaw) (Message, error) {
	msg := Message{Role: m.Role}
	if len(m.Content) == 0 || string(m.Content) == "null" {
		return msg, nil
	}
	var s string
	if err := json.Unmarshal(m.Content, &s); err == nil {
		msg.Content = []ContentPart{{Type: "text", Text: s}}
		return msg, nil
	}
	var blocks []anthropicContentBlockRaw
	if err := json.Unmarshal(m.Content, &blocks); err != nil {
		return Message{}, fmt.Errorf("decode anthropic content: %w", err)
	}
	for _, b := range blocks {
		switch b.Type {
		case "text":
			msg.Content = append(msg.Content, ContentPart{Type: "text", Text: b.Text})
		case "tool_use":
			args := string(b.Input)
			if len(b.Input) > 0 && string(b.Input) != "null" {
				var buf bytes.Buffer
				if err := json.Compact(&buf, b.Input); err == nil {
					args = buf.String()
				}
			}
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID: b.ID, Type: "function", Name: b.Name, Arguments: args,
			})
		case "tool_result":
			content, err := decodeToolResultContent(b.Content)
			if err != nil {
				return Message{}, err
			}
			// IR 统一用 Chat 风格:tool 结果消息 Role=tool + ToolCallID,
			// 内容拍平为 text parts(与 responses.go 的 function_call_output 一致)。
			msg.Role = "tool"
			if msg.ToolCallID == "" {
				msg.ToolCallID = b.ToolUseID
			}
			msg.Content = append(msg.Content, content...)
		}
	}
	return msg, nil
}

func decodeToolResultContent(raw json.RawMessage) ([]ContentPart, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []ContentPart{{Type: "text", Text: s}}, nil
	}
	var blocks []anthropicContentBlockRaw
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil, fmt.Errorf("decode tool_result content: %w", err)
	}
	var parts []ContentPart
	for _, b := range blocks {
		parts = append(parts, ContentPart{Type: "text", Text: b.Text})
	}
	return parts, nil
}

// ---- 请求:IR -> 下游 Anthropic JSON ----

// EncodeAnthropicRequest 把 IR Request 编码为 Anthropic Messages 请求 JSON。
// System -> system(带 cache_control 时输出 blocks 形式);tool_choice 映射;
// CacheHints 映射到 tools/messages 的 cache_control。
func EncodeAnthropicRequest(r *Request) ([]byte, error) {
	out := map[string]any{"model": r.Model, "stream": r.Stream}
	if len(r.System) > 0 {
		out["system"] = encodeAnthropicSystem(r.System)
	}
	var msgs []any
	for _, m := range r.Messages {
		msg := map[string]any{"role": m.Role}
		switch m.Role {
		case "assistant":
			var blocks []any
			for _, p := range m.Content {
				if p.Type == "text" {
					blocks = append(blocks, map[string]any{"type": "text", "text": p.Text})
				}
			}
			for _, tc := range m.ToolCalls {
				var input any
				json.Unmarshal([]byte(tc.Arguments), &input)
				if input == nil {
					input = map[string]any{}
				}
				blocks = append(blocks, map[string]any{
					"type": "tool_use", "id": tc.ID, "name": tc.Name, "input": input,
				})
			}
			msg["content"] = blocks
		case "tool":
			var blocks []any
			for _, p := range m.Content {
				if p.Type == "text" {
					blocks = append(blocks, map[string]any{"type": "text", "text": p.Text})
				}
			}
			msg["content"] = []any{map[string]any{
				"type": "tool_result", "tool_use_id": m.ToolCallID, "content": blocks,
			}}
		default:
			msg["content"] = encodeAnthropicContent(m.Content)
		}
		msgs = append(msgs, msg)
	}
	out["messages"] = msgs
	if len(r.Tools) > 0 {
		var tools []any
		for _, t := range r.Tools {
			tool := map[string]any{
				"name": t.Name, "description": t.Description, "input_schema": t.Parameters,
			}
			if t.CacheControl != nil {
				tool["cache_control"] = map[string]string{"type": t.CacheControl.Type}
			}
			tools = append(tools, tool)
		}
		out["tools"] = tools
	}
	if r.ToolChoice != nil {
		tc := map[string]any{"type": r.ToolChoice.Type}
		if r.ToolChoice.Name != "" {
			tc["name"] = r.ToolChoice.Name
		}
		out["tool_choice"] = tc
	}
	if r.MaxTokens != nil {
		out["max_tokens"] = *r.MaxTokens
	}
	if r.Temperature != nil {
		out["temperature"] = *r.Temperature
	}
	if r.TopP != nil {
		out["top_p"] = *r.TopP
	}
	if len(r.Stop) > 0 {
		out["stop_sequences"] = r.Stop
	}
	return json.Marshal(out)
}

func encodeAnthropicSystem(parts []ContentPart) any {
	if len(parts) == 1 && parts[0].Type == "text" {
		return parts[0].Text
	}
	var blocks []any
	for _, p := range parts {
		if p.Type == "text" {
			blocks = append(blocks, map[string]any{"type": "text", "text": p.Text})
		}
	}
	return blocks
}

func encodeAnthropicContent(parts []ContentPart) any {
	var blocks []any
	for _, p := range parts {
		if p.Type == "text" {
			blocks = append(blocks, map[string]any{"type": "text", "text": p.Text})
		}
	}
	return blocks
}
