package api

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimitConfig configures the RateLimit middleware.
type RateLimitConfig struct {
	Rate            float64                                      // requests per second
	Burst           int                                          // max burst
	KeyFunc         func(r *http.Request) string                 // default: remote IP
	OnLimit         func(w http.ResponseWriter, r *http.Request) // default: 429 response
	CleanupInterval time.Duration                                // how often to prune idle limiters (default: 1m)
	MaxIdle         time.Duration                                // remove limiters idle longer than this (default: 5m)
}

// RateLimit returns middleware that applies per-key rate limiting.
func RateLimit(cfg RateLimitConfig) Middleware {
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = func(r *http.Request) string {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				return r.RemoteAddr
			}
			return host
		}
	}
	if cfg.OnLimit == nil {
		cfg.OnLimit = func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Retry-After", "1")
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		}
	}

	cleanupInterval := cfg.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = time.Minute
	}
	maxIdle := cfg.MaxIdle
	if maxIdle <= 0 {
		maxIdle = 5 * time.Minute
	}

	var (
		mu          sync.Mutex
		limiters    = make(map[string]*limiterEntry)
		lastCleanup time.Time
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := cfg.KeyFunc(r)

			mu.Lock()
			now := time.Now()

			// Lazy cleanup of expired limiters.
			if now.Sub(lastCleanup) >= cleanupInterval {
				for k, e := range limiters {
					if now.Sub(e.lastSeen) > maxIdle {
						delete(limiters, k)
					}
				}
				lastCleanup = now
			}

			entry, ok := limiters[key]
			if !ok {
				entry = &limiterEntry{
					limiter: rate.NewLimiter(rate.Limit(cfg.Rate), cfg.Burst),
				}
				limiters[key] = entry
			}
			entry.lastSeen = now
			mu.Unlock()

			if !entry.limiter.Allow() {
				retryAfter := strconv.FormatFloat(1/cfg.Rate, 'f', 0, 64)
				w.Header().Set("Retry-After", retryAfter)
				cfg.OnLimit(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}
