package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"carryapi/internal/api"
	"carryapi/internal/auth"
	"carryapi/internal/catalog"
	"carryapi/internal/config"
	"carryapi/internal/proxy"
	"carryapi/internal/settings"
	"carryapi/internal/stats"
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
	Setup    *api.SetupHandler
	UsersH   *api.UserHandler
	Keys     *api.KeyHandler
	Quotas   *api.QuotaHandler
	Settings *api.SettingsHandler
	OAuth    *api.OAuthHandler
	Passkey  *api.PasskeyHandler
	Catalog  *catalog.Handler
	Proxy    *proxy.Proxy
	Stats    *stats.Handler
}

type Server struct {
	cfg        config.Config
	deps       Deps
	httpServer *http.Server
	router     http.Handler
	actualAddr atomic.Value
}

func New(cfg config.Config, deps Deps) *Server {
	s := &Server{cfg: cfg, deps: deps}
	s.actualAddr.Store("")
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
	listeners, err := s.resolveListeners()
	if err != nil {
		return err
	}
	actual := make([]string, 0, len(listeners))
	for _, ln := range listeners {
		actual = append(actual, ln.Addr().String())
	}
	s.actualAddr.Store(strings.Join(actual, ", "))
	lc := s.listenerConfig()
	fmt.Printf("carryAPI listening on %s (broadcast=%s)\n", s.actualAddr.Load(), broadcastLabel(lc))
	return s.serveListeners(listeners)
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func broadcastLabel(lc listenerConfig) string {
	if lc.broadcastOn() {
		return fmt.Sprintf("ON (%s, other devices can access)", lc.mode)
	}
	return fmt.Sprintf("OFF (%s, localhost only)", lc.mode)
}
