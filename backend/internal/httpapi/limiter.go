package httpapi

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ipLimiter ограничивает попытки входа/регистрации/refresh с одного IP:
// 10 запросов сразу, затем 1 запрос в 6 секунд.
type ipLimiter struct {
	mu      sync.Mutex
	entries map[string]*limiterEntry
	lastGC  time.Time
}

type limiterEntry struct {
	lim  *rate.Limiter
	seen time.Time
}

func newIPLimiter() *ipLimiter {
	return &ipLimiter{entries: make(map[string]*limiterEntry), lastGC: time.Now()}
}

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.Sub(l.lastGC) > 10*time.Minute {
		for k, e := range l.entries {
			if now.Sub(e.seen) > 10*time.Minute {
				delete(l.entries, k)
			}
		}
		l.lastGC = now
	}
	e := l.entries[ip]
	if e == nil {
		e = &limiterEntry{lim: rate.NewLimiter(rate.Every(6*time.Second), 10)}
		l.entries[ip] = e
	}
	e.seen = now
	return e.lim.Allow()
}
