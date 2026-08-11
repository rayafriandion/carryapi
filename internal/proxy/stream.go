package proxy

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"

	"carryapi/internal/ir"
)

// streamDecoder 统一上游流式解码接口。
type streamDecoder interface {
	DecodeLine([]byte) ([]ir.Event, error)
}

// streamEncoder 统一下游流式编码接口。
type streamEncoder interface {
	Encode(ir.Event) ([][]byte, error)
	Reset()
}

// streamResponse 流式转发:上游 SSE -> 统一事件 -> 下游 SSE。
func (p *Proxy) streamResponse(w http.ResponseWriter, rc *requestContext, upResp *http.Response, downstream string) {
	if upResp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(upResp.Body, 4096))
		e := upstreamErrorFromStatus(upResp.StatusCode, upstreamErrorMessage(rc.provider.Protocol, body))
		p.writeError(w, rc, e)
		return
	}
	// 上游 Decoder(按 provider.Protocol)
	decoder, err := newUpstreamStreamDecoder(rc.provider.Protocol)
	if err != nil {
		p.writeError(w, rc, ir.NewError("internal", "stream_decode_failed", err.Error(), 500))
		return
	}
	// 下游 Encoder(按 downstream)
	encoder, err := newDownstreamStreamEncoder(downstream)
	if err != nil {
		p.writeError(w, rc, ir.NewError("internal", "stream_encode_failed", err.Error(), 500))
		return
	}
	encoder.Reset()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(200)
	rc.statusCode = 200

	flusher, _ := w.(http.Flusher)
	scanner := bufio.NewScanner(upResp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	scanner.Split(splitSSERecords) // 按空行分割(SSE 事件)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		// 每个事件块可能含 event:/data: 多行;提取 data 负载后统一解码。
		payload := extractDataPayload(line)
		if len(payload) == 0 {
			continue
		}
		events, err := decoder.DecodeLine(payload)
		if err != nil {
			// 忽略解析失败的行,继续(流式容错)
			continue
		}
		for _, ev := range events {
			p.collectEvent(rc, ev)
			outLines, err := encoder.Encode(ev)
			if err != nil {
				continue
			}
			for _, ol := range outLines {
				w.Write(ol)
			}
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
	// 流中途失败(scanner 错误,如上游提前断开/超长行):记录错误类型但不改已写头的状态码。
	if err := scanner.Err(); err != nil {
		rc.errorType = "upstream"
		rc.errorMessage = "stream read error: " + err.Error()
	}
	p.recordStats(rc)
}

// extractDataPayload 从 SSE 事件块中提取所有 data: 负载并拼接。
// 忽略注释(:)行与 event: 事件名行;多条 data: 直接拼接(与 ir.SplitSSE 一致,用于重组被拆分的 JSON 片段)。
func extractDataPayload(block []byte) []byte {
	var payload []byte
	for _, line := range bytes.Split(block, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] == ':' {
			continue // 注释
		}
		if bytes.HasPrefix(line, []byte("event:")) {
			continue // 事件名(ir decoder 按 JSON type 分派,不需要)
		}
		if bytes.HasPrefix(line, []byte("data:")) {
			payload = append(payload, bytes.TrimSpace(line[len("data:"):])...)
		}
	}
	return payload
}

// collectEvent 从统一事件提取用量(统计用)。
func (p *Proxy) collectEvent(rc *requestContext, ev ir.Event) {
	if ev.Usage == nil {
		return
	}
	rc.inputTokens = ev.Usage.InputTokens
	rc.outputTokens = ev.Usage.OutputTokens
	rc.cacheRead = ev.Usage.CacheReadTokens
	rc.cacheCreation = ev.Usage.CacheCreationTokens
}

func newUpstreamStreamDecoder(protocol string) (streamDecoder, error) {
	switch protocol {
	case "openai_chat":
		return &ir.ChatStreamDecoder{}, nil
	case "openai_responses":
		return &ir.ResponsesStreamDecoder{}, nil
	case "anthropic":
		return &ir.AnthropicStreamDecoder{}, nil
	}
	return nil, fmt.Errorf("unknown upstream protocol %q", protocol)
}

func newDownstreamStreamEncoder(downstream string) (streamEncoder, error) {
	switch downstream {
	case "chat":
		return &ir.ChatStreamEncoder{}, nil
	case "responses":
		return &ir.ResponsesStreamEncoder{}, nil
	case "anthropic":
		return &ir.AnthropicStreamEncoder{}, nil
	}
	return nil, fmt.Errorf("unknown downstream protocol %q", downstream)
}

// splitSSERecords 是 bufio.SplitFunc:按空行("\n\n" 或 "\r\n\r\n")分割 SSE 事件。
func splitSSERecords(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i := 0; i < len(data); i++ {
		if i+1 < len(data) && data[i] == '\n' && data[i+1] == '\n' {
			return i + 2, data[:i], nil
		}
		// CRLFCRLF
		if i+3 < len(data) && data[i] == '\r' && data[i+1] == '\n' && data[i+2] == '\r' && data[i+3] == '\n' {
			return i + 4, data[:i], nil
		}
	}
	if atEOF {
		if len(data) > 0 {
			return len(data), data, nil
		}
		return 0, nil, nil
	}
	return 0, nil, nil
}
