# carryAPI

自托管 API 聚合路由服务。多上游聚合、三种协议互转(OpenAI Chat / Responses / Anthropic)、可视化配置、用量与费用统计、成功率监控、多用户认证。

> 📖 **完整使用手册见 [MANUAL.md](MANUAL.md)**(安装、构建、启动、配置、API、运维、FAQ)。以下为快速开始。

## 状态

✅ 全部功能已完成:单二进制部署(前端内嵌)、认证(密码/2FA/Passkey/OAuth)、API Key、配额、多协议代理、用量/费用/成功率统计、请求日志、管理后台。

## 快速开始

```bash
# 生产模式(构建 + 启动,默认端口 8067)
bash scripts/run.sh          # Linux / macOS / git-bash
# 或 Windows:
# scripts\run.bat

# 开发模式(前端热更新 + 后端同时启动)
bash scripts/dev.sh
```

访问 `http://localhost:8067/`,用首次启动控制台打印的管理员密码登录。


## 构建

需要 Go 1.22+ 与 Node.js。

```bash
# 前端
cd web && npm install && npm run build

# 后端(内嵌前端)
go build -o carryapi ./cmd/carryapi
```

交叉编译:

```bash
GOOS=linux GOARCH=amd64 go build -o carryapi-linux-amd64 ./cmd/carryapi
GOOS=windows GOARCH=amd64 go build -o carryapi-windows-amd64.exe ./cmd/carryapi
```

## 运行

```bash
./carryapi
```

默认监听 `0.0.0.0:8067`(广播开,其他设备可访问)。数据文件 `./carryapi.db`,主密钥 `./carryapi.key`(首次自动生成)。

### 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `CARRYAPI_PORT` | 8067 | 监听端口 |
| `CARRYAPI_DB_PATH` | ./carryapi.db | 数据库路径 |
| `CARRYAPI_MASTER_KEY` | (自动生成) | 敏感字段加密主密钥,32 字节 |

### 广播开关

广播开 = 监听 `0.0.0.0`(局域网/公网可访问);广播关 = 监听 `127.0.0.1`(仅本机)。存于数据库 `settings` 表 `listen_host` 键。后续子项目的管理后台提供可视化切换(改值后需重启进程)。

## 认证

### 首次启动管理员

首次启动时自动创建管理员账号(检测到库中无 `admin` 角色用户时):

| 变量 | 默认 | 说明 |
|------|------|------|
| `CARRYAPI_ADMIN_EMAIL` | admin@carryapi.local | 管理员邮箱 |
| `CARRYAPI_ADMIN_PASSWORD` | (随机生成) | 管理员密码;未设置时生成随机 16 字节(32 位十六进制)密码并打印到 stdout,请立即修改 |

### 登录方式

| 方式 | 说明 |
|------|------|
| 密码 | `POST /api/auth/login`(邮箱 + 密码),成功后下发 session + CSRF cookie |
| TOTP 2FA | 开启后登录返回 `requires_2fa`,再 `POST /api/auth/2fa/complete`(邮箱 + 验证码/备份码) |
| Passkey | `POST /api/auth/passkey/register/begin` + `/finish` 注册,`POST /api/auth/passkey/login/begin` + `/finish` 登录(WebAuthn) |
| OAuth | Discord / X,`GET /api/auth/oauth/{provider}` 发起,`GET /api/auth/oauth/callback/{provider}` 回调 |

登出:`POST /api/auth/logout`。当前用户信息:`GET /api/auth/me`。

### API Key

- 创建:`POST /api/keys`(JSON `{"label":"..."}`),明文 key **仅此一次**返回在响应的 `key` 字段,请立即保存。
- 列表:`GET /api/keys`(返回 `key_prefix`,不含明文)。
- 更新/删除:`PUT/DELETE /api/keys/{id}`。
- 创建/删除等写操作需携带 `X-CSRF-Token` 请求头(与登录下发的 `carryapi_csrf` cookie 值一致)。

### 2FA 开启

登录后 `POST /api/auth/2fa/setup`,返回 TOTP `secret`、`otpauth_url` 和一次性 `backup_codes`。关闭:`POST /api/auth/2fa/disable`(需密码)。

### OAuth 配置环境变量

OAuth 提供方通过以下 `settings` 表键配置(未设置或未完整设置时对应提供方不启用):

| 键 | 说明 |
|------|------|
| `oauth_discord_client_id` / `oauth_discord_client_secret` / `oauth_discord_redirect_url` | Discord OAuth2 应用 |
| `oauth_x_client_id` / `oauth_x_client_secret` / `oauth_x_redirect_url` | X(Twitter)OAuth2 应用 |

管理后台可改这些键;改后需重启进程生效。

## 代理端点

代理端点将上游 LLM 服务聚合为统一 API。客户端仅需携带 API Key 调用:

| 端点 | 下游协议 |
|------|----------|
| `POST /v1/chat/completions` | OpenAI Chat |
| `POST /v1/responses` | OpenAI Responses |
| `POST /v1/messages` | Anthropic Messages |
| `GET /v1/models` | OpenAI 模型列表 |

鉴权方式二选一:

- `Authorization: Bearer <api-key>`
- `x-api-key: <api-key>`

调用示例(Chat):

```bash
curl http://localhost:8067/v1/chat/completions \
  -H "Authorization: Bearer carry-xxxx..." \
  -H "Content-Type: application/json" \
  -d '{"model":"my-gpt4","messages":[{"role":"user","content":"hi"}]}'
```

模型列表:

```bash
curl -H "Authorization: Bearer carry-xxxx..." http://localhost:8067/v1/models
```

上游供应商、模型与定价由管理员通过管理 API 配置:`/api/providers`、`/api/models`、`/api/models/{id}/price`。上游协议(openai_chat / openai_responses / anthropic)与客户端使用的下游协议可任意组合,由代理自动转换。

## 统计 API

用量、费用与成功率统计及请求日志查询。需登录(session);GET 只读,无需 CSRF 头。

| 端点 | 说明 |
|------|------|
| `GET /api/stats/summary` | 汇总:总请求数、成功/失败数、输入/输出/缓存 token、总费用、平均耗时,并按模型/上游/Key 分列 |
| `GET /api/stats/trend` | 时间趋势(按天/小时):各桶请求数、成功数、token、费用 |
| `GET /api/stats/cost` | 费用核算(基于 request_logs 快照),按模型/Key/上游分组 |
| `GET /api/stats/success-rate` | 成功率(2xx 且无 error_type 才算成功)与平均耗时,按模型/Key/上游分组 |
| `GET /api/logs` | 请求日志分页查询 + 筛选 |

公共查询参数:

- `start` / `end`:时间范围,RFC3339 格式(`2026-08-01T00:00:00Z`);缺省为最近 30 天。
- `granularity`:`day`(默认)或 `hour`,用于 `trend`。
- `group`:`model`(默认)、`key` 或 `provider`,用于 `cost` 与 `success-rate`。
- 日志分页与筛选:`page`(默认 1)、`page_size`(默认 50,上限 200)、`model`、`status`(HTTP 状态码)、`error_type`、`request_id`。

权限:所有端点需登录。普通用户仅能看到自己的数据;admin 可查看全部,并可传 `user_id` 过滤指定用户。

## 管理后台

访问 `http://<host>:8067/` 进入 Vue 3 + Naive UI 管理后台。功能按角色显示:

| 页面 | 功能 | 权限 |
|------|------|------|
| 登录/注册 | 邮箱+密码、TOTP 2FA、Passkey、Discord/X OAuth | 公开 |
| 仪表盘 | 今日请求数/Token/费用/成功率、趋势图、最近日志 | 登录 |
| 统计分析 | 汇总(按模型/上游/Key)、时间趋势、费用核算、成功率(可下钻) | 登录 |
| 请求日志 | 分页 + 筛选(model/状态/错误类型/RequestId/时间) | 登录 |
| API Key | 创建(明文仅显示一次)、复制、编辑标签/禁用/删除 | 登录 |
| 模型管理 | 上游供应商、自定义模型、定价(含价格历史) | admin |
| 配额管理 | 按用户/Key 设 token/费用上限 | admin |
| 用户管理 | 创建/改角色/禁用/删除用户 | admin |
| 系统设置 | 广播开关(展示)、开放注册、强制2FA、日志保留天数 | admin |
| 账号设置 | 基本信息、绑定登录方式、TOTP 开关 | 登录 |

> 说明:广播开关当前为展示项——监听地址需在服务器配置中修改并重启生效。OAuth client id/secret 通过后端设置配置。

前端开发:`cd web && npm run dev`(热更新,`/api`/`/v1` 代理到后端)。

## 开发

```bash
# 前端(热更新)
cd web && npm run dev

# 后端
go run ./cmd/carryapi
```

前端开发服务器把 `/api` 和 `/v1` 代理到后端 `127.0.0.1:8067`。

## 测试

```bash
go test ./...
```
