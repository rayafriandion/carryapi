package catalog

import (
	"encoding/json"
	"net/http"
	"time"
)

type catalogBinding struct {
	ProviderID    int64  `json:"provider_id"`
	ProviderName  string `json:"provider_name"`
	Protocol      string `json:"protocol"`
	UpstreamModel string `json:"upstream_model"`
	Priority      int    `json:"priority"`
	Weight        int    `json:"weight"`
	Enabled       bool   `json:"enabled"`
}

type catalogModel struct {
	ID             int64            `json:"id"`
	Name           string           `json:"name"`
	UpstreamModel  string           `json:"upstream_model"`
	ProviderName   string           `json:"provider_name"`
	Protocol       string           `json:"protocol"`
	Bindings       []catalogBinding `json:"bindings"`
	InputPrice     *float64         `json:"input_price"`
	OutputPrice    *float64         `json:"output_price"`
	CacheReadPrice *float64         `json:"cache_read_price"`
	Currency       string           `json:"currency"`
	TotalRequests  int64            `json:"total_requests"`
	SuccessCount   int64            `json:"success_count"`
	SuccessRate    float64          `json:"success_rate"`
	AvgDurationMs  float64          `json:"avg_duration_ms"`
}

type catalogBindingDetail struct {
	catalogBinding
	ProviderStatus  string   `json:"provider_status"`
	TotalRequests24h int     `json:"total_requests_24h"`
	SuccessRate24h  float64  `json:"success_rate_24h"`
	AvgLatencyMs    int64    `json:"avg_latency_ms"`
	AvgTtftMs       int64    `json:"avg_ttft_ms"`
	Timeline        []string `json:"timeline"`
	LastRequestAt   *string  `json:"last_request_at"`
}

type catalogModelDetail struct {
	ID              int64                  `json:"id"`
	Name            string                 `json:"name"`
	Enabled         bool                   `json:"enabled"`
	RoutingStrategy string                 `json:"routing_strategy"`
	AutoMode        string                 `json:"auto_mode"`
	CreatedAt       time.Time              `json:"created_at"`
	InputPrice      *float64               `json:"input_price"`
	OutputPrice     *float64               `json:"output_price"`
	CacheReadPrice  *float64               `json:"cache_read_price"`
	CacheWritePrice *float64               `json:"cache_write_price"`
	Currency        string                 `json:"currency"`
	TotalRequests   int64                  `json:"total_requests"`
	SuccessCount    int64                  `json:"success_count"`
	SuccessRate     float64                `json:"success_rate"`
	AvgDurationMs   float64                `json:"avg_duration_ms"`
	Bindings        []catalogBindingDetail `json:"bindings"`
}

// ListCatalog 返回启用的模型 + 价格 + 近 30 天全局统计。
// 任何已登录用户均可访问,不暴露供应商密钥等敏感信息。
func (h *Handler) ListCatalog(w http.ResponseWriter, r *http.Request) {
	models, err := h.models.ListEnabled()
	if err != nil {
		jsonErr(w, 500, "failed to list models")
		return
	}
	providers, err := h.providers.List()
	if err != nil {
		jsonErr(w, 500, "failed to list providers")
		return
	}
	providerByID := make(map[int64]Provider, len(providers))
	for _, p := range providers {
		providerByID[p.ID] = p
	}

	now := time.Now().UTC()
	since := now.Add(-30 * 24 * time.Hour)
	statsByModel, err := h.queryModelStats(since, now)
	if err != nil {
		jsonErr(w, 500, "failed to query stats")
		return
	}
	out := make([]catalogModel, 0, len(models))
	for _, m := range models {
		cm := catalogModel{
			ID:            m.ID,
			Name:          m.Name,
			UpstreamModel: m.UpstreamModel,
			ProviderName:  "unknown",
			Bindings:      []catalogBinding{},
		}
		if p, ok := providerByID[m.ProviderID]; ok {
			cm.ProviderName = p.Name
			cm.Protocol = p.Protocol
		}
		bindings, bErr := h.bindings.ListByModel(m.ID)
		if bErr == nil {
			for _, b := range bindings {
				cb := catalogBinding{
					ProviderID:    b.ProviderID,
					UpstreamModel: b.UpstreamModel,
					Priority:      b.Priority,
					Weight:        b.Weight,
					Enabled:       b.Enabled,
					ProviderName:  "unknown",
				}
				if p, ok := providerByID[b.ProviderID]; ok {
					cb.ProviderName = p.Name
					cb.Protocol = p.Protocol
				}
				cm.Bindings = append(cm.Bindings, cb)
			}
		}
		if price, pErr := h.prices.GetCurrent(m.ID); pErr == nil {
			in := price.InputPrice
			outPrice := price.OutputPrice
			cm.InputPrice = &in
			cm.OutputPrice = &outPrice
			cm.CacheReadPrice = price.CacheReadPrice
			cm.Currency = price.Currency
			if cm.Currency == "" {
				cm.Currency = "USD"
			}
		}
		if st, ok := statsByModel[m.Name]; ok {
			cm.TotalRequests = st.total
			cm.SuccessCount = st.success
			if st.total > 0 {
				cm.SuccessRate = float64(st.success) / float64(st.total) * 100
			}
			cm.AvgDurationMs = st.avgDuration
		}
		out = append(out, cm)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(out)
}

// GetCatalogModel 返回单个启用模型的完整详情,含全部上游绑定及每绑定 24h 指标。
func (h *Handler) GetCatalogModel(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	m, err := h.models.Get(id)
	if err != nil || !m.Enabled {
		jsonErr(w, 404, "model not found")
		return
	}
	providers, err := h.providers.List()
	if err != nil {
		jsonErr(w, 500, "failed to list providers")
		return
	}
	providerByID := make(map[int64]Provider, len(providers))
	for _, p := range providers {
		providerByID[p.ID] = p
	}

	now := time.Now().UTC()
	since := now.Add(-30 * 24 * time.Hour)
	statsByModel, err := h.queryModelStats(since, now)
	if err != nil {
		jsonErr(w, 500, "failed to query stats")
		return
	}

	detail := catalogModelDetail{
		ID:              m.ID,
		Name:            m.Name,
		Enabled:         m.Enabled,
		RoutingStrategy: m.RoutingStrategy,
		AutoMode:        m.AutoMode,
		CreatedAt:       m.CreatedAt,
		Bindings:        []catalogBindingDetail{},
	}
	if price, pErr := h.prices.GetCurrent(m.ID); pErr == nil {
		in := price.InputPrice
		outPrice := price.OutputPrice
		detail.InputPrice = &in
		detail.OutputPrice = &outPrice
		detail.CacheReadPrice = price.CacheReadPrice
		detail.CacheWritePrice = price.CacheWritePrice
		detail.Currency = price.Currency
		if detail.Currency == "" {
			detail.Currency = "USD"
		}
	}
	if st, ok := statsByModel[m.Name]; ok {
		detail.TotalRequests = st.total
		detail.SuccessCount = st.success
		if st.total > 0 {
			detail.SuccessRate = float64(st.success) / float64(st.total) * 100
		}
		detail.AvgDurationMs = st.avgDuration
	}

	bindings, err := h.bindings.ListByModel(m.ID)
	if err != nil {
		jsonErr(w, 500, "failed to list bindings")
		return
	}
	for _, b := range bindings {
		bd := catalogBindingDetail{
			catalogBinding: catalogBinding{
				ProviderID:    b.ProviderID,
				UpstreamModel: b.UpstreamModel,
				Priority:      b.Priority,
				Weight:        b.Weight,
				Enabled:       b.Enabled,
				ProviderName:  "unknown",
			},
		}
		if p, ok := providerByID[b.ProviderID]; ok {
			bd.ProviderName = p.Name
			bd.Protocol = p.Protocol
			bd.ProviderStatus = p.Status
		}
		if h.stats != nil {
			if metrics, mErr := h.stats.BindingMetrics(b.ProviderID, b.UpstreamModel, time.Now()); mErr == nil {
				bd.TotalRequests24h = metrics.TotalRequests24h
				bd.SuccessRate24h = metrics.SuccessRate * 100
				bd.AvgLatencyMs = metrics.AvgLatencyMs
				bd.AvgTtftMs = metrics.AvgTtftMs
			}
			if tl, tErr := h.stats.BindingTimeline(b.ProviderID, b.UpstreamModel, time.Now()); tErr == nil {
				for _, bk := range tl.Buckets {
					bd.Timeline = append(bd.Timeline, bk.Status)
				}
				bd.AvgLatencyMs = tl.AvgLatencyMs
				if tl.LastRequestAt != nil {
					s := tl.LastRequestAt.Format(time.RFC3339)
					bd.LastRequestAt = &s
				}
			}
		}
		detail.Bindings = append(detail.Bindings, bd)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(detail)
}

type modelStatAgg struct {
	total       int64
	success     int64
	avgDuration float64
}

func (h *Handler) queryModelStats(start, end time.Time) (map[string]modelStatAgg, error) {
	rows, err := h.db.Query(`SELECT custom_model, COUNT(*),
		COALESCE(SUM(CASE WHEN status_code BETWEEN 200 AND 299 AND error_type='none' THEN 1 ELSE 0 END),0),
		COALESCE(AVG(duration_ms),0)
		FROM request_logs WHERE created_at >= ? AND created_at <= ?
		GROUP BY custom_model`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]modelStatAgg)
	for rows.Next() {
		var name string
		var s modelStatAgg
		if err := rows.Scan(&name, &s.total, &s.success, &s.avgDuration); err != nil {
			return nil, err
		}
		out[name] = s
	}
	return out, rows.Err()
}
