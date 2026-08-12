# 设计：上游模型自动获取 / 供应商可用性测试 / 仪表板 API base_url

日期：2026-08-12
状态：已确认（brainstorming 用户批准）

## 背景与目标

carryAPI 是一个自托管 API 聚合网关。管理员通过 `catalog` 包手动配置上游供应商（provider，含 BaseURL/APIKey/协议）与模型（custom_models，绑定某供应商的上游模型名，须配价格才能被代理使用）。三个新需求：

1. **自动获取上游供应商的模型列表**，选择式导入为草稿（禁用态），再手动配价并启用。
2. **测试供应商可用性/延迟**。
3. **仪表板首页显示网关自身的 API base_url**（供客户端作为 OpenAI-compatible 接入地址）。

## 范围

- 新增 `internal/catalog/probe.go`：上游探针客户端（拉取模型列表 + Ping 连通/延迟）。
- 扩展 `catalog.Handler`：新增 4 个 admin API（fetch 模型、批量导入、测供应商、网关信息）。
- 扩展 `internal/server`：路由注册；网关信息接口需要 config.Port 与请求 Host。
- 前端（`web/src/views/ModelsView.vue`、`DashboardView.vue`）：模型导入交互、供应商测试按钮、base_url 展示。

## 1. 上游探针客户端（internal/catalog/probe.go）

```go
type Prober struct { client *http.Client }

func NewProber(client *http.Client) *Prober
// FetchModels 调用上游模型列表接口，返回模型名列表
func (p *Prober) FetchModels(provider Provider) ([]string, error)
// Ping 测连通性+延迟，返回耗时（从请求发起到响应头返回）
func (p *Prober) Ping(provider Provider) (time.Duration, error)
```

协议差异（与 proxy 一致）：
- OpenAI（`openai_chat`/`openai_responses`）→ `GET {base_url}/models`，`Authorization: Bearer <key>`
- Anthropic → `GET {base_url}/v1/models`，`x-api-key: <key>` + `anthropic-version: 2023-06-01`

要点：
- **探针统一用 `GET /models`**（而非最小 chat 请求）：更轻量、无 token 消耗、直接反映连通性，且与「自动获取模型」是同一端点，一举两得。测模型实际可调用性留待以后。
- 响应解析尽力而为：OpenAI 格式 `{"data":[{"id":...}]}`；Anthropic 格式 `{"data":[{"id":...}]}`。解析失败时按行/未知返回原始 ID 数组为空，不阻断。
- 非 2xx 视为不可用，返回带错误消息的错误。
- 超时：统一 10s（`NewProber` 内若 client 未设 Timeout 则覆盖为 10s）。延迟 = `time.Since(start)`。
- 请求用 `context.Background()` 派生（handler 层传入超时控制）。

## 2. 自动获取上游模型（选择式导入为草稿）

新增两个 admin API（`catalog.Handler`）：

### POST /api/providers/{id}/models/fetch
- 调用 `Prober.FetchModels` 实时拉取该供应商模型名列表。
- 响应：`{ models: [{name, exists: bool}] }`，`exists` 标记是否已存在于 `custom_models`（避免重复导入）。
- 不落库。

### POST /api/models/import
- 请求体：`{ items: [{ provider_id, upstream_model }] }`
- 逐个创建 `custom_models` 记录：`name = upstream_model`（上游模型名），`enabled = 0`（**禁用态草稿**）。
- **同名跳过**：若 `custom_models.name` 已存在同名，跳过该条并标记 `skipped=true`，不覆盖手动配置。
- 响应：`{ imported: n, skipped: m, skipped_names: [...] }`。
- 导入后仍需管理员手动配价格并启用（resolveModel 会拒绝未配价/禁用模型）。

前端（ModelsView「模型」标签页）：
- 顶部加「从供应商导入」按钮 → 弹窗选择供应商 → 点击「获取模型」调 fetch API → 表格展示（含已存在标记，禁勾选）→ 勾选要导入的 → 「导入」调 import API → 刷新模型列表。

## 3. 测供应商可用性/延迟

新增 admin API（`catalog.Handler`）：

### POST /api/providers/{id}/test
- 调用 `Prober.Ping`。
- 响应：`{ ok: bool, latency_ms: number, error?: string }`。
- 非 2xx / 超时 / 网络错误 → `ok=false`，携带错误消息。

前端（ModelsView「供应商」表格）：
- 每行加「测试」按钮 → 调 test API → 用状态徽标（可用/不可用）+ 延迟显示（如 `123ms`），可重复点击刷新。

## 4. 仪表板显示 API base_url

新增 admin API：`GET /api/gateway/info`

- 后端推导接入地址，返回 `{ base_url: "http://<host>:<port>/v1" }`。
- **host**：监听 host 为 `0.0.0.0` 时用请求的 `Host` 头（浏览器当前访问地址）；否则用 `127.0.0.1`。
- **port**：从 `config.Port` 取（监听端口），而非运行时 actualAddr（避免测试态 `:0`）。
- **scheme**：默认 `http`。

实现方式：`catalog.Handler` 需要访问 config.Port 与 settings（listen_host）。为避免 handler 依赖膨胀，将网关信息接口放在 `server` 包（它已持有 config 与 settings），或在 catalog.Handler 注入一个 `GatewayInfo` 提供者。倾向在 `server` 包新增一个轻量 handler（`/api/gateway/info`），因为它已持有 config + settings + 请求上下文，符合现有结构。

前端（DashboardView 顶部）：
- 新增一张卡片/区块显示 `base_url`，用 `n-copy`（naive-ui）支持点击复制。

## 错误处理

- fetch/test 网络错误 → 返回 `{error: ...}`，前端 toast 显示。
- import 部分成功 → 返回 imported/skipped 统计，前端提示「导入 N 个，跳过 M 个（重名）」。
- 全部接口均需 admin 角色（复用现有 `RequireRole("admin")` 分组）。

## 测试

- `internal/catalog/probe_test.go`：用 `httptest.Server` 模拟上游，覆盖 OpenAI/Anthropic 的模型列表解析、Ping 延迟、非 2xx、超时。
- `internal/catalog/handler_test.go` 扩展：fetch、import（含同名跳过）、test 三个 handler 的行为。
- `server` 层：`/api/gateway/info` 在 `0.0.0.0` 与 `127.0.0.1` 两种监听下的 host 推导。
- 前端：构建通过（npm run build）。

## 文档同步

按项目约定（见 keep-manual-in-sync-on-feature-changes），新增功能需同步更新 `MANUAL.md` 与 `README.md`：
- 记录「从供应商导入模型」「供应商测试」用法。
- 记录仪表板 base_url 含义。

## 非目标（YAGNI）

- 不做模型实际可调用性测试（仅供应商层面连通/延迟）。
- 不做模型列表定时自动同步。
- 不自动为导入模型配价格。
