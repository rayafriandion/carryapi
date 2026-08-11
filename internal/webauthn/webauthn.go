package webauthn

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// 重新导出,便于外部包引用(避免多处 import 同一长路径)
type User = webauthn.User
type Credential = webauthn.Credential

// Service 封装 *webauthn.WebAuthn + 内存 session store。
// Begin* 产生的 SessionData 按 sessionKey 存入 map;Finish* 取出并删除(一次性)。
// gc goroutine 每分钟清理已过期 session(SessionData.Expires)。
type Service struct {
	w        *webauthn.WebAuthn
	mu       sync.Mutex
	sessions map[string]*webauthn.SessionData
}

// New 构造 Service。rpID 是 Relying Party 域(如 "localhost"),rpOrigin 是完整 origin(如 "http://localhost:8080")。
func New(rpID, rpOrigin string) (*Service, error) {
	w, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "carryAPI",
		RPID:          rpID,
		RPOrigins:     []string{rpOrigin},
	})
	if err != nil {
		return nil, fmt.Errorf("webauthn init: %w", err)
	}
	s := &Service{w: w, sessions: make(map[string]*webauthn.SessionData)}
	go s.gc()
	return s, nil
}

// LocalUser 把 carryAPI 的 user.User + 已存 credentials 适配成 webauthn.User。
type LocalUser struct {
	ID          int64
	Email       string
	Credentials []webauthn.Credential
}

func (u *LocalUser) WebAuthnID() []byte {
	// 用 user ID 的字节表示作 handle(稳定且唯一)
	return []byte(fmt.Sprintf("uid-%d", u.ID))
}
func (u *LocalUser) WebAuthnName() string                       { return u.Email }
func (u *LocalUser) WebAuthnDisplayName() string                { return u.Email }
func (u *LocalUser) WebAuthnCredentials() []webauthn.Credential { return u.Credentials }

func (s *Service) BeginRegistration(u webauthn.User) (*protocol.CredentialCreation, string, error) {
	creation, session, err := s.w.BeginRegistration(u)
	if err != nil {
		return nil, "", fmt.Errorf("begin registration: %w", err)
	}
	key := s.putSession(session)
	return creation, key, nil
}

func (s *Service) FinishRegistration(u webauthn.User, sessionKey string, r *http.Request) (*webauthn.Credential, error) {
	session, ok := s.takeSession(sessionKey)
	if !ok {
		return nil, fmt.Errorf("unknown or expired session")
	}
	if r == nil {
		return nil, fmt.Errorf("nil request")
	}
	cred, err := s.w.FinishRegistration(u, *session, r)
	if err != nil {
		return nil, fmt.Errorf("finish registration: %w", err)
	}
	return cred, nil
}

func (s *Service) BeginLogin(u webauthn.User) (*protocol.CredentialAssertion, string, error) {
	assertion, session, err := s.w.BeginLogin(u)
	if err != nil {
		return nil, "", fmt.Errorf("begin login: %w", err)
	}
	key := s.putSession(session)
	return assertion, key, nil
}

func (s *Service) FinishLogin(u webauthn.User, sessionKey string, r *http.Request) (*webauthn.Credential, error) {
	session, ok := s.takeSession(sessionKey)
	if !ok {
		return nil, fmt.Errorf("unknown or expired session")
	}
	if r == nil {
		return nil, fmt.Errorf("nil request")
	}
	cred, err := s.w.FinishLogin(u, *session, r)
	if err != nil {
		return nil, fmt.Errorf("finish login: %w", err)
	}
	return cred, nil
}

func (s *Service) putSession(sess *webauthn.SessionData) string {
	b := make([]byte, 16)
	rand.Read(b)
	key := hex.EncodeToString(b)
	s.mu.Lock()
	s.sessions[key] = sess
	s.mu.Unlock()
	return key
}

func (s *Service) getSession(key string) (*webauthn.SessionData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[key]
	return sess, ok
}

// takeSession 取出并删除(一次性)
func (s *Service) takeSession(key string) (*webauthn.SessionData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[key]
	if ok {
		delete(s.sessions, key)
	}
	return sess, ok
}

// gc 定期清理过期 session(SessionData.Expires)
func (s *Service) gc() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for k, sess := range s.sessions {
			if now.After(sess.Expires) {
				delete(s.sessions, k)
			}
		}
		s.mu.Unlock()
	}
}
