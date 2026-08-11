# carryAPI

自托管 API 聚合路由服务。多上游聚合、三种协议互转(OpenAI Chat / Responses / Anthropic)、可视化配置、用量与费用统计、成功率监控、多用户认证。

## 状态

子项目 1(项目骨架与基础设施)已完成。后续子项目:认证、协议适配层(IR)、上游代理、统计与管理 API、前端。

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

默认监听 `0.0.0.0:8080`(广播开,其他设备可访问)。数据文件 `./carryapi.db`,主密钥 `./carryapi.key`(首次自动生成)。

### 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `CARRYAPI_PORT` | 8080 | 监听端口 |
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
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer carry-xxxx..." \
  -H "Content-Type: application/json" \
  -d '{"model":"my-gpt4","messages":[{"role":"user","content":"hi"}]}'
```

模型列表:

```bash
curl -H "Authorization: Bearer carry-xxxx..." http://localhost:8080/v1/models
```

上游供应商、模型与定价由管理员通过管理 API 配置:`/api/providers`、`/api/models`、`/api/models/{id}/price`。上游协议(openai_chat / openai_responses / anthropic)与客户端使用的下游协议可任意组合,由代理自动转换。

## 开发

```bash
# 前端(热更新)
cd web && npm run dev

# 后端
go run ./cmd/carryapi
```

前端开发服务器把 `/api` 和 `/v1` 代理到后端 `127.0.0.1:8080`。

## 测试

```bash
go test ./...
```
