package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"carryapi/internal/auth"
	"carryapi/internal/middleware"
	"carryapi/internal/settings"
	"carryapi/internal/user"
)

type AuthHandler struct {
	ls       *auth.LoginService
	sessions *auth.SessionStore
	users    *user.Store
	settings *settings.Store
}

func NewAuthHandler(ls *auth.LoginService, sessions *auth.SessionStore, users *user.Store, settings *settings.Store) *AuthHandler {
	return &AuthHandler{ls: ls, sessions: sessions, users: users, settings: settings}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Code     string `json:"code"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, 400, "invalid request body")
		return
	}
	sess, requires2FA, err := h.ls.Login(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrUserDisabled) {
			JSONError(w, 403, "user disabled")
			return
		}
		JSONError(w, 401, "invalid credentials")
		return
	}
	if requires2FA {
		JSON(w, 200, map[string]any{"requires_2fa": true})
		return
	}
	setSessionCookie(w, sess.Token)
	setCSRFCookie(w)
	u, _ := h.users.GetByID(sess.UserID)
	JSON(w, 200, map[string]any{
		"user": map[string]any{"id": u.ID, "email": u.Email, "role": u.Role},
	})
}

func (h *AuthHandler) Complete2FA(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, 400, "invalid request body")
		return
	}
	sess, err := h.ls.Complete2FA(req.Email, req.Code)
	if err != nil {
		JSONError(w, 401, "2fa verification failed")
		return
	}
	setSessionCookie(w, sess.Token)
	setCSRFCookie(w)
	u, _ := h.users.GetByID(sess.UserID)
	JSON(w, 200, map[string]any{
		"user": map[string]any{"id": u.ID, "email": u.Email, "role": u.Role},
	})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, 400, "invalid request body")
		return
	}
	u, err := h.ls.Register(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrRegistrationClosed) {
			JSONError(w, 403, "registration closed")
			return
		}
		JSONError(w, 400, err.Error())
		return
	}
	JSON(w, 200, map[string]any{"id": u.ID, "email": u.Email, "role": u.Role})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
		h.sessions.Revoke(cookie.Value)
	}
	// 清 cookie
	http.SetCookie(w, &http.Cookie{Name: auth.SessionCookieName, Value: "", MaxAge: -1, Path: "/"})
	http.SetCookie(w, &http.Cookie{Name: middleware.CSRFCookie, Value: "", MaxAge: -1, Path: "/"})
	JSON(w, 200, map[string]string{"status": "ok"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		JSONError(w, 401, "unauthorized")
		return
	}
	methods, _ := h.users.GetAuthMethods(u.ID)
	providers := []string{}
	for _, m := range methods {
		providers = append(providers, m.Provider)
	}
	JSON(w, 200, map[string]any{
		"user":         map[string]any{"id": u.ID, "email": u.Email, "role": u.Role, "status": u.Status},
		"auth_methods": providers,
	})
}

func (h *AuthHandler) Setup2FA(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		JSONError(w, 401, "unauthorized")
		return
	}
	secret, url, err := auth.GenerateTOTPSecret(u.Email)
	if err != nil {
		JSONError(w, 500, "failed to generate totp")
		return
	}
	backupCodes := auth.GenerateBackupCodes()
	hashedCodes := make([]string, len(backupCodes))
	for i, c := range backupCodes {
		hashedCodes[i] = auth.HashBackupCode(c)
	}
	// 存 totp secret + 备份码哈希
	h.users.AddAuthMethod(u.ID, "totp", "", []byte(secret))
	// 备份码:简化存为单条 auth_method,secret=JSON 哈希数组(加密)
	hashesJSON, _ := json.Marshal(hashedCodes)
	h.users.AddAuthMethod(u.ID, "totp_backup", "", hashesJSON)
	JSON(w, 200, map[string]any{
		"secret":       secret,
		"otpauth_url":  url,
		"backup_codes": backupCodes,
	})
}

func (h *AuthHandler) Disable2FA(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		JSONError(w, 401, "unauthorized")
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			JSONError(w, 401, "password required")
			return
		}
		JSONError(w, 400, "invalid body")
		return
	}
	if req.Password == "" {
		JSONError(w, 401, "password required")
		return
	}
	if !auth.VerifyPassword(req.Password, u.PasswordHash) {
		JSONError(w, 401, "invalid password")
		return
	}
	methods, _ := h.users.GetAuthMethods(u.ID)
	for _, m := range methods {
		if m.Provider == "totp" || m.Provider == "totp_backup" {
			h.users.DeleteAuthMethod(m.ID, u.ID)
		}
	}
	JSON(w, 200, map[string]string{"status": "ok"})
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})
}

func setCSRFCookie(w http.ResponseWriter) {
	b := make([]byte, 16)
	rand.Read(b)
	token := hex.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.CSRFCookie,
		Value:    token,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})
}
