package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"carryapi/internal/apikey"
	"carryapi/internal/auth"
	"carryapi/internal/crypto"
	"carryapi/internal/db"
	"carryapi/internal/middleware"
	"carryapi/internal/settings"
	"carryapi/internal/user"
)

type apiFixture struct {
	auth     *AuthHandler
	users    *user.Store
	keys     *apikey.Store
	sessions *auth.SessionStore
	settings *settings.Store
	ls       *auth.LoginService
	db       *sql.DB
}

func setupAPI(t *testing.T) *apiFixture {
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
	ks := apikey.New(d)
	return &apiFixture{
		auth:     NewAuthHandler(ls, ss, us, st),
		users:    us,
		keys:     ks,
		sessions: ss,
		settings: st,
		ls:       ls,
		db:       d,
	}
}

// withChiParam injects a chi URL param into the request context so handlers
// using chi.URLParam can read it without a real chi router.
func withChiParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestRegisterLoginLogout(t *testing.T) {
	f := setupAPI(t)

	// Register
	body, _ := json.Marshal(map[string]string{"email": "a@x.com", "password": "pw123"})
	rec := serve(f.auth.Register, "POST", "/api/auth/register", body)
	if rec.Code != 200 {
		t.Fatalf("register code=%d body=%s", rec.Code, rec.Body.String())
	}

	// Login
	body, _ = json.Marshal(map[string]string{"email": "a@x.com", "password": "pw123"})
	rec = serve(f.auth.Login, "POST", "/api/auth/login", body)
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
	rec = serve(f.auth.Logout, "POST", "/api/auth/logout", nil)
	if rec.Code != 200 {
		t.Errorf("logout code=%d", rec.Code)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	f := setupAPI(t)
	hash, _ := auth.HashPassword("pw123")
	f.users.Create("b@x.com", hash, "user")
	body, _ := json.Marshal(map[string]string{"email": "b@x.com", "password": "wrong"})
	rec := serve(f.auth.Login, "POST", "/api/auth/login", body)
	if rec.Code != 401 {
		t.Errorf("code=%d, want 401", rec.Code)
	}
}

func TestLoginDisabledUserReturns401(t *testing.T) {
	f := setupAPI(t)
	hash, _ := auth.HashPassword("pw123")
	u, _ := f.users.Create("disabled@x.com", hash, "user")
	f.users.UpdateStatus(u.ID, "disabled")
	body, _ := json.Marshal(map[string]string{"email": "disabled@x.com", "password": "pw123"})
	rec := serve(f.auth.Login, "POST", "/api/auth/login", body)
	// 与无效凭据/不存在的账户一致:401 + 同一 body,不枚举账户状态
	if rec.Code != 401 {
		t.Errorf("code=%d, want 401 (no 403 enumeration)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid credentials") {
		t.Errorf("body = %q, want unified 'invalid credentials'", rec.Body.String())
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
	f := setupAPI(t)
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

func TestUserCreateAsAdmin(t *testing.T) {
	f := setupAPI(t)
	uh := NewUserHandler(f.users, f.sessions)
	// 注入 admin 用户到 context(admin 仅存在于 context,不在 DB,所以 List 只看到新建的 user)
	admin := &user.User{ID: 1, Email: "admin@x.com", Role: "admin", Status: "active"}
	body, _ := json.Marshal(map[string]string{"email": "newuser@x.com", "password": "pw", "role": "user"})
	req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, admin))
	rec := httptest.NewRecorder()
	uh.Create(rec, req)
	if rec.Code != 200 {
		t.Fatalf("create user code=%d body=%s", rec.Code, rec.Body.String())
	}
	// 应能 list 看到
	req = httptest.NewRequest("GET", "/api/users", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, admin))
	rec = httptest.NewRecorder()
	uh.List(rec, req)
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0]["email"] != "newuser@x.com" {
		t.Errorf("list = %+v", list)
	}
}

func TestUserDeletePreventsSelf(t *testing.T) {
	f := setupAPI(t)
	uh := NewUserHandler(f.users, f.sessions)
	admin := &user.User{ID: 1, Email: "admin@x.com", Role: "admin", Status: "active"}
	req := httptest.NewRequest("DELETE", "/api/users/1", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, admin))
	// chi URLParam 需要在 chi 路由上下文;直接调 handler 时手动设
	req = withChiParam(req, "id", "1")
	rec := httptest.NewRecorder()
	uh.Delete(rec, req)
	if rec.Code != 400 {
		t.Errorf("deleting self should be 400, got %d", rec.Code)
	}
}

func TestUserDisableRevokesSessions(t *testing.T) {
	f := setupAPI(t)
	uh := NewUserHandler(f.users, f.sessions)
	// 建一个真实用户并创建会话
	u, _ := f.users.Create("victim@x.com", "h", "user")
	sess, err := f.sessions.Create(u.ID, 7*24*time.Hour, "", "")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if _, err := f.sessions.Lookup(sess.Token); err != nil {
		t.Fatalf("session should be valid: %v", err)
	}
	// admin 禁用该用户
	admin := &user.User{ID: 999, Email: "admin@x.com", Role: "admin", Status: "active"}
	body, _ := json.Marshal(map[string]string{"status": "disabled"})
	req := httptest.NewRequest("PUT", "/api/users/2", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, admin))
	req = withChiParam(req, "id", strconv.FormatInt(u.ID, 10))
	rec := httptest.NewRecorder()
	uh.Update(rec, req)
	if rec.Code != 200 {
		t.Fatalf("update code=%d body=%s", rec.Code, rec.Body.String())
	}
	// 该用户的会话应全部撤销
	if _, err := f.sessions.Lookup(sess.Token); err == nil {
		t.Error("session should be revoked after user disabled")
	}
	got, _ := f.users.GetByID(u.ID)
	if got.Status != "disabled" {
		t.Errorf("user status = %q, want disabled", got.Status)
	}
}

func TestDisable2FARevokesSessions(t *testing.T) {
	f := setupAPI(t)
	hash, _ := auth.HashPassword("pw123")
	u, _ := f.users.Create("2farevoke@x.com", hash, "user")
	f.users.AddAuthMethod(u.ID, "totp", "", []byte("secret"))
	// 为该用户建一个会话
	sess, _ := f.sessions.Create(u.ID, 7*24*time.Hour, "", "")
	if _, err := f.sessions.Lookup(sess.Token); err != nil {
		t.Fatalf("session should be valid: %v", err)
	}
	authH := NewAuthHandler(f.ls, f.sessions, f.users, f.settings)
	body, _ := json.Marshal(map[string]string{"password": "pw123"})
	req := httptest.NewRequest("POST", "/api/auth/2fa/disable", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, &user.User{ID: u.ID, Email: u.Email, PasswordHash: hash, Role: "user", Status: "active"}))
	rec := httptest.NewRecorder()
	authH.Disable2FA(rec, req)
	if rec.Code != 200 {
		t.Fatalf("disable 2fa code=%d body=%s", rec.Code, rec.Body.String())
	}
	// 禁用 2FA 后会话应被撤销
	if _, err := f.sessions.Lookup(sess.Token); err == nil {
		t.Error("session should be revoked after 2fa disable")
	}
}

func TestKeyCreateAndList(t *testing.T) {
	f := setupAPI(t)
	kh := NewKeyHandler(f.keys)
	// 先建真实用户以满足 api_keys.user_id 的外键约束(FK 在 db.Open 中已启用)
	realU, _ := f.users.Create("keyowner@x.com", "dummyhash", "user")
	u := &user.User{ID: realU.ID, Role: "user", Status: "active"}
	// create
	body, _ := json.Marshal(map[string]string{"label": "test"})
	req := httptest.NewRequest("POST", "/api/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, u))
	rec := httptest.NewRecorder()
	kh.Create(rec, req)
	if rec.Code != 200 {
		t.Fatalf("create code=%d body=%s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created["key"] == nil {
		t.Error("expected plaintext key in response")
	}
	// list
	req = httptest.NewRequest("GET", "/api/keys", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, u))
	rec = httptest.NewRecorder()
	kh.List(rec, req)
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0]["label"] != "test" {
		t.Errorf("list = %+v", list)
	}
}

func TestSettingsUpdateAdminOnly(t *testing.T) {
	f := setupAPI(t)
	sh := NewSettingsHandler(f.settings)
	// 普通用户 -> 角色守卫在路由层,handler 本身不校验。
	// 此测试验证:handler 在 admin 调用时能更新;非 admin 由路由 RequireRole 拦截(403)。
	// 这里直接测 admin 成功路径:
	admin := &user.User{ID: 1, Role: "admin", Status: "active"}
	body, _ := json.Marshal(map[string]string{"force_2fa": "true"})
	req := httptest.NewRequest("PUT", "/api/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, admin))
	rec := httptest.NewRecorder()
	sh.Update(rec, req)
	if rec.Code != 200 {
		t.Fatalf("update code=%d", rec.Code)
	}
	// 验证写入
	v, ok, _ := f.settings.Get("force_2fa")
	if !ok || v != "true" {
		t.Errorf("force_2fa = %q ok=%v", v, ok)
	}
}
