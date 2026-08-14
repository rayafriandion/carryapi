package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"carryapi/internal/apikey"
	"carryapi/internal/middleware"
	"carryapi/internal/user"
)

type KeyHandler struct {
	keys  *apikey.Store
	users *user.Store // 可选:配置后 API Key 配额随创建/编辑/删除持久化
}

func NewKeyHandler(keys *apikey.Store) *KeyHandler {
	return &KeyHandler{keys: keys}
}

// SetUsers 注入 user.Store(API Key 配额 CRUD 用)。不设置时配额相关操作被跳过。
func (h *KeyHandler) SetUsers(u *user.Store) { h.users = u }

// keyQuotaReq API Key 配额(创建/编辑时可选)。limit_tokens/limit_cost 为 nil 表示该维度不限制。
type keyQuotaReq struct {
	Period      string   `json:"period"`
	LimitTokens *int64   `json:"limit_tokens"`
	LimitCost   *float64 `json:"limit_cost"`
}

func (q *keyQuotaReq) validate() error {
	if q == nil {
		return nil
	}
	if q.Period != "" && q.Period != "total" && q.Period != "month" {
		return fmt.Errorf("invalid quota period %q", q.Period)
	}
	if q.LimitTokens != nil && *q.LimitTokens < 0 {
		return fmt.Errorf("limit_tokens must be non-negative")
	}
	if q.LimitCost != nil && *q.LimitCost < 0 {
		return fmt.Errorf("limit_cost must be non-negative")
	}
	return nil
}

// applyKeyQuota 持久化 API Key 配额(users store 未配置或请求未携带 quota 时跳过)。
func (h *KeyHandler) applyKeyQuota(keyID int64, q *keyQuotaReq) error {
	if h.users == nil || q == nil {
		return nil
	}
	_, err := h.users.SetKeyQuota(keyID, q.Period, q.LimitTokens, q.LimitCost)
	return err
}

func (h *KeyHandler) keyQuotaMap(q user.Quota) map[string]any {
	return map[string]any{
		"id": q.ID, "scope": q.Scope, "scope_id": q.ScopeID, "period": q.Period,
		"limit_tokens": q.LimitTokens, "limit_cost": q.LimitCost,
		"used_tokens": q.UsedTokens, "used_cost": q.UsedCost,
	}
}

func (h *KeyHandler) List(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	keys, err := h.keys.List(u.ID)
	if err != nil {
		JSONError(w, 500, "failed to list keys")
		return
	}
	out := []map[string]any{}
	for _, k := range keys {
		entry := map[string]any{
			"id": k.ID, "key_prefix": k.KeyPrefix, "label": k.Label,
			"status": k.Status, "created_at": k.CreatedAt, "last_used_at": k.LastUsedAt,
		}
		if h.users != nil {
			if q, qErr := h.users.GetKeyQuota(k.ID); qErr == nil && q.ID != 0 {
				entry["quota"] = h.keyQuotaMap(q)
			} else {
				entry["quota"] = nil
			}
		} else {
			entry["quota"] = nil
		}
		out = append(out, entry)
	}
	JSON(w, 200, out)
}

func (h *KeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	var req struct {
		Label string       `json:"label"`
		Quota *keyQuotaReq `json:"quota"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := req.Quota.validate(); err != nil {
		JSONError(w, 400, err.Error())
		return
	}
	plaintext, ak, err := h.keys.Create(u.ID, req.Label)
	if err != nil {
		JSONError(w, 500, "failed to create key")
		return
	}
	if err := h.applyKeyQuota(ak.ID, req.Quota); err != nil {
		JSONError(w, 500, "failed to save quota: "+err.Error())
		return
	}
	JSON(w, 200, map[string]any{
		"id":         ak.ID,
		"key":        plaintext, // 仅此一次返回明文
		"key_prefix": ak.KeyPrefix,
		"label":      ak.Label,
	})
}

func (h *KeyHandler) Update(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var req struct {
		Label  string       `json:"label"`
		Status string       `json:"status"`
		Quota  *keyQuotaReq `json:"quota"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := req.Quota.validate(); err != nil {
		JSONError(w, 400, err.Error())
		return
	}
	if err := h.keys.Update(id, u.ID, req.Label, req.Status); err != nil {
		JSONError(w, 400, err.Error())
		return
	}
	if err := h.applyKeyQuota(id, req.Quota); err != nil {
		JSONError(w, 500, "failed to save quota: "+err.Error())
		return
	}
	JSON(w, 200, map[string]string{"status": "ok"})
}

func (h *KeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := h.keys.Delete(id, u.ID); err != nil {
		JSONError(w, 400, err.Error())
		return
	}
	if h.users != nil {
		if err := h.users.DeleteKeyQuota(id); err != nil {
			JSONError(w, 500, "failed to delete quota: "+err.Error())
			return
		}
	}
	JSON(w, 200, map[string]string{"status": "ok"})
}
