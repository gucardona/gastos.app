package httpx

import (
	"sync"
	"time"
)

// RateLimiter counts attempts per key in a sliding window.
type RateLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string][]time.Time
}

// NewRateLimiter returns a limiter that allows limit attempts per key within window.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{max: limit, window: window, hits: map[string][]time.Time{}}
}

// Allow records an attempt and reports whether it is still within the limit.
func (l *RateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	kept := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.max {
		l.hits[key] = kept
		return false
	}
	l.hits[key] = append(kept, now)
	return true
}
