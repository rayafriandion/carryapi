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
}

func NewHandler(providers *ProviderStore, models *ModelStore, prices *PriceStore) *Handler {
	return &Handler{providers: providers, models: models, prices: prices}
}

func jsonOut(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func jsonErr(w http.ResponseWriter, status int, msg string) {
	jsonOut(w, status, map[string]string{"error": msg})
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
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
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
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := h.providers.Delete(id); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]string{"status": "ok"})
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
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
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
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := h.models.Delete(id); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]string{"status": "ok"})
}

// ---- prices ----

func (h *Handler) GetModelPrice(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
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
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
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
