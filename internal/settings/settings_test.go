package settings

import (
	"testing"

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
	return New(d)
}

func TestSetAndGet(t *testing.T) {
	s := newStore(t)
	if err := s.Set("listen_host", "127.0.0.1"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, ok, err := s.Get("listen_host")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got != "127.0.0.1" {
		t.Errorf("got %q, want 127.0.0.1", got)
	}
}

func TestGetMissing(t *testing.T) {
	s := newStore(t)
	_, ok, err := s.Get("nope")
	if err != nil {
		t.Fatalf("Get err: %v", err)
	}
	if ok {
		t.Error("expected ok=false for missing key")
	}
}

func TestSetOverwrites(t *testing.T) {
	s := newStore(t)
	s.Set("k", "a")
	s.Set("k", "b")
	got, ok, _ := s.Get("k")
	if !ok || got != "b" {
		t.Errorf("got %q ok=%v, want b", got, ok)
	}
}
