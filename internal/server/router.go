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
			r.With(middleware.RateLimit(10, time.Minute)).Post("/2fa/complete", deps.Auth.Complete2FA)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireLogin())
				r.Use(middleware.CSRFMiddleware())
				r.Post("/logout", deps.Auth.Logout)
				r.Get("/me", deps.Auth.Me)
				r.Post("/2fa/setup", deps.Auth.Setup2FA)
				r.Post("/2fa/disable", deps.Auth.Disable2FA)
				// Passkey registration requires a logged-in user (binds the new
				// credential to the account). CSRF is intentionally NOT applied
				// here: the WebAuthn finish request body is the authenticator's
				// CBOR/JSON attestation, which cannot carry a CSRF token, and
				// the ceremony is itself bound to the BeginRegistration session.
				if deps.Passkey != nil {
					r.Post("/passkey/register/begin", deps.Passkey.RegisterBegin)
					r.Post("/passkey/register/finish", deps.Passkey.RegisterFinish)
				}
			})
			// Passkey login is public (no session yet): the client supplies the
			// email so the server can look up the account + credentials.
			if deps.Passkey != nil {
				r.Post("/passkey/login/begin", deps.Passkey.LoginBegin)
				r.Post("/passkey/login/finish", deps.Passkey.LoginFinish)
			}
			// OAuth begin/callback are public (no RequireLogin): the user is
			// authenticating via the provider, so no session exists yet.
			if deps.OAuth != nil {
				r.Get("/oauth/{provider}", deps.OAuth.Begin)
				r.Get("/oauth/callback/{provider}", deps.OAuth.Callback)
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
				if deps.Catalog != nil {
					r.Get("/api/providers", deps.Catalog.ListProviders)
					r.Post("/api/providers", deps.Catalog.CreateProvider)
					r.Put("/api/providers/{id}", deps.Catalog.UpdateProvider)
					r.Delete("/api/providers/{id}", deps.Catalog.DeleteProvider)
					r.Get("/api/models", deps.Catalog.ListModels)
					r.Post("/api/models", deps.Catalog.CreateModel)
					r.Put("/api/models/{id}", deps.Catalog.UpdateModel)
					r.Delete("/api/models/{id}", deps.Catalog.DeleteModel)
					r.Get("/api/models/{id}/price", deps.Catalog.GetModelPrice)
					r.Put("/api/models/{id}/price", deps.Catalog.SetModelPrice)
				}
			})
		})
	}

	// 代理端点(在 SessionMiddleware 之外,用 API Key 鉴权)
	// chi 的 /v1/* 通配匹配多级路径(/v1/models、/v1/chat/completions 等),
	// 全部交给 Proxy.ServeHTTP 按 Path 内部分派。
	if deps.Proxy != nil {
		r.Handle("/v1/*", deps.Proxy)
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
