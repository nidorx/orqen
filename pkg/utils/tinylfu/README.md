# TinyLFU Cache for Go

[![Go Reference](https://pkg.go.dev/badge/github.com/nidorx/orqen/pkg/vendor/tinylfu.svg)](https://pkg.go.dev/github.com/nidorx/orqen/pkg/vendor/tinylfu)

A high-performance, memory-efficient cache implementation in Go using the **TinyLFU** admission policy.

## Overview

TinyLFU is a cache admission policy that uses historical access patterns to make intelligent decisions about which items to keep in the cache. It achieves near-optimal hit rates using significantly less memory than traditional LRU caches.

**Paper:** [TinyLFU: A Highly Efficient Cache Admission Policy](http://arxiv.org/abs/1512.00727)

## Features

- **High hit rate**: Near-optimal cache hit rates using frequency-based admission control
- **Memory efficient**: Uses a Count-Min Sketch with 4-bit counters for compact frequency tracking
- **Two-tier architecture**: Combines a small LRU (1%) with a Segmented LRU (99%)
- **Bloom filter admission**: Doorkeeper policy filters out one-hit wonders
- **Thread-safe option**: `SyncCache` for concurrent access, `Cache` for single-threaded
- **TTL support**: Per-item expiration with absolute timestamps
- **Eviction callbacks**: Optional `OnEvict` hook for cleanup logic
- **Zero dependencies**: Only requires `xxhash` for hashing

## Installation

This package is vendored within the orqen project. No additional installation required.

## Usage

### Basic Usage (Non-generic, any value type)

```go
import "github.com/nidorx/orqen/pkg/tinylfu"

// Create a cache with capacity 1000 and 10000 samples
cache := tinylfu.New(1000, 10000)

// Set a value
cache.Set(&tinylfu.Item{
    Key:   "user:123",
    Value: userData,
})

// Get a value (returns any, needs type assertion)
if val, ok := cache.Get("user:123"); ok {
    user := val.(*User) // type assertion
}

// Delete a value
cache.Del("user:123")
```

### Generic Usage (Type-safe, recommended)

```go
// Create a type-safe cache with TTL
cache := tinylfu.NewCacheT[string](1000, 10000, time.Hour)

// Set a value (no Item struct needed)
cache.Set("user:123", "userData")

// Get a value (type-safe, no assertion needed)
if val, ok := cache.Get("user:123"); ok {
    // val is already string type
}

// GetOrSet pattern (very common in applications)
user, err := cache.GetOrSet("user:123", func() (string, error) {
    return db.GetUser(123)
})
```

### Thread-Safe Generic Usage (Concurrent access)

```go
// Create a thread-safe type-safe cache
cache := tinylfu.NewSyncCacheT[*model.Execution](1000, 100000, 30*time.Minute)

// GetOrSet with double-check locking (ideal for database lookups)
execution, err := cache.GetOrSet(id.String(), func() (*model.Execution, error) {
    return db.LoadExecution(id)
})
if err != nil {
    // handle error
}
// use execution
```

### With Expiration

```go
cache.Set(&tinylfu.Item{
    Key:      "temp:data",
    Value:    data,
    ExpireAt: time.Now().Add(5 * time.Minute),
})
```

### With Eviction Callback

```go
cache.Set(&tinylfu.Item{
    Key:   "resource:heavy",
    Value: heavyData,
    OnEvict: func() {
        // Cleanup resources when item is evicted
        heavyData.Close()
    },
})
```

## API Reference

### Non-Generic Types (Original API)

#### `Cache` (alias for `T`)

Non-thread-safe TinyLFU cache. Use when you have a single goroutine or external synchronization.

**Why use `Cache`**: Best performance when no concurrent access from multiple goroutines. Avoids mutex overhead.

```go
func New(size int, samples int) *Cache
func (c *Cache) Get(key string) (any, bool)
func (c *Cache) Set(item *Item)
func (c *Cache) Del(key string)
```

#### `SyncCache` (alias for `SyncT`)

Thread-safe wrapper around `Cache` with internal `sync.Mutex`. Use for concurrent access.

**Why use `SyncCache`**: Safe for multiple goroutines accessing the cache simultaneously.

```go
func NewSync(size int, samples int) *SyncCache
func (c *SyncCache) Get(key string) (any, bool)
func (c *SyncCache) Set(item *Item)
func (c *SyncCache) Del(key string)
```

### Generic Types (Recommended API)

#### `CacheT[V any]`

Generic TinyLFU cache with TTL support. Type-safe, no type assertions needed.

**Why use `CacheT`**: When all cached values have the same type and you want automatic TTL management with `GetOrSet` support.

```go
func NewCacheT[V any](size int, samples int, ttl time.Duration) *CacheT[V]
func (c *CacheT[V]) Get(key string) (V, bool)
func (c *CacheT[V]) Set(key string, value V)
func (c *CacheT[V]) SetWithTTL(key string, value V, ttl time.Duration)
func (c *CacheT[V]) Del(key string)
func (c *CacheT[V]) GetOrSet(key string, fn func() (V, error)) (V, error)
func (c *CacheT[V]) UseRandomizedTTL(offset time.Duration)
```

#### `SyncCacheT[V any]`

Thread-safe generic TinyLFU cache with TTL support and double-check locking in `GetOrSet`.

**Why use `SyncCacheT`**: When multiple goroutines need concurrent access and you want type safety with automatic TTL. Ideal for database-backed caches.

```go
func NewSyncCacheT[V any](size int, samples int, ttl time.Duration) *SyncCacheT[V]
func (c *SyncCacheT[V]) Get(key string) (V, bool)
func (c *SyncCacheT[V]) Set(key string, value V)
func (c *SyncCacheT[V]) SetWithTTL(key string, value V, ttl time.Duration)
func (c *SyncCacheT[V]) Del(key string)
func (c *SyncCacheT[V]) GetOrSet(key string, fn func() (V, error)) (V, error)
func (c *SyncCacheT[V]) UseRandomizedTTL(offset time.Duration)
```

### `Item`

Represents a cache entry (used by non-generic types).

```go
type Item struct {
    Key      string         // Unique identifier
    Value    any            // Cached data
    ExpireAt time.Time      // Absolute expiration time (zero = no expiry)
    OnEvict  func()         // Callback on eviction (can be nil)
}
```

### `LFU` Interface

Interface for cache implementations.

```go
type LFU interface {
    Get(key string) (any, bool)
    Set(newItem *Item)
    Del(key string)
}
```

## Architecture

### How TinyLFU Works

1. **Admission Window (LRU - 1%)**: New items enter here first
2. **Main Space (SLRU - 99%)**: Divided into:
   - **Probationary (20%)**: Items that survived the window
   - **Protected (80%)**: Frequently accessed items

3. **Count-Min Sketch**: Tracks access frequencies across a time window
4. **Doorkeeper**: Bloom filter that admits only items seen more than once

When the cache is full, a new item is admitted only if its estimated frequency exceeds the frequency of the victim (LRU item in the main space).

### Reset Cycle

Periodically (every `samples` operations), the frequency tracking is reset to adapt to changing access patterns. This prevents old access patterns from permanently influencing admission decisions.

## Performance

### Benchmarks

Run benchmarks with:

```bash
go test -bench=. -benchmem
```

Key benchmarks include:
- `BenchmarkCacheGet` - Single hot key retrieval
- `BenchmarkCacheGetMiss` - Non-existent key lookup
- `BenchmarkCacheSet` - New item insertion
- `BenchmarkCacheMixedRW` - Mixed read/write workload
- `BenchmarkCacheZipfian` - Realistic skewed access pattern
- `BenchmarkSyncCache*` - Thread-safe variants

## When to Use

| Scenario | Recommendation |
|----------|---------------|
| Single goroutine, any value type | `Cache` |
| Multiple goroutines, any value type | `SyncCache` |
| Single goroutine, same type, need TTL | `CacheT[V]` |
| Multiple goroutines, same type, need TTL | `SyncCacheT[V]` |
| Database-backed cache with GetOrSet | `SyncCacheT[V]` |
| Need precise eviction control | `Cache` with external lock |
| High write throughput | `Cache` or `CacheT[V]` |
| Mixed concurrent read/write | `SyncCache` or `SyncCacheT[V]` |

## Improvements over Original

This package is based on [vmihailenco/go-tinylfu](https://github.com/vmihailenco/go-tinylfu) with the following improvements:

1. **Bug Fixes**:
   - Fixed `newNvec` allocation for odd widths (was truncating)
   - Fixed `bitvector.getset` to use `uint64` instead of platform-dependent `uint`
   - Fixed `lruCache.Len()` to return actual list length instead of map length
   - Fixed race condition in `SyncCache` (changed from `RWMutex` to `Mutex` since Get has side effects)

2. **Generic Types (Go 1.18+)**:
   - Added `CacheT[V any]` - type-safe cache with automatic TTL management
   - Added `SyncCacheT[V any]` - thread-safe generic cache with double-check locking
   - Eliminated need for type assertions when working with specific value types
   - Built-in `GetOrSet` pattern ideal for database-backed caches

3. **TTL Management**:
   - Automatic TTL with randomized jitter to prevent cache stampedes
   - `UseRandomizedTTL()` to configure TTL offset
   - `SetWithTTL()` for per-item custom TTL
   - Default TTL set at cache creation time

4. **Documentation**:
   - Comprehensive godoc for all types, methods, and fields
   - Usage examples and architectural documentation in README
   - Clear guidance on when to use each cache type

5. **Test Coverage**:
   - Removed `testify` dependency (standard library assertions only)
   - Added tests for edge cases, concurrency, expiration, callbacks
   - Stress tests for concurrent access patterns
   - Additional unit tests for internal structures (nvec, cm4, doorkeeper)
   - Comprehensive tests for generic types and GetOrSet pattern

6. **Benchmarks**:
   - Comprehensive benchmark suite covering common workloads
   - Thread-safe vs non-thread-safe performance comparison
   - Generic vs non-generic performance comparison
   - GetOrSet hit/miss benchmarks
   - Various cache sizes and access patterns

7. **Type Naming**:
   - Added `Cache` and `SyncCache` type aliases for clearer code
   - Original `T` and `SyncT` names retained for backwards compatibility

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Attribution

Originally implemented by [Damian Gryski](https://github.com/dgryski/go-tinylfu), updated by [Mihailenco](https://github.com/vmihailenco/go-tinylfu)

Improved and maintained as part of the [orqen](https://github.com/nidorx/orqen) project.
