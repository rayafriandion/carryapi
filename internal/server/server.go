package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"carryapi/internal/api"
	"carryapi/internal/auth"
	"carryapi/internal/config"
	"carryapi/internal/settings"
	"carryapi/internal/user"
)

// Deps aggregates all dependencies the server needs to build its router.
// Handlers may be nil: buildRouter skips mounting routes whose handler is nil,
// so subproject-1 health/broadcast tests (which pass an empty Deps) keep working.
type Deps struct {
	DB       *sql.DB
	Store    *settings.Store // settings store (broadcast toggle etc.)
	Users    *user.Store
	Sessions *auth.SessionStore
	Auth     *api.AuthHandler
	UsersH   *api.UserHandler
	Keys     *api.KeyHandler
	Quotas   *api.QuotaHandler
	Settings *api.SettingsHandler
	OAuth    *api.OAuthHandler
	// Passkey handler is added in Task 13 when that package/type is introduced.
}

type Server struct {
	cfg        config.Config
	deps       Deps
	httpServer *http.Server
	router     http.Handler
	actualAddr string
}

func New(cfg config.Config, deps Deps) *Server {
	s := &Server{cfg: cfg, deps: deps}
	s.buildRouter()
	s.httpServer = &http.Server{
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		// WriteTimeout intentionally unset: this server proxies streaming
		// (SSE) LLM responses that require unbounded write time. Setting
		// WriteTimeout would break long streaming completions. Do not add
		// one without a per-handler write deadline strategy.
	}
	return s
}

func (s *Server) ListenAndServe() error {
	ln, err := s.resolveListener()
	if err != nil {
		return err
	}
	fmt.Printf("carryAPI listening on %s (broadcast=%s)\n", s.actualAddr, broadcastLabel(s.deps.Store))
	return s.httpServer.Serve(ln)
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func broadcastLabel(store *settings.Store) string {
	if listenHost(store) == "0.0.0.0" {
		return "ON (0.0.0.0, other devices can access)"
	}
	return "OFF (127.0.0.1, localhost only)"
}
