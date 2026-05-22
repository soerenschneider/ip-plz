package main

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*entry
}

type entry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var limiter = &rateLimiter{clients: make(map[string]*entry)}

func (rl *rateLimiter) get(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if e, ok := rl.clients[ip]; ok {
		e.lastSeen = time.Now()
		return e.limiter
	}

	l := rate.NewLimiter(rate.Every(time.Minute/3), 5) // 3 req/min, burst of 5
	rl.clients[ip] = &entry{limiter: l, lastSeen: time.Now()}
	return l
}

func (rl *rateLimiter) cleanup() {
	for range time.Tick(5 * time.Minute) {
		rl.mu.Lock()
		for ip, e := range rl.clients {
			if time.Since(e.lastSeen) > 5*time.Minute {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if !limiter.get(ip).Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}
