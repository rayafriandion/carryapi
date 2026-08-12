package catalog

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Model struct {
	ID            int64
	Name          string // 对外暴露的自定义名
	ProviderID    int64
	UpstreamModel string
	Enabled       bool
	CreatedAt     time.Time
}

type ModelStore struct {
	db *sql.DB
}

func NewModelStore(db *sql.DB) *ModelStore {
	return &ModelStore{db: db}
}

func (s *ModelStore) Create(name string, providerID int64, upstreamModel string) (Model, error) {
	res, err := s.db.Exec(
		`INSERT INTO custom_models(name, provider_id, upstream_model, enabled) VALUES(?, ?, ?, 1)`,
		name, providerID, upstreamModel)
	if err != nil {
		return Model{}, fmt.Errorf("create model: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

// CreateDraft 创建禁用态草稿模型(enabled=0),名称取上游模型名。
func (s *ModelStore) CreateDraft(providerID int64, upstreamModel string) (Model, error) {
	res, err := s.db.Exec(
		`INSERT INTO custom_models(name, provider_id, upstream_model, enabled) VALUES(?, ?, ?, 0)`,
		upstreamModel, providerID, upstreamModel)
	if err != nil {
		return Model{}, fmt.Errorf("create draft model: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

func (s *ModelStore) Get(id int64) (Model, error) {
	return s.scan(s.db.QueryRow(
		`SELECT id, name, provider_id, upstream_model, enabled, created_at FROM custom_models WHERE id=?`, id))
}

func (s *ModelStore) GetByName(name string) (Model, error) {
	return s.scan(s.db.QueryRow(
		`SELECT id, name, provider_id, upstream_model, enabled, created_at FROM custom_models WHERE name=?`, name))
}

func (s *ModelStore) List() ([]Model, error) {
	return s.listWhere(`1=1`)
}

func (s *ModelStore) ListEnabled() ([]Model, error) {
	return s.listWhere(`enabled=1`)
}

func (s *ModelStore) listWhere(cond string) ([]Model, error) {
	rows, err := s.db.Query(
		`SELECT id, name, provider_id, upstream_model, enabled, created_at FROM custom_models WHERE ` + cond + ` ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer rows.Close()
	var out []Model
	for rows.Next() {
		m, err := s.scanRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *ModelStore) Update(id int64, name string, providerID int64, upstreamModel string, enabled bool) error {
	_, err := s.db.Exec(
		`UPDATE custom_models SET name=?, provider_id=?, upstream_model=?, enabled=? WHERE id=?`,
		name, providerID, upstreamModel, enabled, id)
	return err
}

func (s *ModelStore) Delete(id int64) error {
	// 先删价格,再删模型
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM model_prices WHERE model_id=?`, id); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM custom_models WHERE id=?`, id); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *ModelStore) scan(row *sql.Row) (Model, error) {
	var m Model
	err := row.Scan(&m.ID, &m.Name, &m.ProviderID, &m.UpstreamModel, &m.Enabled, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Model{}, errors.New("model not found")
	}
	return m, err
}

func (s *ModelStore) scanRows(r interface{ Scan(...any) error }) (Model, error) {
	var m Model
	err := r.Scan(&m.ID, &m.Name, &m.ProviderID, &m.UpstreamModel, &m.Enabled, &m.CreatedAt)
	return m, err
}
