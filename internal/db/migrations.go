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
