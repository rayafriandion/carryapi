# carryAPI 子项目 4:上游代理 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 IR 枢纽接到真实 HTTP 代理端点。实现 catalog store(上游供应商/自定义模型/定价)、admin CRUD、四个代理端点(`/v1/chat/completions`、`/v1/completions`、`/v1/responses`、`/v1/messages`)、`/v1/models` 列表端点、API Key 鉴权、模型解析、配额预检、非流式/流式上游转发(IR 转换)、用量统计 + 费用计算 + request_logs 写入 + 配额累加。完成后,客户端可以用自定义模型名调用,代理自动路由到配置的上游并转换协议。

**Architecture:** 新包 `internal/catalog`(providers/custom_models/model_prices 三个 store + CRUD handler,admin 配置入口)和 `internal/proxy`(代理核心)。proxy 持有:apikey.Store(鉴权)、user.Store(用户状态)、catalog 三个 store(模型解析/取价)、settings.Store(广播等)、ir 转换器、sql.DB(写日志)。请求流:鉴权 → 协议识别(按路径) → 解码下游请求为 IR → 模型解析(自定义名 → provider+上游名) → 配额预检 → IR 编码为上游格式 → HTTP 转发 → 上游响应解码为 IR → 编码为下游格式 → 返回。统计在响应完成后同步写(request_logs + quotas 累加,不阻塞响应流)。`/v1/models` 从 catalog 列出启用模型。

**Tech Stack:** Go 标准库(`encoding/json`、`net/http`、`io`、`bytes`、`context`)+ `carryapi/internal/ir`(子项目3)+ `carryapi/internal/apikey`/`user`/`settings`(子项目2)。`github.com/google/uuid` 已装(生成 request-id)。无 CGO。

## Global Constraints

- Go 1.22+;无 CGO。复用 `internal/ir` 的全部转换器——**禁止在 proxy 里写新的协议解析逻辑**(除 SSE 行流式管道)。
- 上游供应商的 api_key 存 `upstream_providers.api_key` 列,必须用 `internal/crypto.Cipher` 加密后入库,读取时解密。**明文不落库**。
- 鉴权:代理端点接受 `Authorization: Bearer <key>` 或 `x-api-key: <key>`(Anthropic 风格),统一用 `apikey.Store.Authenticate`;用户 status=active 才放行。
- 配额:请求前用 `user.Store.GetQuotas` 预检(限 token/cost 且周期内已用超限 → 429);请求后用 `user.Store.IncrementUsage` 累加。
- 响应格式按请求路径决定(Chat 请求返 Chat 格式,Responses 返 Responses,Anthropic 返 Anthropic),与上游协议无关。
- 错误:按下游协议用 `ir.OpenAIErrorBody` / `ir.AnthropicErrorBody` 编码;HTTP 状态码用 `ir.Error.StatusCode`。
- 每个请求生成 `X-Request-Id`(uuid),写入响应头 + request_logs.request_id。
- 上游超时:`http.Client` 无整体超时(流式),但每行读有 context 取消(客户端断开传播取消)。
- TDD:每个任务先写失败测试,再实现,再验证通过,再提交。
- Git 身份:`rayafriandion <amizhisa@outlook.com>`(本仓库已配置)。

---

## File Structure

```
carryAPI/
└── internal/
    ├── catalog/                      # 上游/模型/定价 配置 store + CRUD handler
    │   ├── provider.go               # Provider struct + ProviderStore(加密 api_key)
    │   ├── provider_test.go
    │   ├── model.go                  # Model struct + ModelStore(含 GetByName 解析)
    │   ├── model_test.go
    │   ├── price.go                  # Price struct + PriceStore(价格历史)
    │   ├── price_test.go
    │   ├── handler.go                # admin CRUD handlers(providers/models/prices)
    │   └── handler_test.go
    ├── proxy/                        # 代理核心
    │   ├── proxy.go                  # Proxy 结构体 + NewProxy(deps) + 路由分派(ServeHTTP)
    │   ├── proxy_test.go             # 单元测试(mock 上游 httptest)
    │   ├── auth.go                   # API Key 鉴权(Bearer / x-api-key)
    │   ├── auth_test.go
    │   ├── model.go                  # 模型解析 + 配额预检
    │   ├── model_test.go
    │   ├── forward.go                # 非流式上游转发
    │   ├── stream.go                 # 流式转发(SSE 管道)
    │   ├── stats.go                  # 统计:费用计算 + request_logs 写入 + quotas 累加
    │   └── stats_test.go
    ├── api/                          # (修改)settings_handler.go 白名单加 oauth 等
    └── server/
        ├── router.go                 # (修改)挂载 /v1/* 代理路由 + catalog admin 路由
        └── server.go                 # (修改)Deps 加 Catalog/Proxy
```

每个文件单一职责:`catalog` 管配置数据;`proxy` 管转发链路(auth/model/forward/stream/stats 各一层)。proxy 不直接碰 DB 表,只通过 store。

---

### Task 1: catalog — ProviderStore

**Files:**
- Create: `internal/catalog/provider.go`
- Test: `internal/catalog/provider_test.go`

**Interfaces:**
- Consumes: `*sql.DB`、`*crypto.Cipher`(加密上游 api_key)
- Produces:

```go
type Provider struct {
    ID        int64
    Name      string
    BaseURL   string
    APIKey    string // 解密后
    Protocol  string // "openai_chat" | "openai_responses" | "anthropic"
    Status    string // "active" | "disabled"
    CreatedAt time.Time
}

type ProviderStore struct { db *sql.DB; cipher *crypto.Cipher }

func NewProviderStore(db *sql.DB, cipher *crypto.Cipher) *ProviderStore

func (s *ProviderStore) Create(name, baseURL, apiKey, protocol string) (Provider, error)
    // 加密 apiKey 存库;protocol 校验(三个合法值);status='active'
func (s *ProviderStore) Get(id int64) (Provider, error)        // 解密 apiKey
func (s *ProviderStore) List() ([]Provider, error)             // 解密
func (s *ProviderStore) Update(id int64, name, baseURL, apiKey, protocol, status string) error
    // apiKey 为空串时保留原值;protocol/status 校验
func (s *ProviderStore) Delete(id int64) error                  // 若被 custom_models 引用,报错
```

- [ ] **Step 1: 写失败测试**

`internal/catalog/provider_test.go`:

```go
package catalog

import (
	"bytes"
	"testing"

	"carryapi/internal/crypto"
	"carryapi/internal/db"
)

func newProviderStore(t *testing.T) *ProviderStore {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	c, _ := crypto.New(bytes.Repeat([]byte{1}, 32))
	return NewProviderStore(d, c)
}

func TestCreateAndGet(t *testing.T) {
	s := newProviderStore(t)
	p, err := s.Create("OpenAI", "https://api.openai.com/v1", "sk-secret", "openai_chat")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == 0 || p.Status != "active" || p.Protocol != "openai_chat" {
		t.Errorf("unexpected provider: %+v", p)
	}
	// Get 返回解密后的 key
	got, err := s.Get(p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.APIKey != "sk-secret" {
		t.Errorf("api key round-trip = %q, want sk-secret", got.APIKey)
	}
}

func TestCreateInvalidProtocol(t *testing.T) {
	s := newProviderStore(t)
	if _, err := s.Create("X", "http://x", "k", "bogus"); err == nil {
		t.Error("expected error for invalid protocol")
	}
}

func TestUpdatePreservesKeyWhenEmpty(t *testing.T) {
	s := newProviderStore(t)
	p, _ := s.Create("OpenAI", "https://api.openai.com/v1", "sk-secret", "openai_chat")
	// 只改 name,apiKey 传空 -> 保留原 key
	if err := s.Update(p.ID, "OpenAI2", "https://api.openai.com/v2", "", "openai_chat", "active"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := s.Get(p.ID)
	if got.Name != "OpenAI2" || got.APIKey != "sk-secret" {
		t.Errorf("after update: name=%q key=%q", got.Name, got.APIKey)
	}
}

func TestDelete(t *testing.T) {
	s := newProviderStore(t)
	p, _ := s.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	if err := s.Delete(p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(p.ID); err == nil {
		t.Error("expected error after delete")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd /d/Projects/carryAPI
go test ./internal/catalog/ -v
```

预期:编译失败。

- [ ] **Step 3: 实现 provider.go**

```go
package catalog

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"carryapi/internal/crypto"
)

type Provider struct {
	ID        int64
	Name      string
	BaseURL   string
	APIKey    string // 解密后
	Protocol  string
	Status    string
	CreatedAt time.Time
}

var validProtocols = map[string]bool{"openai_chat": true, "openai_responses": true, "anthropic": true}

type ProviderStore struct {
	db     *sql.DB
	cipher *crypto.Cipher
}

func NewProviderStore(db *sql.DB, cipher *crypto.Cipher) *ProviderStore {
	return &ProviderStore{db: db, cipher: cipher}
}

func (s *ProviderStore) Create(name, baseURL, apiKey, protocol string) (Provider, error) {
	if !validProtocols[protocol] {
		return Provider{}, fmt.Errorf("invalid protocol %q", protocol)
	}
	enc, err := s.cipher.Encrypt([]byte(apiKey))
	if err != nil {
		return Provider{}, fmt.Errorf("encrypt api key: %w", err)
	}
	res, err := s.db.Exec(
		`INSERT INTO upstream_providers(name, base_url, api_key, protocol, status) VALUES(?, ?, ?, ?, 'active')`,
		name, baseURL, enc, protocol)
	if err != nil {
		return Provider{}, fmt.Errorf("create provider: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

func (s *ProviderStore) Get(id int64) (Provider, error) {
	row := s.db.QueryRow(
		`SELECT id, name, base_url, api_key, protocol, status, created_at FROM upstream_providers WHERE id=?`, id)
	return s.scan(row)
}

func (s *ProviderStore) List() ([]Provider, error) {
	rows, err := s.db.Query(
		`SELECT id, name, base_url, api_key, protocol, status, created_at FROM upstream_providers ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()
	var out []Provider
	for rows.Next() {
		p, err := s.scanRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *ProviderStore) Update(id int64, name, baseURL, apiKey, protocol, status string) error {
	if !validProtocols[protocol] {
		return fmt.Errorf("invalid protocol %q", protocol)
	}
	if status != "active" && status != "disabled" {
		return fmt.Errorf("invalid status %q", status)
	}
	var err error
	if apiKey != "" {
		var enc []byte
		enc, err = s.cipher.Encrypt([]byte(apiKey))
		if err != nil {
			return fmt.Errorf("encrypt api key: %w", err)
		}
		_, err = s.db.Exec(
			`UPDATE upstream_providers SET name=?, base_url=?, api_key=?, protocol=?, status=? WHERE id=?`,
			name, baseURL, enc, protocol, status, id)
	} else {
		_, err = s.db.Exec(
			`UPDATE upstream_providers SET name=?, base_url=?, protocol=?, status=? WHERE id=?`,
			name, baseURL, protocol, status, id)
	}
	return err
}

func (s *ProviderStore) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM upstream_providers WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete provider: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("provider not found")
	}
	return nil
}

func (s *ProviderStore) scan(row *sql.Row) (Provider, error) {
	var p Provider
	var enc []byte
	if err := row.Scan(&p.ID, &p.Name, &p.BaseURL, &enc, &p.Protocol, &p.Status, &p.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Provider{}, errors.New("provider not found")
		}
		return Provider{}, err
	}
	dec, err := s.cipher.Decrypt(enc)
	if err != nil {
		return Provider{}, fmt.Errorf("decrypt api key: %w", err)
	}
	p.APIKey = string(dec)
	return p, nil
}

func (s *ProviderStore) scanRows(r interface{ Scan(...any) error }) (Provider, error) {
	var p Provider
	var enc []byte
	if err := r.Scan(&p.ID, &p.Name, &p.BaseURL, &enc, &p.Protocol, &p.Status, &p.CreatedAt); err != nil {
		return Provider{}, err
	}
	dec, err := s.cipher.Decrypt(enc)
	if err != nil {
		return Provider{}, fmt.Errorf("decrypt api key: %w", err)
	}
	p.APIKey = string(dec)
	return p, nil
}
```

> 注:`validProtocols` 是包级变量;实现者注意 `scanRows` 用局部接口类型更规范(参照 ir 项目的 rowScanner 模式)。

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/catalog/ -v
```

预期:4 个测试 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/catalog/provider.go internal/catalog/provider_test.go
git commit -m "feat(catalog): provider store with encrypted api keys"
```

---

### Task 2: catalog — ModelStore + PriceStore

**Files:**
- Create: `internal/catalog/model.go`
- Create: `internal/catalog/price.go`
- Test: `internal/catalog/model_test.go`
- Test: `internal/catalog/price_test.go`

**Interfaces:**
- Consumes: `*sql.DB`
- Produces:

```go
type Model struct {
    ID            int64
    Name          string // 对外暴露的自定义名
    ProviderID    int64
    UpstreamModel string
    Enabled       bool
    CreatedAt     time.Time
}

type ModelStore struct { db *sql.DB }
func NewModelStore(db *sql.DB) *ModelStore
func (s *ModelStore) Create(name string, providerID int64, upstreamModel string) (Model, error)  // 默认 enabled=true
func (s *ModelStore) Get(id int64) (Model, error)
func (s *ModelStore) GetByName(name string) (Model, error)          // 模型解析用(大小写敏感)
func (s *ModelStore) List() ([]Model, error)                        // 全部
func (s *ModelStore) ListEnabled() ([]Model, error)                 // /v1/models 用
func (s *ModelStore) Update(id int64, name string, providerID int64, upstreamModel string, enabled bool) error
func (s *ModelStore) Delete(id int64) error                          // 若被 model_prices 引用,报错(或级联删价格)

// ---- prices ----

type Price struct {
    ID             int64
    ModelID        int64
    InputPrice     float64   // 每百万 token
    OutputPrice    float64
    CacheReadPrice *float64
    CacheWritePrice *float64
    Currency       string
    EffectiveFrom  time.Time
}

type PriceStore struct { db *sql.DB }
func NewPriceStore(db *sql.DB) *PriceStore
func (s *PriceStore) Set(modelID int64, inputPrice, outputPrice float64, cacheRead, cacheWrite *float64) (Price, error)
    // 插入新价格记录(effective_from=now),形成价格历史
func (s *PriceStore) GetCurrent(modelID int64) (Price, error)       // 最新 effective_from;无价格返回 ErrNoPrice
func (s *PriceStore) List(modelID int64) ([]Price, error)           // 价格历史(倒序)

var ErrNoPrice = errors.New("no price configured")
```

- [ ] **Step 1: 写失败测试**

`internal/catalog/model_test.go`:

```go
package catalog

import (
	"testing"

	"carryapi/internal/db"
)

func newModelStore(t *testing.T) (*ModelStore, *ProviderStore) {
	t.Helper()
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	ps := newProviderStore(t) // 注意:newProviderStore 开了自己的 db,这里要共享
	// 修正:model store 测试需要共享 db。见下方实现说明。
	return NewModelStore(d), nil
}
```

> **测试 helper 修正**:`newProviderStore(t)` 打开独立的 `:memory:` db,无法与 ModelStore 共享(不同连接)。实现者应创建一个 `newCatalogStore(t)` helper 返回共享的 `*sql.DB` + 三个 store:
> ```go
> type catalogFixture struct {
> 	db       *sql.DB
> 	providers *ProviderStore
> 	models    *ModelStore
> 	prices    *PriceStore
> }
> func newCatalogFixture(t *testing.T) *catalogFixture { /* 共享一个 :memory: db */ }
> ```
> 把 Task 1 的 `newProviderStore` 测试改用它(或保留 `newProviderStore` 单独跑,model 测试用 fixture)。实现者统一用 fixture。

模型测试(用共享 fixture):

```go
func TestModelCreateAndGetByName(t *testing.T) {
	f := newCatalogFixture(t)
	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	m, err := f.models.Create("my-gpt4", p.ID, "gpt-4o")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !m.Enabled {
		t.Error("default enabled should be true")
	}
	got, err := f.models.GetByName("my-gpt4")
	if err != nil || got.ProviderID != p.ID || got.UpstreamModel != "gpt-4o" {
		t.Fatalf("GetByName: %+v err %v", got, err)
	}
}

func TestModelGetByNameNotFound(t *testing.T) {
	f := newCatalogFixture(t)
	if _, err := f.models.GetByName("nope"); err == nil {
		t.Error("expected error for unknown model")
	}
}

func TestModelUpdateDisable(t *testing.T) {
	f := newCatalogFixture(t)
	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	m, _ := f.models.Create("m1", p.ID, "gpt-4o")
	if err := f.models.Update(m.ID, "m1", p.ID, "gpt-4o-mini", false); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := f.models.Get(m.ID)
	if got.UpstreamModel != "gpt-4o-mini" || got.Enabled {
		t.Errorf("after update: %+v", got)
	}
	// ListEnabled 不应包含禁用模型
	enabled, _ := f.models.ListEnabled()
	if len(enabled) != 0 {
		t.Errorf("ListEnabled = %d, want 0", len(enabled))
	}
}
```

`internal/catalog/price_test.go`:

```go
package catalog

import (
	"testing"
)

func TestPriceSetAndGetCurrent(t *testing.T) {
	f := newCatalogFixture(t)
	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	m, _ := f.models.Create("m1", p.ID, "gpt-4o")
	var cr float64 = 0.5
	price, err := f.prices.Set(m.ID, 5.0, 15.0, &cr, nil)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if price.InputPrice != 5.0 || price.OutputPrice != 15.0 || *price.CacheReadPrice != 0.5 {
		t.Errorf("price = %+v", price)
	}
	cur, err := f.prices.GetCurrent(m.ID)
	if err != nil || cur.ID != price.ID {
		t.Fatalf("GetCurrent: %+v err %v", cur, err)
	}
}

func TestPriceHistory(t *testing.T) {
	f := newCatalogFixture(t)
	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	m, _ := f.models.Create("m1", p.ID, "gpt-4o")
	f.prices.Set(m.ID, 1.0, 1.0, nil, nil)
	f.prices.Set(m.ID, 2.0, 2.0, nil, nil) // 涨价
	cur, _ := f.prices.GetCurrent(m.ID)
	if cur.InputPrice != 2.0 {
		t.Errorf("current = %f, want 2.0", cur.InputPrice)
	}
	hist, _ := f.prices.List(m.ID)
	if len(hist) != 2 {
		t.Errorf("history = %d, want 2", len(hist))
	}
}

func TestPriceNoPrice(t *testing.T) {
	f := newCatalogFixture(t)
	p, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	m, _ := f.models.Create("m1", p.ID, "gpt-4o")
	if _, err := f.prices.GetCurrent(m.ID); err != ErrNoPrice {
		t.Errorf("err = %v, want ErrNoPrice", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/catalog/ -v
```

预期:编译失败。

- [ ] **Step 3: 实现 model.go**

```go
package catalog

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Model struct {
	ID            int64
	Name          string
	ProviderID    int64
	UpstreamModel string
	Enabled       bool
	CreatedAt     time.Time
}

type ModelStore struct {
	db *sql.DB
}

func NewModelStore(db *sql.DB) *ModelStore {
	return &ModelStore{db: db}
}

func (s *ModelStore) Create(name string, providerID int64, upstreamModel string) (Model, error) {
	res, err := s.db.Exec(
		`INSERT INTO custom_models(name, provider_id, upstream_model, enabled) VALUES(?, ?, ?, 1)`,
		name, providerID, upstreamModel)
	if err != nil {
		return Model{}, fmt.Errorf("create model: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

func (s *ModelStore) Get(id int64) (Model, error) {
	return s.scan(s.db.QueryRow(
		`SELECT id, name, provider_id, upstream_model, enabled, created_at FROM custom_models WHERE id=?`, id))
}

func (s *ModelStore) GetByName(name string) (Model, error) {
	return s.scan(s.db.QueryRow(
		`SELECT id, name, provider_id, upstream_model, enabled, created_at FROM custom_models WHERE name=?`, name))
}

func (s *ModelStore) List() ([]Model, error) {
	return s.listWhere(`1=1`)
}

func (s *ModelStore) ListEnabled() ([]Model, error) {
	return s.listWhere(`enabled=1`)
}

func (s *ModelStore) listWhere(cond string) ([]Model, error) {
	rows, err := s.db.Query(
		`SELECT id, name, provider_id, upstream_model, enabled, created_at FROM custom_models WHERE ` + cond + ` ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer rows.Close()
	var out []Model
	for rows.Next() {
		m, err := s.scanRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *ModelStore) Update(id int64, name string, providerID int64, upstreamModel string, enabled bool) error {
	_, err := s.db.Exec(
		`UPDATE custom_models SET name=?, provider_id=?, upstream_model=?, enabled=? WHERE id=?`,
		name, providerID, upstreamModel, enabled, id)
	return err
}

func (s *ModelStore) Delete(id int64) error {
	// 先删价格,再删模型
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM model_prices WHERE model_id=?`, id); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM custom_models WHERE id=?`, id); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *ModelStore) scan(row *sql.Row) (Model, error) {
	var m Model
	err := row.Scan(&m.ID, &m.Name, &m.ProviderID, &m.UpstreamModel, &m.Enabled, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Model{}, errors.New("model not found")
	}
	return m, err
}

func (s *ModelStore) scanRows(r interface{ Scan(...any) error }) (Model, error) {
	var m Model
	err := r.Scan(&m.ID, &m.Name, &m.ProviderID, &m.UpstreamModel, &m.Enabled, &m.CreatedAt)
	return m, err
}
```

- [ ] **Step 4: 实现 price.go**

```go
package catalog

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrNoPrice = errors.New("no price configured")

type Price struct {
	ID              int64
	ModelID         int64
	InputPrice      float64
	OutputPrice     float64
	CacheReadPrice  *float64
	CacheWritePrice *float64
	Currency        string
	EffectiveFrom   time.Time
}

type PriceStore struct {
	db *sql.DB
}

func NewPriceStore(db *sql.DB) *PriceStore {
	return &PriceStore{db: db}
}

func (s *PriceStore) Set(modelID int64, inputPrice, outputPrice float64, cacheRead, cacheWrite *float64) (Price, error) {
	res, err := s.db.Exec(
		`INSERT INTO model_prices(model_id, input_price, output_price, cache_read_price, cache_write_price) VALUES(?, ?, ?, ?, ?)`,
		modelID, inputPrice, outputPrice, cacheRead, cacheWrite)
	if err != nil {
		return Price{}, fmt.Errorf("set price: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

func (s *PriceStore) Get(id int64) (Price, error) {
	return s.scan(s.db.QueryRow(
		`SELECT id, model_id, input_price, output_price, cache_read_price, cache_write_price, currency, effective_from
		 FROM model_prices WHERE id=?`, id))
}

func (s *PriceStore) GetCurrent(modelID int64) (Price, error) {
	row := s.db.QueryRow(
		`SELECT id, model_id, input_price, output_price, cache_read_price, cache_write_price, currency, effective_from
		 FROM model_prices WHERE model_id=? ORDER BY effective_from DESC, id DESC LIMIT 1`, modelID)
	p, err := s.scan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Price{}, ErrNoPrice
	}
	return p, err
}

func (s *PriceStore) List(modelID int64) ([]Price, error) {
	rows, err := s.db.Query(
		`SELECT id, model_id, input_price, output_price, cache_read_price, cache_write_price, currency, effective_from
		 FROM model_prices WHERE model_id=? ORDER BY effective_from DESC`, modelID)
	if err != nil {
		return nil, fmt.Errorf("list prices: %w", err)
	}
	defer rows.Close()
	var out []Price
	for rows.Next() {
		p, err := s.scanRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *PriceStore) scan(row *sql.Row) (Price, error) {
	var p Price
	err := row.Scan(&p.ID, &p.ModelID, &p.InputPrice, &p.OutputPrice, &p.CacheReadPrice,
		&p.CacheWritePrice, &p.Currency, &p.EffectiveFrom)
	return p, err
}

func (s *PriceStore) scanRows(r interface{ Scan(...any) error }) (Price, error) {
	var p Price
	err := r.Scan(&p.ID, &p.ModelID, &p.InputPrice, &p.OutputPrice, &p.CacheReadPrice,
		&p.CacheWritePrice, &p.Currency, &p.EffectiveFrom)
	return p, err
}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
go test ./internal/catalog/ -v
```

预期:全部 PASS(Task 1 的 4 个 + 模型 3 个 + 价格 3 个)。

- [ ] **Step 6: 提交**

```bash
git add internal/catalog/model.go internal/catalog/price.go internal/catalog/model_test.go internal/catalog/price_test.go
git commit -m "feat(catalog): model and price stores with history"
```

---

### Task 3: catalog — admin CRUD handlers + 路由

**Files:**
- Create: `internal/catalog/handler.go`
- Test: `internal/catalog/handler_test.go`
- Modify: `internal/server/router.go`(挂 catalog admin 路由)
- Modify: `internal/server/server.go`(Deps 加 Catalog)

**Interfaces:**
- Consumes: 三个 store
- Produces:

```go
type Handler struct {
	providers *ProviderStore
	models    *ModelStore
	prices    *PriceStore
}
func NewHandler(providers *ProviderStore, models *ModelStore, prices *PriceStore) *Handler

// admin-only(路由层用 middleware.RequireRole("admin") 守卫)
// GET/POST/PUT/DELETE /api/providers(JSON)
// GET/POST/PUT/DELETE /api/models
// GET/PUT /api/models/{id}/price (取当前价格 / 设新价格)
```

- [ ] **Step 1: 写失败测试**

`internal/catalog/handler_test.go`:

```go
package catalog

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"carryapi/internal/user"
	"carryapi/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// admin context helper
func adminCtx() context.Context {
	u := &user.User{ID: 1, Email: "admin@x.com", Role: "admin", Status: "active"}
	return context.WithValue(context.Background(), middleware.UserKey{}, u)
}

func TestProviderCRUDHandler(t *testing.T) {
	f := newCatalogFixture(t)
	h := NewHandler(f.providers, f.models, f.prices)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Post("/api/providers", h.CreateProvider)
	r.With(middleware.RequireRole("admin")).Get("/api/providers", h.ListProviders)

	// create
	body, _ := json.Marshal(map[string]string{"name": "OpenAI", "base_url": "https://api.openai.com/v1", "api_key": "sk-1", "protocol": "openai_chat"})
	req := httptest.NewRequest("POST", "/api/providers", bytes.NewReader(body))
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("create code=%d body=%s", rec.Code, rec.Body.String())
	}
	// list
	req = httptest.NewRequest("GET", "/api/providers", nil)
	req = req.WithContext(adminCtx())
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0]["name"] != "OpenAI" {
		t.Errorf("list = %+v", list)
	}
}

func TestProviderCRUDNonAdminForbidden(t *testing.T) {
	f := newCatalogFixture(t)
	h := NewHandler(f.providers, f.models, f.prices)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Post("/api/providers", h.CreateProvider)
	// 非 admin context
	u := &user.User{ID: 2, Email: "user@x.com", Role: "user", Status: "active"}
	req := httptest.NewRequest("POST", "/api/providers", bytes.NewReader([]byte(`{}`)))
	req = req.WithContext(context.WithValue(context.Background(), middleware.UserKey{}, u))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 403 {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}
```

> 其余 handler 测试(补全):

```go
func TestModelCRUDHandler(t *testing.T) {
	f := newCatalogFixture(t)
	h := NewHandler(f.providers, f.models, f.prices)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Post("/api/models", h.CreateModel)
	r.With(middleware.RequireRole("admin")).Get("/api/models", h.ListModels)
	// 先建 provider(模型引用它)
	prov, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "sk-1", "openai_chat")
	// create model
	body, _ := json.Marshal(map[string]any{"name": "my-gpt4", "provider_id": prov.ID, "upstream_model": "gpt-4o"})
	req := httptest.NewRequest("POST", "/api/models", bytes.NewReader(body))
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("create model code=%d body=%s", rec.Code, rec.Body.String())
	}
	// list
	req = httptest.NewRequest("GET", "/api/models", nil)
	req = req.WithContext(adminCtx())
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0]["name"] != "my-gpt4" {
		t.Errorf("list = %+v", list)
	}
}

func TestPriceHandler(t *testing.T) {
	f := newCatalogFixture(t)
	h := NewHandler(f.providers, f.models, f.prices)
	r := chi.NewRouter()
	r.With(middleware.RequireRole("admin")).Put("/api/models/{id}/price", h.SetModelPrice)
	r.With(middleware.RequireRole("admin")).Get("/api/models/{id}/price", h.GetModelPrice)
	prov, _ := f.providers.Create("OpenAI", "https://api.openai.com/v1", "sk-1", "openai_chat")
	m, _ := f.models.Create("my-gpt4", prov.ID, "gpt-4o")
	// set price
	body, _ := json.Marshal(map[string]any{"input_price": 5.0, "output_price": 15.0})
	req := httptest.NewRequest("PUT", "/api/models/"+strconv.FormatInt(m.ID, 10)+"/price", bytes.NewReader(body))
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("set price code=%d body=%s", rec.Code, rec.Body.String())
	}
	// get price
	req = httptest.NewRequest("GET", "/api/models/"+strconv.FormatInt(m.ID, 10)+"/price", nil)
	req = req.WithContext(adminCtx())
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	price, ok := resp["price"].(map[string]any)
	if !ok || price["input_price"] != 5.0 {
		t.Errorf("price = %+v", resp)
	}
}
```

> 上述测试需要 import:`bytes`、`context`、`encoding/json`、`net/http`、`net/http/httptest`、`strconv`、`testing`、`github.com/go-chi/chi/v5`、`carryapi/internal/middleware`、`carryapi/internal/user`。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/catalog/ -v -run Handler
```

预期:编译失败。

- [ ] **Step 3: 实现 handler.go**

```go
package catalog

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	providers *ProviderStore
	models    *ModelStore
	prices    *PriceStore
}

func NewHandler(providers *ProviderStore, models *ModelStore, prices *PriceStore) *Handler {
	return &Handler{providers: providers, models: models, prices: prices}
}

func jsonOut(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func jsonErr(w http.ResponseWriter, status int, msg string) {
	jsonOut(w, status, map[string]string{"error": msg})
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
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
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
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := h.providers.Delete(id); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]string{"status": "ok"})
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
		out = append(out, map[string]any{
			"id": m.ID, "name": m.Name, "provider_id": m.ProviderID,
			"upstream_model": m.UpstreamModel, "enabled": m.Enabled, "created_at": m.CreatedAt,
		})
	}
	jsonOut(w, 200, out)
}

func (h *Handler) CreateModel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string `json:"name"`
		ProviderID     int64  `json:"provider_id"`
		UpstreamModel  string `json:"upstream_model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	m, err := h.models.Create(req.Name, req.ProviderID, req.UpstreamModel)
	if err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]any{"id": m.ID, "name": m.Name})
}

func (h *Handler) UpdateModel(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var req struct {
		Name          string `json:"name"`
		ProviderID    int64  `json:"provider_id"`
		UpstreamModel string `json:"upstream_model"`
		Enabled       *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if err := h.models.Update(id, req.Name, req.ProviderID, req.UpstreamModel, enabled); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) DeleteModel(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := h.models.Delete(id); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOut(w, 200, map[string]string{"status": "ok"})
}

// ---- prices ----

func (h *Handler) GetModelPrice(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	price, err := h.prices.GetCurrent(id)
	if err != nil {
		if err == ErrNoPrice {
			jsonOut(w, 200, map[string]any{"model_id": id, "price": nil})
			return
		}
		jsonErr(w, 500, "failed to get price")
		return
	}
	jsonOut(w, 200, map[string]any{"model_id": id, "price": price})
}

func (h *Handler) SetModelPrice(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var req struct {
		InputPrice     float64  `json:"input_price"`
		OutputPrice    float64  `json:"output_price"`
		CacheReadPrice *float64 `json:"cache_read_price"`
		CacheWritePrice *float64 `json:"cache_write_price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid body")
		return
	}
	price, err := h.prices.Set(id, req.InputPrice, req.OutputPrice, req.CacheReadPrice, req.CacheWritePrice)
	if err != nil {
		jsonErr(w, 500, "failed to set price")
		return
	}
	jsonOut(w, 200, map[string]any{"id": price.ID, "model_id": id})
}
```

> 注:`jsonOut`/`jsonErr` 与 `internal/api` 的 `JSON`/`JSONError` 重名但包不同,无冲突。路由挂载:
> ```go
> r.Group(func(r chi.Router) {
>     r.Use(middleware.RequireLogin())
>     r.Use(middleware.RequireRole("admin"))
>     r.Get("/api/providers", s.catalog.ListProviders)
>     r.Post("/api/providers", s.catalog.CreateProvider)
>     r.Put("/api/providers/{id}", s.catalog.UpdateProvider)
>     r.Delete("/api/providers/{id}", s.catalog.DeleteProvider)
>     r.Get("/api/models", s.catalog.ListModels)
>     r.Post("/api/models", s.catalog.CreateModel)
>     r.Put("/api/models/{id}", s.catalog.UpdateModel)
>     r.Delete("/api/models/{id}", s.catalog.DeleteModel)
>     r.Get("/api/models/{id}/price", s.catalog.GetModelPrice)
>     r.Put("/api/models/{id}/price", s.catalog.SetModelPrice)
> })
> ```
> `Deps` 加 `Catalog *catalog.Handler`;main.go 构造 `catalog.NewHandler(...)` 注入。

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/catalog/ -v
go test ./... -count=1
```

预期:全部 PASS(含子项目 1-3 的测试)。

- [ ] **Step 5: 提交**

```bash
git add internal/catalog/handler.go internal/catalog/handler_test.go internal/server/router.go internal/server/server.go cmd/carryapi/main.go
git commit -m "feat(catalog): admin CRUD handlers for providers, models, prices"
```

---

### Task 4: proxy — 结构 + 鉴权 + 模型解析 + 配额预检

**Files:**
- Create: `internal/proxy/proxy.go`
- Create: `internal/proxy/auth.go`
- Create: `internal/proxy/model.go`
- Test: `internal/proxy/auth_test.go`
- Test: `internal/proxy/model_test.go`

**Interfaces:**
- Consumes: `apikey.Store`、`user.Store`、`catalog.ModelStore`、`catalog.ProviderStore`、`catalog.PriceStore`、`user.Store`(配额)、`*sql.DB`(日志)
- Produces:

```go
// proxy.go
type Deps struct {
	DB        *sql.DB
	Keys      *apikey.Store
	Users     *user.Store
	Models    *catalog.ModelStore
	Providers *catalog.ProviderStore
	Prices    *catalog.PriceStore
	Client    *http.Client // 上游 HTTP 客户端(可注入测试)
}

type Proxy struct {
	deps Deps
}

func NewProxy(deps Deps) *Proxy

// ServeHTTP 按路径分派代理端点。
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request)
// 路径: /v1/chat/completions /v1/completions -> chat; /v1/responses -> responses; /v1/messages -> anthropic; /v1/models -> 模型列表
// 返回错误:404 未匹配路径 -> ir.OpenAIErrorBody(下游未知,按 OpenAI 返)

// auth.go
func (p *Proxy) authenticate(r *http.Request) (*user.User, *apikey.APIKey, error)
// 提取 API Key: Authorization: Bearer <key> 或 x-api-key: <key>
// keys.Authenticate -> users.GetByID -> status active 检查
// 错误返回 ir.Error(authentication / user_disabled)

// model.go
func (p *Proxy) resolveModel(name string) (*catalog.Model, *catalog.Provider, *catalog.Price, error)
// 模型名 -> ModelStore.GetByName(检查 enabled) -> ProviderStore.Get(检查 active) -> PriceStore.GetCurrent
// 无价格 -> ErrNoPrice(允许?不:代理要求模型必须配价,否则无法计费 -> 返回配置错误)

func (p *Proxy) checkQuota(u *user.User, keyID int64) error
// user.Store.GetQuotas("user", u.ID) + ("key", keyID)
// 若某 quota 有 limit_tokens/limit_cost 且 used >= limit -> 429 ir.Error{Type:"quota"}
```

- [ ] **Step 1: 写失败测试**

`internal/proxy/auth_test.go`:

```go
package proxy

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"carryapi/internal/apikey"
	"carryapi/internal/db"
	"carryapi/internal/user"
)

func newAuthFixture(t *testing.T) (*Proxy, *user.User) {
	t.Helper()
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	c := mustCipher(t)
	us := user.New(d, c)
	ks := apikey.New(d)
	p := NewProxy(Deps{DB: d, Keys: ks, Users: us})
	// 建用户 + key
	u, _ := us.Create("proxy@x.com", "hash", "user")
	return p, u
}

func TestAuthenticateBearer(t *testing.T) {
	p, u := newAuthFixture(t)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	got, key, err := p.authenticate(req)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if got.ID != u.ID || key.ID == 0 {
		t.Errorf("user=%d key=%d", got.ID, key.ID)
	}
}

func TestAuthenticateXAPIKey(t *testing.T) {
	p, u := newAuthFixture(t)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	req := httptest.NewRequest("POST", "/v1/messages", nil)
	req.Header.Set("x-api-key", plaintext)
	_, _, err := p.authenticate(req)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
}

func TestAuthenticateMissing(t *testing.T) {
	p, _ := newAuthFixture(t)
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	_, _, err := p.authenticate(req)
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestAuthenticateDisabledUser(t *testing.T) {
	p, u := newAuthFixture(t)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	p.deps.Users.UpdateStatus(u.ID, "disabled")
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	_, _, err := p.authenticate(req)
	if err == nil {
		t.Fatal("expected error for disabled user")
	}
}
```

`internal/proxy/model_test.go`:

```go
package proxy

import (
	"testing"

	"carryapi/internal/catalog"
)

func newModelFixture(t *testing.T) (*Proxy, int64) {
	t.Helper()
	p, _ := newAuthFixture(t) // 复用 db + cipher
	d := p.deps.DB
	ps := catalog.NewProviderStore(d, mustCipher(t))
	ms := catalog.NewModelStore(d)
	pr := catalog.NewPriceStore(d)
	prov, _ := ps.Create("OpenAI", "https://api.openai.com/v1", "sk-1", "openai_chat")
	m, _ := ms.Create("my-gpt4", prov.ID, "gpt-4o")
	var cr float64 = 0.5
	pr.Set(m.ID, 5.0, 15.0, &cr, nil)
	p.deps.Providers = ps
	p.deps.Models = ms
	p.deps.Prices = pr
	return p, prov.ID
}

func TestResolveModel(t *testing.T) {
	p, _ := newModelFixture(t)
	model, provider, price, err := p.resolveModel("my-gpt4")
	if err != nil {
		t.Fatalf("resolveModel: %v", err)
	}
	if model.UpstreamModel != "gpt-4o" || provider.BaseURL != "https://api.openai.com/v1" || price.InputPrice != 5.0 {
		t.Errorf("resolve: model=%+v provider=%+v price=%+v", model, provider, price)
	}
}

func TestResolveModelNotFound(t *testing.T) {
	p, _ := newModelFixture(t)
	if _, _, _, err := p.resolveModel("nope"); err == nil {
		t.Error("expected error for unknown model")
	}
}
```

> `mustCipher` helper:测试文件内定义(同前)。注意 `newModelFixture` 里 `newAuthFixture` 已建 cipher,但 ProviderStore 需要 cipher——两个 fixture 的 cipher 不同会怎样?ProviderStore 用 `mustCipher(t)` 新建的 cipher 加密,proxy 的 provider 读取也用它——**必须同一个 cipher**。修正:fixture 里用一个共享 cipher 构造所有 store。实现者注意。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/proxy/ -v
```

预期:编译失败。

- [ ] **Step 3: 实现 proxy.go**

```go
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
	stream     bool // 流式请求(记录到日志)
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
	case "/v1/models":
		p.handleModels(w, r)
	default:
		body := ir.OpenAIErrorBody(ir.NewError("not_found", "invalid_request_url", "invalid request url", 404))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		w.Write(body)
	}
}
```

- [ ] **Step 4: 实现 auth.go**

```go
package proxy

import (
	"net/http"
	"strings"

	"carryapi/internal/apikey"
	"carryapi/internal/ir"
	"carryapi/internal/user"
)

// authenticate 提取 API Key 并校验。
func (p *Proxy) authenticate(r *http.Request) (*user.User, *apikey.APIKey, error) {
	key := extractAPIKey(r)
	if key == "" {
		return nil, nil, ir.NewError("authentication", "invalid_api_key", "missing api key", 401)
	}
	userID, keyID, err := p.deps.Keys.Authenticate(key)
	if err != nil {
		return nil, nil, ir.NewError("authentication", "invalid_api_key", "invalid api key", 401)
	}
	u, err := p.deps.Users.GetByID(userID)
	if err != nil || u.Status != "active" {
		return nil, nil, ir.NewError("user_disabled", "user_disabled", "user is disabled", 403)
	}
	ak, err := p.deps.Keys.Get(keyID, userID)
	if err != nil {
		return nil, nil, ir.NewError("authentication", "invalid_api_key", "invalid api key", 401)
	}
	return u, &ak, nil
}

func extractAPIKey(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if strings.HasPrefix(h, "Bearer ") {
			return strings.TrimPrefix(h, "Bearer ")
		}
		return h
	}
	if h := r.Header.Get("x-api-key"); h != "" {
		return h
	}
	return ""
}
```

- [ ] **Step 5: 实现 model.go**

```go
package proxy

import (
	"carryapi/internal/catalog"
	"carryapi/internal/ir"
	"carryapi/internal/user"
)

func (p *Proxy) resolveModel(name string) (*catalog.Model, *catalog.Provider, *catalog.Price, error) {
	model, err := p.deps.Models.GetByName(name)
	if err != nil {
		return nil, nil, nil, ir.NewError("not_found", "model_not_found", "The model '"+name+"' does not exist", 404)
	}
	if !model.Enabled {
		return nil, nil, nil, ir.NewError("not_found", "model_not_found", "The model '"+name+"' is disabled", 404)
	}
	provider, err := p.deps.Providers.Get(model.ProviderID)
	if err != nil {
		return nil, nil, nil, ir.NewError("internal", "provider_not_found", "provider not configured", 500)
	}
	if provider.Status != "active" {
		return nil, nil, nil, ir.NewError("internal", "provider_disabled", "provider is disabled", 500)
	}
	price, err := p.deps.Prices.GetCurrent(model.ID)
	if err != nil {
		return nil, nil, nil, ir.NewError("internal", "price_not_configured", "model has no price configured", 500)
	}
	return &model, &provider, &price, nil
}

// checkQuota 请求前预检:token/费用上限。
func (p *Proxy) checkQuota(u *user.User, keyID int64) error {
	scopes := []struct {
		scope   string
		scopeID int64
	}{
		{"user", u.ID},
		{"key", keyID},
	}
	for _, s := range scopes {
		quotas, _ := p.deps.Users.GetQuotas(s.scope, s.scopeID)
		for _, q := range quotas {
			limitTokens := q.LimitTokens
			limitCost := q.LimitCost
			if limitTokens != nil && q.UsedTokens >= *limitTokens {
				return ir.NewError("rate_limit", "quota_exceeded", "quota exceeded (tokens)", 429)
			}
			if limitCost != nil && q.UsedCost >= *limitCost {
				return ir.NewError("rate_limit", "quota_exceeded", "quota exceeded (cost)", 429)
			}
		}
	}
	return nil
}
```

- [ ] **Step 6: 运行测试确认通过**

```bash
go test ./internal/proxy/ -v
```

预期:7 个测试 PASS(4 auth + 2 model + 1 fixture)。

- [ ] **Step 7: 提交**

```bash
git add internal/proxy/proxy.go internal/proxy/auth.go internal/proxy/model.go internal/proxy/auth_test.go internal/proxy/model_test.go
git commit -m "feat(proxy): proxy core with api key auth, model resolution, quota check"
```

---

### Task 5: proxy — 非流式转发

**Files:**
- Create: `internal/proxy/forward.go`
- Test: `internal/proxy/forward_test.go`

**Interfaces:**
- Produces:

```go
// handleProxy 是四个代理端点的统一入口(按下游协议分派)。
func (p *Proxy) handleProxy(w http.ResponseWriter, r *http.Request, downstream string)

// 流程:
// 1. 生成 requestID(uuid),设 X-Request-Id 响应头
// 2. authenticate -> 401/403
// 3. 读请求体;按 downstream 选 Decoder(chat/responses/anthropic)
//    -> ir.Request;若失败 -> 400 invalid_request
// 4. resolveModel(irReq.Model) -> 404/500
// 5. checkQuota -> 429
// 6. 按 provider.Protocol 选 Encoder 把 ir.Req 编码成上游格式
// 7. 构造上游请求:POST provider.BaseURL + path(按 provider.Protocol);
//    注入上游 API Key(Authorization Bearer,anthropic 用 x-api-key + anthropic-version)
// 8. 若 irReq.Stream:走 stream.go 的流式转发;否则非流式:
//    - 发上游请求,读完整响应
//    - 上游 Decoder -> ir.Response(若上游错误:按状态码转 ir.Error)
//    - 下游 Encoder -> 写回
// 9. 统计(stats.go):写 request_logs + 配额累加

// providerPath 返回上游请求路径。
func providerPath(protocol string) string
//  openai_chat -> "/chat/completions"; openai_responses -> "/responses"; anthropic -> "/v1/messages"

// buildUpstreamRequest 构造上游 HTTP 请求。
func (p *Proxy) buildUpstreamRequest(r *http.Request, provider *catalog.Provider, payload []byte) (*http.Request, error)
//  BaseURL 拼接 path;Authorization: Bearer <key>;anthropic 额外 x-api-key + anthropic-version: 2023-06-01;Content-Type: application/json
```

- [ ] **Step 1: 写失败测试**

`internal/proxy/forward_test.go`:

```go
package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"carryapi/internal/catalog"
)

// mockUpstream 返回固定 Chat 响应。
func newUpstreamServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-upstream" {
			t.Errorf("upstream auth = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"chatcmpl-up","object":"chat.completion","model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"Hello from upstream"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`))
	}))
}

func newProxyWithUpstream(t *testing.T, upstreamURL string) (*Proxy, *user.User) {
	t.Helper()
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	c := mustCipher(t)
	us := user.New(d, c)
	ks := apikey.New(d)
	ps := catalog.NewProviderStore(d, c)
	ms := catalog.NewModelStore(d)
	pr := catalog.NewPriceStore(d)
	p := NewProxy(Deps{DB: d, Keys: ks, Users: us, Models: ms, Providers: ps, Prices: pr})
	u, _ := us.Create("proxy@x.com", "hash", "user")
	prov, _ := ps.Create("Mock", upstreamURL, "sk-upstream", "openai_chat")
	m, _ := ms.Create("my-gpt4", prov.ID, "gpt-4o")
	pr.Set(m.ID, 5.0, 15.0, nil, nil)
	return p, u
}

func TestNonStreamingChat(t *testing.T) {
	up := newUpstreamServer(t)
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")

	body, _ := json.Marshal(map[string]any{
		"model": "my-gpt4",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Request-Id") == "" {
		t.Error("missing X-Request-Id header")
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	choices := resp["choices"].([]any)
	msg := choices[0].(map[string]any)["message"].(map[string]any)
	if msg["content"] != "Hello from upstream" {
		t.Errorf("content = %v", msg["content"])
	}
	// 统计已写入 request_logs
	var count int
	p.deps.DB.QueryRow("SELECT COUNT(*) FROM request_logs").Scan(&count)
	if count != 1 {
		t.Errorf("request_logs = %d, want 1", count)
	}
}

func TestNonStreamingAuthFailure(t *testing.T) {
	up := newUpstreamServer(t)
	defer up.Close()
	p, _ := newProxyWithUpstream(t, up.URL)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("code=%d, want 401", rec.Code)
	}
}

func TestNonStreamingModelNotFound(t *testing.T) {
	up := newUpstreamServer(t)
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{"model": "nope", "messages": []map[string]string{{"role": "user", "content": "hi"}}})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Errorf("code=%d, want 404", rec.Code)
	}
}

// anthropic 上游
func newAnthropicUpstream(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "sk-anthropic" {
			t.Errorf("anthropic auth = %q", r.Header.Get("x-api-key"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"msg_up","type":"message","role":"assistant","model":"claude-3-5-sonnet-20241022","content":[{"type":"text","text":"Bonjour"}],"stop_reason":"end_turn","usage":{"input_tokens":4,"output_tokens":2,"cache_creation_input_tokens":1,"cache_read_input_tokens":0}}`))
	}))
}

func TestCrossProtocolChatToAnthropic(t *testing.T) {
	up := newAnthropicUpstream(t)
	defer up.Close()
	p, u := newProxyWithUpstreamAnthropic(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	// 客户端用 Chat 协议调用,上游是 Anthropic
	body, _ := json.Marshal(map[string]any{
		"model": "my-claude",
		"messages": []map[string]string{{"role": "user", "content": "salut"}},
	})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	// 下游 Chat 格式
	choices := resp["choices"].([]any)
	msg := choices[0].(map[string]any)["message"].(map[string]any)
	if msg["content"] != "Bonjour" {
		t.Errorf("content = %v", msg["content"])
	}
	// 统计里 cache_creation 应保留
	var cc int
	p.deps.DB.QueryRow("SELECT cache_creation_tokens FROM request_logs").Scan(&cc)
	if cc != 1 {
		t.Errorf("cache_creation = %d, want 1", cc)
	}
}

func newProxyWithUpstreamAnthropic(t *testing.T, upstreamURL string) (*Proxy, *user.User) {
	// 同 newProxyWithUpstream,但 provider protocol="anthropic"
	// 实现者:复制 helper 改 protocol + 模型名 my-claude + 上游模型名
}
```

> `newProxyWithUpstreamAnthropic` 是复制的 helper(改 protocol=anthropic、模型名)。实现者注意两个 helper 有重复代码——可抽一个 `newProxyWithProvider(t, upstreamURL, protocol, modelName, upstreamModel)` 统一。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/proxy/ -v -run "NonStreaming|CrossProtocol"
```

预期:编译失败(handleProxy 未定义)。

- [ ] **Step 3: 实现 forward.go**

```go
package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func (p *Proxy) buildUpstreamRequest(r *http.Request, provider *catalog.Provider, payload []byte) (*http.Request, error) {
	url := provider.BaseURL + providerPath(provider.Protocol)
	req, err := http.NewRequestWithContext(r.Context(), "POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if provider.Protocol == "anthropic" {
		req.Header.Set("x-api-key", provider.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	}
	return req, nil
}

// handleProxy 统一代理入口。
func (p *Proxy) handleProxy(w http.ResponseWriter, r *http.Request, downstream string) {
	rc := &requestContext{downstream: downstream, requestID: uuid.NewString(), start: time.Now()}
	w.Header().Set("X-Request-Id", rc.requestID)

	// 1. 鉴权
	u, ak, err := p.authenticate(r)
	if err != nil {
		p.writeError(w, rc, err)
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

	// 3. 模型解析
	model, provider, price, err := p.resolveModel(irReq.Model)
	if err != nil {
		p.writeError(w, rc, err)
		return
	}
	rc.model = model
	rc.provider = provider
	rc.price = price

	// 4. 配额预检
	if err := p.checkQuota(u, ak.ID); err != nil {
		p.writeError(w, rc, err)
		return
	}

	// 5. 编码为上游格式
	upstreamPayload, err := encodeUpstreamRequest(provider.Protocol, irReq)
	if err != nil {
		p.writeError(w, rc, ir.NewError("internal", "encode_failed", "failed to encode upstream request", 500))
		return
	}

	// 6. 转发
	upReq, err := p.buildUpstreamRequest(r, provider, upstreamPayload)
	if err != nil {
		p.writeError(w, rc, ir.NewError("internal", "upstream_build_failed", "failed to build upstream request", 500))
		return
	}
	upResp, err := p.deps.Client.Do(upReq)
	if err != nil {
		p.writeError(w, rc, ir.NewError("upstream", "upstream_unreachable", "upstream request failed: "+err.Error(), 502))
		return
	}
	defer upResp.Body.Close()

	// 7. 流式 or 非流式
	rc.stream = irReq.Stream
	if irReq.Stream {
		p.streamResponse(w, rc, upResp, downstream)
		return
	}
	p.forwardNonStreaming(w, rc, upResp, downstream)
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
		p.writeError(w, rc, ir.NewError("upstream", "upstream_error", fmt.Sprintf("upstream returned %d: %s", upResp.StatusCode, truncate(body, 200)), 502))
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
```

> 注:`var _ =` 占位若 import 未用则删。`recordStats` 在 Task 6(stats.go)实现——本任务编译需先有 stats 骨架。**顺序**:实现者在本任务先加一个空的 `recordStats` stub 在 stats.go(本任务只建文件+空函数,Task 6 填充),或把 forward.go 的 recordStats 调用推迟。建议:本任务先建 stats.go 的空 `func (p *Proxy) recordStats(rc *requestContext)`(不写日志),Task 6 填充完整逻辑并加测试。

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/proxy/ -v
```

预期:非流式测试 PASS(含跨协议 Chat→Anthropic)。request_logs 计数测试此时因 recordStats 为空会失败——**实现者按 Step 3 注:Task 5 先放空 recordStats,`TestNonStreamingChat` 的 request_logs 断言改为在 Task 6 加**。或直接实现 recordStats 的最小版(INSERT request_logs,不累计配额)。建议直接实现最小版,让测试全绿。

- [ ] **Step 5: 提交**

```bash
git add internal/proxy/forward.go internal/proxy/forward_test.go internal/proxy/stats.go
git commit -m "feat(proxy): non-streaming upstream forwarding with protocol conversion"
```

---

### Task 6: proxy — 流式转发 + 统计层

**Files:**
- Create: `internal/proxy/stream.go`
- Create: `internal/proxy/stats.go`
- Test: `internal/proxy/stream_test.go`
- Test: `internal/proxy/stats_test.go`

**Interfaces:**
- Produces:

```go
// stream.go
// streamResponse 流式转发:上游 SSE -> SplitSSE -> 上游 Decoder -> []Event
// -> 统计收集(EventUsage) -> 下游 Encoder -> EncodeSSELine -> 写回。
func (p *Proxy) streamResponse(w http.ResponseWriter, rc *requestContext, upResp *http.Response, downstream string)

// stats.go
// recordStats 在请求结束时写 request_logs + 累加配额。
func (p *Proxy) recordStats(rc *requestContext)

// Cost 计算:price(每百万) * tokens / 1e6;cache_creation 用 cache_write_price(若无用 input_price)。
func computeCost(price *catalog.Price, rc *requestContext) float64
```

- [ ] **Step 1: 写失败测试**

`internal/proxy/stream_test.go`:

```go
package proxy

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// mockUpstreamStreaming 返回 Chat 流式 SSE。
func newStreamingUpstream(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hel\"}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"lo\"}}]}\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":3,\"total_tokens\":8}}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
}

func TestStreamingChat(t *testing.T) {
	up := newStreamingUpstream(t)
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{
		"model": "my-gpt4",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
		"stream": true,
	})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	out := rec.Body.String()
	if !bytes.Contains(out, []byte(`"content":"Hel"`)) || !bytes.Contains(out, []byte(`"content":"lo"`)) {
		t.Errorf("missing content deltas:\n%s", out)
	}
	if !bytes.Contains(out, []byte(`"finish_reason":"stop"`)) {
		t.Errorf("missing finish reason:\n%s", out)
	}
	if !bytes.Contains(out, []byte("data: [DONE]")) {
		t.Errorf("missing [DONE]:\n%s", out)
	}
	// 统计应记录 token
	var input, output int
	p.deps.DB.QueryRow("SELECT input_tokens, output_tokens FROM request_logs").Scan(&input, &output)
	if input != 5 || output != 3 {
		t.Errorf("tokens = %d/%d, want 5/3", input, output)
	}
}

func TestStreamingClientDisconnect(t *testing.T) {
	// 客户端提前断开 -> 上游取消
	// (简化:验证 context 取消传播——用可取消的 request)
	up := newStreamingUpstream(t)
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{"model": "my-gpt4", "messages": []map[string]string{{"role": "user", "content": "hi"}}, "stream": true})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	// 正常跑完即可(httptest 不模拟真实断开;验证不 panic)
	p.ServeHTTP(rec, req)
}
```

`internal/proxy/stats_test.go`:

```go
package proxy

import (
	"testing"

	"carryapi/internal/catalog"
)

func TestComputeCost(t *testing.T) {
	var cr float64 = 0.5
	var cw float64 = 1.0
	price := &catalog.Price{InputPrice: 5.0, OutputPrice: 15.0, CacheReadPrice: &cr, CacheWritePrice: &cw}
	rc := &requestContext{
		inputTokens: 1000, outputTokens: 2000, cacheRead: 500, cacheCreation: 100,
	}
	// 5*1000/1e6 + 15*2000/1e6 + 0.5*500/1e6 + 1.0*100/1e6
	want := 5.0*1000/1e6 + 15.0*2000/1e6 + 0.5*500/1e6 + 1.0*100/1e6
	got := computeCost(price, rc)
	if got != want {
		t.Errorf("cost = %f, want %f", got, want)
	}
}

func TestComputeCostNoCachePrices(t *testing.T) {
	price := &catalog.Price{InputPrice: 2.0, OutputPrice: 8.0}
	rc := &requestContext{inputTokens: 1000, outputTokens: 500, cacheRead: 100, cacheCreation: 50}
	// 无 cache 价格时:cacheRead 用 input_price,cacheCreation 用 input_price
	want := 2.0*1000/1e6 + 8.0*500/1e6 + 2.0*(100+50)/1e6
	got := computeCost(price, rc)
	if got != want {
		t.Errorf("cost = %f, want %f", got, want)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/proxy/ -v -run "Streaming|ComputeCost"
```

预期:编译失败(streamResponse / computeCost 未定义)。

- [ ] **Step 3: 实现 stream.go**

```go
package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net/http"

	"carryapi/internal/ir"
)

// streamResponse 流式转发:上游 SSE -> 统一事件 -> 下游 SSE。
func (p *Proxy) streamResponse(w http.ResponseWriter, rc *requestContext, upResp *http.Response, downstream string) {
	if upResp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(upResp.Body, 4096))
		p.writeError(w, rc, ir.NewError("upstream", "upstream_error", fmt.Sprintf("upstream returned %d: %s", upResp.StatusCode, truncate(body, 200)), 502))
		return
	}
	// 上游 Decoder(按 provider.Protocol)
	decoder, err := newUpstreamStreamDecoder(rc.provider.Protocol)
	if err != nil {
		p.writeError(w, rc, ir.NewError("internal", "stream_decode_failed", err.Error(), 500))
		return
	}
	// 下游 Encoder(按 downstream)
	encoder, err := newDownstreamStreamEncoder(downstream)
	if err != nil {
		p.writeError(w, rc, ir.NewError("internal", "stream_encode_failed", err.Error(), 500))
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(200)
	rc.statusCode = 200

	flusher, _ := w.(http.Flusher)
	scanner := bufio.NewScanner(upResp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	scanner.Split(splitSSERecords) // 按空行分割(SSE 事件)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		// 处理单条 data 行(每个事件块可能有多行 data,简化:逐行喂 decoder)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(line[len("data:"):])
		if len(payload) == 0 {
			continue
		}
		events, err := decoder.DecodeLine(payload)
		if err != nil {
			// 忽略解析失败的行,继续(流式容错)
			continue
		}
		for _, ev := range events {
			p.collectEvent(rc, ev)
			outLines, err := encoder.Encode(ev)
			if err != nil {
				continue
			}
			for _, ol := range outLines {
				w.Write(ol)
			}
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
	p.recordStats(rc)
}

// collectEvent 从统一事件提取用量(统计用)。
func (p *Proxy) collectEvent(rc *requestContext, ev ir.Event) {
	if ev.Usage == nil {
		return
	}
	rc.inputTokens = ev.Usage.InputTokens
	rc.outputTokens = ev.Usage.OutputTokens
	rc.cacheRead = ev.Usage.CacheReadTokens
	rc.cacheCreation = ev.Usage.CacheCreationTokens
}

func newUpstreamStreamDecoder(protocol string) (interface{ DecodeLine([]byte) ([]ir.Event, error) }, error) {
	switch protocol {
	case "openai_chat":
		return &ir.ChatStreamDecoder{}, nil
	case "openai_responses":
		return &ir.ResponsesStreamDecoder{}, nil
	case "anthropic":
		return &ir.AnthropicStreamDecoder{}, nil
	}
	return nil, fmt.Errorf("unknown upstream protocol %q", protocol)
}

func newDownstreamStreamEncoder(downstream string) (interface {
	Encode(ir.Event) ([][]byte, error)
	Reset()
}, error) {
	switch downstream {
	case "chat":
		return &ir.ChatStreamEncoder{}, nil
	case "responses":
		return &ir.ResponsesStreamEncoder{}, nil
	case "anthropic":
		return &ir.AnthropicStreamEncoder{}, nil
	}
	return nil, fmt.Errorf("unknown downstream protocol %q", downstream)
}

// splitSSERecords 是 bufio.SplitFunc:按 "\n\n" 分割 SSE 事件。
func splitSSERecords(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for i := 0; i+1 < len(data); i++ {
		if data[i] == '\n' && data[i+1] == '\n' {
			return i + 2, data[:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
```

> 注:`newUpstreamStreamDecoder`/`newDownstreamStreamEncoder` 返回匿名接口。实现者可用更清晰的命名接口(如 `type streamDecoder interface { DecodeLine([]byte) ([]ir.Event, error) }`)。`splitSSERecords` 处理 `\n\n`,CRLF 由 TrimSpace 兜底。

- [ ] **Step 4: 实现 stats.go**

```go
package proxy

import (
	"time"

	"carryapi/internal/catalog"
)

// recordStats 写 request_logs + 累加配额。
func (p *Proxy) recordStats(rc *requestContext) {
	if rc.user == nil {
		return // 鉴权前失败,不记日志(或记?简化:不记)
	}
	cost := computeCost(rc.price, rc)
	upstreamModel := ""
	if rc.model != nil {
		upstreamModel = rc.model.UpstreamModel
	}
	providerID := int64(0)
	if rc.provider != nil {
		providerID = rc.provider.ID
	}
	modelName := ""
	if rc.model != nil {
		modelName = rc.model.Name
	}
	var durationMs int64
	if !rc.start.IsZero() {
		durationMs = time.Since(rc.start).Milliseconds()
	}
	_, _ = p.deps.DB.Exec(
		`INSERT INTO request_logs(request_id, user_id, api_key_id, custom_model, provider_id, upstream_model,
		 protocol_in, protocol_out, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
		 cost, duration_ms, status_code, error_type, error_message, stream)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rc.requestID, rc.user.ID, rc.apiKeyID, modelName, providerID, upstreamModel,
		rc.downstream, protocolOutName(rc.provider), rc.inputTokens, rc.outputTokens,
		rc.cacheRead, rc.cacheCreation, cost, durationMs, rc.statusCode, rc.errorType, rc.errorMessage, rc.stream)

	// 配额累加
	if rc.statusCode == 200 {
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

// computeCost 每百万 token 计价。
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
```

> 注:`rc.stream` 字段需在 requestContext 加(bool)。`duration_ms` 暂填 0(可后续用中间件计时)。requestContext 还需 `stream bool`(handleProxy 里设置)。

- [ ] **Step 5: 运行测试确认通过**

```bash
go test ./internal/proxy/ -v
```

预期:全部 PASS(含 Task 5 的非流式 + 本任务流式 + 成本)。

- [ ] **Step 6: 提交**

```bash
git add internal/proxy/stream.go internal/proxy/stats.go internal/proxy/stream_test.go internal/proxy/stats_test.go internal/proxy/forward.go internal/proxy/proxy.go
git commit -m "feat(proxy): streaming SSE forwarding and usage statistics"
```

---

### Task 7: /v1/models 端点 + 路由挂载 + 集成测试

**Files:**
- Create: `internal/proxy/models.go`
- Test: `internal/proxy/models_test.go`
- Modify: `internal/server/router.go`(挂载 /v1/* 到 proxy)
- Modify: `internal/server/server.go`(Deps 加 Proxy)
- Modify: `cmd/carryapi/main.go`(wire proxy)
- Test: `internal/proxy/integration_test.go`(端到端)

**Interfaces:**
- Produces:

```go
// handleModels 返回启用的模型列表(OpenAI 格式)。
// GET /v1/models -> {"object":"list","data":[{"id":"my-gpt4","object":"model","created":...,"owned_by":"carryapi"}]}
func (p *Proxy) handleModels(w http.ResponseWriter, r *http.Request)
// 需要 API Key 鉴权(和代理端点一致)。
```

- [ ] **Step 1: 写失败测试**

`internal/proxy/models_test.go`:

```go
package proxy

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHandleModels(t *testing.T) {
	p, u := newModelFixture(t) // 有 my-gpt4(启用)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	req := httptest.NewRequest("GET", "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Object string `json:"object"`
		Data   []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Object != "list" || len(resp.Data) != 1 || resp.Data[0].ID != "my-gpt4" {
		t.Errorf("models = %+v", resp)
	}
}

func TestHandleModelsAuthRequired(t *testing.T) {
	p, _ := newModelFixture(t)
	req := httptest.NewRequest("GET", "/v1/models", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("code=%d, want 401", rec.Code)
	}
}
```

`internal/proxy/integration_test.go`(端到端:跨协议流式):

```go
package proxy

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// 端到端:客户端 Responses 协议 <-> 上游 Chat 协议(非流式)
func TestEndToEndResponsesToChat(t *testing.T) {
	up := newUpstreamServer(t) // Chat 上游
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	// 但 provider protocol 是 openai_chat,客户端用 Responses
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{
		"model":        "my-gpt4",
		"instructions": "Be brief.",
		"input":        "hi",
	})
	req := httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["object"] != "response" {
		t.Errorf("object = %v", resp["object"])
	}
	output := resp["output"].([]any)
	if len(output) == 0 {
		t.Fatal("empty output")
	}
}
```

> 端到端测试在 Task 7 加分项;若上游 mock 的 Chat 响应固定,Responses 转换应产生 output 数组。实现者验证。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/proxy/ -v -run "Models|EndToEnd"
```

预期:编译失败(handleModels 未定义)。

- [ ] **Step 3: 实现 models.go**

```go
package proxy

import (
	"net/http"

	"carryapi/internal/ir"
)

// handleModels 返回启用的模型列表(OpenAI 格式),需 API Key 鉴权。
func (p *Proxy) handleModels(w http.ResponseWriter, r *http.Request) {
	if _, _, err := p.authenticate(r); err != nil {
		rc := &requestContext{downstream: "chat", requestID: ""}
		p.writeError(w, rc, err)
		return
	}
	models, err := p.deps.Models.ListEnabled()
	if err != nil {
		rc := &requestContext{downstream: "chat", requestID: ""}
		p.writeError(w, rc, ir.NewError("internal", "list_failed", "failed to list models", 500))
		return
	}
	data := make([]map[string]any, 0, len(models))
	for _, m := range models {
		data = append(data, map[string]any{
			"id": m.Name, "object": "model", "created": m.CreatedAt.Unix(), "owned_by": "carryapi",
		})
	}
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, 200, map[string]any{"object": "list", "data": data})
}
```

> 注:proxy 包需要本地 `writeJSON` helper(catalog 包的 `jsonOut` 不可见):
> ```go
> func writeJSON(w http.ResponseWriter, status int, data any) {
> 	w.Header().Set("Content-Type", "application/json")
> 	w.WriteHeader(status)
> 	json.NewEncoder(w).Encode(data)
> }
> ```
> 实现者把 `writeJSON` 放在 `internal/proxy/proxy.go` 顶部,import `encoding/json`。

- [ ] **Step 4: 路由挂载 + main.go 接线**

`internal/server/router.go`:
```go
// 代理端点(在 SessionMiddleware 之外,用 API Key 鉴权)
if s.deps.Proxy != nil {
    r.Handle("/v1/*", s.deps.Proxy)
}
```
注意:chi 的 `r.Handle("/v1/*", ...)` 会把 `/v1/models`、`/v1/chat/completions` 等全交给 Proxy.ServeHTTP(内部按 Path 分派)。**确认 chi 的 `/*` 通配匹配多级路径**:chi 的 `/*` 匹配剩余路径,是的。

`Deps` 加 `Proxy *proxy.Proxy`;main.go:
```go
proxyInstance := proxy.NewProxy(proxy.Deps{
    DB: d, Keys: keyStore, Users: userStore,
    Models: modelStore, Providers: providerStore, Prices: priceStore,
})
deps.Proxy = proxyInstance
```

- [ ] **Step 5: 运行全部测试 + 冒烟**

```bash
go test ./... -count=1
```
预期:全部 PASS(子项目 1-4 全绿)。

冒烟(手动):
```bash
go build -o carryapi.exe ./cmd/carryapi
./carryapi.exe &
# 用 admin 建 provider + model + price(需要 session cookie + CSRF,复杂)
# 简化:直接用 sqlite 手动插?不——冒烟走 admin API:
# 1. 登录 admin(cookie jar)
# 2. 建 provider(model)
# 3. 建 custom model
# 4. 建 price
# 5. 用 API Key 调 /v1/chat/completions
kill %1
```
> 冒烟用 curl 带 cookie jar + CSRF token。实现者在报告中给出完整命令序列。

- [ ] **Step 6: 提交**

```bash
git add internal/proxy/models.go internal/proxy/models_test.go internal/proxy/integration_test.go internal/server/router.go internal/server/server.go cmd/carryapi/main.go
git commit -m "feat(proxy): models list endpoint, route mounting, end-to-end wiring"
```

---

### Task 8: 统计回归测试 + README

**Files:**
- Modify: `internal/proxy/stats_test.go`(错误日志 + 耗时断言)
- Modify: `README.md`

**Interfaces:**
- `recordStats` 已在 Task 6 实现完整版(含 duration_ms + error_message;`start`/`errorMessage` 字段在 Task 4/5 定义)。本任务补测试验证这些字段 + README 代理端点章节。

- [ ] **Step 1: 写失败测试**

在 stats_test.go 加:
```go
func TestRecordStatsWritesError(t *testing.T) {
	// 无鉴权请求 -> 401 -> request_logs 应有一条 error_type=authentication 的记录
	up := newUpstreamServer(t)
	defer up.Close()
	p, _ := newProxyWithUpstream(t, up.URL)
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	var count int
	p.deps.DB.QueryRow("SELECT COUNT(*) FROM request_logs").Scan(&count)
	if count != 1 {
		t.Fatalf("request_logs = %d, want 1 (auth failure logged)", count)
	}
	var errType, errMsg string
	p.deps.DB.QueryRow("SELECT error_type, error_message FROM request_logs").Scan(&errType, &errMsg)
	if errType != "authentication" {
		t.Errorf("error_type = %q, want authentication", errType)
	}
	if errMsg == "" {
		t.Error("error_message should not be empty")
	}
}

func TestRecordStatsDuration(t *testing.T) {
	// 非流式成功请求 -> duration_ms >= 0, stream=false
	up := newUpstreamServer(t)
	defer up.Close()
	p, u := newProxyWithUpstream(t, up.URL)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	body, _ := json.Marshal(map[string]any{"model": "my-gpt4", "messages": []map[string]string{{"role": "user", "content": "hi"}}})
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+plaintext)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	var durationMs int64
	var stream bool
	p.deps.DB.QueryRow("SELECT duration_ms, stream FROM request_logs").Scan(&durationMs, &stream)
	if durationMs < 0 {
		t.Errorf("duration_ms = %d, want >= 0", durationMs)
	}
	if stream {
		t.Error("stream should be false for non-streaming request")
	}
}
```

> 测试需要 import:`bytes`、`encoding/json`、`net/http/httptest`、`testing`。

- [ ] **Step 2: 更新 README**

加"代理端点"章节(在"认证"后):
- 四个端点路径 + 协议对应
- 鉴权:`Authorization: Bearer <api-key>` 或 `x-api-key: <api-key>`
- 调用示例:
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer carry-xxxx..." \
  -H "Content-Type: application/json" \
  -d '{"model":"my-gpt4","messages":[{"role":"user","content":"hi"}]}'
```
- 模型列表:`curl -H "Authorization: Bearer ..." http://localhost:8080/v1/models`

- [ ] **Step 3: 运行测试 + 提交**

```bash
go test ./... -count=1
git add internal/proxy/ internal/server/ cmd/carryapi/main.go README.md
git commit -m "feat(proxy): request duration and error logging, readme proxy section"
```

---

## 子项目 4 完成标准

- [ ] `go test ./...` 全绿(新增 catalog ~10 + proxy ~15 测试,全部包合计 180+)
- [ ] admin 可配置上游供应商/自定义模型/定价(CRUD)
- [ ] 四个代理端点可用,Chat/Responses/Anthropic 下游协议 + 三种上游协议任意组合转换
- [ ] API Key 鉴权(缺失/无效/禁用用户)
- [ ] 模型解析(未找到/禁用 -> 404)+ 配额预检(超限 -> 429)
- [ ] 流式透传(SSE 管道)+ 非流式,用量统计正确(含 cache_creation/cache_read)
- [ ] request_logs 写入(含 error_type、cost、duration_ms)+ 配额累加
- [ ] `/v1/models` 返回启用模型列表(需鉴权)
- [ ] 交叉编译仍无 CGO
