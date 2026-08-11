package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"carryapi/internal/config"
	"carryapi/internal/db"
	"carryapi/internal/settings"
)

func newServer(t *testing.T) *Server {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	cfg := config.Config{Port: 0, MasterKey: make([]byte, 32)}
	// Subproject-1 tests only exercise health/broadcast, so all handlers
	// are left nil; buildRouter's nil-guard skips mounting their routes.
	return New(cfg, Deps{DB: d, Store: settings.New(d)})
}

func TestHealthEndpoint(t *testing.T) {
	s := newServer(t)
	req := httptest.NewRequest("GET", "/api/health", nil)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	if rec.Body.String() != `{"status":"ok"}`+"\n" {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestBroadcastOffListensLoopback(t *testing.T) {
	s := newServer(t)
	s.deps.Store.Set("listen_host", "127.0.0.1")
	addr, err := s.listenAddr()
	if err != nil {
		t.Fatalf("listenAddr: %v", err)
	}
	if !strings.HasPrefix(addr, "127.0.0.1:") {
		t.Errorf("addr = %q, want loopback", addr)
	}
}

func TestBroadcastOnListensAllInterfaces(t *testing.T) {
	s := newServer(t)
	s.deps.Store.Set("listen_host", "0.0.0.0")
	addr, _ := s.listenAddr()
	if !strings.HasPrefix(addr, "0.0.0.0:") {
		t.Errorf("addr = %q, want 0.0.0.0", addr)
	}
}

func TestShutdown(t *testing.T) {
	s := newServer(t)
	if err := s.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown: %v", err)
	}
}
