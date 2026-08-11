package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"carryapi/internal/apikey"
	"carryapi/internal/auth"
	"carryapi/internal/crypto"
	"carryapi/internal/db"
	"carryapi/internal/middleware"
	"carryapi/internal/settings"
	"carryapi/internal/user"
)

func TestFullAuthFlow(t *testing.T) {
	// 复用 Task 11 的 apiFixture(聚合 db/users/keys/sessions/settings + 各 handler)。
	// 这里用 chi 路由器挂载完整 /api 路由,跑端到端流程。
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	c, _ := crypto.New(bytes.Repeat([]byte{1}, 32))
	us := user.New(d, c)
	ss := auth.NewSessionStore(d)
	st := settings.New(d)
	st.Set("registration_open", "true")
	ls := auth.NewLoginService(us, ss, st)
	ks := apikey.New(d)
	authH := NewAuthHandler(ls, ss, us, st)
	keyH := NewKeyHandler(ks)
	usersH := NewUserHandler(us)

	r := chi.NewRouter()
	r.Use(middleware.SessionMiddleware(ss, us))
	r.Post("/api/auth/register", authH.Register)
	r.Post("/api/auth/login", authH.Login)
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireLogin())
		r.Use(middleware.CSRFMiddleware())
		r.Get("/api/keys", keyH.List)
		r.Post("/api/keys", keyH.Create)
	})
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireLogin())
		r.Use(middleware.RequireRole("admin"))
		r.Get("/api/users", usersH.List)
	})

	ts := httptest.NewServer(r)
	defer ts.Close()
	// cookie jar 自动携带 session + csrf cookie
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	// 1. 注册
	resp, _ := client.Post(ts.URL+"/api/auth/register",
		"application/json", strings.NewReader(`{"email":"e2e@x.com","password":"pw123"}`))
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("register status=%d", resp.StatusCode)
	}

	// 2. 登录(拿 session + csrf cookie)
	resp, _ = client.Post(ts.URL+"/api/auth/login",
		"application/json", strings.NewReader(`{"email":"e2e@x.com","password":"pw123"}`))
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login status=%d", resp.StatusCode)
	}
	var csrf string
	for _, c := range resp.Cookies() {
		if c.Name == middleware.CSRFCookie {
			csrf = c.Value
		}
	}

	// 3. 创建 API Key(带 csrf 头 + cookie 由 client jar 自动携带)
	req, _ := http.NewRequest("POST", ts.URL+"/api/keys", strings.NewReader(`{"label":"e2e"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.CSRFHeader, csrf)
	resp, _ = client.Do(req)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("create key status=%d body=%s", resp.StatusCode, body)
	}
	var keyResp map[string]any
	json.Unmarshal(body, &keyResp)
	if keyResp["key"] == nil {
		t.Fatal("expected plaintext key")
	}

	// 4. 普通用户访问 /api/users -> 403(非 admin)
	req, _ = http.NewRequest("GET", ts.URL+"/api/users", nil)
	req.Header.Set(middleware.CSRFHeader, csrf)
	resp, _ = client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Errorf("non-admin /api/users status=%d, want 403", resp.StatusCode)
	}
}
