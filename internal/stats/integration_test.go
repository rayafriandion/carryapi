package stats

import (
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// TestStatsRoutesMounted 用 chi 挂载 stats 路由 + 中间件,验证 401(未登录)与 200(已登录)。
func TestStatsRoutesMounted(t *testing.T) {
	d := newDB(t)
	seedLogs(t, d)
	h := NewHandler(d)

	r := chi.NewRouter()
	r.Get("/api/stats/summary", h.Summary)
	r.Get("/api/logs", h.Logs)

	// 未登录 -> 401(handler 内部检查)
	req := httptest.NewRequest("GET", "/api/stats/summary", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("unauth code=%d, want 401", rec.Code)
	}

	// 已登录(admin)
	req = httptest.NewRequest("GET", "/api/stats/summary", nil)
	req = req.WithContext(ctxUser(1, "admin"))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("auth code=%d, want 200", rec.Code)
	}
}
