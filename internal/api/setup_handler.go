package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"carryapi/internal/auth"
	"carryapi/internal/user"
)

type SetupHandler struct {
	users *user.Store
}

func NewSetupHandler(users *user.Store) *SetupHandler {
	return &SetupHandler{users: users}
}

// Status reports whether the first admin has been created yet.
func (h *SetupHandler) Status(w http.ResponseWriter, r *http.Request) {
	has, err := h.users.HasAdmin()
	if err != nil {
		JSONError(w, 500, "failed to check setup status")
		return
	}
	JSON(w, 200, map[string]any{"needs_setup": !has})
}

type setupAdminRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// CreateAdmin creates the first admin. Only allowed while no admin exists.
func (h *SetupHandler) CreateAdmin(w http.ResponseWriter, r *http.Request) {
	has, err := h.users.HasAdmin()
	if err != nil {
		JSONError(w, 500, "failed to check setup status")
		return
	}
	if has {
		JSONError(w, 403, "setup already complete")
		return
	}
	var req setupAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, 400, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		JSONError(w, 400, "email is required")
		return
	}
	if len(req.Password) < 8 {
		JSONError(w, 400, "password must be at least 8 characters")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		JSONError(w, 500, "hash failed")
		return
	}
	if _, err := h.users.Create(req.Email, hash, "admin"); err != nil {
		JSONError(w, 400, err.Error())
		return
	}
	JSON(w, 200, map[string]any{"ok": true})
}
