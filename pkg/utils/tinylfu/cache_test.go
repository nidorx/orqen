package tinylfu

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestCacheEviction verifies that items are properly evicted when cache is full.
func TestCacheEviction(t *testing.T) {
	const size = 10
	c := New(size, 1000)

	// Fill cache beyond capacity
	for i := 0; i < size*2; i++ {
		c.Set(&Item{
			Key:   fmt.Sprintf("key_%d", i),
			Value: i,
		})
	}

	// Some early items should have been evicted
	// (we can't guarantee exactly which ones, but cache size should be bounded)
	count := 0
	for i := 0; i < size*2; i++ {
		if _, ok := c.Get(fmt.Sprintf("key_%d", i)); ok {
			count++
		}
	}

	if count > size {
		t.Errorf("cache has %d items, expected <= %d", count, size)
	}
}

// TestOnEvictCallbackSet verifies OnEvict is called when item is evicted by admission policy.
func TestOnEvictCallbackSet(t *testing.T) {
	c := New(5, 100)

	evictCount := 0
	for i := 0; i < 20; i++ {
		c.Set(&Item{
			Key:   fmt.Sprintf("key_%d", i),
			Value: i,
			OnEvict: func() {
				evictCount++
			},
		})
	}

	if evictCount == 0 {
		t.Error("OnEvict was never called during cache pressure")
	}
}

// TestSetAndGetWithNilValue verifies that nil values are handled correctly.
func TestSetAndGetWithNilValue(t *testing.T) {
	c := New(100, 1000)

	c.Set(&Item{
		Key:   "nil_key",
		Value: nil,
	})

	val, ok := c.Get("nil_key")
	if !ok {
		t.Error("cache.Get(\"nil_key\") returned ok=false, want true")
	}
	if val != nil {
		t.Errorf("cache.Get(\"nil_key\") = %v, want nil", val)
	}
}

// TestMultipleExpirations verifies multiple expired items are cleaned up.
func TestMultipleExpirations(t *testing.T) {
	c := New(100, 1000)

	// Set many items with short TTL
	for i := 0; i < 50; i++ {
		c.Set(&Item{
			Key:      fmt.Sprintf("exp_%d", i),
			Value:    i,
			ExpireAt: time.Now().Add(50 * time.Millisecond),
		})
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// All should be expired
	for i := 0; i < 50; i++ {
		_, ok := c.Get(fmt.Sprintf("exp_%d", i))
		if ok {
			t.Errorf("item exp_%d should be expired but was found", i)
		}
	}
}

// TestSLRUPromotion verifies items move from probationary to protected segment.
func TestSLRUPromotion(t *testing.T) {
	c := New(100, 1000)

	// Add an item
	c.Set(&Item{
		Key:   "promote_me",
		Value: "value",
	})

	// Access it multiple times to promote
	for i := 0; i < 10; i++ {
		_, ok := c.Get("promote_me")
		if !ok {
			t.Fatal("item should still be in cache")
		}
	}

	// Item should still be accessible
	_, ok := c.Get("promote_me")
	if !ok {
		t.Error("item was evicted but should have been protected by frequent access")
	}
}

// TestCacheStressConcurrency stress tests the non-sync cache with external locking.
func TestCacheStressConcurrency(t *testing.T) {
	c := New(1000, 10000)
	var mu sync.Mutex

	const goroutines = 20
	const iterations = 100

	done := make(chan bool, goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gID int) {
			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf("key_%d_%d", gID, i)

				mu.Lock()
				c.Set(&Item{
					Key:   key,
					Value: i,
				})
				_, _ = c.Get(key)
				c.Del(key)
				mu.Unlock()
			}
			done <- true
		}(g)
	}

	for g := 0; g < goroutines; g++ {
		<-done
	}
}

// TestSyncCacheStressConcurrency stress tests the sync cache.
func TestSyncCacheStressConcurrency(t *testing.T) {
	c := NewSync(1000, 10000)

	const goroutines = 20
	const iterations = 100

	done := make(chan bool, goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gID int) {
			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf("key_%d_%d", gID, i)
				c.Set(&Item{
					Key:   key,
					Value: i,
				})
				_, _ = c.Get(key)
				c.Del(key)
			}
			done <- true
		}(g)
	}

	for g := 0; g < goroutines; g++ {
		<-done
	}
}

// TestCacheUpdateExistingKey verifies that Set on existing key doesn't cause eviction.
func TestCacheUpdateExistingKey(t *testing.T) {
	c := New(5, 100)

	// Fill cache to capacity
	for i := 0; i < 5; i++ {
		c.Set(&Item{
			Key:   fmt.Sprintf("key_%d", i),
			Value: i,
		})
	}

	// Update an existing key (should not evict)
	c.Set(&Item{
		Key:   "key_0",
		Value: "updated",
	})

	// All keys should still be present
	for i := 0; i < 5; i++ {
		_, ok := c.Get(fmt.Sprintf("key_%d", i))
		if !ok {
			t.Errorf("key_%d should exist after update, but was missing", i)
		}
	}
}

// TestEmptyKey verifies empty keys are handled correctly.
func TestEmptyKey(t *testing.T) {
	c := New(100, 1000)

	c.Set(&Item{
		Key:   "",
		Value: "empty",
	})

	val, ok := c.Get("")
	if !ok {
		t.Error("cache.Get(\"\") returned ok=false, want true")
	}
	if val.(string) != "empty" {
		t.Errorf("cache.Get(\"\") = %v, want %q", val, "empty")
	}
}

// TestSpecialCharactersInKey verifies keys with special characters work.
func TestSpecialCharactersInKey(t *testing.T) {
	c := New(100, 1000)

	specialKeys := []string{
		"key with spaces",
		"key\twith\ttabs",
		"key/with/slashes",
		"key?with#special$chars",
		"日本語",
		"",
	}

	for _, key := range specialKeys {
		c.Set(&Item{
			Key:   key,
			Value: "value_for_" + key,
		})

		val, ok := c.Get(key)
		if !ok {
			t.Errorf("cache.Get(%q) returned ok=false, want true", key)
		}
		expected := "value_for_" + key
		if val.(string) != expected {
			t.Errorf("cache.Get(%q) = %v, want %q", key, val, expected)
		}
	}
}

// TestCM4OddWidth verifies cm4 works correctly with odd widths.
func TestCM4OddWidth(t *testing.T) {
	cm := newCM4(33) // odd number

	hash := uint64(0xdeadbeef)
	cm.add(hash)

	if got := cm.estimate(hash); got == 0 {
		t.Error("cm.estimate should be > 0 after add")
	}
}

// TestNvecOddIndex verifies nvec works with odd indices.
func TestNvecOddIndex(t *testing.T) {
	n := newNvec(5) // odd size

	// Test odd index
	n.inc(3)
	if got := n.get(3); got != 1 {
		t.Errorf("n.get(3) = %d, want 1", got)
	}

	// Test even index
	n.inc(2)
	if got := n.get(2); got != 1 {
		t.Errorf("n.get(2) = %d, want 1", got)
	}
}

// TestDoorkeeperNilSafety verifies doorkeeper handles nil gracefully.
func TestDoorkeeperNilSafety(t *testing.T) {
	var d *doorkeeper

	// Should not panic and should return true
	if !d.allow(12345) {
		t.Error("nil doorkeeper.allow() should return true")
	}

	// Reset should not panic
	d.reset() // should be safe
}

// TestCacheResetCycle verifies that count sketch and doorkeeper reset properly.
func TestCacheResetCycle(t *testing.T) {
	samples := 100
	c := New(10, samples)

	// Trigger enough operations to cause a reset
	for i := 0; i < samples*2; i++ {
		c.Set(&Item{
			Key:   fmt.Sprintf("key_%d", i),
			Value: i,
		})
	}

	// Cache should still be functional
	c.Set(&Item{
		Key:   "after_reset",
		Value: "works",
	})

	val, ok := c.Get("after_reset")
	if !ok {
		t.Error("cache should work after internal reset")
	}
	if val.(string) != "works" {
		t.Errorf("cache.Get(\"after_reset\") = %v, want %q", val, "works")
	}
}

// TestLFUInterface verifies that Cache implements the LFU interface.
func TestLFUInterface(t *testing.T) {
	var cache LFU = New(100, 1000)

	cache.Set(&Item{
		Key:   "interface_test",
		Value: "works",
	})

	val, ok := cache.Get("interface_test")
	if !ok {
		t.Error("LFU interface Get failed")
	}
	if val.(string) != "works" {
		t.Errorf("LFU interface Get = %v, want %q", val, "works")
	}

	cache.Del("interface_test")

	_, ok = cache.Get("interface_test")
	if ok {
		t.Error("LFU interface Del failed")
	}
}
