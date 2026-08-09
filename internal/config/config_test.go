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
