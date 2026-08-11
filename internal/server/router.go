package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"carryapi/internal/middleware"
	"carryapi/web"
)

func (s *Server) buildRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/api/health", s.handleHealth)

	deps := s.deps

	// auth(限流包裹登录/注册);only mount if AuthHandler + Sessions + Users present
	if deps.Auth != nil && deps.Sessions != nil && deps.Users != nil {
		r.Route("/api/auth", func(r chi.Router) {
			r.Use(middleware.SessionMiddleware(deps.Sessions, deps.Users))
			r.With(middleware.RateLimit(10, time.Minute)).Post("/login", deps.Auth.Login)
			r.With(middleware.RateLimit(10, time.Minute)).Post("/register", deps.Auth.Register)
			r.Post("/2fa/complete", deps.Auth.Complete2FA)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireLogin())
				r.Use(middleware.CSRFMiddleware())
				r.Post("/logout", deps.Auth.Logout)
				r.Get("/me", deps.Auth.Me)
				r.Post("/2fa/setup", deps.Auth.Setup2FA)
				r.Post("/2fa/disable", deps.Auth.Disable2FA)
			})
			// OAuth begin/callback are public (no RequireLogin): the user is
			// authenticating via the provider, so no session exists yet.
			if deps.OAuth != nil {
				r.Get("/oauth/{provider}", deps.OAuth.Begin)
				r.Get("/oauth/callback", deps.OAuth.Callback)
			}
		})
	}

	// 管理端点(需登录 + CSRF)
	if deps.Sessions != nil && deps.Users != nil {
		r.Group(func(r chi.Router) {
			r.Use(middleware.SessionMiddleware(deps.Sessions, deps.Users))
			r.Use(middleware.CSRFMiddleware())
			// 需登录
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireLogin())
				if deps.Keys != nil {
					r.Get("/api/keys", deps.Keys.List)
					r.Post("/api/keys", deps.Keys.Create)
					r.Put("/api/keys/{id}", deps.Keys.Update)
					r.Delete("/api/keys/{id}", deps.Keys.Delete)
				}
				if deps.Quotas != nil {
					r.Get("/api/quotas", deps.Quotas.List)
				}
				if deps.Settings != nil {
					r.Get("/api/settings", deps.Settings.Get)
				}
			})
			// admin only
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireLogin())
				r.Use(middleware.RequireRole("admin"))
				if deps.UsersH != nil {
					r.Get("/api/users", deps.UsersH.List)
					r.Post("/api/users", deps.UsersH.Create)
					r.Put("/api/users/{id}", deps.UsersH.Update)
					r.Delete("/api/users/{id}", deps.UsersH.Delete)
				}
				if deps.Quotas != nil {
					r.Put("/api/quotas/{id}", deps.Quotas.Update)
				}
				if deps.Settings != nil {
					r.Put("/api/settings", deps.Settings.Update)
				}
			})
		})
	}

	// 前端静态资源(SPA)
	r.Handle("/*", web.Handler())
	s.router = r
	return r
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
