// Package ttlcache is a small in-memory cache whose entries expire and whose size is capped, for the answers of
// remote APIs: keys partly come from users (search queries, durations), so an unbounded map could grow forever.
package ttlcache

import (
	"sync"
	"time"
)

type entry[V any] struct {
	v       V
	expires time.Time
}

// Cache is safe for concurrent use.
type Cache[K comparable, V any] struct {
	ttl time.Duration
	max int

	mu sync.Mutex
	m  map[K]entry[V]
}

// New returns a cache keeping each entry ttl, and at most max entries.
func New[K comparable, V any](ttl time.Duration, max int) *Cache[K, V] {
	return &Cache[K, V]{ttl: ttl, max: max, m: map[K]entry[V]{}}
}

// Get returns the value of k if it is cached and not expired.
func (c *Cache[K, V]) Get(k K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[k]
	if !ok || time.Now().After(e.expires) {
		var zero V
		return zero, false
	}
	return e.v, true
}

// Set caches v for k. A full cache first drops its expired entries, then a random one.
// ponytail: random eviction, an LRU if the hit rate ever matters.
func (c *Cache[K, V]) Set(k K, v V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if _, ok := c.m[k]; !ok && len(c.m) >= c.max {
		for key, e := range c.m {
			if now.After(e.expires) {
				delete(c.m, key)
			}
		}
		for key := range c.m {
			if len(c.m) < c.max {
				break
			}
			delete(c.m, key)
		}
	}
	c.m[k] = entry[V]{v: v, expires: now.Add(c.ttl)}
}
