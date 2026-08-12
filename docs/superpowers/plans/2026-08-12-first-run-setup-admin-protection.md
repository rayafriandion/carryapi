# 首次启动管理员设置向导 + 管理员身份保护 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 删除现有数据库重建后，首次启动通过浏览器向导设置首个 admin 邮箱/密码（不再随机打印），并保护首个 admin 的身份不可被降级/禁用/删除。

**Architecture:** 后端新增公开的 setup 状态/创建 API，替换 `bootstrapAdmin` 的随机创建逻辑；`UserHandler` 增加身份保护。前端新增 `/setup` 向导页，路由守卫在未登录时先检查 `needs_setup`。

**Tech Stack:** Go 1.22+ / chi / SQLite (modernc.org/sqlite) / Vue 3 + Vite + naive-ui + Pinia + axios

## Global Constraints

- 密码校验：非空且至少 8 位。
- 邮箱校验：非空。
- 「首个 admin」= users 表中 id 最小的 admin。
- 首个 admin 不可被降级为普通用户、不可被禁用、不可被删除。
- 任何 admin 不可降级/禁用/删除自己。
- 其他 admin 之间可以互相降级（但不能动自己）。
- 沿用现有代码风格：`JSON`/`JSONError` 响应助手、`user.Store`、`auth.HashPassword`、`setupAPI` 测试 fixture、`serve` helper。

---

### Task 1: 新增 user.Store 辅助方法（IsBootstrapAdmin / HasAdmin / FirstAdminID）

**Files:**
- Modify: `internal/user/user.go`（在 `List` 方法之后、`UpdateStatus` 之前插入）
- Test: `internal/user/user_test.go`

**Interfaces:**
- Produces:
  - `func (s *Store) HasAdmin() (bool, error)`
  - `func (s *Store) FirstAdminID() (int64, bool, error)` — 返回 id 最小的 admin 的 id；无 admin 时 ok=false

- [ ] **Step 1: 写失败测试**

在 `internal/user/user_test.go` 末尾追加：

```go
func TestHasAdminAndFirstAdminID(t *testing.T) {
	s := newStore(t)
	ok, err := s.HasAdmin()
	if err != nil {
		t.Fatalf("HasAdmin: %v", err)
	}
	if ok {
		t.Fatal("expected no admin initially")
	}
	_, found, err := s.FirstAdminID()
	if err != nil {
		t.Fatalf("FirstAdminID: %v", err)
	}
	if found {
		t.Fatal("expected FirstAdminID not found initially")
	}

	s.Create("a@x.com", "hash1", "user")
	s.Create("admin1@x.com", "hash2", "admin")
	s.Create("admin2@x.com", "hash3", "admin")

	ok, err = s.HasAdmin()
	if err != nil || !ok {
		t.Fatalf("HasAdmin after create: ok=%v err=%v", ok, err)
	}
	id, found, err := s.FirstAdminID()
	if err != nil || !found {
		t.Fatalf("FirstAdminID: found=%v err=%v", found, err)
	}
	// admin1 先创建,id 更小,应为首个 admin
	u, _ := s.GetByID(id)
	if u.Email != "admin1@x.com" {
		t.Fatalf("expected first admin admin1@x.com, got %s", u.Email)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/user/ -run TestHasAdminAndFirstAdminID -v`
Expected: FAIL（编译错误 `s.HasAdmin undefined`）

- [ ] **Step 3: 实现**

在 `internal/user/user.go` 的 `List` 方法之后插入：

```go
// HasAdmin reports whether at least one admin user exists.
func (s *Store) HasAdmin() (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role='admin'`).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("count admins: %w", err)
	}
	return n > 0, nil
}

// FirstAdminID returns the id of the lowest-id admin (the bootstrap admin).
// ok is false when no admin exists.
func (s *Store) FirstAdminID() (int64, bool, error) {
	var id int64
	err := s.db.QueryRow(`SELECT id FROM users WHERE role='admin' ORDER BY id LIMIT 1`).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("first admin: %w", err)
	}
	return id, true, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/user/ -run TestHasAdminAndFirstAdminID -v`
Expected: PASS

- [ ] **Step 5: 回归测试**

Run: `go test ./internal/user/`
Expected: PASS（全部）

- [ ] **Step 6: Commit**

```bash
git add internal/user/user.go internal/user/user_test.go
git commit -m "feat(user): add HasAdmin and FirstAdminID store helpers"
```

---

### Task 2: 新增 SetupHandler（setup 状态 + 创建首个 admin）

**Files:**
- Create: `internal/api/setup_handler.go`
- Modify: `internal/server/server.go`（Deps 加 `Setup *api.SetupHandler`）
- Modify: `internal/server/router.go`（挂载 setup 路由）
- Modify: `cmd/carryapi/main.go`（构造并注入 SetupHandler）
- Test: `internal/api/setup_handler_test.go`

**Interfaces:**
- Consumes:
  - `user.Store`：`HasAdmin() (bool, error)`、`Create(email, passwordHash, role string) (user.User, error)`
  - `auth.HashPassword(password string) (string, error)`
  - `api.JSON(w, status, data)`、`api.JSONError(w, status, msg)`
- Produces:
  - `func NewSetupHandler(users *user.Store) *SetupHandler`
  - `func (h *SetupHandler) Status(w http.ResponseWriter, r *http.Request)` → `200 {"needs_setup": bool}`
  - `func (h *SetupHandler) CreateAdmin(w http.ResponseWriter, r *http.Request)` → `200 {"ok":true}` / `400` / `403`
  - `Deps.Setup *api.SetupHandler`

- [ ] **Step 1: 写失败测试**

创建 `internal/api/setup_handler_test.go`：

```go
package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"carryapi/internal/auth"
)

func TestSetupStatusAndCreateAdmin(t *testing.T) {
	f := setupAPI(t)
	sh := NewSetupHandler(f.users)

	// 无 admin -> needs_setup true
	rec := serve(sh.Status, "GET", "/api/setup/status", nil)
	if rec.Code != 200 {
		t.Fatalf("status code=%d", rec.Code)
	}
	var st struct {
		NeedsSetup bool `json:"needs_setup"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil || !st.NeedsSetup {
		t.Fatalf("expected needs_setup=true, got %s", rec.Body.String())
	}

	// 创建 admin
	body, _ := json.Marshal(map[string]string{"email": "admin@x.com", "password": "secret123"})
	rec = serve(sh.CreateAdmin, "POST", "/api/setup/admin", body)
	if rec.Code != 200 {
		t.Fatalf("create admin code=%d body=%s", rec.Code, rec.Body.String())
	}

	// 创建后 needs_setup false
	rec = serve(sh.Status, "GET", "/api/setup/status", nil)
	_ = json.Unmarshal(rec.Body.Bytes(), &st)
	if st.NeedsSetup {
		t.Fatalf("expected needs_setup=false after create, got %s", rec.Body.String())
	}

	// 已存在 admin 再创建 -> 403
	body2, _ := json.Marshal(map[string]string{"email": "admin2@x.com", "password": "secret123"})
	rec = serve(sh.CreateAdmin, "POST", "/api/setup/admin", body2)
	if rec.Code != 403 {
		t.Fatalf("expected 403 when admin exists, got %d body=%s", rec.Code, rec.Body.String())
	}

	// 新 admin 角色正确且可登录
	u, err := f.users.GetByEmail("admin@x.com")
	if err != nil || u.Role != "admin" {
		t.Fatalf("created user role=%q err=%v", u.Role, err)
	}
	if !auth.VerifyPassword("secret123", u.PasswordHash) {
		t.Fatal("created admin password not verifiable")
	}
}

func TestSetupCreateAdminValidation(t *testing.T) {
	f := setupAPI(t)
	sh := NewSetupHandler(f.users)

	// 密码太短 -> 400
	body, _ := json.Marshal(map[string]string{"email": "a@x.com", "password": "short"})
	rec := serve(sh.CreateAdmin, "POST", "/api/setup/admin", body)
	if rec.Code != 400 {
		t.Fatalf("expected 400 for short password, got %d", rec.Code)
	}
	// 空邮箱 -> 400
	body2, _ := json.Marshal(map[string]string{"email": "", "password": "secret123"})
	rec = serve(sh.CreateAdmin, "POST", "/api/setup/admin", body2)
	if rec.Code != 400 {
		t.Fatalf("expected 400 for empty email, got %d", rec.Code)
	}
	// 非法 JSON -> 400
	rec = serve(sh.CreateAdmin, "POST", "/api/setup/admin", []byte(`{bad`))
	if rec.Code != 400 {
		t.Fatalf("expected 400 for bad json, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/api/ -run 'TestSetup' -v`
Expected: FAIL（编译错误 `NewSetupHandler undefined`）

- [ ] **Step 3: 实现 handler**

创建 `internal/api/setup_handler.go`：

```go
package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"carryapi/internal/auth"
	"carryapi/internal/user"
)

type SetupHandler struct {
	users *user.Store
}

func NewSetupHandler(users *user.Store) *SetupHandler {
	return &SetupHandler{users: users}
}

// Status reports whether the first admin has been created yet.
func (h *SetupHandler) Status(w http.ResponseWriter, r *http.Request) {
	has, err := h.users.HasAdmin()
	if err != nil {
		JSONError(w, 500, "failed to check setup status")
		return
	}
	JSON(w, 200, map[string]any{"needs_setup": !has})
}

type setupAdminRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// CreateAdmin creates the first admin. Only allowed while no admin exists.
func (h *SetupHandler) CreateAdmin(w http.ResponseWriter, r *http.Request) {
	has, err := h.users.HasAdmin()
	if err != nil {
		JSONError(w, 500, "failed to check setup status")
		return
	}
	if has {
		JSONError(w, 403, "setup already complete")
		return
	}
	var req setupAdminRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, 400, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		JSONError(w, 400, "email is required")
		return
	}
	if len(req.Password) < 8 {
		JSONError(w, 400, "password must be at least 8 characters")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		JSONError(w, 500, "hash failed")
		return
	}
	if _, err := h.users.Create(req.Email, hash, "admin"); err != nil {
		JSONError(w, 400, err.Error())
		return
	}
	JSON(w, 200, map[string]any{"ok": true})
}
```

- [ ] **Step 4: 注入 Deps 与路由**

在 `internal/server/server.go` 的 `Deps` struct 中 `Auth *api.AuthHandler` 后追加：

```go
	Setup    *api.SetupHandler
```

在 `internal/server/router.go` 的 `buildRouter` 内、`/api/auth` 区块之前插入（公开端点，无鉴权）：

```go
	// setup(首次启动向导,公开)
	if deps.Setup != nil {
		r.Get("/api/setup/status", deps.Setup.Status)
		r.Post("/api/setup/admin", deps.Setup.CreateAdmin)
	}
```

在 `cmd/carryapi/main.go` 中，`usersH := api.NewUserHandler(...)` 之后追加：

```go
	setupH := api.NewSetupHandler(us)
```

并在 `srv := server.New(cfg, server.Deps{ ... })` 中 `UsersH: usersH,` 之后追加：

```go
		Setup:    setupH,
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/api/ -run 'TestSetup' -v`
Expected: PASS

- [ ] **Step 6: 构建验证**

Run: `go build ./...`
Expected: 编译成功，无错误

- [ ] **Step 7: Commit**

```bash
git add internal/api/setup_handler.go internal/api/setup_handler_test.go internal/server/server.go internal/server/router.go cmd/carryapi/main.go
git commit -m "feat(api): add first-run setup wizard endpoints"
```

---

### Task 3: 修改 bootstrapAdmin（不再随机创建，仅显式 env 创建）

**Files:**
- Modify: `cmd/carryapi/main.go`（`bootstrapAdmin` 函数）
- Test: `cmd/carryapi/main_test.go`（新建，若有）

**Interfaces:**
- Consumes: 无新增依赖（复用现有 `bootstrapAdmin(d, us)`）
- Produces: 修改后的 `bootstrapAdmin` 行为

- [ ] **Step 1: 实现**

将 `cmd/carryapi/main.go` 中 `bootstrapAdmin` 函数体整体替换为：

```go
// bootstrapAdmin creates an admin only when both CARRYAPI_ADMIN_EMAIL and
// CARRYAPI_ADMIN_PASSWORD are explicitly set AND no admin exists yet. This
// preserves scripted/provisioned deployments. Otherwise setup is left to the
// first-run browser wizard (/api/setup/admin). No random password is printed.
func bootstrapAdmin(d *sql.DB, us *user.Store) {
	email := os.Getenv("CARRYAPI_ADMIN_EMAIL")
	pw := os.Getenv("CARRYAPI_ADMIN_PASSWORD")
	if email == "" || pw == "" {
		return
	}
	has, err := us.HasAdmin()
	if err != nil {
		log.Printf("admin count check: %v", err)
		return
	}
	if has {
		return
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		log.Printf("hash admin password: %v", err)
		return
	}
	if _, err := us.Create(email, hash, "admin"); err != nil {
		log.Printf("create admin: %v", err)
	}
}
```

如果 `generateRandomPassword` 不再被任何代码引用，删除该函数与 `crypto/rand`、`encoding/hex`、`fmt` 中不再使用的 import（仅删除确实未使用的）。

- [ ] **Step 2: 构建验证**

Run: `go build ./...`
Expected: 编译成功，无错误（若 import 未用会报错，按需清理）

- [ ] **Step 3: 测试**

Run: `go vet ./cmd/... && go test ./cmd/...`
Expected: 无输出错误 / PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/carryapi/main.go
git commit -m "refactor(cmd): only auto-create admin when both env vars set"
```

---

### Task 4: 用户管理接口加入 admin 身份保护

**Files:**
- Modify: `internal/api/user_handler.go`（`Update`、`Delete`）
- Test: `internal/api/user_handler_test.go`（新建）

**Interfaces:**
- Consumes:
  - `user.Store`: `FirstAdminID() (int64, bool, error)`、`GetByID(id)`、`UpdateRole`、`UpdateStatus`
  - `middleware.UserFromContext(r.Context())`
- Produces: 修改后的 `UserHandler.Update` / `UserHandler.Delete` 行为

- [ ] **Step 1: 写失败测试**

创建 `internal/api/user_handler_test.go`：

```go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"carryapi/internal/middleware"
	"carryapi/internal/user"
)

// authedReq returns a request whose context carries the given user (simulates
// a logged-in admin via SessionMiddleware).
func authedReq(usr *user.User, method, path string, body []byte) *http.Request {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.UserKey{}, usr)
	return req.WithContext(ctx)
}

func TestUpdateCannotDemoteSelf(t *testing.T) {
	f := setupAPI(t)
	admin := &user.User{ID: 1, Email: "a@x.com", Role: "admin", Status: "active"}

	body, _ := json.Marshal(map[string]string{"role": "user"})
	req := authedReq(admin, "PUT", "/api/users/1", body)
	rec := httptest.NewRecorder()
	f.users.Create(admin.Email, "hash", "admin")
	h := NewUserHandler(f.users, f.sessions)
	h.Update(rec, req)
	if rec.Code != 400 {
		t.Fatalf("expected 400 demoting self, got %d body=%s", rec.Code, rec.Body.String())
	}
	// 角色未被改变
	got, _ := f.users.GetByID(1)
	if got.Role != "admin" {
		t.Fatalf("role changed unexpectedly: %q", got.Role)
	}
}

func TestUpdateCannotDisableSelf(t *testing.T) {
	f := setupAPI(t)
	admin := &user.User{ID: 1, Email: "a@x.com", Role: "admin", Status: "active"}
	f.users.Create(admin.Email, "hash", "admin")

	body, _ := json.Marshal(map[string]string{"status": "disabled"})
	req := authedReq(admin, "PUT", "/api/users/1", body)
	rec := httptest.NewRecorder()
	NewUserHandler(f.users, f.sessions).Update(rec, req)
	if rec.Code != 400 {
		t.Fatalf("expected 400 disabling self, got %d", rec.Code)
	}
}

func TestUpdateCannotDemoteFirstAdmin(t *testing.T) {
	f := setupAPI(t)
	f.users.Create("boot@x.com", "hash", "admin") // id=1 首个 admin
	f.users.Create("other@x.com", "hash", "admin") // id=2 另一个 admin
	other := &user.User{ID: 2, Email: "other@x.com", Role: "admin", Status: "active"}

	body, _ := json.Marshal(map[string]string{"role": "user"})
	req := authedReq(other, "PUT", "/api/users/1", body)
	rec := httptest.NewRecorder()
	NewUserHandler(f.users, f.sessions).Update(rec, req)
	if rec.Code != 400 {
		t.Fatalf("expected 400 demoting first admin, got %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := f.users.GetByID(1)
	if got.Role != "admin" {
		t.Fatalf("first admin demoted unexpectedly: %q", got.Role)
	}
}

func TestUpdateCanDemoteOtherNonFirstAdmin(t *testing.T) {
	f := setupAPI(t)
	f.users.Create("boot@x.com", "hash", "admin") // id=1 首个
	f.users.Create("other@x.com", "hash", "admin") // id=2
	boot := &user.User{ID: 1, Email: "boot@x.com", Role: "admin", Status: "active"}

	body, _ := json.Marshal(map[string]string{"role": "user"})
	req := authedReq(boot, "PUT", "/api/users/2", body)
	rec := httptest.NewRecorder()
	NewUserHandler(f.users, f.sessions).Update(rec, req)
	if rec.Code != 200 {
		t.Fatalf("expected 200 demoting non-first admin, got %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := f.users.GetByID(2)
	if got.Role != "user" {
		t.Fatalf("expected demoted to user, got %q", got.Role)
	}
}

func TestDeleteCannotDeleteFirstAdmin(t *testing.T) {
	f := setupAPI(t)
	f.users.Create("boot@x.com", "hash", "admin") // id=1 首个
	f.users.Create("other@x.com", "hash", "admin") // id=2
	other := &user.User{ID: 2, Email: "other@x.com", Role: "admin", Status: "active"}

	req := authedReq(other, "DELETE", "/api/users/1", nil)
	rec := httptest.NewRecorder()
	NewUserHandler(f.users, f.sessions).Delete(rec, req)
	if rec.Code != 400 {
		t.Fatalf("expected 400 deleting first admin, got %d", rec.Code)
	}
}
```

> 测试文件需 `import "bytes"`、`"context"`、`"encoding/json"`、`"io"`、`"net/http"`、`"net/http/httptest"`、`"testing"`，以及 `carryapi/internal/middleware`、`carryapi/internal/user`。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/api/ -run 'TestUpdateCannot|TestUpdateCan|TestDeleteCannot' -v`
Expected: FAIL（当前逻辑放行，测试断言不通过）

- [ ] **Step 3: 实现保护逻辑**

修改 `internal/api/user_handler.go`：

在 `Update` 方法开头（解析 id 与 body 之后、执行 Update 之前）插入保护逻辑：

```go
	// admin 身份保护:不能降级/禁用自己,不能降级/禁用首个 admin
	if req.Role != "" && req.Role != "admin" || req.Status == "disabled" {
		if err := h.guardAdminProtection(r, id); err != nil {
			JSONError(w, 400, err.Error())
			return
		}
	}
```

并新增一个 `guardAdminProtection` 方法（放在 `Update` 方法之后）：

```go
// guardAdminProtection rejects demoting/disabling the current user or the
// bootstrap (first) admin. Returns an error describing the violation.
func (h *UserHandler) guardAdminProtection(r *http.Request, id int64) error {
	me, ok := middleware.UserFromContext(r.Context())
	if !ok {
		return fmt.Errorf("unauthorized")
	}
	if me.ID == id {
		return fmt.Errorf("cannot remove your own admin role")
	}
	firstID, found, err := h.users.FirstAdminID()
	if err != nil {
		return fmt.Errorf("failed to check admin")
	}
	if found && firstID == id {
		return fmt.Errorf("cannot modify the initial admin account")
	}
	return nil
}
```

在 `Delete` 方法中、现有「不能删自己」检查后追加「不能删首个 admin」：

```go
	// 防止删首个 admin
	firstID, found, err := h.users.FirstAdminID()
	if err != nil {
		JSONError(w, 500, "failed to check admin")
		return
	}
	if found && firstID == id {
		JSONError(w, 400, "cannot delete the initial admin account")
		return
	}
```

在 `internal/api/user_handler.go` 的 import 中加入 `"fmt"`。

> 注：`guardAdminProtection` 中的 `fmt` 仅用于错误构造；也可用 `errors.New`。本计划统一用 `fmt.Errorf`，需在 import 中加 `"fmt"`。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/api/ -run 'TestUpdateCannot|TestUpdateCan|TestDeleteCannot' -v`
Expected: PASS

- [ ] **Step 5: 回归测试**

Run: `go test ./internal/api/`
Expected: PASS（全部，含既有 handler 测试）

- [ ] **Step 6: Commit**

```bash
git add internal/api/user_handler.go internal/api/user_handler_test.go
git commit -m "feat(api): protect current and first admin from demotion/disable/delete"
```

---

### Task 5: 前端 SetupView 向导页

**Files:**
- Create: `web/src/views/SetupView.vue`
- Modify: `web/src/router/index.ts`（路由 + 守卫）
- Test: `web` 构建验证

**Interfaces:**
- Consumes:
  - `http.get('/api/setup/status')` → `{needs_setup: bool}`
  - `http.post('/api/setup/admin', {email, password})` → `{ok:true}`
  - `useRouter().push('/login')`
- Produces: 路由 `{ path: '/setup', name: 'setup', component: SetupView }`

- [ ] **Step 1: 创建 SetupView.vue**

创建 `web/src/views/SetupView.vue`：

```vue
<template>
  <div class="setup-page">
    <n-card class="setup-card" :bordered="false">
      <div class="setup-title">carryAPI · 首次设置</div>
      <p class="setup-sub">请设置管理员账户，用于登录管理后台。</p>
      <n-form ref="formRef" :model="form" :rules="rules">
        <n-form-item label="邮箱" path="email">
          <n-input v-model:value="form.email" placeholder="admin@example.com" />
        </n-form-item>
        <n-form-item label="密码" path="password">
          <n-input v-model:value="form.password" type="password" show-password-on="click" placeholder="至少 8 位" />
        </n-form-item>
        <n-form-item label="确认密码" path="confirm">
          <n-input v-model:value="form.confirm" type="password" show-password-on="click" placeholder="再次输入密码" @keydown.enter="onSubmit" />
        </n-form-item>
        <n-button type="primary" block :loading="loading" @click="onSubmit">创建管理员</n-button>
      </n-form>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage, NInput, NButton, NForm, NFormItem, NCard } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'
import { http, errorMessage } from '../api/http'

const router = useRouter()
const message = useMessage()

const formRef = ref<FormInst | null>(null)
const loading = ref(false)
const form = reactive({ email: '', password: '', confirm: '' })

const rules: FormRules = {
  email: [{ required: true, message: '请输入邮箱', trigger: 'blur' }],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 8, message: '密码至少 8 位', trigger: 'blur' },
  ],
  confirm: [
    {
      validator: (_rule, value: string) => value === form.password,
      message: '两次输入的密码不一致',
      trigger: 'blur',
    },
  ],
}

async function onSubmit() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  loading.value = true
  try {
    await http.post('/api/setup/admin', { email: form.email, password: form.password })
    message.success('管理员已创建，请登录')
    router.push('/login')
  } catch (e) {
    message.error(errorMessage(e))
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.setup-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
}
.setup-card {
  width: 380px;
}
.setup-title {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 4px;
}
.setup-sub {
  color: #999;
  margin-bottom: 16px;
}
</style>
```

- [ ] **Step 2: 注册路由与守卫**

修改 `web/src/router/index.ts`：

在 routes 数组最前（`/login` 之前）加入：

```ts
    { path: '/setup', name: 'setup', component: () => import('../views/SetupView.vue') },
```

将 `beforeEach` 替换为：

```ts
router.beforeEach(async (to) => {
  const auth = useAuthStore()
  if (!auth.initialized) {
    try { await auth.fetchMe() } catch { /* 未登录 */ }
  }
  // 未登录时先检查是否需要进行首次设置
  if (!auth.isLoggedIn) {
    let needsSetup = false
    try {
      const res = await http.get('/api/setup/status')
      needsSetup = !!res.data?.needs_setup
    } catch { /* 忽略 */ }
    if (needsSetup) {
      if (to.name !== 'setup') return { name: 'setup' }
      return true
    }
    if (to.name === 'setup') return { name: 'login' }
    if (to.name === 'login') return true
    return { name: 'login' }
  }
  // 已登录
  if (to.name === 'login' || to.name === 'setup') return { name: 'dashboard' }
  if (to.meta.admin && !auth.isAdmin) return { name: 'dashboard' }
  return true
})
```

在 `web/src/router/index.ts` 顶部 import 中加入：

```ts
import { http } from '../api/http'
```

- [ ] **Step 3: 类型检查与构建**

Run: `cd web && npx vue-tsc --noEmit 2>/dev/null; npm run build`
Expected: 构建成功（若 `vue-tsc` 未配置，则仅 `npm run build`）

- [ ] **Step 4: Commit**

```bash
git add web/src/views/SetupView.vue web/src/router/index.ts
git commit -m "feat(web): add first-run admin setup wizard page"
```

---

### Task 6: 删除数据库并验证全流程

**Files:**
- Delete: `carryapi.db`、`carryapi.db-wal`、`carryapi.db-shm`

**Interfaces:**
- Consumes: 前面所有任务的产物（编译后的二进制 + 前端构建）

- [ ] **Step 1: 确认当前服务未运行（或先停止）**

Run: `taskkill //F //PID 33672 2>/dev/null; echo "stopped"`（若 PID 不同，用 `netstat -ano | grep :8067` 找实际 PID）

- [ ] **Step 2: 删除数据库文件**

Run: `rm -f carryapi.db carryapi.db-wal carryapi.db-shm`

- [ ] **Step 3: 构建并启动**

Run: `cd web && npm run build && cd .. && go build -o carryapi ./cmd/carryapi && ./carryapi &`
Expected: 启动成功，无随机密码打印

- [ ] **Step 4: 验证 setup 状态**

Run: `curl -s http://localhost:8067/api/setup/status`
Expected: `{"needs_setup":true}`

- [ ] **Step 5: 通过 API 创建首个 admin**

Run:
```bash
curl -s -X POST http://localhost:8067/api/setup/admin \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@carryapi.local","password":"a1234567"}'
```
Expected: `{"ok":true}`

- [ ] **Step 6: 用新密码登录**

Run:
```bash
curl -s -i -X POST http://localhost:8067/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@carryapi.local","password":"a1234567"}'
```
Expected: `200`（不再 401）

- [ ] **Step 7: 回归测试**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 8: 更新文档（MANUAL.md / README.md）**

- 修改 MANUAL.md 第 4 节「首次启动与管理员」、第 5 节 env 表、FAQ 341「不知道管理员密码」。
- 修改 README.md 相关「首次启动」段落。
  内容要点：
  - 首次启动无 admin 时，访问网页会自动进入「设置管理员」向导。
  - `CARRYAPI_ADMIN_EMAIL`/`CARRYAPI_ADMIN_PASSWORD` 需**两个都设置**才会自动创建 admin（供脚本化部署）；否则用向导。
  - 首个 admin 身份不可被降级/禁用/删除。
  - FAQ：忘记密码 → 用向导重建，或删除 `carryapi.db` 后重新走向导。

- [ ] **Step 9: Commit**

```bash
git add MANUAL.md README.md
git commit -m "docs: document first-run setup wizard and admin protection"
```

---

## 自检结果

- **Spec 覆盖**：向导 API（Task 2）、bootstrapAdmin 改动（Task 3）、身份保护（Task 4）、前端向导页 + 路由守卫（Task 5）、删库重建验证（Task 6）、测试（各 Task）——全部覆盖。
- **占位符**：无 TBD/TODO；每步含完整代码与命令。
- **类型一致性**：`SetupHandler.Status`/`CreateAdmin`、`Deps.Setup`、`user.Store.HasAdmin/FirstAdminID` 在各 Task 间命名一致；前端 `needs_setup` 与后端 JSON 字段一致。
