package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"carryapi/internal/settings"
)

type SettingsHandler struct {
	store        *settings.Store
	listenHost   string
	listenLocked bool
	listenSource string
}

func NewSettingsHandler(store *settings.Store, listenHost string, listenLocked bool, listenSource string) *SettingsHandler {
	return &SettingsHandler{store: store, listenHost: listenHost, listenLocked: listenLocked, listenSource: listenSource}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	out := map[string]string{}
	for _, k := range []string{"registration_open", "force_2fa", "log_retention_days"} {
		if v, ok, _ := h.store.Get(k); ok {
			out[k] = v
		}
	}
	if v, ok, _ := h.store.Get("listen_host"); ok && v != "" {
		out["listen_host"] = v
	} else {
		out["listen_host"] = "all"
	}
	if h.listenLocked {
		out["listen_host"] = h.listenHost
	}
	out["listen_host_locked"] = strconvFormatBool(h.listenLocked)
	out["listen_host_source"] = h.listenSource
	JSON(w, 200, out)
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, 400, "invalid request body")
		return
	}

	restartRequired := false
	for k, v := range req {
		switch k {
		case "registration_open", "force_2fa", "log_retention_days":
			_ = h.store.Set(k, v)
		case "listen_host":
			if h.listenLocked {
				JSONError(w, 400, "listen host is controlled by --host or CARRYAPI_HOST")
				return
			}
			mode := strings.TrimSpace(v)
			if !validListenHost(mode) {
				JSONError(w, 400, "invalid listen_host")
				return
			}
			if err := h.store.Set("listen_host", mode); err != nil {
				JSONError(w, 500, "could not save listen_host")
				return
			}
			restartRequired = true
		}
	}
	JSON(w, 200, map[string]any{"status": "ok", "restart_required": restartRequired})
}

func validListenHost(v string) bool {
	switch v {
	case "all", "0.0.0.0", "::", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func strconvFormatBool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
