package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"carryapi/internal/auth"
	"carryapi/internal/crypto"
	"carryapi/internal/db"
	"carryapi/internal/middleware"
	"carryapi/internal/settings"
	"carryapi/internal/user"
)

func setupAPI(t *testing.T) (*AuthHandler, *user.Store) {
	t.Helper()
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	c := mustCipher(t)
	us := user.New(d, c)
	ss := auth.NewSessionStore(d)
	st := settings.New(d)
	st.Set("registration_open", "true")
	ls := auth.NewLoginService(us, ss, st)
	return NewAuthHandler(ls, ss, us, st), us
}

type apiFixture struct {
	auth     *AuthHandler
	users    *user.Store
	sessions *auth.SessionStore
	settings *settings.Store
	ls       *auth.LoginService
}

func setupAPIFixture(t *testing.T) *apiFixture {
	t.Helper()
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	c, _ := crypto.New(bytes.Repeat([]byte{1}, 32))
	us := user.New(d, c)
	ss := auth.NewSessionStore(d)
	st := settings.New(d)
	st.Set("registration_open", "true")
	ls := auth.NewLoginService(us, ss, st)
	return &apiFixture{auth: NewAuthHandler(ls, ss, us, st), users: us, sessions: ss, settings: st, ls: ls}
}

func TestRegisterLoginLogout(t *testing.T) {
	h, _ := setupAPI(t)

	// Register
	body, _ := json.Marshal(map[string]string{"email": "a@x.com", "password": "pw123"})
	rec := serve(h.Register, "POST", "/api/auth/register", body)
	if rec.Code != 200 {
		t.Fatalf("register code=%d body=%s", rec.Code, rec.Body.String())
	}

	// Login
	body, _ = json.Marshal(map[string]string{"email": "a@x.com", "password": "pw123"})
	rec = serve(h.Login, "POST", "/api/auth/login", body)
	if rec.Code != 200 {
		t.Fatalf("login code=%d", rec.Code)
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["requires_2fa"] == true {
		t.Error("should not require 2fa")
	}
	// session cookie 应设置
	if !hasCookie(rec, auth.SessionCookieName) {
		t.Error("missing session cookie")
	}

	// Logout
	rec = serve(h.Logout, "POST", "/api/auth/logout", nil)
	if rec.Code != 200 {
		t.Errorf("logout code=%d", rec.Code)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	h, us := setupAPI(t)
	hash, _ := auth.HashPassword("pw123")
	us.Create("b@x.com", hash, "user")
	body, _ := json.Marshal(map[string]string{"email": "b@x.com", "password": "wrong"})
	rec := serve(h.Login, "POST", "/api/auth/login", body)
	if rec.Code != 401 {
		t.Errorf("code=%d, want 401", rec.Code)
	}
}

// helpers
func serve(handler http.HandlerFunc, method, path string, body []byte) *httptest.ResponseRecorder {
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, r)
	return rec
}

func hasCookie(rec *httptest.ResponseRecorder, name string) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return true
		}
	}
	return false
}

func mustCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	c, err := crypto.New(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	return c
}

func TestDisable2FARequiresPassword(t *testing.T) {
	f := setupAPIFixture(t)
	// Build a user with a known password hash + a totp auth method
	hash, _ := auth.HashPassword("pw123")
	u, _ := f.users.Create("2fa@x.com", hash, "user")
	f.users.AddAuthMethod(u.ID, "totp", "", []byte("secret"))
	authH := NewAuthHandler(f.ls, f.sessions, f.users, f.settings)

	// Case 1: empty body -> should 401 (not bypass)
	req := httptest.NewRequest("POST", "/api/auth/2fa/disable", nil)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, &user.User{ID: u.ID, Email: u.Email, PasswordHash: hash, Role: "user", Status: "active"}))
	rec := httptest.NewRecorder()
	authH.Disable2FA(rec, req)
	if rec.Code != 401 {
		t.Errorf("empty body: code=%d, want 401", rec.Code)
	}
	// totp method should still exist
	methods, _ := f.users.GetAuthMethods(u.ID)
	if len(methods) != 1 {
		t.Errorf("after bypass attempt: %d methods, want 1 (not deleted)", len(methods))
	}

	// Case 2: wrong password -> 401, not deleted
	body, _ := json.Marshal(map[string]string{"password": "wrong"})
	req = httptest.NewRequest("POST", "/api/auth/2fa/disable", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, &user.User{ID: u.ID, Email: u.Email, PasswordHash: hash, Role: "user", Status: "active"}))
	rec = httptest.NewRecorder()
	authH.Disable2FA(rec, req)
	if rec.Code != 401 {
		t.Errorf("wrong password: code=%d, want 401", rec.Code)
	}

	// Case 3: correct password -> 200, methods deleted
	body, _ = json.Marshal(map[string]string{"password": "pw123"})
	req = httptest.NewRequest("POST", "/api/auth/2fa/disable", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, &user.User{ID: u.ID, Email: u.Email, PasswordHash: hash, Role: "user", Status: "active"}))
	rec = httptest.NewRecorder()
	authH.Disable2FA(rec, req)
	if rec.Code != 200 {
		t.Errorf("correct password: code=%d, want 200", rec.Code)
	}
	methods, _ = f.users.GetAuthMethods(u.ID)
	if len(methods) != 0 {
		t.Errorf("after disable: %d methods, want 0", len(methods))
	}
}
