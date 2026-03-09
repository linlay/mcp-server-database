package transport

import (
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"mcp-server-database/internal/observability"
)

type RateLimitConfig struct {
	Enabled bool
	RPS     float64
	Burst   int
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	rps     float64
	burst   float64
	buckets map[string]tokenBucket
}

func newRateLimiter(cfg RateLimitConfig) *rateLimiter {
	rps := cfg.RPS
	if rps <= 0 {
		rps = 5
	}
	burst := float64(cfg.Burst)
	if burst < 1 {
		burst = 10
	}
	return &rateLimiter{
		rps:     rps,
		burst:   burst,
		buckets: map[string]tokenBucket{},
	}
}

func (l *rateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, ok := l.buckets[key]
	if !ok {
		bucket = tokenBucket{tokens: l.burst, last: now}
	}
	elapsed := now.Sub(bucket.last).Seconds()
	if elapsed > 0 {
		bucket.tokens = math.Min(l.burst, bucket.tokens+elapsed*l.rps)
		bucket.last = now
	}
	if bucket.tokens < 1 {
		l.buckets[key] = bucket
		return false
	}
	bucket.tokens--
	l.buckets[key] = bucket
	return true
}

func WithRateLimit(next http.Handler, cfg RateLimitConfig, logger *observability.Logger) http.Handler {
	if next == nil {
		next = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}
	if !cfg.Enabled {
		return next
	}
	if logger == nil {
		logger = observability.NopLogger()
	}
	limiter := newRateLimiter(cfg)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := rateLimitKey(r)
		if limiter.allow(key, time.Now()) {
			next.ServeHTTP(w, r)
			return
		}
		logger.LogMCPError(nil, "http.rate_limit", 0, "rate_limited", key)
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
	})
}

func rateLimitKey(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			first := strings.TrimSpace(parts[0])
			if first != "" {
				return first
			}
		}
	}
	xri := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if xri != "" {
		return xri
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}
