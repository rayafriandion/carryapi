package middleware

import (
	"context"
	"net/http"

	"carryapi/internal/auth"
	"carryapi/internal/user"
)

type UserKey struct{}

func SessionMiddleware(sessions *auth.SessionStore, users *user.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(auth.SessionCookieName)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			sess, err := sessions.Lookup(cookie.Value)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			u, err := users.GetByID(sess.UserID)
			if err != nil || u.Status != "active" {
				next.ServeHTTP(w, r)
				return
			}
			ctx := context.WithValue(r.Context(), UserKey{}, &u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserFromContext(ctx context.Context) (*user.User, bool) {
	u, ok := ctx.Value(UserKey{}).(*user.User)
	return u, ok
}

func RequireLogin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := UserFromContext(r.Context()); !ok {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
