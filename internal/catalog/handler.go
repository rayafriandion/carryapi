package catalog

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	db        *sql.DB
	providers *ProviderStore
	models    *ModelStore
	bindings  *ModelBindingStore
	prices    *PriceStore
	prober    *Prober
	stats     *RoutingStats
}

func NewHandler(db *sql.DB, providers *ProviderStore, models *ModelStore, prices *PriceStore, stats *RoutingStats) *Handler {
	return &Handler{
		db:        db,
		providers: providers,
		models:    models,
		bindings:  NewModelBindingStore(db),
		prices:    prices,
		prober:    NewProber(nil),
		stats:     stats,
	}
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
	failed := 0
	var errors []string
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
			failed++
			errors = append(errors, fmt.Sprintf("%s: %v", it.UpstreamModel, err))
			continue
		}
		imported++
	}
	jsonOut(w, 200, map[string]any{"imported": imported, "failed": failed, "errors": errors, "skipped": len(skipped), "skipped_names": skipped})
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
		bindings, _ := h.bindings.ListByModel(m.ID)
		entry := map[string]any{
			"id": m.ID, "name": m.Name, "provider_id": m.ProviderID,
			"upstream_model": m.UpstreamModel, "enabled": m.Enabled,
			"routing_strategy": m.RoutingStrategy, "auto_mode": m.AutoMode,
			"bindings": h.bindingMaps(bindings), "created_at": m.CreatedAt,
		}
		if price, pErr := h.prices.GetCurrent(m.ID); pErr == nil {
			entry["price"] = h.priceMap(price)
		} else {
			entry["price"] = nil
		}
		out = append(out, entry)
	}
	jsonOut(w, 200, out)
}

type modelPriceReq struct {
	Currency        string   `json:"currency"`
	InputPrice      *float64 `json:"input_price"`
	OutputPrice     *float64 `json:"output_price"`
	CacheReadPrice  *float64 `json:"cache_read_price"`
	CacheWritePrice *float64 `json:"cache_write_price"`
}

func (p modelPriceReq) validate() (input, output float64, cacheRead, cacheWrite *float64, currency string, err error) {
	if p.Currency == "" {
		return 0, 0, nil, nil, "", fmt.Errorf("currency is required")
	}
	if !validCurrencies[p.Currency] {
		return 0, 0, nil, nil, "", fmt.Errorf("invalid currency %q", p.Currency)
	}
	if p.InputPrice == nil {
		return 0, 0, nil, nil, "", fmt.Errorf("input_price is required")
	}
	if p.OutputPrice == nil {
		return 0, 0, nil, nil, "", fmt.Errorf("output_price is required")
	}
	if *p.InputPrice < 0 || *p.OutputPrice < 0 {
		return 0, 0, nil, nil, "", fmt.Errorf("price must be non-negative")
	}
	return *p.InputPrice, *p.OutputPrice, p.CacheReadPrice, p.CacheWritePrice, p.Currency, nil
}

// priceChanged 判断新价格与当前价格是否不同(决定是否追加历史行)。
func priceChanged(current Price, input, output float64, cacheRead, cacheWrite *float64, currency string) bool {
	if current.InputPrice != input || current.OutputPrice != output || current.Currency != currency {
		return true
	}
	if !floatPtrEqual(current.CacheReadPrice, cacheRead) || !floatPtrEqual(current.CacheWritePrice, cacheWrite) {
		return true
	}
	return false
}

func floatPtrEqual(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (h *Handler) CreateModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            string `json:"name"`
		ProviderID      int64  `json:"provider_id"`
		UpstreamModel   string `json:"upstream_model"`
		RoutingStrategy string `json:"routing_strategy"`
		AutoMode        string `json:"auto_mode"`
		Enabled         *bool  `json:"enabled"`
		modelPriceReq
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	input, output, cacheRead, cacheWrite, currency, err := req.modelPriceReq.validate()
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	tx, err := h.db.Begin()
	if err != nil {
		jsonErr(w, 500, "failed to create model")
		return
	}
	defer tx.Rollback()
	id, err := h.models.CreateInTx(tx, req.Name, req.ProviderID, req.UpstreamModel, enabled)
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	if req.RoutingStrategy != "" || req.AutoMode != "" {
		if err := h.models.UpdateRoutingTx(tx, id, req.RoutingStrategy, req.AutoMode); err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
	}
	if _, err := h.prices.SetTx(tx, id, input, output, cacheRead, cacheWrite, currency); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		jsonErr(w, 500, "failed to create model")
		return
	}
	m, _ := h.models.Get(id)
	jsonOut(w, 200, map[string]any{"id": m.ID, "name": m.Name})
}

func (h *Handler) UpdateModel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	var req struct {
		Name            string `json:"name"`
		ProviderID      int64  `json:"provider_id"`
		UpstreamModel   string `json:"upstream_model"`
		Enabled         *bool  `json:"enabled"`
		RoutingStrategy string `json:"routing_strategy"`
		AutoMode        string `json:"auto_mode"`
		modelPriceReq
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	if _, err := h.models.Get(id); err != nil {
		jsonErr(w, 404, "model not found")
		return
	}
	input, output, cacheRead, cacheWrite, currency, err := req.modelPriceReq.validate()
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	tx, err := h.db.Begin()
	if err != nil {
		jsonErr(w, 500, "failed to update model")
		return
	}
	defer tx.Rollback()
	if err := h.models.UpdateInTx(tx, id, req.Name, req.ProviderID, req.UpstreamModel, enabled); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	if req.RoutingStrategy != "" || req.AutoMode != "" {
		if err := h.models.UpdateRoutingTx(tx, id, req.RoutingStrategy, req.AutoMode); err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
	}
	current, pErr := h.prices.GetCurrent(id)
	if pErr != nil || priceChanged(current, input, output, cacheRead, cacheWrite, currency) {
		if _, err := h.prices.SetTx(tx, id, input, output, cacheRead, cacheWrite, currency); err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
	}
	if err := tx.Commit(); err != nil {
		jsonErr(w, 500, "failed to update model")
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

func (h *Handler) ListModelBindings(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	bindings, err := h.bindings.ListByModel(id)
	if err != nil {
		jsonErr(w, 500, "failed to list bindings")
		return
	}
	jsonOut(w, 200, map[string]any{"bindings": h.bindingMaps(bindings)})
}

func (h *Handler) CreateModelBinding(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	var req struct {
		ProviderID    int64  `json:"provider_id"`
		UpstreamModel string `json:"upstream_model"`
		Priority      int    `json:"priority"`
		Weight        int    `json:"weight"`
		Enabled       *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	if _, err := h.models.Get(id); err != nil {
		jsonErr(w, 404, "model not found")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	b, err := h.bindings.Create(id, req.ProviderID, req.UpstreamModel, req.Priority, req.Weight, enabled)
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, h.bindingMap(b))
}

func (h *Handler) UpdateModelBinding(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	bindingID, err := strconv.ParseInt(chi.URLParam(r, "bindingID"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "invalid binding id")
		return
	}
	var req struct {
		ProviderID    int64  `json:"provider_id"`
		UpstreamModel string `json:"upstream_model"`
		Priority      int    `json:"priority"`
		Weight        int    `json:"weight"`
		Enabled       *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	b, err := h.bindings.Get(bindingID)
	if err != nil || b.ModelID != id {
		jsonErr(w, 404, "binding not found")
		return
	}
	if req.ProviderID == 0 {
		req.ProviderID = b.ProviderID
	}
	if req.UpstreamModel == "" {
		req.UpstreamModel = b.UpstreamModel
	}
	if req.Priority == 0 {
		req.Priority = b.Priority
	}
	if req.Weight == 0 {
		req.Weight = b.Weight
	}
	enabled := b.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if err := h.bindings.Update(bindingID, req.ProviderID, req.UpstreamModel, req.Priority, req.Weight, enabled); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	b, _ = h.bindings.Get(bindingID)
	jsonOut(w, 200, h.bindingMap(b))
}

func (h *Handler) DeleteModelBinding(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	bindingID, err := strconv.ParseInt(chi.URLParam(r, "bindingID"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "invalid binding id")
		return
	}
	b, err := h.bindings.Get(bindingID)
	if err != nil || b.ModelID != id {
		jsonErr(w, 404, "binding not found")
		return
	}
	count, err := h.bindings.CountByModel(id)
	if err != nil {
		jsonErr(w, 500, "failed to count bindings")
		return
	}
	if count <= 1 {
		jsonErr(w, 400, "model must have at least one upstream binding")
		return
	}
	if err := h.bindings.Delete(bindingID); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) UpdateModelRouting(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	var req struct {
		RoutingStrategy string `json:"routing_strategy"`
		AutoMode        string `json:"auto_mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	if err := h.models.UpdateRouting(id, req.RoutingStrategy, req.AutoMode); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	m, _ := h.models.Get(id)
	jsonOut(w, 200, map[string]any{
		"id": m.ID, "routing_strategy": m.RoutingStrategy, "auto_mode": m.AutoMode,
	})
}

func (h *Handler) bindingMap(b ModelBinding) map[string]any {
	return map[string]any{
		"id": b.ID, "model_id": b.ModelID, "provider_id": b.ProviderID,
		"upstream_model": b.UpstreamModel, "priority": b.Priority,
		"weight": b.Weight, "enabled": b.Enabled, "created_at": b.CreatedAt,
	}
}

func (h *Handler) bindingMaps(bindings []ModelBinding) []map[string]any {
	out := make([]map[string]any, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, h.bindingMap(b))
	}
	return out
}

// ---- prices ----

func (h *Handler) priceMap(price Price) map[string]any {
	return map[string]any{
		"id":                price.ID,
		"model_id":          price.ModelID,
		"input_price":       price.InputPrice,
		"output_price":      price.OutputPrice,
		"cache_read_price":  price.CacheReadPrice,
		"cache_write_price": price.CacheWritePrice,
		"currency":          price.Currency,
		"effective_from":    price.EffectiveFrom,
	}
}

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
	jsonOut(w, 200, map[string]any{"model_id": id, "price": h.priceMap(price)})
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
		Currency        string   `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	price, err := h.prices.Set(id, req.InputPrice, req.OutputPrice, req.CacheReadPrice, req.CacheWritePrice, req.Currency)
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]any{"id": price.ID, "model_id": id, "currency": price.Currency})
}

// ---- routing ----

// parseBindingIDParam parses the {bindingID} URL param. Returns (0, false) and
// writes a 400 on parse failure.
func parseBindingIDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "bindingID"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "invalid binding id")
		return 0, false
	}
	return id, true
}

// GetRoutingStatus 返回所有 enabled model 的 bindings + 6 格时间轴 + 24h 平均延迟。
func (h *Handler) GetRoutingStatus(w http.ResponseWriter, r *http.Request) {
	models, err := h.models.ListEnabled()
	if err != nil {
		jsonErr(w, 500, "failed to list models")
		return
	}
	now := time.Now()
	type bindingOut struct {
		BindingID      int64    `json:"binding_id"`
		ProviderID    int64    `json:"provider_id"`
		ProviderName   string   `json:"provider_name"`
		ProviderStatus string   `json:"provider_status"`
		UpstreamModel  string   `json:"upstream_model"`
		Priority       int      `json:"priority"`
		Weight         int      `json:"weight"`
		Enabled        bool     `json:"enabled"`
		Timeline       []string `json:"timeline"`
		AvgLatencyMs   int64    `json:"avg_latency_ms"`
		LastRequestAt  *string  `json:"last_request_at"`
	}
	type modelOut struct {
		ModelID         int64        `json:"model_id"`
		Name            string       `json:"name"`
		Enabled         bool         `json:"enabled"`
		RoutingStrategy string       `json:"routing_strategy"`
		AutoMode        string       `json:"auto_mode"`
		Bindings        []bindingOut `json:"bindings"`
	}
	out := struct {
		Models []modelOut `json:"models"`
	}{Models: []modelOut{}}

	for _, m := range models {
		bindings, err := h.bindings.ListByModel(m.ID)
		if err != nil {
			continue
		}
		mo := modelOut{
			ModelID:         m.ID,
			Name:            m.Name,
			Enabled:         m.Enabled,
			RoutingStrategy: m.RoutingStrategy,
			AutoMode:        m.AutoMode,
			Bindings:        []bindingOut{},
		}
		for _, b := range bindings {
			provider, err := h.providers.Get(b.ProviderID)
			if err != nil {
				continue
			}
			bo := bindingOut{
				BindingID:      b.ID,
				ProviderID:     b.ProviderID,
				ProviderName:   provider.Name,
				ProviderStatus: provider.Status,
				UpstreamModel:  b.UpstreamModel,
				Priority:       b.Priority,
				Weight:         b.Weight,
				Enabled:        b.Enabled,
				Timeline:       []string{},
			}
			if h.stats != nil {
				tl, err := h.stats.BindingTimeline(b.ProviderID, b.UpstreamModel, now)
				if err == nil {
					bo.AvgLatencyMs = tl.AvgLatencyMs
					for _, bk := range tl.Buckets {
						bo.Timeline = append(bo.Timeline, bk.Status)
					}
					if tl.LastRequestAt != nil {
						s := tl.LastRequestAt.Format(time.RFC3339)
						bo.LastRequestAt = &s
					}
				}
			}
			mo.Bindings = append(mo.Bindings, bo)
		}
		out.Models = append(out.Models, mo)
	}
	jsonOut(w, 200, out)
}

// GetBindingMetrics 返回某 binding 的 24h 性能详情。
func (h *Handler) GetBindingMetrics(w http.ResponseWriter, r *http.Request) {
	bindingID, ok := parseBindingIDParam(w, r)
	if !ok {
		return
	}
	binding, err := h.bindings.Get(bindingID)
	if err != nil {
		jsonErr(w, 404, "binding not found")
		return
	}
	provider, err := h.providers.Get(binding.ProviderID)
	if err != nil {
		jsonErr(w, 404, "provider not found")
		return
	}
	m, err := h.stats.BindingMetrics(binding.ProviderID, binding.UpstreamModel, time.Now())
	if err != nil {
		jsonErr(w, 500, "failed to compute metrics")
		return
	}
	jsonOut(w, 200, map[string]any{
		"binding_id":          binding.ID,
		"provider_id":         binding.ProviderID,
		"provider_name":       provider.Name,
		"upstream_model":      binding.UpstreamModel,
		"avg_latency_ms":      m.AvgLatencyMs,
		"avg_ttft_ms":         m.AvgTtftMs,
		"throughput_per_hour": m.ThroughputPerHour,
		"total_requests_24h":  m.TotalRequests24h,
		"success_rate":        m.SuccessRate,
	})
}
