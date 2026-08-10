package middleware

import (
	"net"
	"net/http"
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
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
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

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		rl.counts = make(map[string]int)
		rl.mu.Unlock()
	}
}
