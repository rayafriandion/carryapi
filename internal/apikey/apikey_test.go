package apikey

import (
	"strings"
	"testing"
	"time"

	"carryapi/internal/db"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	// Seed users (IDs 1, 2) so FK on api_keys.user_id REFERENCES users(id) is
	// satisfied. The brief's test bodies call s.Create(1, ...) / s.Create(2, ...)
	// without pre-existing users. Same pattern as Task 5 session_test fix.
	d.Exec("INSERT INTO users(id, email, role, status) VALUES (1, 'u1@x.com', 'user', 'active'), (2, 'u2@x.com', 'user', 'active')")
	t.Cleanup(func() { d.Close() })
	return New(d)
}

func TestCreate(t *testing.T) {
	s := newStore(t)
	plaintext, ak, err := s.Create(1, "my-key")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !strings.HasPrefix(plaintext, "carry-") || len(plaintext) != 38 {
		t.Errorf("plaintext = %q (len %d), want carry-<32hex>", plaintext, len(plaintext))
	}
	if ak.KeyPrefix != plaintext[:12] {
		t.Errorf("prefix = %q, want %q", ak.KeyPrefix, plaintext[:12])
	}
	if ak.Label != "my-key" || ak.Status != "active" {
		t.Errorf("ak = %+v", ak)
	}
}

func TestAuthenticate(t *testing.T) {
	s := newStore(t)
	plaintext, _, _ := s.Create(1, "k")
	uid, kid, err := s.Authenticate(plaintext)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if uid != 1 || kid == 0 {
		t.Errorf("uid=%d kid=%d", uid, kid)
	}
}

func TestAuthenticateWrongKey(t *testing.T) {
	s := newStore(t)
	s.Create(1, "k")
	_, _, err := s.Authenticate("carry-deadbeefdeadbeefdeadbeefdeadbeef")
	if err == nil {
		t.Error("expected error for wrong key")
	}
}

func TestAuthenticateDisabled(t *testing.T) {
	s := newStore(t)
	plaintext, ak, _ := s.Create(1, "k")
	s.Update(ak.ID, 1, "k", "disabled")
	_, _, err := s.Authenticate(plaintext)
	if err == nil {
		t.Error("expected error for disabled key")
	}
}

func TestAuthenticateExpired(t *testing.T) {
	s := newStore(t)
	plaintext, ak, _ := s.Create(1, "k")
	// 直接更新过期时间为过去
	s.db.Exec("UPDATE api_keys SET expires_at=? WHERE id=?", time.Now().Add(-time.Hour), ak.ID)
	_, _, err := s.Authenticate(plaintext)
	if err == nil {
		t.Error("expected error for expired key")
	}
}

func TestListAndDelete(t *testing.T) {
	s := newStore(t)
	s.Create(1, "k1")
	s.Create(1, "k2")
	s.Create(2, "k3")
	keys, _ := s.List(1)
	if len(keys) != 2 {
		t.Errorf("List(1) = %d, want 2", len(keys))
	}
	// 删除带 userID 防越权
	s.Delete(keys[0].ID, 1)
	keys, _ = s.List(1)
	if len(keys) != 1 {
		t.Errorf("after delete = %d, want 1", len(keys))
	}
}

func TestDeleteWrongUser(t *testing.T) {
	s := newStore(t)
	_, ak, _ := s.Create(1, "k")
	err := s.Delete(ak.ID, 2) // user 2 删 user 1 的 key
	if err == nil {
		t.Error("expected error deleting other user's key")
	}
}
