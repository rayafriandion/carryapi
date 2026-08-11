package proxy

import (
	"time"

	"carryapi/internal/catalog"
)

// recordStats 写 request_logs + 累加配额。
// 鉴权失败等无用户上下文的请求记日志(user_id/api_key_id 为 NULL,便于错误率统计);配额只累加成功请求。
func (p *Proxy) recordStats(rc *requestContext) {
	var userID, apiKeyID any
	if rc.user != nil {
		userID = rc.user.ID
		apiKeyID = rc.apiKeyID
	}
	cost := computeCost(rc.price, rc)
	upstreamModel := ""
	if rc.model != nil {
		upstreamModel = rc.model.UpstreamModel
	}
	var providerID any
	if rc.provider != nil {
		providerID = rc.provider.ID
	}
	modelName := ""
	if rc.model != nil {
		modelName = rc.model.Name
	} else if rc.requestedModel != "" {
		modelName = rc.requestedModel
	}
	var durationMs int64
	if !rc.start.IsZero() {
		durationMs = time.Since(rc.start).Milliseconds()
	}
	errorType := rc.errorType
	if errorType == "" {
		errorType = "none"
	}
	var errorMessage any
	if rc.errorMessage != "" {
		errorMessage = rc.errorMessage
	}
	_, _ = p.deps.DB.Exec(
		`INSERT INTO request_logs(request_id, user_id, api_key_id, custom_model, provider_id, upstream_model,
		 protocol_in, protocol_out, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
		 cost, duration_ms, status_code, error_type, error_message, stream)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rc.requestID, userID, apiKeyID, modelName, providerID, upstreamModel,
		rc.downstream, protocolOutName(rc.provider), rc.inputTokens, rc.outputTokens,
		rc.cacheRead, rc.cacheCreation, cost, durationMs, rc.statusCode, errorType, errorMessage, rc.stream)

	// 配额累加(仅成功且有用户上下文)
	if rc.user != nil && rc.statusCode == 200 {
		p.deps.Users.IncrementUsage("user", rc.user.ID, int64(rc.inputTokens+rc.outputTokens), cost)
		p.deps.Users.IncrementUsage("key", rc.apiKeyID, int64(rc.inputTokens+rc.outputTokens), cost)
	}
}

func protocolOutName(p *catalog.Provider) string {
	if p == nil {
		return ""
	}
	return p.Protocol
}

// computeCost 每百万 token 计价:price(每百万) * tokens / 1e6;
// cache_read 用 cache_read_price(若无用 input_price),cache_creation 用 cache_write_price(若无用 input_price)。
func computeCost(price *catalog.Price, rc *requestContext) float64 {
	if price == nil {
		return 0
	}
	inputRate := price.InputPrice
	outputRate := price.OutputPrice
	cacheReadRate := price.InputPrice
	if price.CacheReadPrice != nil {
		cacheReadRate = *price.CacheReadPrice
	}
	cacheWriteRate := price.InputPrice
	if price.CacheWritePrice != nil {
		cacheWriteRate = *price.CacheWritePrice
	}
	return inputRate*float64(rc.inputTokens)/1e6 +
		outputRate*float64(rc.outputTokens)/1e6 +
		cacheReadRate*float64(rc.cacheRead)/1e6 +
		cacheWriteRate*float64(rc.cacheCreation)/1e6
}
