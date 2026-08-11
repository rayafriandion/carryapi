# carryAPI 子项目 3:协议适配层(IR 枢纽) 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现三种模型协议(OpenAI Chat Completions、OpenAI Responses、Anthropic Messages)之间的 IR 枢纽转换:IR 类型定义、6 个请求/响应转换器(3 Decoder + 3 Encoder)、3 套流式事件转换(SSE 双向)、统一错误 IR + 各协议错误格式编码、9 种上下游组合的转换矩阵集成测试。完成后,子项目 4 可直接用它做上游代理转发。

**Architecture:** 纯库子项目,新包 `internal/ir`,不依赖子项目 2 的认证/HTTP 层(代理端点接线留给子项目 4)。核心是中间表示(IR)类型 + 纯函数转换器,方向为 `下游格式 <-> IR <-> 上游格式`。流式用统一 `InternalEvent` 事件流 + 有状态 Encoder(维护跨 chunk 的索引/ID),无状态 Decoder(每行 SSE data 转 0..n 个事件)。错误统一为 `ir.Error`,按下游协议编码成 OpenAI 或 Anthropic 错误 JSON。

**Tech Stack:** Go 标准库(`encoding/json`、`strings`、`bytes`)。无新第三方依赖,无 CGO。

## Global Constraints

- Go 1.22+;无 CGO。纯标准库。
- 转换器必须是纯函数(Decoder)或可 Reset 的有状态结构(流式 Encoder);不得依赖全局状态。
- IR 是唯一内部表示:所有跨协议转换必须经过 IR,禁止协议间直接转换。
- 工具调用统一:三种协议的 tool/function 概念映射到 `ir.Tool`/`ir.ToolCall`;跨协议时做格式适配(function name/arguments JSON 字符串 <-> JSON 对象)。
- 用量统一:`ir.Usage` 覆盖三协议并集(input/output/cache_read/cache_creation);转回无 cache 概念的协议时丢弃对应字段,不影响统计。
- 流式用量:消费统一的 `EventUsage`;上游转下游时把 usage 放到目标协议的末尾事件(Chat 的 usage chunk / Responses 的 response.completed / Anthropic 的 message_delta+message_stop)。
- 错误:`ir.Error` 带协议无关的 Type/Code/Message/StatusCode;`OpenAIErrorBody`/`AnthropicErrorBody` 按下游协议编码。
- TDD:每个任务先写失败测试,再实现,再验证通过,再提交。
- Git 身份:`rayafriandion <amizhisa@outlook.com>`(本仓库已配置)。
- fixture 放 `internal/ir/testdata/`,用真实协议样例(基于 OpenAI/Anthropic 公开文档),不依赖网络。

---

## File Structure

```
carryAPI/
└── internal/
    └── ir/
        ├── types.go            # IR 类型:Request/Message/ContentPart/Tool/ToolChoice/CacheControl/Response/Choice/ToolCall/Usage/Event
        ├── types_test.go
        ├── sse.go              # SSE 解析/编码工具:SplitSSE、EncodeSSELine、DecodeSSEData
        ├── sse_test.go
        ├── chat.go             # Chat 转换器:DecodeChatRequest/EncodeChatRequest/DecodeChatResponse/EncodeChatResponse + ChatStreamDecoder/ChatStreamEncoder
        ├── chat_test.go
        ├── responses.go        # Responses 转换器:DecodeResponsesRequest/EncodeResponsesRequest/DecodeResponsesResponse/EncodeResponsesResponse + ResponsesStreamDecoder/ResponsesStreamEncoder
        ├── responses_test.go
        ├── anthropic.go        # Anthropic 转换器:DecodeAnthropicRequest/EncodeAnthropicRequest/DecodeAnthropicResponse/EncodeAnthropicResponse + AnthropicStreamDecoder/AnthropicStreamEncoder
        ├── anthropic_test.go
        ├── errors.go           # ir.Error + OpenAIErrorBody/AnthropicErrorBody
        ├── errors_test.go
        ├── matrix_test.go      # 9 组合矩阵集成测试
        └── testdata/           # fixture:request_chat.json, request_responses.json, request_anthropic.json, response_*.json, stream_*.jsonl
            ├── req_chat.json
            ├── req_responses.json
            ├── req_anthropic.json
            ├── resp_chat.json
            ├── resp_responses.json
            ├── resp_anthropic.json
            ├── stream_chat.jsonl
            ├── stream_responses.jsonl
            └── stream_anthropic.jsonl
```

`types.go` 定义 IR;`sse.go` 是 SSE 行级工具;`chat.go`/`responses.go`/`anthropic.go` 各含一个协议的 Decoder+Encoder(+流式);`errors.go` 是错误 IR 与协议错误编码;`matrix_test.go` 做端到端矩阵验证。每个文件单一职责,转换器间通过 IR 类型通信,互不 import。

---

### Task 1: IR 核心类型

**Files:**
- Create: `internal/ir/types.go`
- Test: `internal/ir/types_test.go`

**Interfaces:**
- Produces: 以下类型(子项目 3 全部任务依赖):

```go
package ir

import "encoding/json"

// Request 是三种协议请求的并集(IR 枢纽的输入)。
type Request struct {
	Model       string
	Messages    []Message
	System      []ContentPart // Anthropic system 段 / Responses instructions;可含 cache_control
	Tools       []Tool
	ToolChoice  *ToolChoice
	Stream      bool
	Temperature *float64
	TopP        *float64
	MaxTokens   *int
	Stop        []string
}

// Message 是统一消息序列(role + 多模态 content)。
type Message struct {
	Role       string // system/user/assistant/tool
	Content    []ContentPart
	ToolCallID string // role=tool 时关联的 tool_use/call id
	Name       string // role=tool 时的函数名(Anthropic 无,置空)
}

// ContentPart 是多模态/工具内容单元。
type ContentPart struct {
	Type      string // text / image_url / tool_use / tool_result
	Text      string
	ImageURL  string
	// tool_use
	ToolUseID  string
	ToolName   string
	ToolInput  json.RawMessage // 工具入参,JSON 对象(原样保留)
	// tool_result
	ToolResultContent []ContentPart // 工具返回(可嵌套 text/image)
	IsError           bool
}

// Tool 是统一工具定义。
type Tool struct {
	Type        string // 恒为 "function"(Anthropic 无 type,编码时省略)
	Name        string
	Description string
	Parameters  json.RawMessage // JSON Schema 对象
	CacheControl *CacheControl  // 仅 Anthropic
}

type ToolChoice struct {
	Type string // auto / none / required / any / function / tool
	Name string // function|tool 类型时的名字
}

type CacheControl struct {
	Type string // 恒为 "ephemeral"
}

// Response 是三种协议非流式响应的并集。
type Response struct {
	ID      string
	Model   string
	Choices []Choice
	Usage   Usage
}

type Choice struct {
	Index        int
	Role         string // 恒为 "assistant"
	Content      []ContentPart
	ToolCalls    []ToolCall
	FinishReason string // stop / length / tool_calls / end_turn / stop_sequence
}

type ToolCall struct {
	ID        string
	Type      string // function
	Name      string
	Arguments string // JSON 字符串(函数参数的序列化文本)
}

// Usage 是用量并集。
type Usage struct {
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
	TotalTokens         int
}

// EventType 是统一流式事件类型。
type EventType int

const (
	EventContentDelta EventType = iota // 文本增量
	EventToolCallDelta                 // 工具参数增量
	EventUsage                         // 用量(末尾)
	EventDone                          // 结束
)

// Event 是统一流式事件。
type Event struct {
	Type     EventType
	Delta    string    // 文本或工具参数增量
	ToolCall *ToolCall // 增量(部分 Arguments)
	Usage    *Usage
	Finish   string // EventDone 时的 finish_reason
}
```

- [ ] **Step 1: 写失败测试**

`internal/ir/types_test.go`:

```go
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
	if len(back.Messages) != 1 || back.Messages[0].Content[0].Text != "hi" {
		t.Errorf("round trip mismatch: %+v", back)
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
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /d/Projects/carryAPI
go test ./internal/ir/ -v
```

预期:编译失败(找不到 `ir` 包)。

- [ ] **Step 3: 实现 types.go**

见上方 Interfaces 代码块(整个文件)。

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/ir/ -v
```

预期:4 个测试 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/ir/types.go internal/ir/types_test.go
git commit -m "feat(ir): core intermediate representation types"
```

---

### Task 2: SSE 行级工具

**Files:**
- Create: `internal/ir/sse.go`
- Test: `internal/ir/sse_test.go`

**Interfaces:**
- Produces:

```go
// SplitSSE 把 SSE 响应体按 "\n\n" 或 "\r\n\r\n" 分割成事件块。
// 每块内找 "data:" 行;支持多行 data(拼接);忽略以 ":" 开头的注释行。
// 返回每个事件块的 data 内容(不含 "data:" 前缀)。"data: [DONE]" 返回一个特殊值。
func SplitSSE(body []byte) ([][]byte, error)

// EncodeSSELine 把一条消息编码成 SSE 行:"data: <payload>\n\n"。
func EncodeSSELine(payload []byte) []byte

// SSEEventType 从 data 负载解析事件类型字段(用于 Responses/Anthropic 的分派)。
// 找不到返回空串。
func SSEEventType(data []byte) string
```

- [ ] **Step 1: 写失败测试**

`internal/ir/sse_test.go`:

```go
package ir

import (
	"bytes"
	"testing"
)

func TestSplitSSEBasic(t *testing.T) {
	body := []byte("data: {\"a\":1}\n\ndata: {\"b\":2}\n\n")
	events, err := SplitSSE(body)
	if err != nil {
		t.Fatalf("SplitSSE: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if string(events[0]) != `{"a":1}` {
		t.Errorf("event0 = %q", events[0])
	}
}

func TestSplitSSEMultiLineData(t *testing.T) {
	body := []byte("data: {\"a\":1}\ndata: ,\"b\":2}\n\n")
	events, _ := SplitSSE(body)
	if len(events) != 1 || string(events[0]) != `{"a":1,"b":2}` {
		t.Errorf("multiline join: %q", events)
	}
}

func TestSplitSSEDone(t *testing.T) {
	body := []byte("data: [DONE]\n\n")
	events, _ := SplitSSE(body)
	if len(events) != 1 || string(events[0]) != "[DONE]" {
		t.Errorf("done: %q", events)
	}
}

func TestSplitSSECommentAndCRLF(t *testing.T) {
	body := []byte(": keep-alive comment\r\n\r\ndata: {\"x\":1}\r\n\r\n")
	events, _ := SplitSSE(body)
	if len(events) != 1 || string(events[0]) != `{"x":1}` {
		t.Errorf("comment/crlf: %q", events)
	}
}

func TestEncodeSSELine(t *testing.T) {
	got := EncodeSSELine([]byte(`{"x":1}`))
	if string(got) != "data: {\"x\":1}\n\n" {
		t.Errorf("encoded = %q", got)
	}
}

func TestSSEEventType(t *testing.T) {
	if got := SSEEventType([]byte(`{"type":"response.output_text.delta","delta":"hi"}`)); got != "response.output_text.delta" {
		t.Errorf("type = %q", got)
	}
	if got := SSEEventType([]byte(`{"x":1}`)); got != "" {
		t.Errorf("no type should be empty, got %q", got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/ir/ -v -run SSE
```

预期:编译失败。

- [ ] **Step 3: 实现 sse.go**

```go
package ir

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

// SplitSSE 按空行分割 SSE 流,返回每个事件块的 data 内容。
func SplitSSE(body []byte) ([][]byte, error) {
	var events [][]byte
	var current []byte
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSuffix(line, []byte("\r"))
		if len(line) == 0 {
			// 空行 = 事件结束
			if len(current) > 0 {
				events = append(events, current)
				current = nil
			}
			continue
		}
		if bytes.HasPrefix(line, []byte(":")) {
			continue // 注释行
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			payload := bytes.TrimSpace(line[len("data:"):])
			if len(current) > 0 {
				// 多行 data:拼接(SSE 规范:多行 data 用单个换行连接)
				current = append(current, '\n')
			}
			current = append(current, payload...)
		}
		// 其他字段(event/id/retry)忽略
	}
	if len(current) > 0 {
		events = append(events, current)
	}
	return events, nil
}

// EncodeSSELine 编码单条 SSE data 行。
func EncodeSSELine(payload []byte) []byte {
	var b bytes.Buffer
	b.WriteString("data: ")
	b.Write(payload)
	b.WriteString("\n\n")
	return b.Bytes()
}

// SSEEventType 从 SSE data JSON 解析 "type" 字段。
func SSEEventType(data []byte) string {
	var v struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return ""
	}
	return v.Type
}

var _ = errors.New // 保留 import 一致性(如无用到可删)
var _ = strings.TrimSpace
```

> 注:末尾两行 `var _ =` 是防御性占位,实现者应删除未用 import。本文件的实际逻辑不需要 errors/strings;若编译器报未使用,删掉对应 import 即可。

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/ir/ -v -run SSE
```

预期:6 个 SSE 测试 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/ir/sse.go internal/ir/sse_test.go
git commit -m "feat(ir): SSE line-level parsing and encoding utilities"
```

---

### Task 3: Chat 请求转换(Decode/Encode)

**Files:**
- Create: `internal/ir/chat.go`(本任务只加请求部分)
- Test: `internal/ir/chat_test.go`(本任务只测请求部分)

**Interfaces:**
- Produces:

```go
// DecodeChatRequest 把 OpenAI Chat Completions 请求 JSON 解码为 IR Request。
// 处理:messages(含 string/array content、tool_calls、tool 消息)、tools、
// tool_choice、temperature/max_tokens/max_completion_tokens/top_p/stop/stream。
// role=system 的 message 抽出到 System;其余进 Messages。
func DecodeChatRequest(body []byte) (*Request, error)

// EncodeChatRequest 把 IR Request 编码为 OpenAI Chat Completions 请求 JSON。
// System 变为首位 system message;工具 arguments 字符串转 JSON 对象;
// ToolChoice{Type:"function",Name} 转 {"type":"function","function":{"name":...}}。
func EncodeChatRequest(r *Request) ([]byte, error)
```

- [ ] **Step 1: 写失败测试**

`internal/ir/chat_test.go`(请求部分):

```go
package ir

import (
	"encoding/json"
	"testing"
)

func TestDecodeChatRequestBasic(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "system", "content": "You are helpful."},
			{"role": "user", "content": "Hi there"},
			{"role": "assistant", "content": "Hello!"}
		],
		"temperature": 0.7,
		"max_tokens": 100,
		"stream": true
	}`)
	r, err := DecodeChatRequest(body)
	if err != nil {
		t.Fatalf("DecodeChatRequest: %v", err)
	}
	if r.Model != "gpt-4o" {
		t.Errorf("model = %q", r.Model)
	}
	if len(r.System) != 1 || r.System[0].Text != "You are helpful." {
		t.Errorf("system = %+v", r.System)
	}
	if len(r.Messages) != 2 || r.Messages[0].Role != "user" || r.Messages[1].Role != "assistant" {
		t.Errorf("messages = %+v", r.Messages)
	}
	if r.Temperature == nil || *r.Temperature != 0.7 {
		t.Error("temperature not decoded")
	}
	if r.MaxTokens == nil || *r.MaxTokens != 100 {
		t.Error("max_tokens not decoded")
	}
	if !r.Stream {
		t.Error("stream not decoded")
	}
}

func TestDecodeChatRequestTools(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"messages": [{"role": "user", "content": "weather?"}],
		"tools": [{"type": "function", "function": {"name": "get_weather", "description": "Get weather", "parameters": {"type": "object", "properties": {"city": {"type": "string"}}}}}],
		"tool_choice": {"type": "function", "function": {"name": "get_weather"}}
	}`)
	r, err := DecodeChatRequest(body)
	if err != nil {
		t.Fatalf("DecodeChatRequest: %v", err)
	}
	if len(r.Tools) != 1 || r.Tools[0].Name != "get_weather" || r.Tools[0].Type != "function" {
		t.Fatalf("tools = %+v", r.Tools)
	}
	if r.ToolChoice == nil || r.ToolChoice.Type != "function" || r.ToolChoice.Name != "get_weather" {
		t.Errorf("tool_choice = %+v", r.ToolChoice)
	}
}

func TestDecodeChatRequestToolMessages(t *testing.T) {
	body := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "assistant", "content": null, "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"beijing\"}"}}]},
			{"role": "tool", "tool_call_id": "call_1", "content": "sunny"}
		]
	}`)
	r, err := DecodeChatRequest(body)
	if err != nil {
		t.Fatalf("DecodeChatRequest: %v", err)
	}
	assistant := r.Messages[0]
	if len(assistant.Content) != 0 {
		t.Errorf("assistant content should be empty, got %+v", assistant.Content)
	}
	if len(assistant.ToolCalls) != 1 { // Message.ToolCalls 字段由本任务 Step 2(前置步骤)加到 types.go
		t.Errorf("assistant tool calls missing")
	}
	tool := r.Messages[1]
	if tool.Role != "tool" || tool.ToolCallID != "call_1" || tool.Content[0].Text != "sunny" {
		t.Errorf("tool message = %+v", tool)
	}
}
```

> 注:上面的测试引用了 `assistant.ToolCalls`——但 Task 1 的 `Message` 类型没有 ToolCalls 字段。**修正**:IR `Message` 需要加 `ToolCalls []ToolCall` 字段(assistant 消息携带工具调用)。更新 types.go:在 `Message` 结构体加 `ToolCalls []ToolCall`(仅 assistant 用)。实现者先改 types.go 加字段 + types_test.go 补一个断言,再做本任务。

- [ ] **Step 2: 更新 Message 类型(前置步骤)**

在 `internal/ir/types.go` 的 `Message` 结构体加字段:

```go
type Message struct {
	Role       string
	Content    []ContentPart
	ToolCalls  []ToolCall // assistant 消息的工具调用(Chat/Responses)
	ToolCallID string
	Name       string
}
```

- [ ] **Step 3: 运行测试确认失败**

```bash
go test ./internal/ir/ -v -run Chat
```

预期:编译失败(DecodeChatRequest 未定义)。

- [ ] **Step 4: 实现 Chat 请求转换**

在 `internal/ir/chat.go` 加(文件开头含 import `bytes`、`encoding/json`、`fmt`):

```go
package ir

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ---- 请求:下游 Chat JSON -> IR ----

type chatRequestRaw struct {
	Model       string          `json:"model"`
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
	Type     string          `json:"type"`
	Text     string          `json:"text"`
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
	Model       string              `json:"model"`
	Messages    []chatMessageOut    `json:"messages"`
	Tools       []chatToolOut       `json:"tools,omitempty"`
	ToolChoice  json.RawMessage     `json:"tool_choice,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
	Temperature *float64            `json:"temperature,omitempty"`
	TopP        *float64            `json:"top_p,omitempty"`
	MaxTokens   *int                `json:"max_tokens,omitempty"`
	Stop        json.RawMessage     `json:"stop,omitempty"`
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
			fn := chatToolCallOut{ID: tc.ID, Type: tc.Type}
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
			"type": "function",
			"function": map[string]string{"name": tc.Name},
		})
	default:
		return nil, fmt.Errorf("unsupported tool_choice type %q", tc.Type)
	}
}
```

> 注:`ToolCall.Type` 为空时 `chatToolCallOut.Type` 会是 `""`——Chat 的 tool_calls 元素 `type` 恒为 "function",上游可接受。为稳妥,编码时若 `tc.Type == ""` 则补 "function"。实现者加一行判断。

- [ ] **Step 5: 运行测试确认通过**

```bash
go test ./internal/ir/ -v -run Chat
```

预期:3 个 Chat 请求测试 PASS。

- [ ] **Step 6: 提交**

```bash
git add internal/ir/types.go internal/ir/types_test.go internal/ir/chat.go internal/ir/chat_test.go
git commit -m "feat(ir): chat request decoder and encoder"
```

---

### Task 4: Chat 响应转换(非流式 + 流式)

**Files:**
- Modify: `internal/ir/chat.go`
- Modify: `internal/ir/chat_test.go`

**Interfaces:**
- Produces:

```go
// DecodeChatResponse 把 OpenAI Chat 非流式响应 JSON 解码为 IR Response。
func DecodeChatResponse(body []byte) (*Response, error)

// EncodeChatResponse 把 IR Response 编码为 OpenAI Chat 响应 JSON。
// Usage 含 CacheReadTokens 时输出 prompt_tokens_details.cached_tokens。
func EncodeChatResponse(r *Response) ([]byte, error)

// ChatStreamDecoder 把上游 Chat SSE data 行转成 0..n 个 Event。
// chunk 的 delta.content -> EventContentDelta;delta.tool_calls[i].function.arguments
// -> EventToolCallDelta;finish_reason 非空 -> EventDone{Finish};usage -> EventUsage。
// "data: [DONE]" 由调用方(代理层)处理,decoder 收到 [DONE] 返回 nil 事件。
type ChatStreamDecoder struct{}

func (d *ChatStreamDecoder) DecodeLine(data []byte) ([]Event, error)

// ChatStreamEncoder 把 Event 序列编码成上游 Chat SSE 行(每行完整 "data: xxx\n\n")。
// 有状态:跨 chunk 跟踪 tool_call index;EventDone 时输出 usage chunk(若携带)
// + [DONE];Reset() 清状态。
type ChatStreamEncoder struct{}

func (e *ChatStreamEncoder) Encode(ev Event) ([][]byte, error)
func (e *ChatStreamEncoder) Reset()
```

- [ ] **Step 1: 写失败测试(响应 + 流式)**

`internal/ir/chat_test.go` 追加:

```go
func TestDecodeChatResponseBasic(t *testing.T) {
	body := []byte(`{
		"id": "chatcmpl-123",
		"model": "gpt-4o",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": "Hello!"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}
	}`)
	r, err := DecodeChatResponse(body)
	if err != nil {
		t.Fatalf("DecodeChatResponse: %v", err)
	}
	if r.ID != "chatcmpl-123" || r.Model != "gpt-4o" {
		t.Errorf("id/model = %q/%q", r.ID, r.Model)
	}
	if len(r.Choices) != 1 || r.Choices[0].FinishReason != "stop" || r.Choices[0].Content[0].Text != "Hello!" {
		t.Errorf("choices = %+v", r.Choices)
	}
	if r.Usage.InputTokens != 10 || r.Usage.OutputTokens != 20 || r.Usage.TotalTokens != 30 {
		t.Errorf("usage = %+v", r.Usage)
	}
}

func TestDecodeChatResponseToolCall(t *testing.T) {
	body := []byte(`{
		"choices": [{"index": 0, "message": {"role": "assistant", "content": null, "tool_calls": [{"id": "call_9", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"beijing\"}"}}]}, "finish_reason": "tool_calls"}],
		"usage": {"prompt_tokens": 5, "completion_tokens": 8, "total_tokens": 13}
	}`)
	r, _ := DecodeChatResponse(body)
	ch := r.Choices[0]
	if ch.FinishReason != "tool_calls" || len(ch.ToolCalls) != 1 {
		t.Fatalf("choice = %+v", ch)
	}
	tc := ch.ToolCalls[0]
	if tc.Name != "get_weather" || tc.Arguments != `{"city":"beijing"}` {
		t.Errorf("tool call = %+v", tc)
	}
}

func TestDecodeChatResponseCachedTokens(t *testing.T) {
	body := []byte(`{
		"choices": [{"message": {"role": "assistant", "content": "x"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 100, "completion_tokens": 5, "total_tokens": 105, "prompt_tokens_details": {"cached_tokens": 60}}
	}`)
	r, _ := DecodeChatResponse(body)
	if r.Usage.CacheReadTokens != 60 {
		t.Errorf("cache read = %d, want 60", r.Usage.CacheReadTokens)
	}
}

func TestEncodeChatResponseRoundTrip(t *testing.T) {
	r := &Response{
		ID: "chatcmpl-1", Model: "gpt-4o",
		Choices: []Choice{{Index: 0, Role: "assistant", Content: []ContentPart{{Type: "text", Text: "hi"}}, FinishReason: "stop"}},
		Usage:   Usage{InputTokens: 5, OutputTokens: 3, TotalTokens: 8, CacheReadTokens: 2},
	}
	out, err := EncodeChatResponse(r)
	if err != nil {
		t.Fatalf("EncodeChatResponse: %v", err)
	}
	back, err := DecodeChatResponse(out)
	if err != nil {
		t.Fatalf("re-decode: %v", err)
	}
	if back.Choices[0].Content[0].Text != "hi" || back.Choices[0].FinishReason != "stop" {
		t.Errorf("round trip choice: %+v", back.Choices[0])
	}
	if back.Usage.CacheReadTokens != 2 {
		t.Errorf("round trip cache: %d", back.Usage.CacheReadTokens)
	}
}

func TestChatStreamDecoder(t *testing.T) {
	d := &ChatStreamDecoder{}
	// content delta
	evs, err := d.DecodeLine([]byte(`{"id":"x","choices":[{"index":0,"delta":{"content":"Hel"},"finish_reason":null}]}`))
	if err != nil || len(evs) != 1 || evs[0].Type != EventContentDelta || evs[0].Delta != "Hel" {
		t.Fatalf("content delta: %+v err %v", evs, err)
	}
	// tool call arguments delta
	evs, _ = d.DecodeLine([]byte(`{"id":"x","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"get_weather","arguments":"{\"city\":"}}]},"finish_reason":null}]}`))
	if len(evs) != 1 || evs[0].Type != EventToolCallDelta {
		t.Fatalf("tool delta: %+v", evs)
	}
	// finish + usage
	evs, _ = d.DecodeLine([]byte(`{"id":"x","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`))
	hasDone, hasUsage := false, false
	for _, ev := range evs {
		if ev.Type == EventDone && ev.Finish == "stop" {
			hasDone = true
		}
		if ev.Type == EventUsage && ev.Usage != nil && ev.Usage.TotalTokens == 3 {
			hasUsage = true
		}
	}
	if !hasDone || !hasUsage {
		t.Errorf("finish+usage events: %+v", evs)
	}
}

func TestChatStreamEncoder(t *testing.T) {
	e := &ChatStreamEncoder{}
	lines, err := e.Encode(Event{Type: EventContentDelta, Delta: "Hel"})
	if err != nil || len(lines) != 1 {
		t.Fatalf("encode delta: %d lines err %v", len(lines), err)
	}
	if !bytes.Contains(lines[0], []byte(`"content":"Hel"`)) {
		t.Errorf("delta line = %s", lines[0])
	}
	// done with usage -> usage chunk + [DONE]
	u := &Usage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3}
	lines, _ = e.Encode(Event{Type: EventDone, Finish: "stop", Usage: u})
	if len(lines) != 2 {
		t.Fatalf("done should produce 2 lines, got %d", len(lines))
	}
	if !bytes.Contains(lines[0], []byte(`"finish_reason":"stop"`)) || !bytes.Contains(lines[0], []byte(`"total_tokens":3`)) {
		t.Errorf("usage chunk = %s", lines[0])
	}
	if string(lines[1]) != "data: [DONE]\n\n" {
		t.Errorf("done line = %q", lines[1])
	}
}
```

> 测试里 `assistant.ToolCalls`(Task 3 修正过 types.go 已加字段)与 `Choice.ToolCalls`(Task 1 已有)。`bytes` import 需加。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/ir/ -v -run Chat
```

预期:编译失败(响应/流式函数未定义)。

- [ ] **Step 3: 实现 Chat 响应转换 + 流式**

在 `internal/ir/chat.go` 追加:

```go
// ---- 响应:上游 Chat JSON -> IR ----

type chatResponseRaw struct {
	ID      string            `json:"id"`
	Model   string            `json:"model"`
	Choices []chatChoiceRaw   `json:"choices"`
	Usage   chatUsageRaw      `json:"usage"`
}

type chatChoiceRaw struct {
	Index        int             `json:"index"`
	Message      chatMessageRaw  `json:"message"`
	FinishReason string          `json:"finish_reason"`
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
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Model   string         `json:"model"`
	Choices []chatChoiceOut `json:"choices"`
	Usage   chatUsageOut   `json:"usage"`
}

type chatChoiceOut struct {
	Index        int              `json:"index"`
	Message      chatMessageOut   `json:"message"`
	FinishReason string           `json:"finish_reason"`
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
		out.Usage.PromptDetails = &struct{ CachedTokens int `json:"cached_tokens"` }{r.Usage.CacheReadTokens}
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
		Index        int `json:"index"`
		Delta        struct {
			Content   string            `json:"content"`
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
			fin := *c.FinishReason
			ev := Event{Type: EventDone, Finish: fin}
			if chunk.Usage != nil {
				ev.Usage = &Usage{
					InputTokens: chunk.Usage.PromptTokens, OutputTokens: chunk.Usage.CompletionTokens,
					TotalTokens: chunk.Usage.TotalTokens, CacheReadTokens: chunk.Usage.PromptDetails.CachedTokens,
				}
			}
			evs = append(evs, ev)
		}
	}
	if chunk.Usage != nil {
		// 无 finish_reason 的独立 usage chunk(极少见)
		evs = append(evs, Event{Type: EventUsage, Usage: &Usage{
			InputTokens: chunk.Usage.PromptTokens, OutputTokens: chunk.Usage.CompletionTokens,
			TotalTokens: chunk.Usage.TotalTokens, CacheReadTokens: chunk.Usage.PromptDetails.CachedTokens,
		}})
	}
	return evs, nil
}

// ---- 流式:[]Event -> 上游 Chat SSE 行 ----

type ChatStreamEncoder struct {
	toolCallIndex int
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
		// 独立 usage 行(无 finish)
		chunk := map[string]any{
			"choices": []map[string]any{{"index": 0, "delta": map[string]any{}}},
			"usage":   usageToChat(ev.Usage),
		}
		return [][]byte{EncodeSSELine(mustJSON(chunk))}, nil
	case EventDone:
		chunk := map[string]any{
			"choices": []map[string]any{{
				"index":         0,
				"delta":         map[string]any{},
				"finish_reason": ev.Finish,
			}},
		}
		if ev.Usage != nil {
			chunk["usage"] = usageToChat(ev.Usage)
		}
		return [][]byte{
			EncodeSSELine(mustJSON(chunk)),
			EncodeSSELine([]byte("[DONE]")),
		}, nil
	}
	return nil, fmt.Errorf("unknown event type %d", ev.Type)
}

func (e *ChatStreamEncoder) Reset() { e.toolCallIndex = 0 }

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
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/ir/ -v -run Chat
```

预期:全部 Chat 测试(3 请求 + 6 响应/流式)PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/ir/chat.go internal/ir/chat_test.go
git commit -m "feat(ir): chat response conversion and streaming"
```

---

### Task 5: Responses 请求转换

**Files:**
- Create: `internal/ir/responses.go`(请求部分)
- Test: `internal/ir/responses_test.go`(请求部分)

**Interfaces:**
- Produces:

```go
// DecodeResponsesRequest 把 OpenAI Responses 请求 JSON 解码为 IR Request。
// instructions -> System;input 元素可以是 string 或 {role,content};
// content 可以是 string 或 [{type:input_text,text},{type:input_image,image_url}]。
// assistant 消息的 output_text/function_call items -> Messages/ToolCalls。
// max_output_tokens -> MaxTokens。
func DecodeResponsesRequest(body []byte) (*Request, error)

// EncodeResponsesRequest 把 IR Request 编码为 Responses 请求 JSON。
// System -> instructions;tool 消息 -> {type:function_call_output,call_id,output};
// tool_choice 映射;CacheReadTokens 无对应字段,忽略。
func EncodeResponsesRequest(r *Request) ([]byte, error)
```

- [ ] **Step 1: 写失败测试**

`internal/ir/responses_test.go`:

```go
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
		Model: "gpt-4o",
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
```

> `intPtr` 是包级 helper(多个测试文件可能都用,放 chat_test.go 或单独 helper;实现者放一处即可,重复定义会编译错)。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/ir/ -v -run Responses
```

预期:编译失败。

- [ ] **Step 3: 实现 Responses 请求转换**

`internal/ir/responses.go`:

```go
package ir

import (
	"encoding/json"
	"fmt"
)

// ---- 请求:下游 Responses JSON -> IR ----

type responsesRequestRaw struct {
	Model          string              `json:"model"`
	Instructions   string              `json:"instructions"`
	Input          json.RawMessage     `json:"input"`
	Tools          []responsesToolRaw  `json:"tools"`
	ToolChoice     json.RawMessage     `json:"tool_choice"`
	Stream         bool                `json:"stream"`
	Temperature    *float64            `json:"temperature"`
	TopP           *float64            `json:"top_p"`
	MaxOutputToks  *int                `json:"max_output_tokens"`
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
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/ir/ -v -run Responses
```

预期:4 个 Responses 请求测试 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/ir/responses.go internal/ir/responses_test.go
git commit -m "feat(ir): responses request decoder and encoder"
```

---

### Task 6: Responses 响应转换(非流式 + 流式)

**Files:**
- Modify: `internal/ir/responses.go`
- Modify: `internal/ir/responses_test.go`

**Interfaces:**
- Produces:

```go
// DecodeResponsesResponse 把 Responses 非流式响应 JSON 解码为 IR Response。
// output 数组:message item -> Choice;function_call item -> ToolCall(追加到上一个
// assistant Choice 或新建);usage.input_tokens_details.cached_tokens -> CacheReadTokens。
func DecodeResponsesResponse(body []byte) (*Response, error)

// EncodeResponsesResponse 把 IR Response 编码为 Responses 响应 JSON。
// 输出 output 数组(文本 message + function_call items)。
func EncodeResponsesResponse(r *Response) ([]byte, error)

// ResponsesStreamDecoder:按 SSE data 的事件类型分派:
//   response.output_text.delta  -> EventContentDelta
//   response.function_call_arguments.delta -> EventToolCallDelta
//   response.completed -> EventUsage + EventDone(Fin 取 response.status,通常 "completed")
// 其他事件类型(created/in_progress/output_item.done 等)忽略,返回空。
type ResponsesStreamDecoder struct{}

func (d *ResponsesStreamDecoder) DecodeLine(data []byte) ([]Event, error)

// ResponsesStreamEncoder:Event -> Responses SSE 行。有状态(工具调用 item id 生成)。
// EventDone 时输出 response.completed 事件(带 usage + 完整 output 骨架)。
type ResponsesStreamEncoder struct {
	toolItemID int
}

func (e *ResponsesStreamEncoder) Encode(ev Event) ([][]byte, error)
func (e *ResponsesStreamEncoder) Reset()
```

- [ ] **Step 1: 写失败测试**

`internal/ir/responses_test.go` 追加:

```go
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
			Role: "assistant",
			Content: []ContentPart{{Type: "text", Text: "hi"}},
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
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/ir/ -v -run Responses
```

预期:编译失败。

- [ ] **Step 3: 实现 Responses 响应转换 + 流式**

在 `internal/ir/responses.go` 追加:

```go
// ---- 响应:上游 Responses JSON -> IR ----

type responsesResponseRaw struct {
	ID      string            `json:"id"`
	Model   string            `json:"model"`
	Status  string            `json:"status"`
	Output  []responsesOutItem `json:"output"`
	Usage   responsesUsageRaw  `json:"usage"`
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
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	TotalTokens     int `json:"total_tokens"`
	InputDetails    struct {
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
			TotalTokens: v.Response.Usage.TotalTokens,
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
			"type": "response.function_call_arguments.delta",
			"item_id": fmt.Sprintf("fc_%d", e.toolItemID),
			"delta": ev.ToolCall.Arguments,
		}
		return [][]byte{EncodeSSELine(mustJSON(line))}, nil
	case EventDone:
		line := map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id": "resp_stream", "object": "response", "model": "",
				"status": "completed",
				"output": []any{},
				"usage": usageToResponses(ev.Usage),
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
```

> 注:`ResponsesStreamEncoder.Encode(EventDone)` 的 `response.completed` 事件按 OpenAI 流式规范可只带 usage/status,下游 SDK 能接受。模型字段留空可由代理层填(子项目 4)。可接受。

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/ir/ -v -run Responses
```

预期:全部 Responses 测试(4 请求 + 4 响应/流式)PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/ir/responses.go internal/ir/responses_test.go
git commit -m "feat(ir): responses response conversion and streaming"
```

---

### Task 7: Anthropic 请求转换

**Files:**
- Create: `internal/ir/anthropic.go`(请求部分)
- Test: `internal/ir/anthropic_test.go`(请求部分)

**Interfaces:**
- Produces:

```go
// DecodeAnthropicRequest 把 Anthropic Messages 请求 JSON 解码为 IR Request。
// system(string 或 [{type:text,text,cache_control}]) -> System;
// messages 的 content(string 或 blocks:text/tool_use/tool_result) -> Messages/ToolCalls;
// tools({name,description,input_schema,cache_control}) -> Tools;tool_choice 映射;
// max_tokens 必填;stop_sequences -> Stop。
func DecodeAnthropicRequest(body []byte) (*Request, error)

// EncodeAnthropicRequest 把 IR Request 编码为 Anthropic Messages 请求 JSON。
// System -> system(带 cache_control 时输出 blocks 形式);tool_choice 映射;
// CacheHints 映射到 tools/messages 的 cache_control。
func EncodeAnthropicRequest(r *Request) ([]byte, error)
```

- [ ] **Step 1: 写失败测试**

`internal/ir/anthropic_test.go`:

```go
package ir

import (
	"encoding/json"
	"testing"
)

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
	if toolMsg.Role != "tool" || toolMsg.ToolCallID != "tu_1" || toolMsg.Content[0].Text != "sunny" {
		t.Errorf("tool message = %+v", toolMsg)
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
		Model: "claude-3-5-sonnet-20241022",
		System: []ContentPart{{Type: "text", Text: "Be concise."}},
		Messages: []Message{
			{Role: "user", Content: []ContentPart{{Type: "text", Text: "hello"}}},
			{Role: "assistant", ToolCalls: []ToolCall{{ID: "tu_1", Type: "function", Name: "get_weather", Arguments: `{"city":"beijing"}`}}},
			{Role: "tool", ToolCallID: "tu_1", Content: []ContentPart{{Type: "text", Text: "sunny"}}},
		},
		MaxTokens: intPtr(100),
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
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/ir/ -v -run Anthropic
```

预期:编译失败。

- [ ] **Step 3: 实现 Anthropic 请求转换**

`internal/ir/anthropic.go`:

```go
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
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	InputSchema   json.RawMessage `json:"input_schema"`
	CacheControl  *anthropicCache `json:"cache_control"`
}

type anthropicCache struct {
	Type string `json:"type"`
}

type anthropicToolChoice struct {
	Type string `json:"type"` // auto / any / tool
	Name string `json:"name"`
}

type anthropicContentBlockRaw struct {
	Type        string          `json:"type"`
	Text        string          `json:"text"`
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Input       json.RawMessage `json:"input"`
	ToolUseID   string          `json:"tool_use_id"`
	Content     json.RawMessage `json:"content"` // tool_result 的内容(string 或 blocks)
	IsError     bool            `json:"is_error"`
	CacheControl *anthropicCache `json:"cache_control"`
}

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
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID: b.ID, Type: "function", Name: b.Name, Arguments: args,
			})
		case "tool_result":
			content, err := decodeToolResultContent(b.Content)
			if err != nil {
				return Message{}, err
			}
			msg.Content = append(msg.Content, ContentPart{
				Type: "tool_result", ToolUseID: b.ToolUseID, ToolResultContent: content, IsError: b.IsError,
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

var _ = bytes.Equal
```

> 注:`bytes` import 若未用则删。`decodeAnthropicMessage` 的 tool_result 分支把 tool_result 放在 Message.Content(与 Chat 的 tool 消息 content 不同——IR 里统一用 `ContentPart{Type:"tool_result"}`)。

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/ir/ -v -run Anthropic
```

预期:3 个 Anthropic 请求测试 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/ir/anthropic.go internal/ir/anthropic_test.go
git commit -m "feat(ir): anthropic request decoder and encoder"
```

---

### Task 8: Anthropic 响应转换(非流式 + 流式)

**Files:**
- Modify: `internal/ir/anthropic.go`
- Modify: `internal/ir/anthropic_test.go`

**Interfaces:**
- Produces:

```go
// DecodeAnthropicResponse 把 Anthropic 非流式响应 JSON 解码为 IR Response。
// content blocks(text/tool_use) -> Choice;stop_reason 映射
// (end_turn->stop, max_tokens->length, tool_use->tool_calls, stop_sequence->stop);
// usage 含 cache_creation_input_tokens/cache_read_input_tokens。
func DecodeAnthropicResponse(body []byte) (*Response, error)

// EncodeAnthropicResponse 把 IR Response 编码为 Anthropic 响应 JSON。
func EncodeAnthropicResponse(r *Response) ([]byte, error)

// AnthropicStreamDecoder:按事件类型分派:
//   message_start -> 记录 input_tokens/cache(暂存,不产事件)
//   content_block_delta(text_delta) -> EventContentDelta
//   content_block_delta(input_json_delta) -> EventToolCallDelta
//   message_delta -> 记录 output_tokens + stop_reason
//   message_stop -> EventUsage(汇总 input+output+cache) + EventDone
// 其他(content_block_start/stop 等)忽略。
type AnthropicStreamDecoder struct{ pending *Usage }

func (d *AnthropicStreamDecoder) DecodeLine(data []byte) ([]Event, error)

// AnthropicStreamEncoder:Event -> Anthropic SSE 行。有状态(content block index)。
// EventDone 时输出 message_delta(带 stop_reason+usage) + message_stop。
type AnthropicStreamEncoder struct {
	blockIndex int
}

func (e *AnthropicStreamEncoder) Encode(ev Event) ([][]byte, error)
func (e *AnthropicStreamEncoder) Reset()
```

- [ ] **Step 1: 写失败测试**

`internal/ir/anthropic_test.go` 追加:

```go
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
			Role: "assistant",
			Content: []ContentPart{{Type: "text", Text: "hi"}},
			ToolCalls: []ToolCall{{ID: "tu_1", Type: "function", Name: "get_weather", Arguments: `{"city":"beijing"}`}},
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
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/ir/ -v -run Anthropic
```

预期:编译失败。

- [ ] **Step 3: 实现 Anthropic 响应转换 + 流式**

在 `internal/ir/anthropic.go` 追加:

```go
// ---- 响应:上游 Anthropic JSON -> IR ----

type anthropicResponseRaw struct {
	ID         string                     `json:"id"`
	Model      string                     `json:"model"`
	Content    []anthropicContentBlockRaw `json:"content"`
	StopReason string                     `json:"stop_reason"`
	Usage      struct {
		InputTokens          int `json:"input_tokens"`
		OutputTokens         int `json:"output_tokens"`
		CacheCreationTokens  int `json:"cache_creation_input_tokens"`
		CacheReadTokens      int `json:"cache_read_input_tokens"`
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
			ch.ToolCalls = append(ch.ToolCalls, ToolCall{
				ID: b.ID, Type: "function", Name: b.Name, Arguments: string(b.Input),
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
		"cache_read_input_tokens": r.Usage.CacheReadTokens,
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
			InputTokens: v.Message.Usage.InputTokens,
			CacheCreationTokens: v.Message.Usage.CacheCreationTokens,
			CacheReadTokens: v.Message.Usage.CacheReadTokens,
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
		evs := []Event{{Type: EventDone, Finish: finish}}
		if d.pendingUsage != nil {
			evs = append(evs, Event{Type: EventUsage, Usage: d.pendingUsage})
		}
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
	blockIndex int
}

func (e *AnthropicStreamEncoder) Encode(ev Event) ([][]byte, error) {
	switch ev.Type {
	case EventContentDelta:
		line := map[string]any{
			"type": "content_block_delta",
			"index": e.blockIndex,
			"delta": map[string]any{"type": "text_delta", "text": ev.Delta},
		}
		return [][]byte{EncodeSSELine(mustJSON(line))}, nil
	case EventToolCallDelta:
		if ev.ToolCall == nil {
			return nil, fmt.Errorf("tool call delta without ToolCall")
		}
		line := map[string]any{
			"type": "content_block_delta",
			"index": e.blockIndex,
			"delta": map[string]any{"type": "input_json_delta", "partial_json": ev.ToolCall.Arguments},
		}
		e.blockIndex++
		return [][]byte{EncodeSSELine(mustJSON(line))}, nil
	case EventDone:
		stopReason := "end_turn"
		switch ev.Finish {
		case "length":
			stopReason = "max_tokens"
		case "tool_calls":
			stopReason = "tool_use"
		}
		usage := map[string]any{"output_tokens": 0}
		if ev.Usage != nil {
			usage = map[string]any{"output_tokens": ev.Usage.OutputTokens}
		}
		deltaLine := map[string]any{
			"type": "message_delta",
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

func (e *AnthropicStreamEncoder) Reset() { e.blockIndex = 0 }
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/ir/ -v -run Anthropic
```

预期:全部 Anthropic 测试(3 请求 + 4 响应/流式)PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/ir/anthropic.go internal/ir/anthropic_test.go
git commit -m "feat(ir): anthropic response conversion and streaming"
```

---

### Task 9: 错误 IR + 协议错误编码

**Files:**
- Create: `internal/ir/errors.go`
- Test: `internal/ir/errors_test.go`

**Interfaces:**
- Produces:

```go
// Error 是协议无关的错误 IR。Type 取以下枚举值:
//   invalid_request / authentication / permission / not_found / rate_limit
//   / internal / upstream / parse / quota / 2fa_required / user_disabled
type Error struct {
	Type       string
	Code       string
	Message    string
	StatusCode int
}

func (e *Error) Error() string

// NewError 便捷构造(StatusCode 默认按 Type 映射,可被调用方覆盖)。
func NewError(typ, code, message string, status int) *Error

// OpenAIErrorBody 把 Error 编码为 OpenAI 风格错误 JSON:
//   {"error": {"message": ..., "type": ..., "code": ...}}
func OpenAIErrorBody(e *Error) []byte

// AnthropicErrorBody 把 Error 编码为 Anthropic 风格错误 JSON:
//   {"type": "error", "error": {"type": ..., "message": ...}}
func AnthropicErrorBody(e *Error) []byte
```

Anthropic 错误类型映射(OpenAI Type -> Anthropic type):
`invalid_request->invalid_request_error`、`authentication->authentication_error`、`permission->permission_error`、`not_found->not_found_error`、`rate_limit->rate_limit_error`、`internal->api_error`、其余 -> `api_error`。

- [ ] **Step 1: 写失败测试**

`internal/ir/errors_test.go`:

```go
package ir

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestOpenAIErrorBody(t *testing.T) {
	e := NewError("invalid_request", "invalid_model", "model not found", 400)
	body := OpenAIErrorBody(e)
	var m struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Error.Message != "model not found" || m.Error.Type != "invalid_request" || m.Error.Code != "invalid_model" {
		t.Errorf("error = %+v", m.Error)
	}
}

func TestAnthropicErrorBody(t *testing.T) {
	e := NewError("rate_limit", "rate_limited", "too many requests", 429)
	body := AnthropicErrorBody(e)
	var m struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(body, &m)
	if m.Type != "error" || m.Error.Type != "rate_limit_error" || m.Error.Message != "too many requests" {
		t.Errorf("error = %+v", m)
	}
}

func TestNewErrorDefaultStatus(t *testing.T) {
	e := NewError("authentication", "invalid_api_key", "bad key", 0)
	if e.StatusCode == 0 {
		t.Error("expected default status for authentication")
	}
	// status 覆盖
	e2 := NewError("authentication", "invalid_api_key", "bad key", 403)
	if e2.StatusCode != 403 {
		t.Errorf("status = %d, want 403", e2.StatusCode)
	}
}

func TestErrorErrorMethod(t *testing.T) {
	e := NewError("internal", "err", "boom", 500)
	if e.Error() != "boom" {
		t.Errorf("Error() = %q", e.Error())
	}
}

func TestOpenAIErrorBodyMatchesExpected(t *testing.T) {
	e := NewError("not_found", "model_not_found", "The model 'gpt-x' does not exist", 404)
	got := OpenAIErrorBody(e)
	want := `{"error":{"message":"The model 'gpt-x' does not exist","type":"not_found","code":"model_not_found"}}`
	if !bytes.Equal(bytes.TrimSpace(got), []byte(want)) {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/ir/ -v -run Error
```

预期:编译失败。

- [ ] **Step 3: 实现 errors.go**

```go
package ir

import (
	"encoding/json"
)

type Error struct {
	Type       string
	Code       string
	Message    string
	StatusCode int
}

func (e *Error) Error() string { return e.Message }

func defaultStatus(typ string) int {
	switch typ {
	case "invalid_request":
		return 400
	case "authentication":
		return 401
	case "permission":
		return 403
	case "not_found":
		return 404
	case "rate_limit":
		return 429
	case "2fa_required", "user_disabled":
		return 403
	default:
		return 500
	}
}

func NewError(typ, code, message string, status int) *Error {
	if status == 0 {
		status = defaultStatus(typ)
	}
	return &Error{Type: typ, Code: code, Message: message, StatusCode: status}
}

func anthropicType(typ string) string {
	switch typ {
	case "invalid_request":
		return "invalid_request_error"
	case "authentication":
		return "authentication_error"
	case "permission":
		return "permission_error"
	case "not_found":
		return "not_found_error"
	case "rate_limit":
		return "rate_limit_error"
	default:
		return "api_error"
	}
}

func OpenAIErrorBody(e *Error) []byte {
	body, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"message": e.Message,
			"type":    e.Type,
			"code":    e.Code,
		},
	})
	return body
}

func AnthropicErrorBody(e *Error) []byte {
	body, _ := json.Marshal(map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    anthropicType(e.Type),
			"message": e.Message,
		},
	})
	return body
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/ir/ -v -run Error
```

预期:5 个错误测试 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/ir/errors.go internal/ir/errors_test.go
git commit -m "feat(ir): protocol-agnostic error IR with per-protocol encoders"
```

---

### Task 10: 9 组合矩阵集成测试 + fixture

**Files:**
- Create: `internal/ir/matrix_test.go`
- Create: `internal/ir/testdata/req_chat.json`
- Create: `internal/ir/testdata/req_responses.json`
- Create: `internal/ir/testdata/req_anthropic.json`
- Create: `internal/ir/testdata/resp_chat.json`
- Create: `internal/ir/testdata/resp_responses.json`
- Create: `internal/ir/testdata/resp_anthropic.json`
- Create: `internal/ir/testdata/stream_chat.jsonl`
- Create: `internal/ir/testdata/stream_responses.jsonl`
- Create: `internal/ir/testdata/stream_anthropic.jsonl`

**目的:** 验证"任何下游协议请求都能转成任何上游协议请求,任何上游协议响应都能转成任何下游协议响应"。用真实 fixture 跑全矩阵,防止转换器之间的隐式耦合被单元测试漏掉。

**Fixtures**(写文件):

`internal/ir/testdata/req_chat.json`:
```json
{
  "model": "gpt-4o",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "What's the weather in Beijing?"},
    {"role": "assistant", "content": null, "tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"beijing\"}"}}]},
    {"role": "tool", "tool_call_id": "call_1", "content": "sunny"}
  ],
  "tools": [{"type": "function", "function": {"name": "get_weather", "description": "Get weather for a city", "parameters": {"type": "object", "properties": {"city": {"type": "string"}}}}}],
  "tool_choice": "auto",
  "temperature": 0.7,
  "max_tokens": 100
}
```

`internal/ir/testdata/req_responses.json`:
```json
{
  "model": "gpt-4o",
  "instructions": "You are a helpful assistant.",
  "input": [
    {"role": "user", "content": [{"type": "input_text", "text": "What's the weather in Beijing?"}]},
    {"type": "function_call", "call_id": "call_1", "name": "get_weather", "arguments": "{\"city\":\"beijing\"}"},
    {"type": "function_call_output", "call_id": "call_1", "output": "sunny"}
  ],
  "tools": [{"type": "function", "name": "get_weather", "description": "Get weather for a city", "parameters": {"type": "object", "properties": {"city": {"type": "string"}}}}],
  "tool_choice": "auto",
  "temperature": 0.7,
  "max_output_tokens": 100
}
```

`internal/ir/testdata/req_anthropic.json`:
```json
{
  "model": "claude-3-5-sonnet-20241022",
  "system": "You are a helpful assistant.",
  "messages": [
    {"role": "user", "content": "What's the weather in Beijing?"},
    {"role": "assistant", "content": [{"type": "tool_use", "id": "tu_1", "name": "get_weather", "input": {"city": "beijing"}}]},
    {"role": "user", "content": [{"type": "tool_result", "tool_use_id": "tu_1", "content": [{"type": "text", "text": "sunny"}]}]}
  ],
  "tools": [{"name": "get_weather", "description": "Get weather for a city", "input_schema": {"type": "object", "properties": {"city": {"type": "string"}}}}],
  "tool_choice": {"type": "auto"},
  "temperature": 0.7,
  "max_tokens": 100
}
```

`internal/ir/testdata/resp_chat.json`:
```json
{
  "id": "chatcmpl-123",
  "object": "chat.completion",
  "model": "gpt-4o",
  "choices": [{"index": 0, "message": {"role": "assistant", "content": "The weather in Beijing is sunny.", "tool_calls": [{"id": "call_9", "type": "function", "function": {"name": "get_weather", "arguments": "{\"city\":\"beijing\"}"}}]}, "finish_reason": "tool_calls"}],
  "usage": {"prompt_tokens": 10, "completion_tokens": 15, "total_tokens": 25, "prompt_tokens_details": {"cached_tokens": 4}}
}
```

`internal/ir/testdata/resp_responses.json`:
```json
{
  "id": "resp_123",
  "object": "response",
  "model": "gpt-4o",
  "status": "completed",
  "output": [
    {"type": "message", "id": "msg_1", "role": "assistant", "content": [{"type": "output_text", "text": "The weather in Beijing is sunny.", "annotations": []}]},
    {"type": "function_call", "id": "fc_9", "call_id": "call_9", "name": "get_weather", "arguments": "{\"city\":\"beijing\"}"}
  ],
  "usage": {"input_tokens": 10, "output_tokens": 15, "total_tokens": 25, "input_tokens_details": {"cached_tokens": 4}}
}
```

`internal/ir/testdata/resp_anthropic.json`:
```json
{
  "id": "msg_123",
  "type": "message",
  "role": "assistant",
  "model": "claude-3-5-sonnet-20241022",
  "content": [
    {"type": "text", "text": "The weather in Beijing is sunny."},
    {"type": "tool_use", "id": "tu_9", "name": "get_weather", "input": {"city": "beijing"}}
  ],
  "stop_reason": "tool_use",
  "usage": {"input_tokens": 10, "output_tokens": 15, "cache_creation_input_tokens": 2, "cache_read_input_tokens": 3}
}
```

`internal/ir/testdata/stream_chat.jsonl`(每行一个 chunk;`[DONE]` 行是裸内容,不含 `data: ` 前缀——SSE 前缀剥离由代理层调用 `SplitSSE` 完成,decoder 直接吃裸负载):
```jsonl
{"id":"x","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"The weather "},"finish_reason":null}]}
{"id":"x","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"is sunny."},"finish_reason":null}]}
{"id":"x","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":15,"total_tokens":25,"prompt_tokens_details":{"cached_tokens":4}}}
[DONE]
```

`internal/ir/testdata/stream_responses.jsonl`:
```jsonl
{"type":"response.created","response":{"id":"resp_1","object":"response","status":"in_progress"}}
{"type":"response.output_text.delta","delta":"The weather "}
{"type":"response.output_text.delta","delta":"is sunny."}
{"type":"response.completed","response":{"id":"resp_1","object":"response","status":"completed","output":[],"usage":{"input_tokens":10,"output_tokens":15,"total_tokens":25,"input_tokens_details":{"cached_tokens":4}}}}
```

`internal/ir/testdata/stream_anthropic.jsonl`:
```jsonl
{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","usage":{"input_tokens":10,"cache_creation_input_tokens":2,"cache_read_input_tokens":3}}}
{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}
{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"The weather "}}
{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"is sunny."}}
{"type":"content_block_stop","index":0}
{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":15}}
{"type":"message_stop"}
```

**Matrix 测试**(`internal/ir/matrix_test.go`):

```go
package ir

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// testdata 读取 helper
func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return data
}

// 请求矩阵:三种下游协议都能转成三种上游协议
func TestRequestMatrix(t *testing.T) {
	type decodeFn func([]byte) (*Request, error)
	type encodeFn func(*Request) ([]byte, error)
	cases := []struct {
		name        string
		downstream  decodeFn
		upstream    encodeFn
		fixture     string
		checkModel  string
	}{
		{"chat->chat", DecodeChatRequest, EncodeChatRequest, "req_chat.json", "gpt-4o"},
		{"chat->responses", DecodeChatRequest, EncodeResponsesRequest, "req_chat.json", "gpt-4o"},
		{"chat->anthropic", DecodeChatRequest, EncodeAnthropicRequest, "req_chat.json", "gpt-4o"},
		{"responses->chat", DecodeResponsesRequest, EncodeChatRequest, "req_responses.json", "gpt-4o"},
		{"responses->responses", DecodeResponsesRequest, EncodeResponsesRequest, "req_responses.json", "gpt-4o"},
		{"responses->anthropic", DecodeResponsesRequest, EncodeAnthropicRequest, "req_responses.json", "gpt-4o"},
		{"anthropic->chat", DecodeAnthropicRequest, EncodeChatRequest, "req_anthropic.json", "claude-3-5-sonnet-20241022"},
		{"anthropic->responses", DecodeAnthropicRequest, EncodeResponsesRequest, "req_anthropic.json", "claude-3-5-sonnet-20241022"},
		{"anthropic->anthropic", DecodeAnthropicRequest, EncodeAnthropicRequest, "req_anthropic.json", "claude-3-5-sonnet-20241022"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := c.downstream(readTestdata(t, c.fixture))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if r.Model != c.checkModel {
				t.Errorf("model = %q, want %q", r.Model, c.checkModel)
			}
			if len(r.System) != 1 || r.System[0].Text != "You are a helpful assistant." {
				t.Errorf("system = %+v", r.System)
			}
			// 工具调用应在任一协议转换后保留
			foundToolCall := false
			for _, m := range r.Messages {
				if len(m.ToolCalls) > 0 {
					foundToolCall = true
				}
			}
			if !foundToolCall {
				t.Error("tool calls lost in conversion")
			}
			out, err := c.upstream(r)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if !json.Valid(out) {
				t.Errorf("output not valid JSON: %s", out)
			}
			// 上游 JSON 里应含 model
			var m map[string]any
			json.Unmarshal(out, &m)
			if m["model"] == nil {
				t.Errorf("output missing model: %s", out)
			}
		})
	}
}

// 响应矩阵:三种上游响应都能转成三种下游响应
func TestResponseMatrix(t *testing.T) {
	type decodeFn func([]byte) (*Response, error)
	type encodeFn func(*Response) ([]byte, error)
	cases := []struct {
		name      string
		upstream  decodeFn
		downstream encodeFn
		fixture   string
		checkText string
	}{
		{"chat->chat", DecodeChatResponse, EncodeChatResponse, "resp_chat.json", "The weather in Beijing is sunny."},
		{"chat->responses", DecodeChatResponse, EncodeResponsesResponse, "resp_chat.json", "The weather in Beijing is sunny."},
		{"chat->anthropic", DecodeChatResponse, EncodeAnthropicResponse, "resp_chat.json", "The weather in Beijing is sunny."},
		{"responses->chat", DecodeResponsesResponse, EncodeChatResponse, "resp_responses.json", "The weather in Beijing is sunny."},
		{"responses->responses", DecodeResponsesResponse, EncodeResponsesResponse, "resp_responses.json", "The weather in Beijing is sunny."},
		{"responses->anthropic", DecodeResponsesResponse, EncodeAnthropicResponse, "resp_responses.json", "The weather in Beijing is sunny."},
		{"anthropic->chat", DecodeAnthropicResponse, EncodeChatResponse, "resp_anthropic.json", "The weather in Beijing is sunny."},
		{"anthropic->responses", DecodeAnthropicResponse, EncodeResponsesResponse, "resp_anthropic.json", "The weather in Beijing is sunny."},
		{"anthropic->anthropic", DecodeAnthropicResponse, EncodeAnthropicResponse, "resp_anthropic.json", "The weather in Beijing is sunny."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := c.upstream(readTestdata(t, c.fixture))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			// 文本 + 工具调用都应保留
			if len(r.Choices) == 0 {
				t.Fatal("no choices")
			}
			foundText := false
			for _, ch := range r.Choices {
				for _, p := range ch.Content {
					if p.Text == c.checkText {
						foundText = true
					}
				}
				if len(ch.ToolCalls) > 0 {
					if ch.ToolCalls[0].Name != "get_weather" || ch.ToolCalls[0].Arguments != `{"city":"beijing"}` {
						t.Errorf("tool call = %+v", ch.ToolCalls[0])
					}
				}
			}
			if !foundText {
				t.Error("text lost in conversion")
			}
			out, err := c.downstream(r)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if !json.Valid(out) {
				t.Errorf("output not valid JSON: %s", out)
			}
		})
	}
}

// 流式矩阵:上游流式事件 -> 统一事件流 -> 下游流式事件
func TestStreamMatrix(t *testing.T) {
	type decodeLine func([]byte) ([]Event, error)
	cases := []struct {
		name    string
		decoder decodeLine
		encoder resetEncoder
		fixture string
	}{
		{"chat->chat", (&ChatStreamDecoder{}).DecodeLine, &ChatStreamEncoder{}, "stream_chat.jsonl"},
		{"responses->responses", (&ResponsesStreamDecoder{}).DecodeLine, &ResponsesStreamEncoder{}, "stream_responses.jsonl"},
		{"anthropic->anthropic", (&AnthropicStreamDecoder{}).DecodeLine, &AnthropicStreamEncoder{}, "stream_anthropic.jsonl"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			lines, err := readTestdataLines(t, c.fixture)
			if err != nil {
				t.Fatalf("read lines: %v", err)
			}
			var events []Event
			for _, ln := range lines {
				evs, err := c.decoder(ln)
				if err != nil {
					t.Fatalf("decode line %q: %v", ln, err)
				}
				events = append(events, evs...)
			}
			// 应收到至少一个 content delta + 一个 done
			hasContent, hasDone := false, false
			for _, ev := range events {
				if ev.Type == EventContentDelta {
					hasContent = true
				}
				if ev.Type == EventDone {
					hasDone = true
				}
			}
			if !hasContent || !hasDone {
				t.Fatalf("events missing content/done: %+v", events)
			}
			// 编码回下游
			outLines, err := encodeAll(c.encoder, events)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if len(outLines) == 0 {
				t.Error("no encoded lines")
			}
		})
	}
}

type resetEncoder interface {
	Encode(ev Event) ([][]byte, error)
	Reset()
}

func encodeAll(e resetEncoder, events []Event) ([][]byte, error) {
	var out [][]byte
	for _, ev := range events {
		lines, err := e.Encode(ev)
		if err != nil {
			return nil, err
		}
		out = append(out, lines...)
	}
	return out, nil
}

func readTestdataLines(t *testing.T, name string) ([][]byte, error) {
	data := readTestdata(t, name)
	var lines [][]byte
	for _, ln := range splitLines(data) {
		if len(ln) > 0 {
			lines = append(lines, ln)
		}
	}
	return lines, nil
}

func splitLines(data []byte) [][]byte {
	var out [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			out = append(out, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		out = append(out, data[start:])
	}
	return out
}
```

> 注:`ChatStreamDecoder.DecodeLine` 的签名是 `func([]byte) ([]Event, error)`——测试里的 `(&ChatStreamDecoder{}).DecodeLine` 直接取方法值,类型匹配。`encoder` 字段直接放 `&ChatStreamEncoder{}` 实例(实现 `resetEncoder` 接口),`encodeAll` 接收该接口。三个 Encoder 都实现了 `Encode(Event) ([][]byte, error)` + `Reset()`,满足接口。

- [ ] **Step 1: 写 fixture 文件**

按上方内容创建 9 个 fixture(stream 文件注意每行一个事件,`data: [DONE]` 是 SSE 原始行,decoder 收到的是 `[DONE]` 本身——`readTestdataLines` 读到的是去掉 `data: ` 前缀的裸 `[DONE]`?**不对**。fixture 的 stream_chat.jsonl 里写了 `data: [DONE]`,但 `readTestdataLines` 会把它当一行传给 decoder。`ChatStreamDecoder.DecodeLine([]byte("data: [DONE]"))` 会因 `string(data) == "[DONE]"` 为假而走 JSON 解析失败。

**修正**:stream fixture 中 `data: [DONE]` 行改为裸 `[DONE]`(测试直接喂裸内容,不含 SSE 前缀;SSE 前缀处理由代理层调用 `SplitSSE` 完成)。实现者注意:jsonl fixture 里的 `[DONE]` 行必须是裸 `[DONE]`,不带 `data: `。

- [ ] **Step 2: 运行测试确认失败/通过**

```bash
go test ./internal/ir/ -v -run Matrix
```

预期:9 个请求矩阵 + 9 个响应矩阵 + 3 个流式矩阵 subtests 全部 PASS。若个别组合失败(如 Chat tool 消息转 Anthropic 的 tool_result 结构、Responses 的 function_call_output),是转换器 bug,按失败信息修正转换器代码(这是本任务的真正价值——矩阵测试抓跨协议 bug)。

- [ ] **Step 3: 全量测试**

```bash
go test ./... -count=1
```

预期:全部包 PASS。

- [ ] **Step 4: 提交**

```bash
git add internal/ir/matrix_test.go internal/ir/testdata/
git commit -m "test(ir): 9-way conversion matrix with protocol fixtures"
```

---

## 子项目 3 完成标准

- [ ] `go test ./...` 全绿(新增 ir 包约 35+ 测试,全部包合计 120+)
- [ ] 9 组合请求矩阵 + 9 组合响应矩阵 + 3 流式矩阵全过
- [ ] 工具调用、多模态、缓存 token、finish_reason 在跨协议转换中不丢失
- [ ] 流式:统一事件流能在三种协议间双向转换,末尾 usage 提取正确
- [ ] 错误 IR 能编码成 OpenAI / Anthropic 两种错误格式
- [ ] 无 CGO;纯标准库
