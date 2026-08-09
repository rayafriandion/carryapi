package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"carryapi/web"
)

func (s *Server) buildRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/api/health", s.handleHealth)
	// 前端静态资源(SPA)
	r.Handle("/*", web.Handler())
	s.router = r
	return r
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
