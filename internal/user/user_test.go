package user

import (
	"bytes"
	"testing"

	"carryapi/internal/crypto"
	"carryapi/internal/db"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	c, _ := crypto.New(bytes.Repeat([]byte{1}, 32))
	return New(d, c)
}

func TestCreateAndGet(t *testing.T) {
	s := newStore(t)
	u, err := s.Create("alice@example.com", "$2a$12$hash", "user")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == 0 || u.Email != "alice@example.com" || u.Role != "user" || u.Status != "active" {
		t.Errorf("unexpected user: %+v", u)
	}
	got, err := s.GetByID(u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("GetByID email = %q", got.Email)
	}
	byEmail, err := s.GetByEmail("alice@example.com")
	if err != nil || byEmail.ID != u.ID {
		t.Errorf("GetByEmail: got %+v err %v", byEmail, err)
	}
}

func TestCreateDuplicateEmail(t *testing.T) {
	s := newStore(t)
	s.Create("dup@example.com", "h", "user")
	_, err := s.Create("dup@example.com", "h", "user")
	if err == nil {
		t.Error("expected error for duplicate email")
	}
}

func TestListAndUpdate(t *testing.T) {
	s := newStore(t)
	s.Create("a@x.com", "h", "user")
	s.Create("b@x.com", "h", "admin")
	users, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("List len = %d, want 2", len(users))
	}
	// disable + role change
	u, _ := s.GetByEmail("a@x.com")
	s.UpdateStatus(u.ID, "disabled")
	s.UpdateRole(u.ID, "admin")
	got, _ := s.GetByID(u.ID)
	if got.Status != "disabled" || got.Role != "admin" {
		t.Errorf("after update: status=%s role=%s", got.Status, got.Role)
	}
}

func TestDelete(t *testing.T) {
	s := newStore(t)
	u, _ := s.Create("del@x.com", "h", "user")
	if err := s.Delete(u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.GetByID(u.ID); err == nil {
		t.Error("expected error getting deleted user")
	}
}

func TestDeleteCascade(t *testing.T) {
	s := newStore(t)
	u, _ := s.Create("cascade@x.com", "h", "user")
	// 子行:auth_method + api_key + session
	if err := s.AddAuthMethod(u.ID, "totp", "", []byte("secret")); err != nil {
		t.Fatalf("AddAuthMethod: %v", err)
	}
	if _, err := s.db.Exec(
		`INSERT INTO api_keys(user_id, key_hash, key_prefix, label, status) VALUES(?, 'hhh', 'carry-000001', 'k', 'active')`,
		u.ID); err != nil {
		t.Fatalf("insert api_key: %v", err)
	}
	if _, err := s.db.Exec(
		`INSERT INTO sessions(user_id, token_hash, expires_at) VALUES(?, 'tokhash', datetime('now', '+1 hour'))`,
		u.ID); err != nil {
		t.Fatalf("insert session: %v", err)
	}

	if err := s.DeleteCascade(u.ID); err != nil {
		t.Fatalf("DeleteCascade: %v", err)
	}
	if _, err := s.GetByID(u.ID); err == nil {
		t.Error("expected error getting deleted user")
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM auth_methods WHERE user_id=?`, u.ID).Scan(&n)
	if n != 0 {
		t.Errorf("auth_methods left: %d", n)
	}
	s.db.QueryRow(`SELECT COUNT(*) FROM api_keys WHERE user_id=?`, u.ID).Scan(&n)
	if n != 0 {
		t.Errorf("api_keys left: %d", n)
	}
	s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE user_id=?`, u.ID).Scan(&n)
	if n != 0 {
		t.Errorf("sessions left: %d", n)
	}
}

func TestHasAdminAndFirstAdminID(t *testing.T) {
	s := newStore(t)
	ok, err := s.HasAdmin()
	if err != nil {
		t.Fatalf("HasAdmin: %v", err)
	}
	if ok {
		t.Fatal("expected no admin initially")
	}
	_, found, err := s.FirstAdminID()
	if err != nil {
		t.Fatalf("FirstAdminID: %v", err)
	}
	if found {
		t.Fatal("expected FirstAdminID not found initially")
	}

	s.Create("a@x.com", "hash1", "user")
	s.Create("admin1@x.com", "hash2", "admin")
	s.Create("admin2@x.com", "hash3", "admin")

	ok, err = s.HasAdmin()
	if err != nil || !ok {
		t.Fatalf("HasAdmin after create: ok=%v err=%v", ok, err)
	}
	id, found, err := s.FirstAdminID()
	if err != nil || !found {
		t.Fatalf("FirstAdminID: found=%v err=%v", found, err)
	}
	// admin1 先创建,id 更小,应为首个 admin
	u, _ := s.GetByID(id)
	if u.Email != "admin1@x.com" {
		t.Fatalf("expected first admin admin1@x.com, got %s", u.Email)
	}
}
