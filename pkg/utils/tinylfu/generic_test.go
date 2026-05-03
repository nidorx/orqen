package tinylfu

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestCacheTBasic verifies basic Get/Set operations for generic cache.
func TestCacheTBasic(t *testing.T) {
	c := NewCacheT[string](100, 1000, time.Hour)

	c.Set("key1", "value1")

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("cache.Get(\"key1\") returned ok=false, want true")
	}
	if val != "value1" {
		t.Errorf("cache.Get(\"key1\") = %v, want %q", val, "value1")
	}
}

// TestCacheTWithType verifies generic cache works with custom types.
func TestCacheTWithType(t *testing.T) {
	type User struct {
		ID   int
		Name string
	}

	c := NewCacheT[User](100, 1000, time.Hour)

	c.Set("user:1", User{ID: 1, Name: "Alice"})

	user, ok := c.Get("user:1")
	if !ok {
		t.Fatal("cache.Get(\"user:1\") returned ok=false, want true")
	}
	if user.Name != "Alice" {
		t.Errorf("cache.Get(\"user:1\").Name = %v, want %q", user.Name, "Alice")
	}
}

// TestCacheTWithPointer verifies generic cache works with pointer types.
func TestCacheTWithPointer(t *testing.T) {
	type Data struct {
		Value int
	}

	c := NewCacheT[*Data](100, 1000, time.Hour)

	data := &Data{Value: 42}
	c.Set("data:1", data)

	retrieved, ok := c.Get("data:1")
	if !ok {
		t.Fatal("cache.Get(\"data:1\") returned ok=false, want true")
	}
	if retrieved.Value != 42 {
		t.Errorf("cache.Get(\"data:1\").Value = %v, want 42", retrieved.Value)
	}
}

// TestCacheTGetOrSet verifies GetOrSet functionality.
func TestCacheTGetOrSet(t *testing.T) {
	c := NewCacheT[string](100, 1000, time.Hour)

	callCount := 0
	fn := func() (string, error) {
		callCount++
		return "computed_value", nil
	}

	// First call should execute the function
	val, err := c.GetOrSet("key1", fn)
	if err != nil {
		t.Fatalf("GetOrSet returned error: %v", err)
	}
	if val != "computed_value" {
		t.Errorf("GetOrSet = %v, want %q", val, "computed_value")
	}
	if callCount != 1 {
		t.Errorf("fn called %d times, want 1", callCount)
	}

	// Second call should use cached value
	val, err = c.GetOrSet("key1", fn)
	if err != nil {
		t.Fatalf("GetOrSet returned error: %v", err)
	}
	if val != "computed_value" {
		t.Errorf("GetOrSet = %v, want %q", val, "computed_value")
	}
	if callCount != 1 {
		t.Errorf("fn called %d times after cache hit, want 1", callCount)
	}
}

// TestCacheTGetOrSetError verifies GetOrSet propagates errors.
func TestCacheTGetOrSetError(t *testing.T) {
	c := NewCacheT[string](100, 1000, time.Hour)

	fn := func() (string, error) {
		return "", fmt.Errorf("database error")
	}

	_, err := c.GetOrSet("key1", fn)
	if err == nil {
		t.Fatal("GetOrSet returned nil error, want error")
	}
	if err.Error() != "database error" {
		t.Errorf("GetOrSet error = %v, want %q", err, "database error")
	}

	// Key should not be in cache after error
	_, ok := c.Get("key1")
	if ok {
		t.Error("key should not be in cache after GetOrSet error")
	}
}

// TestCacheTTTL verifies TTL expiration.
func TestCacheTTTL(t *testing.T) {
	c := NewCacheT[string](100, 1000, 50*time.Millisecond)

	c.Set("expiring", "value")

	// Should be retrievable immediately
	val, ok := c.Get("expiring")
	if !ok {
		t.Fatal("cache.Get(\"expiring\") returned ok=false, want true")
	}
	if val != "value" {
		t.Errorf("cache.Get(\"expiring\") = %v, want %q", val, "value")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	_, ok = c.Get("expiring")
	if ok {
		t.Error("cache.Get(\"expiring\") returned ok=true after expiry, want false")
	}
}

// TestCacheTRandomizedTTL verifies UseRandomizedTTL works.
func TestCacheTRandomizedTTL(t *testing.T) {
	c := NewCacheT[string](100, 1000, time.Second)

	// Disable randomization
	c.UseRandomizedTTL(0)

	c.Set("key1", "value1")

	// Should still work
	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("cache.Get(\"key1\") returned ok=false, want true")
	}
	if val != "value1" {
		t.Errorf("cache.Get(\"key1\") = %v, want %q", val, "value1")
	}
}

// TestCacheTSetWithTTL verifies custom TTL.
func TestCacheTSetWithTTL(t *testing.T) {
	c := NewCacheT[string](100, 1000, time.Hour)

	// Set with short TTL and disable randomization to test exact expiry
	c.UseRandomizedTTL(0)
	c.SetWithTTL("short", "value", 50*time.Millisecond)

	val, ok := c.Get("short")
	if !ok {
		t.Fatal("cache.Get(\"short\") returned ok=false, want true")
	}
	if val != "value" {
		t.Errorf("cache.Get(\"short\") = %v, want %q", val, "value")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	_, ok = c.Get("short")
	if ok {
		t.Error("cache.Get(\"short\") returned ok=true after expiry, want false")
	}

	// Re-enable randomization for default TTL test
	c.UseRandomizedTTL(time.Second)
	c.SetWithTTL("default", "value2", 0)
	val, ok = c.Get("default")
	if !ok {
		t.Fatal("cache.Get(\"default\") returned ok=false, want true")
	}
	if val != "value2" {
		t.Errorf("cache.Get(\"default\") = %v, want %q", val, "value2")
	}
}

// TestCacheTDel verifies Del operation.
func TestCacheTDel(t *testing.T) {
	c := NewCacheT[string](100, 1000, time.Hour)

	c.Set("key1", "value1")
	c.Del("key1")

	_, ok := c.Get("key1")
	if ok {
		t.Error("cache.Get(\"key1\") returned ok=true after Del, want false")
	}
}

// TestCacheTEmptyCache verifies empty cache behavior.
func TestCacheTEmptyCache(t *testing.T) {
	c := NewCacheT[int](1, 100, time.Hour)

	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("cache.Get on empty cache returned ok=true, want false")
	}
}

// TestSyncCacheTBasic verifies basic operations for thread-safe generic cache.
func TestSyncCacheTBasic(t *testing.T) {
	c := NewSyncCacheT[string](100, 1000, time.Hour)

	c.Set("key1", "value1")

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("cache.Get(\"key1\") returned ok=false, want true")
	}
	if val != "value1" {
		t.Errorf("cache.Get(\"key1\") = %v, want %q", val, "value1")
	}
}

// TestSyncCacheTGetOrSet verifies GetOrSet with double-check locking.
func TestSyncCacheTGetOrSet(t *testing.T) {
	c := NewSyncCacheT[string](100, 1000, time.Hour)

	callCount := 0
	fn := func() (string, error) {
		callCount++
		return "computed_value", nil
	}

	// First call should execute the function
	val, err := c.GetOrSet("key1", fn)
	if err != nil {
		t.Fatalf("GetOrSet returned error: %v", err)
	}
	if val != "computed_value" {
		t.Errorf("GetOrSet = %v, want %q", val, "computed_value")
	}
	if callCount != 1 {
		t.Errorf("fn called %d times, want 1", callCount)
	}

	// Second call should use cached value
	val, err = c.GetOrSet("key1", fn)
	if err != nil {
		t.Fatalf("GetOrSet returned error: %v", err)
	}
	if val != "computed_value" {
		t.Errorf("GetOrSet = %v, want %q", val, "computed_value")
	}
	if callCount != 1 {
		t.Errorf("fn called %d times after cache hit, want 1", callCount)
	}
}

// TestSyncCacheTGetOrSetConcurrent verifies GetOrSet under concurrent access.
func TestSyncCacheTGetOrSetConcurrent(t *testing.T) {
	c := NewSyncCacheT[int](1000, 10000, time.Hour)

	callCount := 0
	var mu sync.Mutex

	fn := func() (int, error) {
		mu.Lock()
		callCount++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // Simulate slow operation
		return 42, nil
	}

	const goroutines = 10
	done := make(chan bool, goroutines)

	// Multiple goroutines trying to GetOrSet the same key
	for i := 0; i < goroutines; i++ {
		go func() {
			val, err := c.GetOrSet("shared_key", fn)
			if err != nil {
				t.Errorf("GetOrSet returned error: %v", err)
			}
			if val != 42 {
				t.Errorf("GetOrSet = %v, want 42", val)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < goroutines; i++ {
		<-done
	}

	// Function should have been called only once (or very few times due to race)
	mu.Lock()
	count := callCount
	mu.Unlock()

	if count > 2 {
		t.Errorf("fn called %d times under contention, expected 1-2", count)
	}
}

// TestSyncCacheTConcurrentAccess stress tests concurrent operations.
func TestSyncCacheTConcurrentAccess(t *testing.T) {
	c := NewSyncCacheT[int](1000, 10000, time.Hour)

	const goroutines = 20
	const iterations = 100

	done := make(chan bool, goroutines)

	for g := 0; g < goroutines; g++ {
		go func(gID int) {
			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf("key_%d_%d", gID, i)
				c.Set(key, i)
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

// TestSyncCacheTTTL verifies TTL in thread-safe cache.
func TestSyncCacheTTTL(t *testing.T) {
	c := NewSyncCacheT[string](100, 1000, 50*time.Millisecond)

	c.Set("expiring", "value")

	// Should be retrievable immediately
	val, ok := c.Get("expiring")
	if !ok {
		t.Fatal("cache.Get(\"expiring\") returned ok=false, want true")
	}
	if val != "value" {
		t.Errorf("cache.Get(\"expiring\") = %v, want %q", val, "value")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	_, ok = c.Get("expiring")
	if ok {
		t.Error("cache.Get(\"expiring\") returned ok=true after expiry, want false")
	}
}

// TestSyncCacheTSetWithTTL verifies custom TTL in thread-safe cache.
func TestSyncCacheTSetWithTTL(t *testing.T) {
	c := NewSyncCacheT[string](100, 1000, time.Hour)

	// Disable randomization to test exact TTL
	c.UseRandomizedTTL(0)
	c.SetWithTTL("short", "value", 50*time.Millisecond)

	val, ok := c.Get("short")
	if !ok {
		t.Fatal("cache.Get(\"short\") returned ok=false, want true")
	}
	if val != "value" {
		t.Errorf("cache.Get(\"short\") = %v, want %q", val, "value")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	_, ok = c.Get("short")
	if ok {
		t.Error("cache.Get(\"short\") returned ok=true after expiry, want false")
	}
}

// TestCacheTWithNilValue verifies nil values work correctly.
func TestCacheTWithNilValue(t *testing.T) {
	type Data struct {
		Value int
	}

	c := NewCacheT[*Data](100, 1000, time.Hour)

	c.Set("nil_key", nil)

	val, ok := c.Get("nil_key")
	if !ok {
		t.Fatal("cache.Get(\"nil_key\") returned ok=false, want true")
	}
	if val != nil {
		t.Errorf("cache.Get(\"nil_key\") = %v, want nil", val)
	}
}

// TestCacheTGetOrSetWithNilErrorResult verifies GetOrSet handles nil returns.
func TestCacheTGetOrSetWithNilResult(t *testing.T) {
	c := NewCacheT[*int](100, 1000, time.Hour)

	fn := func() (*int, error) {
		return nil, nil
	}

	val, err := c.GetOrSet("nil_value", fn)
	if err != nil {
		t.Fatalf("GetOrSet returned error: %v", err)
	}
	if val != nil {
		t.Errorf("GetOrSet = %v, want nil", val)
	}
}

// TestCacheTEviction verifies items are evicted under pressure.
func TestCacheTEviction(t *testing.T) {
	const size = 10
	c := NewCacheT[string](size, 1000, time.Hour)

	// Fill cache beyond capacity
	for i := 0; i < size*2; i++ {
		c.Set(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
	}

	// Count how many items still exist
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

// TestSyncCacheTEviction verifies eviction in thread-safe cache.
func TestSyncCacheTEviction(t *testing.T) {
	const size = 10
	c := NewSyncCacheT[string](size, 1000, time.Hour)

	for i := 0; i < size*2; i++ {
		c.Set(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
	}

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
