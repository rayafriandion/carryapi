package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"carryapi/internal/auth"
	"carryapi/internal/crypto"
	"carryapi/internal/db"
	"carryapi/internal/user"
)

func setupStores(t *testing.T) (*auth.SessionStore, *user.Store) {
	t.Helper()
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	c := mustCipher(t)
	us := user.New(d, c)
	ss := auth.NewSessionStore(d)
	return ss, us
}

func mustCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	c, err := crypto.New(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	return c
}

func TestSessionMiddlewareLoadsUser(t *testing.T) {
	ss, us := setupStores(t)
	u, _ := us.Create("m@x.com", "h", "user")
	sess, _ := ss.Create(u.ID, time.Hour, "", "")
	called := false
	h := SessionMiddleware(ss, us)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := UserFromContext(r.Context())
		if !ok || got.ID != u.ID {
			t.Errorf("expected user %d, got %+v ok=%v", u.ID, got, ok)
		}
		called = true
	}))
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sess.Token})
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !called {
		t.Error("handler not called")
	}
}

func TestSessionMiddlewareAnonymous(t *testing.T) {
	ss, us := setupStores(t)
	h := SessionMiddleware(ss, us)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFromContext(r.Context()); ok {
			t.Error("expected no user for anonymous request")
		}
	}))
	req := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
}

func TestRequireLogin(t *testing.T) {
	h := RequireLogin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler")
	}))
	// 无 user in context
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("code = %d, want 401", rec.Code)
	}
}

func TestRequireRoleDenied(t *testing.T) {
	u := &user.User{ID: 1, Role: "user"}
	ctx := context.WithValue(context.Background(), UserKey{}, u)
	h := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("non-admin should not reach admin handler")
	}))
	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

func TestRateLimit(t *testing.T) {
	rl := RateLimit(2, time.Minute)
	blocked := 0
	h := rl(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			blocked++
		}
	}
	if blocked != 2 {
		t.Errorf("blocked = %d, want 2 (max=2, 4 requests)", blocked)
	}
}

func TestRateLimitHonorsXForwardedFor(t *testing.T) {
	rl := RateLimit(2, time.Minute)
	h := rl(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	// 同一 XFF 的前 2 次放行,第 3 次被限;不同 XFF 互不影响。
	blocked := 0
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "10.0.0.1:1234" // 代理地址,相同
		req.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			blocked++
		}
	}
	if blocked != 1 {
		t.Errorf("same XFF blocked = %d, want 1", blocked)
	}
	// 不同 XFF -> 新的配额
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.9, 10.0.0.1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusTooManyRequests {
		t.Error("different XFF client should not be limited")
	}
}
