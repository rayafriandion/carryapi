package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const keyPrefix = "carry-"

type APIKey struct {
	ID         int64
	UserID     int64
	KeyPrefix  string
	Label      string
	Status     string
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	CreatedAt  time.Time
}

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(userID int64, label string) (string, APIKey, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", APIKey{}, fmt.Errorf("apikey rand: %w", err)
	}
	plaintext := keyPrefix + hex.EncodeToString(raw)
	hash := hashKey(plaintext)
	prefix := plaintext[:12]
	res, err := s.db.Exec(
		`INSERT INTO api_keys(user_id, key_hash, key_prefix, label, status) VALUES(?, ?, ?, ?, 'active')`,
		userID, hash, prefix, label)
	if err != nil {
		return "", APIKey{}, fmt.Errorf("create apikey: %w", err)
	}
	id, _ := res.LastInsertId()
	ak, err := s.Get(id, userID)
	if err != nil {
		return "", APIKey{}, err
	}
	return plaintext, ak, nil
}

func (s *Store) List(userID int64) ([]APIKey, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, key_prefix, label, status, expires_at, last_used_at, created_at
		 FROM api_keys WHERE user_id=? ORDER BY id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list apikeys: %w", err)
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		ak, err := scanKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ak)
	}
	return out, rows.Err()
}

func (s *Store) Get(id, userID int64) (APIKey, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, key_prefix, label, status, expires_at, last_used_at, created_at
		 FROM api_keys WHERE id=? AND user_id=?`, id, userID)
	ak, err := scanKey(row)
	if errors.Is(err, sql.ErrNoRows) {
		return APIKey{}, errors.New("api key not found")
	}
	return ak, err
}

func (s *Store) Update(id, userID int64, label, status string) error {
	_, err := s.db.Exec(`UPDATE api_keys SET label=?, status=? WHERE id=? AND user_id=?`, label, status, id, userID)
	return err
}

func (s *Store) Delete(id, userID int64) error {
	res, err := s.db.Exec(`DELETE FROM api_keys WHERE id=? AND user_id=?`, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("api key not found or not owned")
	}
	return nil
}

func (s *Store) Authenticate(plaintext string) (int64, int64, error) {
	hash := hashKey(plaintext)
	var ak APIKey
	row := s.db.QueryRow(
		`SELECT id, user_id, key_prefix, label, status, expires_at, last_used_at, created_at
		 FROM api_keys WHERE key_hash=?`, hash)
	if err := row.Scan(&ak.ID, &ak.UserID, &ak.KeyPrefix, &ak.Label, &ak.Status, &ak.ExpiresAt, &ak.LastUsedAt, &ak.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, errors.New("invalid api key")
		}
		return 0, 0, err
	}
	if ak.Status != "active" {
		return 0, 0, errors.New("api key disabled")
	}
	if ak.ExpiresAt != nil && time.Now().After(*ak.ExpiresAt) {
		return 0, 0, errors.New("api key expired")
	}
	s.TouchLastUsed(ak.ID)
	return ak.UserID, ak.ID, nil
}

func (s *Store) TouchLastUsed(id int64) error {
	_, err := s.db.Exec(`UPDATE api_keys SET last_used_at=? WHERE id=?`, time.Now(), id)
	return err
}

func hashKey(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(h[:])
}

type keyScanner interface {
	Scan(dest ...any) error
}

func scanKey(r keyScanner) (APIKey, error) {
	var ak APIKey
	err := r.Scan(&ak.ID, &ak.UserID, &ak.KeyPrefix, &ak.Label, &ak.Status, &ak.ExpiresAt, &ak.LastUsedAt, &ak.CreatedAt)
	return ak, err
}
