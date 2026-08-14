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
`},
	{2, `
-- 鉴权失败等无用户上下文的请求也要记 request_logs(用于错误率监控),
-- 故 user_id / api_key_id 允许 NULL。
ALTER TABLE request_logs ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE request_logs ALTER COLUMN api_key_id DROP NOT NULL;
`},
	{3, `
CREATE TABLE IF NOT EXISTS model_bindings (
    id              INTEGER PRIMARY KEY,
    model_id        INTEGER NOT NULL REFERENCES custom_models(id) ON DELETE CASCADE,
    provider_id     INTEGER NOT NULL REFERENCES upstream_providers(id),
    upstream_model  TEXT NOT NULL,
    priority        INTEGER NOT NULL DEFAULT 100,
    weight          INTEGER NOT NULL DEFAULT 1,
    enabled         BOOLEAN NOT NULL DEFAULT 1,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(model_id, provider_id, upstream_model)
);
CREATE INDEX IF NOT EXISTS idx_model_bindings_model ON model_bindings(model_id, enabled, priority);
CREATE INDEX IF NOT EXISTS idx_model_bindings_provider ON model_bindings(provider_id);

INSERT INTO model_bindings(model_id, provider_id, upstream_model, priority, weight, enabled)
SELECT id, provider_id, upstream_model, 100, 1, enabled
FROM custom_models
WHERE NOT EXISTS (
    SELECT 1 FROM model_bindings mb WHERE mb.model_id = custom_models.id
);

ALTER TABLE custom_models ADD COLUMN routing_strategy TEXT NOT NULL DEFAULT 'auto';
ALTER TABLE custom_models ADD COLUMN auto_mode TEXT NOT NULL DEFAULT 'priority';
`},
	{4, `
ALTER TABLE request_logs ADD COLUMN ttft_ms INTEGER;
CREATE INDEX IF NOT EXISTS idx_request_logs_provider_model
    ON request_logs(provider_id, upstream_model, created_at);
`},
	{5, `
-- 供应商多 API Key 池:同一供应商可配置多个上游 key,
-- 按优先级(priority)与用户亲和(cache 命中)选择,失败自动降级/冷却/删除。
CREATE TABLE IF NOT EXISTS provider_api_keys (
    id              INTEGER PRIMARY KEY,
    provider_id     INTEGER NOT NULL REFERENCES upstream_providers(id) ON DELETE CASCADE,
    key_enc         TEXT NOT NULL,
    label           TEXT NOT NULL DEFAULT '',
    priority        INTEGER NOT NULL DEFAULT 100,
    base_priority   INTEGER NOT NULL DEFAULT 100,
    status          TEXT NOT NULL DEFAULT 'active',
    fail_count      INTEGER NOT NULL DEFAULT 0,
    retry_after     TIMESTAMP,
    last_used_at    TIMESTAMP,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at      TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_provider_api_keys_provider
    ON provider_api_keys(provider_id, status, priority);

-- 用户-上游 key 亲和:记录某用户最近使用过的上游 key,
-- 保证同一用户的请求尽量落在同一上游 key 上以提高缓存命中率。
CREATE TABLE IF NOT EXISTS provider_key_prefs (
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_id          INTEGER NOT NULL REFERENCES provider_api_keys(id) ON DELETE CASCADE,
    last_used_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(user_id, key_id)
);
CREATE INDEX IF NOT EXISTS idx_provider_key_prefs_key ON provider_key_prefs(key_id);

-- 上游 key 生命周期审计日志(创建/降级/冷却/重试/恢复/删除),即"API key 调用日志"。
CREATE TABLE IF NOT EXISTS provider_api_key_events (
    id              INTEGER PRIMARY KEY,
    key_id          INTEGER NOT NULL REFERENCES provider_api_keys(id) ON DELETE CASCADE,
    provider_id     INTEGER NOT NULL REFERENCES upstream_providers(id) ON DELETE CASCADE,
    event           TEXT NOT NULL,
    detail          TEXT,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_provider_key_events_key ON provider_api_key_events(key_id, created_at);

-- 存量单 key 迁移为池中第一个 key(幂等:跳过已迁移的 provider)。
INSERT INTO provider_api_keys(provider_id, key_enc, label, priority, base_priority, status)
SELECT id, api_key, '', 100, 100, 'active' FROM upstream_providers
WHERE id NOT IN (SELECT DISTINCT provider_id FROM provider_api_keys);

-- 请求日志记录实际命中的上游 key(用于 API key 调用日志)。
ALTER TABLE request_logs ADD COLUMN provider_api_key_id INTEGER;
CREATE INDEX IF NOT EXISTS idx_request_logs_provider_key ON request_logs(provider_api_key_id);
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
