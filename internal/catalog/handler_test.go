package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"testing"

	"carryapi/internal/middleware"
	"carryapi/internal/user"
	"github.com/go-chi/chi/v5"
)

// admin context helper
func adminCtx() context.Context {
	u := &user.User{ID: 1, Email: "admin@x.com", Role: "admin", Status: "active"}
	return context.WithValue(context.Background(), middleware.UserKey{}, u)
}

func TestProviderCRUDHandler(t *testing.T) {
	f := newCatalogFixture(t)
	h := NewHandler(f.providers, f.models, f.prices)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Post("/api/providers", h.CreateProvider)
	r.With(middleware.RequireRole("admin")).Get("/api/providers", h.ListProviders)

	// create
	body, _ := json.Marshal(map[string]string{"name": "OpenAI", "base_url": "https://api.openai.com/v1", "api_key": "sk-1", "protocol": "openai_chat"})
	req := httptest.NewRequest("POST", "/api/providers", bytes.NewReader(body))
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("create code=%d body=%s", rec.Code, rec.Body.String())
	}
	// list
	req = httptest.NewRequest("GET", "/api/providers", nil)
	req = req.WithContext(adminCtx())
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0]["name"] != "OpenAI" {
		t.Errorf("list = %+v", list)
	}
}

func TestProviderCRUDNonAdminForbidden(t *testing.T) {
	f := newCatalogFixture(t)
	h := NewHandler(f.providers, f.models, f.prices)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Post("/api/providers", h.CreateProvider)
	// 非 admin context
	u := &user.User{ID: 2, Email: "user@x.com", Role: "user", Status: "active"}
	req := httptest.NewRequest("POST", "/api/providers", bytes.NewReader([]byte(`{}`)))
	req = req.WithContext(context.WithValue(context.Background(), middleware.UserKey{}, u))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

func TestModelCRUDHandler(t *testing.T) {
	f := newCatalogFixture(t)
	h := NewHandler(f.providers, f.models, f.prices)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Post("/api/models", h.CreateModel)
	r.With(middleware.RequireRole("admin")).Get("/api/models", h.ListModels)
	// 先建 provider(模型引用它)
	prov, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "sk-1", "openai_chat")
	// create model
	body, _ := json.Marshal(map[string]any{"name": "my-gpt4", "provider_id": prov.ID, "upstream_model": "gpt-4o"})
	req := httptest.NewRequest("POST", "/api/models", bytes.NewReader(body))
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("create model code=%d body=%s", rec.Code, rec.Body.String())
	}
	// list
	req = httptest.NewRequest("GET", "/api/models", nil)
	req = req.WithContext(adminCtx())
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0]["name"] != "my-gpt4" {
		t.Errorf("list = %+v", list)
	}
}

func TestInvalidIDParamReturns400(t *testing.T) {
	f := newCatalogFixture(t)
	h := NewHandler(f.providers, f.models, f.prices)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Put("/api/providers/{id}", h.UpdateProvider)
	// 非数字 id -> 400 invalid id
	req := httptest.NewRequest("PUT", "/api/providers/abc", bytes.NewReader([]byte(`{"name":"x"}`)))
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Errorf("code = %d, want 400", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("invalid id")) {
		t.Errorf("body = %s, want invalid id", rec.Body.String())
	}
}

func TestPriceHandler(t *testing.T) {
	f := newCatalogFixture(t)
	h := NewHandler(f.providers, f.models, f.prices)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Put("/api/models/{id}/price", h.SetModelPrice)
	r.With(middleware.RequireRole("admin")).Get("/api/models/{id}/price", h.GetModelPrice)
	prov, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "sk-1", "openai_chat")
	m, _ := f.models.Create("my-gpt4", prov.ID, "gpt-4o")
	// set price
	body, _ := json.Marshal(map[string]any{"input_price": 5.0, "output_price": 15.0})
	req := httptest.NewRequest("PUT", "/api/models/"+strconv.FormatInt(m.ID, 10)+"/price", bytes.NewReader(body))
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("set price code=%d body=%s", rec.Code, rec.Body.String())
	}
	// get price
	req = httptest.NewRequest("GET", "/api/models/"+strconv.FormatInt(m.ID, 10)+"/price", nil)
	req = req.WithContext(adminCtx())
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	price, ok := resp["price"].(map[string]any)
	if !ok || price["input_price"] != 5.0 {
		t.Errorf("price = %+v", resp)
	}
}
