package ir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
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
			part := ContentPart{Type: "text", Text: b.Text}
			if b.CacheControl != nil {
				part.CacheControl = &CacheControl{Type: b.CacheControl.Type}
			}
			parts = append(parts, part)
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
			part := ContentPart{Type: "text", Text: b.Text}
			if b.CacheControl != nil {
				part.CacheControl = &CacheControl{Type: b.CacheControl.Type}
			}
			msg.Content = append(msg.Content, part)
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
			// IR 约定:一条 tool_result 块 = 一条 role=tool 消息,
			// 内容为 tool_result ContentPart(保留 is_error,失败的工具执行对下游可检测)。
			msg.Role = "tool"
			if msg.ToolCallID == "" {
				msg.ToolCallID = b.ToolUseID
			}
			msg.Content = append(msg.Content, ContentPart{
				Type: "tool_result", ToolUseID: b.ToolUseID,
				ToolResultContent: content, IsError: b.IsError,
			})
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
					blocks = append(blocks, anthropicTextBlock(p))
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
			// Anthropic Messages API 只接受 user/assistant 角色;
			// role:"tool" 是 IR 内部约定,tool_result 块 + tool_use_id 承载语义。
			msg["role"] = "user"
			msg["content"] = encodeAnthropicToolResults(m)
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

// encodeAnthropicToolResults 编码 role=tool 消息的 content:
// 有 tool_result part 时每个 part 输出为一块(保留 is_error);
// 其余 text part 包进一个 tool_result 块(与旧行为一致,Chat/Responses 解码路径)。
func encodeAnthropicToolResults(m Message) []any {
	var toolBlocks []any
	var textParts []ContentPart
	for _, p := range m.Content {
		switch p.Type {
		case "tool_result":
			toolBlocks = append(toolBlocks, encodeAnthropicToolResultBlock(p))
		case "text":
			textParts = append(textParts, p)
		}
	}
	if len(textParts) > 0 {
		var inner []any
		for _, p := range textParts {
			inner = append(inner, map[string]any{"type": "text", "text": p.Text})
		}
		toolBlocks = append(toolBlocks, map[string]any{
			"type": "tool_result", "tool_use_id": m.ToolCallID, "content": inner,
		})
	}
	return toolBlocks
}

func encodeAnthropicToolResultBlock(p ContentPart) map[string]any {
	var inner []any
	for _, ip := range p.ToolResultContent {
		if ip.Type == "text" {
			inner = append(inner, map[string]any{"type": "text", "text": ip.Text})
		}
	}
	tr := map[string]any{"type": "tool_result", "tool_use_id": p.ToolUseID, "content": inner}
	if p.IsError {
		tr["is_error"] = true
	}
	return tr
}

// anthropicTextBlock 编码 text ContentPart 为 Anthropic 文本块;
// 带 CacheControl 时附上 cache_control(prompt-cache 写计费依赖)。
func anthropicTextBlock(p ContentPart) map[string]any {
	block := map[string]any{"type": "text", "text": p.Text}
	if p.CacheControl != nil {
		block["cache_control"] = map[string]string{"type": p.CacheControl.Type}
	}
	return block
}

func encodeAnthropicSystem(parts []ContentPart) any {
	if len(parts) == 1 && parts[0].Type == "text" && parts[0].CacheControl == nil {
		return parts[0].Text
	}
	var blocks []any
	for _, p := range parts {
		if p.Type == "text" {
			blocks = append(blocks, anthropicTextBlock(p))
		}
	}
	return blocks
}

func encodeAnthropicContent(parts []ContentPart) any {
	var blocks []any
	for _, p := range parts {
		if p.Type == "text" {
			blocks = append(blocks, anthropicTextBlock(p))
		}
	}
	return blocks
}

// ---- 响应:上游 Anthropic JSON -> IR ----

type anthropicResponseRaw struct {
	ID         string                     `json:"id"`
	Model      string                     `json:"model"`
	Content    []anthropicContentBlockRaw `json:"content"`
	StopReason string                     `json:"stop_reason"`
	Usage      struct {
		InputTokens         int `json:"input_tokens"`
		OutputTokens        int `json:"output_tokens"`
		CacheCreationTokens int `json:"cache_creation_input_tokens"`
		CacheReadTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

func DecodeAnthropicResponse(body []byte) (*Response, error) {
	var raw anthropicResponseRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode anthropic response: %w", err)
	}
	r := &Response{ID: raw.ID, Model: raw.Model}
	r.Usage = Usage{
		InputTokens: raw.Usage.InputTokens, OutputTokens: raw.Usage.OutputTokens,
		CacheCreationTokens: raw.Usage.CacheCreationTokens, CacheReadTokens: raw.Usage.CacheReadTokens,
	}
	ch := Choice{Index: 0, Role: "assistant", FinishReason: mapAnthropicStop(raw.StopReason)}
	for _, b := range raw.Content {
		switch b.Type {
		case "text":
			ch.Content = append(ch.Content, ContentPart{Type: "text", Text: b.Text})
		case "tool_use":
			args := string(b.Input)
			if len(b.Input) > 0 && string(b.Input) != "null" {
				var buf bytes.Buffer
				if err := json.Compact(&buf, b.Input); err == nil {
					args = buf.String()
				}
			}
			ch.ToolCalls = append(ch.ToolCalls, ToolCall{
				ID: b.ID, Type: "function", Name: b.Name, Arguments: args,
			})
		}
	}
	r.Choices = append(r.Choices, ch)
	return r, nil
}

func mapAnthropicStop(stop string) string {
	switch stop {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return stop
	}
}

// ---- 响应:IR -> 下游 Anthropic JSON ----

func EncodeAnthropicResponse(r *Response) ([]byte, error) {
	out := map[string]any{
		"id": r.ID, "type": "message", "role": "assistant", "model": r.Model,
		"stop_reason": mapIRStopToAnthropic(r),
	}
	var blocks []any
	for _, ch := range r.Choices {
		for _, p := range ch.Content {
			if p.Type == "text" {
				blocks = append(blocks, map[string]any{"type": "text", "text": p.Text})
			}
		}
		for _, tc := range ch.ToolCalls {
			var input any
			json.Unmarshal([]byte(tc.Arguments), &input)
			if input == nil {
				input = map[string]any{}
			}
			blocks = append(blocks, map[string]any{
				"type": "tool_use", "id": tc.ID, "name": tc.Name, "input": input,
			})
		}
	}
	out["content"] = blocks
	usage := map[string]any{
		"input_tokens": r.Usage.InputTokens, "output_tokens": r.Usage.OutputTokens,
		"cache_creation_input_tokens": r.Usage.CacheCreationTokens,
		"cache_read_input_tokens":     r.Usage.CacheReadTokens,
	}
	out["usage"] = usage
	return json.Marshal(out)
}

func mapIRStopToAnthropic(r *Response) string {
	if len(r.Choices) == 0 {
		return "end_turn"
	}
	switch r.Choices[0].FinishReason {
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	default:
		return "end_turn"
	}
}

// ---- 流式:上游 Anthropic SSE data -> []Event ----

type AnthropicStreamDecoder struct {
	pendingUsage *Usage
	stopReason   string
}

func (d *AnthropicStreamDecoder) DecodeLine(data []byte) ([]Event, error) {
	evType := SSEEventType(data)
	switch evType {
	case "message_start":
		var v struct {
			Message struct {
				Usage struct {
					InputTokens         int `json:"input_tokens"`
					CacheCreationTokens int `json:"cache_creation_input_tokens"`
					CacheReadTokens     int `json:"cache_read_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, fmt.Errorf("decode message_start: %w", err)
		}
		d.pendingUsage = &Usage{
			InputTokens:         v.Message.Usage.InputTokens,
			CacheCreationTokens: v.Message.Usage.CacheCreationTokens,
			CacheReadTokens:     v.Message.Usage.CacheReadTokens,
		}
		return nil, nil
	case "content_block_delta":
		var v struct {
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, fmt.Errorf("decode content_block_delta: %w", err)
		}
		switch v.Delta.Type {
		case "text_delta":
			return []Event{{Type: EventContentDelta, Delta: v.Delta.Text}}, nil
		case "input_json_delta":
			return []Event{{Type: EventToolCallDelta, ToolCall: &ToolCall{Type: "function", Arguments: v.Delta.PartialJSON}}}, nil
		}
		return nil, nil
	case "message_delta":
		var v struct {
			Delta struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, fmt.Errorf("decode message_delta: %w", err)
		}
		d.stopReason = v.Delta.StopReason
		if d.pendingUsage == nil {
			d.pendingUsage = &Usage{}
		}
		d.pendingUsage.OutputTokens = v.Usage.OutputTokens
		return nil, nil
	case "message_stop":
		finish := mapAnthropicStop(d.stopReason)
		// usage 先于 done 发出,便于下游编码器把用量并入终止事件
		evs := []Event{}
		if d.pendingUsage != nil {
			evs = append(evs, Event{Type: EventUsage, Usage: d.pendingUsage})
		}
		evs = append(evs, Event{Type: EventDone, Finish: finish})
		// 重置状态
		d.pendingUsage = nil
		d.stopReason = ""
		return evs, nil
	default:
		return nil, nil
	}
}

// ---- 流式:[]Event -> 下游 Anthropic SSE 行 ----

type AnthropicStreamEncoder struct {
	blockIndex    int
	pendingUsage  *Usage
	toolBlockOpen bool
}

func (e *AnthropicStreamEncoder) Encode(ev Event) ([][]byte, error) {
	switch ev.Type {
	case EventContentDelta:
		// 文本增量开启新的 text 块;若上一个块是 tool 块则关闭。
		e.toolBlockOpen = false
		line := map[string]any{
			"type":  "content_block_delta",
			"index": e.blockIndex,
			"delta": map[string]any{"type": "text_delta", "text": ev.Delta},
		}
		return [][]byte{EncodeSSELine(mustJSON(line))}, nil
	case EventToolCallDelta:
		if ev.ToolCall == nil {
			return nil, fmt.Errorf("tool call delta without ToolCall")
		}
		var lines [][]byte
		if !e.toolBlockOpen {
			// 每个 tool 块开始时先发 content_block_start(带 synthetic id 和名字,
			// Anthropic 客户端靠它拿到工具名;input_json_delta 只带 partial_json)。
			cb := map[string]any{
				"type":        "tool_use",
				"id":          "toolu_" + strconv.Itoa(e.blockIndex+1),
				"name":        ev.ToolCall.Name,
				"input":       map[string]any{},
			}
			lines = append(lines, EncodeSSELine(mustJSON(map[string]any{
				"type":         "content_block_start",
				"index":        e.blockIndex,
				"content_block": cb,
			})))
			e.toolBlockOpen = true
		}
		line := map[string]any{
			"type":  "content_block_delta",
			"index": e.blockIndex,
			"delta": map[string]any{"type": "input_json_delta", "partial_json": ev.ToolCall.Arguments},
		}
		lines = append(lines, EncodeSSELine(mustJSON(line)))
		e.blockIndex++
		return lines, nil
	case EventUsage:
		// 独立 usage 事件:暂存,并入随后的 message_delta
		if ev.Usage != nil {
			e.pendingUsage = ev.Usage
		}
		return nil, nil
	case EventDone:
		stopReason := "end_turn"
		switch ev.Finish {
		case "length":
			stopReason = "max_tokens"
		case "tool_calls":
			stopReason = "tool_use"
		}
		usage := map[string]any{"output_tokens": 0}
		switch {
		case ev.Usage != nil:
			usage = map[string]any{"output_tokens": ev.Usage.OutputTokens}
		case e.pendingUsage != nil:
			usage = map[string]any{"output_tokens": e.pendingUsage.OutputTokens}
		}
		deltaLine := map[string]any{
			"type":  "message_delta",
			"delta": map[string]any{"stop_reason": stopReason, "stop_sequence": nil},
			"usage": usage,
		}
		stopLine := map[string]any{"type": "message_stop"}
		return [][]byte{
			EncodeSSELine(mustJSON(deltaLine)),
			EncodeSSELine(mustJSON(stopLine)),
		}, nil
	}
	return nil, fmt.Errorf("unknown event type %d", ev.Type)
}

func (e *AnthropicStreamEncoder) Reset() {
	e.blockIndex = 0
	e.pendingUsage = nil
	e.toolBlockOpen = false
}
