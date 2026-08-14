package catalog

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"carryapi/internal/currency"
	"carryapi/internal/settings"
	"carryapi/internal/user"
)

type Handler struct {
	db        *sql.DB
	providers *ProviderStore
	models    *ModelStore
	bindings  *ModelBindingStore
	prices    *PriceStore
	prober    *Prober
	stats     *RoutingStats
	users     *user.Store     // 可选:配置后模型配额随模型创建/编辑/删除持久化
	settings  *settings.Store // 可选:配置后系统统一币种用于定价与展示
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

// SetUsers 注入 user.Store(模型配额 CRUD 用)。不设置时模型配额相关操作被跳过。
func (h *Handler) SetUsers(u *user.Store) { h.users = u }

// SetSettings 注入 settings.Store(读取系统统一币种)。
func (h *Handler) SetSettings(s *settings.Store) { h.settings = s }

// currency 返回系统统一币种代码;settings 未配置或读取失败时回退到默认 USD。
func (h *Handler) currency() string {
	if h.settings != nil {
		if c, err := h.settings.Currency(); err == nil && c != "" {
			return c
		}
	}
	return currency.Default
}

// modelQuotaReq 模型配额(创建/编辑时可选)。limit_tokens/limit_cost 为 nil 表示该维度不限制。
type modelQuotaReq struct {
	Period      string   `json:"period"`
	LimitTokens *int64   `json:"limit_tokens"`
	LimitCost   *float64 `json:"limit_cost"`
}

func (q *modelQuotaReq) validate() error {
	if q == nil {
		return nil
	}
	if q.Period != "" && q.Period != "total" && q.Period != "month" {
		return fmt.Errorf("invalid quota period %q", q.Period)
	}
	if q.LimitTokens != nil && *q.LimitTokens < 0 {
		return fmt.Errorf("limit_tokens must be non-negative")
	}
	if q.LimitCost != nil && *q.LimitCost < 0 {
		return fmt.Errorf("limit_cost must be non-negative")
	}
	return nil
}

// applyModelQuota 持久化模型配额(users store 未配置或请求未携带 quota 时跳过)。
func (h *Handler) applyModelQuota(modelID int64, q *modelQuotaReq) error {
	if h.users == nil || q == nil {
		return nil
	}
	_, err := h.users.SetModelQuota(modelID, q.Period, q.LimitTokens, q.LimitCost)
	return err
}

func (h *Handler) quotaMap(q user.Quota) map[string]any {
	return map[string]any{
		"id": q.ID, "scope": q.Scope, "scope_id": q.ScopeID, "period": q.Period,
		"limit_tokens": q.LimitTokens, "limit_cost": q.LimitCost,
		"used_tokens": q.UsedTokens, "used_cost": q.UsedCost,
	}
}

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
		keyCount, activeCount, coolingCount := 0, 0, 0
		if keys, kErr := h.providers.Keys(p.ID); kErr == nil {
			keyCount = len(keys)
			for _, k := range keys {
				switch k.Status {
				case KeyStatusActive:
					activeCount++
				case KeyStatusCoolingDown:
					coolingCount++
				}
			}
		}
		out = append(out, map[string]any{
			"id": p.ID, "name": p.Name, "base_url": p.BaseURL, "protocol": p.Protocol,
			"status": p.Status, "created_at": p.CreatedAt,
			"key_count": keyCount, "active_key_count": activeCount, "cooling_key_count": coolingCount,
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

// ---- provider api keys(多 key 池)----

// ListProviderKeys 返回某供应商的全部上游 key(掩码展示,不暴露明文)。
func (h *Handler) ListProviderKeys(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	if _, err := h.providers.Get(id); err != nil {
		jsonErr(w, 404, "provider not found")
		return
	}
	keys, err := h.providers.Keys(id)
	if err != nil {
		jsonErr(w, 500, "failed to list provider api keys")
		return
	}
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, h.providerKeyMap(k))
	}
	jsonOut(w, 200, map[string]any{"provider_id": id, "keys": out})
}

func (h *Handler) providerKeyMap(k ProviderAPIKey) map[string]any {
	m := map[string]any{
		"id": k.ID, "provider_id": k.ProviderID, "label": k.Label,
		"masked": MaskKey(k.APIKey), "priority": k.Priority, "base_priority": k.BasePriority,
		"status": k.Status, "fail_count": k.FailCount,
		"created_at": k.CreatedAt,
	}
	if k.RetryAfter != nil {
		m["retry_after"] = k.RetryAfter
	}
	if k.LastUsedAt != nil {
		m["last_used_at"] = k.LastUsedAt
	}
	if k.DeletedAt != nil {
		m["deleted_at"] = k.DeletedAt
	}
	return m
}

// AddProviderKey 为供应商新增一个上游 key。
func (h *Handler) AddProviderKey(w http.ResponseWriter, r *http.Request) {
	id, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	var req struct {
		APIKey string `json:"api_key"`
		Label  string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	if _, err := h.providers.Get(id); err != nil {
		jsonErr(w, 404, "provider not found")
		return
	}
	k, err := h.providers.AddKey(id, req.APIKey, req.Label)
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, h.providerKeyMap(k))
}

// UpdateProviderKey 更新 key 的标签/基准优先级。
func (h *Handler) UpdateProviderKey(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	keyID, err := strconv.ParseInt(chi.URLParam(r, "keyID"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "invalid key id")
		return
	}
	k, err := h.providers.GetKey(keyID)
	if err != nil || k.ProviderID != pid {
		jsonErr(w, 404, "provider api key not found")
		return
	}
	var req struct {
		Label    string `json:"label"`
		Priority *int   `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	if err := h.providers.UpdateKeyMeta(keyID, req.Label, req.Priority); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]string{"status": "ok"})
}

// DeleteProviderKey 手动删除某上游 key(软删除,保留日志)。
func (h *Handler) DeleteProviderKey(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	keyID, err := strconv.ParseInt(chi.URLParam(r, "keyID"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "invalid key id")
		return
	}
	k, err := h.providers.GetKey(keyID)
	if err != nil || k.ProviderID != pid {
		jsonErr(w, 404, "provider api key not found")
		return
	}
	if err := h.providers.DeleteKey(keyID, true, "deleted manually by admin"); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]string{"status": "ok"})
}

// ProviderKeyLogs 返回某上游 key 的"API key 调用日志":
// 生命周期事件(创建/降级/冷却/重试/恢复/删除) + 近期使用该 key 的请求日志。
func (h *Handler) ProviderKeyLogs(w http.ResponseWriter, r *http.Request) {
	pid, ok := parseIDParam(w, r)
	if !ok {
		return
	}
	keyID, err := strconv.ParseInt(chi.URLParam(r, "keyID"), 10, 64)
	if err != nil {
		jsonErr(w, 400, "invalid key id")
		return
	}
	k, err := h.providers.GetKey(keyID)
	if err != nil || k.ProviderID != pid {
		jsonErr(w, 404, "provider api key not found")
		return
	}
	events, err := h.providers.KeyEvents(keyID)
	if err != nil {
		jsonErr(w, 500, "failed to load key events")
		return
	}
	requests, err := h.providerKeyRequests(keyID)
	if err != nil {
		jsonErr(w, 500, "failed to load key requests")
		return
	}
	jsonOut(w, 200, map[string]any{
		"key": h.providerKeyMap(k), "events": events, "requests": requests,
	})
}

// providerKeyRequests 查询最近使用某上游 key 的请求日志(用于 key 调用日志)。
func (h *Handler) providerKeyRequests(keyID int64) ([]map[string]any, error) {
	rows, err := h.db.Query(
		`SELECT rl.request_id, COALESCE(u.email, ''), COALESCE(rl.custom_model, ''), COALESCE(rl.upstream_model, ''),
		        COALESCE(rl.status_code, 0), COALESCE(rl.error_type, ''), COALESCE(rl.error_message, ''),
		        COALESCE(rl.input_tokens, 0), COALESCE(rl.output_tokens, 0), rl.created_at
		 FROM request_logs rl LEFT JOIN users u ON rl.user_id = u.id
		 WHERE rl.provider_api_key_id = ? ORDER BY rl.id DESC LIMIT 100`, keyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var reqID, email, model, upstreamModel, errType, errMsg string
		var status, inTok, outTok int
		var created time.Time
		if err := rows.Scan(&reqID, &email, &model, &upstreamModel, &status, &errType, &errMsg, &inTok, &outTok, &created); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"request_id": reqID, "email": email, "custom_model": model, "upstream_model": upstreamModel,
			"status_code": status, "error_type": errType, "error_message": errMsg,
			"input_tokens": inTok, "output_tokens": outTok, "created_at": created,
		})
	}
	return out, rows.Err()
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
		if h.users != nil {
			if q, qErr := h.users.GetModelQuota(m.ID); qErr == nil && q.ID != 0 {
				entry["quota"] = h.quotaMap(q)
			} else {
				entry["quota"] = nil
			}
		} else {
			entry["quota"] = nil
		}
		out = append(out, entry)
	}
	jsonOut(w, 200, out)
}

type modelPriceReq struct {
	Currency        string   `json:"currency"` // 已弃用:币种由系统统一设置,此处仅保留以兼容旧请求
	InputPrice      *float64 `json:"input_price"`
	OutputPrice     *float64 `json:"output_price"`
	CacheReadPrice  *float64 `json:"cache_read_price"`
	CacheWritePrice *float64 `json:"cache_write_price"`
}

func (p modelPriceReq) validate() (input, output float64, cacheRead, cacheWrite *float64, err error) {
	if p.InputPrice == nil {
		return 0, 0, nil, nil, fmt.Errorf("input_price is required")
	}
	if p.OutputPrice == nil {
		return 0, 0, nil, nil, fmt.Errorf("output_price is required")
	}
	if *p.InputPrice < 0 || *p.OutputPrice < 0 {
		return 0, 0, nil, nil, fmt.Errorf("price must be non-negative")
	}
	return *p.InputPrice, *p.OutputPrice, p.CacheReadPrice, p.CacheWritePrice, nil
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
		Bindings        []struct {
			ProviderID    int64  `json:"provider_id"`
			UpstreamModel string `json:"upstream_model"`
			Priority      int    `json:"priority"`
			Weight        int    `json:"weight"`
			Enabled       *bool  `json:"enabled"`
		} `json:"bindings"`
		Quota *modelQuotaReq `json:"quota"`
		modelPriceReq
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	if err := req.Quota.validate(); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	input, output, cacheRead, cacheWrite, err := req.modelPriceReq.validate()
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		jsonErr(w, 400, "name is required")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	// 归一化 bindings:若未传数组,则用顶层 provider_id/upstream_model 构造一条
	type bentry struct {
		ProviderID    int64
		UpstreamModel string
		Priority      int
		Weight        int
		Enabled       bool
	}
	var entries []bentry
	if len(req.Bindings) > 0 {
		for i, b := range req.Bindings {
			bEnabled := true
			if b.Enabled != nil {
				bEnabled = *b.Enabled
			}
			entries = append(entries, bentry{
				ProviderID:    b.ProviderID,
				UpstreamModel: strings.TrimSpace(b.UpstreamModel),
				Priority:      b.Priority,
				Weight:        b.Weight,
				Enabled:       bEnabled,
			})
			if entries[i].ProviderID <= 0 {
				jsonErr(w, 400, fmt.Sprintf("bindings[%d].provider_id is required", i))
				return
			}
			if entries[i].UpstreamModel == "" {
				jsonErr(w, 400, fmt.Sprintf("bindings[%d].upstream_model is required", i))
				return
			}
		}
	} else {
		if req.ProviderID <= 0 {
			jsonErr(w, 400, "provider_id is required")
			return
		}
		upstreamModel := strings.TrimSpace(req.UpstreamModel)
		if upstreamModel == "" {
			jsonErr(w, 400, "upstream_model is required")
			return
		}
		entries = append(entries, bentry{
			ProviderID: req.ProviderID, UpstreamModel: upstreamModel,
			Priority: 100, Weight: 1, Enabled: enabled,
		})
	}
	// 校验所有 provider 存在
	providerIDs := map[int64]struct{}{}
	for _, e := range entries {
		providerIDs[e.ProviderID] = struct{}{}
	}
	for pid := range providerIDs {
		if _, err := h.providers.Get(pid); err != nil {
			jsonErr(w, 400, fmt.Sprintf("provider %d not found", pid))
			return
		}
	}

	tx, err := h.db.Begin()
	if err != nil {
		jsonErr(w, 500, "failed to create model")
		return
	}
	defer tx.Rollback()
	// 主记录使用第一条 binding 作为旧字段(provider_id/upstream_model)兼容
	first := entries[0]
	id, err := h.models.CreateInTx(tx, name, first.ProviderID, first.UpstreamModel, enabled)
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	for _, e := range entries {
		if err := h.models.CreateBindingInTx(tx, id, e.ProviderID, e.UpstreamModel, e.Priority, e.Weight, e.Enabled); err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
	}
	if req.RoutingStrategy != "" || req.AutoMode != "" {
		if err := h.models.UpdateRoutingTx(tx, id, req.RoutingStrategy, req.AutoMode); err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
	}
	if _, err := h.prices.SetTx(tx, id, input, output, cacheRead, cacheWrite, h.currency()); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	if err := tx.Commit(); err != nil {
		jsonErr(w, 500, "failed to create model")
		return
	}
	if err := h.applyModelQuota(id, req.Quota); err != nil {
		jsonErr(w, 500, "failed to save quota: "+err.Error())
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
		Name            string         `json:"name"`
		Enabled         *bool          `json:"enabled"`
		RoutingStrategy string         `json:"routing_strategy"`
		AutoMode        string         `json:"auto_mode"`
		Quota           *modelQuotaReq `json:"quota"`
		modelPriceReq
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	if err := req.Quota.validate(); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	if _, err := h.models.Get(id); err != nil {
		jsonErr(w, 404, "model not found")
		return
	}
	input, output, cacheRead, cacheWrite, err := req.modelPriceReq.validate()
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	// 读取当前价格需在事务开启前,否则单连接数据库(:memory: 测试)会因事务占用唯一连接而死锁。
	current, pErr := h.prices.GetCurrent(id)

	tx, err := h.db.Begin()
	if err != nil {
		jsonErr(w, 500, "failed to update model")
		return
	}
	defer tx.Rollback()
	if err := h.models.UpdateInTx(tx, id, req.Name, enabled); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	if req.RoutingStrategy != "" || req.AutoMode != "" {
		if err := h.models.UpdateRoutingTx(tx, id, req.RoutingStrategy, req.AutoMode); err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
	}
	if pErr != nil || priceChanged(current, input, output, cacheRead, cacheWrite, h.currency()) {
		if _, err := h.prices.SetTx(tx, id, input, output, cacheRead, cacheWrite, h.currency()); err != nil {
			jsonErr(w, 400, err.Error())
			return
		}
	}
	if err := tx.Commit(); err != nil {
		jsonErr(w, 500, "failed to update model")
		return
	}
	if err := h.applyModelQuota(id, req.Quota); err != nil {
		jsonErr(w, 500, "failed to save quota: "+err.Error())
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
	if h.users != nil {
		if err := h.users.DeleteModelQuota(id); err != nil {
			jsonErr(w, 500, "failed to delete quota: "+err.Error())
			return
		}
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
		"currency":          h.currency(),
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
	price, err := h.prices.Set(id, req.InputPrice, req.OutputPrice, req.CacheReadPrice, req.CacheWritePrice, h.currency())
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]any{"id": price.ID, "model_id": id, "currency": price.Currency})
}

// ---- pricing (系统统一币种设置) ----

// GetPricing 返回系统统一币种与常用货币预设。
func (h *Handler) GetPricing(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, 200, map[string]any{
		"currency": h.currency(),
		"presets":  currency.Presets,
	})
}

// UpdatePricing 设置系统统一币种,并把所有模型价格标注同步为新币种(数值不变)。
func (h *Handler) UpdatePricing(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Currency string `json:"currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	code := currency.Normalize(req.Currency)
	if code == "" {
		jsonErr(w, 400, "currency is required")
		return
	}
	if !currency.Valid(code) {
		jsonErr(w, 400, "invalid currency code (2-8 位大写字母)")
		return
	}
	if h.settings == nil {
		jsonErr(w, 500, "settings store not configured")
		return
	}
	if err := h.settings.SetCurrency(code); err != nil {
		jsonErr(w, 500, "failed to save currency: "+err.Error())
		return
	}
	if _, err := h.db.Exec(`UPDATE model_prices SET currency=?`, code); err != nil {
		jsonErr(w, 500, "failed to migrate model prices: "+err.Error())
		return
	}
	jsonOut(w, 200, map[string]any{"status": "ok", "currency": code})
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
		ProviderID     int64    `json:"provider_id"`
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
