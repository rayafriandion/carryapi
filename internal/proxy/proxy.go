package proxy

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"carryapi/internal/apikey"
	"carryapi/internal/catalog"
	"carryapi/internal/ir"
	"carryapi/internal/user"
)

type Deps struct {
	DB          *sql.DB
	Keys        *apikey.Store
	Users       *user.Store
	Models      *catalog.ModelStore
	Bindings    *catalog.ModelBindingStore
	Providers   *catalog.ProviderStore
	Prices      *catalog.PriceStore
	HealthCache catalog.HealthCacheReader
	Client      *http.Client
}

type Proxy struct {
	deps   Deps
	router *catalog.Router
}

func NewProxy(deps Deps) *Proxy {
	if deps.Client == nil {
		deps.Client = &http.Client{}
	}
	if deps.Bindings == nil {
		deps.Bindings = catalog.NewModelBindingStore(deps.DB)
	}
	return &Proxy{deps: deps, router: nil}
}

func (p *Proxy) getRouter() *catalog.Router {
	if p.router == nil {
		p.router = catalog.NewRouter(p.deps.Providers, p.deps.HealthCache)
	}
	return p.router
}

// writeJSON 写 JSON 响应(catalog 包的 jsonOut 是 handler 私有,不可见,故在 proxy 包内定义)。
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// requestContext 承载一次代理请求的解析结果,贯穿转发与统计。
type requestContext struct {
	user           *user.User
	apiKeyID       int64
	downstream     string // "chat" | "responses" | "anthropic"
	requestID      string
	stream         bool      // 流式请求(记录到日志)
	start          time.Time // 请求开始时间(算 duration_ms)
	firstByteAt    time.Time // 上游首字节到达时间(算 ttft_ms)
	requestedModel string    // 客户端请求中的模型名(resolveModel 失败时仍用于统计)
	model          *catalog.Model
	provider       *catalog.Provider
	selected       *catalog.SelectedBinding
	candidates     []catalog.ModelBinding
	price          *catalog.Price
	// 上游 key 池选择结果(用于 request_logs 与失败降级)
	providerKeyID   int64
	providerKey     string
	providerKeyLabel string
	// 统计
	inputTokens   int
	outputTokens  int
	cacheRead     int
	cacheCreation int
	statusCode    int
	errorType     string
	errorMessage  string
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/v1/chat/completions", "/v1/completions":
		p.handleProxy(w, r, "chat")
	case "/v1/responses":
		p.handleProxy(w, r, "responses")
	case "/v1/messages":
		p.handleProxy(w, r, "anthropic")
	case "/v1/models":
		p.handleModels(w, r)
	default:
		body := ir.OpenAIErrorBody(ir.NewError("not_found", "invalid_request_url", "invalid request url", 404))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		w.Write(body)
	}
}
