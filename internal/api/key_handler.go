package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"carryapi/internal/apikey"
	"carryapi/internal/middleware"
)

type KeyHandler struct {
	keys *apikey.Store
}

func NewKeyHandler(keys *apikey.Store) *KeyHandler {
	return &KeyHandler{keys: keys}
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
		out = append(out, map[string]any{
			"id": k.ID, "key_prefix": k.KeyPrefix, "label": k.Label,
			"status": k.Status, "created_at": k.CreatedAt, "last_used_at": k.LastUsedAt,
		})
	}
	JSON(w, 200, out)
}

func (h *KeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	var req struct {
		Label string `json:"label"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	plaintext, ak, err := h.keys.Create(u.ID, req.Label)
	if err != nil {
		JSONError(w, 500, "failed to create key")
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
		Label  string `json:"label"`
		Status string `json:"status"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := h.keys.Update(id, u.ID, req.Label, req.Status); err != nil {
		JSONError(w, 400, err.Error())
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
	JSON(w, 200, map[string]string{"status": "ok"})
}
