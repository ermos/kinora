package api

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

const (
	loginMaxFailures = 10
	// accountMaxFailures caps the guesses on one account from all IPs together, higher than the per IP limit so a
	// family member mistyping does not lock the account.
	accountMaxFailures = 30
	loginWindow        = 15 * time.Minute
)

// defaultTrustedProxies covers a reverse proxy on the host or on a Docker network: loopback and private ranges.
var defaultTrustedProxies = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128"), netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("192.168.0.0/16"), netip.MustParsePrefix("fc00::/7"),
}

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

// clientIP is the peer address, or the last X-Forwarded-For entry when the peer is a trusted reverse proxy (the
// entry it appended itself): from anywhere else, a forged header is ignored.
func clientIP(r *http.Request, trusted []netip.Prefix) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	for _, p := range trusted {
		if p.Contains(ip.Unmap()) {
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				parts := strings.Split(xff, ",")
				return strings.TrimSpace(parts[len(parts)-1])
			}
			break
		}
	}
	return host
}
