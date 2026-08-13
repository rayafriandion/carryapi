package catalog

import (
	"encoding/json"
	"net/http"
	"time"
)

type catalogModel struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	UpstreamModel  string   `json:"upstream_model"`
	ProviderName   string   `json:"provider_name"`
	Protocol       string   `json:"protocol"`
	InputPrice     *float64 `json:"input_price"`
	OutputPrice    *float64 `json:"output_price"`
	CacheReadPrice *float64 `json:"cache_read_price"`
	Currency       string   `json:"currency"`
	TotalRequests  int64    `json:"total_requests"`
	SuccessCount   int64    `json:"success_count"`
	SuccessRate    float64  `json:"success_rate"`
	AvgDurationMs  float64  `json:"avg_duration_ms"`
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
		}
		if p, ok := providerByID[m.ProviderID]; ok {
			cm.ProviderName = p.Name
			cm.Protocol = p.Protocol
		}
		if price, pErr := h.prices.GetCurrent(m.ID); pErr == nil {
			in := price.InputPrice
			out := price.OutputPrice
			cm.InputPrice = &in
			cm.OutputPrice = &out
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
