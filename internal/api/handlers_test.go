package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"carryapi/internal/auth"
	"carryapi/internal/crypto"
	"carryapi/internal/db"
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
