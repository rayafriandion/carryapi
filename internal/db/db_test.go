package db

import (
	"context"
	"database/sql"
	"path/filepath"
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

func TestForeignKeyEnforcedAcrossPool(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test_fk.db")
	d, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()
	d.SetMaxOpenConns(4)
	if err := Migrate(d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		// api_keys.user_id REFERENCES users(id); user_id 9999 does not exist
		_, err := d.ExecContext(ctx, `INSERT INTO api_keys(user_id, key_hash, key_prefix, status) VALUES (9999, 'h', 'p', 'active')`)
		if err == nil {
			t.Fatalf("iteration %d: FK violation was allowed (foreign_keys not enforced on pooled connection)", i)
		}
	}
}
