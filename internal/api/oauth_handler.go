package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"carryapi/internal/auth"
	"carryapi/internal/oauth"
	"carryapi/internal/settings"
	"carryapi/internal/user"
)

// OAuthHandler drives the OAuth login flow: it begins the redirect to a
// provider's AuthURL and handles the callback (state check, token exchange,
// user-id fetch, auth-method lookup, session creation).
type OAuthHandler struct {
	providers map[string]oauth.Provider
	users     *user.Store
	sessions  *auth.SessionStore
	settings  *settings.Store
}

// NewOAuthHandler constructs an OAuthHandler and loads any providers whose
// client id/secret/redirect are present in settings.
func NewOAuthHandler(users *user.Store, sessions *auth.SessionStore, settings *settings.Store) *OAuthHandler {
	h := &OAuthHandler{providers: map[string]oauth.Provider{}, users: users, sessions: sessions, settings: settings}
	// 从 settings 读 client id/secret 初始化(若已配置)
	h.loadProviders()
	return h
}

// loadProviders reads oauth_* settings and registers Discord/X when fully
// configured. A provider is only added when all three of its settings
// (client_id, client_secret, redirect_url) are present and non-empty.
func (h *OAuthHandler) loadProviders() {
	// Discord
	dID, idOK, _ := h.settings.Get("oauth_discord_client_id")
	dSecret, secretOK, _ := h.settings.Get("oauth_discord_client_secret")
	dRedirect, redirOK, _ := h.settings.Get("oauth_discord_redirect_url")
	if idOK && secretOK && redirOK && dID != "" && dSecret != "" && dRedirect != "" {
		h.providers["discord"] = oauth.NewDiscord(dID, dSecret, dRedirect)
	}
	// X
	xID, xIDOK, _ := h.settings.Get("oauth_x_client_id")
	xSecret, xSecretOK, _ := h.settings.Get("oauth_x_client_secret")
	xRedirect, xRedirOK, _ := h.settings.Get("oauth_x_redirect_url")
	if xIDOK && xSecretOK && xRedirOK && xID != "" && xSecret != "" && xRedirect != "" {
		h.providers["x"] = oauth.NewX(xID, xSecret, xRedirect)
	}
}

// RegisterProvider adds a provider directly (used by tests to inject a mock
// provider without configuring settings).
func (h *OAuthHandler) RegisterProvider(p oauth.Provider) {
	h.providers[p.Name()] = p
}

// Begin generates a random state, stores it in a short-lived cookie, and
// redirects the user to the provider's authorization URL.
func (h *OAuthHandler) Begin(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	p, ok := h.providers[provider]
	if !ok {
		JSONError(w, 400, "unknown provider")
		return
	}
	state := randomHex(16)
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", Value: state, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 600})
	http.Redirect(w, r, p.AuthURL(state), http.StatusTemporaryRedirect)
}

// Callback validates the state cookie, exchanges the code for a token, fetches
// the provider user id, and either establishes a session (already bound) or
// creates+binds a new user when registration is open.
func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	p, ok := h.providers[provider]
	if !ok {
		JSONError(w, 400, "unknown provider")
		return
	}
	// 校验 state
	cookie, err := r.Cookie("oauth_state")
	if err != nil || cookie.Value != r.URL.Query().Get("state") {
		JSONError(w, 400, "invalid state")
		return
	}
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	tok, err := p.Exchange(r.Context(), code, state)
	if err != nil {
		JSONError(w, 400, "token exchange failed")
		return
	}
	uid, err := p.FetchUserID(r.Context(), tok)
	if err != nil {
		JSONError(w, 400, "failed to fetch user id")
		return
	}
	// 查 auth_methods:已绑定则建 session
	m, err := h.users.GetAuthMethod(p.Name(), uid)
	if err == nil {
		sess, err := h.sessions.Create(m.UserID, 7*24*time.Hour, "", "")
		if err != nil {
			JSONError(w, 500, "session create failed")
			return
		}
		setSessionCookie(w, sess.Token) // 复用 AuthHandler 的(提取公共)
		setCSRFCookie(w)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	// 未绑定
	open, _, _ := h.settings.Get("registration_open")
	if open == "false" {
		JSONError(w, 403, "not linked; login first to bind")
		return
	}
	// 注册开 -> 创建用户(无密码)+ 绑定
	u, err := h.users.Create(p.Name()+"-"+uid, "", "user")
	if err != nil {
		JSONError(w, 400, "failed to create user")
		return
	}
	if err := h.users.AddAuthMethod(u.ID, p.Name(), uid, nil); err != nil {
		JSONError(w, 500, "failed to bind oauth account")
		return
	}
	sess, err := h.sessions.Create(u.ID, 7*24*time.Hour, "", "")
	if err != nil {
		JSONError(w, 500, "session create failed")
		return
	}
	setSessionCookie(w, sess.Token)
	setCSRFCookie(w)
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
