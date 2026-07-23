package api

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type fixedWindowRateLimiter struct {
	limit   int
	window  time.Duration
	mu      sync.Mutex
	entries map[string]rateLimitEntry
}

type rateLimitEntry struct {
	count       int
	windowStart time.Time
}

func newFixedWindowRateLimiter(limit int, window time.Duration) *fixedWindowRateLimiter {
	if limit <= 0 {
		return nil
	}
	return &fixedWindowRateLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string]rateLimitEntry),
	}
}

func (l *fixedWindowRateLimiter) Allow(key string) bool {
	if l == nil {
		return true
	}

	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := l.entries[key]
	if entry.windowStart.IsZero() || now.Sub(entry.windowStart) >= l.window {
		l.entries[key] = rateLimitEntry{count: 1, windowStart: now}
		l.cleanupLocked(now)
		return true
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	l.entries[key] = entry
	return true
}

func (l *fixedWindowRateLimiter) cleanupLocked(now time.Time) {
	for key, entry := range l.entries {
		if now.Sub(entry.windowStart) >= 2*l.window {
			delete(l.entries, key)
		}
	}
}

func rateLimitKey(r *http.Request, workflowID string) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		host = r.RemoteAddr
	}
	return workflowID + "|" + host
}
