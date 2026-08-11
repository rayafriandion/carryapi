package proxy

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"carryapi/internal/apikey"
	"carryapi/internal/crypto"
	"carryapi/internal/db"
	"carryapi/internal/user"
)

func mustCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	c, err := crypto.New(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatalf("cipher: %v", err)
	}
	return c
}

// newAuthFixture 建一个内存 db + 用户/密钥 store,并返回共享 cipher,供
// newModelFixture 复用(ProviderStore 加密/解密必须使用同一个 cipher)。
func newAuthFixture(t *testing.T) (*Proxy, *user.User, *crypto.Cipher) {
	t.Helper()
	d, _ := db.Open(":memory:")
	db.Migrate(d)
	t.Cleanup(func() { d.Close() })
	c := mustCipher(t)
	us := user.New(d, c)
	ks := apikey.New(d)
	p := NewProxy(Deps{DB: d, Keys: ks, Users: us})
	// 建用户 + key
	u, _ := us.Create("proxy@x.com", "hash", "user")
	return p, &u, c
}

func TestAuthenticateBearer(t *testing.T) {
	p, u, _ := newAuthFixture(t)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	got, key, err := p.authenticate(req)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if got.ID != u.ID || key.ID == 0 {
		t.Errorf("user=%d key=%d", got.ID, key.ID)
	}
}

func TestAuthenticateXAPIKey(t *testing.T) {
	p, u, _ := newAuthFixture(t)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	req := httptest.NewRequest("POST", "/v1/messages", nil)
	req.Header.Set("x-api-key", plaintext)
	_, _, err := p.authenticate(req)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
}

func TestAuthenticateMissing(t *testing.T) {
	p, _, _ := newAuthFixture(t)
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	_, _, err := p.authenticate(req)
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestAuthenticateDisabledUser(t *testing.T) {
	p, u, _ := newAuthFixture(t)
	plaintext, _, _ := p.deps.Keys.Create(u.ID, "test")
	p.deps.Users.UpdateStatus(u.ID, "disabled")
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	_, _, err := p.authenticate(req)
	if err == nil {
		t.Fatal("expected error for disabled user")
	}
}
