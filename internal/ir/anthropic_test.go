package ir

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestEncodeAnthropicRequestToolResultRoleUser(t *testing.T) {
	// C1 回归:Anthropic Messages API 不接受 role:"tool",编码时必须映射为 "user"。
	r := &Request{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []Message{
			{Role: "assistant", ToolCalls: []ToolCall{{ID: "tu_1", Type: "function", Name: "get_weather", Arguments: `{"city":"beijing"}`}}},
			{Role: "tool", ToolCallID: "tu_1", Content: []ContentPart{{Type: "text", Text: "sunny"}}},
		},
		MaxTokens: intPtr(50),
	}
	out, err := EncodeAnthropicRequest(r)
	if err != nil {
		t.Fatalf("EncodeAnthropicRequest: %v", err)
	}
	var m map[string]any
	json.Unmarshal(out, &m)
	msgs := m["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2", len(msgs))
	}
	// 第二条消息是 tool_result,角色必须是 user
	tm := msgs[1].(map[string]any)
	if tm["role"] != "user" {
		t.Errorf("tool_result message role = %v, want \"user\"", tm["role"])
	}
	content := tm["content"].([]any)
	tr := content[0].(map[string]any)
	if tr["type"] != "tool_result" || tr["tool_use_id"] != "tu_1" {
		t.Errorf("tool_result block = %+v", tr)
	}
}

func TestEncodeAnthropicRequestToolResultRoleUserFromFixture(t *testing.T) {
	// C1 回归(协议真实路径):decode anthropic fixture -> encode,断言角色重映射。
	r, err := DecodeAnthropicRequest(readTestdata(t, "req_anthropic.json"))
	if err != nil {
		t.Fatalf("DecodeAnthropicRequest: %v", err)
	}
	out, err := EncodeAnthropicRequest(r)
	if err != nil {
		t.Fatalf("EncodeAnthropicRequest: %v", err)
	}
	var m map[string]any
	json.Unmarshal(out, &m)
	msgs := m["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("messages = %d, want 3", len(msgs))
	}
	tm := msgs[2].(map[string]any)
	if tm["role"] != "user" {
		t.Errorf("tool_result message role = %v, want \"user\"", tm["role"])
	}
	content := tm["content"].([]any)
	tr := content[0].(map[string]any)
	if tr["type"] != "tool_result" || tr["tool_use_id"] != "tu_1" {
		t.Errorf("tool_result block = %+v", tr)
	}
}

func TestDecodeAnthropicRequestToolResultIsError(t *testing.T) {
	// I1 回归:is_error=true 的 tool_result 必须保留,失败的工具执行不能被拍平成成功文本。
	body := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"messages": [
			{"role": "assistant", "content": [{"type": "tool_use", "id": "tu_2", "name": "get_weather", "input": {"city": "x"}}]},
			{"role": "user", "content": [{"type": "tool_result", "tool_use_id": "tu_2", "is_error": true, "content": "city not found"}]}
		],
		"max_tokens": 50
	}`)
	r, err := DecodeAnthropicRequest(body)
	if err != nil {
		t.Fatalf("DecodeAnthropicRequest: %v", err)
	}
	toolMsg := r.Messages[1]
	if toolMsg.Role != "tool" || toolMsg.ToolCallID != "tu_2" {
		t.Fatalf("tool message = %+v", toolMsg)
	}
	if len(toolMsg.Content) != 1 || toolMsg.Content[0].Type != "tool_result" {
		t.Fatalf("tool content = %+v", toolMsg.Content)
	}
	tr := toolMsg.Content[0]
	if !tr.IsError {
		t.Error("is_error not preserved on decode")
	}
	if len(tr.ToolResultContent) != 1 || tr.ToolResultContent[0].Text != "city not found" {
		t.Errorf("tool_result content = %+v", tr.ToolResultContent)
	}
	// 编码 -> 重新解码,is_error 保留
	out, err := EncodeAnthropicRequest(r)
	if err != nil {
		t.Fatalf("EncodeAnthropicRequest: %v", err)
	}
	back, err := DecodeAnthropicRequest(out)
	if err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if len(back.Messages) != 2 {
		t.Fatalf("re-decoded messages = %+v", back.Messages)
	}
	backTR := back.Messages[1].Content[0]
	if backTR.Type != "tool_result" || !backTR.IsError || backTR.ToolResultContent[0].Text != "city not found" {
		t.Errorf("round-trip is_error lost: %+v", backTR)
	}
	// 编码输出含 is_error:true
	var m map[string]any
	json.Unmarshal(out, &m)
	tm := m["messages"].([]any)[1].(map[string]any)
	block := tm["content"].([]any)[0].(map[string]any)
	if block["is_error"] != true {
		t.Errorf("encoded tool_result missing is_error=true: %+v", block)
	}
	if block["tool_use_id"] != "tu_2" {
		t.Errorf("encoded tool_use_id = %v", block["tool_use_id"])
	}
}

func TestAnthropicStreamEncoderToolBlockStart(t *testing.T) {
	// I2 回归:tool 增量序列首条 delta 前先发 content_block_start(tool_use),
	// 后续增量只发 content_block_delta(input_json_delta)。
	e := &AnthropicStreamEncoder{}
	lines, err := e.Encode(Event{Type: EventToolCallDelta, ToolCall: &ToolCall{
		Type: "function", Name: "get_weather", Arguments: `{"ci`,
	}})
	if err != nil {
		t.Fatalf("encode tool delta: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("first tool delta should produce 2 lines (start + delta), got %d", len(lines))
	}
	if !bytes.Contains(lines[0], []byte(`"content_block_start"`)) || !bytes.Contains(lines[0], []byte(`"tool_use"`)) {
		t.Errorf("start line = %s", lines[0])
	}
	if !bytes.Contains(lines[0], []byte(`"name":"get_weather"`)) || !bytes.Contains(lines[0], []byte(`"id":"toolu_1"`)) {
		t.Errorf("start line missing name/id: %s", lines[0])
	}
	if !bytes.Contains(lines[1], []byte(`"input_json_delta"`)) || !bytes.Contains(lines[1], []byte(`"{\"ci"`)) {
		t.Errorf("delta line = %s", lines[1])
	}
	// 同一 tool 块的后续增量:只发 content_block_delta,不再发 start
	lines, _ = e.Encode(Event{Type: EventToolCallDelta, ToolCall: &ToolCall{
		Type: "function", Arguments: `ty":"beijing"}`,
	}})
	if len(lines) != 1 || !bytes.Contains(lines[0], []byte(`"input_json_delta"`)) {
		t.Errorf("second delta should be 1 input_json_delta line: %d lines", len(lines))
	}
	if bytes.Contains(lines[0], []byte(`"content_block_start"`)) {
		t.Errorf("duplicate content_block_start: %s", lines[0])
	}
	// 文本增量开启新块,下一个 tool 增量重新发 start
	e.Encode(Event{Type: EventContentDelta, Delta: "text"})
	lines, _ = e.Encode(Event{Type: EventToolCallDelta, ToolCall: &ToolCall{Type: "function", Arguments: `{"a"`}})
	if len(lines) != 2 || !bytes.Contains(lines[0], []byte(`"content_block_start"`)) {
		t.Errorf("tool delta after text should restart block: %d lines", len(lines))
	}
}

func TestAnthropicCacheControlRoundTrip(t *testing.T) {
	// I3 回归:system + 消息文本块的 cache_control 在解码/编码间往返保留。
	body := []byte(`{
		"model": "claude-3-5-sonnet-20241022",
		"system": [
			{"type": "text", "text": "cache me", "cache_control": {"type": "ephemeral"}},
			{"type": "text", "text": "plain"}
		],
		"messages": [
			{"role": "user", "content": [{"type": "text", "text": "hi", "cache_control": {"type": "ephemeral"}}]}
		],
		"max_tokens": 50
	}`)
	r, err := DecodeAnthropicRequest(body)
	if err != nil {
		t.Fatalf("DecodeAnthropicRequest: %v", err)
	}
	if len(r.System) != 2 {
		t.Fatalf("system parts = %+v", r.System)
	}
	if r.System[0].CacheControl == nil || r.System[0].CacheControl.Type != "ephemeral" {
		t.Errorf("system[0] cache_control not decoded: %+v", r.System[0])
	}
	if r.System[1].CacheControl != nil {
		t.Errorf("system[1] should have no cache_control: %+v", r.System[1])
	}
	if r.Messages[0].Content[0].CacheControl == nil || r.Messages[0].Content[0].CacheControl.Type != "ephemeral" {
		t.Errorf("message text cache_control not decoded: %+v", r.Messages[0].Content[0])
	}
	// 编码:system 必须是 blocks 形式且带 cache_control;消息文本块同样带
	out, err := EncodeAnthropicRequest(r)
	if err != nil {
		t.Fatalf("EncodeAnthropicRequest: %v", err)
	}
	var m map[string]any
	json.Unmarshal(out, &m)
	sys := m["system"].([]any)
	if len(sys) != 2 {
		t.Fatalf("encoded system = %+v", sys)
	}
	b0 := sys[0].(map[string]any)
	cc0, ok := b0["cache_control"].(map[string]any)
	if !ok || cc0["type"] != "ephemeral" {
		t.Errorf("encoded system[0] cache_control = %+v", b0["cache_control"])
	}
	if b1 := sys[1].(map[string]any); b1["cache_control"] != nil {
		t.Errorf("encoded system[1] should have no cache_control: %+v", b1)
	}
	msg0 := m["messages"].([]any)[0].(map[string]any)
	block := msg0["content"].([]any)[0].(map[string]any)
	if block["cache_control"] == nil {
		t.Errorf("encoded message text block missing cache_control: %+v", block)
	}
	// 重新解码,再次断言保留
	back, err := DecodeAnthropicRequest(out)
	if err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if back.System[0].CacheControl == nil || back.System[0].CacheControl.Type != "ephemeral" {
		t.Errorf("round-trip system cache_control lost: %+v", back.System[0])
	}
	if back.Messages[0].Content[0].CacheControl == nil || back.Messages[0].Content[0].CacheControl.Type != "ephemeral" {
		t.Errorf("round-trip message cache_control lost: %+v", back.Messages[0].Content[0])
	}
}

func TestAnthropicSystemCacheControlSingleText(t *testing.T) {
	// I3 回归:单条 system 文本带 cache_control 时不能退化为 string 形式(会丢 cache_control)。
	r := &Request{
		Model:  "claude-3-5-sonnet-20241022",
		System: []ContentPart{{Type: "text", Text: "cache me", CacheControl: &CacheControl{Type: "ephemeral"}}},
		Messages: []Message{
			{Role: "user", Content: []ContentPart{{Type: "text", Text: "hi"}}},
		},
		MaxTokens: intPtr(50),
	}
	out, err := EncodeAnthropicRequest(r)
	if err != nil {
		t.Fatalf("EncodeAnthropicRequest: %v", err)
	}
	var m map[string]any
	json.Unmarshal(out, &m)
	sys, ok := m["system"].([]any)
	if !ok || len(sys) != 1 {
		t.Fatalf("system should be blocks form, got %v", m["system"])
	}
	b := sys[0].(map[string]any)
	if b["cache_control"] == nil {
		t.Errorf("single text system cache_control dropped: %+v", b)
	}
}

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
	if toolMsg.Role != "tool" || toolMsg.ToolCallID != "tu_1" {
		t.Errorf("tool message = %+v", toolMsg)
	}
	// I1:tool_result 解码为 tool_result ContentPart(非拍平 text)
	if len(toolMsg.Content) != 1 || toolMsg.Content[0].Type != "tool_result" {
		t.Fatalf("tool content = %+v, want tool_result part", toolMsg.Content)
	}
	if tr := toolMsg.Content[0].ToolResultContent; len(tr) != 1 || tr[0].Type != "text" || tr[0].Text != "sunny" {
		t.Errorf("tool_result content = %+v, want [text sunny]", tr)
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
