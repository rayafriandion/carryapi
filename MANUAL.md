# carryAPI 使用手册

> carryAPI 是一个自托管的 API 聚合路由服务:把多个上游模型供应商(OpenAI 兼容、Anthropic 等)聚合,对外统一暴露自定义模型名,提供可视化配置、用量/费用/成功率统计、多用户认证。
>
> 本文档是**完整操作手册**。**以后每新增或减少一个功能,必须同步更新本文档**(见「维护约定」)。

---

## 目录

- [1. 概述](#1-概述)
- [2. 安装与构建](#2-安装与构建)
- [3. 启动方式](#3-启动方式)
- [4. 首次启动与管理员](#4-首次启动与管理员)
- [5. 配置](#5-配置)
- [6. 管理后台](#6-管理后台)
- [7. 模型/供应商/定价配置](#7-模型供应商定价配置)
- [8. 代理端点(API)](#8-代理端点api)
- [9. 统计与日志](#9-统计与日志)
- [10. 认证体系](#10-认证体系)
- [11. 运维](#11-运维)
- [12. 常见问题](#12-常见问题)
- [13. 维护约定](#13-维护约定)

---

## 1. 概述

### 核心能力

- **多协议聚合代理**:客户端可用 OpenAI Chat / Responses / Anthropic 三种协议之一调用,代理自动转换为上游支持的协议。上游同样支持这三种协议任意组合。
- **多用户认证**:邮箱+密码、TOTP 2FA、Passkey(WebAuthn)、OAuth(Discord / X)。
- **API Key**:每个用户可创建多个带标签的 Key,用于调用代理端点。
- **配额管理**:按用户或 Key 设置 token / 费用上限。
- **用量统计**:输入/输出/缓存 token、费用、成功率,按模型/上游/Key 维度 + 时间趋势。
- **请求日志**:每次请求明细,支持分页与多条件筛选。
- **广播开关**:控制服务监听 `0.0.0.0`(局域网/公网可访问)或 `127.0.0.1`(仅本机)。
- **单二进制部署**:前端已内嵌,无需单独部署前端。

### 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go(单二进制) |
| 前端 | Vue 3 + Naive UI + Vite + Pinia + ECharts |
| 数据库 | SQLite(单文件,纯 Go 驱动) |
| 认证 | bcrypt、TOTP、WebAuthn、OAuth 2.0 |

### 跨平台

支持 **Windows(x64)** 与 **Linux(x64)**,无 CGO,交叉编译即可。启动脚本提供 `.bat`(Windows)与 `.sh`(Linux / macOS / git-bash)两套。

---

## 2. 安装与构建

### 前置要求

- **Go 1.22+**
- **Node.js**(仅构建前端时需要;运行单二进制不需要)

### 构建

```bash
# 1. 构建前端(产物 web/dist)
cd web && npm install && npm run build && cd ..

# 2. 构建后端单二进制(内嵌前端)
go build -o carryapi ./cmd/carryapi

# Windows 交叉编译:
# GOOS=windows GOARCH=amd64 go build -o carryapi.exe ./cmd/carryapi
# Linux 交叉编译:
# GOOS=linux   GOARCH=amd64 go build -o carryapi-linux-amd64 ./cmd/carryapi
```

> 前端只影响 `web/dist/`,后端构建时自动嵌入。

### 测试

```bash
# Go 全量测试
go test ./...

# 前端测试
cd web && npm run test

# 或一次跑全部
make test
```

---

## 3. 启动方式

> **统一启动**:提供了跨平台脚本,Windows 与 Linux 通用,不需要手动点击 exe 文件。

### 生产模式(推荐)

一条命令构建并启动(默认端口 **8067**):

```bash
# Linux / macOS / git-bash
bash scripts/run.sh

# Windows(cmd 或双击)
scripts\run.bat

# 指定端口
bash scripts/run.sh 9000
scripts\run.bat 9000
```

等价于手动执行:

```bash
go build -o carryapi ./cmd/carryapi
./carryapi
```

生产模式是**单进程**:前端已内嵌在后端二进制里,启动后访问 `http://localhost:8067/` 即是完整管理后台。

### 开发模式(前后端同时热更新)

```bash
# Linux / macOS / git-bash:同时启动前端 Vite + 后端
bash scripts/dev.sh

# Windows
scripts\dev.bat
```

- 前端开发服务器:`http://localhost:5173`(热更新,`/api` `/v1` 自动代理到后端 8067)
- 后端:`http://localhost:8067`
- 停止:按 `Ctrl+C`(`dev.bat` 需关闭前端窗口)。

### Makefile(可选)

有 `make` 的环境可用统一入口:

```bash
make run       # 构建并运行(端口可用 make run PORT=9000 覆盖)
make dev       # 开发模式
make test      # 全部测试
make build     # 仅构建
```

---

## 4. 首次启动与管理员

首次启动(无 `carryapi.db` 时)自动:

1. 创建数据库与全部表。
2. 创建**管理员账号**,控制台打印密码:
   ```
   created admin admin@carryapi.local with password: xxxx (change it immediately)
   ```
3. 生成主密钥文件 `carryapi.key`(如未设置 `CARRYAPI_MASTER_KEY`)。

用打印的账号(默认 `admin@carryapi.local`)与密码登录管理后台。**首次登录后建议立即开启 TOTP 两步验证**(账号设置页)。

可通过环境变量预设管理员(见下节),避免随机密码。

---

## 5. 配置

### 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `CARRYAPI_PORT` | 8067 | 监听端口 |
| `CARRYAPI_DB_PATH` | ./carryapi.db | SQLite 数据库文件路径 |
| `CARRYAPI_MASTER_KEY` | 自动生成到 carryapi.key | 敏感字段加密主密钥,32 字节 |
| `CARRYAPI_ADMIN_EMAIL` | admin@carryapi.local | 首次启动管理员邮箱 |
| `CARRYAPI_ADMIN_PASSWORD` | 随机生成 | 首次启动管理员密码 |
| `CARRYAPI_RP_ID` | localhost | WebAuthn Passkey 的 Relying Party 域 |
| `CARRYAPI_RP_ORIGIN` | http://localhost:{port} | WebAuthn origin(公网部署需改) |

### 广播开关(监听地址)

- **广播开** = 监听 `0.0.0.0`(局域网/公网可访问)
- **广播关** = 监听 `127.0.0.1`(仅本机)

存于数据库 `settings` 表 `listen_host`。管理后台「系统设置」页展示当前值。**修改监听地址需改配置并重启**(当前版本未提供运行时热切换)。

### OAuth 配置

Discord / X 登录需在管理后台后端配置 OAuth client id/secret。公网部署时还需把回调地址注册到对应平台(回调路径含 provider,如 `/api/auth/oauth/callback/discord`)。

---

## 6. 管理后台

访问 `http://<host>:8067/`。功能按角色显示:

| 页面 | 功能 | 权限 |
|------|------|------|
| 登录/注册 | 邮箱+密码、TOTP、Passkey、Discord/X OAuth | 公开 |
| 仪表盘 | 今日请求数/Token/费用/成功率、趋势图、最近日志 | 登录 |
| 统计分析 | 汇总(模型/上游/Key)、时间趋势、费用核算、成功率(失败下钻) | 登录 |
| 请求日志 | 分页 + 筛选(model/状态/错误类型/RequestId/时间) | 登录 |
| API Key | 创建(明文仅显示一次)、复制、编辑/禁用/删除 | 登录 |
| 模型管理 | 上游供应商、自定义模型、定价(含价格历史) | admin |
| 配额管理 | 设置 token/费用上限 | admin |
| 用户管理 | 创建/改角色/禁用/删除用户 | admin |
| 系统设置 | 广播开关(展示)、开放注册、强制2FA、日志保留天数 | admin |
| 账号设置 | 基本信息、绑定登录方式、TOTP 开关 | 登录 |

---

## 7. 模型/供应商/定价配置

要使用代理端点,需先用管理员配置:

1. **上游供应商**(模型管理 → 供应商):添加真实上游,填 `名称`、`base_url`、`API Key`、`协议`(openai_chat / openai_responses / anthropic)。
2. **自定义模型**(模型管理 → 模型):建一个对外模型名(如 `my-gpt4`),映射到某供应商的真实模型(如 `gpt-4o`)。
3. **定价**(模型管理 → 定价):给模型设输入/输出价格,可选缓存读/写价格(每百万 token)。调价只影响后续请求,旧账单不变。

配置完成后,即可用 API Key 调用 `/v1/*` 端点(见下节)。

---

## 8. 代理端点(API)

客户端调用代理端点,需携带 API Key(`Authorization: Bearer <key>` 或 `x-api-key: <key>`,Anthropic 风格)。

| 端点 | 协议 | 说明 |
|------|------|------|
| `/v1/chat/completions` | OpenAI Chat | 标准 Chat 补全 |
| `/v1/completions` | OpenAI Chat(旧) | 同上 |
| `/v1/responses` | OpenAI Responses | Responses API |
| `/v1/messages` | Anthropic Messages | Anthropic 格式 |
| `/v1/models` | - | 列出启用的自定义模型 |

**响应格式按请求路径决定**(Chat 请求返 Chat 格式,Responses 返 Responses 格式…),与上游协议无关。

### 示例(用 API Key 调 Chat)

```bash
curl http://localhost:8067/v1/chat/completions \
  -H "Authorization: Bearer carry-xxxx..." \
  -H "Content-Type: application/json" \
  -d '{"model":"my-gpt4","messages":[{"role":"user","content":"hi"}]}'
```

> `model` 字段填**自定义模型名**(如 `my-gpt4`),代理会映射到上游真实模型并转换协议。

### 列出模型

```bash
curl -H "Authorization: Bearer carry-xxxx..." http://localhost:8067/v1/models
```

---

## 9. 统计与日志

管理后台「统计分析」与「请求日志」页提供可视化查询,底层是以下只读 API(需登录,session):

| 端点 | 说明 |
|------|------|
| `GET /api/stats/summary` | 汇总:总请求/成功/失败/token/费用/平均耗时,按模型/上游/Key 分列 |
| `GET /api/stats/trend` | 时间趋势(按天/小时) |
| `GET /api/stats/cost` | 费用核算,按模型/Key/上游分组 |
| `GET /api/stats/success-rate` | 成功率(2xx 且无 error_type 才算成功)与平均耗时 |
| `GET /api/logs` | 请求日志分页 + 筛选 |

公共参数:

- `start` / `end`:时间范围,RFC3339;缺省最近 30 天。
- `granularity`:`day`(默认)或 `hour`(trend)。
- `group`:`model`(默认)/`key`/`provider`(cost、success-rate)。
- 日志:`page`/`page_size`(默认 50,上限 200)/`model`/`status`/`error_type`/`request_id`。

权限:普通用户只看自己的数据;admin 看全部并可传 `user_id` 过滤。

---

## 10. 认证体系

### 登录方式

一个账号可绑定多种登录方式(账号设置页管理):

| 方式 | 说明 |
|------|------|
| 密码 | bcrypt 存储 |
| TOTP 2FA | Authenticator App;提供一次性备份码 |
| Passkey | WebAuthn,指纹/安全密钥 |
| Discord / X | OAuth 登录 |

### API Key(代理端点鉴权)

- 在「API Key 管理」页创建,格式 `carry-<32 hex>`。
- **明文只在创建时显示一次**,关闭后无法再次查看,请立即复制保存。
- 可编辑标签、禁用、删除。

### 配额

- 按用户或 Key 设置 token / 费用上限,周期可选 day / month / total。
- 超限时代理端点返回 `429 quota_exceeded`。

---

## 11. 运维

### 备份

单文件 `carryapi.db`,停服后拷贝即可。管理后台提供「导出配置 JSON」(上游/模型/定价/配额,不含密钥)便于迁移。

### 日志保留

请求日志按「系统设置 → 日志保留天数」定期清理(默认 30 天),防止库膨胀。

### 升级

替换二进制重启即可。SQLite 表结构变更通过版本化迁移自动处理(`schema_version` 表)。

### 健康检查

```bash
curl http://localhost:8067/api/health
# {"status":"ok"}
```

### 安全

- 上游 API Key 与敏感字段 AES-GCM 加密存储,主密钥在 `carryapi.key`。
- 代理端点始终要求 API Key 鉴权(无论广播开关状态)。
- 广播开启时若未创建任何 API Key,建议尽快创建,避免被滥用。

---

## 12. 常见问题

**Q: 端口被占用怎么办?**
用 `CARRYAPI_PORT=其他端口` 或 `scripts/run.sh 其他端口` 指定。

**Q: 不知道管理员密码?**
删除 `carryapi.db` 重新启动(会重建库并打印新管理员密码);或设置 `CARRYAPI_ADMIN_EMAIL`/`CARRYAPI_ADMIN_PASSWORD` 预设。

**Q: 前端打不开 / 空白?**
确认访问的是单二进制(`go build` 后运行)而非仅后端。生产模式访问 `http://localhost:8067/`。

**Q: 调用代理端点报 model not found?**
自定义模型未配置或未启用。先在「模型管理」配置供应商 + 模型 + 定价。

**Q: 广播开但其他设备访问不了?**
确认监听 `0.0.0.0`,且防火墙/安全组放行端口。

---

## 13. 维护约定

> **重要**:每次**新增或减少功能**,必须同步修改本文档,包括:
> 1. 涉及新端点/新配置/新页面 → 更新对应章节(代理端点、配置、管理后台、统计等)。
> 2. 涉及启动方式 → 更新第 3 节,并同步 `scripts/` 脚本与 `Makefile`。
> 3. 涉及环境变量 → 更新第 5 节。
> 4. 涉及认证/权限 → 更新第 10 节。

任何代码改动若改变了用户可见行为、CLI、API、配置或部署方式,都应让本文档与 README 保持一致。
