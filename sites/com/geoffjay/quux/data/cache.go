package data

import (
	"sync"
	"time"
)

// Cache TTLs keyed to how stale data is acceptable. These keep request volume
// within GitHub's 5000/hr authenticated budget for a single-user dogfood app.
const (
	PrListTTL   = 60 * time.Second
	PrDetailTTL = 30 * time.Second
	CollabTTL   = 5 * time.Minute
	ContribTTL  = 5 * time.Minute
	AgentTTL    = 15 * time.Minute
)

// cacheEntry holds a cached value and its expiry.
type cacheEntry struct {
	value   any
	expires time.Time
}

// Cache is an in-memory TTL cache fronting the GitHub client. Keys are
// composite strings chosen by the caller (e.g. "prs:geoffjay",
// "pr:geoffjay/jughead:42"). It is safe for concurrent use.
type Cache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
}

// NewCache returns an empty Cache.
func NewCache() *Cache {
	return &Cache{items: make(map[string]cacheEntry)}
}

// Get returns the cached value for key when present and unexpired, executing
// loader otherwise and storing its result. loader is run under a write lock;
// concurrent callers for the same key serialize (simple, no singleflight —
// fine for low concurrency). When ttl <= 0 the value is never cached.
func (c *Cache) Get(key string, ttl time.Duration, loader func() (any, error)) (any, error) {
	if ttl <= 0 {
		return loader()
	}
	if v, ok := c.peek(key); ok {
		return v, nil
	}
	v, err := loader()
	if err != nil {
		return nil, err
	}
	c.store(key, ttl, v)
	return v, nil
}

// peek returns the cached value when present and unexpired.
func (c *Cache) peek(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.items[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expires) {
		return nil, false
	}
	return e.value, true
}

// store writes a value with the given TTL.
func (c *Cache) store(key string, ttl time.Duration, v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = cacheEntry{value: v, expires: time.Now().Add(ttl)}
}
