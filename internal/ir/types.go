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
	ToolCalls  []ToolCall // assistant 消息的工具调用(Chat/Responses)
	ToolCallID string     // role=tool 时关联的 tool_use/call id
	Name       string     // role=tool 时的函数名(Anthropic 无,置空)
}

// ContentPart 是多模态/工具内容单元。
type ContentPart struct {
	Type     string // text / image_url / tool_use / tool_result
	Text     string
	ImageURL string
	// tool_use
	ToolUseID string
	ToolName  string
	ToolInput json.RawMessage // 工具入参,JSON 对象(原样保留)
	// tool_result
	ToolResultContent []ContentPart // 工具返回(可嵌套 text/image)
	IsError           bool
}

// Tool 是统一工具定义。
type Tool struct {
	Type         string // 恒为 "function"(Anthropic 无 type,编码时省略)
	Name         string
	Description  string
	Parameters   json.RawMessage // JSON Schema 对象
	CacheControl *CacheControl   // 仅 Anthropic
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
	EventContentDelta  EventType = iota // 文本增量
	EventToolCallDelta                  // 工具参数增量
	EventUsage                          // 用量(末尾)
	EventDone                           // 结束
)

// Event 是统一流式事件。
type Event struct {
	Type     EventType
	Delta    string    // 文本或工具参数增量
	ToolCall *ToolCall // 增量(部分 Arguments)
	Usage    *Usage
	Finish   string // EventDone 时的 finish_reason
}
