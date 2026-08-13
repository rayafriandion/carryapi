package catalog

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type ModelBinding struct {
	ID            int64
	ModelID       int64
	ProviderID    int64
	UpstreamModel string
	Priority      int
	Weight        int
	Enabled       bool
	CreatedAt     time.Time
}

type ModelBindingStore struct {
	db *sql.DB
}

func NewModelBindingStore(db *sql.DB) *ModelBindingStore {
	return &ModelBindingStore{db: db}
}

func (s *ModelBindingStore) Create(modelID, providerID int64, upstreamModel string, priority, weight int, enabled bool) (ModelBinding, error) {
	if upstreamModel == "" {
		return ModelBinding{}, errors.New("upstream model is required")
	}
	if priority <= 0 {
		priority = 100
	}
	if weight <= 0 {
		weight = 1
	}
	res, err := s.db.Exec(
		`INSERT INTO model_bindings(model_id, provider_id, upstream_model, priority, weight, enabled)
		 VALUES(?, ?, ?, ?, ?, ?)`,
		modelID, providerID, upstreamModel, priority, weight, enabled)
	if err != nil {
		return ModelBinding{}, fmt.Errorf("create model binding: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

func (s *ModelBindingStore) Get(id int64) (ModelBinding, error) {
	return s.scan(s.db.QueryRow(
		`SELECT id, model_id, provider_id, upstream_model, priority, weight, enabled, created_at
		 FROM model_bindings WHERE id=?`, id))
}

func (s *ModelBindingStore) ListByModel(modelID int64) ([]ModelBinding, error) {
	rows, err := s.db.Query(
		`SELECT id, model_id, provider_id, upstream_model, priority, weight, enabled, created_at
		 FROM model_bindings WHERE model_id=? ORDER BY priority, id`, modelID)
	if err != nil {
		return nil, fmt.Errorf("list model bindings: %w", err)
	}
	defer rows.Close()
	var out []ModelBinding
	for rows.Next() {
		b, err := scanModelBinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *ModelBindingStore) ListByProvider(providerID int64) ([]ModelBinding, error) {
	rows, err := s.db.Query(
		`SELECT id, model_id, provider_id, upstream_model, priority, weight, enabled, created_at
		 FROM model_bindings WHERE provider_id=? ORDER BY id`, providerID)
	if err != nil {
		return nil, fmt.Errorf("list by provider: %w", err)
	}
	defer rows.Close()
	var out []ModelBinding
	for rows.Next() {
		b, err := scanModelBinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *ModelBindingStore) ListEnabledByModel(modelID int64) ([]ModelBinding, error) {
	rows, err := s.db.Query(
		`SELECT id, model_id, provider_id, upstream_model, priority, weight, enabled, created_at
		 FROM model_bindings WHERE model_id=? AND enabled=1 ORDER BY priority, id`, modelID)
	if err != nil {
		return nil, fmt.Errorf("list enabled model bindings: %w", err)
	}
	defer rows.Close()
	var out []ModelBinding
	for rows.Next() {
		b, err := scanModelBinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *ModelBindingStore) Update(id int64, providerID int64, upstreamModel string, priority, weight int, enabled bool) error {
	if priority <= 0 {
		priority = 100
	}
	if weight <= 0 {
		weight = 1
	}
	_, err := s.db.Exec(
		`UPDATE model_bindings SET provider_id=?, upstream_model=?, priority=?, weight=?, enabled=? WHERE id=?`,
		providerID, upstreamModel, priority, weight, enabled, id)
	return err
}

func (s *ModelBindingStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM model_bindings WHERE id=?`, id)
	return err
}

func (s *ModelBindingStore) CountByModel(modelID int64) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM model_bindings WHERE model_id=?`, modelID).Scan(&count)
	return count, err
}

func (s *ModelBindingStore) scan(row *sql.Row) (ModelBinding, error) {
	var b ModelBinding
	err := row.Scan(&b.ID, &b.ModelID, &b.ProviderID, &b.UpstreamModel, &b.Priority, &b.Weight, &b.Enabled, &b.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ModelBinding{}, errors.New("model binding not found")
	}
	return b, err
}

func scanModelBinding(r interface{ Scan(...any) error }) (ModelBinding, error) {
	var b ModelBinding
	err := r.Scan(&b.ID, &b.ModelID, &b.ProviderID, &b.UpstreamModel, &b.Priority, &b.Weight, &b.Enabled, &b.CreatedAt)
	return b, err
}
