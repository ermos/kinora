package ttlcache

import (
	"testing"
	"time"
)

func TestCacheExpiresAndStaysBounded(t *testing.T) {
	c := New[int, string](time.Hour, 3)
	for i := range 10 {
		c.Set(i, "v")
	}
	if len(c.m) != 3 {
		t.Fatalf("%d entries, want 3", len(c.m))
	}
	if v, ok := c.Get(9); !ok || v != "v" {
		t.Fatal("the last entry set is missing")
	}
	e := c.m[9]
	e.expires = time.Now().Add(-time.Second)
	c.m[9] = e
	if _, ok := c.Get(9); ok {
		t.Fatal("expired entry returned")
	}
}
