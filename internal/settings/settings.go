package settings

import (
	"database/sql"
	"errors"

	"carryapi/internal/currency"
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

// CurrencyKey 系统统一币种的 settings 键。
const CurrencyKey = "currency"

// Currency 返回系统统一币种代码(未设置时返回默认 USD)。
func (s *Store) Currency() (string, error) {
	v, ok, err := s.Get(CurrencyKey)
	if err != nil {
		return "", err
	}
	if !ok || v == "" {
		return currency.Default, nil
	}
	return currency.Normalize(v), nil
}

// SetCurrency 设置系统统一币种代码(应已通过 currency.Valid 校验)。
func (s *Store) SetCurrency(code string) error {
	return s.Set(CurrencyKey, code)
}
