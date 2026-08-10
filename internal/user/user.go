package user

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"carryapi/internal/crypto"
)

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Role         string
	Status       string
	CreatedAt    time.Time
}

type Store struct {
	db     *sql.DB
	cipher *crypto.Cipher
}

func New(db *sql.DB, cipher *crypto.Cipher) *Store {
	return &Store{db: db, cipher: cipher}
}

func (s *Store) Create(email, passwordHash, role string) (User, error) {
	res, err := s.db.Exec(
		`INSERT INTO users(email, password_hash, role, status) VALUES(?, ?, ?, 'active')`,
		email, passwordHash, role)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetByID(id)
}

func (s *Store) GetByID(id int64) (User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, email, password_hash, role, status, created_at FROM users WHERE id=?`, id))
}

func (s *Store) GetByEmail(email string) (User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, email, password_hash, role, status, created_at FROM users WHERE email=?`, email))
}

func (s *Store) List() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, email, password_hash, role, status, created_at FROM users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) UpdateStatus(id int64, status string) error {
	_, err := s.db.Exec(`UPDATE users SET status=? WHERE id=?`, status, id)
	return err
}

func (s *Store) UpdateRole(id int64, role string) error {
	_, err := s.db.Exec(`UPDATE users SET role=? WHERE id=?`, role, id)
	return err
}

func (s *Store) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id=?`, id)
	return err
}

func (s *Store) scanUser(row *sql.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, fmt.Errorf("user not found: %w", err)
	}
	return u, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUserRow(r rowScanner) (User, error) {
	var u User
	err := r.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &u.CreatedAt)
	return u, err
}
