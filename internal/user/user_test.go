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
