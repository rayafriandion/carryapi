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
		p.writeError(w, rc, ir.NewError("upstream", "upstream_error", fmt.Sprintf("upstream returned %d: %s", upResp.StatusCode, truncate(body, 200)), 502))
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
		// 处理单条 data 行(每个事件块可能有多行 data,简化:逐行喂 decoder)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(line[len("data:"):])
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
	p.recordStats(rc)
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

// splitSSERecords 是 bufio.SplitFunc:按 "\n\n" 分割 SSE 事件。
func splitSSERecords(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i := 0; i+1 < len(data); i++ {
		if data[i] == '\n' && data[i+1] == '\n' {
			return i + 2, data[:i], nil
		}
	}
	if atEOF {
		if len(data) == 0 {
			return 0, nil, nil
		}
		return len(data), data, nil
	}
	return 0, nil, nil
}
