# 首次启动管理员设置向导 + 管理员身份保护

日期:2026-08-12

## 背景 / 问题

用户无法登录 admin 账号。根因:

- 数据库里的 admin 账户创建于更早日期,其密码是**首次启动时随机生成并在控制台打印**的,不是 `.env` 里的 `a1234567`。
- `CARRYAPI_ADMIN_PASSWORD` 只在首次启动、库中无 admin 时才生效(`bootstrapAdmin`),库中已有 admin 后修改该环境变量无效。
- `.env` 文件从未被程序读取(代码中无 godotenv / LoadEnv 逻辑),应用只读 `os.Getenv()`。

## 目标

1. 删除现有数据库(`carryapi.db`),重新初始化。
2. 首次启动时,**不用随机打印密码**,而是提供一个浏览器初始化向导,由用户设置首个 admin 的邮箱和密码。
3. admin 可以把其他用户设为管理员(现有 `PUT /api/users/{id}` 的 role 已支持,保留)。
4. admin 的管理员身份不可移除 —— 具体约束:**仅保护自己 + 首个 admin**。

## 设计

### 后端

#### A. 初始化向导 API

新增 `internal/api/setup_handler.go`:

- `GET /api/setup/status`(公开,无需登录)
  - 返回 `{"needs_setup": true|false}`
  - `needs_setup = (users 表中无任何 admin)`

- `POST /api/setup/admin`(公开,但带防护)
  - body: `{email, password}`
  - 若库中已存在任意 admin → 403(防止二次覆盖)
  - 校验 email 非空、password 至少 8 位,否则 400
  - 创建 `role=admin` 的用户,返回 `{ok:true}`

#### B. `bootstrapAdmin` 改动

`cmd/carryapi/main.go` 中 `bootstrapAdmin` 不再无条件创建 admin:

- 仅当 `CARRYAPI_ADMIN_EMAIL` 且 `CARRYAPI_ADMIN_PASSWORD` 都被显式设置时,自动创建 admin(保留给脚本化部署场景)。
- 否则跳过,交给前端向导。
- **不再生成随机密码并打印。**

#### C. 管理员身份保护(用户管理接口)

`internal/api/user_handler.go`:

- `PUT /api/users/{id}`(改 role / status):
  - 禁止操作者自己被降级:若 `id == 当前用户` 且要把 role 改为非 admin → 400 "cannot remove your own admin role"
  - 禁止首个 admin 被降级或禁用(首个 admin = users 表中 id 最小的 admin)。
- `DELETE /api/users/{id}`:
  - 禁止删除自己(保留现有逻辑)
  - 禁止删除首个 admin

定义 helper:`store.IsBootstrapAdmin(id)` 或通过查询判断目标用户是否为“首个 admin”(id 最小的 admin)。

### 前端

#### D. 初始化向导页

新增 `web/src/views/SetupView.vue`(`/setup` 路由,公开):

- 标题「首次使用 · 设置管理员账户」
- 表单:邮箱 + 密码 + 确认密码
- 校验:邮箱非空、密码至少 8 位、两次一致
- 提交调 `POST /api/setup/admin`,成功后跳转 `/login`

#### E. 路由守卫调整

`web/src/router/index.ts` `beforeEach`:

- 未登录时先调 `GET /api/setup/status`
  - `needs_setup=true` 且不在 `/setup` → 跳 `/setup`
  - 否则 → 走原有 `/login` 逻辑

### 数据库重建

删除 `carryapi.db`、`carryapi.db-wal`、`carryapi.db-shm`,由新逻辑首次启动时重建,向导创建首个 admin。

### 测试

- 后端单元测试:
  - `GET /api/setup/status` 返回 needs_setup 正确
  - `POST /api/setup/admin` 创建成功;已有 admin 时返回 403
  - 降级/删除保护:自己、首个 admin 不可被降级/禁用/删除
- 前端 `npm run build` 通过

## 明确约定

- 「首个 admin」= users 表中 id 最小的 admin(即向导创建的那个初始管理员)。它永远不能被降级/禁用/删除。
- 后续其他 admin 之间可以互相降级,但不能降级自己。
