package catalog

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const (
	RoutingStrategyAuto   = "auto"
	RoutingStrategyRandom = "random"

	AutoModePriority = "priority"
	AutoModeFailover = "failover"
	AutoModeHealth   = "health"
)

type Model struct {
	ID              int64
	Name            string
	ProviderID      int64
	UpstreamModel   string
	Enabled         bool
	RoutingStrategy string
	AutoMode        string
	CreatedAt       time.Time
}

type ModelStore struct {
	db *sql.DB
}

func NewModelStore(db *sql.DB) *ModelStore {
	return &ModelStore{db: db}
}

func (s *ModelStore) Create(name string, providerID int64, upstreamModel string) (Model, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Model{}, fmt.Errorf("create model: %w", err)
	}
	defer tx.Rollback()
	id, err := s.createInTx(tx, name, providerID, upstreamModel, true)
	if err != nil {
		return Model{}, err
	}
	if err := tx.Commit(); err != nil {
		return Model{}, fmt.Errorf("create model: %w", err)
	}
	return s.Get(id)
}

// CreateInTx 在已有事务中创建模型(及首条绑定),返回新模型 id。
func (s *ModelStore) CreateInTx(tx *sql.Tx, name string, providerID int64, upstreamModel string, enabled bool) (int64, error) {
	return s.createInTx(tx, name, providerID, upstreamModel, enabled)
}

func (s *ModelStore) createInTx(tx *sql.Tx, name string, providerID int64, upstreamModel string, enabled bool) (int64, error) {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	res, err := tx.Exec(
		`INSERT INTO custom_models(name, provider_id, upstream_model, enabled, routing_strategy, auto_mode)
		 VALUES(?, ?, ?, ?, ?, ?)`,
		name, providerID, upstreamModel, enabledInt, RoutingStrategyAuto, AutoModePriority)
	if err != nil {
		return 0, fmt.Errorf("create model: %w", err)
	}
	id, _ := res.LastInsertId()
	if _, err := tx.Exec(
		`INSERT INTO model_bindings(model_id, provider_id, upstream_model, priority, weight, enabled)
		 VALUES(?, ?, ?, 100, 1, ?)`,
		id, providerID, upstreamModel, enabledInt); err != nil {
		return 0, fmt.Errorf("create model binding: %w", err)
	}
	return id, nil
}

func (s *ModelStore) CreateDraft(providerID int64, upstreamModel string) (Model, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Model{}, fmt.Errorf("create draft model: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO custom_models(name, provider_id, upstream_model, enabled, routing_strategy, auto_mode)
		 VALUES(?, ?, ?, 0, ?, ?)`,
		upstreamModel, providerID, upstreamModel, RoutingStrategyAuto, AutoModePriority)
	if err != nil {
		return Model{}, fmt.Errorf("create draft model: %w", err)
	}
	id, _ := res.LastInsertId()
	if _, err := tx.Exec(
		`INSERT INTO model_bindings(model_id, provider_id, upstream_model, priority, weight, enabled)
		 VALUES(?, ?, ?, 100, 1, 0)`,
		id, providerID, upstreamModel); err != nil {
		return Model{}, fmt.Errorf("create draft model binding: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Model{}, fmt.Errorf("create draft model: %w", err)
	}
	return s.Get(id)
}

func (s *ModelStore) Get(id int64) (Model, error) {
	return s.scan(s.db.QueryRow(
		`SELECT id, name, provider_id, upstream_model, enabled, COALESCE(routing_strategy, 'auto'), COALESCE(auto_mode, 'priority'), created_at
		 FROM custom_models WHERE id=?`, id))
}

func (s *ModelStore) GetByName(name string) (Model, error) {
	return s.scan(s.db.QueryRow(
		`SELECT id, name, provider_id, upstream_model, enabled, COALESCE(routing_strategy, 'auto'), COALESCE(auto_mode, 'priority'), created_at
		 FROM custom_models WHERE name=?`, name))
}

func (s *ModelStore) List() ([]Model, error) {
	return s.listWhere(`1=1`)
}

func (s *ModelStore) ListEnabled() ([]Model, error) {
	return s.listWhere(`enabled=1`)
}

func (s *ModelStore) listWhere(cond string) ([]Model, error) {
	rows, err := s.db.Query(
		`SELECT id, name, provider_id, upstream_model, enabled, COALESCE(routing_strategy, 'auto'), COALESCE(auto_mode, 'priority'), created_at
		 FROM custom_models WHERE ` + cond + ` ORDER BY id`)
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

func (s *ModelStore) Update(id int64, name string, enabled bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.updateInTx(tx, id, name, enabled); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateInTx 在已有事务中更新模型的 name + enabled。binding 不再由此处维护,
// 改由 RoutingView 的 binding CRUD API 专门管理。
func (s *ModelStore) UpdateInTx(tx *sql.Tx, id int64, name string, enabled bool) error {
	return s.updateInTx(tx, id, name, enabled)
}

func (s *ModelStore) updateInTx(tx *sql.Tx, id int64, name string, enabled bool) error {
	_, err := tx.Exec(
		`UPDATE custom_models SET name=?, enabled=? WHERE id=?`,
		name, enabled, id)
	return err
}

func (s *ModelStore) UpdateRouting(id int64, routingStrategy, autoMode string) error {
	return s.updateRouting(s.db, id, routingStrategy, autoMode)
}

// UpdateRoutingTx 在已有事务中更新模型路由策略。
func (s *ModelStore) UpdateRoutingTx(tx *sql.Tx, id int64, routingStrategy, autoMode string) error {
	return s.updateRouting(tx, id, routingStrategy, autoMode)
}

func (s *ModelStore) updateRouting(exe sqlExecutor, id int64, routingStrategy, autoMode string) error {
	if routingStrategy == "" {
		routingStrategy = RoutingStrategyAuto
	}
	if autoMode == "" {
		autoMode = AutoModePriority
	}
	if routingStrategy != RoutingStrategyAuto && routingStrategy != RoutingStrategyRandom {
		return errors.New("invalid routing strategy")
	}
	if autoMode != AutoModePriority && autoMode != AutoModeFailover && autoMode != AutoModeHealth {
		return errors.New("invalid auto mode")
	}
	_, err := exe.Exec(`UPDATE custom_models SET routing_strategy=?, auto_mode=? WHERE id=?`, routingStrategy, autoMode, id)
	return err
}

func (s *ModelStore) Delete(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM model_bindings WHERE model_id=?`, id); err != nil {
		tx.Rollback()
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
	err := row.Scan(&m.ID, &m.Name, &m.ProviderID, &m.UpstreamModel, &m.Enabled, &m.RoutingStrategy, &m.AutoMode, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Model{}, errors.New("model not found")
	}
	return m, err
}

func (s *ModelStore) scanRows(r interface{ Scan(...any) error }) (Model, error) {
	var m Model
	err := r.Scan(&m.ID, &m.Name, &m.ProviderID, &m.UpstreamModel, &m.Enabled, &m.RoutingStrategy, &m.AutoMode, &m.CreatedAt)
	return m, err
}
