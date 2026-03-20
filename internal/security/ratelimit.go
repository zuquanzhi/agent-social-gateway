package security

import (
	"net/http"
	"sync"
	"time"

	"github.com/zuwance/agent-social-gateway/internal/config"
)

type RateLimiter struct {
	cfg     *config.RateLimitConfig
	buckets map[string]*tokenBucket
	mu      sync.Mutex
}

type tokenBucket struct {
	tokens    float64
	maxTokens float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func NewRateLimiter(cfg *config.RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		cfg:     cfg,
		buckets: make(map[string]*tokenBucket),
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.cfg.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		key := r.RemoteAddr
		if agentID := GetAgentID(r.Context()); agentID != "" {
			key = string(agentID)
		}

		if !rl.Allow(key) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, ok := rl.buckets[key]
	if !ok {
		maxTokens := float64(rl.cfg.RequestsPerMin)
		bucket = &tokenBucket{
			tokens:     maxTokens,
			maxTokens:  maxTokens,
			refillRate: maxTokens / 60.0,
			lastRefill: time.Now(),
		}
		rl.buckets[key] = bucket
	}

	bucket.refill()

	if bucket.tokens < 1 {
		return false
	}
	bucket.tokens--
	return true
}

func (b *tokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now
}

func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for key, bucket := range rl.buckets {
		if bucket.lastRefill.Before(cutoff) {
			delete(rl.buckets, key)
		}
	}
}
