package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"carryapi/internal/catalog"
	"carryapi/internal/ir"
)

func providerPath(protocol string) string {
	switch protocol {
	case "openai_chat":
		return "/chat/completions"
	case "openai_responses":
		return "/responses"
	case "anthropic":
		return "/v1/messages"
	}
	return "/"
}

func (p *Proxy) buildUpstreamRequest(r *http.Request, provider *catalog.Provider, apiKey string, payload []byte) (*http.Request, error) {
	url := provider.BaseURL + providerPath(provider.Protocol)
	req, err := http.NewRequestWithContext(r.Context(), "POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if provider.Protocol == "anthropic" {
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return req, nil
}

// handleProxy 统一代理入口:鉴权 -> 解码 -> 模型解析 -> 配额 -> 编码 -> 转发。
func (p *Proxy) handleProxy(w http.ResponseWriter, r *http.Request, downstream string) {
	rc := &requestContext{downstream: downstream, requestID: uuid.NewString(), start: time.Now()}
	w.Header().Set("X-Request-Id", rc.requestID)

	// 1. 鉴权
	u, ak, err := p.authenticate(r)
	if err != nil {
		p.writeError(w, rc, asIRError(err))
		return
	}
	rc.user = u
	rc.apiKeyID = ak.ID

	// 2. 读请求体 + 解码
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		p.writeError(w, rc, ir.NewError("invalid_request", "read_failed", "failed to read request body", 400))
		return
	}
	irReq, err := decodeDownstreamRequest(downstream, rawBody)
	if err != nil {
		p.writeError(w, rc, ir.NewError("invalid_request", "parse_failed", "failed to parse request: "+err.Error(), 400))
		return
	}
	rc.requestedModel = irReq.Model

	resolved, err := p.resolveModel(irReq.Model)
	if err != nil {
		p.writeError(w, rc, asIRError(err))
		return
	}
	rc.model = resolved.model
	rc.provider = resolved.provider
	rc.selected = resolved.selected
	rc.candidates = resolved.candidates
	rc.price = resolved.price

	if err := p.checkQuota(u, ak.ID, rc.model.ID); err != nil {
		p.writeError(w, rc, asIRError(err))
		return
	}

	if err := p.forwardSelected(w, r, rc, irReq, downstream, *resolved.selected, resolved.candidates); err != nil {
		p.writeError(w, rc, asIRError(err))
	}
}

// keyRetryLimit 单次请求中对同一上游 key 的瞬时失败重试次数(需求:重试 3 次)。
const keyRetryLimit = 3

func keyRetryBackoff(attempt int) time.Duration {
	return time.Duration(attempt) * 300 * time.Millisecond
}

func (p *Proxy) forwardSelected(w http.ResponseWriter, r *http.Request, rc *requestContext, irReq *ir.Request, downstream string, selected catalog.SelectedBinding, candidates []catalog.ModelBinding) error {
	provider := selected.Provider

	// 1. 从该供应商的多 key 池中选择上游 key:优先该用户之前用过的 key(保证缓存命中率),
	//    其余按优先级/ID 排序。
	key, err := p.deps.Providers.SelectKey(provider.ID, rc.user.ID)
	if err != nil {
		return p.failoverOrError(r, w, rc, irReq, downstream, selected, candidates,
			ir.NewError("upstream", "provider_unavailable", "no available upstream api key for provider", 502))
	}
	rc.providerKeyID = key.ID
	rc.providerKey = key.APIKey
	rc.providerKeyLabel = key.Label

	irReq.Model = selected.UpstreamModel
	upstreamPayload, err := encodeUpstreamRequest(provider.Protocol, irReq)
	if err != nil {
		return ir.NewError("internal", "encode_failed", "failed to encode upstream request", 500)
	}

	// 2. 用该 key 发起请求;按需求处理失败:
	//    - 404/429/402/401/403 或"余额不足/欠费"等明确信号 -> 立即降级,不重试;
	//    - 其他瞬时失败(网络错误/5xx) -> 重试 keyRetryLimit(3) 次,仍失败则降级。
	var (
		upResp   *http.Response
		lastErr  *ir.Error
		degraded bool
	)
	attempts := 0
	for {
		attempts++
		upReq, err := p.buildUpstreamRequest(r, &provider, key.APIKey, upstreamPayload)
		if err != nil {
			lastErr = ir.NewError("internal", "upstream_build_failed", "failed to build upstream request", 500)
			if attempts <= keyRetryLimit {
				time.Sleep(keyRetryBackoff(attempts))
				continue
			}
			break
		}
		upResp, err = p.deps.Client.Do(upReq)
		if err != nil {
			lastErr = ir.NewError("upstream", "upstream_unreachable", "upstream request failed: "+err.Error(), 502)
			if attempts <= keyRetryLimit {
				time.Sleep(keyRetryBackoff(attempts))
				continue
			}
			break
		}
		if upResp.StatusCode >= 400 {
			body, _ := io.ReadAll(upResp.Body)
			upResp.Body.Close()
			msg := upstreamErrorMessage(provider.Protocol, body)
			lastErr = upstreamErrorFromStatus(upResp.StatusCode, msg)
			if isKeyUnavailable(upResp.StatusCode, msg) {
				p.degradeProviderKey(key, fmt.Sprintf("http %d: %s", upResp.StatusCode, msg))
				degraded = true
				break
			}
			if attempts <= keyRetryLimit {
				time.Sleep(keyRetryBackoff(attempts))
				continue
			}
			break
		}
		break // 成功
	}

	// 3. 记录用户-上游 key 亲和(缓存命中)与 key 最后使用时间。
	_ = p.deps.Providers.MarkUsed(key.ID, rc.user.ID)

	// 重试 3 次后仍失败(瞬时错误):按需求把该 key 降级到优先级末尾并进入冷却。
	if !degraded && lastErr != nil && attempts > keyRetryLimit {
		p.degradeProviderKey(key, "retried 3 times, still failing: "+lastErr.Message)
	}

	if lastErr != nil {
		return p.failoverOrError(r, w, rc, irReq, downstream, selected, candidates, lastErr)
	}

	defer upResp.Body.Close()
	rc.firstByteAt = time.Now()
	rc.stream = irReq.Stream
	if shouldFailoverStatus(upResp.StatusCode) && rc.model.RoutingStrategy == catalog.RoutingStrategyAuto && rc.model.AutoMode == catalog.AutoModeFailover {
		body, _ := io.ReadAll(upResp.Body)
		upResp.Body.Close()
		return p.failoverOrError(r, w, rc, irReq, downstream, selected, candidates,
			upstreamErrorFromStatus(upResp.StatusCode, upstreamErrorMessage(provider.Protocol, body)))
	}
	rc.provider = &provider
	rc.selected = &selected
	if irReq.Stream {
		p.streamResponse(w, rc, upResp, downstream)
		return nil
	}
	p.forwardNonStreaming(w, rc, upResp, downstream)
	return nil
}

// degradeProviderKey 把上游 key 降级到优先级末尾并进入 1 小时冷却(失败不阻断请求)。
func (p *Proxy) degradeProviderKey(key catalog.ProviderAPIKey, reason string) {
	if err := p.deps.Providers.DegradeKey(key.ID, reason); err != nil {
		// 记录失败即可,下次请求仍会尝试该 key。
	}
}

// isKeyUnavailable 判断某上游响应是否表明"该 API key 不可用":
// 404/429/402/401/403,或错误消息命中"余额不足/欠费"等关键字。
func isKeyUnavailable(status int, msg string) bool {
	switch status {
	case 401, 402, 403, 404, 429:
		return true
	}
	return keyUnavailableMessage(msg)
}

func keyUnavailableMessage(msg string) bool {
	m := strings.ToLower(msg)
	for _, k := range keyUnavailableKeywords {
		if strings.Contains(m, k) {
			return true
		}
	}
	return false
}

// keyUnavailableKeywords 余额不足/欠费等上游常见提示(OpenAI/Anthropic/DeepSeek 等)。
var keyUnavailableKeywords = []string{
	"insufficient balance", "insufficient_quota", "billing", "arrears",
	"credit balance", "balance is too low", "account balance", "payment required",
	"payment_required", "no enough balance", "overdue", "quota exhausted",
	"欠费", "余额不足",
}

func (p *Proxy) failoverOrError(r *http.Request, w http.ResponseWriter, rc *requestContext, irReq *ir.Request, downstream string, selected catalog.SelectedBinding, candidates []catalog.ModelBinding, fallback *ir.Error) error {
	next, ok, err := p.getRouter().Next(selected.Binding, candidates)
	if err != nil || !ok {
		return fallback
	}
	return p.forwardSelected(w, r, rc, irReq, downstream, next, candidates)
}

func shouldFailoverStatus(status int) bool {
	return status == 429 || status >= 500
}

func decodeDownstreamRequest(downstream string, body []byte) (*ir.Request, error) {
	switch downstream {
	case "chat":
		return ir.DecodeChatRequest(body)
	case "responses":
		return ir.DecodeResponsesRequest(body)
	case "anthropic":
		return ir.DecodeAnthropicRequest(body)
	}
	return nil, fmt.Errorf("unknown downstream protocol %q", downstream)
}

func encodeUpstreamRequest(protocol string, req *ir.Request) ([]byte, error) {
	switch protocol {
	case "openai_chat":
		return ir.EncodeChatRequest(req)
	case "openai_responses":
		return ir.EncodeResponsesRequest(req)
	case "anthropic":
		return ir.EncodeAnthropicRequest(req)
	}
	return nil, fmt.Errorf("unknown upstream protocol %q", protocol)
}

// forwardNonStreaming 非流式:上游响应 -> IR -> 下游格式。
func (p *Proxy) forwardNonStreaming(w http.ResponseWriter, rc *requestContext, upResp *http.Response, downstream string) {
	body, err := io.ReadAll(upResp.Body)
	if err != nil {
		p.writeError(w, rc, ir.NewError("upstream", "upstream_read_failed", "failed to read upstream response", 502))
		return
	}
	if upResp.StatusCode >= 400 {
		// 提取上游错误信息(尽力):尝试解码上游协议错误体的 message
		msg := upstreamErrorMessage(rc.provider.Protocol, body)
		e := upstreamErrorFromStatus(upResp.StatusCode, msg)
		p.writeError(w, rc, e)
		return
	}
	// 上游 -> IR
	irResp, err := decodeUpstreamResponse(rc.provider.Protocol, body)
	if err != nil {
		p.writeError(w, rc, ir.NewError("upstream", "upstream_parse_failed", "failed to parse upstream response", 502))
		return
	}
	// 统计
	rc.inputTokens = irResp.Usage.InputTokens
	rc.outputTokens = irResp.Usage.OutputTokens
	rc.cacheRead = irResp.Usage.CacheReadTokens
	rc.cacheCreation = irResp.Usage.CacheCreationTokens

	// IR -> 下游
	out, err := encodeDownstreamResponse(downstream, irResp)
	if err != nil {
		p.writeError(w, rc, ir.NewError("internal", "encode_failed", "failed to encode downstream response", 500))
		return
	}
	rc.statusCode = 200
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(out)
	p.recordStats(rc)
}

// upstreamErrorFromStatus 把上游 HTTP 状态码映射为 ir.Error。
// 状态码透传给客户端(429->429, 401->401 等),保留上游语义。
func upstreamErrorFromStatus(status int, message string) *ir.Error {
	typ := "upstream"
	code := "upstream_error"
	if message == "" {
		message = fmt.Sprintf("upstream returned %d", status)
	}
	switch {
	case status == 429:
		typ, code = "rate_limit", "upstream_rate_limited"
	case status == 401:
		typ, code = "authentication", "upstream_unauthorized"
	case status == 403:
		typ, code = "permission", "upstream_forbidden"
	case status == 404:
		typ, code = "not_found", "upstream_not_found"
	case status >= 400 && status < 500:
		typ, code = "invalid_request", "upstream_bad_request"
	}
	// 透传上游状态码(客户端需据此退避/重试),4xx/5xx 均保留原值
	return ir.NewError(typ, code, message, status)
}

// upstreamErrorMessage 尽力从上游错误体提取 message。
// OpenAI: {"error":{"message":...}};Anthropic: {"error":{"message":...}}。
// 提取失败返回空串(调用方用通用消息)。
func upstreamErrorMessage(protocol string, body []byte) string {
	var v struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return truncate(body, 200)
	}
	if v.Error.Message != "" {
		return v.Error.Message
	}
	return truncate(body, 200)
}

func decodeUpstreamResponse(protocol string, body []byte) (*ir.Response, error) {
	switch protocol {
	case "openai_chat":
		return ir.DecodeChatResponse(body)
	case "openai_responses":
		return ir.DecodeResponsesResponse(body)
	case "anthropic":
		return ir.DecodeAnthropicResponse(body)
	}
	return nil, fmt.Errorf("unknown upstream protocol %q", protocol)
}

func encodeDownstreamResponse(downstream string, resp *ir.Response) ([]byte, error) {
	switch downstream {
	case "chat":
		return ir.EncodeChatResponse(resp)
	case "responses":
		return ir.EncodeResponsesResponse(resp)
	case "anthropic":
		return ir.EncodeAnthropicResponse(resp)
	}
	return nil, fmt.Errorf("unknown downstream protocol %q", downstream)
}

// writeError 按下游协议编码错误 + 记日志。
func (p *Proxy) writeError(w http.ResponseWriter, rc *requestContext, e *ir.Error) {
	rc.statusCode = e.StatusCode
	rc.errorType = e.Type
	rc.errorMessage = e.Message
	var body []byte
	if rc.downstream == "anthropic" {
		body = ir.AnthropicErrorBody(e)
	} else {
		body = ir.OpenAIErrorBody(e)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.StatusCode)
	w.Write(body)
	p.recordStats(rc)
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n])
	}
	return string(b)
}

// asIRError 把普通 error 规整为 *ir.Error(authenticate/resolveModel 返回的是 error 接口)。
func asIRError(err error) *ir.Error {
	if e, ok := err.(*ir.Error); ok {
		return e
	}
	return ir.NewError("internal", "internal_error", err.Error(), 500)
}
