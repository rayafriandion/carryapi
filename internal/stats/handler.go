package stats

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"carryapi/internal/middleware"
	"carryapi/internal/user"
)

// errUnauthorized 表示请求没有经过身份验证的用户。
var errUnauthorized = errors.New("unauthorized")

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// currentUser 返回 context 中的用户;缺失返回 nil。
func currentUser(r *http.Request) *user.User {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		return nil
	}
	return u
}

// params 解析公共查询参数(start/end/user_id),按角色确定范围。
func (h *Handler) params(r *http.Request) (QueryParams, error) {
	u := currentUser(r)
	if u == nil {
		return QueryParams{}, errUnauthorized
	}
	start, end, err := parseTimeRange(r)
	if err != nil {
		return QueryParams{}, err
	}
	p := QueryParams{Start: start, End: end}
	if u.Role == "admin" {
		// admin 可选 user_id 过滤
		if v := r.URL.Query().Get("user_id"); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return QueryParams{}, fmt.Errorf("invalid user_id")
			}
			p.UserID = &id
		}
	} else {
		// 普通用户只看自己
		uid := u.ID
		p.UserID = &uid
	}
	return p, nil
}

// writeParamsErr 根据参数解析错误区分 401(未认证)与 400(参数错误)。
func writeParamsErr(w http.ResponseWriter, err error) {
	if errors.Is(err, errUnauthorized) {
		writeErr(w, 401, "unauthorized")
		return
	}
	writeErr(w, 400, err.Error())
}

func parseTimeRange(r *http.Request) (time.Time, time.Time, error) {
	// SQLite 用 CURRENT_TIMESTAMP 存 UTC;默认范围必须基于 UTC 才不会漏掉近期数据。
	now := time.Now().UTC()
	start := now.Add(-30 * 24 * time.Hour)
	end := now
	if v := r.URL.Query().Get("start"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid start (RFC3339): %v", err)
		}
		start = t.UTC()
	}
	if v := r.URL.Query().Get("end"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end (RFC3339): %v", err)
		}
		end = t.UTC()
	}
	return start, end, nil
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	p, err := h.params(r)
	if err != nil {
		writeParamsErr(w, err)
		return
	}
	s, err := QuerySummary(h.db, p)
	if err != nil {
		writeErr(w, 500, "query failed")
		return
	}
	writeJSON(w, 200, s)
}

func (h *Handler) Trend(w http.ResponseWriter, r *http.Request) {
	p, err := h.params(r)
	if err != nil {
		writeParamsErr(w, err)
		return
	}
	g := TrendGranularity(r.URL.Query().Get("granularity"))
	if g == "" {
		g = GranularityDay
	}
	if g != GranularityDay && g != GranularityHour {
		writeErr(w, 400, "granularity must be day or hour")
		return
	}
	pts, err := QueryTrend(h.db, p, g)
	if err != nil {
		writeErr(w, 500, "query failed")
		return
	}
	writeJSON(w, 200, pts)
}

func (h *Handler) Cost(w http.ResponseWriter, r *http.Request) {
	p, err := h.params(r)
	if err != nil {
		writeParamsErr(w, err)
		return
	}
	group := CostGroup(r.URL.Query().Get("group"))
	if group == "" {
		group = CostByModel
	}
	rows, err := QueryCost(h.db, p, group)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, rows)
}

func (h *Handler) SuccessRate(w http.ResponseWriter, r *http.Request) {
	p, err := h.params(r)
	if err != nil {
		writeParamsErr(w, err)
		return
	}
	group := r.URL.Query().Get("group")
	if group == "" {
		group = "model"
	}
	rows, err := QuerySuccessRate(h.db, p, group)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, rows)
}

func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		writeErr(w, 401, "unauthorized")
		return
	}
	start, end, err := parseTimeRange(r)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	f := LogFilter{Start: start, End: end}
	if u.Role == "admin" {
		if v := r.URL.Query().Get("user_id"); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				writeErr(w, 400, "invalid user_id")
				return
			}
			f.UserID = &id
		}
	} else {
		uid := u.ID
		f.UserID = &uid
	}
	f.Model = r.URL.Query().Get("model")
	if v := r.URL.Query().Get("status"); v != "" {
		if sc, err := strconv.Atoi(v); err == nil {
			f.StatusCode = &sc
		}
	}
	f.ErrorType = r.URL.Query().Get("error_type")
	f.RequestID = r.URL.Query().Get("request_id")
	if v := r.URL.Query().Get("provider_key_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.ProviderKeyID = &id
		}
	}
	if v := r.URL.Query().Get("page"); v != "" {
		f.Page, _ = strconv.Atoi(v)
	}
	if v := r.URL.Query().Get("page_size"); v != "" {
		f.PageSize, _ = strconv.Atoi(v)
	}
	total, items, err := QueryLogs(h.db, f)
	if err != nil {
		writeErr(w, 500, "query failed")
		return
	}
	writeJSON(w, 200, map[string]any{
		"total": total, "page": f.Page, "page_size": f.PageSize, "items": items,
	})
}
