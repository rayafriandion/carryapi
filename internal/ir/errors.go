package ir

import "encoding/json"

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

// OpenAIErrorBody encodes e as an OpenAI-style error JSON body:
// {"error": {"message": ..., "type": ..., "code": ...}}.
// A struct (not a map) is used so field order is deterministic.
func OpenAIErrorBody(e *Error) []byte {
	var body struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	body.Error.Message = e.Message
	body.Error.Type = e.Type
	body.Error.Code = e.Code
	out, _ := json.Marshal(body)
	return out
}

// AnthropicErrorBody encodes e as an Anthropic-style error JSON body:
// {"type": "error", "error": {"type": ..., "message": ...}}.
// A struct (not a map) is used so field order is deterministic.
func AnthropicErrorBody(e *Error) []byte {
	var body struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	body.Type = "error"
	body.Error.Type = anthropicType(e.Type)
	body.Error.Message = e.Message
	out, _ := json.Marshal(body)
	return out
}
