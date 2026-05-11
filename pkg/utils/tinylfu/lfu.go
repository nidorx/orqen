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
	"container/list"
	"iter"
	"time"

	"github.com/cespare/xxhash/v2"
)

// LFU interface defines the core cache operations.
type LFU interface {
	Get(key string) (any, bool)
	Set(newItem *Item)
	Del(key string)
	Len() int
	Values() iter.Seq[any]
}

// Item represents a cache entry with optional expiration and eviction callback.
type Item struct {
	Key      string    // Key is the unique identifier for the cache entry.
	Value    any       // Value is the cached data.
	ExpireAt time.Time // ExpireAt is the absolute time when the item expires. Zero value means no expiration.
	OnEvict  func()    // OnEvict is called when the item is evicted from the cache. May be nil.
	listid   int       // listid identifies which list the item belongs to (0=LRU, 1=SLRU-one, 2=SLRU-two).
	keyh     uint64    // keyh stores the pre-computed hash of the key.
}

// expired reports whether the item has passed its expiration time.
func (item Item) expired() bool {
	return !item.ExpireAt.IsZero() && time.Now().After(item.ExpireAt)
}

// Cache is a TinyLFU cache implementation with automatic admission control.
// It combines an LRU cache (1% of capacity) with a segmented LRU cache (99% of capacity)
// and uses a Count-Min Sketch to estimate item frequency for admission decisions.
//
// Use [Cache] when you have a single goroutine or external synchronization.
// For concurrent access, use [SyncCache] instead.
type Cache = T

// T is the core TinyLFU cache type (alias for [Cache]).
// Prefer using [Cache] for clearer code.
type T struct {
	w           int                      // w is the write counter (for periodic reset of count sketch and doorkeeper).
	samples     int                      // samples is the number of samples before resetting frequency tracking.
	countSketch *cm4                     // countSketch tracks access frequencies using a Count-Min Sketch.
	bouncer     *doorkeeper              // bouncer is the Bloom filter admission policy.
	data        map[string]*list.Element // data maps keys to their list elements.
	lru         *lruCache                // lru is the small LRU cache (1% of total).
	slru        *slruCache               // slru is the segmented LRU cache (99% of total).
}

const lruPct = 1

// New creates a new TinyLFU cache with the given capacity and sample size.
//
//   - size: the maximum number of items the cache can hold.
//   - samples: the number of operations before resetting frequency tracking.
//     A good default is 10x the size (e.g., 10000 for a 1000-item cache).
//
// The cache allocates 1% of capacity to a small LRU and 99% to a segmented LRU.
func New(size int, samples int) *T {

	var (
		lruSize  = max((lruPct*size)/100, 1)
		slruSize = max(int(float64(size)*((100.0-lruPct)/100.0)), 1)
		slru20   = max(int(0.2*float64(slruSize)), 1)
		data     = make(map[string]*list.Element, size)
	)

	return &T{
		w:           0,
		samples:     samples,
		countSketch: newCM4(size),
		bouncer:     newDoorkeeper(samples, 0.01),
		data:        data,
		lru:         newLRU(lruSize, data),
		slru:        newSLRU(slru20, slruSize-slru20, data),
	}
}

// onEvict safely calls the item's OnEvict callback if set.
func (t *T) onEvict(item *Item) {
	if item.OnEvict != nil {
		item.OnEvict()
	}
}

func (t *T) Len() int {
	return len(t.data)
}

// Values https://go.dev/blog/range-functions
func (t *T) Values() iter.Seq[any] {
	return func(yield func(any) bool) {
		for key := range t.data {
			if val, ok := t.Get(key); ok && !yield(val) {
				// yield returns false if the loop should stop (e.g., 'break' was called)
				return
			}
		}
	}
}

// Get retrieves an item from the cache by key.
// Returns the value and true if found, or nil and false otherwise.
// Expired items are automatically evicted.
// A successful hit updates the item's position in the cache (promotes it).
func (t *T) Get(key string) (any, bool) {
	t.w++
	if t.w == t.samples {
		t.countSketch.reset()
		t.bouncer.reset()
		t.w = 0
	}

	keyh := xxhash.Sum64String(key)
	t.countSketch.add(keyh)

	val, ok := t.data[key]
	if !ok {
		return nil, false
	}

	item := val.Value.(*Item)
	if item.expired() {
		t.del(val)
		return nil, false
	}

	// Save the value since it is overwritten below.
	value := item.Value

	if item.listid == 0 {
		t.lru.get(val)
	} else {
		t.slru.get(val)
	}

	return value, true
}

// Set inserts or updates an item in the cache.
// If the key already exists, the value is updated and the item is promoted.
// If the key is new, cache admission policy decides whether to accept it
// based on historical access frequency.
func (t *T) Set(newItem *Item) {
	if e, ok := t.data[newItem.Key]; ok {
		// Key is already in our cache.
		// `Set` will act as a `Get` for list movements
		item := e.Value.(*Item)
		item.Value = newItem.Value
		t.countSketch.add(item.keyh)

		if item.listid == 0 {
			t.lru.get(e)
		} else {
			t.slru.get(e)
		}
		return
	}

	newItem.keyh = xxhash.Sum64String(newItem.Key)

	oldItem, evicted := t.lru.add(newItem)
	if !evicted {
		return
	}

	// estimate count of what will be evicted from slru
	victim := t.slru.victim()
	if victim == nil {
		t.slru.add(oldItem)
		return
	}

	if !t.bouncer.allow(oldItem.keyh) {
		t.onEvict(oldItem)
		return
	}

	victimCount := t.countSketch.estimate(victim.keyh)
	itemCount := t.countSketch.estimate(oldItem.keyh)

	if itemCount > victimCount {
		t.slru.add(oldItem)
	} else {
		t.onEvict(oldItem)
	}
}

// Del removes an item from the cache by key.
// If the item exists, its OnEvict callback (if set) will be called.
func (t *T) Del(key string) {
	if val, ok := t.data[key]; ok {
		t.del(val)
	}
}

// del is the internal delete implementation that handles list removal and callbacks.
func (t *T) del(val *list.Element) {
	item := val.Value.(*Item)
	delete(t.data, item.Key)

	if item.listid == 0 {
		t.lru.Remove(val)
	} else {
		t.slru.Remove(val)
	}

	t.onEvict(item)
}
