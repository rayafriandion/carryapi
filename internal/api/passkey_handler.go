package api

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"carryapi/internal/auth"
	"carryapi/internal/middleware"
	"carryapi/internal/user"
	"carryapi/internal/webauthn"
)

// PasskeyHandler drives the WebAuthn (passkey) registration and login flows.
// Registration requires an authenticated user (binds a new credential to the
// account); login is public (client supplies the email to discover the user +
// their existing passkey credentials).
type PasskeyHandler struct {
	svc      *webauthn.Service
	users    *user.Store
	sessions *auth.SessionStore
}

// NewPasskeyHandler constructs a PasskeyHandler.
func NewPasskeyHandler(svc *webauthn.Service, users *user.Store, sessions *auth.SessionStore) *PasskeyHandler {
	return &PasskeyHandler{svc: svc, users: users, sessions: sessions}
}

// loadWebAuthnUser loads a user's existing passkey credentials from auth_methods
// (provider="passkey", secret=JSON-encoded webauthn.Credential) and adapts the
// user.User into a webauthn.LocalUser.
func (h *PasskeyHandler) loadWebAuthnUser(u *user.User) *webauthn.LocalUser {
	methods, _ := h.users.GetAuthMethods(u.ID)
	var creds []webauthn.Credential
	for _, m := range methods {
		if m.Provider == "passkey" {
			var c webauthn.Credential
			if json.Unmarshal(m.Secret, &c) == nil {
				creds = append(creds, c)
			}
		}
	}
	return &webauthn.LocalUser{ID: u.ID, Email: u.Email, Credentials: creds}
}

// RegisterBegin starts a WebAuthn registration ceremony for the logged-in user.
// Returns the publicKey creation options + a session_key the client must echo
// back to RegisterFinish.
func (h *PasskeyHandler) RegisterBegin(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		JSONError(w, 401, "unauthorized")
		return
	}
	wu := h.loadWebAuthnUser(u)
	creation, sessionKey, err := h.svc.BeginRegistration(wu)
	if err != nil {
		JSONError(w, 500, "begin registration failed")
		return
	}
	JSON(w, 200, map[string]any{
		"publicKey":   creation,
		"session_key": sessionKey, // 客户端在 finish 时回传
	})
}

// RegisterFinish completes a registration ceremony, verifies the authenticator
// response, and stores the credential as an auth_method (provider="passkey",
// provider_uid=hex(cred.ID), secret=JSON(cred)).
func (h *PasskeyHandler) RegisterFinish(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		JSONError(w, 401, "unauthorized")
		return
	}
	sessionKey := r.URL.Query().Get("session_key")
	if sessionKey == "" {
		JSONError(w, 400, "missing session_key")
		return
	}
	wu := h.loadWebAuthnUser(u)
	cred, err := h.svc.FinishRegistration(wu, sessionKey, r)
	if err != nil {
		JSONError(w, 400, "finish registration failed: "+err.Error())
		return
	}
	// 存 auth_methods:provider=passkey, provider_uid=credentialID hex, secret=credential JSON(加密)
	credJSON, _ := json.Marshal(cred)
	if err := h.users.AddAuthMethod(u.ID, "passkey", hex.EncodeToString(cred.ID), credJSON); err != nil {
		JSONError(w, 500, "failed to store credential")
		return
	}
	JSON(w, 200, map[string]string{"status": "ok"})
}

// LoginBegin starts a WebAuthn login ceremony. No session required: the client
// supplies the user's email so the server can look up the account + its existing
// passkey credentials.
func (h *PasskeyHandler) LoginBegin(w http.ResponseWriter, r *http.Request) {
	// 无登录态:客户端传 email 找用户
	email := r.URL.Query().Get("email")
	if email == "" {
		JSONError(w, 400, "missing email")
		return
	}
	u, err := h.users.GetByEmail(email)
	if err != nil {
		JSONError(w, 401, "user not found")
		return
	}
	wu := h.loadWebAuthnUser(&u)
	if len(wu.Credentials) == 0 {
		JSONError(w, 400, "user has no passkey credentials")
		return
	}
	assertion, sessionKey, err := h.svc.BeginLogin(wu)
	if err != nil {
		JSONError(w, 500, "begin login failed")
		return
	}
	JSON(w, 200, map[string]any{
		"publicKey":   assertion,
		"session_key": sessionKey,
	})
}

// LoginFinish completes a login ceremony, verifies the authenticator assertion,
// and establishes a session (sets session + CSRF cookies).
func (h *PasskeyHandler) LoginFinish(w http.ResponseWriter, r *http.Request) {
	sessionKey := r.URL.Query().Get("session_key")
	email := r.URL.Query().Get("email")
	if sessionKey == "" || email == "" {
		JSONError(w, 400, "missing session_key or email")
		return
	}
	u, err := h.users.GetByEmail(email)
	if err != nil {
		JSONError(w, 401, "user not found")
		return
	}
	wu := h.loadWebAuthnUser(&u)
	cred, err := h.svc.FinishLogin(wu, sessionKey, r)
	if err != nil {
		JSONError(w, 401, "finish login failed: "+err.Error())
		return
	}
	// 持久化更新后的 credential(含递增的 sign counter),否则 go-webauthn
	// 无法在后续登录时做 replay/clone 检测。
	if cred != nil {
		providerUID := hex.EncodeToString(cred.ID)
		methods, _ := h.users.GetAuthMethods(u.ID)
		for _, m := range methods {
			if m.Provider == "passkey" && m.ProviderUID == providerUID {
				credJSON, _ := json.Marshal(cred)
				if err := h.users.UpdateAuthMethodSecret(m.ID, u.ID, credJSON); err != nil {
					JSONError(w, 500, "failed to persist credential")
					return
				}
				break
			}
		}
	}
	sess, err := h.sessions.Create(u.ID, 7*24*time.Hour, "", "")
	if err != nil {
		JSONError(w, 500, "session create failed")
		return
	}
	setSessionCookie(w, sess.Token)
	setCSRFCookie(w)
	JSON(w, 200, map[string]any{
		"user": map[string]any{"id": u.ID, "email": u.Email, "role": u.Role},
	})
}
