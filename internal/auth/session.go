package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const SessionCookieName = "carryapi_session"

type Session struct {
	ID        int64
	UserID    int64
	Token     string // 原始 token,仅创建时返回
	ExpiresAt time.Time
	CreatedAt time.Time
	IP        string
	UserAgent string
}

type SessionStore struct {
	db *sql.DB
}

func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{db: db}
}

func (s *SessionStore) Create(userID int64, ttl time.Duration, ip, userAgent string) (Session, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Session{}, fmt.Errorf("session token rand: %w", err)
	}
	token := hex.EncodeToString(raw)
	hash := hashToken(token)
	expires := time.Now().Add(ttl)
	res, err := s.db.Exec(
		`INSERT INTO sessions(user_id, token_hash, expires_at, ip, user_agent) VALUES(?, ?, ?, ?, ?)`,
		userID, hash, expires, ip, userAgent)
	if err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	id, _ := res.LastInsertId()
	return Session{ID: id, UserID: userID, Token: token, ExpiresAt: expires, CreatedAt: time.Now(), IP: ip, UserAgent: userAgent}, nil
}

func (s *SessionStore) Lookup(token string) (Session, error) {
	var sess Session
	var hash string
	err := s.db.QueryRow(
		`SELECT id, user_id, token_hash, expires_at, created_at, ip, user_agent FROM sessions WHERE token_hash=?`,
		hashToken(token)).Scan(&sess.ID, &sess.UserID, &hash, &sess.ExpiresAt, &sess.CreatedAt, &sess.IP, &sess.UserAgent)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, errors.New("session not found")
	}
	if err != nil {
		return Session{}, err
	}
	if time.Now().After(sess.ExpiresAt) {
		return Session{}, errors.New("session expired")
	}
	return sess, nil
}

func (s *SessionStore) Revoke(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash=?`, hashToken(token))
	return err
}

func (s *SessionStore) RevokeAllForUser(userID int64) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE user_id=?`, userID)
	return err
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
