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
		name       string
		downstream decodeFn
		upstream   encodeFn
		fixture    string
		checkModel string
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
		name       string
		upstream   decodeFn
		downstream encodeFn
		fixture    string
		checkText  string
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
