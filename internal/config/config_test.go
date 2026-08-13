package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("CARRYAPI_HOST", "::")
	t.Setenv("CARRYAPI_PORT", "9090")
	t.Setenv("CARRYAPI_DB_PATH", "/tmp/test.db")
	t.Setenv("CARRYAPI_MASTER_KEY", "0123456789abcdef0123456789abcdef")
	cfg, err := LoadWithArgs(nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Host != "::" || cfg.ListenHostFrom != "env" {
		t.Errorf("Host = %q source=%q, want :: env", cfg.Host, cfg.ListenHostFrom)
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
	t.Setenv("CARRYAPI_HOST", "")
	t.Setenv("CARRYAPI_PORT", "")
	t.Setenv("CARRYAPI_DB_PATH", "")
	t.Setenv("CARRYAPI_MASTER_KEY", "")
	keyFile := filepath.Join(t.TempDir(), "carryapi.key")
	t.Setenv("CARRYAPI_KEY_FILE", keyFile)

	cfg, err := LoadWithArgs(nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Port != 8067 {
		t.Errorf("default Port = %d, want 8067", cfg.Port)
	}
	if cfg.Host != "" || cfg.ListenHostSet {
		t.Errorf("default Host = %q set=%v, want empty/unset", cfg.Host, cfg.ListenHostSet)
	}
	if cfg.DBPath != "./carryapi.db" {
		t.Errorf("default DBPath = %q, want ./carryapi.db", cfg.DBPath)
	}
	if len(cfg.MasterKey) != 32 {
		t.Errorf("generated MasterKey len = %d, want 32", len(cfg.MasterKey))
	}
	if _, err := os.Stat(keyFile); err != nil {
		t.Errorf("carryapi.key not created: %v", err)
	}
}

func TestMasterKeyInvalidLength(t *testing.T) {
	t.Setenv("CARRYAPI_MASTER_KEY", "short")
	_, err := LoadWithArgs(nil)
	if err == nil {
		t.Error("expected error for short master key, got nil")
	}
}

func TestInvalidHostRejected(t *testing.T) {
	t.Setenv("CARRYAPI_MASTER_KEY", "0123456789abcdef0123456789abcdef")
	_, err := LoadWithArgs([]string{"--host", "192.168.1.10"})
	if err == nil {
		t.Error("expected invalid host error, got nil")
	}
}

func TestLoadFromEnvKeyFile(t *testing.T) {
	t.Setenv("CARRYAPI_MASTER_KEY", "")
	keyFile := filepath.Join(t.TempDir(), "existing.key")
	if err := os.WriteFile(keyFile, bytes.Repeat([]byte{7}, 32), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("CARRYAPI_KEY_FILE", keyFile)

	cfg, err := LoadWithArgs(nil)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !bytes.Equal(cfg.MasterKey, bytes.Repeat([]byte{7}, 32)) {
		t.Errorf("MasterKey = %v, want 32 bytes of 7", cfg.MasterKey)
	}
}

func TestLoadFlagsOverrideEnv(t *testing.T) {
	t.Setenv("CARRYAPI_HOST", "127.0.0.1")
	t.Setenv("CARRYAPI_PORT", "8080")
	t.Setenv("CARRYAPI_MASTER_KEY", "0123456789abcdef0123456789abcdef")
	cfg, err := LoadWithArgs([]string{"--host", "::1", "--port", "9091"})
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Host != "::1" || cfg.ListenHostFrom != "flag" {
		t.Errorf("Host = %q source=%q, want ::1 flag", cfg.Host, cfg.ListenHostFrom)
	}
	if cfg.Port != 9091 {
		t.Errorf("Port = %d, want 9091", cfg.Port)
	}
}
