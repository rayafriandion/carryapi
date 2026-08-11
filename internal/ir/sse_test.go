package ir

import (
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
	body := []byte("data: {\"a\":1\ndata: ,\"b\":2}\n\n")
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
