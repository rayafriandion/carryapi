package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"carryapi/internal/crypto"
	"carryapi/internal/middleware"
	"carryapi/internal/settings"
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
	h := NewHandler(f.db, f.providers, f.models, f.prices, nil)
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
	h := NewHandler(f.db, f.providers, f.models, f.prices, nil)
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
	h := NewHandler(f.db, f.providers, f.models, f.prices, nil)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Post("/api/models", h.CreateModel)
	r.With(middleware.RequireRole("admin")).Get("/api/models", h.ListModels)
	// 先建 provider(模型引用它)
	prov, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "sk-1", "openai_chat")
	// create model (price fields now required)
	body, _ := json.Marshal(map[string]any{
		"name": "my-gpt4", "provider_id": prov.ID, "upstream_model": "gpt-4o",
		"currency": "USD", "input_price": 2.5, "output_price": 10.0,
	})
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
	price, ok := list[0]["price"].(map[string]any)
	if !ok || price["input_price"] != 2.5 || price["output_price"] != 10.0 || price["currency"] != "USD" {
		t.Errorf("list price = %+v", list[0]["price"])
	}
}

func TestInvalidIDParamReturns400(t *testing.T) {
	f := newCatalogFixture(t)
	h := NewHandler(f.db, f.providers, f.models, f.prices, nil)
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
	h := NewHandler(f.db, f.providers, f.models, f.prices, nil)
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

func newTestHandler(t *testing.T) (*Handler, *httptest.Server) {
	f := newCatalogFixture(t)
	h := NewHandler(f.db, f.providers, f.models, f.prices, nil)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"gpt-4o"},{"id":"gpt-4o-mini"}]}`))
	}))
	t.Cleanup(up.Close)
	h.SetProber(NewProber(up.Client()))
	// 建一个指向 up 的 provider
	f.providers.Create("Up", up.URL, "sk-test", "openai_chat")
	return h, up
}

func TestFetchProviderModels(t *testing.T) {
	h, _ := newTestHandler(t)
	r := chi.NewRouter()
	r.Get("/api/providers/{id}/models/fetch", h.FetchProviderModels)
	req := httptest.NewRequest("GET", "/api/providers/1/models/fetch", nil).WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Models []map[string]any `json:"models"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Models) != 2 {
		t.Fatalf("models=%+v", resp.Models)
	}
}

func TestImportModels(t *testing.T) {
	h, _ := newTestHandler(t)
	body, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"provider_id": 1, "upstream_model": "gpt-4o"},
			{"provider_id": 1, "upstream_model": "gpt-4o"},
			{"provider_id": 1, "upstream_model": "gpt-4o-mini"},
		},
	})
	req := httptest.NewRequest("POST", "/api/models/import", bytes.NewReader(body)).WithContext(adminCtx())
	rec := httptest.NewRecorder()
	h.ImportModels(rec, req)
	var resp struct {
		Imported     int      `json:"imported"`
		Skipped      int      `json:"skipped"`
		SkippedNames []string `json:"skipped_names"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Imported != 2 || resp.Skipped != 1 {
		t.Fatalf("imported=%d skipped=%d", resp.Imported, resp.Skipped)
	}
	// 确认导入为禁用态
	m, err := h.models.GetByName("gpt-4o")
	if err != nil || m.Enabled {
		t.Fatalf("draft should be disabled: %+v err=%v", m, err)
	}
}

func TestImportModelsFailed(t *testing.T) {
	h, _ := newTestHandler(t)
	// provider_id 999 doesn't exist -> CreateDraft FK failure -> counted as failed
	body, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"provider_id": 1, "upstream_model": "gpt-4o"},
			{"provider_id": 999, "upstream_model": "gpt-4o-broken"},
		},
	})
	req := httptest.NewRequest("POST", "/api/models/import", bytes.NewReader(body)).WithContext(adminCtx())
	rec := httptest.NewRecorder()
	h.ImportModels(rec, req)
	var resp struct {
		Imported int      `json:"imported"`
		Failed   int      `json:"failed"`
		Errors   []string `json:"errors"`
		Skipped  int      `json:"skipped"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Imported != 1 || resp.Failed != 1 || len(resp.Errors) != 1 {
		t.Fatalf("imported=%d failed=%d errors=%v", resp.Imported, resp.Failed, resp.Errors)
	}
	if !bytes.Contains([]byte(resp.Errors[0]), []byte("gpt-4o-broken")) {
		t.Errorf("errors[0]=%q, want mention of upstream model", resp.Errors[0])
	}
	// the failed item must not be persisted
	if _, err := h.models.GetByName("gpt-4o-broken"); err == nil {
		t.Errorf("failed item should not have been persisted")
	}
}

func TestTestProviderOK(t *testing.T) {
	h, _ := newTestHandler(t)
	r := chi.NewRouter()
	r.Post("/api/providers/{id}/test", h.TestProvider)
	req := httptest.NewRequest("POST", "/api/providers/1/test", nil).WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["ok"] != true {
		t.Fatalf("resp=%+v", resp)
	}
}

func TestTestProviderFailure(t *testing.T) {
	f := newCatalogFixture(t)
	h := NewHandler(f.db, f.providers, f.models, f.prices, nil)
	// dead/unreachable upstream URL
	f.providers.Create("Dead", "http://127.0.0.1:1", "sk-test", "openai_chat")
	h.SetProber(NewProber(&http.Client{Timeout: 500 * time.Millisecond}))
	r := chi.NewRouter()
	r.Post("/api/providers/{id}/test", h.TestProvider)
	req := httptest.NewRequest("POST", "/api/providers/1/test", nil).WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	// per-spec: failure returns 200 with ok:false
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["ok"] != false {
		t.Fatalf("resp=%+v, want ok=false", resp)
	}
	if errMsg, _ := resp["error"].(string); errMsg == "" {
		t.Fatalf("resp=%+v, want non-empty error", resp)
	}
}

func TestGetRoutingStatus(t *testing.T) {
	f := newCatalogFixture(t)
	rs := NewRoutingStats(f.db)
	h := NewHandler(f.db, f.providers, f.models, f.prices, rs)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Get("/api/routing/status", h.GetRoutingStatus)

	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	m, _ := f.models.Create("my-gpt4", p.ID, "gpt-4o")
	_, _ = f.bindingsStore().Create(m.ID, p.ID, "gpt-4o", 100, 1, true)
	insertLog(t, f.db, p.ID, "gpt-4o", 200, "none", 1*time.Hour, 100, 50)

	req := httptest.NewRequest("GET", "/api/routing/status", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Models []struct {
			Name     string `json:"name"`
			Bindings []struct {
				ProviderID    int64    `json:"provider_id"`
				UpstreamModel string   `json:"upstream_model"`
				Timeline      []string `json:"timeline"`
				AvgLatencyMs  int64    `json:"avg_latency_ms"`
			} `json:"bindings"`
		} `json:"models"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Models) != 1 || resp.Models[0].Name != "my-gpt4" {
		t.Fatalf("unexpected models: %+v", resp.Models)
	}
	if len(resp.Models[0].Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(resp.Models[0].Bindings))
	}
	b := resp.Models[0].Bindings[0]
	if len(b.Timeline) != 6 {
		t.Errorf("expected 6 timeline buckets, got %d", len(b.Timeline))
	}
	if b.AvgLatencyMs != 100 {
		t.Errorf("expected avg latency 100, got %d", b.AvgLatencyMs)
	}
}

func TestGetBindingMetrics(t *testing.T) {
	f := newCatalogFixture(t)
	rs := NewRoutingStats(f.db)
	h := NewHandler(f.db, f.providers, f.models, f.prices, rs)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Get("/api/routing/bindings/{bindingID}/metrics", h.GetBindingMetrics)

	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	m, _ := f.models.Create("my-gpt4", p.ID, "gpt-4o")
	// models.Create auto-creates the first binding; fetch it rather than
	// re-creating (which would violate the unique constraint).
	bs, _ := f.bindingsStore().ListByModel(m.ID)
	if len(bs) != 1 {
		t.Fatalf("expected 1 auto-created binding, got %d", len(bs))
	}
	b := bs[0]
	insertLog(t, f.db, p.ID, "gpt-4o", 200, "none", 1*time.Hour, 100, 50)

	req := httptest.NewRequest("GET", "/api/routing/bindings/"+strconv.FormatInt(b.ID, 10)+"/metrics", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		AvgLatencyMs int64 `json:"avg_latency_ms"`
		AvgTtftMs    int64 `json:"avg_ttft_ms"`
	}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.AvgLatencyMs != 100 {
		t.Errorf("avg latency: expected 100, got %d", resp.AvgLatencyMs)
	}
	if resp.AvgTtftMs != 50 {
		t.Errorf("avg ttft: expected 50, got %d", resp.AvgTtftMs)
	}
}

func TestPricingEndpoint(t *testing.T) {
	f := newCatalogFixture(t)
	st := settings.New(f.db)
	h := NewHandler(f.db, f.providers, f.models, f.prices, nil)
	h.SetSettings(st)

	// 先建一个模型价格(USD)
	prov, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	m, _ := f.models.Create("m1", prov.ID, "gpt-4o")
	if _, err := f.prices.Set(m.ID, 1.0, 1.0, nil, nil, "USD"); err != nil {
		t.Fatalf("set price: %v", err)
	}

	// GET -> 默认 USD + 预设列表
	rec := httptest.NewRecorder()
	h.GetPricing(rec, httptest.NewRequest("GET", "/api/settings/pricing", nil))
	if rec.Code != 200 {
		t.Fatalf("get pricing code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["currency"] != "USD" {
		t.Errorf("currency = %v, want USD", resp["currency"])
	}
	if presets, ok := resp["presets"].([]any); !ok || len(presets) < 5 {
		t.Errorf("presets missing: %+v", resp["presets"])
	}

	// PUT -> 切换为 EUR(小写自动归一化)
	body, _ := json.Marshal(map[string]string{"currency": "eur"})
	rec = httptest.NewRecorder()
	h.UpdatePricing(rec, httptest.NewRequest("PUT", "/api/settings/pricing", bytes.NewReader(body)))
	if rec.Code != 200 {
		t.Fatalf("update pricing code=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := h.currency(); got != "EUR" {
		t.Errorf("system currency = %q, want EUR", got)
	}
	// 已有模型价格应被迁移为新币种
	pc, err := f.prices.GetCurrent(m.ID)
	if err != nil || pc.Currency != "EUR" {
		t.Errorf("migrated currency = %q err=%v, want EUR", pc.Currency, err)
	}

	// 非法币种 -> 400
	bad, _ := json.Marshal(map[string]string{"currency": "123"})
	rec = httptest.NewRecorder()
	h.UpdatePricing(rec, httptest.NewRequest("PUT", "/api/settings/pricing", bytes.NewReader(bad)))
	if rec.Code != 400 {
		t.Errorf("invalid currency code=%d, want 400", rec.Code)
	}
}

func TestModelQuotaHandler(t *testing.T) {
	f := newCatalogFixture(t)
	c, _ := crypto.New(bytes.Repeat([]byte{1}, 32))
	us := user.New(f.db, c)
	h := NewHandler(f.db, f.providers, f.models, f.prices, nil)
	h.SetUsers(us)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Post("/api/models", h.CreateModel)
	r.With(middleware.RequireRole("admin")).Get("/api/models", h.ListModels)
	r.With(middleware.RequireRole("admin")).Put("/api/models/{id}", h.UpdateModel)
	r.With(middleware.RequireRole("admin")).Delete("/api/models/{id}", h.DeleteModel)
	prov, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "sk-1", "openai_chat")

	// 创建模型并携带配额
	var lim int64 = 1000000
	var cost float64 = 10.0
	body, _ := json.Marshal(map[string]any{
		"name": "my-gpt4", "provider_id": prov.ID, "upstream_model": "gpt-4o",
		"currency": "USD", "input_price": 2.5, "output_price": 10.0,
		"quota": map[string]any{"period": "month", "limit_tokens": lim, "limit_cost": cost},
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("POST", "/api/models", bytes.NewReader(body)).WithContext(adminCtx()))
	if rec.Code != 200 {
		t.Fatalf("create model code=%d body=%s", rec.Code, rec.Body.String())
	}

	// 列表应包含配额
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/api/models", nil).WithContext(adminCtx()))
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list len=%d", len(list))
	}
	qm, ok := list[0]["quota"].(map[string]any)
	if !ok {
		t.Fatalf("quota missing in list: %+v", list[0])
	}
	if qm["limit_tokens"] != float64(lim) || qm["limit_cost"] != cost || qm["period"] != "month" {
		t.Errorf("quota = %+v", qm)
	}
	mid := int64(list[0]["id"].(float64))

	// 编辑模型更新配额(upsert)
	var newLim int64 = 500000
	ubody, _ := json.Marshal(map[string]any{
		"name": "my-gpt4", "enabled": true, "currency": "USD", "input_price": 2.5, "output_price": 10.0,
		"quota": map[string]any{"period": "total", "limit_tokens": newLim, "limit_cost": nil},
	})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("PUT", "/api/models/"+strconv.FormatInt(mid, 10), bytes.NewReader(ubody)).WithContext(adminCtx()))
	if rec.Code != 200 {
		t.Fatalf("update model code=%d body=%s", rec.Code, rec.Body.String())
	}
	qs, _ := us.GetQuotas("model", mid)
	if len(qs) != 1 {
		t.Fatalf("expected 1 model quota row, got %d", len(qs))
	}
	if *qs[0].LimitTokens != newLim || qs[0].LimitCost != nil || qs[0].Period != "total" {
		t.Errorf("updated quota = %+v", qs[0])
	}

	// 清空配额(两个 limit 均为 nil) -> 记录删除
	cbody, _ := json.Marshal(map[string]any{
		"name": "my-gpt4", "enabled": true, "currency": "USD", "input_price": 2.5, "output_price": 10.0,
		"quota": map[string]any{"period": "total", "limit_tokens": nil, "limit_cost": nil},
	})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("PUT", "/api/models/"+strconv.FormatInt(mid, 10), bytes.NewReader(cbody)).WithContext(adminCtx()))
	if rec.Code != 200 {
		t.Fatalf("clear quota code=%d body=%s", rec.Code, rec.Body.String())
	}
	if qs, _ := us.GetQuotas("model", mid); len(qs) != 0 {
		t.Fatalf("expected quota cleared, got %+v", qs)
	}

	// 删除模型 -> 配额一并清理(先重新设置一条再删)
	rb, _ := json.Marshal(map[string]any{
		"name": "my-gpt4", "enabled": true, "currency": "USD", "input_price": 2.5, "output_price": 10.0,
		"quota": map[string]any{"period": "total", "limit_tokens": newLim, "limit_cost": nil},
	})
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("PUT", "/api/models/"+strconv.FormatInt(mid, 10), bytes.NewReader(rb)).WithContext(adminCtx()))
	if rec.Code != 200 {
		t.Fatalf("reset quota code=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("DELETE", "/api/models/"+strconv.FormatInt(mid, 10), nil).WithContext(adminCtx()))
	if rec.Code != 200 {
		t.Fatalf("delete model code=%d body=%s", rec.Code, rec.Body.String())
	}
	if qs, _ := us.GetQuotas("model", mid); len(qs) != 0 {
		t.Fatalf("expected quota deleted with model, got %+v", qs)
	}
}
