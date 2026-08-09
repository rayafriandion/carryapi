# carryAPI 设计文档

- **日期**:2026-08-10
- **状态**:已通过设计评审,待实现
- **项目路径**:`D:\Projects\carryAPI`

## 1. 概述

carryAPI 是一个自托管的 API 聚合路由服务。它把多个上游模型供应商(OpenAI 兼容、Anthropic 等)聚合,对外统一暴露自定义模型名,提供可视化配置界面,支持广播开关、模型映射、定价设置、用量与费用统计、成功率监控,以及完整的多用户认证体系。

### 核心目标

- 单二进制部署,Win/Linux x64 各一份,无需运行时依赖。
- 三种协议(OpenAI Chat Completions、OpenAI Responses、Anthropic Messages)在上下游间互转。
- 可视化管理后台(Vue 3 + Naive UI,构建后嵌入二进制)。
- 多用户 + 完整认证(密码 + 2FA + Passkey + OAuth)。

### 非目标

- 不内置公网穿透(frp/cloudflared 等),公网暴露由用户自行用反代或穿透工具完成。
- 不做权重负载均衡(采用固定映射,一个自定义模型 -> 一个上游+模型)。

## 2. 技术栈

| 层 | 技术 | 说明 |
|----|------|------|
| 后端 | Go | 单二进制,`//go:embed` 嵌入前端 |
| 前端 | Vue 3 + Naive UI + Vite + Pinia | 构建产物嵌入二进制 |
| 数据库 | SQLite | `modernc.org/sqlite`,纯 Go 无 CGO,跨平台编译 |
| WebAuthn | `go-webauthn/webauthn` | Passkey 支持 |

## 3. 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                      客户端 (ChatUI / Cursor / SDK)          │
└───────────────┬────────────────────────────────────────────┘
                │ OpenAI Chat / Responses / Anthropic 三种端点
                ▼
┌─────────────────────────────────────────────────────────────┐
│  carryAPI 单二进制 (Go + embed 前端)                          │
│                                                              │
│  ┌────────────┐   ┌──────────────┐   ┌──────────────────┐  │
│  │ HTTP 层    │──▶│ 路由/鉴权层   │──▶│ 协议适配层 (IR)  │  │
│  │ (端点暴露) │   │ (Key+用户+配额)│   │ (3解码+3编码)    │  │
│  └────────────┘   └──────────────┘   └────────┬─────────┘  │
│                                                │             │
│                                ┌───────────────▼──────────┐ │
│                                │ 上游代理层 (流式透传)      │ │
│                                │ OpenAI/Anthropic 上游      │ │
│                                └───────────────┬──────────┘ │
│                                                │             │
│  ┌─────────────────────────────────────────────▼──────────┐ │
│  │ 统计层:token 解析 + 费用计算 + 日志写入 (异步)         │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌────────────┐   ┌──────────────┐   ┌──────────────────┐  │
│  │ 配置模块   │   │ 认证模块      │   │ 前端静态资源     │  │
│  │ (广播/模型)│   │ (本地/OAuth/  │   │ (Vue3+Naive UI)  │  │
│  │            │   │  Passkey/2FA) │   │                  │  │
│  └────────────┘   └──────────────┘   └──────────────────┘  │
│                                                              │
│  ┌───────────────────────────────────────────────────────┐  │
│  │ SQLite (modernc.org/sqlite, 纯 Go 无 CGO)              │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

### 请求主流程

客户端发起请求 -> HTTP 层按路径分流到三种端点 -> 鉴权层验证 API Key/用户/配额 -> 协议适配层把请求解码成 IR -> 按模型映射找到上游 -> 把 IR 编码成上游格式 -> 上游代理层转发(流式透传)-> 统计层从响应中解析 token/费用,异步写库 -> 响应编码成客户端期望格式返回。

### 关键设计点

- **单二进制**:Go 编译,前端 `dist/` 通过 `//go:embed` 嵌入,运行时只依赖一个 SQLite 数据库文件。
- **广播开关**:全局配置 `listen_host`,`0.0.0.0`(广播开)或 `127.0.0.1`(广播关),前端一键切换。
- **统计异步**:token 解析在流结束/响应完成时同步提取,写库走异步队列,不阻塞响应。

## 4. 协议适配层(IR 枢纽)

采用 IR(中间表示)枢纽模式,所有协议转换走 `上游格式 <-> IR <-> 下游格式`。转换矩阵从 9 个降到 6 个(3 解码 + 3 编码),新增协议只需加 2 个转换器。

### IR 设计

```go
// InternalRequest:覆盖 Chat/Responses/Anthropic 请求的所有字段
type InternalRequest struct {
    Model       string
    Messages    []InternalMessage   // 统一消息序列(role/content)
    System      *string             // Anthropic 单独的 system 段
    Tools       []InternalTool      // 工具定义
    ToolChoice  *InternalToolChoice
    Stream      bool
    Temperature *float64
    MaxTokens   *int
    // ... 其他采样参数
    Cache       *InternalCacheHint  // prompt cache 控制点
}

type InternalMessage struct {
    Role       string                // system/user/assistant/tool
    Content    []InternalContentPart // 多模态:text/image/audio/tool_call/tool_result
}

// InternalResponse:非流式完整响应
type InternalResponse struct {
    ID      string
    Model   string
    Choices []InternalChoice         // 统一 choices(含 message + finish_reason)
    Usage   InternalUsage            // input/output/cache token
}

// InternalUsage:三协议用量字段的并集
type InternalUsage struct {
    InputTokens          int
    OutputTokens         int
    CacheReadTokens      int   // OpenAI: prompt_tokens_details.cached_tokens
    CacheCreationTokens  int   // Anthropic 专有:cache_creation_input_tokens
}
```

### 转换器矩阵

```
              解码(下游请求格式 -> IR)         编码(IR -> 上游请求格式)
Chat Compl.   ChatDecoder     ──┐          ┌──   ChatEncoder
Responses     ResponsesDecoder──┼──▶ IR ──┼──   ResponsesEncoder
Anthropic     AnthropicDecoder ─┘          └──   AnthropicEncoder

实际方向:
  下游客户端请求 ──[解码]──▶ IR ──[编码]──▶ 上游格式
  上游响应       ──[解码]──▶ IR ──[编码]──▶ 下游格式
```

6 个转换器,每个是纯函数,易于单元测试。新增协议只加 2 个。

### 流式处理:统一事件流

IR 层把三种协议的流归一成 `[]InternalEvent`,再编码成目标格式:

```go
type InternalEventType int
const (
    EventContentDelta InternalEventType = iota  // 文本/多模态增量
    EventToolCallDelta                          // 工具调用增量
    EventUsage                                  // 用量(末尾)
    EventDone                                   // 结束
)

type InternalEvent struct {
    Type    InternalEventType
    Delta   string           // 文本增量
    ToolCall *InternalToolCall
    Usage   *InternalUsage   // 末尾事件携带
    Finish  string           // stop/length/tool_calls
}
```

**流式用量提取**:
- **OpenAI Chat**:末尾 chunk 的 `usage`(需上游开启 `stream_options.include_usage`)。
- **OpenAI Responses**:末尾 `response.completed` 事件的 `usage`。
- **Anthropic**:末尾 `message_delta` 事件的 `usage` + `message_start` 的 `input_tokens`。

转换器内部各自把上游流 chunk 翻译成 `InternalEvent`,统计层只消费统一的 `EventUsage`。

### 协议特有字段处理原则

- **能映射的映射**:Anthropic `system` 段 <-> Chat 的 `system` role message。
- **无法映射的降级保留**:Anthropic 的 `cache_creation` token 在转 Chat 时仍记录到 `InternalUsage`(用于统计),但不输出到 Chat 响应里。
- **工具调用统一**:三种协议都有 tool/function 概念,映射到 `InternalToolCall`,跨协议转换时做格式适配。

## 5. 数据模型与存储

SQLite,纯 Go 驱动。所有表:

### 用户与认证

```sql
-- 用户表
users (
    id              INTEGER PRIMARY KEY,
    email           TEXT UNIQUE NOT NULL,
    password_hash   TEXT,              -- 可空(OAuth-only 用户无密码)
    role            TEXT NOT NULL,     -- admin / user
    status          TEXT NOT NULL,     -- active / disabled
    created_at      TIMESTAMP
)

-- 用户绑定的登录方式(一个用户多种登录)
auth_methods (
    id              INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL,
    provider        TEXT NOT NULL,     -- password / totp / passkey / discord / x
    provider_uid    TEXT,              -- OAuth provider用户ID / passkey credentialID
    secret          TEXT,              -- TOTP密钥/passkey公钥(加密存储)
    created_at      TIMESTAMP
)
```

### API Key 与配额

```sql
-- API Key(用户可创建多个,带标签)
api_keys (
    id              INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL,
    key_hash        TEXT NOT NULL,     -- 存哈希,不存明文
    key_prefix      TEXT NOT NULL,     -- 显示用前缀(如 carry-xxxx)
    label           TEXT,              -- 标签:"家用"/"公司"
    status          TEXT NOT NULL,     -- active / disabled
    expires_at      TIMESTAMP,         -- 可空
    created_at      TIMESTAMP
)

-- 配额(按用户或按 Key)
quotas (
    id              INTEGER PRIMARY KEY,
    scope           TEXT NOT NULL,     -- user / key
    scope_id        INTEGER NOT NULL,  -- user_id 或 key_id
    period          TEXT NOT NULL,     -- day / month / total
    limit_tokens    INTEGER,           -- token 上限(可空=不限)
    limit_cost      REAL,              -- 费用上限(可空=不限)
    used_tokens     INTEGER DEFAULT 0,
    used_cost       REAL DEFAULT 0,
    period_start    TIMESTAMP          -- 周期起始(用于重置)
)
```

### 上游与模型映射

```sql
-- 上游供应商配置
upstream_providers (
    id              INTEGER PRIMARY KEY,
    name            TEXT NOT NULL,     -- "OpenAI官方"/"DeepSeek"
    base_url        TEXT NOT NULL,
    api_key         TEXT NOT NULL,     -- 加密存储
    protocol        TEXT NOT NULL,     -- openai_chat / openai_responses / anthropic
    status          TEXT NOT NULL,     -- active / disabled
    created_at      TIMESTAMP
)

-- 自定义模型映射(固定映射:一个自定义模型 -> 一个上游+模型)
custom_models (
    id              INTEGER PRIMARY KEY,
    name            TEXT UNIQUE NOT NULL,      -- 对外暴露的名字:my-gpt4
    provider_id     INTEGER NOT NULL,
    upstream_model  TEXT NOT NULL,             -- 上游真实模型名:gpt-4o
    enabled         BOOLEAN DEFAULT 1,
    created_at      TIMESTAMP
)

-- 模型定价(每自定义模型一套价格)
model_prices (
    id              INTEGER PRIMARY KEY,
    model_id        INTEGER NOT NULL,
    input_price     REAL NOT NULL,     -- 每百万 token 价格
    output_price    REAL NOT NULL,
    cache_read_price REAL,              -- 可空(部分模型无缓存计费)
    cache_write_price REAL,
    currency        TEXT DEFAULT 'USD',
    effective_from  TIMESTAMP          -- 支持价格历史
)
```

### 使用量统计与日志

```sql
-- 请求明细日志(每次请求一条)
request_logs (
    id              INTEGER PRIMARY KEY,
    request_id      TEXT NOT NULL,     -- UUID,响应头返回
    user_id         INTEGER NOT NULL,
    api_key_id      INTEGER NOT NULL,
    custom_model    TEXT NOT NULL,
    provider_id     INTEGER,
    upstream_model  TEXT,
    protocol_in     TEXT NOT NULL,     -- 客户端用的协议
    protocol_out    TEXT NOT NULL,     -- 转发上游用的协议
    input_tokens    INTEGER DEFAULT 0,
    output_tokens   INTEGER DEFAULT 0,
    cache_read_tokens   INTEGER DEFAULT 0,
    cache_creation_tokens INTEGER DEFAULT 0,
    cost            REAL DEFAULT 0,    -- 快照当时价格
    duration_ms     INTEGER,
    status_code     INTEGER,
    error_type      TEXT DEFAULT 'none', -- none/upstream_error/timeout/auth/quota/parse/other
    error_message   TEXT,
    stream          BOOLEAN,
    created_at      TIMESTAMP NOT NULL
)
```

### 全局配置

```sql
-- 全局配置(键值表)
settings (
    key     TEXT PRIMARY KEY,
    value   TEXT NOT NULL              -- JSON 字符串
)
```

关键配置项:`listen_host`、`registration_open`、`force_2fa`、`log_retention_days`、`oauth_discord_client_id/secret`、`oauth_x_client_id/secret`。

### 会话

```sql
sessions (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER NOT NULL,
    token_hash  TEXT NOT NULL,
    expires_at  TIMESTAMP,
    created_at  TIMESTAMP,
    ip          TEXT,
    user_agent  TEXT
)
```

### 设计要点

- **明文不落库**:API Key(用户的、上游的)存哈希或加密,用主密钥(AES-GCM)加密敏感字段。
- **配额检查**:请求前查 `quotas`,超限拒绝(429);完成后异步累加,事务 + 原子更新避免竞态。
- **日志保留**:后台定时任务按 `log_retention_days` 清理。
- **价格快照**:`request_logs.cost` 快照当时价格,调价不影响旧账单。
- **错误归类**:`error_type` 字段用于成功率下钻。

## 6. API 端点与请求生命周期

### 代理端点(三种协议)

| 路径 | 协议 | 方法 |
|------|------|------|
| `/v1/chat/completions` | OpenAI Chat | POST |
| `/v1/completions` | OpenAI Chat (旧) | POST |
| `/v1/responses` | OpenAI Responses | POST |
| `/v1/messages` | Anthropic Messages | POST |
| `/v1/models` | 列出可用自定义模型 | GET |

三种端点接受 `Authorization: Bearer <key>`(OpenAI 风格)或 `x-api-key: <key>`(Anthropic 风格)。响应格式按请求路径决定,与上游协议无关。

### 管理后台 API

```
POST   /api/auth/login            本地登录(密码+2FA)
POST   /api/auth/register         注册(开关控制)
POST   /api/auth/oauth/:provider  OAuth(Discord/X)发起
GET    /api/auth/oauth/callback   OAuth 回调
POST   /api/auth/passkey/register Passkey 注册
POST   /api/auth/passkey/login    Passkey 登录
POST   /api/auth/2fa/setup        开启 TOTP(返回二维码)
POST   /api/auth/2fa/verify       验证 TOTP
POST   /api/auth/2fa/disable      关闭 2FA
POST   /api/auth/logout

GET    /api/settings              读全局配置
PUT    /api/settings              改全局配置(admin)

GET    /api/providers             上游列表(admin)
POST   /api/providers             增(admin)
PUT    /api/providers/:id         改(admin)
DELETE /api/providers/:id         删(admin)

GET    /api/models                自定义模型列表(admin)
POST   /api/models                增(admin,含映射+定价)
PUT    /api/models/:id            改(admin)
DELETE /api/models/:id            删(admin)

GET    /api/keys                  当前用户 Key 列表
POST   /api/keys                  增(返回明文一次)
DELETE /api/keys/:id              删
PUT    /api/keys/:id              改标签/状态

GET    /api/quotas                配额列表(admin 全部,用户自己)
PUT    /api/quotas/:id            改配额(admin)

GET    /api/users                 用户列表(admin)
POST   /api/users                 创建用户(admin)
PUT    /api/users/:id             改状态/角色(admin)
DELETE /api/users/:id             删用户(admin)

GET    /api/stats/summary         汇总(总数/按模型/按上游/按Key)
GET    /api/stats/trend           时间趋势(按天/小时)
GET    /api/stats/cost            费用核算(按模型/Key/时间段)
GET    /api/stats/success-rate    模型成功率(支持 model/provider/key 维度+时间筛选)
GET    /api/logs                  请求日志(分页+筛选)
GET    /api/health                健康检查
```

### 请求生命周期(代理端点)

```
1. 接收请求
   ├─ 识别协议(按路径)和目标自定义模型(请求体 model 字段)
   └─ 提取 API Key

2. 鉴权
   ├─ 查 Key 哈希 -> 用户
   ├─ 校验 Key 状态/过期
   └─ 用户状态检查

3. 模型解析
   ├─ 查 custom_models:自定义名 -> 上游 provider + 真实模型名
   ├─ 上游 provider 状态/协议
   └─ 找不到模型 -> 404 model not found

4. 配额预检
   ├─ 查用户/Key 的 quotas
   ├─ 当前周期已用 >= 上限 -> 429 quota_exceeded
   └─ (token 上限无法预判输入,费用上限按预估或仅事后校验)

5. 协议转换
   ├─ 下游请求 --[Decoder]--> IR
   └─ IR --[Encoder(按上游protocol)]--> 上游请求

6. 上游转发(流式透传)
   ├─ 构造上游 HTTP 请求,注入上游 API Key
   ├─ stream=true:开 SSE 管道,边收边转边发
   └─ stream=false:等完整响应再转

7. 用量解析(统计层)
   ├─ 流式:消费 InternalEvent 末尾 Usage
   └─ 非流式:从响应 Usage 字段提取

8. 响应返回
   ├─ IR --[Encoder(按下游协议)]--> 下游格式
   └─ 写响应头 X-Request-Id 供排查

9. 异步收尾(不阻塞响应)
   ├─ 计算费用(price x token)
   ├─ 写 request_logs(含 error_type)
   ├─ 累加 quotas.used_tokens/used_cost
   └─ 周期满则重置 period_start
```

### 成功率统计

- **成功**:`status_code` 2xx 且无 `error_message`。
- **失败**:其余(4xx/5xx/超时/连接失败等)。
- 成功率 = 成功 / 总数 × 100%,按 `custom_model` 分组聚合。
- 前端表格:每行一个模型(总请求数/成功/失败/成功率/平均耗时),失败可按 `error_type` 下钻。

### 关键设计点

- **错误格式**:按下游协议返回。OpenAI 端点返 `{"error":{...}}`;Anthropic 端点返 `{"type":"error","error":{...}}`。统一错误 IR 再编码。
- **超时与取消**:上游请求带超时;客户端断开传播 context 取消上游请求。
- **可观测**:每个请求带 `X-Request-Id`(UUID),贯穿日志。
- **并发控制**:单个上游可设并发上限(令牌桶)。

## 7. 角色权限

| 功能 | 普通用户 | 管理员 |
|------|---------|--------|
| 仪表盘(自己的统计) | 查看 | 查看全部 |
| 统计分析(自己的) | 查看 | 全部用户 |
| 请求日志(自己的) | 查看 | 全部 |
| API Key 管理(自己的) | 增删改 | 自己的 |
| 上游供应商 | 不可见 | 增删改 |
| 自定义模型映射 | 不可见映射细节 | 增删改 |
| 模型定价 | 不可见 | 增删改 |
| 可用模型列表(`/v1/models`) | 只看模型名 | 同 |
| 配额管理 | 只看自己的 | 改全部 |
| 用户管理 | 不可见 | 全部 |
| 系统设置 | 不可见 | 全部 |

**边界**:普通用户只能看到可用的自定义模型名,看不到映射细节/价格/上游信息。前端隐藏菜单,后端做角色校验(调 admin 端点返回 403)。

## 8. 认证体系

### 登录方式与绑定

一个 `users` 账号可绑定多种登录方式,登录后是同一个账号:

| 登录方式 | 存储位置 |
|---------|---------|
| 本地密码 | `auth_methods`(provider=password),bcrypt 哈希 |
| TOTP 2FA | `auth_methods`(provider=totp),加密密钥 |
| Passkey | `auth_methods`(provider=passkey),WebAuthn 公钥 |
| Discord | `auth_methods`(provider=discord) |
| X(Twitter) | `auth_methods`(provider=x) |

**登录流程**:任一方式验证通过 -> 建 session -> 若 `force_2fa` 且用户未开 2FA -> 强制跳转 2FA 设置。OAuth/Passkey 登录时 provider_uid 未绑定:`registration_open` 开则创建新账号,关则要求先登录已有账号再绑定。

### 2FA 细节

**TOTP**:生成密钥 -> 返回 `otpauth://` 二维码 -> 用户扫码 -> 验证 6 位码(容忍前后各一个时间窗)-> 提供一次性备份码。

**Passkey(WebAuthn)**:注册调 `navigator.credentials.create()`,后端用 `go-webauthn/webauthn` 校验 attestation 存公钥 + credentialID;登录调 `navigator.credentials.get()` 校验 assertion。多设备支持:一个账号多个 Passkey。

### OAuth 流程

```
1. GET /api/auth/oauth/discord -> 重定向到 Discord 授权页(state 防 CSRF)
2. 用户授权 -> 回调 /api/auth/oauth/callback?code=...&state=...
3. 后端用 code 换 access_token,调 /api/users/@me 拿用户ID
4. 查 auth_methods:
   ├─ 已绑定 -> 建 session
   └─ 未绑定:
      ├─ 注册开 -> 创建用户+绑定+建 session
      └─ 注册关 -> 提示"先登录后绑定"
```

X(Twitter)用 OAuth 2.0 PKCE。OAuth 配置存 `settings`。

### Session 管理

服务端 session(存 SQLite),session ID 放 HttpOnly cookie。比 JWT 易吊销(改密/关 2FA/禁用账号立即失效)。登录表见第 5 节。

### 安全要点

- **密码**:bcrypt,cost=12。
- **敏感字段加密**:上游 API Key、TOTP 密钥、Passkey 公钥用 AES-GCM,主密钥从 `CARRYAPI_MASTER_KEY` 环境变量或配置文件读,缺失则生成并提示。
- **CSRF**:session cookie + CSRF token(双提交 cookie 或 SameSite=Strict)。
- **限流**:登录/注册端点按 IP 限流,TOTP 失败次数计数锁定。
- **API Key vs Session**:代理端点用 API Key;管理后台用 session。两套鉴权隔离。

## 9. 前端设计

Vue 3 + Naive UI + Vite + Pinia,构建后 `//go:embed` 嵌入。

### 页面结构

```
顶部导航栏:logo / 当前用户 / 主题切换 / 登出
│
├─ 登录注册页(未登录)
│   ├─ 本地登录(邮箱+密码+2FA)
│   ├─ Discord / X 登录按钮
│   ├─ Passkey 登录按钮
│   └─ 注册(开关控制显示)
│
└─ 主布局(已登录)
    │
    ├─ 仪表盘(Dashboard)
    │   ├─ 今日概览卡片:请求数/Token数/费用/成功率
    │   ├─ 趋势图(默认近7天):请求量 + 成功率双轴
    │   └─ 最近请求日志(前10条)
    │
    ├─ 统计分析
    │   ├─ 汇总面板:按模型/上游/Key 维度切换
    │   ├─ 时间趋势:折线图,可选按天/小时,时间范围筛选
    │   ├─ 费用核算:按模型/Key/时间段,表格+导出CSV
    │   ├─ 模型成功率:表格(总数/成功/失败/成功率/平均耗时)+失败原因下钻
    │   └─ 请求日志:分页表格,筛选(模型/Key/状态/时间/RequestId)
    │
    ├─ 模型管理(admin)
    │   ├─ 上游供应商:列表+增删改(名称/URL/Key/协议/状态)
    │   ├─ 自定义模型:列表+增删改(名称/映射上游/真实模型名/启用)
    │   └─ 模型定价:每模型一套价格(输入/输出/缓存读写),价格历史
    │
    ├─ API Key 管理
    │   ├─ Key 列表(前缀/标签/状态/创建时间/最后使用)
    │   ├─ 创建(返回明文一次)+ 复制
    │   └─ 编辑标签/禁用/删除
    │
    ├─ 配额管理(admin)
    │   └─ 按用户/Key 设额(token/费用,周期day/month/total)
    │
    ├─ 用户管理(admin)
    │   ├─ 用户列表(角色/状态/2FA/登录方式/创建时间)
    │   ├─ 创建用户/改角色/禁用/删除
    │   └─ 用户用量摘要
    │
    ├─ 系统设置(admin)
    │   ├─ 广播开关(0.0.0.0 / 127.0.0.1 一键切换+重启提示)
    │   ├─ 开放注册开关
    │   ├─ 强制2FA开关
    │   ├─ 日志保留天数
    │   ├─ OAuth配置(Discord/X client_id/secret)
    │   └─ 主密钥状态显示
    │
    └─ 账号设置(当前用户)
        ├─ 基本信息(邮箱)
        ├─ 安全:改密码 / 开关TOTP / 管理Passkey / 绑定解绑OAuth
        └─ 登录会话(查看/吊销)
```

### 关键交互

- **广播开关**:系统设置页醒目开关。切换后写 `settings.listen_host` + 触发优雅重启(关旧 listener 开新的),失败回滚 + 提示手动重启。广播开时若无任何 API Key,给风险提示。
- **模型定价编辑**:模型详情页内嵌定价表单,价格历史时间线展示。调价只影响后续请求,旧账单不变。
- **成功率页失败下钻**:点失败数 -> 按 `error_type` 分组饼图/条形图 -> 点类型展开对应日志。
- **Key 创建**:弹窗生成后只显示一次明文(复制按钮 + 警告),列表只存前缀。
- **配置生效**:大部分配置实时生效(读 DB);监听地址变更需重启。

### 前端工程

- **构建**:Vite,产物 `dist/` 被 Go `//go:embed` 嵌入。
- **路由**:Vue Router,后台路由守卫检查 session + 角色。
- **状态**:Pinia 存 session/用户/设置。
- **图表**:Naive UI 内置或轻量库(ECharts 按需)。
- **API 调用**:axios 封装,统一错误处理 + 401 跳登录。
- **响应式**:桌面为主,移动端可用。

## 10. 测试、部署与运维

### 测试策略

**单元测试(转换器为核心)**
- 6 个 Decoder/Encoder 纯函数测试:格式 X 输入 -> 断言 IR;IR -> 断言格式 X 输出。
- 9 种上下游组合往返测试(3 下游 × 3 上游)。
- 流式事件:喂真实 SSE chunk 序列,断言 `InternalEvent` 序列 + 末尾 `Usage` 提取。
- 边界:空消息、纯工具调用、多模态、cache_control、不同 finish_reason。

**统计层测试**:mock 上游响应,断言 token 解析、费用计算、配额累加;成功率判定各种 `status_code` + `error_type` 组合。

**集成测试**:起 mock 上游,端到端跑客户端 -> carryAPI -> mock 上游,覆盖流式透传、协议转换、鉴权、配额拦截、日志写入、超时/取消。

**认证测试**:密码登录、TOTP、Passkey(mock 凭证)、OAuth 回调(各分支)、权限(普通用户调 admin 端点返 403)。

**存储测试**:配额并发累加(多 goroutine)、日志保留清理任务。

**约定**:Go 原生 `testing` + `testify/assert`;转换器 fixture 放 `testdata/`;核心转换层 >90% 覆盖。

### 部署

**构建**

```bash
# 前端
cd web && npm install && npm run build   # 产物 web/dist

# 后端(前端被 embed)
GOOS=linux GOARCH=amd64 go build -o carryapi-linux-amd64 ./cmd/carryapi
GOOS=windows GOARCH=amd64 go build -o carryapi-windows-amd64.exe ./cmd/carryapi
```

单二进制,内含前端。`modernc.org/sqlite` 纯 Go 无 CGO,直接交叉编译。

**运行**

```bash
./carryapi   # 默认 0.0.0.0:8080,数据文件 ./carryapi.db
```

**首次启动**:无 `carryapi.db` 则自动建表 + 创建管理员账号(从 `CARRYAPI_ADMIN_EMAIL`/`CARRYAPI_ADMIN_PASSWORD` 读,或生成随机密码打印控制台)。无 `CARRYAPI_MASTER_KEY` 则生成并写入 `./carryapi.key` 提示。

**配置(环境变量)**

```
CARRYAPI_PORT=8080
CARRYAPI_DB_PATH=./carryapi.db
CARRYAPI_MASTER_KEY=...
CARRYAPI_ADMIN_EMAIL=...
CARRYAPI_ADMIN_PASSWORD=...
```

运行时配置(广播、注册、2FA、OAuth、定价等)走数据库 + 管理后台,不依赖重启(除监听地址)。

**监听地址变更**:写 `settings.listen_host` -> 优雅重启(`http.Server` 关旧 listener 开新的),失败回滚 + 提示手动重启。

### 运维

- **备份**:单文件 `carryapi.db`,停服拷贝。后台提供"导出配置 JSON"(上游/模型/定价/配额,不含密钥)。
- **日志保留**:后台定时任务(每小时)按 `log_retention_days` 删除老日志。
- **可观测**:控制台结构化日志(JSON,含 request_id/user_id/model/耗时/状态);`/api/health` 健康检查。
- **升级**:单二进制替换重启,SQLite 自动 migrate(`schema_version` 表 + 增量迁移)。
