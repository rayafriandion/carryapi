package catalog

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	providers *ProviderStore
	models    *ModelStore
	prices    *PriceStore
	prober    *Prober
}

func NewHandler(providers *ProviderStore, models *ModelStore, prices *PriceStore) *Handler {
	return &Handler{providers: providers, models: models, prices: prices, prober: NewProber(nil)}
}

func (h *Handler) SetProber(p *Prober) { h.prober = p }

func jsonOut(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func jsonErr(w http.ResponseWriter, status int, msg string) {
	jsonOut(w, status, map[string]string{"error": msg})
}

// parseIDParam 解析 URL 中的数字 id,非法时写 400 并返回 (0, false)。
func parseIDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "invalid id")
		return 0, false
	}
	return id, true
}

// ---- providers ----

func (h *Handler) ListProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.providers.List()
	if err != nil {
		jsonErr(w, 500, "failed to list providers")
		return
	}
	out := make([]map[string]any, 0, len(providers))
	for _, p := range providers {
		out = append(out, map[string]any{
			"id": p.ID, "name": p.Name, "base_url": p.BaseURL, "protocol": p.Protocol,
			"status": p.Status, "created_at": p.CreatedAt,
		})
	}
	jsonOut(w, 200, out)
}

func (h *Handler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		BaseURL  string `json:"base_url"`
		APIKey   string `json:"api_key"`
		Protocol string `json:"protocol"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	p, err := h.providers.Create(req.Name, req.BaseURL, req.APIKey, req.Protocol)
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]any{"id": p.ID, "name": p.Name, "protocol": p.Protocol})
}

func (h *Handler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	var req struct {
		Name     string `json:"name"`
		BaseURL  string `json:"base_url"`
		APIKey   string `json:"api_key"`
		Protocol string `json:"protocol"`
		Status   string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	if err := h.providers.Update(id, req.Name, req.BaseURL, req.APIKey, req.Protocol, req.Status); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if err := h.providers.Delete(id); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]string{"status": "ok"})
}

// FetchProviderModels 拉取某供应商的上游模型列表(不落库),标注是否已存在。
func (h *Handler) FetchProviderModels(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	provider, err := h.providers.Get(id)
	if err != nil {
		jsonErr(w, 400, "provider not found")
		return
	}
	names, err := h.prober.FetchModels(r.Context(), provider)
	if err != nil {
		jsonErr(w, 502, "failed to fetch models: "+err.Error())
		return
	}
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		_, mErr := h.models.GetByName(name)
		out = append(out, map[string]any{"name": name, "exists": mErr == nil})
	}
	jsonOut(w, 200, map[string]any{"models": out})
}

// ImportModels 批量导入勾选的模型为禁用态草稿(enabled=0),同名跳过。
func (h *Handler) ImportModels(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []struct {
			ProviderID    int64  `json:"provider_id"`
			UpstreamModel string `json:"upstream_model"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	imported := 0
	var skipped []string
	for _, it := range req.Items {
		if it.UpstreamModel == "" {
			continue
		}
		if _, err := h.models.GetByName(it.UpstreamModel); err == nil {
			skipped = append(skipped, it.UpstreamModel)
			continue
		}
		if _, err := h.models.CreateDraft(it.ProviderID, it.UpstreamModel); err != nil {
			continue
		}
		imported++
	}
	jsonOut(w, 200, map[string]any{"imported": imported, "skipped": len(skipped), "skipped_names": skipped})
}

// TestProvider 测某供应商连通性/延迟。
func (h *Handler) TestProvider(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	provider, err := h.providers.Get(id)
	if err != nil {
		jsonErr(w, 400, "provider not found")
		return
	}
	latency, err := h.prober.Ping(r.Context(), provider)
	if err != nil {
		jsonOut(w, 200, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	jsonOut(w, 200, map[string]any{"ok": true, "latency_ms": latency.Milliseconds()})
}

// ---- models ----

func (h *Handler) ListModels(w http.ResponseWriter, r *http.Request) {
	models, err := h.models.List()
	if err != nil {
		jsonErr(w, 500, "failed to list models")
		return
	}
	out := make([]map[string]any, 0, len(models))
	for _, m := range models {
		out = append(out, map[string]any{
			"id": m.ID, "name": m.Name, "provider_id": m.ProviderID,
			"upstream_model": m.UpstreamModel, "enabled": m.Enabled, "created_at": m.CreatedAt,
		})
	}
	jsonOut(w, 200, out)
}

func (h *Handler) CreateModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string `json:"name"`
		ProviderID    int64  `json:"provider_id"`
		UpstreamModel string `json:"upstream_model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	m, err := h.models.Create(req.Name, req.ProviderID, req.UpstreamModel)
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]any{"id": m.ID, "name": m.Name})
}

func (h *Handler) UpdateModel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	var req struct {
		Name          string `json:"name"`
		ProviderID    int64  `json:"provider_id"`
		UpstreamModel string `json:"upstream_model"`
		Enabled       *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if err := h.models.Update(id, req.Name, req.ProviderID, req.UpstreamModel, enabled); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) DeleteModel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if err := h.models.Delete(id); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]string{"status": "ok"})
}

// ---- prices ----

func (h *Handler) GetModelPrice(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	price, err := h.prices.GetCurrent(id)
	if err != nil {
		if err == ErrNoPrice {
			jsonOut(w, 200, map[string]any{"model_id": id, "price": nil})
			return
		}
		jsonErr(w, 500, "failed to get price")
		return
	}
	// Price has no JSON tags, so marshal into a lowercase-key map for a
	// stable API shape (matches the handler_test contract).
	jsonOut(w, 200, map[string]any{
		"model_id": id,
		"price": map[string]any{
			"id":                price.ID,
			"model_id":          price.ModelID,
			"input_price":       price.InputPrice,
			"output_price":      price.OutputPrice,
			"cache_read_price":  price.CacheReadPrice,
			"cache_write_price": price.CacheWritePrice,
			"currency":          price.Currency,
			"effective_from":    price.EffectiveFrom,
		},
	})
}

func (h *Handler) SetModelPrice(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	var req struct {
		InputPrice      float64  `json:"input_price"`
		OutputPrice     float64  `json:"output_price"`
		CacheReadPrice  *float64 `json:"cache_read_price"`
		CacheWritePrice *float64 `json:"cache_write_price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	price, err := h.prices.Set(id, req.InputPrice, req.OutputPrice, req.CacheReadPrice, req.CacheWritePrice)
	if err != nil {
		jsonErr(w, 500, "failed to set price")
		return
	}
	jsonOut(w, 200, map[string]any{"id": price.ID, "model_id": id})
}
