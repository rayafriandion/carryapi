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

// ---- 响应:上游 Chat JSON -> IR ----

type chatResponseRaw struct {
	ID      string          `json:"id"`
	Model   string          `json:"model"`
	Choices []chatChoiceRaw `json:"choices"`
	Usage   chatUsageRaw    `json:"usage"`
}

type chatChoiceRaw struct {
	Index        int            `json:"index"`
	Message      chatMessageRaw `json:"message"`
	FinishReason string         `json:"finish_reason"`
}

type chatUsageRaw struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	PromptDetails    struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
}

func DecodeChatResponse(body []byte) (*Response, error) {
	var raw chatResponseRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode chat response: %w", err)
	}
	r := &Response{ID: raw.ID, Model: raw.Model}
	r.Usage = Usage{
		InputTokens: raw.Usage.PromptTokens, OutputTokens: raw.Usage.CompletionTokens,
		TotalTokens: raw.Usage.TotalTokens, CacheReadTokens: raw.Usage.PromptDetails.CachedTokens,
	}
	for _, c := range raw.Choices {
		ch := Choice{Index: c.Index, Role: c.Message.Role, FinishReason: c.FinishReason}
		content, err := decodeChatContent(c.Message.Content)
		if err != nil {
			return nil, err
		}
		ch.Content = content
		for _, tc := range c.Message.ToolCalls {
			ch.ToolCalls = append(ch.ToolCalls, ToolCall{
				ID: tc.ID, Type: tc.Type, Name: tc.Function.Name, Arguments: tc.Function.Arguments,
			})
		}
		r.Choices = append(r.Choices, ch)
	}
	return r, nil
}

// ---- 响应:IR -> 下游 Chat JSON ----

type chatResponseOut struct {
	ID      string          `json:"id"`
	Object  string          `json:"object"`
	Model   string          `json:"model"`
	Choices []chatChoiceOut `json:"choices"`
	Usage   chatUsageOut    `json:"usage"`
}

type chatChoiceOut struct {
	Index        int            `json:"index"`
	Message      chatMessageOut `json:"message"`
	FinishReason string         `json:"finish_reason"`
}

type chatUsageOut struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	PromptDetails    *struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details,omitempty"`
}

func EncodeChatResponse(r *Response) ([]byte, error) {
	out := chatResponseOut{ID: r.ID, Object: "chat.completion", Model: r.Model}
	out.Usage = chatUsageOut{
		PromptTokens: r.Usage.InputTokens, CompletionTokens: r.Usage.OutputTokens,
		TotalTokens: r.Usage.TotalTokens,
	}
	if r.Usage.CacheReadTokens > 0 {
		out.Usage.PromptDetails = &struct {
			CachedTokens int `json:"cached_tokens"`
		}{r.Usage.CacheReadTokens}
	}
	for _, c := range r.Choices {
		co := chatChoiceOut{Index: c.Index, FinishReason: c.FinishReason}
		co.Message.Role = c.Role
		content, err := encodeChatContent(c.Content)
		if err != nil {
			return nil, err
		}
		co.Message.Content = content
		for _, tc := range c.ToolCalls {
			cto := chatToolCallOut{ID: tc.ID, Type: "function"}
			cto.Function.Name = tc.Name
			cto.Function.Arguments = tc.Arguments
			co.Message.ToolCalls = append(co.Message.ToolCalls, cto)
		}
		out.Choices = append(out.Choices, co)
	}
	return json.Marshal(out)
}

// ---- 流式:上游 Chat SSE data -> []Event ----

type ChatStreamDecoder struct{}

type chatChunkRaw struct {
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Content   string              `json:"content"`
			ToolCalls []chatStreamToolRaw `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *chatUsageRaw `json:"usage"`
}

type chatStreamToolRaw struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

func (d *ChatStreamDecoder) DecodeLine(data []byte) ([]Event, error) {
	if string(data) == "[DONE]" {
		return nil, nil
	}
	var chunk chatChunkRaw
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil, fmt.Errorf("decode chat chunk: %w", err)
	}
	var evs []Event
	finished := false
	for _, c := range chunk.Choices {
		if c.Delta.Content != "" {
			evs = append(evs, Event{Type: EventContentDelta, Delta: c.Delta.Content})
		}
		for _, tc := range c.Delta.ToolCalls {
			evs = append(evs, Event{Type: EventToolCallDelta, ToolCall: &ToolCall{
				ID: tc.ID, Type: "function", Name: tc.Function.Name, Arguments: tc.Function.Arguments,
			}})
		}
		if c.FinishReason != nil {
			finished = true
			// 用量先于 done 发出(与 Responses/Anthropic 解码器一致),Done 不内嵌用量,避免重复。
			if chunk.Usage != nil {
				evs = append(evs, Event{Type: EventUsage, Usage: chatUsageToIR(chunk.Usage)})
			}
			evs = append(evs, Event{Type: EventDone, Finish: *c.FinishReason})
		}
	}
	// 无 finish_reason 的独立 usage chunk(极少见)
	if !finished && chunk.Usage != nil {
		evs = append(evs, Event{Type: EventUsage, Usage: chatUsageToIR(chunk.Usage)})
	}
	return evs, nil
}

func chatUsageToIR(u *chatUsageRaw) *Usage {
	return &Usage{
		InputTokens: u.PromptTokens, OutputTokens: u.CompletionTokens,
		TotalTokens: u.TotalTokens, CacheReadTokens: u.PromptDetails.CachedTokens,
	}
}

// ---- 流式:[]Event -> 上游 Chat SSE 行 ----

type ChatStreamEncoder struct {
	toolCallIndex int
	pendingUsage  *Usage
}

func (e *ChatStreamEncoder) Encode(ev Event) ([][]byte, error) {
	switch ev.Type {
	case EventContentDelta:
		chunk := map[string]any{
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{"content": ev.Delta},
			}},
		}
		return [][]byte{EncodeSSELine(mustJSON(chunk))}, nil
	case EventToolCallDelta:
		if ev.ToolCall == nil {
			return nil, fmt.Errorf("tool call delta without ToolCall")
		}
		tc := map[string]any{
			"index":    e.toolCallIndex,
			"function": map[string]string{"name": ev.ToolCall.Name, "arguments": ev.ToolCall.Arguments},
		}
		if ev.ToolCall.ID != "" {
			tc["id"] = ev.ToolCall.ID
			tc["type"] = "function"
		}
		e.toolCallIndex++
		chunk := map[string]any{
			"choices": []map[string]any{{
				"index": 0,
				"delta": map[string]any{"tool_calls": []map[string]any{tc}},
			}},
		}
		return [][]byte{EncodeSSELine(mustJSON(chunk))}, nil
	case EventUsage:
		// 独立 usage 事件:暂存,并入随后的 EventDone(与 Responses/Anthropic 编码器一致)。
		if ev.Usage != nil {
			e.pendingUsage = ev.Usage
		}
		return nil, nil
	case EventDone:
		usage := ev.Usage
		if usage == nil {
			usage = e.pendingUsage
		}
		chunk := map[string]any{
			"choices": []map[string]any{{
				"index":         0,
				"delta":         map[string]any{},
				"finish_reason": ev.Finish,
			}},
		}
		if usage != nil {
			chunk["usage"] = usageToChat(usage)
		}
		return [][]byte{
			EncodeSSELine(mustJSON(chunk)),
			EncodeSSELine([]byte("[DONE]")),
		}, nil
	}
	return nil, fmt.Errorf("unknown event type %d", ev.Type)
}

func (e *ChatStreamEncoder) Reset() {
	e.toolCallIndex = 0
	e.pendingUsage = nil
}

func usageToChat(u *Usage) map[string]any {
	if u == nil {
		return map[string]any{}
	}
	out := map[string]any{
		"prompt_tokens":     u.InputTokens,
		"completion_tokens": u.OutputTokens,
		"total_tokens":      u.TotalTokens,
	}
	if u.CacheReadTokens > 0 {
		out["prompt_tokens_details"] = map[string]int{"cached_tokens": u.CacheReadTokens}
	}
	return out
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err) // 仅在编码内部类型时触发,不应发生
	}
	return b
}
