package user

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type Quota struct {
	ID          int64
	Scope       string
	ScopeID     int64
	Period      string
	LimitTokens *int64
	LimitCost   *float64
	UsedTokens  int64
	UsedCost    float64
	PeriodStart *time.Time
}

func (s *Store) SetQuota(q Quota) (Quota, error) {
	res, err := s.db.Exec(
		`INSERT INTO quotas(scope, scope_id, period, limit_tokens, limit_cost, period_start)
		 VALUES(?, ?, ?, ?, ?, ?)`,
		q.Scope, q.ScopeID, q.Period, q.LimitTokens, q.LimitCost, time.Now())
	if err != nil {
		return Quota{}, fmt.Errorf("set quota: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetQuota(id)
}

func (s *Store) GetQuotas(scope string, scopeID int64) ([]Quota, error) {
	rows, err := s.db.Query(
		`SELECT id, scope, scope_id, period, limit_tokens, limit_cost, used_tokens, used_cost, period_start
		 FROM quotas WHERE scope=? AND scope_id=?`, scope, scopeID)
	if err != nil {
		return nil, fmt.Errorf("get quotas: %w", err)
	}
	defer rows.Close()
	var out []Quota
	for rows.Next() {
		q, err := scanQuota(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (s *Store) GetQuota(id int64) (Quota, error) {
	row := s.db.QueryRow(
		`SELECT id, scope, scope_id, period, limit_tokens, limit_cost, used_tokens, used_cost, period_start
		 FROM quotas WHERE id=?`, id)
	q, err := scanQuota(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Quota{}, fmt.Errorf("quota not found: %w", err)
	}
	return q, err
}

func (s *Store) UpdateQuota(id int64, limitTokens *int64, limitCost *float64) error {
	_, err := s.db.Exec(
		`UPDATE quotas SET limit_tokens=?, limit_cost=? WHERE id=?`, limitTokens, limitCost, id)
	return err
}

func (s *Store) DeleteQuota(id int64) error {
	_, err := s.db.Exec(`DELETE FROM quotas WHERE id=?`, id)
	return err
}

// GetModelQuota 返回某模型的配额(scope="model")。未设置时返回零值 Quota(其 ID==0)。
func (s *Store) GetModelQuota(modelID int64) (Quota, error) {
	return s.getScopeQuota("model", modelID)
}

// GetKeyQuota 返回某 API Key 的配额(scope="key")。未设置时返回零值 Quota(其 ID==0)。
func (s *Store) GetKeyQuota(keyID int64) (Quota, error) {
	return s.getScopeQuota("key", keyID)
}

func (s *Store) getScopeQuota(scope string, scopeID int64) (Quota, error) {
	quotas, err := s.GetQuotas(scope, scopeID)
	if err != nil {
		return Quota{}, err
	}
	if len(quotas) == 0 {
		return Quota{}, nil
	}
	return quotas[0], nil
}

// SetModelQuota 设置/更新某模型的配额(upsert,scope="model")。
// limitTokens/limitCost 为 nil 表示该维度不限制;两者均为 nil 时删除该配额记录。
func (s *Store) SetModelQuota(modelID int64, period string, limitTokens *int64, limitCost *float64) (Quota, error) {
	return s.setScopeQuota("model", modelID, period, limitTokens, limitCost)
}

// SetKeyQuota 设置/更新某 API Key 的配额(upsert,scope="key")。
// limitTokens/limitCost 为 nil 表示该维度不限制;两者均为 nil 时删除该配额记录。
func (s *Store) SetKeyQuota(keyID int64, period string, limitTokens *int64, limitCost *float64) (Quota, error) {
	return s.setScopeQuota("key", keyID, period, limitTokens, limitCost)
}

func (s *Store) setScopeQuota(scope string, scopeID int64, period string, limitTokens *int64, limitCost *float64) (Quota, error) {
	if period == "" {
		period = "total"
	}
	existing, err := s.GetQuotas(scope, scopeID)
	if err != nil {
		return Quota{}, err
	}
	if len(existing) > 0 {
		q := existing[0]
		if limitTokens == nil && limitCost == nil {
			if err := s.DeleteQuota(q.ID); err != nil {
				return Quota{}, err
			}
			return Quota{}, nil
		}
		if _, err := s.db.Exec(
			`UPDATE quotas SET period=?, limit_tokens=?, limit_cost=? WHERE id=?`,
			period, limitTokens, limitCost, q.ID); err != nil {
			return Quota{}, fmt.Errorf("set %s quota: %w", scope, err)
		}
		return s.GetQuota(q.ID)
	}
	if limitTokens == nil && limitCost == nil {
		return Quota{}, nil
	}
	return s.SetQuota(Quota{
		Scope: scope, ScopeID: scopeID, Period: period,
		LimitTokens: limitTokens, LimitCost: limitCost,
	})
}

// DeleteModelQuota 删除某模型的全部配额记录(模型删除时清理)。
func (s *Store) DeleteModelQuota(modelID int64) error {
	return s.deleteScopeQuota("model", modelID)
}

// DeleteKeyQuota 删除某 API Key 的全部配额记录(Key 删除时清理)。
func (s *Store) DeleteKeyQuota(keyID int64) error {
	return s.deleteScopeQuota("key", keyID)
}

func (s *Store) deleteScopeQuota(scope string, scopeID int64) error {
	_, err := s.db.Exec(`DELETE FROM quotas WHERE scope=? AND scope_id=?`, scope, scopeID)
	return err
}

func (s *Store) IncrementUsage(scope string, scopeID int64, tokens int64, cost float64) error {
	// 原子累加;周期重置在子项目 4 调用时按 period 判断(此处先简单累加,周期重置留 TODO 由子项目4封装)
	// 用事务保证原子
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	_, err = tx.Exec(
		`UPDATE quotas SET used_tokens = used_tokens + ?, used_cost = used_cost + ? WHERE scope=? AND scope_id=?`,
		tokens, cost, scope, scopeID)
	if err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

type quotaScanner interface {
	Scan(dest ...any) error
}

func scanQuota(r quotaScanner) (Quota, error) {
	var q Quota
	err := r.Scan(&q.ID, &q.Scope, &q.ScopeID, &q.Period, &q.LimitTokens, &q.LimitCost,
		&q.UsedTokens, &q.UsedCost, &q.PeriodStart)
	return q, err
}
