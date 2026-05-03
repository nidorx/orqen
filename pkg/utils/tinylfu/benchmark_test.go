package tinylfu

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// BenchmarkCacheGet measures single Get performance on a hot key.
func BenchmarkCacheGet(b *testing.B) {
	cache := New(1000, 10000)
	cache.Set(&Item{Key: "hot_key", Value: "hot_value"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get("hot_key")
	}
}

// BenchmarkCacheGetMiss measures Get performance on non-existent keys.
func BenchmarkCacheGetMiss(b *testing.B) {
	cache := New(1000, 10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get(fmt.Sprintf("miss_%d", i))
	}
}

// BenchmarkCacheSet measures Set performance for new items.
func BenchmarkCacheSet(b *testing.B) {
	cache := New(1000, 10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(&Item{
			Key:   fmt.Sprintf("key_%d", i),
			Value: i,
		})
	}
}

// BenchmarkCacheSetUpdate measures Set performance for existing keys (updates).
func BenchmarkCacheSetUpdate(b *testing.B) {
	cache := New(1000, 10000)

	// Pre-populate
	for i := 0; i < 1000; i++ {
		cache.Set(&Item{
			Key:   fmt.Sprintf("key_%d", i),
			Value: i,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(&Item{
			Key:   fmt.Sprintf("key_%d", i%1000),
			Value: i,
		})
	}
}

// BenchmarkCacheDel measures Del performance.
func BenchmarkCacheDel(b *testing.B) {
	cache := New(1000, 10000)

	// Pre-populate
	for i := 0; i < b.N; i++ {
		cache.Set(&Item{
			Key:   fmt.Sprintf("key_%d", i),
			Value: i,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Del(fmt.Sprintf("key_%d", i))
	}
}

// BenchmarkCacheMixedRW measures performance with mixed reads and writes.
func BenchmarkCacheMixedRW(b *testing.B) {
	cache := New(1000, 10000)

	// Pre-populate
	for i := 0; i < 1000; i++ {
		cache.Set(&Item{
			Key:   fmt.Sprintf("key_%d", i),
			Value: i,
		})
	}

	rng := rand.New(rand.NewSource(42))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if rng.Intn(2) == 0 {
			// Read
			_, _ = cache.Get(fmt.Sprintf("key_%d", rng.Intn(1000)))
		} else {
			// Write
			cache.Set(&Item{
				Key:   fmt.Sprintf("key_%d", rng.Intn(1000)),
				Value: i,
			})
		}
	}
}

// BenchmarkCacheZipfian simulates a realistic zipfian access pattern.
func BenchmarkCacheZipfian(b *testing.B) {
	const size = 10000
	cache := New(1000, 10000)

	// Create keys with zipfian-like weights
	keys := make([]string, 100)
	for i := range keys {
		keys[i] = fmt.Sprintf("key_%d", i)
	}

	// Pre-warm cache
	for _, k := range keys {
		cache.Set(&Item{Key: k, Value: "value"})
	}

	rng := rand.New(rand.NewSource(42))
	weights := make([]int, len(keys))
	for i := range weights {
		weights[i] = i + 1 // higher index = more frequent
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Weighted random selection
		sum := 0
		for _, w := range weights {
			sum += w
		}
		r := rng.Intn(sum)
		cumulative := 0
		for j, w := range weights {
			cumulative += w
			if r < cumulative {
				_, _ = cache.Get(keys[j])
				break
			}
		}
	}
}

// BenchmarkSyncCacheGet measures thread-safe Get performance.
func BenchmarkSyncCacheGet(b *testing.B) {
	cache := NewSync(1000, 10000)
	cache.Set(&Item{Key: "hot_key", Value: "hot_value"})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.Get("hot_key")
		}
	})
}

// BenchmarkSyncCacheSet measures thread-safe Set performance.
func BenchmarkSyncCacheSet(b *testing.B) {
	cache := NewSync(1000, 10000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Set(&Item{
				Key:   fmt.Sprintf("key_%d", i),
				Value: i,
			})
			i++
		}
	})
}

// BenchmarkSyncCacheMixedRW measures thread-safe mixed read/write performance.
func BenchmarkSyncCacheMixedRW(b *testing.B) {
	cache := NewSync(1000, 10000)

	// Pre-populate
	for i := 0; i < 1000; i++ {
		cache.Set(&Item{
			Key:   fmt.Sprintf("key_%d", i),
			Value: i,
		})
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		i := 0
		for pb.Next() {
			if rng.Intn(2) == 0 {
				keyIdx := rng.Intn(1000)
				_, _ = cache.Get(fmt.Sprintf("key_%d", keyIdx))
			} else {
				cache.Set(&Item{
					Key:   fmt.Sprintf("key_%d", i%1000),
					Value: i,
				})
				i++
			}
		}
	})
}

// BenchmarkCacheGetWithExpiration measures Get performance with expired items.
func BenchmarkCacheGetWithExpiration(b *testing.B) {
	cache := New(1000, 10000)

	// Mix of expired and non-expired items
	for i := 0; i < 500; i++ {
		cache.Set(&Item{
			Key:      fmt.Sprintf("expired_%d", i),
			Value:    i,
			ExpireAt: time.Now().Add(-time.Second), // already expired
		})
	}
	for i := 0; i < 500; i++ {
		cache.Set(&Item{
			Key:      fmt.Sprintf("valid_%d", i),
			Value:    i,
			ExpireAt: time.Now().Add(time.Hour), // not expired
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			_, _ = cache.Get(fmt.Sprintf("expired_%d", i%500))
		} else {
			_, _ = cache.Get(fmt.Sprintf("valid_%d", i%500))
		}
	}
}

// BenchmarkCacheSizeComparison compares performance across different cache sizes.
func BenchmarkCacheSizeComparison(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			cache := New(size, size*10)

			// Pre-populate
			for i := 0; i < size; i++ {
				cache.Set(&Item{
					Key:   fmt.Sprintf("key_%d", i),
					Value: i,
				})
			}

			rng := rand.New(rand.NewSource(42))

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = cache.Get(fmt.Sprintf("key_%d", rng.Intn(size)))
			}
		})
	}
}

//------------------------------------------------------------------------------

// BenchmarkCacheTGet measures generic cache Get performance.
func BenchmarkCacheTGet(b *testing.B) {
	cache := NewCacheT[string](1000, 10000, time.Hour)
	cache.Set("hot_key", "hot_value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get("hot_key")
	}
}

// BenchmarkCacheTSet measures generic cache Set performance.
func BenchmarkCacheTSet(b *testing.B) {
	cache := NewCacheT[int](1000, 10000, time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(fmt.Sprintf("key_%d", i), i)
	}
}

// BenchmarkCacheTGetOrSetHit measures GetOrSet when key exists.
func BenchmarkCacheTGetOrSetHit(b *testing.B) {
	cache := NewCacheT[string](1000, 10000, time.Hour)
	cache.Set("key", "value")

	fn := func() (string, error) {
		return "should_not_be_called", nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.GetOrSet("key", fn)
	}
}

// BenchmarkCacheTGetOrSetMiss measures GetOrSet when key doesn't exist.
func BenchmarkCacheTGetOrSetMiss(b *testing.B) {
	cache := NewCacheT[string](1000, 10000, time.Hour)
	i := 0

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		key := fmt.Sprintf("key_%d", i)
		_, _ = cache.GetOrSet(key, func() (string, error) {
			return "computed", nil
		})
		i++
	}
}

// BenchmarkSyncCacheTGet measures thread-safe generic cache Get performance.
func BenchmarkSyncCacheTGet(b *testing.B) {
	cache := NewSyncCacheT[string](1000, 10000, time.Hour)
	cache.Set("hot_key", "hot_value")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.Get("hot_key")
		}
	})
}

// BenchmarkSyncCacheTSet measures thread-safe generic cache Set performance.
func BenchmarkSyncCacheTSet(b *testing.B) {
	cache := NewSyncCacheT[int](1000, 10000, time.Hour)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Set(fmt.Sprintf("key_%d", i), i)
			i++
		}
	})
}

// BenchmarkSyncCacheTGetOrSetHit measures GetOrSet hit performance under contention.
func BenchmarkSyncCacheTGetOrSetHit(b *testing.B) {
	cache := NewSyncCacheT[string](1000, 10000, time.Hour)
	cache.Set("key", "value")

	fn := func() (string, error) {
		return "should_not_be_called", nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.GetOrSet("key", fn)
		}
	})
}

// BenchmarkSyncCacheTGetOrSetMiss measures GetOrSet miss performance under contention.
func BenchmarkSyncCacheTGetOrSetMiss(b *testing.B) {
	cache := NewSyncCacheT[int](1000, 10000, time.Hour)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key_%d", i)
			_, _ = cache.GetOrSet(key, func() (int, error) {
				return i, nil
			})
			i++
		}
	})
}

// BenchmarkSyncCacheTGetOrSetConcurrent measures GetOrSet with concurrent goroutines hitting same key.
func BenchmarkSyncCacheTGetOrSetConcurrent(b *testing.B) {
	cache := NewSyncCacheT[int](1000, 10000, time.Hour)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.GetOrSet("shared_key", func() (int, error) {
				time.Sleep(time.Millisecond) // Simulate slow operation
				return 42, nil
			})
		}
	})
}
