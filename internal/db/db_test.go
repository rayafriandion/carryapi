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
