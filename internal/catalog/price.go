package catalog

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrNoPrice = errors.New("no price configured")

type Price struct {
	ID              int64
	ModelID         int64
	InputPrice      float64 // 每百万 token
	OutputPrice     float64
	CacheReadPrice  *float64
	CacheWritePrice *float64
	Currency        string
	EffectiveFrom   time.Time
}

type PriceStore struct {
	db *sql.DB
}

func NewPriceStore(db *sql.DB) *PriceStore {
	return &PriceStore{db: db}
}

func (s *PriceStore) Set(modelID int64, inputPrice, outputPrice float64, cacheRead, cacheWrite *float64) (Price, error) {
	res, err := s.db.Exec(
		`INSERT INTO model_prices(model_id, input_price, output_price, cache_read_price, cache_write_price) VALUES(?, ?, ?, ?, ?)`,
		modelID, inputPrice, outputPrice, cacheRead, cacheWrite)
	if err != nil {
		return Price{}, fmt.Errorf("set price: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.Get(id)
}

func (s *PriceStore) Get(id int64) (Price, error) {
	return s.scan(s.db.QueryRow(
		`SELECT id, model_id, input_price, output_price, cache_read_price, cache_write_price, currency, effective_from
		 FROM model_prices WHERE id=?`, id))
}

func (s *PriceStore) GetCurrent(modelID int64) (Price, error) {
	row := s.db.QueryRow(
		`SELECT id, model_id, input_price, output_price, cache_read_price, cache_write_price, currency, effective_from
		 FROM model_prices WHERE model_id=? ORDER BY effective_from DESC, id DESC LIMIT 1`, modelID)
	p, err := s.scan(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Price{}, ErrNoPrice
	}
	return p, err
}

func (s *PriceStore) List(modelID int64) ([]Price, error) {
	rows, err := s.db.Query(
		`SELECT id, model_id, input_price, output_price, cache_read_price, cache_write_price, currency, effective_from
		 FROM model_prices WHERE model_id=? ORDER BY effective_from DESC`, modelID)
	if err != nil {
		return nil, fmt.Errorf("list prices: %w", err)
	}
	defer rows.Close()
	var out []Price
	for rows.Next() {
		p, err := s.scanRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *PriceStore) scan(row *sql.Row) (Price, error) {
	var p Price
	err := row.Scan(&p.ID, &p.ModelID, &p.InputPrice, &p.OutputPrice, &p.CacheReadPrice,
		&p.CacheWritePrice, &p.Currency, &p.EffectiveFrom)
	return p, err
}

func (s *PriceStore) scanRows(r interface{ Scan(...any) error }) (Price, error) {
	var p Price
	err := r.Scan(&p.ID, &p.ModelID, &p.InputPrice, &p.OutputPrice, &p.CacheReadPrice,
		&p.CacheWritePrice, &p.Currency, &p.EffectiveFrom)
	return p, err
}
