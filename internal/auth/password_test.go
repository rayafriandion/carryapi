package auth

import (
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" || hash == "correct horse battery staple" {
		t.Fatal("hash empty or equals plaintext")
	}
	if !VerifyPassword("correct horse battery staple", hash) {
		t.Error("VerifyPassword should accept correct password")
	}
	if VerifyPassword("wrong", hash) {
		t.Error("VerifyPassword should reject wrong password")
	}
}

func TestHashIsSalted(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Error("two hashes of same password should differ (bcrypt salt)")
	}
}

func TestHashCost(t *testing.T) {
	h, _ := HashPassword("x")
	// bcrypt hash 前7字符 "$2a$12$" 表示 cost 12
	if len(h) < 7 || h[:7] != "$2a$12$" {
		t.Errorf("hash prefix = %q, want $2a$12$", h[:7])
	}
}
