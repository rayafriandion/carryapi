package user

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

type AuthMethod struct {
	ID          int64
	UserID      int64
	Provider    string
	ProviderUID string
	Secret      []byte // 解密后
	CreatedAt   time.Time
}

func (s *Store) AddAuthMethod(userID int64, provider, providerUID string, secret []byte) error {
	var enc []byte
	if len(secret) > 0 {
		encBytes, err := s.cipher.Encrypt(secret)
		if err != nil {
			return fmt.Errorf("encrypt auth method secret: %w", err)
		}
		enc = encBytes
	}
	_, err := s.db.Exec(
		`INSERT INTO auth_methods(user_id, provider, provider_uid, secret) VALUES(?, ?, ?, ?)`,
		userID, provider, providerUID, enc)
	if err != nil {
		return fmt.Errorf("add auth method: %w", err)
	}
	return nil
}

func (s *Store) GetAuthMethods(userID int64) ([]AuthMethod, error) {
	rows, err := s.db.Query(
		`SELECT id, user_id, provider, provider_uid, secret, created_at FROM auth_methods WHERE user_id=?`, userID)
	if err != nil {
		return nil, fmt.Errorf("get auth methods: %w", err)
	}
	defer rows.Close()
	var out []AuthMethod
	for rows.Next() {
		m, err := s.scanAuthMethod(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) GetAuthMethod(provider, providerUID string) (AuthMethod, error) {
	row := s.db.QueryRow(
		`SELECT id, user_id, provider, provider_uid, secret, created_at FROM auth_methods WHERE provider=? AND provider_uid=?`,
		provider, providerUID)
	return s.scanAuthMethod(row)
}

func (s *Store) DeleteAuthMethod(id, userID int64) error {
	res, err := s.db.Exec(`DELETE FROM auth_methods WHERE id=? AND user_id=?`, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("auth method not found or not owned by user")
	}
	return nil
}

// UpdateAuthMethodSecret updates the (encrypted) secret of an auth_method
// owned by the given user. Used to persist the updated WebAuthn credential
// (including the sign counter) after a successful passkey login.
func (s *Store) UpdateAuthMethodSecret(id, userID int64, secret []byte) error {
	enc, err := s.cipher.Encrypt(secret)
	if err != nil {
		return fmt.Errorf("encrypt auth method secret: %w", err)
	}
	res, err := s.db.Exec(`UPDATE auth_methods SET secret=? WHERE id=? AND user_id=?`, enc, id, userID)
	if err != nil {
		return fmt.Errorf("update auth method secret: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("auth method not found or not owned by user")
	}
	return nil
}

func (s *Store) scanAuthMethod(r rowScanner) (AuthMethod, error) {
	var m AuthMethod
	var enc []byte
	if err := r.Scan(&m.ID, &m.UserID, &m.Provider, &m.ProviderUID, &enc, &m.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AuthMethod{}, fmt.Errorf("auth method not found: %w", err)
		}
		return AuthMethod{}, err
	}
	if len(enc) > 0 {
		dec, err := s.cipher.Decrypt(enc)
		if err != nil {
			return AuthMethod{}, fmt.Errorf("decrypt auth method secret: %w", err)
		}
		m.Secret = dec
	}
	return m, nil
}
