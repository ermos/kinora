package stream

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// limiter paces one stream to a byte rate, shared by all its requests: HLS segments and the parallel ranges a
// player opens on a file. Each write waits for its slot, so bursts (buffering, seeks) stay under the cap.
type limiter struct {
	mu   sync.Mutex
	rate float64   // bytes per second
	next time.Time // when the bytes sent so far are paid off
}

func (l *limiter) wait(ctx context.Context, n int) error {
	l.mu.Lock()
	now := time.Now()
	if l.next.Before(now) {
		l.next = now // no credit saved while idle
	}
	at := l.next
	l.next = l.next.Add(time.Duration(float64(n) / l.rate * float64(time.Second)))
	l.mu.Unlock()
	if d := time.Until(at); d > 0 {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-t.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// limiters holds the limiter of each stream being played.
// ponytail: swept on access, an entry is a few bytes per title played.
type limiters struct {
	mu    sync.Mutex
	m     map[string]*limiter
	swept time.Time
}

const limiterIdle = 10 * time.Minute

func (ls *limiters) get(id string, bytesPerSec float64) *limiter {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	now := time.Now()
	if ls.m == nil {
		ls.m = map[string]*limiter{}
	}
	if now.Sub(ls.swept) > limiterIdle {
		for k, l := range ls.m {
			l.mu.Lock()
			idle := now.Sub(l.next) > limiterIdle
			l.mu.Unlock()
			if idle {
				delete(ls.m, k)
			}
		}
		ls.swept = now
	}
	l, ok := ls.m[id]
	if !ok {
		l = &limiter{rate: bytesPerSec}
		ls.m[id] = l
	}
	return l
}

// throttled paces the body of a response; headers pass through untouched.
type throttled struct {
	http.ResponseWriter
	ctx context.Context
	l   *limiter
}

func (t throttled) Write(b []byte) (int, error) {
	written := 0
	for len(b) > 0 {
		n := min(len(b), 32<<10) // small slices keep the pace smooth
		if err := t.l.wait(t.ctx, n); err != nil {
			return written, err
		}
		m, err := t.ResponseWriter.Write(b[:n])
		written += m
		if err != nil {
			return written, err
		}
		b = b[n:]
	}
	return written, nil
}
