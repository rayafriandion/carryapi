package proxy

import (
	"net/http"
	"strings"

	"carryapi/internal/apikey"
	"carryapi/internal/ir"
	"carryapi/internal/user"
)

// authenticate 提取 API Key 并校验。
func (p *Proxy) authenticate(r *http.Request) (*user.User, *apikey.APIKey, error) {
	key := extractAPIKey(r)
	if key == "" {
		return nil, nil, ir.NewError("authentication", "invalid_api_key", "missing api key", 401)
	}
	userID, keyID, err := p.deps.Keys.Authenticate(key)
	if err != nil {
		return nil, nil, ir.NewError("authentication", "invalid_api_key", "invalid api key", 401)
	}
	u, err := p.deps.Users.GetByID(userID)
	if err != nil || u.Status != "active" {
		return nil, nil, ir.NewError("user_disabled", "user_disabled", "user is disabled", 403)
	}
	ak, err := p.deps.Keys.Get(keyID, userID)
	if err != nil {
		return nil, nil, ir.NewError("authentication", "invalid_api_key", "invalid api key", 401)
	}
	return &u, &ak, nil
}

func extractAPIKey(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if strings.HasPrefix(h, "Bearer ") {
			return strings.TrimPrefix(h, "Bearer ")
		}
		return h
	}
	if h := r.Header.Get("x-api-key"); h != "" {
		return h
	}
	return ""
}
