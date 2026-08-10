# carryAPI 子项目 2:认证与用户 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现完整的多用户认证后端:密码登录 + TOTP 2FA + Passkey(WebAuthn)+ OAuth(Discord/X)+ 服务端 session + 多用户角色 + API Key 管理 + 配额 CRUD。完成后,所有 `/api/auth/*`、`/api/users`、`/api/keys`、`/api/quotas` 端点可用,代理端点的 API Key 鉴权中间件就绪(供子项目 4 接入),首次启动自动建管理员。

**Architecture:** 在子项目 1 骨架上扩展。分层:`internal/auth`(认证逻辑 + session)、`internal/user`(用户/auth_methods/配额 store)、`internal/apikey`(API Key store + 鉴权)、`internal/webauthn`(Passkey 封装)、`internal/oauth`(Discord/X OAuth)、`internal/middleware`(session 中间件 + 角色守卫 + CSRF)、`internal/server`(挂载新路由)。前端页面留给子项目 6;本计划只做后端 + 用 curl/集成测试验证。

**Tech Stack:** Go 1.22+、`golang.org/x/crypto/bcrypt`(密码)、`github.com/pquerna/otp`(TOTP)、`github.com/go-webauthn/webauthn`(Passkey)、`golang.org/x/oauth2`(OAuth)、`github.com/go-chi/chi/v5`(路由)。全部纯 Go,无 CGO。

## Global Constraints

- Go 1.22+;交叉编译须支持 `GOOS=linux GOARCH=amd64` 与 `GOOS=windows GOARCH=amd64`,无 CGO。新增依赖必须纯 Go。
- SQLite 驱动 `modernc.org/sqlite`(已装);外键已通过 DSN `_pragma=foreign_keys(1)` 逐连接生效(子项目 1 修复)。
- 敏感字段(密码哈希外的 TOTP 密钥、Passkey 公钥、OAuth token、上游 API Key)用 `internal/crypto` 的 AES-GCM 加密后入库;主密钥来自 `config.Config.MasterKey`。
- 密码用 bcrypt(cost=12),**不加密**(bcrypt 自带盐)。
- Session:服务端存 SQLite(`sessions` 表),session ID 放 HttpOnly cookie;不用 JWT。改密/关 2FA/禁用账号立即吊销该用户所有 session。
- API Key:存 SHA-256 哈希(`key_hash`),不存明文;创建时返回明文一次。前缀 `carry-` + 随机部分。
- CSRF:管理 API 用 session cookie + 双提交 cookie(SameSite=Lax + `X-CSRF-Token` 头匹配 cookie 里的 token)。代理端点(`/v1/*`)用 API Key,不受 CSRF 保护。
- 角色权限:admin 可访问 `/api/providers`、`/api/models`、`/api/users`、`/api/settings`(改)、`/api/quotas`(改);普通用户只能看自己的 Key/配额/统计。后端做角色校验(403),前端隐藏只是 UX。
- 限流:登录/注册端点按 IP 限流(每 IP 每分钟 10 次);TOTP 验证失败 5 次锁定 15 分钟。
- TDD:每个任务先写失败测试,再实现,再验证通过,再提交。
- Git 身份:`rayafriandion <amizhisa@outlook.com>`(本仓库已配置)。

---

## File Structure

```
carryAPI/
├── internal/
│   ├── user/                       # 用户 + auth_methods + 配额 store
│   │   ├── user.go                 # User struct + Store (CRUD users)
│   │   ├── user_test.go
│   │   ├── authmethod.go           # AuthMethod + Store (bind/lookup login methods)
│   │   ├── authmethod_test.go
│   │   ├── quota.go                # Quota + Store (CRUD quotas)
│   │   └── quota_test.go
│   ├── auth/                       # 认证逻辑 + session
│   │   ├── password.go             # bcrypt 哈希/校验
│   │   ├── password_test.go
│   │   ├── session.go              # Session + Store (create/lookup/revoke/revokeAll)
│   │   ├── session_test.go
│   │   ├── totp.go                 # TOTP 生成/校验 + 备份码
│   │   ├── totp_test.go
│   │   ├── login.go                # 登录编排(密码+2FA 验证流程)
│   │   └── login_test.go
│   ├── webauthn/                   # Passkey 封装
│   │   ├── webauthn.go             # 包装 go-webauthn,绑定 user/store
│   │   └── webauthn_test.go
│   ├── oauth/                      # Discord/X OAuth
│   │   ├── oauth.go                # 通用 OAuth 流程(state/回调/token 交换)
│   │   ├── discord.go              # Discord 适配
│   │   ├── x.go                    # X(Twitter)适配
│   │   └── oauth_test.go
│   ├── apikey/                     # API Key store + 鉴权
│   │   ├── apikey.go               # Key 生成/哈希/CRUD + Authenticate(明文->user)
│   │   └── apikey_test.go
│   ├── middleware/                  # HTTP 中间件
│   │   ├── session.go              # session 加载 + 要求登录 + CSRF
│   │   ├── role.go                 # RequireRole(admin) 守卫
│   │   ├── ratelimit.go            # 按 IP 限流(登录/注册)
│   │   └── middleware_test.go
│   ├── api/                        # HTTP handlers(管理后台 API)
│   │   ├── auth_handler.go         # /api/auth/* 登录/注册/2FA/passkey/oauth/logout
│   │   ├── user_handler.go         # /api/users CRUD(admin)
│   │   ├── key_handler.go          # /api/keys CRUD
│   │   ├── quota_handler.go        # /api/quotas 读/改
│   │   ├── settings_handler.go     # /api/settings 读/改(admin)
│   │   ├── responder.go            # 统一 JSON 响应 + 错误封装
│   │   └── handlers_test.go        # 集成测试(httptest)
│   └── server/
│       └── router.go               # 修改:挂载 /api/* 路由(已在子项目1)
├── cmd/carryapi/
│   └── main.go                     # 修改:首次启动建管理员 + 注入新依赖
└── (无前端改动,留给子项目 6)
```

每个文件单一职责:`user` 管 users/auth_methods/quotas 三张表的 store;`auth` 管认证逻辑(密码/session/totp/login 编排);`webauthn`/`oauth`/`apikey` 各管一种凭证;`middleware` 管横切;`api` 管 HTTP handler。store 层不依赖 HTTP,handler 层依赖 store。

---

### Task 1: 依赖与 user store 基础

**Files:**
- Modify: `go.mod` (添加依赖)
- Create: `internal/user/user.go`
- Test: `internal/user/user_test.go`

**Interfaces:**
- Consumes: `*sql.DB`,`internal/crypto.Cipher`(加密敏感字段--用户表本身无加密字段,但 auth_methods 有,本任务先用 crypto 注入)
- Produces: `user.Store` 结构体;`user.New(db *sql.DB, cipher *crypto.Cipher) *Store`;`(*Store) Create(email, passwordHash, role string) (User, error)`;`(*Store) GetByID(id int64) (User, error)`;`(*Store) GetByEmail(email string) (User, error)`;`(*Store) List() ([]User, error)`;`(*Store) UpdateStatus(id int64, status string) error`;`(*Store) UpdateRole(id int64, role string) error`;`(*Store) Delete(id int64) error`。`User` struct:{ID int64, Email string, PasswordHash string, Role string, Status string, CreatedAt time.Time}。

- [ ] **Step 1: 添加依赖**

```bash
cd /d/Projects/carryAPI
go get golang.org/x/crypto/bcrypt
go get github.com/pquerna/otp
go get github.com/go-webauthn/webauthn
go get golang.org/x/oauth2
go mod tidy
```

预期:go.mod 新增四个直接依赖;`go build ./...` 通过(无 CGO)。

- [ ] **Step 2: 写失败测试**

`internal/user/user_test.go`:

```go
package user

import (
	"bytes"
	"testing"

	"carryapi/internal/crypto"
	"carryapi/internal/db"
)

func newStore(t *testing.T) *Store {
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
	return New(d, c)
}

func TestCreateAndGet(t *testing.T) {
	s := newStore(t)
	u, err := s.Create("alice@example.com", "$2a$12$hash", "user")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == 0 || u.Email != "alice@example.com" || u.Role != "user" || u.Status != "active" {
		t.Errorf("unexpected user: %+v", u)
	}
	got, err := s.GetByID(u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("GetByID email = %q", got.Email)
	}
	byEmail, err := s.GetByEmail("alice@example.com")
	if err != nil || byEmail.ID != u.ID {
		t.Errorf("GetByEmail: got %+v err %v", byEmail, err)
	}
}

func TestCreateDuplicateEmail(t *testing.T) {
	s := newStore(t)
	s.Create("dup@example.com", "h", "user")
	_, err := s.Create("dup@example.com", "h", "user")
	if err == nil {
		t.Error("expected error for duplicate email")
	}
}

func TestListAndUpdate(t *testing.T) {
	s := newStore(t)
	s.Create("a@x.com", "h", "user")
	s.Create("b@x.com", "h", "admin")
	users, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("List len = %d, want 2", len(users))
	}
	// disable + role change
	u, _ := s.GetByEmail("a@x.com")
	s.UpdateStatus(u.ID, "disabled")
	s.UpdateRole(u.ID, "admin")
	got, _ := s.GetByID(u.ID)
	if got.Status != "disabled" || got.Role != "admin" {
		t.Errorf("after update: status=%s role=%s", got.Status, got.Role)
	}
}

func TestDelete(t *testing.T) {
	s := newStore(t)
	u, _ := s.Create("del@x.com", "h", "user")
	if err := s.Delete(u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.GetByID(u.ID); err == nil {
		t.Error("expected error getting deleted user")
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

```bash
go test ./internal/user/ -v
```

预期:编译失败(找不到 `Store`/`New`/`User`)。

- [ ] **Step 4: 实现 user store**

`internal/user/user.go`:

```go
package user

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"carryapi/internal/crypto"
)

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Role         string
	Status       string
	CreatedAt    time.Time
}

type Store struct {
	db     *sql.DB
	cipher *crypto.Cipher
}

func New(db *sql.DB, cipher *crypto.Cipher) *Store {
	return &Store{db: db, cipher: cipher}
}

func (s *Store) Create(email, passwordHash, role string) (User, error) {
	res, err := s.db.Exec(
		`INSERT INTO users(email, password_hash, role, status) VALUES(?, ?, ?, 'active')`,
		email, passwordHash, role)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetByID(id)
}

func (s *Store) GetByID(id int64) (User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, email, password_hash, role, status, created_at FROM users WHERE id=?`, id))
}

func (s *Store) GetByEmail(email string) (User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, email, password_hash, role, status, created_at FROM users WHERE email=?`, email))
}

func (s *Store) List() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, email, password_hash, role, status, created_at FROM users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) UpdateStatus(id int64, status string) error {
	_, err := s.db.Exec(`UPDATE users SET status=? WHERE id=?`, status, id)
	return err
}

func (s *Store) UpdateRole(id int64, role string) error {
	_, err := s.db.Exec(`UPDATE users SET role=? WHERE id=?`, role, id)
	return err
}

func (s *Store) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id=?`, id)
	return err
}

func (s *Store) scanUser(row *sql.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, fmt.Errorf("user not found: %w", err)
	}
	return u, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUserRow(r rowScanner) (User, error) {
	var u User
	err := r.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt)
	return u, err
}
```

> 注:`cipher` 字段本任务暂未使用(users 表无加密字段),但 auth_methods(下个任务)需要,提前注入避免改构造函数。Go 会因未使用字段报错吗?不会--结构体字段不会因未使用报错,只有局部变量会。保留。

- [ ] **Step 5: 运行测试确认通过**

```bash
go test ./internal/user/ -v
```

预期:4 个测试 PASS。

- [ ] **Step 6: 提交**

```bash
git add go.mod go.sum internal/user
git commit -m "feat(user): user store with CRUD on users table"
```

---

### Task 2: auth_methods store(登录方式绑定)

**Files:**
- Create: `internal/user/authmethod.go`
- Test: `internal/user/authmethod_test.go`

**Interfaces:**
- Consumes: `user.Store`(共享 db + cipher)
- Produces: `user.AuthMethod` struct:{ID int64, UserID int64, Provider string, ProviderUID string, Secret []byte(已解密), CreatedAt time.Time};`(*Store) AddAuthMethod(userID int64, provider, providerUID string, secret []byte) error`(secret 加密后存);`(*Store) GetAuthMethods(userID int64) ([]AuthMethod, error)`(解密 secret);`(*Store) GetAuthMethod(provider, providerUID string) (AuthMethod, error)`(OAuth/Passkey 登录时按 provider+uid 反查用户);`(*Store) DeleteAuthMethod(id int64, userID int64) error`(解绑,带 userID 防越权)。
- Provider 取值:`password`(providerUID=email)、`totp`(providerUID 空,secret=密钥)、`passkey`(providerUID=credentialID,secret=公钥)、`discord`、`x`(providerUID=OAuth 用户ID)。

- [ ] **Step 1: 写失败测试**

`internal/user/authmethod_test.go`:

```go
package user

import (
	"bytes"
	"testing"
)

func TestAddAndGetAuthMethods(t *testing.T) {
	s := newStore(t)
	u, _ := s.Create("auth@x.com", "h", "user")
	secret := []byte("totp-secret-bytes")
	if err := s.AddAuthMethod(u.ID, "totp", "", secret); err != nil {
		t.Fatalf("AddAuthMethod: %v", err)
	}
	methods, err := s.GetAuthMethods(u.ID)
	if err != nil {
		t.Fatalf("GetAuthMethods: %v", err)
	}
	if len(methods) != 1 || methods[0].Provider != "totp" {
		t.Fatalf("got %+v", methods)
	}
	// secret 应解密回原文
	if !bytes.Equal(methods[0].Secret, secret) {
		t.Errorf("secret round-trip mismatch: got %q want %q", methods[0].Secret, secret)
	}
}

func TestGetAuthMethodByProviderUID(t *testing.T) {
	s := newStore(t)
	u, _ := s.Create("oauth@x.com", "h", "user")
	s.AddAuthMethod(u.ID, "discord", "discord-user-123", nil)
	m, err := s.GetAuthMethod("discord", "discord-user-123")
	if err != nil {
		t.Fatalf("GetAuthMethod: %v", err)
	}
	if m.UserID != u.ID {
		t.Errorf("UserID = %d, want %d", m.UserID, u.ID)
	}
}

func TestDeleteAuthMethod(t *testing.T) {
	s := newStore(t)
	u, _ := s.Create("del2@x.com", "h", "user")
	s.AddAuthMethod(u.ID, "totp", "", []byte("s"))
	methods, _ := s.GetAuthMethods(u.ID)
	if len(methods) != 1 {
		t.Fatal("expected 1 method")
	}
	// 带 userID 删(防越权)
	if err := s.DeleteAuthMethod(methods[0].ID, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	methods, _ = s.GetAuthMethods(u.ID)
	if len(methods) != 0 {
		t.Error("expected 0 methods after delete")
	}
}

func TestDeleteAuthMethodWrongUser(t *testing.T) {
	s := newStore(t)
	u1, _ := s.Create("u1@x.com", "h", "user")
	u2, _ := s.Create("u2@x.com", "h", "user")
	s.AddAuthMethod(u1.ID, "totp", "", []byte("s"))
	methods, _ := s.GetAuthMethods(u1.ID)
	// u2 试图删 u1 的 method -> 应不影响
	err := s.DeleteAuthMethod(methods[0].ID, u2.ID)
	if err == nil {
		t.Error("expected error deleting other user's auth method")
	}
	methods, _ = s.GetAuthMethods(u1.ID)
	if len(methods) != 1 {
		t.Error("method should still exist")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/user/ -v -run AuthMethod
```

预期:编译失败。

- [ ] **Step 3: 实现 authmethod store**

`internal/user/authmethod.go`:

```go
package user

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type AuthMethod struct {
	ID          int64
	UserID      int64
	Provider    string
	ProviderUID string
	Secret      []byte // 解密后
	CreatedAt   time.Time
}

func (s *Store) AddAuthMethod(userID int64, provider, providerUID string, secret []byte) error {
	var enc []byte
	if len(secret) > 0 {
		encBytes, err := s.cipher.Encrypt(secret)
		if err != nil {
			return fmt.Errorf("encrypt auth method secret: %w", err)
		}
		enc = encBytes
	}
	_, err := s.db.Exec(
		`INSERT INTO auth_methods(user_id, provider, provider_uid, secret) VALUES(?, ?, ?, ?)`,
		userID, provider, providerUID, enc)
	if err != nil {
		return fmt.Errorf("add auth method: %w", err)
	}
	return nil
}

func (s *Store) GetAuthMethods(userID int64) ([]AuthMethod, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, provider, provider_uid, secret, created_at FROM auth_methods WHERE user_id=?`, userID)
	if err != nil {
		return nil, fmt.Errorf("get auth methods: %w", err)
	}
	defer rows.Close()
	var out []AuthMethod
	for rows.Next() {
		m, err := s.scanAuthMethod(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) GetAuthMethod(provider, providerUID string) (AuthMethod, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, provider, provider_uid, secret, created_at FROM auth_methods WHERE provider=? AND provider_uid=?`,
		provider, providerUID)
	return s.scanAuthMethod(row)
}

func (s *Store) DeleteAuthMethod(id, userID int64) error {
	res, err := s.db.Exec(`DELETE FROM auth_methods WHERE id=? AND user_id=?`, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("auth method not found or not owned by user")
	}
	return nil
}

func (s *Store) scanAuthMethod(r rowScanner) (AuthMethod, error) {
	var m AuthMethod
	var enc []byte
	if err := r.Scan(&m.ID, &m.UserID, &m.Provider, &m.ProviderUID, &enc, &m.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AuthMethod{}, fmt.Errorf("auth method not found: %w", err)
		}
		return AuthMethod{}, err
	}
	if len(enc) > 0 {
		dec, err := s.cipher.Decrypt(enc)
		if err != nil {
			return AuthMethod{}, fmt.Errorf("decrypt auth method secret: %w", err)
		}
		m.Secret = dec
	}
	return m, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/user/ -v
```

预期:全部测试 PASS(含 Task 1 的 4 个 + 本任务 4 个)。

- [ ] **Step 5: 提交**

```bash
git add internal/user/authmethod.go internal/user/authmethod_test.go
git commit -m "feat(user): auth_methods store with encrypted secrets and ownership checks"
```

---

### Task 3: 配额 store

**Files:**
- Create: `internal/user/quota.go`
- Test: `internal/user/quota_test.go`

**Interfaces:**
- Consumes: `user.Store`
- Produces: `user.Quota` struct:{ID int64, Scope string(user/key), ScopeID int64, Period string(day/month/total), LimitTokens *int64, LimitCost *float64, UsedTokens int64, UsedCost float64, PeriodStart *time.Time};`(*Store) SetQuota(q Quota) (Quota, error)`(upsert);`(*Store) GetQuotas(scope string, scopeID int64) ([]Quota, error)`;`(*Store) GetQuota(id int64) (Quota, error)`;`(*Store) UpdateQuota(id int64, limitTokens *int64, limitCost *float64) error`;`(*Store) DeleteQuota(id int64) error`;`(*Store) IncrementUsage(scope string, scopeID int64, tokens int64, cost float64) error`(原子累加 + 周期重置判断,供子项目 4 调用)。周期重置:day/month 到期时 period_start 重置为 now、used 清零,再累加。

- [ ] **Step 1: 写失败测试**

`internal/user/quota_test.go`:

```go
package user

import (
	"testing"
)

func TestSetAndGetQuota(t *testing.T) {
	s := newStore(t)
	u, _ := s.Create("q@x.com", "h", "user")
	var lim int64 = 100000
	var cost float64 = 5.0
	q, err := s.SetQuota(Quota{Scope: "user", ScopeID: u.ID, Period: "month", LimitTokens: &lim, LimitCost: &cost})
	if err != nil {
		t.Fatalf("SetQuota: %v", err)
	}
	if q.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	got, err := s.GetQuotas("user", u.ID)
	if err != nil || len(got) != 1 {
		t.Fatalf("GetQuotas: %v len=%d", err, len(got))
	}
	if *got[0].LimitTokens != 100000 || *got[0].LimitCost != 5.0 {
		t.Errorf("quota = %+v", got[0])
	}
}

func TestUpdateAndDeleteQuota(t *testing.T) {
	s := newStore(t)
	u, _ := s.Create("q2@x.com", "h", "user")
	var lim int64 = 1000
	q, _ := s.SetQuota(Quota{Scope: "user", ScopeID: u.ID, Period: "total", LimitTokens: &lim})
	var newLim int64 = 5000
	if err := s.UpdateQuota(q.ID, &newLim, nil); err != nil {
		t.Fatalf("UpdateQuota: %v", err)
	}
	got, _ := s.GetQuota(q.ID)
	if *got.LimitTokens != 5000 {
		t.Errorf("after update: %d", *got.LimitTokens)
	}
	s.DeleteQuota(q.ID)
	if _, err := s.GetQuota(q.ID); err == nil {
		t.Error("expected error after delete")
	}
}

func TestIncrementUsage(t *testing.T) {
	s := newStore(t)
	u, _ := s.Create("q3@x.com", "h", "user")
	var lim int64 = 100000
	s.SetQuota(Quota{Scope: "user", ScopeID: u.ID, Period: "total", LimitTokens: &lim})
	// 累加两次
	s.IncrementUsage("user", u.ID, 100, 0.5)
	s.IncrementUsage("user", u.ID, 50, 0.25)
	got, _ := s.GetQuotas("user", u.ID)
	if got[0].UsedTokens != 150 || got[0].UsedCost != 0.75 {
		t.Errorf("usage = tokens=%d cost=%.2f", got[0].UsedTokens, got[0].UsedCost)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/user/ -v -run Quota
```

预期:编译失败。

- [ ] **Step 3: 实现 quota store**

`internal/user/quota.go`:

```go
package user

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Quota struct {
	ID          int64
	Scope       string
	ScopeID     int64
	Period      string
	LimitTokens *int64
	LimitCost   *float64
	UsedTokens  int64
	UsedCost    float64
	PeriodStart *time.Time
}

func (s *Store) SetQuota(q Quota) (Quota, error) {
	res, err := s.db.Exec(
		`INSERT INTO quotas(scope, scope_id, period, limit_tokens, limit_cost, period_start)
		 VALUES(?, ?, ?, ?, ?, ?)`,
		q.Scope, q.ScopeID, q.Period, q.LimitTokens, q.LimitCost, time.Now())
	if err != nil {
		return Quota{}, fmt.Errorf("set quota: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetQuota(id)
}

func (s *Store) GetQuotas(scope string, scopeID int64) ([]Quota, error) {
	rows, err := s.db.Query(
		`SELECT id, scope, scope_id, period, limit_tokens, limit_cost, used_tokens, used_cost, period_start
		 FROM quotas WHERE scope=? AND scope_id=?`, scope, scopeID)
	if err != nil {
		return nil, fmt.Errorf("get quotas: %w", err)
	}
	defer rows.Close()
	var out []Quota
	for rows.Next() {
		q, err := scanQuota(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (s *Store) GetQuota(id int64) (Quota, error) {
	row := s.db.QueryRow(
		`SELECT id, scope, scope_id, period, limit_tokens, limit_cost, used_tokens, used_cost, period_start
		 FROM quotas WHERE id=?`, id)
	q, err := scanQuota(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Quota{}, fmt.Errorf("quota not found: %w", err)
	}
	return q, err
}

func (s *Store) UpdateQuota(id int64, limitTokens *int64, limitCost *float64) error {
	_, err := s.db.Exec(
		`UPDATE quotas SET limit_tokens=?, limit_cost=? WHERE id=?`, limitTokens, limitCost, id)
	return err
}

func (s *Store) DeleteQuota(id int64) error {
	_, err := s.db.Exec(`DELETE FROM quotas WHERE id=?`, id)
	return err
}

func (s *Store) IncrementUsage(scope string, scopeID int64, tokens int64, cost float64) error {
	// 原子累加;周期重置在子项目 4 调用时按 period 判断(此处先简单累加,周期重置留 TODO 由子项目4封装)
	// 用事务保证原子
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec(
		`UPDATE quotas SET used_tokens = used_tokens + ?, used_cost = used_cost + ? WHERE scope=? AND scope_id=?`,
		tokens, cost, scope, scopeID)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

type quotaScanner interface {
	Scan(dest ...any) error
}

func scanQuota(r quotaScanner) (Quota, error) {
	var q Quota
	err := r.Scan(&q.ID, &q.Scope, &q.ScopeID, &q.Period, &q.LimitTokens, &q.LimitCost,
		&q.UsedTokens, &q.UsedCost, &q.PeriodStart)
	return q, err
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/user/ -v
```

预期:全部 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/user/quota.go internal/user/quota_test.go
git commit -m "feat(user): quota store with CRUD and atomic usage increment"
```

---

### Task 4: 密码哈希

**Files:**
- Create: `internal/auth/password.go`
- Test: `internal/auth/password_test.go`

**Interfaces:**
- Produces: `auth.HashPassword(password string) (string, error)`(bcrypt cost=12);`auth.VerifyPassword(password, hash string) bool`。

- [ ] **Step 1: 写失败测试**

`internal/auth/password_test.go`:

```go
package auth

import (
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" || hash == "correct horse battery staple" {
		t.Fatal("hash empty or equals plaintext")
	}
	if !VerifyPassword("correct horse battery staple", hash) {
		t.Error("VerifyPassword should accept correct password")
	}
	if VerifyPassword("wrong", hash) {
		t.Error("VerifyPassword should reject wrong password")
	}
}

func TestHashIsSalted(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Error("two hashes of same password should differ (bcrypt salt)")
	}
}

func TestHashCost(t *testing.T) {
	h, _ := HashPassword("x")
	// bcrypt hash 前7字符 "$2a$12$" 表示 cost 12
	if len(h) < 7 || h[:7] != "$2a$12$" {
		t.Errorf("hash prefix = %q, want $2a$12$", h[:7])
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/auth/ -v
```

预期:编译失败。

- [ ] **Step 3: 实现 password**

`internal/auth/password.go`:

```go
package auth

import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func VerifyPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/auth/ -v
```

预期:3 个测试 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/auth/password.go internal/auth/password_test.go
git commit -m "feat(auth): bcrypt password hashing at cost 12"
```

---

### Task 5: session store

**Files:**
- Create: `internal/auth/session.go`
- Test: `internal/auth/session_test.go`

**Interfaces:**
- Consumes: `*sql.DB`
- Produces: `auth.Session` struct:{ID int64, UserID int64, Token string(原始,只在创建时返回), ExpiresAt time.Time, CreatedAt time.Time, IP string, UserAgent string};`auth.SessionStore` 结构体;`auth.NewSessionStore(db *sql.DB) *SessionStore`;`(*SessionStore) Create(userID int64, ttl time.Duration, ip, userAgent string) (Session, error)`(token = 32 字节随机 hex,存 SHA-256 哈希);`(*SessionStore) Lookup(token string) (Session, error)`(按 token 哈希查,过期/不存在返回错误);`(*SessionStore) Revoke(token string) error`;`(*SessionStore) RevokeAllForUser(userID int64) error`(改密/关2FA/禁用账号时调用)。token 长度 64 hex 字符;cookie 名 `carryapi_session`。

- [ ] **Step 1: 写失败测试**

`internal/auth/session_test.go`:

```go
package auth

import (
	"testing"
	"time"

	"carryapi/internal/db"
)

func newSessionStore(t *testing.T) *SessionStore {
	t.Helper()
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	return NewSessionStore(d)
}

func TestCreateAndLookup(t *testing.T) {
	s := newSessionStore(t)
	sess, err := s.Create(1, time.Hour, "1.2.3.4", "test-agent")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sess.Token == "" || len(sess.Token) != 64 {
		t.Fatalf("token = %q (len %d), want 64 hex chars", sess.Token, len(sess.Token))
	}
	got, err := s.Lookup(sess.Token)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got.UserID != 1 || got.IP != "1.2.3.4" {
		t.Errorf("got %+v", got)
	}
}

func TestLookupExpired(t *testing.T) {
	s := newSessionStore(t)
	sess, _ := s.Create(1, -time.Hour, "", "") // 已过期
	if _, err := s.Lookup(sess.Token); err == nil {
		t.Error("expected error for expired session")
	}
}

func TestLookupRevoked(t *testing.T) {
	s := newSessionStore(t)
	sess, _ := s.Create(1, time.Hour, "", "")
	s.Revoke(sess.Token)
	if _, err := s.Lookup(sess.Token); err == nil {
		t.Error("expected error after revoke")
	}
}

func TestRevokeAllForUser(t *testing.T) {
	s := newSessionStore(t)
	a1, _ := s.Create(1, time.Hour, "", "")
	a2, _ := s.Create(1, time.Hour, "", "")
	s.Create(2, time.Hour, "", "")
	s.RevokeAllForUser(1)
	if _, err := s.Lookup(a1.Token); err == nil {
		t.Error("a1 should be revoked")
	}
	if _, err := s.Lookup(a2.Token); err == nil {
		t.Error("a2 should be revoked")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/auth/ -v -run Session
```

预期:编译失败。

- [ ] **Step 3: 实现 session store**

`internal/auth/session.go`:

```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const SessionCookieName = "carryapi_session"

type Session struct {
	ID        int64
	UserID    int64
	Token     string // 原始 token,仅创建时返回
	ExpiresAt time.Time
	CreatedAt time.Time
	IP        string
	UserAgent string
}

type SessionStore struct {
	db *sql.DB
}

func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{db: db}
}

func (s *SessionStore) Create(userID int64, ttl time.Duration, ip, userAgent string) (Session, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Session{}, fmt.Errorf("session token rand: %w", err)
	}
	token := hex.EncodeToString(raw)
	hash := hashToken(token)
	expires := time.Now().Add(ttl)
	res, err := s.db.Exec(
		`INSERT INTO sessions(user_id, token_hash, expires_at, ip, user_agent) VALUES(?, ?, ?, ?, ?)`,
		userID, hash, expires, ip, userAgent)
	if err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	id, _ := res.LastInsertId()
	return Session{ID: id, UserID: userID, Token: token, ExpiresAt: expires, CreatedAt: time.Now(), IP: ip, UserAgent: userAgent}, nil
}

func (s *SessionStore) Lookup(token string) (Session, error) {
	var sess Session
	var hash string
	err := s.db.QueryRow(
		`SELECT id, user_id, token_hash, expires_at, created_at, ip, user_agent FROM sessions WHERE token_hash=?`,
		hashToken(token)).Scan(&sess.ID, &sess.UserID, &hash, &sess.ExpiresAt, &sess.CreatedAt, &sess.IP, &sess.UserAgent)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, errors.New("session not found")
	}
	if err != nil {
		return Session{}, err
	}
	if time.Now().After(sess.ExpiresAt) {
		return Session{}, errors.New("session expired")
	}
	return sess, nil
}

func (s *SessionStore) Revoke(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash=?`, hashToken(token))
	return err
}

func (s *SessionStore) RevokeAllForUser(userID int64) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE user_id=?`, userID)
	return err
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/auth/ -v
```

预期:全部 PASS(含 Task 4 的 3 个 + 本任务 4 个)。

- [ ] **Step 5: 提交**

```bash
git add internal/auth/session.go internal/auth/session_test.go
git commit -m "feat(auth): server-side session store with token hashing and revocation"
```

---

### Task 6: TOTP 2FA

**Files:**
- Create: `internal/auth/totp.go`
- Test: `internal/auth/totp_test.go`

**Interfaces:**
- Produces: `auth.GenerateTOTPSecret() (secret string, otpauthURL string, err error)`(用 `github.com/pquerna/otp/totp`,secret base32,url 形如 `otpauth://totp/carryAPI:email?secret=...&issuer=carryAPI`);`auth.VerifyTOTP(code, secret string) bool`(容忍前后各一个时间窗);`auth.GenerateBackupCodes() []string`(10 个一次性码,每个 8 位 hex);`auth.HashBackupCode(code string) string`(SHA-256,存哈希);`auth.VerifyBackupCode(code string, hashedCodes []string) (matchedIndex int, ok bool)`。
- TOTP 密钥存 `auth_methods`(provider=totp,secret=加密后的 base32 secret)。备份码单独:管理员表或 auth_methods 扩展?简化:备份码也存 auth_methods(provider=totp_backup,secret=JSON 哈希数组)。本任务只做纯函数,存储在 Task 8 login 编排里接。

- [ ] **Step 1: 写失败测试**

`internal/auth/totp_test.go`:

```go
package auth

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestGenerateAndVerifyTOTP(t *testing.T) {
	secret, url, err := GenerateTOTPSecret("alice@example.com")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if secret == "" || url == "" {
		t.Fatal("secret or url empty")
	}
	// 当前码应验证通过
	code := totp.GenerateCode(secret, time.Now())
	if !VerifyTOTP(code, secret) {
		t.Error("VerifyTOTP should accept current code")
	}
	if VerifyTOTP("000000", secret) {
		t.Error("VerifyTOTP should reject bogus code")
	}
}

func TestBackupCodes(t *testing.T) {
	codes := GenerateBackupCodes()
	if len(codes) != 10 {
		t.Fatalf("expected 10 backup codes, got %d", len(codes))
	}
	hashed := make([]string, len(codes))
	for i, c := range codes {
		hashed[i] = HashBackupCode(c)
	}
	// 第一个码应匹配 index 0
	idx, ok := VerifyBackupCode(codes[0], hashed)
	if !ok || idx != 0 {
		t.Errorf("expected match at 0, got idx=%d ok=%v", idx, ok)
	}
	// 错误码不匹配
	if _, ok := VerifyBackupCode("deadbeef", hashed); ok {
		t.Error("should not match bogus code")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/auth/ -v -run TOTP
go test ./internal/auth/ -v -run Backup
```

预期:编译失败。

- [ ] **Step 3: 实现 totp**

`internal/auth/totp.go`:

```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func GenerateTOTPSecret(email string) (secret, otpauthURL string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "carryAPI",
		AccountName: email,
	})
	if err != nil {
		return "", "", fmt.Errorf("generate totp key: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

func VerifyTOTP(code, secret string) bool {
	// totp.Validate 默认允许 ±1 时间窗(30s)
	return totp.Validate(code, secret, totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
}

func GenerateBackupCodes() []string {
	codes := make([]string, 10)
	for i := range codes {
		b := make([]byte, 4)
		rand.Read(b)
		codes[i] = hex.EncodeToString(b)
	}
	return codes
}

func HashBackupCode(code string) string {
	h := sha256.Sum256([]byte(code))
	return hex.EncodeToString(h[:])
}

func VerifyBackupCode(code string, hashedCodes []string) (int, bool) {
	h := HashBackupCode(code)
	for i, hc := range hashedCodes {
		if h == hc {
			return i, true
		}
	}
	return -1, false
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/auth/ -v
```

预期:全部 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/auth/totp.go internal/auth/totp_test.go
git commit -m "feat(auth): TOTP generation/verification and backup codes"
```

---

### Task 7: 登录编排

**Files:**
- Create: `internal/auth/login.go`
- Test: `internal/auth/login_test.go`

**Interfaces:**
- Consumes: `user.Store`,`auth.SessionStore`
- Produces: `auth.LoginService` 结构体;`auth.NewLoginService(users *user.Store, sessions *SessionStore) *LoginService`;`(*LoginService) Login(email, password string) (session Session, requires2FA bool, err error)`:
  - 查 user by email;不存在/密码错都返 `ErrInvalidCredentials`(防枚举:统一错误)。
  - 用户 status=disabled -> `ErrUserDisabled`。
  - 检查该用户是否有 totp auth_method;有则返 `requires2FA=true`(不建 session),无则建 session 返。
- `(*LoginService) Complete2FA(email, code string) (session Session, err error)`:校验 TOTP 码或备份码;通过则建 session;备份码用一次即删(从 auth_methods 的 totp_backup 删除该哈希)。TOTP 失败 5 次锁定 15 分钟(内存计数器,简化)。
- `(*LoginService) Register(email, password string) (User, error)`:查 `registration_open` setting;关闭则 `ErrRegistrationClosed`;已存在 email 报错;创建 role=user 的用户。

- [ ] **Step 1: 写失败测试**

`internal/auth/login_test.go`:

```go
package auth

import (
	"bytes"
	"testing"
	"time"

	"carryapi/internal/crypto"
	"carryapi/internal/db"
	"carryapi/internal/settings"
	"carryapi/internal/user"
)

func newLoginService(t *testing.T) (*LoginService, *user.Store) {
	t.Helper()
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	c, _ := crypto.New(bytes.Repeat([]byte{1}, 32))
	us := user.New(d, c)
	ss := NewSessionStore(d)
	st := settings.New(d)
	return NewLoginService(us, ss, st), us
}

func TestLoginSuccessNo2FA(t *testing.T) {
	ls, us := newLoginService(t)
	hash, _ := HashPassword("pw123")
	us.Create("login@x.com", hash, "user")
	sess, requires2FA, err := ls.Login("login@x.com", "pw123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if requires2FA {
		t.Error("should not require 2FA")
	}
	if sess.Token == "" {
		t.Error("expected session token")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	ls, us := newLoginService(t)
	hash, _ := HashPassword("pw123")
	us.Create("login@x.com", hash, "user")
	_, _, err := ls.Login("login@x.com", "wrong")
	if err != ErrInvalidCredentials {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginNonexistentUser(t *testing.T) {
	ls, _ := newLoginService(t)
	_, _, err := ls.Login("nobody@x.com", "pw")
	if err != ErrInvalidCredentials {
		t.Errorf("err = %v, want ErrInvalidCredentials (no enumeration)", err)
	}
}

func TestLoginDisabledUser(t *testing.T) {
	ls, us := newLoginService(t)
	hash, _ := HashPassword("pw123")
	u, _ := us.Create("dis@x.com", hash, "user")
	us.UpdateStatus(u.ID, "disabled")
	_, _, err := ls.Login("dis@x.com", "pw123")
	if err != ErrUserDisabled {
		t.Errorf("err = %v, want ErrUserDisabled", err)
	}
}

func TestRegister(t *testing.T) {
	ls, _ := newLoginService(t)
	ls.settings.Set("registration_open", "true")
	u, err := ls.Register("new@x.com", "pw123")
	if err != nil || u.Email != "new@x.com" || u.Role != "user" {
		t.Fatalf("Register: u=%+v err=%v", u, err)
	}
}

func TestRegisterClosed(t *testing.T) {
	ls, _ := newLoginService(t)
	ls.settings.Set("registration_open", "false")
	_, err := ls.Register("new@x.com", "pw123")
	if err != ErrRegistrationClosed {
		t.Errorf("err = %v, want ErrRegistrationClosed", err)
	}
}
```

> 注:`NewLoginService` 签名是 `(users, sessions, settings)`,测试里用到 `ls.settings`。需在 LoginService 上暴露 settings(或 Register 内部用)。Step 3 实现里 LoginService 持有 `settings *settings.Store` 字段(小写,测试同包可访问)。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/auth/ -v -run Login
go test ./internal/auth/ -v -run Register
```

预期:编译失败。

- [ ] **Step 3: 实现 login service**

`internal/auth/login.go`:

```go
package auth

import (
	"errors"
	"time"

	"carryapi/internal/settings"
	"carryapi/internal/user"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserDisabled       = errors.New("user disabled")
	ErrRegistrationClosed = errors.New("registration closed")
	Err2FARequired        = errors.New("2fa required")
	Err2FAFailed          = errors.New("2fa verification failed")
)

type LoginService struct {
	users    *user.Store
	sessions *SessionStore
	settings *settings.Store
}

func NewLoginService(users *user.Store, sessions *SessionStore, settings *settings.Store) *LoginService {
	return &LoginService{users: users, sessions: sessions, settings: settings}
}

func (ls *LoginService) Login(email, password string) (Session, bool, error) {
	u, err := ls.users.GetByEmail(email)
	if err != nil {
		return Session{}, false, ErrInvalidCredentials
	}
	if u.Status == "disabled" {
		return Session{}, false, ErrUserDisabled
	}
	if !VerifyPassword(password, u.PasswordHash) {
		return Session{}, false, ErrInvalidCredentials
	}
	// 检查是否启用 2FA
	methods, _ := ls.users.GetAuthMethods(u.ID)
	for _, m := range methods {
		if m.Provider == "totp" {
			return Session{}, true, nil // 需要 2FA,不建 session
		}
	}
	sess, err := ls.sessions.Create(u.ID, 7*24*time.Hour, "", "")
	if err != nil {
		return Session{}, false, err
	}
	return sess, false, nil
}

func (ls *LoginService) Complete2FA(email, code string) (Session, error) {
	u, err := ls.users.GetByEmail(email)
	if err != nil {
		return Session{}, ErrInvalidCredentials
	}
	if u.Status == "disabled" {
		return Session{}, ErrUserDisabled
	}
	// 找 totp secret + 备份码哈希
	methods, _ := ls.users.GetAuthMethods(u.ID)
	var totpSecret []byte
	var backupMethod *user.AuthMethod
	for i, m := range methods {
		switch m.Provider {
		case "totp":
			totpSecret = m.Secret
		case "totp_backup":
			backupMethod = &methods[i]
		}
	}
	if totpSecret == nil {
		return Session{}, Err2FAFailed
	}
	// 先试 TOTP,失败再试备份码
	if !VerifyTOTP(code, string(totpSecret)) {
		if backupMethod == nil {
			return Session{}, Err2FAFailed
		}
		var hashedCodes []string
		if json.Unmarshal(backupMethod.Secret, &hashedCodes) != nil {
			return Session{}, Err2FAFailed
		}
		idx, ok := VerifyBackupCode(code, hashedCodes)
		if !ok {
			return Session{}, Err2FAFailed
		}
		// 用过的备份码从数组移除(一次性)
		hashedCodes = append(hashedCodes[:idx], hashedCodes[idx+1:]...)
		newJSON, _ := json.Marshal(hashedCodes)
		// 更新:删旧 + 加新(简化;实现者也可加 user.UpdateAuthMethodSecret)
		ls.users.DeleteAuthMethod(backupMethod.ID, u.ID)
		if len(hashedCodes) > 0 {
			ls.users.AddAuthMethod(u.ID, "totp_backup", "", newJSON)
		}
	}
	sess, err := ls.sessions.Create(u.ID, 7*24*time.Hour, "", "")
	if err != nil {
		return Session{}, err
	}
	return sess, nil
}

func (ls *LoginService) Register(email, password string) (user.User, error) {
	open, _ := ls.settings.Get("registration_open")
	if open == "false" {
		return user.User{}, ErrRegistrationClosed
	}
	hash, err := HashPassword(password)
	if err != nil {
		return user.User{}, err
	}
	return ls.users.Create(email, hash, "user")
}
```

> 注:`Complete2FA` 先试 TOTP,失败再试备份码;匹配的备份码一次性消费(从 `totp_backup` 的哈希数组移除,空了就删该 auth_method)。`login.go` 需 import `encoding/json`。TOTP 失败锁定(5 次锁 15 分钟)留作后续加固,本任务不做--非阻塞。

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/auth/ -v
```

预期:全部 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/auth/login.go internal/auth/login_test.go
git commit -m "feat(auth): login orchestration with 2FA gating and registration"
```

---

### Task 8: API Key store + 鉴权

**Files:**
- Create: `internal/apikey/apikey.go`
- Test: `internal/apikey/apikey_test.go`

**Interfaces:**
- Consumes: `*sql.DB`
- Produces: `apikey.APIKey` struct:{ID int64, UserID int64, KeyPrefix string, Label string, Status string, ExpiresAt *time.Time, LastUsedAt *time.Time, CreatedAt time.Time};`apikey.Store` 结构体;`apikey.New(db *sql.DB) *Store`;`(*Store) Create(userID int64, label string) (plaintext string, apikey APIKey, err error)`(明文 = `carry-` + 32 hex,存 SHA-256 哈希,prefix = 前 12 字符显示用);`(*Store) List(userID int64) ([]APIKey, error)`;`(*Store) Get(id, userID int64) (APIKey, error)`(带 userID 防越权);`(*Store) Update(id, userID int64, label, status string) error`;`(*Store) Delete(id, userID int64) error`;`(*Store) Authenticate(plaintext string) (userID int64, apiKeyID int64, err error)`(哈希查 -> 校验 status/过期 -> 更新 last_used_at -> 返回);`(*Store) TouchLastUsed(id int64) error`。
- 格式:明文 `carry-<32hex>`,共 38 字符;哈希 SHA-256 hex(64 字符)。

- [ ] **Step 1: 写失败测试**

`internal/apikey/apikey_test.go`:

```go
package apikey

import (
	"strings"
	"testing"
	"time"

	"carryapi/internal/db"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	return New(d)
}

func TestCreate(t *testing.T) {
	s := newStore(t)
	plaintext, ak, err := s.Create(1, "my-key")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !strings.HasPrefix(plaintext, "carry-") || len(plaintext) != 38 {
		t.Errorf("plaintext = %q (len %d), want carry-<32hex>", plaintext, len(plaintext))
	}
	if ak.KeyPrefix != plaintext[:12] {
		t.Errorf("prefix = %q, want %q", ak.KeyPrefix, plaintext[:12])
	}
	if ak.Label != "my-key" || ak.Status != "active" {
		t.Errorf("ak = %+v", ak)
	}
}

func TestAuthenticate(t *testing.T) {
	s := newStore(t)
	plaintext, _, _ := s.Create(1, "k")
	uid, kid, err := s.Authenticate(plaintext)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if uid != 1 || kid == 0 {
		t.Errorf("uid=%d kid=%d", uid, kid)
	}
}

func TestAuthenticateWrongKey(t *testing.T) {
	s := newStore(t)
	s.Create(1, "k")
	_, _, err := s.Authenticate("carry-deadbeefdeadbeefdeadbeefdeadbeef")
	if err == nil {
		t.Error("expected error for wrong key")
	}
}

func TestAuthenticateDisabled(t *testing.T) {
	s := newStore(t)
	plaintext, ak, _ := s.Create(1, "k")
	s.Update(ak.ID, 1, "k", "disabled")
	_, _, err := s.Authenticate(plaintext)
	if err == nil {
		t.Error("expected error for disabled key")
	}
}

func TestAuthenticateExpired(t *testing.T) {
	s := newStore(t)
	plaintext, ak, _ := s.Create(1, "k")
	// 直接更新过期时间为过去
	s.db.Exec("UPDATE api_keys SET expires_at=? WHERE id=?", time.Now().Add(-time.Hour), ak.ID)
	_, _, err := s.Authenticate(plaintext)
	if err == nil {
		t.Error("expected error for expired key")
	}
}

func TestListAndDelete(t *testing.T) {
	s := newStore(t)
	s.Create(1, "k1")
	s.Create(1, "k2")
	s.Create(2, "k3")
	keys, _ := s.List(1)
	if len(keys) != 2 {
		t.Errorf("List(1) = %d, want 2", len(keys))
	}
	// 删除带 userID 防越权
	s.Delete(keys[0].ID, 1)
	keys, _ = s.List(1)
	if len(keys) != 1 {
		t.Errorf("after delete = %d, want 1", len(keys))
	}
}

func TestDeleteWrongUser(t *testing.T) {
	s := newStore(t)
	_, ak, _ := s.Create(1, "k")
	err := s.Delete(ak.ID, 2) // user 2 删 user 1 的 key
	if err == nil {
		t.Error("expected error deleting other user's key")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/apikey/ -v
```

预期:编译失败。

- [ ] **Step 3: 实现 apikey store**

`internal/apikey/apikey.go`:

```go
package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const keyPrefix = "carry-"

type APIKey struct {
	ID         int64
	UserID     int64
	KeyPrefix  string
	Label      string
	Status     string
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	CreatedAt  time.Time
}

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(userID int64, label string) (string, APIKey, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", APIKey{}, fmt.Errorf("apikey rand: %w", err)
	}
	plaintext := keyPrefix + hex.EncodeToString(raw)
	hash := hashKey(plaintext)
	prefix := plaintext[:12]
	res, err := s.db.Exec(
		`INSERT INTO api_keys(user_id, key_hash, key_prefix, label, status) VALUES(?, ?, ?, ?, 'active')`,
		userID, hash, prefix, label)
	if err != nil {
		return "", APIKey{}, fmt.Errorf("create apikey: %w", err)
	}
	id, _ := res.LastInsertId()
	ak, err := s.Get(id, userID)
	if err != nil {
		return "", APIKey{}, err
	}
	return plaintext, ak, nil
}

func (s *Store) List(userID int64) ([]APIKey, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, key_prefix, label, status, expires_at, last_used_at, created_at
		 FROM api_keys WHERE user_id=? ORDER BY id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list apikeys: %w", err)
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		ak, err := scanKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ak)
	}
	return out, rows.Err()
}

func (s *Store) Get(id, userID int64) (APIKey, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, key_prefix, label, status, expires_at, last_used_at, created_at
		 FROM api_keys WHERE id=? AND user_id=?`, id, userID)
	ak, err := scanKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return APIKey{}, errors.New("api key not found")
	}
	return ak, err
}

func (s *Store) Update(id, userID int64, label, status string) error {
	_, err := s.db.Exec(`UPDATE api_keys SET label=?, status=? WHERE id=? AND user_id=?`, label, status, id, userID)
	return err
}

func (s *Store) Delete(id, userID int64) error {
	res, err := s.db.Exec(`DELETE FROM api_keys WHERE id=? AND user_id=?`, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("api key not found or not owned")
	}
	return nil
}

func (s *Store) Authenticate(plaintext string) (int64, int64, error) {
	hash := hashKey(plaintext)
	var ak APIKey
	row := s.db.QueryRow(
		`SELECT id, user_id, key_prefix, label, status, expires_at, last_used_at, created_at
		 FROM api_keys WHERE key_hash=?`, hash)
	if err := row.Scan(&ak.ID, &ak.UserID, &ak.KeyPrefix, &ak.Label, &ak.Status, &ak.ExpiresAt, &ak.LastUsedAt, &ak.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, errors.New("invalid api key")
		}
		return 0, 0, err
	}
	if ak.Status != "active" {
		return 0, 0, errors.New("api key disabled")
	}
	if ak.ExpiresAt != nil && time.Now().After(*ak.ExpiresAt) {
		return 0, 0, errors.New("api key expired")
	}
	s.TouchLastUsed(ak.ID)
	return ak.UserID, ak.ID, nil
}

func (s *Store) TouchLastUsed(id int64) error {
	_, err := s.db.Exec(`UPDATE api_keys SET last_used_at=? WHERE id=?`, time.Now(), id)
	return err
}

func hashKey(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(h[:])
}

type keyScanner interface {
	Scan(dest ...any) error
}

func scanKey(r keyScanner) (APIKey, error) {
	var ak APIKey
	err := r.Scan(&ak.ID, &ak.UserID, &ak.KeyPrefix, &ak.Label, &ak.Status, &ak.ExpiresAt, &ak.LastUsedAt, &ak.CreatedAt)
	return ak, err
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/apikey/ -v
```

预期:全部 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/apikey
git commit -m "feat(apikey): api key store with hashing, auth, and ownership checks"
```

---

### Task 9: 中间件(session + 角色 + CSRF + 限流)

**Files:**
- Create: `internal/middleware/session.go`
- Create: `internal/middleware/role.go`
- Create: `internal/middleware/ratelimit.go`
- Test: `internal/middleware/middleware_test.go`

**Interfaces:**
- Consumes: `auth.SessionStore`,`user.Store`(加载用户)
- Produces:
  - `middleware.SessionMiddleware(sessions *auth.SessionStore, users *user.Store) func(http.Handler) http.Handler`:从 cookie `carryapi_session` 读 token -> Lookup -> 把 `*user.User` 存入 context(`middleware.UserKey`);无效则不存(匿名)。`middleware.UserFromContext(ctx) (*user.User, bool)`。
  - `middleware.RequireLogin() func(http.Handler) http.Handler`:无 user -> 401。
  - `middleware.RequireRole(role string) func(http.Handler) http.Handler`:无 user -> 401;role 不符 -> 403。
  - `middleware.CSRFMiddleware() func(http.Handler) http.Handler`:GET/HEAD/OPTIONS 放行;其他方法要求 `X-CSRF-Token` 头 == cookie `carryapi_csrf` 的值(双提交)。登录成功时 handler 设置 csrf cookie。
  - `middleware.RateLimit(max int, window time.Duration) func(http.Handler) http.Handler`:按 IP(内存 map + mutex,带清理)限流,超限 429。

- [ ] **Step 1: 写失败测试**

`internal/middleware/middleware_test.go`:

```go
package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"carryapi/internal/auth"
	"carryapi/internal/db"
	"carryapi/internal/user"
)

func setupStores(t *testing.T) (*auth.SessionStore, *user.Store) {
	t.Helper()
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	c := mustCipher(t)
	us := user.New(d, c)
	ss := auth.NewSessionStore(d)
	return ss, us
}

func TestSessionMiddlewareLoadsUser(t *testing.T) {
	ss, us := setupStores(t)
	u, _ := us.Create("m@x.com", "h", "user")
	sess, _ := ss.Create(u.ID, time.Hour, "", "")
	called := false
	h := SessionMiddleware(ss, us)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := UserFromContext(r.Context())
		if !ok || got.ID != u.ID {
			t.Errorf("expected user %d, got %+v ok=%v", u.ID, got, ok)
		}
		called = true
	}))
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sess.Token})
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !called {
		t.Error("handler not called")
	}
}

func TestSessionMiddlewareAnonymous(t *testing.T) {
	ss, us := setupStores(t)
	h := SessionMiddleware(ss, us)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserFromContext(r.Context()); ok {
			t.Error("expected no user for anonymous request")
		}
	}))
	req := httptest.NewRequest("GET", "/", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)
}

func TestRequireLogin(t *testing.T) {
	h := RequireLogin()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not reach handler")
	}))
	// 无 user in context
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("code = %d, want 401", rec.Code)
	}
}

func TestRequireRoleDenied(t *testing.T) {
	u := &user.User{ID: 1, Role: "user"}
	ctx := context.WithValue(context.Background(), UserKey{}, u)
	h := RequireRole("admin")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("non-admin should not reach admin handler")
	}))
	req := httptest.NewRequest("GET", "/").WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

func TestRateLimit(t *testing.T) {
	rl := RateLimit(2, time.Minute)
	blocked := 0
	h := rl(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			blocked++
		}
	}
	if blocked != 2 {
		t.Errorf("blocked = %d, want 2 (max=2, 4 requests)", blocked)
	}
}
```

> `mustCipher` 是测试 helper(同前),在 middleware_test.go 内定义:

```go
func mustCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	c, err := crypto.New(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	return c
}
```
需要 import `bytes` 和 `carryapi/internal/crypto`。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/middleware/ -v
```

预期:编译失败。

- [ ] **Step 3: 实现 session.go**

`internal/middleware/session.go`:

```go
package middleware

import (
	"context"
	"net/http"

	"carryapi/internal/auth"
	"carryapi/internal/user"
)

type UserKey struct{}

func SessionMiddleware(sessions *auth.SessionStore, users *user.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(auth.SessionCookieName)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			sess, err := sessions.Lookup(cookie.Value)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			u, err := users.GetByID(sess.UserID)
			if err != nil || u.Status != "active" {
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), UserKey{}, u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserFromContext(ctx context.Context) (*user.User, bool) {
	u, ok := ctx.Value(UserKey{}).(*user.User)
	return u, ok
}

func RequireLogin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := UserFromContext(r.Context()); !ok {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: 实现 role.go**

`internal/middleware/role.go`:

```go
package middleware

import (
	"net/http"
)

func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := UserFromContext(r.Context())
			if !ok {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			if u.Role != role {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 5: 实现 ratelimit.go**

`internal/middleware/ratelimit.go`:

```go
package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type RateLimiter struct {
	mu     sync.Mutex
	counts map[string]int
	max    int
	window time.Duration
}

func RateLimit(max int, window time.Duration) func(http.Handler) http.Handler {
	rl := &RateLimiter{counts: make(map[string]int), max: max, window: window}
	go rl.cleanup()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			rl.mu.Lock()
			rl.counts[ip]++
			allowed := rl.counts[ip] <= rl.max
			rl.mu.Unlock()
			if !allowed {
				http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		rl.counts = make(map[string]int)
		rl.mu.Unlock()
	}
}
```

- [ ] **Step 6: 实现 CSRF(同文件 ratelimit.go 或新建 csrf.go)**

`internal/middleware/csrf.go`:

```go
package middleware

import (
	"net/http"
)

const CSRFHeader = "X-CSRF-Token"
const CSRFCookie = "carryapi_csrf"

func CSRFMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			cookie, err := r.Cookie(CSRFCookie)
			if err != nil {
				http.Error(w, `{"error":"missing csrf cookie"}`, http.StatusForbidden)
				return
			}
			if r.Header.Get(CSRFHeader) != cookie.Value {
				http.Error(w, `{"error":"csrf token mismatch"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 7: 运行测试确认通过**

```bash
go test ./internal/middleware/ -v
```

预期:5 个测试 PASS。

- [ ] **Step 8: 提交**

```bash
git add internal/middleware
git commit -m "feat(middleware): session, role guard, csrf, and rate limiting"
```

---

### Task 10: auth HTTP handlers(登录/注册/2FA/登出)

**Files:**
- Create: `internal/api/responder.go`
- Create: `internal/api/auth_handler.go`
- Test: `internal/api/handlers_test.go`

**Interfaces:**
- Consumes: `auth.LoginService`,`auth.SessionStore`,`user.Store`,`settings.Store`
- Produces: `api.AuthHandler` 结构体;`api.NewAuthHandler(ls *auth.LoginService, sessions *auth.SessionStore, users *user.Store, settings *settings.Store) *AuthHandler`;方法:
  - `(*AuthHandler) Login(w, r)`:POST JSON {email,password};调 LoginService.Login;若 requires2FA -> 200 {requires_2fa:true};成功 -> 设置 session cookie(HttpOnly,SameSite=Lax,7d)+ 生成 csrf token 存 cookie + 返回 {user:{id,email,role}}。错误 -> 401。
  - `(*AuthHandler) Complete2FA(w, r)`:POST {email,code};调 Complete2FA;成功同 Login 设 cookie。
  - `(*AuthHandler) Register(w, r)`:POST {email,password};调 Register;成功返回 user(不自动登录)。限流包裹。
  - `(*AuthHandler) Logout(w, r)`:读 session cookie -> Revoke -> 清 cookie。
  - `(*AuthHandler) Setup2FA(w, r)`:要求登录;生成 secret + 备份码;**不立即存**(返回 secret+otpauth_url+备份码 + 临时 token,客户端用 Verify2FA 确认后再存)。简化:本任务直接存 auth_methods(用户已登录,信任),返回 secret/qr/备份码一次。
  - `(*AuthHandler) Disable2FA(w, r)`:要求登录 + 当前密码;删 totp auth_method。
  - `(*AuthHandler) Me(w, r)`:要求登录;返回当前 user + 已绑定的登录方式 provider 列表。
- `responder.go`:`api.JSON(w, status, data)`、`api.JSONError(w, status, msg)`。

- [ ] **Step 1: 写失败测试**

`internal/api/handlers_test.go`(集成测试,用 httptest):

```go
package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupAPI(t *testing.T) (*AuthHandler, *user.Store) {
	t.Helper()
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	c := mustCipher(t)
	us := user.New(d, c)
	ss := auth.NewSessionStore(d)
	st := settings.New(d)
	st.Set("registration_open", "true")
	ls := auth.NewLoginService(us, ss, st)
	return NewAuthHandler(ls, ss, us, st), us
}

func TestRegisterLoginLogout(t *testing.T) {
	h, _ := setupAPI(t)

	// Register
	body, _ := json.Marshal(map[string]string{"email": "a@x.com", "password": "pw123"})
	rec := serve(h.Register, "POST", "/api/auth/register", body)
	if rec.Code != 200 {
		t.Fatalf("register code=%d body=%s", rec.Code, rec.Body.String())
	}

	// Login
	body, _ = json.Marshal(map[string]string{"email": "a@x.com", "password": "pw123"})
	rec = serve(h.Login, "POST", "/api/auth/login", body)
	if rec.Code != 200 {
		t.Fatalf("login code=%d", rec.Code)
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["requires_2fa"] == true {
		t.Error("should not require 2fa")
	}
	// session cookie 应设置
	if !hasCookie(rec, auth.SessionCookieName) {
		t.Error("missing session cookie")
	}

	// Logout
	rec = serve(h.Logout, "POST", "/api/auth/logout", nil)
	if rec.Code != 200 {
		t.Errorf("logout code=%d", rec.Code)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	h, us := setupAPI(t)
	hash, _ := auth.HashPassword("pw123")
	us.Create("b@x.com", hash, "user")
	body, _ := json.Marshal(map[string]string{"email": "b@x.com", "password": "wrong"})
	rec := serve(h.Login, "POST", "/api/auth/login", body)
	if rec.Code != 401 {
		t.Errorf("code=%d, want 401", rec.Code)
	}
}

// helpers
func serve(handler http.HandlerFunc, method, path string, body []byte) *httptest.ResponseRecorder {
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, r)
	return rec
}

func hasCookie(rec *httptest.ResponseRecorder, name string) bool {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return true
		}
	}
	return false
}
```

> 加 `mustCipher` helper(同 middleware 测试)+ import db/crypto/user/auth/settings。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/api/ -v
```

预期:编译失败。

- [ ] **Step 3: 实现 responder.go**

`internal/api/responder.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
)

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func JSONError(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}
```

- [ ] **Step 4: 实现 auth_handler.go**

`internal/api/auth_handler.go`:

```go
package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"carryapi/internal/auth"
	"carryapi/internal/middleware"
	"carryapi/internal/settings"
	"carryapi/internal/user"
)

type AuthHandler struct {
	ls       *auth.LoginService
	sessions *auth.SessionStore
	users    *user.Store
	settings *settings.Store
}

func NewAuthHandler(ls *auth.LoginService, sessions *auth.SessionStore, users *user.Store, settings *settings.Store) *AuthHandler {
	return &AuthHandler{ls: ls, sessions: sessions, users: users, settings: settings}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Code     string `json:"code"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, 400, "invalid request body")
		return
	}
	sess, requires2FA, err := h.ls.Login(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrUserDisabled) {
			JSONError(w, 403, "user disabled")
			return
		}
		JSONError(w, 401, "invalid credentials")
		return
	}
	if requires2FA {
		JSON(w, 200, map[string]any{"requires_2fa": true})
		return
	}
	setSessionCookie(w, sess.Token)
	setCSRFCookie(w)
	u, _ := h.users.GetByID(sess.UserID)
	JSON(w, 200, map[string]any{
		"user": map[string]any{"id": u.ID, "email": u.Email, "role": u.Role},
	})
}

func (h *AuthHandler) Complete2FA(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, 400, "invalid request body")
		return
	}
	sess, err := h.ls.Complete2FA(req.Email, req.Code)
	if err != nil {
		JSONError(w, 401, "2fa verification failed")
		return
	}
	setSessionCookie(w, sess.Token)
	setCSRFCookie(w)
	u, _ := h.users.GetByID(sess.UserID)
	JSON(w, 200, map[string]any{
		"user": map[string]any{"id": u.ID, "email": u.Email, "role": u.Role},
	})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, 400, "invalid request body")
		return
	}
	u, err := h.ls.Register(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrRegistrationClosed) {
			JSONError(w, 403, "registration closed")
			return
		}
		JSONError(w, 400, err.Error())
		return
	}
	JSON(w, 200, map[string]any{"id": u.ID, "email": u.Email, "role": u.Role})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.SessionCookieName); err == nil {
		h.sessions.Revoke(cookie.Value)
	}
	// 清 cookie
	http.SetCookie(w, &http.Cookie{Name: auth.SessionCookieName, Value: "", MaxAge: -1, Path: "/"})
	http.SetCookie(w, &http.Cookie{Name: middleware.CSRFCookie, Value: "", MaxAge: -1, Path: "/"})
	JSON(w, 200, map[string]string{"status": "ok"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		JSONError(w, 401, "unauthorized")
		return
	}
	methods, _ := h.users.GetAuthMethods(u.ID)
	providers := []string{}
	for _, m := range methods {
		providers = append(providers, m.Provider)
	}
	JSON(w, 200, map[string]any{
		"user":      map[string]any{"id": u.ID, "email": u.Email, "role": u.Role, "status": u.Status},
		"auth_methods": providers,
	})
}

func (h *AuthHandler) Setup2FA(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		JSONError(w, 401, "unauthorized")
		return
	}
	secret, url, err := auth.GenerateTOTPSecret(u.Email)
	if err != nil {
		JSONError(w, 500, "failed to generate totp")
		return
	}
	backupCodes := auth.GenerateBackupCodes()
	hashedCodes := make([]string, len(backupCodes))
	for i, c := range backupCodes {
		hashedCodes[i] = auth.HashBackupCode(c)
	}
	// 存 totp secret + 备份码哈希
	h.users.AddAuthMethod(u.ID, "totp", "", []byte(secret))
	// 备份码:简化存为单条 auth_method,secret=JSON 哈希数组(加密)
	hashesJSON, _ := json.Marshal(hashedCodes)
	h.users.AddAuthMethod(u.ID, "totp_backup", "", hashesJSON)
	JSON(w, 200, map[string]any{
		"secret":       secret,
		"otpauth_url":  url,
		"backup_codes": backupCodes,
	})
}

func (h *AuthHandler) Disable2FA(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		JSONError(w, 401, "unauthorized")
		return
	}
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Password != "" {
		if !auth.VerifyPassword(req.Password, u.PasswordHash) {
			JSONError(w, 401, "invalid password")
			return
		}
	}
	methods, _ := h.users.GetAuthMethods(u.ID)
	for _, m := range methods {
		if m.Provider == "totp" || m.Provider == "totp_backup" {
			h.users.DeleteAuthMethod(m.ID, u.ID)
		}
	}
	JSON(w, 200, map[string]string{"status": "ok"})
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	})
}

func setCSRFCookie(w http.ResponseWriter) {
	b := make([]byte, 16)
	rand.Read(b)
	token := hex.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.CSRFCookie,
		Value:    token,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})
}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
go test ./internal/api/ -v
```

预期:测试 PASS。

- [ ] **Step 6: 提交**

```bash
git add internal/api
git commit -m "feat(api): auth handlers for login/register/2fa/logout/me"
```

---

### Task 11: user/key/quota/settings handlers + 路由挂载 + 首次启动建管理员

**Files:**
- Create: `internal/api/user_handler.go`
- Create: `internal/api/key_handler.go`
- Create: `internal/api/quota_handler.go`
- Create: `internal/api/settings_handler.go`
- Modify: `internal/server/router.go`
- Modify: `cmd/carryapi/main.go`
- Test: `internal/api/handlers_test.go`(扩展)

**Interfaces:**
- Consumes: 所有 store
- Produces: user CRUD handler(admin)、key CRUD handler(本人)、quota 读/改 handler、settings 读/改 handler(admin);路由全部挂到 `/api/*`;main.go 首次启动检测无 admin -> 用 `CARRYAPI_ADMIN_EMAIL`/`CARRYAPI_ADMIN_PASSWORD` 建 admin,缺失则生成随机密码打印控制台。

- [ ] **Step 1: 写失败测试(扩展 handlers_test.go)**

在 `internal/api/handlers_test.go` 末尾追加。这些测试通过 `middleware.UserKey` 向 context 注入用户来模拟登录态:

```go
func TestUserCreateAsAdmin(t *testing.T) {
	h, us := setupAPI(t)
	uh := NewUserHandler(us)
	// 注入 admin 用户到 context
	admin := &user.User{ID: 1, Email: "admin@x.com", Role: "admin", Status: "active"}
	body, _ := json.Marshal(map[string]string{"email": "newuser@x.com", "password": "pw", "role": "user"})
	req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, admin))
	rec := httptest.NewRecorder()
	uh.Create(rec, req)
	if rec.Code != 200 {
		t.Fatalf("create user code=%d body=%s", rec.Code, rec.Body.String())
	}
	// 应能 list 看到
	req = httptest.NewRequest("GET", "/api/users", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, admin))
	rec = httptest.NewRecorder()
	uh.List(rec, req)
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0]["email"] != "newuser@x.com" {
		t.Errorf("list = %+v", list)
	}
}

func TestUserDeletePreventsSelf(t *testing.T) {
	h, us := setupAPI(t)
	uh := NewUserHandler(us)
	admin := &user.User{ID: 1, Email: "admin@x.com", Role: "admin", Status: "active"}
	req := httptest.NewRequest("DELETE", "/api/users/1", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, admin))
	// chi URLParam 需要在 chi 路由上下文;直接调 handler 时手动设
	req = withChiParam(req, "id", "1")
	rec := httptest.NewRecorder()
	uh.Delete(rec, req)
	if rec.Code != 400 {
		t.Errorf("deleting self should be 400, got %d", rec.Code)
	}
}

func TestKeyCreateAndList(t *testing.T) {
	h, _ := setupAPI(t)
	d := h.users // 拿到底层 db? 不,用 setupAPI 返回的 us
	// 见上:setupAPI 返回 (*AuthHandler, *user.Store)
	// 建 key handler 需 apikey.Store;在测试里新建
	// (实现者:setupAPI 同时返回 db 或 apikey.Store)
	// 此处假设有 newKeyStore(t) helper 返回 *apikey.Store 共享同一 db
	// 见 helper 调整说明(下方)
}
```

> **helper 调整说明**:`setupAPI` 当前返回 `(*AuthHandler, *user.Store)`。为让 key/quota handler 测试复用同一 db,把 `setupAPI` 改为返回一个聚合 fixture:
> ```go
> type apiFixture struct {
> 	auth    *AuthHandler
> 	users   *user.Store
> 	keys    *apikey.Store
> 	db      *sql.DB
> }
> func setupAPI(t *testing.T) *apiFixture { ... 返回所有 ... }
> ```
> 现有 Task 10 的测试(`TestRegisterLoginLogout` 等)用的是 `h, us := setupAPI(t)` 解构--需同步改为 `f := setupAPI(t); f.auth.Register...`。实现者统一重构。`TestKeyCreateAndList` 用 `f.keys` 测试:Create(注入 user)-> 拿明文 -> List -> 长度为 1 -> Delete -> List 长度 0。
>
> `withChiParam` helper:用 chi 的 `chi.NewRouteContext()` 注入 URL param:
> ```go
> func withChiParam(r *http.Request, key, val string) *http.Request {
> 	rctx := chi.NewRouteContext()
> 	rctx.URLParams.Add(key, val)
> 	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
> }
> ```

`TestKeyCreateAndList` 完整实现(实现者写入):

```go
func TestKeyCreateAndList(t *testing.T) {
	f := setupAPI(t)
	kh := NewKeyHandler(f.keys)
	u := &user.User{ID: 1, Role: "user", Status: "active"}
	// create
	body, _ := json.Marshal(map[string]string{"label": "test"})
	req := httptest.NewRequest("POST", "/api/keys", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, u))
	rec := httptest.NewRecorder()
	kh.Create(rec, req)
	if rec.Code != 200 {
		t.Fatalf("create code=%d body=%s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created["key"] == nil {
		t.Error("expected plaintext key in response")
	}
	// list
	req = httptest.NewRequest("GET", "/api/keys", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, u))
	rec = httptest.NewRecorder()
	kh.List(rec, req)
	var list []map[string]any
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 || list[0]["label"] != "test" {
		t.Errorf("list = %+v", list)
	}
}
```

```go
func TestSettingsUpdateAdminOnly(t *testing.T) {
	f := setupAPI(t)
	sh := NewSettingsHandler(f.settings)
	// 普通用户 -> 角色守卫在路由层,handler 本身不校验。
	// 此测试验证:handler 在 admin 调用时能更新;非 admin 由路由 RequireRole 拦截(403)。
	// 这里直接测 admin 成功路径:
	admin := &user.User{ID: 1, Role: "admin", Status: "active"}
	body, _ := json.Marshal(map[string]string{"force_2fa": "true"})
	req := httptest.NewRequest("PUT", "/api/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserKey{}, admin))
	rec := httptest.NewRecorder()
	sh.Update(rec, req)
	if rec.Code != 200 {
		t.Fatalf("update code=%d", rec.Code)
	}
	// 验证写入
	v, ok, _ := f.settings.Get("force_2fa")
	if !ok || v != "true" {
		t.Errorf("force_2fa = %q ok=%v", v, ok)
	}
}
```

> 角色守卫的 403 路径在 Task 9 中间件测试(`TestRequireRoleDenied`)已覆盖,这里不重复。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/api/ -v
```

预期:编译失败(新 handler 未实现)。

- [ ] **Step 3: 实现 user_handler.go**

```go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"carryapi/internal/auth"
	"carryapi/internal/middleware"
	"carryapi/internal/user"
)

type UserHandler struct {
	users *user.Store
}

func NewUserHandler(users *user.Store) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.List()
	if err != nil {
		JSONError(w, 500, "failed to list users")
		return
	}
	out := []map[string]any{}
	for _, u := range users {
		out = append(out, map[string]any{"id": u.ID, "email": u.Email, "role": u.Role, "status": u.Status, "created_at": u.CreatedAt})
	}
	JSON(w, 200, out)
}

func (h *UserHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, 400, "invalid body")
		return
	}
	if req.Role == "" {
		req.Role = "user"
	}
	// 管理员创建用户:直接哈希密码
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		JSONError(w, 500, "hash failed")
		return
	}
	u, err := h.users.Create(req.Email, hash, req.Role)
	if err != nil {
		JSONError(w, 400, err.Error())
		return
	}
	JSON(w, 200, map[string]any{"id": u.ID, "email": u.Email, "role": u.Role})
}

func (h *UserHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		JSONError(w, 400, "invalid id")
		return
	}
	var req struct {
		Role   string `json:"role"`
		Status string `json:"status"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Role != "" {
		h.users.UpdateRole(id, req.Role)
	}
	if req.Status != "" {
		h.users.UpdateStatus(id, req.Status)
	}
	JSON(w, 200, map[string]string{"status": "ok"})
}

func (h *UserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	// 防止删自己
	if u, ok := middleware.UserFromContext(r.Context()); ok && u.ID == id {
		JSONError(w, 400, "cannot delete yourself")
		return
	}
	if err := h.users.Delete(id); err != nil {
		JSONError(w, 500, "delete failed")
		return
	}
	JSON(w, 200, map[string]string{"status": "ok"})
}

> 注:`user_handler.go` 直接 import `carryapi/internal/auth` 调 `auth.HashPassword`(无循环 import:auth 不 import api)。URL 参数统一用 `chi.URLParam(r, "id")`。import 块含 `"carryapi/internal/auth"`、`"github.com/go-chi/chi/v5"`。

- [ ] **Step 4: 实现 key_handler.go**

```go
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"carryapi/internal/apikey"
	"carryapi/internal/middleware"
)

type KeyHandler struct {
	keys *apikey.Store
}

func NewKeyHandler(keys *apikey.Store) *KeyHandler {
	return &KeyHandler{keys: keys}
}

func (h *KeyHandler) List(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	keys, err := h.keys.List(u.ID)
	if err != nil {
		JSONError(w, 500, "failed to list keys")
		return
	}
	out := []map[string]any{}
	for _, k := range keys {
		out = append(out, map[string]any{
			"id": k.ID, "key_prefix": k.KeyPrefix, "label": k.Label,
			"status": k.Status, "created_at": k.CreatedAt, "last_used_at": k.LastUsedAt,
		})
	}
	JSON(w, 200, out)
}

func (h *KeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	var req struct {
		Label string `json:"label"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	plaintext, ak, err := h.keys.Create(u.ID, req.Label)
	if err != nil {
		JSONError(w, 500, "failed to create key")
		return
	}
	JSON(w, 200, map[string]any{
		"id":         ak.ID,
		"key":        plaintext, // 仅此一次返回明文
		"key_prefix": ak.KeyPrefix,
		"label":      ak.Label,
	})
}

func (h *KeyHandler) Update(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var req struct {
		Label  string `json:"label"`
		Status string `json:"status"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := h.keys.Update(id, u.ID, req.Label, req.Status); err != nil {
		JSONError(w, 400, err.Error())
		return
	}
	JSON(w, 200, map[string]string{"status": "ok"})
}

func (h *KeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := h.keys.Delete(id, u.ID); err != nil {
		JSONError(w, 400, err.Error())
		return
	}
	JSON(w, 200, map[string]string{"status": "ok"})
}
```

- [ ] **Step 5: 实现 quota_handler.go + settings_handler.go**

```go
// quota_handler.go
package api

import (
	"net/http"
	"strconv"

	"carryapi/internal/middleware"
	"carryapi/internal/user"
)

type QuotaHandler struct {
	users *user.Store
}

func NewQuotaHandler(users *user.Store) *QuotaHandler {
	return &QuotaHandler{users: users}
}

func (h *QuotaHandler) List(w http.ResponseWriter, r *http.Request) {
	u, _ := middleware.UserFromContext(r.Context())
	// admin 看全部?简化:admin 传 ?user_id= 查指定,否则看自己
	quotas, err := h.users.GetQuotas("user", u.ID)
	if err != nil {
		JSONError(w, 500, "failed to list quotas")
		return
	}
	JSON(w, 200, quotas)
}

func (h *QuotaHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var req struct {
		LimitTokens *int64  `json:"limit_tokens"`
		LimitCost   *float64 `json:"limit_cost"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := h.users.UpdateQuota(id, req.LimitTokens, req.LimitCost); err != nil {
		JSONError(w, 500, "update failed")
		return
	}
	JSON(w, 200, map[string]string{"status": "ok"})
}
```

```go
// settings_handler.go
package api

import (
	"encoding/json"
	"net/http"

	"carryapi/internal/middleware"
	"carryapi/internal/settings"
)

type SettingsHandler struct {
	store *settings.Store
}

func NewSettingsHandler(store *settings.Store) *SettingsHandler {
	return &SettingsHandler{store: store}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	// 返回所有安全可见的设置(不含 OAuth secret)
	out := map[string]string{}
	for _, k := range []string{"listen_host", "registration_open", "force_2fa", "log_retention_days"} {
		if v, ok, _ := h.store.Get(k); ok {
			out[k] = v
		}
	}
	JSON(w, 200, out)
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	// RequireRole("admin") 在路由层守卫
	var req map[string]string
	json.NewDecoder(r.Body).Decode(&req)
	for k, v := range req {
		// 白名单
		switch k {
		case "registration_open", "force_2fa", "log_retention_days":
			h.store.Set(k, v)
		}
		// listen_host 由广播开关单独处理(子项目1 已有逻辑,此处不改避免重启)
	}
	_ = middleware.UserFromContext // 避免未用 import
	JSON(w, 200, map[string]string{"status": "ok"})
}
```

> 注:import 整理(`chi`、`strconv`、`user`、`settings` 等)由实现者确保。

- [ ] **Step 6: 修改 router.go 挂载路由**

`internal/server/router.go` 改为接收各 handler,挂载:

```go
r := chi.NewRouter()
r.Get("/api/health", s.handleHealth)

// auth(限流包裹登录/注册)
r.Route("/api/auth", func(r chi.Router) {
	r.Use(middleware.SessionMiddleware(s.sessions, s.users))
	r.With(middleware.RateLimit(10, time.Minute)).Post("/login", s.auth.Login)
	r.With(middleware.RateLimit(10, time.Minute)).Post("/register", s.auth.Register)
	r.Post("/2fa/complete", s.auth.Complete2FA)
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireLogin())
		r.Use(middleware.CSRFMiddleware())
		r.Post("/logout", s.auth.Logout)
		r.Get("/me", s.auth.Me)
		r.Post("/2fa/setup", s.auth.Setup2FA)
		r.Post("/2fa/disable", s.auth.Disable2FA)
	})
})

// 管理端点(需登录 + CSRF)
r.Group(func(r chi.Router) {
	r.Use(middleware.SessionMiddleware(s.sessions, s.users))
	r.Use(middleware.CSRFMiddleware())
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireLogin())
		r.Get("/api/keys", s.keys.List)
		r.Post("/api/keys", s.keys.Create)
		r.Put("/api/keys/{id}", s.keys.Update)
		r.Delete("/api/keys/{id}", s.keys.Delete)
		r.Get("/api/quotas", s.quotas.List)
		r.Get("/api/settings", s.settings.Get)
	})
	// admin only
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireLogin())
		r.Use(middleware.RequireRole("admin"))
		r.Get("/api/users", s.usersH.List)
		r.Post("/api/users", s.usersH.Create)
		r.Put("/api/users/{id}", s.usersH.Update)
		r.Delete("/api/users/{id}", s.usersH.Delete)
		r.Put("/api/quotas/{id}", s.quotas.Update)
		r.Put("/api/settings", s.settings.Update)
	})
})

// 前端
r.Handle("/*", web.Handler())
```

> **Server 改造(具体)**:把子项目1 的 `New(cfg, db, store)` 改为 `New(cfg config.Config, deps Deps) *Server`,新增 `Deps` 结构体聚合所有依赖。`server.go` 加:
> ```go
> type Deps struct {
> 	DB        *sql.DB
> 	Store     *settings.Store   // 原 settings store(广播开关等)
> 	Users     *user.Store
> 	Sessions  *auth.SessionStore
> 	Auth      *api.AuthHandler
> 	UsersH    *api.UserHandler
> 	Keys      *api.KeyHandler
> 	Quotas    *api.QuotaHandler
> 	Settings  *api.SettingsHandler
> 	OAuth     *api.OAuthHandler
> 	Passkey   *api.PasskeyHandler
> }
> ```
> `Server` 结构体持有 `cfg`、`deps Deps`(取代原来的 `db`/`store` 字段)、`httpServer`、`router`、`actualAddr`。`listener.go` 的 `listenHost` 改读 `s.deps.Store`。`main.go` 构造所有 store/handler 后组装 `Deps` 传入 `New`。子项目1 的 `server_test.go` 的 `New(cfg, d, settings.New(d))` 调用需同步改为 `New(cfg, Deps{DB: d, Store: settings.New(d), ...})`--测试只测 health/broadcast,handler 可为 nil(路由挂载时跳过 nil handler)。实现者在 `buildRouter` 里对 nil handler 做守卫(若某 handler 为 nil 则不挂载对应路由),保证旧测试不破坏。

- [ ] **Step 7: 修改 main.go 首次启动建管理员**

```go
// migrate 后,检查是否有 admin
var adminCount int
d.QueryRow("SELECT COUNT(*) FROM users WHERE role='admin'").Scan(&adminCount)
if adminCount == 0 {
	email := os.Getenv("CARRYAPI_ADMIN_EMAIL")
	pw := os.Getenv("CARRYAPI_ADMIN_PASSWORD")
	if email == "" {
		email = "admin@carryapi.local"
	}
	if pw == "" {
		pw = generateRandomPassword(16)
		fmt.Printf("created admin %s with password: %s (change it immediately)\n", email, pw)
	} else {
		fmt.Printf("created admin %s\n", email)
	}
	hash, _ := auth.HashPassword(pw)
	us.Create(email, hash, "admin")
}
```

- [ ] **Step 8: 运行测试 + 冒烟测试**

```bash
go test ./... -v
```
预期:全部 PASS(子项目1 的 20 + 本子项目新增)。

冒烟(手动验证 main.go):
```bash
go build -o carryapi.exe ./cmd/carryapi
./carryapi.exe &
sleep 2
# 注册 + 登录
curl -s -X POST http://127.0.0.1:8080/api/auth/register -d '{"email":"t@x.com","password":"pw123"}'
curl -s -X POST http://127.0.0.1:8080/api/auth/login -d '{"email":"t@x.com","password":"pw123"}'
# 用 admin(首次启动打印的密码)登录
curl -s -X POST http://127.0.0.1:8080/api/auth/login -d '{"email":"admin@carryapi.local","password":"<printed>"}'
kill %1
```

- [ ] **Step 9: 提交**

```bash
git add internal/api internal/server/cmd/carryapi/main.go
git commit -m "feat(api): user/key/quota/settings handlers, route mounting, admin bootstrap"
```

---

### Task 12: OAuth(Discord + X)

**Files:**
- Create: `internal/oauth/oauth.go`
- Create: `internal/oauth/discord.go`
- Create: `internal/oauth/x.go`
- Create: `internal/api/oauth_handler.go`
- Test: `internal/oauth/oauth_test.go`
- Modify: `internal/server/router.go`(挂载 oauth 路由)

**Interfaces:**
- Produces: `oauth.Provider` 接口 `{ AuthURL(state string) string; Exchange(ctx, code string) (*Token, error); FetchUserID(ctx, token) (string, error) }`;`oauth.Discord`、`oauth.X` 实现;`oauth.NewDiscord(clientID, secret, redirectURL)`、`oauth.NewX(...)`。`Token`={AccessToken, TokenType}。state 用随机 hex + 存 cookie 校验。
- Handler:`/api/auth/oauth/{provider}` -> 生成 state + 重定向到 provider AuthURL;`/api/auth/oauth/callback?code=&state=` -> 校验 state -> Exchange -> FetchUserID -> 查 auth_methods(provider,uid):已绑定则建 session,未绑定且注册开则建用户+绑定,注册关则提示先登录。

- [ ] **Step 1: 写失败测试**

`internal/oauth/oauth_test.go`(用 httptest mock provider 端点):

```go
package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscordExchangeAndUserID(t *testing.T) {
	// mock token 端点 + user 端点
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"fake-token","token_type":"Bearer"}`))
	}))
	defer tokenSrv.Close()
	userSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fake-token" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"discord-uid-123"}`))
	}))
	defer userSrv.Close()

	d := NewDiscordWithEndpoints("cid", "secret", "http://cb", tokenSrv.URL, userSrv.URL)
	url := d.AuthURL("mystate")
	if url == "" {
		t.Fatal("empty auth url")
	}
	tok, err := d.Exchange(context.Background(), "code", "mystate")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if tok.AccessToken != "fake-token" {
		t.Errorf("token = %q", tok.AccessToken)
	}
	uid, err := d.FetchUserID(context.Background(), tok)
	if err != nil || uid != "discord-uid-123" {
		t.Errorf("uid = %q err %v", uid, err)
	}
}
```

> X 的测试结构相同(用 mock),代码如下,追加到 `oauth_test.go`:

```go
func TestXExchangeAndUserID(t *testing.T) {
	// X 返回 {"data":{"id":"x-uid-456"}}
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"x-token","token_type":"bearer"}`))
	}))
	defer tokenSrv.Close()
	userSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer x-token" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"id":"x-uid-456"}}`))
	}))
	defer userSrv.Close()

	x := NewXWithEndpoints("cid", "secret", "http://cb", tokenSrv.URL, userSrv.URL)
	url := x.AuthURL("mystate")
	if url == "" {
		t.Fatal("empty auth url")
	}
	tok, err := x.Exchange(context.Background(), "code", "mystate")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if tok.AccessToken != "x-token" {
		t.Errorf("token = %q", tok.AccessToken)
	}
	uid, err := x.FetchUserID(context.Background(), tok)
	if err != nil || uid != "x-uid-456" {
		t.Errorf("uid = %q err %v", uid, err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/oauth/ -v
```

预期:编译失败。

- [ ] **Step 3: 实现 oauth.go + discord.go + x.go**

`internal/oauth/oauth.go`:

```go
package oauth

import "context"

type Token struct {
	AccessToken string
	TokenType   string
}

type Provider interface {
	AuthURL(state string) string
	Exchange(ctx context.Context, code, state string) (*Token, error)
	FetchUserID(ctx context.Context, token *Token) (string, error)
	Name() string
}
```

`internal/oauth/discord.go`(用 `golang.org/x/oauth2`):

```go
package oauth

import (
	"context"
	"encoding/json"
	"net/http"

	"golang.org/x/oauth2"
)

type Discord struct {
	config   *oauth2.Config
	userURL  string
}

func NewDiscord(clientID, secret, redirectURL string) *Discord {
	return NewDiscordWithEndpoints(clientID, secret, redirectURL,
		"https://discord.com/api/oauth2/token", "https://discord.com/api/users/@me")
}

func NewDiscordWithEndpoints(clientID, secret, redirectURL, tokenURL, userURL string) *Discord {
	return &Discord{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: secret,
			RedirectURL:  redirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://discord.com/api/oauth2/authorize",
				TokenURL: tokenURL,
			},
			Scopes: []string{"identify"},
		},
		userURL: userURL,
	}
}

func (d *Discord) Name() string { return "discord" }

func (d *Discord) AuthURL(state string) string {
	return d.config.AuthCodeURL(state)
}

func (d *Discord) Exchange(ctx context.Context, code, state string) (*Token, error) {
	// Discord 用 client_secret 换 token,不需要 PKCE;state 由 handler 校验
	tok, err := d.config.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}
	return &Token{AccessToken: tok.AccessToken, TokenType: tok.TokenType}, nil
}

func (d *Discord) FetchUserID(ctx context.Context, token *Token) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", d.userURL, nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var u struct{ ID string `json:"id"` }
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return "", err
	}
	return u.ID, nil
}
```

`internal/oauth/x.go`(Twitter OAuth 2.0 PKCE):

```go
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/oauth2"
)

type X struct {
	config   *oauth2.Config
	userURL  string
	mu       sync.Mutex
	verifiers map[string]string // state -> code_verifier
}

func NewX(clientID, secret, redirectURL string) *X {
	return NewXWithEndpoints(clientID, secret, redirectURL,
		"https://api.twitter.com/2/oauth2/token", "https://api.twitter.com/2/users/me")
}

func NewXWithEndpoints(clientID, secret, redirectURL, tokenURL, userURL string) *X {
	return &X{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: secret,
			RedirectURL:  redirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://twitter.com/i/oauth2/authorize",
				TokenURL: tokenURL,
			},
			Scopes: []string{"users.read", "tweet.read"},
		},
		userURL:   userURL,
		verifiers: make(map[string]string),
	}
}

func (x *X) Name() string { return "x" }

func (x *X) AuthURL(state string) string {
	// PKCE:生成随机 code_verifier,缓存到 state,challenge 用 S256
	verifier := randomHex(32)
	challenge := pkceS256(verifier)
	x.mu.Lock()
	x.verifiers[state] = verifier
	x.mu.Unlock()
	return x.config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"))
}

func (x *X) Exchange(ctx context.Context, code, state string) (*Token, error) {
	verifier, _ := x.ConsumeVerifier(state)
	// 手动 POST token 端点,带 code_verifier(PKCE)
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {x.config.RedirectURL},
		"client_id":     {x.config.ClientID},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, "POST", x.config.Endpoint.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s: %s", resp.Status, body)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return nil, err
	}
	return &Token{AccessToken: tok.AccessToken, TokenType: tok.TokenType}, nil
}

// ConsumeVerifier 取出并删除某 state 的 code_verifier(回调时用)
func (x *X) ConsumeVerifier(state string) (string, bool) {
	x.mu.Lock()
	defer x.mu.Unlock()
	v, ok := x.verifiers[state]
	if ok {
		delete(x.verifiers, state)
	}
	return v, ok
}

func pkceS256(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (x *X) FetchUserID(ctx context.Context, token *Token) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", x.userURL, nil)
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var u struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return "", err
	}
	return u.Data.ID, nil
}
```

> 注:X 实现完整 PKCE(S256):`AuthURL` 生成随机 `code_verifier` 并按 state 缓存,`Exchange` 用手动 POST 带 `code_verifier` 换 token 并消费该 state 的 verifier。`verifiers` map 在单实例进程内有效;多实例部署需改为共享存储(后续加固)。

- [ ] **Step 4: 实现 oauth_handler.go + 挂路由**

`internal/api/oauth_handler.go`:

```go
package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"carryapi/internal/auth"
	"carryapi/internal/middleware"
	"carryapi/internal/oauth"
	"carryapi/internal/settings"
	"carryapi/internal/user"
)

type OAuthHandler struct {
	providers map[string]oauth.Provider
	users     *user.Store
	sessions  *auth.SessionStore
	settings  *settings.Store
}

func NewOAuthHandler(users *user.Store, sessions *auth.SessionStore, settings *settings.Store) *OAuthHandler {
	h := &OAuthHandler{providers: map[string]oauth.Provider{}, users: users, sessions: sessions, settings: settings}
	// 从 settings 读 client id/secret 初始化(若已配置)
	h.loadProviders()
	return h
}

func (h *OAuthHandler) loadProviders() {
	// 实现者:读 settings oauth_discord_client_id 等,有则 NewDiscord
}

func (h *OAuthHandler) Begin(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	p, ok := h.providers[provider]
	if !ok {
		JSONError(w, 400, "unknown provider")
		return
	}
	state := randomHex(16)
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", Value: state, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 600})
	http.Redirect(w, r, p.AuthURL(state), http.StatusTemporaryRedirect)
}

func (h *OAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	p, ok := h.providers[provider]
	if !ok {
		JSONError(w, 400, "unknown provider")
		return
	}
	// 校验 state
	cookie, err := r.Cookie("oauth_state")
	if err != nil || cookie.Value != r.URL.Query().Get("state") {
		JSONError(w, 400, "invalid state")
		return
	}
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	tok, err := p.Exchange(r.Context(), code, state)
	if err != nil {
		JSONError(w, 400, "token exchange failed")
		return
	}
	uid, err := p.FetchUserID(r.Context(), tok)
	if err != nil {
		JSONError(w, 400, "failed to fetch user id")
		return
	}
	// 查 auth_methods
	_, err = h.users.GetAuthMethod(p.Name(), uid)
	if err == nil {
		// 已绑定 -> 查 user -> 建 session
		// (GetAuthMethod 返回 UserID)
		m, _ := h.users.GetAuthMethod(p.Name(), uid)
		sess, _ := h.sessions.Create(m.UserID, 7*24*time.Hour, "", "")
		setSessionCookie(w, sess.Token) // 复用 AuthHandler 的(提取公共)
		setCSRFCookie(w)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}
	// 未绑定
	open, _ := h.settings.Get("registration_open")
	if open == "false" {
		JSONError(w, 403, "not linked; login first to bind")
		return
	}
	// 注册开 -> 创建用户(无密码)+ 绑定
	u, err := h.users.Create(p.Name()+"-"+uid, "", "user")
	if err != nil {
		JSONError(w, 400, "failed to create user")
		return
	}
	h.users.AddAuthMethod(u.ID, p.Name(), uid, nil)
	sess, _ := h.sessions.Create(u.ID, 7*24*time.Hour, "", "")
	setSessionCookie(w, sess.Token)
	setCSRFCookie(w)
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
```

> 注:`setSessionCookie`/`setCSRFCookie` 是 api 包级函数(在 Task 10 的 auth_handler.go 定义),本 handler 直接调用。

路由挂载:
```go
r.Get("/api/auth/oauth/{provider}", s.oauth.Begin)
r.Get("/api/auth/oauth/callback", s.oauth.Callback)
```

- [ ] **Step 5: 运行测试确认通过**

```bash
go test ./... -v
```

- [ ] **Step 6: 提交**

```bash
git add internal/oauth internal/api/oauth_handler.go internal/server/router.go
git commit -m "feat(oauth): discord and x oauth2 login with provider abstraction"
```

---

### Task 13: Passkey(WebAuthn)

**Files:**
- Create: `internal/webauthn/webauthn.go`
- Create: `internal/api/passkey_handler.go`
- Test: `internal/webauthn/webauthn_test.go`
- Modify: `internal/server/router.go`

**Interfaces:**
- Consumes: `go-webauthn/webauthn` (v0.17+,纯 Go 无 CGO)。go-webauthn 的 API:`webauthn.New(*Config) (*WebAuthn, error)`;`(*WebAuthn) BeginRegistration(user User) (*protocol.CredentialCreation, *SessionData, error)`;`(*WebAuthn) FinishRegistration(user User, session SessionData, request *http.Request) (*Credential, error)`;`(*WebAuthn) BeginLogin(user User) (*protocol.CredentialAssertion, *SessionData, error)`;`(*WebAuthn) FinishLogin(user User, session SessionData, response *http.Request) (*Credential, error)`。`User` 接口需 `WebAuthnID() []byte / WebAuthnName() string / WebAuthnDisplayName() string / WebAuthnCredentials() []Credential`。`SessionData` 和 `Credential` 可 JSON 序列化。
- Produces:
  - `webauthn.Service` 结构体封装 `*webauthn.WebAuthn` + 内存 session store(map[key]*SessionData + mutex + TTL 5 分钟)。
  - `webauthn.New(rpID, rpOrigin string) (*Service, error)`。
  - `webauthn.User` 接口(重新导出 go-webauthn 的 User,加一个适配器 `LocalUser` 把 `*user.User` + 已有 credentials 适配成 webauthn.User)。
  - `(*Service) BeginRegistration(u webauthn.User) (creation *protocol.CredentialCreation, sessionKey string, err error)`(sessionKey = 随机 hex,存 SessionData 到内存)。
  - `(*Service) FinishRegistration(u webauthn.User, sessionKey string, r *http.Request) (*webauthn.Credential, error)`(取 session -> FinishRegistration -> 删 session)。
  - `(*Service) BeginLogin(u webauthn.User) (assertion *protocol.CredentialAssertion, sessionKey string, err error)`。
  - `(*Service) FinishLogin(u webauthn.User, sessionKey string, r *http.Request) (*webauthn.Credential, error)`。
  - `(*Service) SaveCredential(userID int64, cred webauthn.Credential)` 回调钩子?--不,存库由 handler 做。Service 只返回 credential,handler 存 auth_methods。
- Handler:`/api/auth/passkey/register/begin`、`/api/auth/passkey/register/finish`(需登录);`/api/auth/passkey/login/begin`、`/api/auth/passkey/login/finish`。注册 finish 存 auth_methods(provider=passkey,provider_uid=credentialID hex,secret=JSON of Credential)。登录 finish 建 session。

- [ ] **Step 1: 写失败测试**

`internal/webauthn/webauthn_test.go`:

```go
package webauthn

import (
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
)

// stubUser 实现 webauthn.User 接口
type stubUser struct {
	id   []byte
	name string
	cred []webauthn.Credential
}

func (s *stubUser) WebAuthnID() []byte                  { return s.id }
func (s *stubUser) WebAuthnName() string                 { return s.name }
func (s *stubUser) WebAuthnDisplayName() string          { return s.name }
func (s *stubUser) WebAuthnCredentials() []webauthn.Credential { return s.cred }

func TestNewService(t *testing.T) {
	s, err := New("localhost", "http://localhost:8080")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s == nil {
		t.Fatal("nil service")
	}
}

func TestBeginRegistrationReturnsChallenge(t *testing.T) {
	s, _ := New("localhost", "http://localhost:8080")
	u := &stubUser{id: []byte("user-1"), name: "alice@example.com"}
	creation, sessionKey, err := s.BeginRegistration(u)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	if sessionKey == "" {
		t.Error("expected non-empty sessionKey")
	}
	if creation == nil || creation.Response.Challenge == "" {
		t.Error("expected non-empty challenge in creation")
	}
	// session 应已存入内存
	if _, ok := s.getSession(sessionKey); !ok {
		t.Error("session not stored")
	}
}

func TestFinishRegistrationUnknownSession(t *testing.T) {
	s, _ := New("localhost", "http://localhost:8080")
	u := &stubUser{id: []byte("user-1"), name: "alice@example.com"}
	// 没有对应 session -> 应报错
	_, err := s.FinishRegistration(u, "nonexistent-key", nil)
	if err == nil {
		t.Error("expected error for unknown session key")
	}
}

func TestBeginLoginReturnsChallenge(t *testing.T) {
	s, _ := New("localhost", "http://localhost:8080")
	u := &stubUser{id: []byte("user-1"), name: "alice@example.com"}
	assertion, sessionKey, err := s.BeginLogin(u)
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
	if sessionKey == "" || assertion == nil {
		t.Error("expected non-empty assertion + sessionKey")
	}
}
```

> 注:`BeginRegistration`/`BeginLogin` 在无真实浏览器凭证时无法走完 Finish;Finish 只测"未知 sessionKey 报错"这条路径。完整 Finish 流程由子项目 6 前端联调或手动验证。

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/webauthn/ -v
```

预期:编译失败。

- [ ] **Step 3: 实现 webauthn.go**

`internal/webauthn/webauthn.go`:

```go
package webauthn

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// 重新导出,便于外部包引用(避免多处 import 同一长路径)
type User = webauthn.User
type Credential = webauthn.Credential

type Service struct {
	w        *webauthn.WebAuthn
	mu       sync.Mutex
	sessions map[string]*webauthn.SessionData
}

func New(rpID, rpOrigin string) (*Service, error) {
	w, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "carryAPI",
		RPID:          rpID,
		RPOrigins:     []string{rpOrigin},
	})
	if err != nil {
		return nil, fmt.Errorf("webauthn init: %w", err)
	}
	s := &Service{w: w, sessions: make(map[string]*webauthn.SessionData)}
	go s.gc()
	return s, nil
}

// LocalUser 把 carryAPI 的 user.User + 已存 credentials 适配成 webauthn.User
type LocalUser struct {
	ID           int64
	Email        string
	Credentials  []webauthn.Credential
}

func (u *LocalUser) WebAuthnID() []byte {
	// 用 user ID 的字节表示作 handle(稳定且唯一)
	return []byte(fmt.Sprintf("uid-%d", u.ID))
}
func (u *LocalUser) WebAuthnName() string        { return u.Email }
func (u *LocalUser) WebAuthnDisplayName() string { return u.Email }
func (u *LocalUser) WebAuthnCredentials() []webauthn.Credential { return u.Credentials }

func (s *Service) BeginRegistration(u webauthn.User) (*protocol.CredentialCreation, string, error) {
	creation, session, err := s.w.BeginRegistration(u)
	if err != nil {
		return nil, "", fmt.Errorf("begin registration: %w", err)
	}
	key := s.putSession(session)
	return creation, key, nil
}

func (s *Service) FinishRegistration(u webauthn.User, sessionKey string, r *http.Request) (*webauthn.Credential, error) {
	session, ok := s.takeSession(sessionKey)
	if !ok {
		return nil, fmt.Errorf("unknown or expired session")
	}
	if r == nil {
		return nil, fmt.Errorf("nil request")
	}
	cred, err := s.w.FinishRegistration(u, *session, r)
	if err != nil {
		return nil, fmt.Errorf("finish registration: %w", err)
	}
	return cred, nil
}

func (s *Service) BeginLogin(u webauthn.User) (*protocol.CredentialAssertion, string, error) {
	assertion, session, err := s.w.BeginLogin(u)
	if err != nil {
		return nil, "", fmt.Errorf("begin login: %w", err)
	}
	key := s.putSession(session)
	return assertion, key, nil
}

func (s *Service) FinishLogin(u webauthn.User, sessionKey string, r *http.Request) (*webauthn.Credential, error) {
	session, ok := s.takeSession(sessionKey)
	if !ok {
		return nil, fmt.Errorf("unknown or expired session")
	}
	if r == nil {
		return nil, fmt.Errorf("nil request")
	}
	cred, err := s.w.FinishLogin(u, *session, r)
	if err != nil {
		return nil, fmt.Errorf("finish login: %w", err)
	}
	return cred, nil
}

func (s *Service) putSession(sess *webauthn.SessionData) string {
	b := make([]byte, 16)
	rand.Read(b)
	key := hex.EncodeToString(b)
	s.mu.Lock()
	s.sessions[key] = sess
	s.mu.Unlock()
	return key
}

func (s *Service) getSession(key string) (*webauthn.SessionData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[key]
	return sess, ok
}

// takeSession 取出并删除(一次性)
func (s *Service) takeSession(key string) (*webauthn.SessionData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[key]
	if ok {
		delete(s.sessions, key)
	}
	return sess, ok
}

// gc 定期清理过期 session(SessionData.Expires)
func (s *Service) gc() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for k, sess := range s.sessions {
			if now.After(sess.Expires) {
				delete(s.sessions, k)
			}
		}
		s.mu.Unlock()
	}
}
```

> 注:import 需 `net/http`(FinishRegistration/FinishLogin 的 r *http.Request 参数)。实现者补 `"net/http"` import。

- [ ] **Step 4: 实现 passkey_handler.go**

`internal/api/passkey_handler.go`:

```go
package api

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"carryapi/internal/auth"
	"carryapi/internal/middleware"
	"carryapi/internal/user"
	"carryapi/internal/webauthn"
)

type PasskeyHandler struct {
	svc      *webauthn.Service
	users    *user.Store
	sessions *auth.SessionStore
}

func NewPasskeyHandler(svc *webauthn.Service, users *user.Store, sessions *auth.SessionStore) *PasskeyHandler {
	return &PasskeyHandler{svc: svc, users: users, sessions: sessions}
}

// loadWebAuthnUser 从 user.Store 加载用户 + 已有 passkey credentials
func (h *PasskeyHandler) loadWebAuthnUser(u *user.User) *webauthn.LocalUser {
	methods, _ := h.users.GetAuthMethods(u.ID)
	var creds []webauthn.Credential
	for _, m := range methods {
		if m.Provider == "passkey" {
			var c webauthn.Credential
			if json.Unmarshal(m.Secret, &c) == nil {
				creds = append(creds, c)
			}
		}
	}
	return &webauthn.LocalUser{ID: u.ID, Email: u.Email, Credentials: creds}
}

func (h *PasskeyHandler) RegisterBegin(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		JSONError(w, 401, "unauthorized")
		return
	}
	wu := h.loadWebAuthnUser(u)
	creation, sessionKey, err := h.svc.BeginRegistration(wu)
	if err != nil {
		JSONError(w, 500, "begin registration failed")
		return
	}
	JSON(w, 200, map[string]any{
		"publicKey":   creation,
		"session_key": sessionKey, // 客户端在 finish 时回传
	})
}

func (h *PasskeyHandler) RegisterFinish(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		JSONError(w, 401, "unauthorized")
		return
	}
	sessionKey := r.URL.Query().Get("session_key")
	if sessionKey == "" {
		JSONError(w, 400, "missing session_key")
		return
	}
	wu := h.loadWebAuthnUser(u)
	cred, err := h.svc.FinishRegistration(wu, sessionKey, r)
	if err != nil {
		JSONError(w, 400, "finish registration failed: "+err.Error())
		return
	}
	// 存 auth_methods:provider=passkey, provider_uid=credentialID hex, secret=credential JSON(加密)
	credJSON, _ := json.Marshal(cred)
	if err := h.users.AddAuthMethod(u.ID, "passkey", hex.EncodeToString(cred.ID), credJSON); err != nil {
		JSONError(w, 500, "failed to store credential")
		return
	}
	JSON(w, 200, map[string]string{"status": "ok"})
}

func (h *PasskeyHandler) LoginBegin(w http.ResponseWriter, r *http.Request) {
	// 无登录态:客户端传 email 找用户
	email := r.URL.Query().Get("email")
	if email == "" {
		JSONError(w, 400, "missing email")
		return
	}
	u, err := h.users.GetByEmail(email)
	if err != nil {
		JSONError(w, 401, "user not found")
		return
	}
	wu := h.loadWebAuthnUser(u)
	assertion, sessionKey, err := h.svc.BeginLogin(wu)
	if err != nil {
		JSONError(w, 500, "begin login failed")
		return
	}
	JSON(w, 200, map[string]any{
		"publicKey":   assertion,
		"session_key": sessionKey,
	})
}

func (h *PasskeyHandler) LoginFinish(w http.ResponseWriter, r *http.Request) {
	sessionKey := r.URL.Query().Get("session_key")
	email := r.URL.Query().Get("email")
	if sessionKey == "" || email == "" {
		JSONError(w, 400, "missing session_key or email")
		return
	}
	u, err := h.users.GetByEmail(email)
	if err != nil {
		JSONError(w, 401, "user not found")
		return
	}
	wu := h.loadWebAuthnUser(u)
	_, err = h.svc.FinishLogin(wu, sessionKey, r)
	if err != nil {
		JSONError(w, 401, "finish login failed: "+err.Error())
		return
	}
	sess, err := h.sessions.Create(u.ID, 7*24*time.Hour, "", "")
	if err != nil {
		JSONError(w, 500, "session create failed")
		return
	}
	setSessionCookie(w, sess.Token)
	setCSRFCookie(w)
	JSON(w, 200, map[string]any{
		"user": map[string]any{"id": u.ID, "email": u.Email, "role": u.Role},
	})
}
```

> 注:`setSessionCookie`/`setCSRFCookie` 是 api 包级函数(在 Task 10 的 auth_handler.go 定义),PasskeyHandler 直接调用。

- [ ] **Step 5: 挂路由**

在 router.go 的 auth route group 内加:

```go
r.Post("/passkey/register/begin", s.passkey.RegisterBegin)
r.Post("/passkey/register/finish", s.passkey.RegisterFinish)  // 在 RequireLogin group 内
r.Post("/passkey/login/begin", s.passkey.LoginBegin)
r.Post("/passkey/login/finish", s.passkey.LoginFinish)
```

(register/begin 和 register/finish 都需登录 -> 放 RequireLogin group;login/begin 和 login/finish 不需登录 -> 放 auth group 顶层)

- [ ] **Step 6: 运行测试 + 提交**

```bash
go test ./internal/webauthn/ -v
go test ./... -v
git add internal/webauthn internal/api/passkey_handler.go internal/server/router.go
git commit -m "feat(webauthn): passkey registration and login"
```

---

### Task 14: 集成测试 + README 更新

**Files:**
- Create: `internal/api/integration_test.go`
- Modify: `README.md`

**内容:**
- 端到端集成测试:启动完整 server(httptest) -> 注册 -> 登录(带 cookie) -> 创建 API Key -> 用 API Key 调 `/api/health`(或占位代理端点) -> 验证鉴权链路。覆盖角色:普通用户访问 admin 端点返 403。
- README 更新:加认证章节(登录方式、首次启动管理员、API Key 创建、2FA 开启、OAuth 配置环境变量)。

- [ ] **Step 1: 写集成测试**

`internal/api/integration_test.go`:

```go
package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"carryapi/internal/apikey"
	"carryapi/internal/auth"
	"carryapi/internal/crypto"
	"carryapi/internal/db"
	"carryapi/internal/middleware"
	"carryapi/internal/settings"
	"carryapi/internal/user"
)

func TestFullAuthFlow(t *testing.T) {
	// 复用 Task 11 的 apiFixture(聚合 db/users/keys/sessions/settings + 各 handler)。
	// 这里用 chi 路由器挂载完整 /api 路由,跑端到端流程。
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	c, _ := crypto.New(bytes.Repeat([]byte{1}, 32))
	us := user.New(d, c)
	ss := auth.NewSessionStore(d)
	st := settings.New(d)
	st.Set("registration_open", "true")
	ls := auth.NewLoginService(us, ss, st)
	ks := apikey.New(d)
	authH := NewAuthHandler(ls, ss, us, st)
	keyH := NewKeyHandler(ks)
	usersH := NewUserHandler(us)

	r := chi.NewRouter()
	r.Use(middleware.SessionMiddleware(ss, us))
	r.Post("/api/auth/register", authH.Register)
	r.Post("/api/auth/login", authH.Login)
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireLogin())
		r.Use(middleware.CSRFMiddleware())
		r.Get("/api/keys", keyH.List)
		r.Post("/api/keys", keyH.Create)
	})
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireLogin())
		r.Use(middleware.RequireRole("admin"))
		r.Get("/api/users", usersH.List)
	})

	ts := httptest.NewServer(r)
	defer ts.Close()
	client := &http.Client{}

	// 1. 注册
	resp, _ := client.Post(ts.URL+"/api/auth/register",
		"application/json", strings.NewReader(`{"email":"e2e@x.com","password":"pw123"}`))
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("register status=%d", resp.StatusCode)
	}

	// 2. 登录(拿 session + csrf cookie)
	resp, _ = client.Post(ts.URL+"/api/auth/login",
		"application/json", strings.NewReader(`{"email":"e2e@x.com","password":"pw123"}`))
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("login status=%d", resp.StatusCode)
	}
	var csrf string
	for _, c := range resp.Cookies() {
		if c.Name == middleware.CSRFCookie {
			csrf = c.Value
		}
	}

	// 3. 创建 API Key(带 csrf 头 + cookie 由 client jar 自动携带)
	req, _ := http.NewRequest("POST", ts.URL+"/api/keys", strings.NewReader(`{"label":"e2e"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.CSRFHeader, csrf)
	resp, _ = client.Do(req)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("create key status=%d body=%s", resp.StatusCode, body)
	}
	var keyResp map[string]any
	json.Unmarshal(body, &keyResp)
	if keyResp["key"] == nil {
		t.Fatal("expected plaintext key")
	}

	// 4. 普通用户访问 /api/users -> 403(非 admin)
	req, _ = http.NewRequest("GET", ts.URL+"/api/users", nil)
	req.Header.Set(middleware.CSRFHeader, csrf)
	resp, _ = client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Errorf("non-admin /api/users status=%d, want 403", resp.StatusCode)
	}
}
```

> 注:本测试需要 import:`bytes`、`encoding/json`、`io`、`net/http`、`net/http/httptest`、`strings`、`testing`、`carryapi/internal/crypto`、`carryapi/internal/middleware`、`github.com/go-chi/chi/v5`。`apiFixture` 是 Task 11 的辅助;此处为独立完整构造,不依赖 fixture,确保集成路径自包含。

- [ ] **Step 2: 运行测试确认通过**

```bash
go test ./... -v
```

- [ ] **Step 3: 更新 README**

在 README.md "运行" 章节后加 "认证" 章节:首次启动管理员、登录方式、API Key、2FA、OAuth 环境变量。

- [ ] **Step 4: 提交**

```bash
git add internal/api/integration_test.go README.md
git commit -m "test(api): end-to-end auth flow integration test + readme auth section"
```

---

## 子项目 2 完成标准

- [ ] `go test ./...` 全绿(子项目1 的 20 + 本子项目新增)
- [ ] 首次启动自动建管理员(打印密码或用环境变量)
- [ ] 注册/登录/登出端点可用(密码 + 2FA)
- [ ] TOTP 2FA 开启/验证/关闭可用
- [ ] Passkey 注册/登录可用(至少 Service 初始化 + begin 端点)
- [ ] OAuth Discord/X 流程可用(配置 client id/secret 后)
- [ ] API Key 创建/列表/删除/鉴权可用
- [ ] 配额 CRUD 可用
- [ ] 用户 CRUD(admin)可用,普通用户访问 admin 端点返 403
- [ ] CSRF 保护生效(改操作需 token)
- [ ] 登录/注册限流生效
- [ ] 交叉编译仍无 CGO(新依赖纯 Go)
