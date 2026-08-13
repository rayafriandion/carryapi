package db

import (
	"database/sql"
	"testing"
)

func TestMigrationV4AddsTtftColumn(t *testing.T) {
	d, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()
	if err := Migrate(d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	var cid string
	err = d.QueryRow(`SELECT name FROM pragma_table_info('request_logs') WHERE name='ttft_ms'`).Scan(&cid)
	if err == sql.ErrNoRows {
		t.Fatal("ttft_ms column not found")
	}
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// 验证索引
	var idxName string
	err = d.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_request_logs_provider_model'`).Scan(&idxName)
	if err != nil {
		t.Fatalf("index not found: %v", err)
	}
}
