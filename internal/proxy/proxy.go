package proxy

import (
	"database/sql"
	"net/http"
	"time"

	"carryapi/internal/apikey"
	"carryapi/internal/catalog"
	"carryapi/internal/ir"
	"carryapi/internal/user"
)

type Deps struct {
	DB        *sql.DB
	Keys      *apikey.Store
	Users     *user.Store
	Models    *catalog.ModelStore
	Providers *catalog.ProviderStore
	Prices    *catalog.PriceStore
	Client    *http.Client
}

type Proxy struct {
	deps Deps
}

func NewProxy(deps Deps) *Proxy {
	if deps.Client == nil {
		deps.Client = &http.Client{}
	}
	return &Proxy{deps: deps}
}

// requestContext 承载一次代理请求的解析结果,贯穿转发与统计。
type requestContext struct {
	user       *user.User
	apiKeyID   int64
	downstream string // "chat" | "responses" | "anthropic"
	requestID  string
	stream     bool      // 流式请求(记录到日志)
	start      time.Time // 请求开始时间(算 duration_ms)
	model      *catalog.Model
	provider   *catalog.Provider
	price      *catalog.Price
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
	// 注意:/v1/models 由 Task 7 的 handleModels 处理,在此任务中不注册,
	// 未匹配路径直接落到默认 404。
	default:
		body := ir.OpenAIErrorBody(ir.NewError("not_found", "invalid_request_url", "invalid request url", 404))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		w.Write(body)
	}
}

// handleProxy 是占位实现:真正的上游转发在 Task 5。ServeHTTP 需要按路径分派
// 到它才能编译,目前统一返回 501 Not Implemented。
func (p *Proxy) handleProxy(w http.ResponseWriter, r *http.Request, downstream string) {
	body := ir.OpenAIErrorBody(ir.NewError("invalid_request", "not_implemented", "proxy forwarding not implemented yet", 501))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(501)
	w.Write(body)
}
