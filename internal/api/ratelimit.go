package api

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	loginMaxFailures = 10
	loginWindow      = 15 * time.Minute
)

// failureLimiter counts failed attempts per IP in a fixed window: only failures count, so a family behind
// one address can still sign in while a guesser gets locked out.
type failureLimiter struct {
	max    int
	window time.Duration
	mu     sync.Mutex
	ips    map[string]*failures
}

type failures struct {
	count int
	reset time.Time
}

func newFailureLimiter(max int, window time.Duration) *failureLimiter {
	return &failureLimiter{max: max, window: window, ips: map[string]*failures{}}
}

// blocked returns how long the IP stays locked out, 0 when it may try.
func (l *failureLimiter) blocked(ip string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	f := l.ips[ip]
	if f == nil || f.count < l.max {
		return 0
	}
	return max(time.Until(f.reset), 0)
}

func (l *failureLimiter) fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.ips) > 4096 { // drop expired windows so scanning IPs cannot grow the map forever
		for k, f := range l.ips {
			if now.After(f.reset) {
				delete(l.ips, k)
			}
		}
	}
	f := l.ips[ip]
	if f == nil || now.After(f.reset) {
		f = &failures{reset: now.Add(l.window)}
		l.ips[ip] = f
	}
	f.count++
}

// clientIP is the peer address, or the last X-Forwarded-For entry when the peer is a reverse proxy on a
// private network (the entry it appended itself): from the internet, a forged header is ignored.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate()) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	return host
}
