package stats

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"carryapi/internal/middleware"
	"carryapi/internal/user"
)

// ctxUser 注入用户到 context。
func ctxUser(uid int64, role string) context.Context {
	u := &user.User{ID: uid, Email: "u@x.com", Role: role, Status: "active"}
	return context.WithValue(context.Background(), middleware.UserKey{}, u)
}

func newHandler(t *testing.T) (*Handler, *sql.DB) {
	d := newDB(t)
	seedLogs(t, d)
	return NewHandler(d), d
}

func TestHandlerSummaryAdmin(t *testing.T) {
	h, _ := newHandler(t)
	req := httptest.NewRequest("GET", "/api/stats/summary", nil)
	req = req.WithContext(ctxUser(1, "admin"))
	rec := httptest.NewRecorder()
	h.Summary(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var s Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.TotalRequests != 4 {
		t.Errorf("TotalRequests = %d, want 4 (admin sees all)", s.TotalRequests)
	}
}

func TestHandlerSummaryUserScoped(t *testing.T) {
	h, _ := newHandler(t)
	req := httptest.NewRequest("GET", "/api/stats/summary", nil)
	req = req.WithContext(ctxUser(2, "user")) // 普通用户
	rec := httptest.NewRecorder()
	h.Summary(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
	var s Summary
	json.Unmarshal(rec.Body.Bytes(), &s)
	if s.TotalRequests != 1 {
		t.Errorf("TotalRequests = %d, want 1 (user 2 only)", s.TotalRequests)
	}
}

func TestHandlerTrend(t *testing.T) {
	h, _ := newHandler(t)
	req := httptest.NewRequest("GET", "/api/stats/trend?granularity=day", nil)
	req = req.WithContext(ctxUser(1, "admin"))
	rec := httptest.NewRecorder()
	h.Trend(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var pts []TrendPoint
	json.Unmarshal(rec.Body.Bytes(), &pts)
	if len(pts) == 0 {
		t.Error("no trend points")
	}
}

func TestHandlerCost(t *testing.T) {
	h, _ := newHandler(t)
	req := httptest.NewRequest("GET", "/api/stats/cost?group=model", nil)
	req = req.WithContext(ctxUser(1, "admin"))
	rec := httptest.NewRecorder()
	h.Cost(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestHandlerSuccessRate(t *testing.T) {
	h, _ := newHandler(t)
	req := httptest.NewRequest("GET", "/api/stats/success-rate?group=model", nil)
	req = req.WithContext(ctxUser(1, "admin"))
	rec := httptest.NewRecorder()
	h.SuccessRate(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
	var rows []SuccessStat
	json.Unmarshal(rec.Body.Bytes(), &rows)
	if len(rows) != 2 {
		t.Errorf("rows = %d, want 2", len(rows))
	}
}

func TestHandlerLogs(t *testing.T) {
	h, _ := newHandler(t)
	req := httptest.NewRequest("GET", "/api/logs?page=1&page_size=10", nil)
	req = req.WithContext(ctxUser(1, "admin"))
	rec := httptest.NewRecorder()
	h.Logs(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Total int64      `json:"total"`
		Items []LogEntry `json:"items"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Total != 4 || len(resp.Items) != 4 {
		t.Errorf("total/items = %d/%d, want 4/4", resp.Total, len(resp.Items))
	}
}

func TestHandlerRequiresLogin(t *testing.T) {
	h, _ := newHandler(t)
	req := httptest.NewRequest("GET", "/api/stats/summary", nil)
	// 无用户 in context
	rec := httptest.NewRecorder()
	h.Summary(rec, req)
	if rec.Code != 401 {
		t.Errorf("code=%d, want 401", rec.Code)
	}
}

// TestParseTimeRangeDefaultsUTC 验证默认时间范围基于 UTC(修复本地时区漏数据)。
func TestParseTimeRangeDefaultsUTC(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/stats/summary", nil)
	start, end, err := parseTimeRange(r)
	if err != nil {
		t.Fatalf("parseTimeRange: %v", err)
	}
	if _, offset := start.Zone(); offset != 0 {
		t.Errorf("default start zone offset = %d, want 0 (UTC)", offset)
	}
	if _, offset := end.Zone(); offset != 0 {
		t.Errorf("default end zone offset = %d, want 0 (UTC)", offset)
	}
	// start 应为 30 天前
	if time.Since(start) < 29*24*time.Hour || time.Since(start) > 31*24*time.Hour {
		t.Errorf("start = %v, not ~30 days ago", start)
	}
}

// TestHandlerSummaryLocalTimezoneRange 用默认范围应覆盖 UTC 存储的数据(无论服务器时区)。
func TestHandlerSummaryLocalTimezoneRange(t *testing.T) {
	h, _ := newHandler(t)
	req := httptest.NewRequest("GET", "/api/stats/summary", nil)
	req = req.WithContext(ctxUser(1, "admin"))
	rec := httptest.NewRecorder()
	h.Summary(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var s Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.TotalRequests != 4 {
		t.Errorf("TotalRequests = %d, want 4 (default range must cover UTC-stored data regardless of server tz)", s.TotalRequests)
	}
}
