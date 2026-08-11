package proxy

import "time"

// recordStats 将一次代理请求写入 request_logs。
// Task 5 为最小实现:cost 固定 0,不累加配额。Task 6 补充 computeCost 与配额累加。
// 鉴权失败等无用户上下文的请求不记日志(request_logs.user_id/api_key_id 为 NOT NULL)。
func (p *Proxy) recordStats(rc *requestContext) {
	if rc.user == nil {
		return
	}
	modelName := ""
	if rc.model != nil {
		modelName = rc.model.Name
	} else if rc.requestedModel != "" {
		modelName = rc.requestedModel
	}
	var providerID any
	var upstreamModel any
	protocolOut := ""
	if rc.provider != nil {
		providerID = rc.provider.ID
		protocolOut = rc.provider.Protocol
	}
	if rc.model != nil {
		upstreamModel = rc.model.UpstreamModel
	}
	errorType := rc.errorType
	if errorType == "" {
		errorType = "none"
	}
	var errorMessage any
	if rc.errorMessage != "" {
		errorMessage = rc.errorMessage
	}
	_, _ = p.deps.DB.Exec(`
		INSERT INTO request_logs(
			request_id, user_id, api_key_id, custom_model, provider_id, upstream_model,
			protocol_in, protocol_out, input_tokens, output_tokens, cache_read_tokens,
			cache_creation_tokens, cost, duration_ms, status_code, error_type, error_message, stream)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?)`,
		rc.requestID, rc.user.ID, rc.apiKeyID, modelName, providerID, upstreamModel,
		rc.downstream, protocolOut,
		rc.inputTokens, rc.outputTokens, rc.cacheRead, rc.cacheCreation,
		int(time.Since(rc.start).Milliseconds()),
		rc.statusCode, errorType, errorMessage, rc.stream)
}
