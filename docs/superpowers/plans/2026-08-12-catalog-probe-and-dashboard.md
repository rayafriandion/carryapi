# 上游模型获取 / 供应商测试 / 仪表板 base_url 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 carryAPI 网关添加「从上游供应商自动获取模型列表并选择式导入为草稿」「测试供应商可用性/延迟」以及「仪表板首页显示 API base_url」三个功能。

**Architecture:** 在 `internal/catalog` 新增一个 `Prober` 探针客户端，封装上游 `GET /models` 请求（拉取模型列表 + 测连通/延迟）。扩展 `catalog.Handler` 新增 4 个 admin API（fetch 模型、批量导入、测供应商），并在 `internal/server` 新增网关信息接口。前端在 ModelsView 加导入/测试交互，在 Dashboard 加 base_url 展示。

**Tech Stack:** Go 1.22+, chi router, database/sql (SQLite), Vue 3 + naive-ui + echarts。

## Global Constraints

- Go 1.22+，单二进制内嵌前端。
- 上游探针统一用 `GET /models`（非最小 chat 请求）。
- 导入的模型 `enabled=0`（禁用态草稿），名称取上游模型名；同名跳过不覆盖。
- 所有新 API 均需 admin 角色（复用 `RequireRole("admin")` 分组）。
- 新功能同步更新 `MANUAL.md` 与 `README.md`（见 keep-manual-in-sync-on-feature-changes）。
- Mimosa 会拦截 commit：本次为真实代码提交，若被误报需跑深扫核实 + 用户授权绕过（见 mimosa-false-positives-on-parameterized-sql）。

---

### Task 1: 探针客户端 Prober

**Files:**
- Create: `internal/catalog/probe.go`
- Test: `internal/catalog/probe_test.go`

**Interfaces:**
- Produces:
  - `func NewProber(client *http.Client) *Prober`
  - `func (p *Prober) FetchModels(ctx context.Context, provider Provider) ([]string, error)`
  - `func (p *Prober) Ping(ctx context.Context, provider Provider) (time.Duration, error)`
  - `func (p *Prober) SetClient(client *http.Client)`（测试注入用）

- [ ] **Step 1: Write the failing test**

`internal/catalog/probe_test.go`:

```go
package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testProvider(baseURL, protocol string) Provider {
	return Provider{BaseURL: baseURL, APIKey: "sk-test", Protocol: protocol}
}

func TestFetchModelsOpenAI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		w.Write([]byte(`{"data":[{"id":"gpt-4o"},{"id":"gpt-4o-mini"}]}`))
	}))
	defer srv.Close()

	p := NewProber(srv.Client())
	models, err := p.FetchModels(context.Background(), testProvider(srv.URL, "openai_chat"))
	if err != nil {
		t.Fatalf("FetchModels: %v", err)
	}
	if len(models) != 2 || models[0] != "gpt-4o" || models[1] != "gpt-4o-mini" {
		t.Errorf("models = %v", models)
	}
}

func TestFetchModelsAnthropic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "sk-test" {
			t.Errorf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		w.Write([]byte(`{"data":[{"id":"claude-3-5-sonnet"}]}`))
	}))
	defer srv.Close()

	p := NewProber(srv.Client())
	models, err := p.FetchModels(context.Background(), testProvider(srv.URL, "anthropic"))
	if err != nil {
		t.Fatalf("FetchModels: %v", err)
	}
	if len(models) != 1 || models[0] != "claude-3-5-sonnet" {
		t.Errorf("models = %v", models)
	}
}

func TestFetchModelsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer srv.Close()

	p := NewProber(srv.Client())
	_, err := p.FetchModels(context.Background(), testProvider(srv.URL, "openai_chat"))
	if err == nil {
		t.Fatal("expected error for 401")
	}
}

func TestPing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	p := NewProber(srv.Client())
	d, err := p.Ping(context.Background(), testProvider(srv.URL, "openai_chat"))
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if d < 0 {
		t.Errorf("latency negative: %v", d)
	}
}

func TestPingNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	p := NewProber(srv.Client())
	_, err := p.Ping(context.Background(), testProvider(srv.URL, "openai_chat"))
	if err == nil {
		t.Fatal("expected error for 500")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /d/Projects/carryAPI && go test ./internal/catalog/ -run TestFetchModels -v`
Expected: FAIL with "undefined: NewProber"

- [ ] **Step 3: Write minimal implementation**

`internal/catalog/probe.go`:

```go
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Prober 对上游供应商发起轻量 GET /models 请求,用于拉取模型列表与测连通/延迟。
type Prober struct {
	client *http.Client
}

func NewProber(client *http.Client) *Prober {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Prober{client: client}
}

func (p *Prober) SetClient(client *http.Client) { p.client = client }

// modelsPath 返回该供应商协议对应的模型列表路径。
func modelsPath(protocol string) string {
	if protocol == "anthropic" {
		return "/v1/models"
	}
	return "/models"
}

// authHeaders 按协议设置鉴权头。
func authHeaders(provider Provider) map[string]string {
	if provider.Protocol == "anthropic" {
		return map[string]string{
			"x-api-key":         provider.APIKey,
			"anthropic-version": "2023-06-01",
		}
	}
	return map[string]string{"Authorization": "Bearer " + provider.APIKey}
}

// do 发起 GET /models,返回响应体;非 2xx 返回错误。
func (p *Prober) do(ctx context.Context, provider Provider) ([]byte, error) {
	url := provider.BaseURL + modelsPath(provider.Protocol)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range authHeaders(provider) {
		req.Header.Set(k, v)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, truncateBody(body))
	}
	return body, nil
}

func truncateBody(b []byte) string {
	if len(b) > 200 {
		return string(b[:200])
	}
	return string(b)
}

// FetchModels 返回该供应商的模型名列表(尽力解析;解析失败返回空列表)。
func (p *Prober) FetchModels(ctx context.Context, provider Provider) ([]string, error) {
	body, err := p.do(ctx, provider)
	if err != nil {
		return nil, err
	}
	var v struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		// 解析失败不阻断,返回空列表(调用方走手动添加路径)
		return []string{}, nil
	}
	out := make([]string, 0, len(v.Data))
	for _, d := range v.Data {
		if d.ID != "" {
			out = append(out, d.ID)
		}
	}
	return out, nil
}

// Ping 返回请求耗时(连通成功时),非 2xx/超时/网络错误返回 error。
func (p *Prober) Ping(ctx context.Context, provider Provider) (time.Duration, error) {
	start := time.Now()
	if _, err := p.do(ctx, provider); err != nil {
		return 0, err
	}
	return time.Since(start), nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /d/Projects/carryAPI && go test ./internal/catalog/ -run 'TestFetchModels|TestPing' -v`
Expected: PASS (6 tests)

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/probe.go internal/catalog/probe_test.go
git commit -m "feat(catalog): add Prober for upstream model fetch and ping"
```

---

### Task 2: 扩展 Handler 新增 fetch/import/test API

**Files:**
- Modify: `internal/catalog/handler.go`（在 `Handler` 结构加 `prober` 字段 + `NewHandler` 默认构造 + 3 个方法）
- Modify: `internal/catalog/handler_test.go`（新增 3 个测试）
- Test: `internal/catalog/handler_test.go`

**Interfaces:**
- Consumes: `Prober`（Task 1）、`ProviderStore`、`ModelStore`
- Produces:
  - `Handler` 新增字段 `prober *Prober` 与 `SetProber(p *Prober)`
  - `func (h *Handler) FetchProviderModels(w, r)` → `{models:[{name,exists}]}`
  - `func (h *Handler) ImportModels(w, r)` → `{imported,skipped,skipped_names}`
  - `func (h *Handler) TestProvider(w, r)` → `{ok,latency_ms,error?}`

- [ ] **Step 1: 修改 NewHandler 与结构体（写实现骨架，先保证编译）**

在 `internal/catalog/handler.go`：

```go
type Handler struct {
	providers *ProviderStore
	models    *ModelStore
	prices    *PriceStore
	prober    *Prober
}

func NewHandler(providers *ProviderStore, models *ModelStore, prices *PriceStore) *Handler {
	return &Handler{providers: providers, models: models, prices: prices, prober: NewProber(nil)}
}

func (h *Handler) SetProber(p *Prober) { h.prober = p }
```

- [ ] **Step 2: 新增 FetchProviderModels**

在 `internal/catalog/handler.go` 的 `// ---- providers ----` 段后追加：

```go
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
			continue
		}
		imported++
	}
	jsonOut(w, 200, map[string]any{"imported": imported, "skipped": len(skipped), "skipped_names": skipped})
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
```

- [ ] **Step 3: 新增 ModelStore.CreateDraft**

在 `internal/catalog/model.go` 追加：

```go
// CreateDraft 创建禁用态草稿模型(enabled=0),名称取上游模型名。
func (s *ModelStore) CreateDraft(providerID int64, upstreamModel string) (Model, error) {
	res, err := s.db.Exec(
		`INSERT INTO custom_models(name, provider_id, upstream_model, enabled) VALUES(?, ?, ?, 0)`,
		upstreamModel, providerID, upstreamModel)
	if err != nil {
		return Model{}, fmt.Errorf("create draft model: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}
```

- [ ] **Step 4: 写 handler 测试**

在 `internal/catalog/handler_test.go` 追加：

```go
func newTestHandler(t *testing.T) (*Handler, *httptest.Server) {
	f := newCatalogFixture(t)
	h := NewHandler(f.providers, f.models, f.prices)
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"gpt-4o"},{"id":"gpt-4o-mini"}]}`))
	}))
	t.Cleanup(up.Close)
	h.SetProber(NewProber(up.Client()))
	// 建一个指向 up 的 provider
	f.providers.Create("Up", up.URL, "sk-test", "openai_chat")
	return h, up
}

func TestFetchProviderModels(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest("GET", "/api/providers/1/models/fetch", nil).WithContext(adminCtx())
	rec := httptest.NewRecorder()
	h.FetchProviderModels(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Models []map[string]any `json:"models"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Models) != 2 {
		t.Fatalf("models=%+v", resp.Models)
	}
}

func TestImportModels(t *testing.T) {
	h, _ := newTestHandler(t)
	body, _ := json.Marshal(map[string]any{
		"items": []map[string]any{
			{"provider_id": 1, "upstream_model": "gpt-4o"},
			{"provider_id": 1, "upstream_model": "gpt-4o"},
			{"provider_id": 1, "upstream_model": "gpt-4o-mini"},
		},
	})
	req := httptest.NewRequest("POST", "/api/models/import", bytes.NewReader(body)).WithContext(adminCtx())
	rec := httptest.NewRecorder()
	h.ImportModels(rec, req)
	var resp struct {
		Imported     int      `json:"imported"`
		Skipped      int      `json:"skipped"`
		SkippedNames []string `json:"skipped_names"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Imported != 2 || resp.Skipped != 1 {
		t.Fatalf("imported=%d skipped=%d", resp.Imported, resp.Skipped)
	}
	// 确认导入为禁用态
	m, err := h.models.GetByName("gpt-4o")
	if err != nil || m.Enabled {
		t.Fatalf("draft should be disabled: %+v err=%v", m, err)
	}
}

func TestTestProviderOK(t *testing.T) {
	h, _ := newTestHandler(t)
	req := httptest.NewRequest("POST", "/api/providers/1/test", nil).WithContext(adminCtx())
	rec := httptest.NewRecorder()
	h.TestProvider(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["ok"] != true {
		t.Fatalf("resp=%+v", resp)
	}
}
```

注意：handler_test.go 已 import `context`、`bytes`、`encoding/json`、`net/http/httptest`。需补 import `net/http`。

- [ ] **Step 5: 确保 import 完整并跑测试**

在 `handler_test.go` 顶部 import 块加 `"net/http"`。

Run: `cd /d/Projects/carryAPI && go test ./internal/catalog/ -run 'TestFetchProviderModels|TestImportModels|TestTestProvider' -v`
Expected: PASS

- [ ] **Step 6: 全量跑 catalog 测试确认无回归**

Run: `cd /d/Projects/carryAPI && go test ./internal/catalog/`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/catalog/handler.go internal/catalog/model.go internal/catalog/handler_test.go
git commit -m "feat(catalog): add fetch/import/test provider model APIs"
```

---

### Task 3: 网关信息接口 + 路由注册

**Files:**
- Create: `internal/server/gateway.go`
- Modify: `internal/server/router.go`
- Modify: `internal/server/server_test.go`（新增测试）
- Modify: `cmd/carryapi/main.go`（无，proxy 已存在；本任务只加路由）

**Interfaces:**
- Consumes: `server.Server`（持有 `cfg config.Config`、`deps.Store *settings.Store`）
- Produces:
  - `func (s *Server) handleGatewayInfo(w, r)` → `{base_url:"http://host:port/v1"}`

- [ ] **Step 1: 写实现 `internal/server/gateway.go`**

`server` 包没有自带 JSON helper，`JSON` 在 `internal/api` 包（server 已 import api 用于各 handler，无循环依赖）。直接复用：

```go
package server

import (
	"net/http"

	"carryapi/internal/api"
)

// handleGatewayInfo 返回网关对外接入地址。
// host:监听 0.0.0.0 时用请求 Host 头,否则用 127.0.0.1;port 由 config 提供(拼接进 host 的 host:port)。
func (s *Server) handleGatewayInfo(w http.ResponseWriter, r *http.Request) {
	host := "127.0.0.1"
	if listenHost(s.deps.Store) == "0.0.0.0" {
		host = r.Host
	}
	baseURL := "http://" + host + "/v1"
	api.JSON(w, 200, map[string]string{"base_url": baseURL})
}
```

说明：`listenHost(s.deps.Store)` 返回监听 host；`r.Host` 已含 `host:port`（浏览器访问地址），故 `baseURL = "http://" + r.Host + "/v1"`。`127.0.0.1` 分支下 host 不含端口，故补上 `:port`：

```go
	if listenHost(s.deps.Store) == "0.0.0.0" {
		host = r.Host // 已含 host:port
	} else {
		host = "127.0.0.1:" + strconv.Itoa(s.cfg.Port)
	}
```

最终实现（含 strconv import）：

```go
package server

import (
	"net/http"
	"strconv"

	"carryapi/internal/api"
)

func (s *Server) handleGatewayInfo(w http.ResponseWriter, r *http.Request) {
	host := "127.0.0.1:" + strconv.Itoa(s.cfg.Port)
	if listenHost(s.deps.Store) == "0.0.0.0" {
		host = r.Host // 已含 host:port
	}
	api.JSON(w, 200, map[string]string{"base_url": "http://" + host + "/v1"})
}
```

- [ ] **Step 2: 注册路由**

`internal/server/router.go` 中 admin-only 分组（RequireRole("admin")）内、`deps.Catalog` 分支之后追加（该分组已有 Sessions+Users 守卫）：

```go
r.Get("/api/gateway/info", s.handleGatewayInfo)
```

- [ ] **Step 3: 写测试（直接调 handler 方法，绕开会话/CSRF）**

在 `internal/server/server_test.go` 追加。`handleGatewayInfo` 自身不检查角色（角色由路由中间件负责），故可直接作为方法单测，仿 catalog 的 adminCtx 模式无需会话：

```go
func TestGatewayInfoLoopback(t *testing.T) {
	s := newServer(t) // cfg.Port=0,Store 已设
	s.deps.Store.Set("listen_host", "127.0.0.1")
	req := httptest.NewRequest("GET", "/api/gateway/info", nil)
	rec := httptest.NewRecorder()
	s.handleGatewayInfo(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
	if rec.Body.String() != `{"base_url":"http://127.0.0.1:0/v1"}`+"\n" {
		t.Errorf("body=%q", rec.Body.String())
	}
}

func TestGatewayInfoBroadcast(t *testing.T) {
	s := newServer(t)
	s.deps.Store.Set("listen_host", "0.0.0.0")
	req := httptest.NewRequest("GET", "/api/gateway/info", nil)
	req.Host = "example.com:8067"
	rec := httptest.NewRecorder()
	s.handleGatewayInfo(rec, req)
	if rec.Body.String() != `{"base_url":"http://example.com:8067/v1"}`+"\n" {
		t.Errorf("body=%q", rec.Body.String())
	}
}
```

注意：`api.JSON` 用 `json.NewEncoder` 输出，末尾带 `\n`（与 TestHealthEndpoint 断言一致）。

- [ ] **Step 4: 跑测试**

Run: `cd /d/Projects/carryAPI && go test ./internal/server/ -run TestGatewayInfo -v`
Expected: PASS (2 tests)

- [ ] **Step 5: 全量回归**

Run: `cd /d/Projects/carryAPI && go build ./... && go test ./...`
Expected: 全 PASS

- [ ] **Step 6: Commit**

```bash
git add internal/server/gateway.go internal/server/router.go internal/server/server_test.go
git commit -m "feat(server): expose gateway base_url endpoint"
```

---

### Task 4: 前端 — ModelsView 导入与测试交互

**Files:**
- Modify: `web/src/views/ModelsView.vue`

**Interfaces:**
- Consumes: 后台 API `GET /api/providers/{id}/models/fetch`、`POST /api/models/import`、`POST /api/providers/{id}/test`

- [ ] **Step 1: 供应商表格加「测试」列**

在 `providerColumns` 的 actions render 中,在「编辑」前加「测试」按钮：

```ts
h(NButton, { size: 'small', loading: row._testing, onClick: () => onProviderTest(row) }, { default: () => '测试' }),
```

- [ ] **Step 2: 加 onProviderTest 逻辑**

```ts
async function onProviderTest(row: any) {
  row._testing = true
  try {
    const res = await http.post(`/api/providers/${row.id}/test`)
    const d = res.data || {}
    row._testResult = d.ok
      ? `可用 ${d.latency_ms}ms`
      : `不可用: ${d.error || ''}`
  } catch (e) {
    row._testResult = '测试失败'
  } finally {
    row._testing = false
  }
}
```

在 `providerColumns` 的「状态」列后加一列显示结果：

```ts
{ title: '测试', key: 'test', render(row: any) {
  return h('span', { class: row._testResult?.startsWith('可用') ? 'ok' : 'bad' }, row._testResult || '-')
} },
```

- [ ] **Step 3: 模型标签页加「从供应商导入」按钮 + 弹窗**

在「模型」toolbar 加按钮 + 新 modal：

```html
<n-button @click="openImportModal">从供应商导入</n-button>
```

```html
<!-- 导入弹窗 -->
<n-modal v-model:show="importShow" preset="card" title="从供应商导入模型" style="width: 560px">
  <n-form>
    <n-form-item label="供应商">
      <n-select v-model:value="importProviderId" :options="providerOptions" placeholder="选择供应商" @update:value="onFetchModels" />
    </n-form-item>
    <div v-if="fetchedModels.length">
      <n-checkbox-group v-model:value="importSelected">
        <n-space vertical>
          <n-checkbox v-for="m in fetchedModels" :key="m.name" :value="m.name" :disabled="m.exists">
            {{ m.name }} <n-tag v-if="m.exists" size="small" type="warning">已存在</n-tag>
          </n-checkbox>
        </n-space>
      </n-checkbox-group>
    </div>
    <n-empty v-else-if="importProviderId" description="点击上方供应商后获取模型列表" />
  </n-form>
  <template #footer>
    <n-button @click="importShow = false">取消</n-button>
    <n-button type="primary" :loading="importing" :disabled="!importSelected.length" @click="onImport">导入 ({{ importSelected.length }})</n-button>
  </template>
</n-modal>
```

- [ ] **Step 4: 加导入逻辑**

```ts
const importShow = ref(false)
const importProviderId = ref<number | null>(null)
const fetchedModels = ref<any[]>([])
const importSelected = ref<string[]>([])
const importing = ref(false)

function openImportModal() {
  importProviderId.value = null
  fetchedModels.value = []
  importSelected.value = []
  importShow.value = true
}
async function onFetchModels() {
  if (importProviderId.value == null) return
  fetchedModels.value = []
  importSelected.value = []
  try {
    const res = await http.get(`/api/providers/${importProviderId.value}/models/fetch`)
    fetchedModels.value = res.data?.models || []
  } catch (e) {
    message.error(errorMessage(e))
  }
}
async function onImport() {
  if (!importProviderId.value) return
  importing.value = true
  try {
    const items = importSelected.value.map((name) => ({ provider_id: importProviderId.value, upstream_model: name }))
    const res = await http.post('/api/models/import', { items })
    const d = res.data || {}
    message.success(`导入 ${d.imported} 个,跳过 ${d.skipped} 个`)
    importShow.value = false
    loadModels()
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    importing.value = false
  }
}
```

需在 naive-ui import 中补 `NCheckboxGroup, NCheckbox, NSpace, NTag`。

- [ ] **Step 5: 前端构建验证**

Run: `cd /d/Projects/carryAPI/web && npm run build`
Expected: 构建成功,无 TS 错误

- [ ] **Step 6: Commit**

```bash
git add web/src/views/ModelsView.vue
git commit -m "feat(web): model import from provider and provider test buttons"
```

---

### Task 5: 前端 — Dashboard 显示 base_url

**Files:**
- Modify: `web/src/views/DashboardView.vue`

**Interfaces:**
- Consumes: 后台 API `GET /api/gateway/info`

- [ ] **Step 1: 顶部加 base_url 卡片**

在 `<div class="dashboard">` 最上方、grid 之前插入：

```html
<n-card class="section base-url-card">
  <div class="base-url">
    <span class="label">API Base URL</span>
    <n-copy :text="baseUrl" v-if="baseUrl">
      <n-text code>{{ baseUrl }}</n-text>
    </n-copy>
    <n-text v-else type="secondary">—</n-text>
  </div>
</n-card>
```

- [ ] **Step 2: 拉取并渲染**

在 `<script setup>` 加：

```ts
import { NCopy, NText } from 'naive-ui'
const baseUrl = ref('')

// 在 onMounted 的 Promise.all 中追加
const info = await http.get('/api/gateway/info')
baseUrl.value = info.data?.base_url || ''
```

（将 `baseUrl` 的获取加入现有 `Promise.all`,或单独请求均可。）

- [ ] **Step 3: 样式**

```css
.base-url-card { margin-bottom: 16px; }
.base-url { display: flex; align-items: center; gap: 12px; }
.base-url .label { font-weight: 600; }
```

- [ ] **Step 4: 前端构建验证**

Run: `cd /d/Projects/carryAPI/web && npm run build`
Expected: 构建成功,无 TS 错误

- [ ] **Step 5: Commit**

```bash
git add web/src/views/DashboardView.vue
git commit -m "feat(web): show API base url on dashboard"
```

---

### Task 6: 文档同步 MANUAL.md / README.md

**Files:**
- Modify: `MANUAL.md`
- Modify: `README.md`

- [ ] **Step 1: MANUAL.md 增补**

在管理后台 / 供应商 / 模型相关章节补充：
- 「从供应商导入模型」：在「模型」页选择供应商 → 获取模型列表 → 勾选导入为草稿（禁用态）→ 手动配价格并启用。
- 「供应商测试」：供应商列表每行「测试」按钮显示可用性/延迟。
- 「仪表板 base_url」：仪表板首页显示网关 API 接入地址 `http://host:port/v1`。

- [ ] **Step 2: README.md 增补**

在功能列表或相关章节补充以上三个功能的简述。

- [ ] **Step 3: Commit**

```bash
git add MANUAL.md README.md
git commit -m "docs: document model import, provider test, dashboard base url"
```

---

## Self-Review

**Spec coverage:** 探针(Task1)、fetch/import/test API(Task2)、网关信息+路由(Task3)、前端导入/测试(Task4)、前端 base_url(Task5)、文档(Task6)。覆盖设计全部要求。

**Placeholder scan:** 无 TBD/TODO。所有代码步骤含完整实现。

**Type consistency:** `NewProber`、`FetchModels`、`Ping`、`CreateDraft`、`SetProber`、`handleGatewayInfo` 在各 task 间签名一致。`handler_test.go` 需补 `net/http` import（已在 Task2 Step5 标注）。
