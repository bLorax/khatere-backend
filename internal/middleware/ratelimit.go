package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimit limits how many times one client IP may hit the wrapped
// handler inside window. The count is stored in Redis under
// "ratelimit:login:<ip>".
func RateLimit(cache *redis.Client, limit int, window time.Duration) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			key := "ratelimit:login:" + ip

			count, err := cache.Incr(r.Context(), key).Result()
			if err != nil {
				// Redis is down or unreachable: fail open, don't block login
				// over an infrastructure problem.
				next(w, r)
				return
			}

			if count == 1 {
				cache.Expire(r.Context(), key, window)
			}

			if count > int64(limit) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(map[string]any{
					"error": map[string]string{
						"code":    "RATE_LIMITED",
						"message": "too many login attempts, try again later",
					},
				})
				return
			}

			next(w, r)
		}
	}
}

// clientIP strips the port from r.RemoteAddr. Falls back to the raw
// value if it does not look like host:port (e.g. behind some test
// setups or unusual transports).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
