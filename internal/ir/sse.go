package ir

import (
	"bytes"
	"encoding/json"
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
			// 多行 data 直接拼接(用于重组被拆分的 JSON 片段)
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
