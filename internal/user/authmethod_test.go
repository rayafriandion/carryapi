package user

import (
	"bytes"
	"testing"
)

func TestAddAndGetAuthMethods(t *testing.T) {
	s := newStore(t)
	u, _ := s.Create("auth@x.com", "h", "user")
	secret := []byte("totp-secret-bytes")
	if err := s.AddAuthMethod(u.ID, "totp", "", secret); err != nil {
		t.Fatalf("AddAuthMethod: %v", err)
	}
	methods, err := s.GetAuthMethods(u.ID)
	if err != nil {
		t.Fatalf("GetAuthMethods: %v", err)
	}
	if len(methods) != 1 || methods[0].Provider != "totp" {
		t.Fatalf("got %+v", methods)
	}
	// secret 应解密回原文
	if !bytes.Equal(methods[0].Secret, secret) {
		t.Errorf("secret round-trip mismatch: got %q want %q", methods[0].Secret, secret)
	}
}

func TestGetAuthMethodByProviderUID(t *testing.T) {
	s := newStore(t)
	u, _ := s.Create("oauth@x.com", "h", "user")
	s.AddAuthMethod(u.ID, "discord", "discord-user-123", nil)
	m, err := s.GetAuthMethod("discord", "discord-user-123")
	if err != nil {
		t.Fatalf("GetAuthMethod: %v", err)
	}
	if m.UserID != u.ID {
		t.Errorf("UserID = %d, want %d", m.UserID, u.ID)
	}
}

func TestDeleteAuthMethod(t *testing.T) {
	s := newStore(t)
	u, _ := s.Create("del2@x.com", "h", "user")
	s.AddAuthMethod(u.ID, "totp", "", []byte("s"))
	methods, _ := s.GetAuthMethods(u.ID)
	if len(methods) != 1 {
		t.Fatal("expected 1 method")
	}
	// 带 userID 删(防越权)
	if err := s.DeleteAuthMethod(methods[0].ID, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	methods, _ = s.GetAuthMethods(u.ID)
	if len(methods) != 0 {
		t.Error("expected 0 methods after delete")
	}
}

func TestDeleteAuthMethodWrongUser(t *testing.T) {
	s := newStore(t)
	u1, _ := s.Create("u1@x.com", "h", "user")
	u2, _ := s.Create("u2@x.com", "h", "user")
	s.AddAuthMethod(u1.ID, "totp", "", []byte("s"))
	methods, _ := s.GetAuthMethods(u1.ID)
	// u2 试图删 u1 的 method -> 应不影响
	err := s.DeleteAuthMethod(methods[0].ID, u2.ID)
	if err == nil {
		t.Error("expected error deleting other user's auth method")
	}
	methods, _ = s.GetAuthMethods(u1.ID)
	if len(methods) != 1 {
		t.Error("method should still exist")
	}
}
