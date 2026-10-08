package api

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestFailureLimiter(t *testing.T) {
	l := newFailureLimiter(3, time.Minute)
	for range 3 {
		if l.blocked("1.2.3.4") > 0 {
			t.Fatal("blocked before the limit")
		}
		l.fail("1.2.3.4")
	}
	if l.blocked("1.2.3.4") == 0 {
		t.Fatal("not blocked after 3 failures")
	}
	if l.blocked("5.6.7.8") > 0 {
		t.Fatal("another IP is blocked")
	}
	l.ips["1.2.3.4"].reset = time.Now().Add(-time.Second)
	if l.blocked("1.2.3.4") > 0 {
		t.Fatal("still blocked after the window")
	}
}

func TestClientIP(t *testing.T) {
	for _, tc := range []struct{ remote, xff, want string }{
		{"203.0.113.7:1234", "", "203.0.113.7"},
		{"203.0.113.7:1234", "1.1.1.1", "203.0.113.7"},               // forged from the internet: ignored
		{"172.18.0.5:1234", "6.6.6.6, 198.51.100.2", "198.51.100.2"}, // behind a proxy: the entry it appended
		{"127.0.0.1:1234", "", "127.0.0.1"},
	} {
		r := httptest.NewRequest("POST", "/", nil)
		r.RemoteAddr = tc.remote
		if tc.xff != "" {
			r.Header.Set("X-Forwarded-For", tc.xff)
		}
		if got := clientIP(r); got != tc.want {
			t.Errorf("clientIP(%s, %q) = %s, want %s", tc.remote, tc.xff, got, tc.want)
		}
	}
}
