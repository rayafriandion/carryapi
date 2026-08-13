package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"carryapi/internal/auth"
	"carryapi/internal/catalog"
	"carryapi/internal/config"
	"carryapi/internal/crypto"
	"carryapi/internal/db"
	"carryapi/internal/settings"
	"carryapi/internal/user"
)

// newCatalogServer builds a *Server whose router mounts the catalog admin
// routes (requires Sessions, Users and Catalog in Deps).
func newCatalogServer(t *testing.T) *Server {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	cipher, err := crypto.New(make([]byte, 32))
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}

	cfg := config.Config{Port: 0, MasterKey: make([]byte, 32)}
	return New(cfg, Deps{
		DB:       d,
		Store:    settings.New(d),
		Users:    user.New(d, cipher),
		Sessions: auth.NewSessionStore(d),
		Catalog: catalog.NewHandler(
			d,
			catalog.NewProviderStore(d, cipher),
			catalog.NewModelStore(d),
			catalog.NewPriceStore(d),
			nil,
		),
	})
}

// TestCatalogRoutesRegistered verifies the three catalog admin routes are
// actually mounted on the router. Each is a POST behind CSRFMiddleware: an
// unauthenticated POST with no CSRF cookie returns 403 when the route exists.
// If the route were NOT mounted, chi would fall through to the SPA catch-all
// (web.Handler), which returns 200 (index.html fallback). So a 403 proves the
// route is registered; a 200 proves it is missing.
func TestCatalogRoutesRegistered(t *testing.T) {
	paths := []string{
		"/api/providers/1/test",
		"/api/providers/1/models/fetch",
		"/api/models/import",
	}

	s := newCatalogServer(t)
	for _, p := range paths {
		req := httptest.NewRequest(http.MethodPost, p, nil)
		rec := httptest.NewRecorder()
		s.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("POST %s: code = %d, want 403 (route not mounted; SPA fallback returns 200)", p, rec.Code)
		}
	}
}
