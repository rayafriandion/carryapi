package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"carryapi/internal/middleware"
	"carryapi/internal/user"
)

type QuotaHandler struct {
	users *user.Store
}

func NewQuotaHandler(users *user.Store) *QuotaHandler {
	return &QuotaHandler{users: users}
}

func (h *QuotaHandler) List(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	// admin 看全部?简化:admin 传 ?user_id= 查指定,否则看自己
	quotas, err := h.users.GetQuotas("user", u.ID)
	if err != nil {
		JSONError(w, 500, "failed to list quotas")
		return
	}
	JSON(w, 200, quotas)
}

func (h *QuotaHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var req struct {
		LimitTokens *int64   `json:"limit_tokens"`
		LimitCost   *float64 `json:"limit_cost"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := h.users.UpdateQuota(id, req.LimitTokens, req.LimitCost); err != nil {
		JSONError(w, 500, "update failed")
		return
	}
	JSON(w, 200, map[string]string{"status": "ok"})
}
