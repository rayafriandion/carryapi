package auth

import (
	"testing"
	"time"

	"carryapi/internal/db"
)

func newSessionStore(t *testing.T) *SessionStore {
	t.Helper()
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	// sessions.user_id REFERENCES users(id); seed users 1 and 2 so the
	// hardcoded user IDs in the tests below satisfy the FK constraint.
	d.Exec(`INSERT INTO users(id, email, role, status) VALUES (1, 'u1@x.com', 'user', 'active'), (2, 'u2@x.com', 'user', 'active')`)
	t.Cleanup(func() { d.Close() })
	return NewSessionStore(d)
}

func TestCreateAndLookup(t *testing.T) {
	s := newSessionStore(t)
	sess, err := s.Create(1, time.Hour, "1.2.3.4", "test-agent")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sess.Token == "" || len(sess.Token) != 64 {
		t.Fatalf("token = %q (len %d), want 64 hex chars", sess.Token, len(sess.Token))
	}
	got, err := s.Lookup(sess.Token)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got.UserID != 1 || got.IP != "1.2.3.4" {
		t.Errorf("got %+v", got)
	}
}

func TestLookupExpired(t *testing.T) {
	s := newSessionStore(t)
	sess, _ := s.Create(1, -time.Hour, "", "") // 已过期
	if _, err := s.Lookup(sess.Token); err == nil {
		t.Error("expected error for expired session")
	}
}

func TestLookupRevoked(t *testing.T) {
	s := newSessionStore(t)
	sess, _ := s.Create(1, time.Hour, "", "")
	s.Revoke(sess.Token)
	if _, err := s.Lookup(sess.Token); err == nil {
		t.Error("expected error after revoke")
	}
}

func TestRevokeAllForUser(t *testing.T) {
	s := newSessionStore(t)
	a1, _ := s.Create(1, time.Hour, "", "")
	a2, _ := s.Create(1, time.Hour, "", "")
	s.Create(2, time.Hour, "", "")
	s.RevokeAllForUser(1)
	if _, err := s.Lookup(a1.Token); err == nil {
		t.Error("a1 should be revoked")
	}
	if _, err := s.Lookup(a2.Token); err == nil {
		t.Error("a2 should be revoked")
	}
}
