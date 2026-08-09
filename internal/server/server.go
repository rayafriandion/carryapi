package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"carryapi/internal/config"
	"carryapi/internal/settings"
)

type Server struct {
	cfg        config.Config
	db         *sql.DB
	store      *settings.Store
	httpServer *http.Server
	router     http.Handler
	actualAddr string
}

func New(cfg config.Config, db *sql.DB, store *settings.Store) *Server {
	s := &Server{cfg: cfg, db: db, store: store}
	s.buildRouter()
	s.httpServer = &http.Server{
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

func (s *Server) ListenAndServe() error {
	ln, err := s.resolveListener()
	if err != nil {
		return err
	}
	fmt.Printf("carryAPI listening on %s (broadcast=%s)\n", s.actualAddr, broadcastLabel(s.store))
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
