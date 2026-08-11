package catalog

import (
	"bytes"
	"testing"

	"carryapi/internal/crypto"
	"carryapi/internal/db"
)

func newProviderStore(t *testing.T) *ProviderStore {
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
	return NewProviderStore(d, c)
}

func TestCreateAndGet(t *testing.T) {
	s := newProviderStore(t)
	p, err := s.Create("OpenAI", "https://api.openai.com/v1", "sk-secret", "openai_chat")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == 0 || p.Status != "active" || p.Protocol != "openai_chat" {
		t.Errorf("unexpected provider: %+v", p)
	}
	// Get 返回解密后的 key
	got, err := s.Get(p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.APIKey != "sk-secret" {
		t.Errorf("api key round-trip = %q, want sk-secret", got.APIKey)
	}
}

func TestCreateInvalidProtocol(t *testing.T) {
	s := newProviderStore(t)
	if _, err := s.Create("X", "http://x", "k", "bogus"); err == nil {
		t.Error("expected error for invalid protocol")
	}
}

func TestUpdatePreservesKeyWhenEmpty(t *testing.T) {
	s := newProviderStore(t)
	p, _ := s.Create("OpenAI", "https://api.openai.com/v1", "sk-secret", "openai_chat")
	// 只改 name,apiKey 传空 -> 保留原 key
	if err := s.Update(p.ID, "OpenAI2", "https://api.openai.com/v2", "", "openai_chat", "active"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := s.Get(p.ID)
	if got.Name != "OpenAI2" || got.APIKey != "sk-secret" {
		t.Errorf("after update: name=%q key=%q", got.Name, got.APIKey)
	}
}

func TestDelete(t *testing.T) {
	s := newProviderStore(t)
	p, _ := s.Create("OpenAI", "https://api.openai.com/v1", "k", "openai_chat")
	if err := s.Delete(p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(p.ID); err == nil {
		t.Error("expected error after delete")
	}
}
