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
	"sync"
)

// SyncCache is a thread-safe wrapper around [Cache].
// It uses an internal Mutex to provide safe concurrent access.
//
// Use [SyncCache] when multiple goroutines need concurrent access to the cache.
// Note: Unlike typical read-optimized caches, TinyLFU's Get operation has side effects
// (updates frequency tracking and item positions), so all operations use exclusive locks.
// If you have a single goroutine or external synchronization, use [Cache] instead
// for better performance (avoids mutex overhead).
type SyncCache = SyncT

// SyncT is the thread-safe cache type (alias for [SyncCache]).
// Prefer using [SyncCache] for clearer code.
type SyncT struct {
	mu  sync.RWMutex
	lfu *T
}

// NewSync creates a new thread-safe TinyLFU cache.
// See [New] for parameter documentation.
func NewSync(size int, samples int) *SyncT {
	return &SyncT{lfu: New(size, samples)}
}

func (s *SyncT) Len() int {
	return s.lfu.Len()
}

// Values https://go.dev/blog/range-functions
func (s *SyncT) Values() func(func(any) bool) {
	return func(yield func(any) bool) {
		s.mu.Lock()
		defer s.mu.Unlock()

		for key := range s.lfu.data {
			if val, ok := s.lfu.Get(key); ok && !yield(val) {
				// yield returns false if the loop should stop (e.g., 'break' was called)
				return
			}
		}
	}
}

// Get retrieves an item from the cache (thread-safe).
// Note: Get has side effects (updates frequency tracking), so it uses an exclusive lock.
func (s *SyncT) Get(key string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lfu.Get(key)
}

// Set inserts or updates an item in the cache (thread-safe).
func (s *SyncT) Set(item *Item) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lfu.Set(item)
}

// Del removes an item from the cache (thread-safe).
func (s *SyncT) Del(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lfu.Del(key)
}
