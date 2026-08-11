package api

import (
	"encoding/json"
	"net/http"

	"carryapi/internal/middleware"
	"carryapi/internal/settings"
)

type SettingsHandler struct {
	store *settings.Store
}

func NewSettingsHandler(store *settings.Store) *SettingsHandler {
	return &SettingsHandler{store: store}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	// 返回所有安全可见的设置(不含 OAuth secret)
	out := map[string]string{}
	for _, k := range []string{"listen_host", "registration_open", "force_2fa", "log_retention_days"} {
		if v, ok, _ := h.store.Get(k); ok {
			out[k] = v
		}
	}
	JSON(w, 200, out)
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	// RequireRole("admin") 在路由层守卫
	var req map[string]string
	json.NewDecoder(r.Body).Decode(&req)
	for k, v := range req {
		// 白名单
		switch k {
		case "registration_open", "force_2fa", "log_retention_days":
			h.store.Set(k, v)
		}
		// listen_host 由广播开关单独处理(子项目1 已有逻辑,此处不改避免重启)
	}
	_ = middleware.UserFromContext // 避免未用 import
	JSON(w, 200, map[string]string{"status": "ok"})
}
