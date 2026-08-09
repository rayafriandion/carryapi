# carryAPI 子项目 1:项目骨架与基础设施 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 搭建 carryAPI 单二进制骨架:Go 模块、SQLite 自动建表迁移、配置加载、HTTP 服务(广播开关控制监听地址)、Vue3 前端占位页构建并 `//go:embed` 嵌入二进制。完成后产出一个可运行的 `carryapi` 二进制。

**Architecture:** Go 单二进制,`cmd/carryapi` 为入口。分层:`internal/config`(配置)、`internal/db`(SQLite + 迁移)、`internal/server`(HTTP + 路由)、`web`(前端工程 + embed 静态资源)。前端 Vue3 + Vite 构建产物放 `web/dist`,Go 编译时由 `web` 包 `//go:embed` 嵌入。广播开关 = 监听地址 `0.0.0.0`(开)/`127.0.0.1`(关),存 DB settings 表,启动时读取决定 listener。

**Tech Stack:** Go 1.22+、`modernc.org/sqlite`(纯 Go,无 CGO)、`github.com/go-chi/chi/v5`(路由)、Vue 3 + Vite + Naive UI(前端占位)。

## Global Constraints

- Go 1.22+;交叉编译须支持 `GOOS=linux GOARCH=amd64` 与 `GOOS=windows GOARCH=amd64`,无 CGO。
- SQLite 驱动用 `modernc.org/sqlite`(纯 Go),禁止用 `mattn/go-sqlite3`(需 CGO)。
- 数据库单文件,默认 `./carryapi.db`,路径由 `CARRYAPI_DB_PATH` 覆盖。
- 敏感字段加密主密钥:环境变量 `CARRYAPI_MASTER_KEY`,缺失时生成并写入 `./carryapi.key`(0600 权限),启动时读取。
- 监听端口默认 `8080`,由 `CARRYAPI_PORT` 覆盖。
- 前端构建产物 `web/dist` 通过 `//go:embed` 嵌入;开发时前端跑 `npm run dev`,后端跑 `go run`,通过 Vite 代理转发 API。
- Git 身份:`rayafriandion <amizhisa@outlook.com>`(本仓库已配置)。
- TDD:每个任务先写失败测试,再实现,再验证通过,再提交。

---

## File Structure

```
carryAPI/
├── go.mod                          # Go 模块定义
├── go.sum
├── cmd/
│   └── carryapi/
│       └── main.go                 # 入口:加载配置->初始化DB->启动server
├── internal/
│   ├── config/
│   │   ├── config.go               # 配置结构 + 加载(环境变量+主密钥)
│   │   └── config_test.go
│   ├── db/
│   │   ├── db.go                   # 打开连接 + 迁移执行器
│   │   ├── migrations.go           # 建表 SQL(版本化)
│   │   └── db_test.go
│   ├── crypto/
│   │   ├── crypto.go               # AES-GCM 加解密(主密钥)
│   │   └── crypto_test.go
│   ├── settings/
│   │   ├── settings.go             # settings 表读写(键值)
│   │   └── settings_test.go
│   ├── server/
│   │   ├── server.go               # HTTP server 构建 + 启动/优雅关闭
│   │   ├── router.go               # 路由挂载(本计划只挂健康检查+静态)
│   │   ├── listener.go             # 广播开关:读 settings 决定监听地址
│   │   └── server_test.go
├── web/                            # 前端工程 + embed 包(//go:embed 必须同包目录)
│   ├── embed.go                    # package web,//go:embed dist + Handler()
│   ├── embed_test.go
│   ├── package.json
│   ├── vite.config.ts
│   ├── index.html
│   ├── src/
│   │   └── main.ts                 # 占位页:显示 "carryAPI"
│   └── dist/                       # 前端构建产物(embed 目标;开发外生成)
│       └── .gitkeep
```

每个文件单一职责:`config` 只管加载环境变量与主密钥;`db` 只管连接与迁移;`crypto` 只管加解密;`settings` 只管键值表读写;`server` 管 HTTP 生命周期与路由;`web`(包)管静态资源 embed。后续子项目(认证/IR/代理)在各自己的 `internal/` 子包扩展。

> **为什么 embed 放 `web/` 包**:`//go:embed` 只能嵌入 Go 源文件所在包目录及其子目录下的文件,不能用 `..`。`web/dist` 产物在 `web/` 下,所以 embed 源文件也必须在 `web/` 包内,这样 `//go:embed dist` 才能命中。

---

### Task 1: 初始化 Go 模块与依赖

**Files:**
- Create: `go.mod`
- Create: `web/dist/.gitkeep`

**Interfaces:**
- Produces: Go 模块 `carryapi`,后续任务在此模块内建包。

- [ ] **Step 1: 初始化模块**

```bash
cd /d/Projects/carryAPI
go mod init carryapi
```

预期:生成 `go.mod`,首行 `module carryapi`。

- [ ] **Step 2: 添加依赖**

```bash
go get modernc.org/sqlite@latest
go get github.com/go-chi/chi/v5@latest
```

- [ ] **Step 3: 验证可编译(空 main 占位)**

创建 `cmd/carryapi/main.go`:

```go
package main

func main() {}
```

```bash
go build ./...
```

预期:无报错。

- [ ] **Step 4: 创建前端产物占位**

```bash
mkdir -p web/dist
touch web/dist/.gitkeep
```

- [ ] **Step 5: 添加 .gitignore**

创建 `.gitignore`:

```
carryapi.db
carryapi.db-*
carryapi.key
node_modules/
```

- [ ] **Step 6: 提交**

```bash
git add go.mod go.sum cmd .gitignore web/dist/.gitkeep
git commit -m "chore: init go module and project skeleton"
```

---

### Task 2: 配置加载(config 包)

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `config.Load() (Config, error)` 返回结构体,含 `Port int`、`DBPath string`、`MasterKey []byte`。`MasterKey` 来自 `CARRYAPI_MASTER_KEY` 环境变量;若缺失则读 `./carryapi.key`,若文件也不存在则生成 32 字节随机密钥写入 `carryapi.key`(0600)。

- [ ] **Step 1: 写失败测试**

`internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("CARRYAPI_PORT", "9090")
	t.Setenv("CARRYAPI_DB_PATH", "/tmp/test.db")
	t.Setenv("CARRYAPI_MASTER_KEY", "0123456789abcdef0123456789abcdef") // 32 字节
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("DBPath = %q, want /tmp/test.db", cfg.DBPath)
	}
	if len(cfg.MasterKey) != 32 {
		t.Errorf("MasterKey len = %d, want 32", len(cfg.MasterKey))
	}
}

func TestLoadDefaults(t *testing.T) {
	os.Unsetenv("CARRYAPI_PORT")
	os.Unsetenv("CARRYAPI_DB_PATH")
	os.Unsetenv("CARRYAPI_MASTER_KEY")
	dir := t.TempDir()
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("default Port = %d, want 8080", cfg.Port)
	}
	if cfg.DBPath != "./carryapi.db" {
		t.Errorf("default DBPath = %q, want ./carryapi.db", cfg.DBPath)
	}
	if len(cfg.MasterKey) != 32 {
		t.Errorf("generated MasterKey len = %d, want 32", len(cfg.MasterKey))
	}
	// carryapi.key 应已生成
	if _, err := os.Stat(filepath.Join(dir, "carryapi.key")); err != nil {
		t.Errorf("carryapi.key not created: %v", err)
	}
}

func TestMasterKeyInvalidLength(t *testing.T) {
	t.Setenv("CARRYAPI_MASTER_KEY", "short")
	_, err := Load()
	if err == nil {
		t.Error("expected error for short master key, got nil")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/config/ -v
```

预期:编译失败(找不到 `Load`)。

- [ ] **Step 3: 实现 config 包**

`internal/config/config.go`:

```go
package config

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port      int
	DBPath    string
	MasterKey []byte
}

func Load() (Config, error) {
	cfg := Config{
		Port:   8080,
		DBPath: "./carryapi.db",
	}
	if v := os.Getenv("CARRYAPI_PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CARRYAPI_PORT: %w", err)
		}
		cfg.Port = p
	}
	if v := os.Getenv("CARRYAPI_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	key, err := loadMasterKey()
	if err != nil {
		return Config{}, err
	}
	cfg.MasterKey = key
	return cfg, nil
}

func loadMasterKey() ([]byte, error) {
	if v := os.Getenv("CARRYAPI_MASTER_KEY"); v != "" {
		if len(v) != 32 {
			return nil, errors.New("CARRYAPI_MASTER_KEY must be 32 bytes")
		}
		return []byte(v), nil
	}
	const keyFile = "carryapi.key"
	if data, err := os.ReadFile(keyFile); err == nil {
		if len(data) != 32 {
			return nil, fmt.Errorf("%s must be 32 bytes", keyFile)
		}
		return data, nil
	}
	// 生成新密钥
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}
	if err := os.WriteFile(keyFile, key, 0600); err != nil {
		return nil, fmt.Errorf("write %s: %w", keyFile, err)
	}
	fmt.Printf("generated new master key at %s (keep it safe)\n", keyFile)
	return key, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/config/ -v
```

预期:3 个测试全部 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/config go.mod go.sum
git commit -m "feat(config): load config from env with master key management"
```

---

### Task 3: AES-GCM 加密工具(crypto 包)

**Files:**
- Create: `internal/crypto/crypto.go`
- Test: `internal/crypto/crypto_test.go`

**Interfaces:**
- Consumes: `config.Config.MasterKey`([]byte, 32 字节)
- Produces: `crypto.New(masterKey []byte) (*Cipher, error)`;`(*Cipher) Encrypt(plaintext []byte) ([]byte, error)` 返回 `nonce(12)+ciphertext`;`(*Cipher) Decrypt(blob []byte) ([]byte, error)`。后续子项目用它加解密上游 API Key、TOTP 密钥等。

- [ ] **Step 1: 写失败测试**

`internal/crypto/crypto_test.go`:

```go
package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	c, err := New(key)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	plain := []byte("upstream-secret-api-key")
	blob, err := c.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	got, err := c.Decrypt(blob)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("round trip mismatch: got %q, want %q", got, plain)
	}
}

func TestEncryptProducesDifferentCiphertext(t *testing.T) {
	c, _ := New(bytes.Repeat([]byte{1}, 32))
	plain := []byte("same-input")
	a, _ := c.Encrypt(plain)
	b, _ := c.Encrypt(plain)
	if bytes.Equal(a, b) {
		t.Error("two encryptions of same plaintext produced identical ciphertext (nonce not random)")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	c1, _ := New(bytes.Repeat([]byte{1}, 32))
	c2, _ := New(bytes.Repeat([]byte{2}, 32))
	blob, _ := c1.Encrypt([]byte("secret"))
	if _, err := c2.Decrypt(blob); err == nil {
		t.Error("expected error decrypting with wrong key, got nil")
	}
}

func TestNewInvalidKey(t *testing.T) {
	if _, err := New(bytes.Repeat([]byte{1}, 16)); err == nil {
		t.Error("expected error for 16-byte key, got nil")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/crypto/ -v
```

预期:编译失败。

- [ ] **Step 3: 实现 crypto 包**

`internal/crypto/crypto.go`:

```go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

type Cipher struct {
	gcm cipher.AEAD
}

func New(masterKey []byte) (*Cipher, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("master key must be 32 bytes (AES-256)")
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("aes new: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm new: %w", err)
	}
	return &Cipher{gcm: gcm}, nil
}

func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	blob := c.gcm.Seal(nonce, nonce, plaintext, nil)
	return blob, nil
}

func (c *Cipher) Decrypt(blob []byte) ([]byte, error) {
	ns := c.gcm.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ciphertext := blob[:ns], blob[ns:]
	plain, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}
	return plain, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/crypto/ -v
```

预期:4 个测试 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/crypto
git commit -m "feat(crypto): AES-GCM encrypt/decrypt with random nonce"
```

---

### Task 4: SQLite 连接与版本化迁移(db 包)

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/migrations.go`
- Test: `internal/db/db_test.go`

**Interfaces:**
- Produces: `db.Open(dbPath string) (*sql.DB, error)`(用 `modernc.org/sqlite` 驱动);`db.Migrate(db *sql.DB) error` 执行版本化迁移;迁移维护 `schema_version` 表。
- 本任务创建本计划范围内的全部表:users、auth_methods、api_keys、quotas、upstream_providers、custom_models、model_prices、request_logs、settings、sessions。后续子项目直接用这些表,不再建表(认证子项目只用,不建)。

- [ ] **Step 1: 写失败测试**

`internal/db/db_test.go`:

```go
package db

import (
	"database/sql"
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()
	if err := Migrate(d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// schema_version 表存在且有记录
	var version int
	err = d.QueryRow("SELECT version FROM schema_version ORDER BY version DESC LIMIT 1").Scan(&version)
	if err != nil {
		t.Fatalf("query schema_version: %v", err)
	}
	if version < 1 {
		t.Errorf("version = %d, want >= 1", version)
	}
	// 关键表存在
	for _, tbl := range []string{"users", "auth_methods", "api_keys", "quotas",
		"upstream_providers", "custom_models", "model_prices", "request_logs",
		"settings", "sessions"} {
		var name string
		err = d.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&name)
		if err == sql.ErrNoRows {
			t.Errorf("table %q missing after migrate", tbl)
		} else if err != nil {
			t.Fatalf("check table %q: %v", tbl, err)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	d, _ := Open(":memory:")
	defer d.Close()
	if err := Migrate(d); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := Migrate(d); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/db/ -v
```

预期:编译失败。

- [ ] **Step 3: 实现 db 包**

`internal/db/db.go`:

```go
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func Open(dbPath string) (*sql.DB, error) {
	d, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// 启用外键 + WAL
	if _, err := d.Exec("PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL;"); err != nil {
		d.Close()
		return nil, fmt.Errorf("pragma: %w", err)
	}
	return d, nil
}
```

`internal/db/migrations.go`:

```go
package db

import (
	"database/sql"
	"fmt"
)

type migration struct {
	version int
	stmt    string
}

var migrations = []migration{
	{1, `
CREATE TABLE IF NOT EXISTS users (
    id              INTEGER PRIMARY KEY,
    email           TEXT UNIQUE NOT NULL,
    password_hash   TEXT,
    role            TEXT NOT NULL,
    status          TEXT NOT NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS auth_methods (
    id              INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(id),
    provider        TEXT NOT NULL,
    provider_uid    TEXT,
    secret          TEXT,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS api_keys (
    id              INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(id),
    key_hash        TEXT NOT NULL,
    key_prefix      TEXT NOT NULL,
    label           TEXT,
    status          TEXT NOT NULL,
    expires_at      TIMESTAMP,
    last_used_at    TIMESTAMP,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS quotas (
    id              INTEGER PRIMARY KEY,
    scope           TEXT NOT NULL,
    scope_id        INTEGER NOT NULL,
    period          TEXT NOT NULL,
    limit_tokens    INTEGER,
    limit_cost      REAL,
    used_tokens     INTEGER NOT NULL DEFAULT 0,
    used_cost       REAL NOT NULL DEFAULT 0,
    period_start    TIMESTAMP
);
CREATE TABLE IF NOT EXISTS upstream_providers (
    id              INTEGER PRIMARY KEY,
    name            TEXT NOT NULL,
    base_url        TEXT NOT NULL,
    api_key         TEXT NOT NULL,
    protocol        TEXT NOT NULL,
    status          TEXT NOT NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS custom_models (
    id              INTEGER PRIMARY KEY,
    name            TEXT UNIQUE NOT NULL,
    provider_id     INTEGER NOT NULL REFERENCES upstream_providers(id),
    upstream_model  TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT 1,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS model_prices (
    id              INTEGER PRIMARY KEY,
    model_id        INTEGER NOT NULL REFERENCES custom_models(id),
    input_price     REAL NOT NULL,
    output_price    REAL NOT NULL,
    cache_read_price REAL,
    cache_write_price REAL,
    currency        TEXT NOT NULL DEFAULT 'USD',
    effective_from  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS request_logs (
    id              INTEGER PRIMARY KEY,
    request_id      TEXT NOT NULL,
    user_id         INTEGER NOT NULL REFERENCES users(id),
    api_key_id      INTEGER NOT NULL REFERENCES api_keys(id),
    custom_model    TEXT NOT NULL,
    provider_id     INTEGER,
    upstream_model  TEXT,
    protocol_in     TEXT NOT NULL,
    protocol_out    TEXT NOT NULL,
    input_tokens    INTEGER NOT NULL DEFAULT 0,
    output_tokens   INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens   INTEGER NOT NULL DEFAULT 0,
    cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
    cost            REAL NOT NULL DEFAULT 0,
    duration_ms     INTEGER,
    status_code     INTEGER,
    error_type      TEXT NOT NULL DEFAULT 'none',
    error_message   TEXT,
    stream          BOOLEAN,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_request_logs_created ON request_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_request_logs_user ON request_logs(user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_request_logs_model ON request_logs(custom_model, created_at);

CREATE TABLE IF NOT EXISTS settings (
    key     TEXT PRIMARY KEY,
    value   TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id),
    token_hash  TEXT NOT NULL,
    expires_at  TIMESTAMP,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ip          TEXT,
    user_agent  TEXT
);
CREATE TABLE IF NOT EXISTS schema_version (
    version     INTEGER PRIMARY KEY,
    applied_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`},
}

func Migrate(d *sql.DB) error {
	if _, err := d.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
        version INTEGER PRIMARY KEY,
        applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
    );`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}
	var current int
	_ = d.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&current)
	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		tx, err := d.Begin()
		if err != nil {
			return fmt.Errorf("begin v%d: %w", m.version, err)
		}
		if _, err := tx.Exec(m.stmt); err != nil {
			tx.Rollback()
			return fmt.Errorf("exec v%d: %w", m.version, err)
		}
		if _, err := tx.Exec("INSERT INTO schema_version(version) VALUES (?)", m.version); err != nil {
			tx.Rollback()
			return fmt.Errorf("record v%d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit v%d: %w", m.version, err)
		}
	}
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/db/ -v
```

预期:3 个测试 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/db
git commit -m "feat(db): sqlite open + versioned migrations with full schema"
```

---

### Task 5: settings 键值表读写(settings 包)

**Files:**
- Create: `internal/settings/settings.go`
- Test: `internal/settings/settings_test.go`

**Interfaces:**
- Consumes: `*sql.DB`
- Produces: `settings.Store` 结构体,`New(db *sql.DB) *Store`;`(*Store) Get(key string) (string, bool, error)`;`(*Store) Set(key, value string) error`。本计划用 `listen_host` 键(`"0.0.0.0"`/`"127.0.0.1"`),默认 `"0.0.0.0"`(广播开);后续子项目复用此包存 `registration_open`、`force_2fa`、`log_retention_days` 等。

- [ ] **Step 1: 写失败测试**

`internal/settings/settings_test.go`:

```go
package settings

import (
	"testing"

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
	return New(d)
}

func TestSetAndGet(t *testing.T) {
	s := newStore(t)
	if err := s.Set("listen_host", "127.0.0.1"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, ok, err := s.Get("listen_host")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got != "127.0.0.1" {
		t.Errorf("got %q, want 127.0.0.1", got)
	}
}

func TestGetMissing(t *testing.T) {
	s := newStore(t)
	_, ok, err := s.Get("nope")
	if err != nil {
		t.Fatalf("Get err: %v", err)
	}
	if ok {
		t.Error("expected ok=false for missing key")
	}
}

func TestSetOverwrites(t *testing.T) {
	s := newStore(t)
	s.Set("k", "a")
	s.Set("k", "b")
	got, ok, _ := s.Get("k")
	if !ok || got != "b" {
		t.Errorf("got %q ok=%v, want b", got, ok)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/settings/ -v
```

预期:编译失败。

- [ ] **Step 3: 实现 settings 包**

`internal/settings/settings.go`:

```go
package settings

import (
	"database/sql"
	"errors"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Get(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key=?", key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return v, true, nil
}

func (s *Store) Set(key, value string) error {
	_, err := s.db.Exec(
		"INSERT INTO settings(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value",
		key, value)
	return err
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/settings/ -v
```

预期:3 个测试 PASS。

- [ ] **Step 5: 提交**

```bash
git add internal/settings
git commit -m "feat(settings): key-value settings store on sqlite"
```

---

### Task 6: 嵌入前端静态资源(web 包)

**Files:**
- Create: `web/embed.go`(package `web`)
- Test: `web/embed_test.go`
- Create: `web/dist/index.html`(占位产物,供 embed 测试)

**Interfaces:**
- Produces: `web.Handler() http.Handler`,服务 `web/dist` 下文件;对未匹配路径回退 `index.html`(SPA)。
- 说明:`//go:embed` 只能嵌入源文件所在包目录下的文件,所以 embed 源文件必须放在 `web/` 内,`//go:embed all:dist` 命中 `web/dist`。

- [ ] **Step 1: 创建占位 dist 产物**

`web/dist/index.html`:

```html
<!doctype html><html><head><meta charset="utf-8"><title>carryAPI</title></head>
<body><div id="app">carryAPI</div></body></html>
```

- [ ] **Step 2: 写失败测试**

`web/embed_test.go`:

```go
package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServesIndex(t *testing.T) {
	h := Handler()
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if body == "" {
		t.Error("index body empty")
	}
}

func TestSPAFallback(t *testing.T) {
	h := Handler()
	req := httptest.NewRequest("GET", "/some/spa/route", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200 (fallback)", rec.Code)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

```bash
go test ./web/ -v
```

预期:编译失败(找不到 `Handler`)。

- [ ] **Step 4: 实现 web embed 包**

`web/embed.go`:

```go
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// dist 不存在时 embed 编译就会失败,这里兜底理论不触发
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"frontend not built"}`, http.StatusServiceUnavailable)
		})
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(sub, p); err != nil {
			// 文件不存在 -> 回退 index.html(SPA 路由)
			data, err := fs.ReadFile(sub, "index.html")
			if err != nil {
				http.Error(w, "index.html missing", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(data)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
go test ./web/ -v
```

预期:2 个测试 PASS。

- [ ] **Step 6: 提交**

```bash
git add web/embed.go web/embed_test.go web/dist/index.html
git commit -m "feat(web): embed frontend dist with SPA fallback"
```

---

### Task 7: HTTP server 与广播监听器(server 包)

**Files:**
- Create: `internal/server/server.go`
- Create: `internal/server/listener.go`
- Create: `internal/server/router.go`
- Test: `internal/server/server_test.go`

**Interfaces:**
- Consumes: `config.Config`、`*sql.DB`、`settings.Store`
- Produces: `server.New(cfg config.Config, db *sql.DB, store *settings.Store) *Server`(db 存入 Server 供后续子项目的 handler 挂载使用);`(*Server) ListenAndServe() error`(监听地址由 settings `listen_host` 决定);`(*Server) Shutdown(ctx) error`。健康检查端点 `GET /api/health` 返回 `{"status":"ok"}`。
- 广播逻辑:`listen_host` = `"0.0.0.0"` -> 广播开(其他设备可访问);`"127.0.0.1"` -> 广播关(仅本机)。默认 `"0.0.0.0"`(settings 无值时)。

- [ ] **Step 1: 写失败测试**

`internal/server/server_test.go`:

```go
package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"carryapi/internal/config"
	"carryapi/internal/db"
	"carryapi/internal/settings"
)

func newServer(t *testing.T) *Server {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	cfg := config.Config{Port: 0, MasterKey: make([]byte, 32)}
	return New(cfg, d, settings.New(d))
}

func TestHealthEndpoint(t *testing.T) {
	s := newServer(t)
	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	if rec.Body.String() != `{"status":"ok"}`+"\n" {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestBroadcastOffListensLoopback(t *testing.T) {
	s := newServer(t)
	s.store.Set("listen_host", "127.0.0.1")
	addr, err := s.listenAddr()
	if err != nil {
		t.Fatalf("listenAddr: %v", err)
	}
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Errorf("addr = %q, want loopback", addr)
	}
}

func TestBroadcastOnListensAllInterfaces(t *testing.T) {
	s := newServer(t)
	s.store.Set("listen_host", "0.0.0.0")
	addr, _ := s.listenAddr()
	if !strings.HasPrefix(addr, "0.0.0.0:") {
		t.Errorf("addr = %q, want 0.0.0.0", addr)
	}
}

func TestShutdown(t *testing.T) {
	s := newServer(t)
	if err := s.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/server/ -v
```

预期:编译失败。

- [ ] **Step 3: 实现 listener.go**

`internal/server/listener.go`:

```go
package server

import (
	"fmt"
	"net"
	"strconv"

	"carryapi/internal/settings"
)

// listenHost 从 settings 读 listen_host;默认 0.0.0.0(广播开)
func listenHost(store *settings.Store) string {
	v, ok, _ := store.Get("listen_host")
	if !ok || v == "" {
		return "0.0.0.0"
	}
	return v
}

// listenAddr 返回最终监听 host:port
func (s *Server) listenAddr() (string, error) {
	host := listenHost(s.store)
	// Port=0 让系统分配端口(测试用)
	return net.JoinHostPort(host, strconv.Itoa(s.cfg.Port)), nil
}

// resolveListener 真正开 listener
func (s *Server) resolveListener() (net.Listener, error) {
	addr, err := s.listenAddr()
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	s.actualAddr = ln.Addr().String()
	return ln, nil
}
```

- [ ] **Step 4: 实现 router.go**

`internal/server/router.go`:

```go
package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"carryapi/web"
)

func (s *Server) buildRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/api/health", s.handleHealth)
	// 前端静态资源(SPA)
	r.Handle("/*", web.Handler())
	s.router = r
	return r
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
```

- [ ] **Step 5: 实现 server.go**

`internal/server/server.go`:

```go
package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"carryapi/internal/config"
	"carryapi/internal/settings"
)

type Server struct {
	cfg        config.Config
	db         *sql.DB
	store      *settings.Store
	httpServer *http.Server
	router     http.Handler
	actualAddr string
}

func New(cfg config.Config, db *sql.DB, store *settings.Store) *Server {
	s := &Server{cfg: cfg, db: db, store: store}
	s.buildRouter()
	s.httpServer = &http.Server{
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

func (s *Server) ListenAndServe() error {
	ln, err := s.resolveListener()
	if err != nil {
		return err
	}
	fmt.Printf("carryAPI listening on %s (broadcast=%s)\n", s.actualAddr, broadcastLabel(s.store))
	return s.httpServer.Serve(ln)
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func broadcastLabel(store *settings.Store) string {
	if listenHost(store) == "0.0.0.0" {
		return "ON (0.0.0.0, other devices can access)"
	}
	return "OFF (127.0.0.1, localhost only)"
}
```

- [ ] **Step 6: 运行测试确认通过**

```bash
go test ./internal/server/ -v
```

预期:4 个测试 PASS。

- [ ] **Step 7: 提交**

```bash
git add internal/server
git commit -m "feat(server): http server with broadcast listener and health check"
```

---

### Task 8: 程序入口(cmd/carryapi/main.go)

**Files:**
- Modify: `cmd/carryapi/main.go`

**Interfaces:**
- Consumes: `config.Load()`、`db.Open`、`db.Migrate`、`server.New`、`server.ListenAndServe`
- Produces:可运行二进制。处理信号(SIGINT/SIGTERM)做优雅关闭。

- [ ] **Step 1: 实现 main.go**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"carryapi/internal/config"
	"carryapi/internal/db"
	"carryapi/internal/server"
	"carryapi/internal/settings"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	d, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer d.Close()
	if err := db.Migrate(d); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	srv := server.New(cfg, d, settings.New(d))

	// 信号处理
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		fmt.Println("\nshutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
		log.Fatalf("serve: %v", err)
	}
}
```

- [ ] **Step 2: 构建并冒烟测试**

```bash
go build -o carryapi.exe ./cmd/carryapi
./carryapi.exe &
sleep 1
curl -s http://127.0.0.1:8080/api/health
kill %1
```

预期:输出 `{"status":"ok"}`,进程优雅退出。生成 `carryapi.db` 和 `carryapi.key` 文件。

- [ ] **Step 3: 提交**

```bash
git add cmd/carryapi/main.go
git commit -m "feat(cmd): main entrypoint with graceful shutdown"
```

---

### Task 9: Vue3 前端占位工程

**Files:**
- Create: `web/package.json`
- Create: `web/vite.config.ts`
- Create: `web/index.html`
- Create: `web/src/main.ts`
- Create: `web/tsconfig.json`
- Modify: `web/dist/index.html`(由构建覆盖)

**Interfaces:**
- Produces:`web/dist/` 构建产物,被 `web` 包 `//go:embed` 嵌入。本计划只做占位页(显示 "carryAPI" + 版本号),后续前端子项目扩展为完整管理后台。
- 开发代理:`vite.config.ts` 把 `/api` 代理到 `http://127.0.0.1:8080`。

- [ ] **Step 1: 初始化前端工程**

```bash
cd /d/Projects/carryAPI/web
npm init -y
npm install vue
npm install -D vite @vitejs/plugin-vue typescript naive-ui
```

- [ ] **Step 2: 写 package.json scripts**

编辑 `web/package.json`,确保有:

```json
{
  "name": "carryapi-web",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "vue": "^3.4.0"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.0.0",
    "naive-ui": "^2.38.0",
    "typescript": "^5.4.0",
    "vite": "^5.4.0"
  }
}
```

- [ ] **Step 3: 写配置文件**

`web/vite.config.ts`:

```ts
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080',
      '/v1': 'http://127.0.0.1:8080',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
```

`web/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"]
  },
  "include": ["src"]
}
```

`web/index.html`:

```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>carryAPI</title>
</head>
<body>
  <div id="app"></div>
  <script type="module" src="/src/main.ts"></script>
</body>
</html>
```

- [ ] **Step 4: 写占位页**

`web/src/main.ts`:

```ts
import { createApp, h } from 'vue'
import { NMessageProvider, NCard, NConfigProvider } from 'naive-ui'

const App = {
  render() {
    return h(NConfigProvider, null, {
      default: () => h(NMessageProvider, null, {
        default: () => h(NCard, { title: 'carryAPI' }, {
          default: () => 'carryAPI management console — coming soon'
        })
      })
    })
  }
}

createApp(App).mount('#app')
```

- [ ] **Step 5: 构建并验证产物**

```bash
cd /d/Projects/carryAPI/web
npm run build
ls dist/
```

预期:`dist/` 含 `index.html`、`assets/`。

- [ ] **Step 6: 重新构建后端验证嵌入**

```bash
cd /d/Projects/carryAPI
go build -o carryapi.exe ./cmd/carryapi
./carryapi.exe &
sleep 1
curl -s http://127.0.0.1:8080/
kill %1
```

预期:返回构建出的 HTML(含 carryAPI 卡片)。

- [ ] **Step 7: 更新 .gitignore 并提交**

`.gitignore` 增加:

```
web/node_modules/
```

```bash
git add web/package.json web/vite.config.ts web/tsconfig.json web/index.html web/src web/dist .gitignore
git commit -m "feat(web): vue3 + naive-ui placeholder scaffold"
```

---

### Task 10: 交叉编译验证

**Files:**
- 无新文件;验证构建矩阵。

- [ ] **Step 1: Linux x64 交叉编译**

```bash
cd /d/Projects/carryAPI
GOOS=linux GOARCH=amd64 go build -o carryapi-linux-amd64 ./cmd/carryapi
ls -la carryapi-linux-amd64
```

预期:生成二进制,无 CGO 错误。

- [ ] **Step 2: Windows x64 编译**

```bash
GOOS=windows GOARCH=amd64 go build -o carryapi-windows-amd64.exe ./cmd/carryapi
ls -la carryapi-windows-amd64.exe
```

预期:生成 `.exe`。

- [ ] **Step 3: 验证无 CGO 依赖**

```bash
go build -ldflags="-linkmode=external" ./... 2>&1 | head -5 || true
file carryapi-linux-amd64 2>/dev/null || true
```

确认 `modernc.org/sqlite` 未引入 CGO(构建无需 `CGO_ENABLED=1`)。

- [ ] **Step 4: 更新 .gitignore 忽略产物二进制**

`.gitignore` 增加:

```
carryapi
carryapi.exe
carryapi-linux-amd64
carryapi-windows-amd64.exe
```

- [ ] **Step 5: 提交**

```bash
git add .gitignore
git commit -m "build: verify cross-compilation for linux/windows x64"
```

---

### Task 11: 全量测试与 README

**Files:**
- Create: `README.md`

- [ ] **Step 1: 运行全部测试**

```bash
cd /d/Projects/carryAPI
go test ./... -v
```

预期:所有包测试 PASS(config / crypto / db / settings / web / server)。

- [ ] **Step 2: 写 README**

`README.md`:

````markdown
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
````

- [ ] **Step 3: 提交**

```bash
git add README.md
git commit -m "docs: readme for skeleton milestone"
```

---

## 子项目 1 完成标准

- [ ] `go test ./...` 全绿
- [ ] `carryapi` 二进制可启动,`GET /api/health` 返回 `{"status":"ok"}`
- [ ] 前端占位页可访问(`/` 返回 carryAPI 卡片)
- [ ] 广播开关逻辑正确:`listen_host=0.0.0.0` 监听全接口,`127.0.0.1` 仅本机
- [ ] 交叉编译 Linux/Windows x64 二进制成功,无 CGO
- [ ] 首次启动自动建表 + 生成主密钥
