package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	mu      sync.Mutex
	entries map[string]*entry
	limit   int
	window  time.Duration
}
type entry struct {
	started time.Time
	count   int
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{entries: map[string]*entry{}, limit: limit, window: window}
}
func (l *Limiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[key]
	if e == nil || now.Sub(e.started) >= l.window {
		l.entries[key] = &entry{started: now, count: 1}
		return true
	}
	if e.count >= l.limit {
		return false
	}
	e.count++
	return true
}
