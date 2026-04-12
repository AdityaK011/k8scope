package auth

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipLimiter struct {
	mu       sync.Mutex
	limiters map[string]*entry
}

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newIPLimiter(done <-chan struct{}) *ipLimiter {
	l := &ipLimiter{limiters: make(map[string]*entry)}
	// Prune stale entries every 5 minutes.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				l.mu.Lock()
				for ip, e := range l.limiters {
					if time.Since(e.lastSeen) > 10*time.Minute {
						delete(l.limiters, ip)
					}
				}
				l.mu.Unlock()
			}
		}
	}()
	return l
}

func (l *ipLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	if e, ok := l.limiters[ip]; ok {
		e.lastSeen = time.Now()
		return e.limiter
	}
	// 10 requests per minute, burst of 5.
	limiter := rate.NewLimiter(rate.Every(6*time.Second), 5)
	l.limiters[ip] = &entry{limiter: limiter, lastSeen: time.Now()}
	return limiter
}

// NewRateLimitMiddleware creates a rate limit middleware with a shared IP limiter.
// The returned stop function stops the background cleanup goroutine.
func NewRateLimitMiddleware() (func(http.Handler) http.Handler, func()) {
	done := make(chan struct{})
	limiter := newIPLimiter(done)

	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			if ip == "" {
				ip = r.RemoteAddr
			}
			if !limiter.get(ip).Allow() {
				http.Error(w, `{"error":"rate_limited","hint":"too many requests, try again later"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}

	return mw, func() { close(done) }
}
