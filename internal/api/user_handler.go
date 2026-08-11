package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"carryapi/internal/auth"
	"carryapi/internal/middleware"
	"carryapi/internal/user"
)

type UserHandler struct {
	users *user.Store
}

func NewUserHandler(users *user.Store) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.List()
	if err != nil {
		JSONError(w, 500, "failed to list users")
		return
	}
	out := []map[string]any{}
	for _, u := range users {
		out = append(out, map[string]any{"id": u.ID, "email": u.Email, "role": u.Role, "status": u.Status, "created_at": u.CreatedAt})
	}
	JSON(w, 200, out)
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, 400, "invalid body")
		return
	}
	if req.Role == "" {
		req.Role = "user"
	}
	// 管理员创建用户:直接哈希密码
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		JSONError(w, 500, "hash failed")
		return
	}
	u, err := h.users.Create(req.Email, hash, req.Role)
	if err != nil {
		JSONError(w, 400, err.Error())
		return
	}
	JSON(w, 200, map[string]any{"id": u.ID, "email": u.Email, "role": u.Role})
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		JSONError(w, 400, "invalid id")
		return
	}
	var req struct {
		Role   string `json:"role"`
		Status string `json:"status"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Role != "" {
		h.users.UpdateRole(id, req.Role)
	}
	if req.Status != "" {
		h.users.UpdateStatus(id, req.Status)
	}
	JSON(w, 200, map[string]string{"status": "ok"})
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	// 防止删自己
	if u, ok := middleware.UserFromContext(r.Context()); ok && u.ID == id {
		JSONError(w, 400, "cannot delete yourself")
		return
	}
	if err := h.users.DeleteCascade(id); err != nil {
		JSONError(w, 500, "delete failed")
		return
	}
	JSON(w, 200, map[string]string{"status": "ok"})
}
