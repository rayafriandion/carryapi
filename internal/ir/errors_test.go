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
