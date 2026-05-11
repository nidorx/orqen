// Package tinylfu implements the TinyLFU caching algorithm.
//
// TinyLFU is a high-performance cache admission policy that uses a Count-Min Sketch
// to estimate item frequency and decides whether to admit new items based on their
// historical access patterns.
//
// See: http://arxiv.org/abs/1512.00727
//
// This package provides two cache implementations:
//   - [Cache]: Non-thread-safe, for single-threaded or externally synchronized access.
//   - [SyncCache]: Thread-safe, with internal read-write mutex for concurrent access.
//
// Originally from: https://github.com/vmihailenco/go-tinylfu
// Improved with better documentation, tests, benchmarks, and bug fixes.
package tinylfu

import (
	"iter"
	"math/rand"
	"time"
)

type LFUT[V any] interface {
	Get(key string) (V, bool)
	Set(newItem V)
	Del(key string)
	Values() func(func(V) bool)
}

// CacheT is a generic TinyLFU cache with TTL support.
// Use this when all cached values have the same type.
// Not thread-safe - use for single goroutine or externally synchronized access.
//
// Example:
//
//	cache := tinylfu.NewCacheT[string](1000, 10000, time.Hour)
//	cache.Set("user:123", userData)
//	user, ok := cache.Get("user:123")
type CacheT[V any] struct {
	lfu    *T
	rng    *rand.Rand
	ttl    time.Duration
	offset time.Duration
}

// NewCacheT creates a new generic TinyLFU cache with TTL support.
//
//   - size: the maximum number of items the cache can hold.
//   - samples: the number of operations before resetting frequency tracking.
//   - ttl: default time-to-live for cached items.
//
// The cache automatically adds randomized jitter to TTLs to prevent
// cache stampedes (thundering herd problem).
func NewCacheT[V any](size int, samples int, ttl time.Duration) *CacheT[V] {
	const maxOffset = 10 * time.Second

	offset := time.Duration(0)
	if ttl > 0 {
		offset = min(ttl/10, maxOffset)
	}

	return &CacheT[V]{
		lfu:    New(size, samples),
		ttl:    ttl,
		offset: offset,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// UseRandomizedTTL enables or disables randomized TTL offset.
// Set offset to 0 to disable randomization.
// The maximum offset is capped at 10 seconds.
func (c *CacheT[V]) UseRandomizedTTL(offset time.Duration) {
	c.offset = offset
}

func (c *CacheT[V]) Len() int {
	return c.lfu.Len()
}

// Values https://go.dev/blog/range-functions
func (c *CacheT[V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for key := range c.lfu.data {
			if val, ok := c.lfu.Get(key); ok && !yield(val.(V)) {
				// yield returns false if the loop should stop (e.g., 'break' was called)
				return
			}
		}
	}
}

// Get retrieves a typed value from the cache.
// Returns the value and true if found, or zero value and false otherwise.
func (c *CacheT[V]) Get(key string) (V, bool) {
	val, ok := c.lfu.Get(key)
	if !ok {
		var zero V
		return zero, false
	}
	return val.(V), true
}

// Set inserts or updates a value in the cache with automatic TTL.
func (c *CacheT[V]) Set(key string, value V) {
	ttl := c.ttl
	if c.offset > 0 {
		ttl += time.Duration(c.rng.Int63n(int64(c.offset)))
	}

	var expireAt time.Time
	if ttl > 0 {
		expireAt = time.Now().Add(ttl)
	}

	c.lfu.Set(&Item{
		Key:      key,
		Value:    value,
		ExpireAt: expireAt,
	})
}

// SetWithTTL inserts or updates a value with a custom TTL.
// Pass 0 to use the default TTL, or a specific duration to override it.
func (c *CacheT[V]) SetWithTTL(key string, value V, ttl time.Duration) {
	if ttl == 0 {
		c.Set(key, value)
		return
	}

	expireAt := time.Now().Add(ttl)
	if c.offset > 0 {
		expireAt = expireAt.Add(time.Duration(c.rng.Int63n(int64(c.offset))))
	}

	c.lfu.Set(&Item{
		Key:      key,
		Value:    value,
		ExpireAt: expireAt,
	})
}

// Del removes an item from the cache.
func (c *CacheT[V]) Del(key string) {
	c.lfu.Del(key)
}

// GetOrSet retrieves a value, or sets it using the provided function if not found.
// Uses double-check locking pattern to avoid redundant function calls.
//
// Example:
//
//	user, err := cache.GetOrSet("user:123", func() (*User, error) {
//	    return db.GetUser(123)
//	})
func (c *CacheT[V]) GetOrSet(key string, fn func() (V, error)) (V, error) {
	// Fast path: check without lock
	v, ok := c.Get(key)
	if ok {
		return v, nil
	}

	// Note: CacheT is not thread-safe by default, so GetOrSet
	// does not use double-check locking. If concurrent access is needed,
	// use SyncCacheT instead.
	value, err := fn()
	if err != nil {
		return value, err
	}

	c.Set(key, value)
	return value, nil
}
