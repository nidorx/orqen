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
	"sync"
	"time"
)

//------------------------------------------------------------------------------

// SyncCacheT is a thread-safe generic TinyLFU cache with TTL support.
// Use this when all cached values have the same type and multiple goroutines
// need concurrent access.
//
// Example:
//
//	cache := tinylfu.NewSyncCacheT[*model.Execution](1000, 100000, 30*time.Minute)
//	execution, err := cache.GetOrSet(id.String(), func() (*model.Execution, error) {
//	    return loadFromDB(id)
//	})
type SyncCacheT[V any] struct {
	mu     sync.Mutex
	lfu    *T
	rng    *rand.Rand
	ttl    time.Duration
	offset time.Duration
}

// NewSyncCacheT creates a new thread-safe generic TinyLFU cache with TTL support.
//
//   - size: the maximum number of items the cache can hold.
//   - samples: the number of operations before resetting frequency tracking.
//   - ttl: default time-to-live for cached items.
//
// The cache automatically adds randomized jitter to TTLs to prevent
// cache stampedes (thundering herd problem).
func NewSyncCacheT[V any](size int, samples int, ttl time.Duration) *SyncCacheT[V] {
	const maxOffset = 10 * time.Second

	offset := time.Duration(0)
	if ttl > 0 {
		offset = min(ttl/10, maxOffset)
	}

	return &SyncCacheT[V]{
		lfu:    New(size, samples),
		ttl:    ttl,
		offset: offset,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// UseRandomizedTTL enables or disables randomized TTL offset.
// Set offset to 0 to disable randomization.
// The maximum offset is capped at 10 seconds.
func (c *SyncCacheT[V]) UseRandomizedTTL(offset time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.offset = offset
}

func (c *SyncCacheT[V]) Len() int {
	return c.lfu.Len()
}

// Values https://go.dev/blog/range-functions
func (c *SyncCacheT[V]) Values() iter.Seq[V] {
	// Snapshot keys under lock to avoid holding the mutex during iteration.
	// This prevents blocking concurrent Set/Get/Del operations while the
	// caller iterates over values.
	c.mu.Lock()
	keys := make([]string, 0, len(c.lfu.data))
	for key := range c.lfu.data {
		keys = append(keys, key)
	}
	c.mu.Unlock()

	return func(yield func(V) bool) {
		for _, key := range keys {
			if val, ok := c.Get(key); ok && !yield(val) {
				// yield returns false if the loop should stop (e.g., 'break' was called)
				return
			}
		}
	}
}

// Get retrieves a typed value from the cache (thread-safe).
// Returns the value and true if found, or zero value and false otherwise.
func (c *SyncCacheT[V]) Get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	val, ok := c.lfu.Get(key)
	if !ok {
		var zero V
		return zero, false
	}
	return val.(V), true
}

// Set inserts or updates a value in the cache with automatic TTL (thread-safe).
func (c *SyncCacheT[V]) Set(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Store in cache
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

// SetWithTTL inserts or updates a value with a custom TTL (thread-safe).
// Pass 0 to use the default TTL, or a specific duration to override it.
func (c *SyncCacheT[V]) SetWithTTL(key string, value V, ttl time.Duration) {
	if ttl == 0 {
		c.Set(key, value)
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

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

// Del removes an item from the cache (thread-safe).
func (c *SyncCacheT[V]) Del(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lfu.Del(key)
}

// GetOrSet retrieves a value, or sets it using the provided function if not found.
// Uses double-check locking pattern to avoid redundant function calls under contention.
//
// Example:
//
//	execution, err := cache.GetOrSet(id.String(), func() (*model.Execution, error) {
//	    return db.LoadExecution(id)
//	})
func (c *SyncCacheT[V]) GetOrSet(key string, fn func() (V, error)) (V, error) {
	// Fast path: check without full lock contention
	v, ok := c.Get(key)
	if ok {
		return v, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring lock (another goroutine may have set the value)
	val, ok := c.lfu.Get(key)
	if ok {
		return val.(V), nil
	}

	// Execute the function to get the value
	value, err := fn()
	if err != nil {
		return value, err
	}

	// Store in cache
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

	return value, nil
}
