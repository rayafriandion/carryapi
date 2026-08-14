package catalog

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"carryapi/internal/middleware"
	"github.com/go-chi/chi/v5"
)

func TestProviderKeyHandlerEndpoints(t *testing.T) {
	f := newCatalogFixture(t)
	h := NewHandler(f.db, f.providers, f.models, f.prices, nil)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Get("/api/providers/{id}/keys", h.ListProviderKeys)
	r.With(middleware.RequireRole("admin")).Post("/api/providers/{id}/keys", h.AddProviderKey)
	r.With(middleware.RequireRole("admin")).Delete("/api/providers/{id}/keys/{keyID}", h.DeleteProviderKey)
	r.With(middleware.RequireRole("admin")).Get("/api/providers/{id}/keys/{keyID}/logs", h.ProviderKeyLogs)

	_, _ = f.providers.Create("OpenAI", "https://api.openai.com/v1", "sk-seed", "openai_chat")

	// 1. 追加 key
	body, _ := json.Marshal(map[string]string{"api_key": "sk-added", "label": "b"})
	req := httptest.NewRequest("POST", "/api/providers/1/keys", bytes.NewReader(body))
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("add key code=%d body=%s", rec.Code, rec.Body.String())
	}
	var added map[string]any
	json.Unmarshal(rec.Body.Bytes(), &added)
	if added["masked"] == "" || added["id"] == nil {
		t.Errorf("add key response = %+v", added)
	}
	if added["masked"] == "sk-added" {
		t.Error("masked must not equal plaintext")
	}

	// 2. 列出 keys(应 2 条,掩码不泄露明文)
	req = httptest.NewRequest("GET", "/api/providers/1/keys", nil)
	req = req.WithContext(adminCtx())
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var listResp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &listResp)
	keys, _ := listResp["keys"].([]any)
	if len(keys) != 2 {
		t.Fatalf("keys = %d, want 2; resp=%s", len(keys), rec.Body.String())
	}
	for _, k := range keys {
		km := k.(map[string]any)
		if km["masked"] == "sk-seed" || km["masked"] == "sk-added" {
			t.Error("key list leaked plaintext")
		}
	}

	// 3. key 调用日志(应有 created 事件)
	req = httptest.NewRequest("GET", "/api/providers/1/keys/1/logs", nil)
	req = req.WithContext(adminCtx())
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var logResp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &logResp)
	events, _ := logResp["events"].([]any)
	if len(events) == 0 {
		t.Fatalf("expected created event, got %s", rec.Body.String())
	}
	first := events[0].(map[string]any)
	if first["event"] != "created" {
		t.Errorf("first event = %v, want created", first["event"])
	}

	// 4. 删除 key(手动)
	req = httptest.NewRequest("DELETE", "/api/providers/1/keys/2", nil)
	req = req.WithContext(adminCtx())
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("delete key code=%d body=%s", rec.Code, rec.Body.String())
	}
	// 删除后 key 2 状态应为 deleted
	got, err := f.providers.GetKey(2)
	if err != nil || got.Status != KeyStatusDeleted {
		t.Errorf("after delete: status=%q err=%v", got.Status, err)
	}
}

func TestListProvidersIncludesKeyCounts(t *testing.T) {
	f := newCatalogFixture(t)
	h := NewHandler(f.db, f.providers, f.models, f.prices, nil)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Get("/api/providers", h.ListProviders)

	prov, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "sk-a", "openai_chat")
	_, _ = f.providers.AddKey(prov.ID, "sk-b", "b")

	req := httptest.NewRequest("GET", "/api/providers", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list = %+v", list)
	}
	if list[0]["key_count"].(float64) != 2 || list[0]["active_key_count"].(float64) != 2 {
		t.Errorf("key counts = %+v", list[0])
	}
}
