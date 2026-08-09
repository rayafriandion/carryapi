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
