package middleware

import (
	"net/http"
)

const CSRFHeader = "X-CSRF-Token"
const CSRFCookie = "carryapi_csrf"

func CSRFMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			cookie, err := r.Cookie(CSRFCookie)
			if err != nil {
				http.Error(w, `{"error":"missing csrf cookie"}`, http.StatusForbidden)
				return
			}
			if r.Header.Get(CSRFHeader) != cookie.Value {
				http.Error(w, `{"error":"csrf token mismatch"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
