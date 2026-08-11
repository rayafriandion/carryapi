package webauthn

import (
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
)

// stubUser 实现 webauthn.User 接口
type stubUser struct {
	id   []byte
	name string
	cred []webauthn.Credential
}

func (s *stubUser) WebAuthnID() []byte                         { return s.id }
func (s *stubUser) WebAuthnName() string                       { return s.name }
func (s *stubUser) WebAuthnDisplayName() string                { return s.name }
func (s *stubUser) WebAuthnCredentials() []webauthn.Credential { return s.cred }

func TestNewService(t *testing.T) {
	s, err := New("localhost", "http://localhost:8067")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s == nil {
		t.Fatal("nil service")
	}
}

func TestBeginRegistrationReturnsChallenge(t *testing.T) {
	s, _ := New("localhost", "http://localhost:8067")
	u := &stubUser{id: []byte("user-1"), name: "alice@example.com"}
	creation, sessionKey, err := s.BeginRegistration(u)
	if err != nil {
		t.Fatalf("BeginRegistration: %v", err)
	}
	if sessionKey == "" {
		t.Error("expected non-empty sessionKey")
	}
	// protocol.URLEncodedBase64 is a []byte; compare via len to check non-empty.
	if creation == nil || len(creation.Response.Challenge) == 0 {
		t.Error("expected non-empty challenge in creation")
	}
	// session 应已存入内存
	if _, ok := s.getSession(sessionKey); !ok {
		t.Error("session not stored")
	}
}

func TestFinishRegistrationUnknownSession(t *testing.T) {
	s, _ := New("localhost", "http://localhost:8067")
	u := &stubUser{id: []byte("user-1"), name: "alice@example.com"}
	// 没有对应 session -> 应报错
	_, err := s.FinishRegistration(u, "nonexistent-key", nil)
	if err == nil {
		t.Error("expected error for unknown session key")
	}
}

func TestBeginLoginReturnsChallenge(t *testing.T) {
	s, _ := New("localhost", "http://localhost:8067")
	// go-webauthn's BeginLogin rejects users with no credentials ("Found no
	// credentials for user"), so we attach a minimal placeholder credential here.
	// FinishLogin is exercised end-to-end only with real browser credentials.
	u := &stubUser{
		id:   []byte("user-1"),
		name: "alice@example.com",
		cred: []webauthn.Credential{{ID: []byte("cred-1")}},
	}
	assertion, sessionKey, err := s.BeginLogin(u)
	if err != nil {
		t.Fatalf("BeginLogin: %v", err)
	}
	if sessionKey == "" || assertion == nil {
		t.Error("expected non-empty assertion + sessionKey")
	}
}
