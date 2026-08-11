package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type RateLimiter struct {
	mu     sync.Mutex
	counts map[string]int
	max    int
	window time.Duration
}

func RateLimit(max int, window time.Duration) func(http.Handler) http.Handler {
	rl := &RateLimiter{counts: make(map[string]int), max: max, window: window}
	go rl.cleanup()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			rl.mu.Lock()
			rl.counts[ip]++
			allowed := rl.counts[ip] <= rl.max
			rl.mu.Unlock()
			if !allowed {
				http.Error(w, `{"error":"rate limited"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP extracts the client IP for rate limiting, preferring the first
// X-Forwarded-For value (reverse-proxy deployments) and falling back to
// RemoteAddr. NOTE: X-Forwarded-For can be spoofed if the server is directly
// exposed to clients; in production validate it against a trusted proxy /
// require XFF to be set only by a trusted reverse proxy before relying on it.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i] // 取第一个(离客户端最近)
		}
		if ip := strings.TrimSpace(xff); ip != "" {
			return ip
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		rl.counts = make(map[string]int)
		rl.mu.Unlock()
	}
}
