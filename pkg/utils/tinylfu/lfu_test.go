package tinylfu

import (
	"context"
	cryptorand "crypto/rand"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"time"
)

func TestCache(t *testing.T) {
	cache := New(1e3, 10e3)
	keys := []string{"one", "two", "three"}

	for _, key := range keys {
		cache.Set(&Item{
			Key:   key,
			Value: key,
		})

		got, ok := cache.Get(key)
		if !ok {
			t.Fatalf("cache.Get(%q) returned ok=false, want true", key)
		}
		if got != key {
			t.Errorf("cache.Get(%q) = %v, want %v", key, got, key)
		}
	}

	for _, key := range keys {
		got, ok := cache.Get(key)
		if !ok {
			t.Fatalf("cache.Get(%q) returned ok=false, want true", key)
		}
		if got != key {
			t.Errorf("cache.Get(%q) = %v, want %v", key, got, key)
		}

		cache.Set(&Item{
			Key:   key,
			Value: key + key,
		})
	}

	for _, key := range keys {
		got, ok := cache.Get(key)
		if !ok {
			t.Fatalf("cache.Get(%q) returned ok=false, want true", key)
		}
		if got != key+key {
			t.Errorf("cache.Get(%q) = %v, want %v", key, got, key+key)
		}
	}

	for _, key := range keys {
		cache.Del(key)
	}

	for _, key := range keys {
		_, ok := cache.Get(key)
		if ok {
			t.Errorf("cache.Get(%q) returned ok=true after Del, want false", key)
		}
	}
}

func TestOOM(t *testing.T) {
	keys := make([]string, 10000)
	for i := range keys {
		keys[i] = randWord()
	}

	cache := New(1e3, 10e3)

	for i := 0; i < 5e6; i++ {
		key := keys[i%len(keys)]
		cache.Set(&Item{
			Key:   key,
			Value: key,
		})
	}
}

func TestCorruptionOnExpiry(t *testing.T) {
	const size = 50000

	strFor := func(i int) string {
		return fmt.Sprintf("a string %d", i)
	}
	keyName := func(i int) string {
		return fmt.Sprintf("key-%00000d", i)
	}

	mycache := New(1000, 10000)
	// Put a bunch of stuff in the cache with a TTL of 1 second
	for i := 0; i < size; i++ {
		key := keyName(i)
		mycache.Set(&Item{
			Key:      key,
			Value:    []byte(strFor(i)),
			ExpireAt: time.Now().Add(time.Second),
		})
	}

	// Read stuff for a bit longer than the TTL - that's when the corruption occurs
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := ctx.Done()
loop:
	for {
		select {
		case <-done:
			// this is expected
			break loop
		default:
			i := rand.Intn(size)
			key := keyName(i)

			b, ok := mycache.Get(key)
			if !ok {
				continue loop
			}

			got := string(b.([]byte))
			expected := strFor(i)
			if got != expected {
				t.Fatalf("expected=%q got=%q key=%q", expected, got, key)
			}
		}
	}
}

func randWord() string {
	buf := make([]byte, 64)
	io.ReadFull(cryptorand.Reader, buf)
	return string(buf)
}

func TestAddAlreadyInCache(t *testing.T) {
	c := New(100, 10000)

	c.Set(&Item{
		Key:   "foo",
		Value: "bar",
	})

	val, ok := c.Get("foo")
	if !ok {
		t.Fatal("cache.Get(\"foo\") returned ok=false, want true")
	}
	if val.(string) != "bar" {
		t.Errorf("c.Get(foo)=%q, want %q", val, "bar")
	}

	c.Set(&Item{
		Key:   "foo",
		Value: "baz",
	})

	val, ok = c.Get("foo")
	if !ok {
		t.Fatal("cache.Get(\"foo\") returned ok=false, want true")
	}
	if val.(string) != "baz" {
		t.Errorf("c.Get(foo)=%q, want %q", val, "baz")
	}
}

func TestCacheExpiration(t *testing.T) {
	c := New(100, 10000)

	// Set item with short TTL
	c.Set(&Item{
		Key:      "expiring",
		Value:    "value",
		ExpireAt: time.Now().Add(50 * time.Millisecond),
	})

	// Should be retrievable immediately
	val, ok := c.Get("expiring")
	if !ok {
		t.Fatal("cache.Get(\"expiring\") returned ok=false, want true")
	}
	if val.(string) != "value" {
		t.Errorf("c.Get(expiring)=%v, want %q", val, "value")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	_, ok = c.Get("expiring")
	if ok {
		t.Error("cache.Get(\"expiring\") returned ok=true after expiry, want false")
	}
}

func TestOnEvictCallback(t *testing.T) {
	c := New(2, 100)

	evicted := make(map[string]bool)

	c.Set(&Item{
		Key:   "evict_me",
		Value: "value1",
		OnEvict: func() {
			evicted["evict_me"] = true
		},
	})

	c.Set(&Item{
		Key:   "stay",
		Value: "value2",
	})

	// Manually delete should trigger OnEvict
	c.Del("evict_me")
	if !evicted["evict_me"] {
		t.Error("OnEvict not called after Del")
	}
}

func TestCacheOverwrite(t *testing.T) {
	c := New(100, 10000)

	c.Set(&Item{
		Key:   "key1",
		Value: "initial",
	})

	// Overwrite should update the value
	c.Set(&Item{
		Key:   "key1",
		Value: "updated",
	})

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("cache.Get(\"key1\") returned ok=false, want true")
	}
	if val.(string) != "updated" {
		t.Errorf("c.Get(key1)=%v, want %q", val, "updated")
	}
}

func TestEmptyCache(t *testing.T) {
	c := New(1, 100)

	_, ok := c.Get("nonexistent")
	if ok {
		t.Error("cache.Get on empty cache returned ok=true, want false")
	}

	c.Del("nonexistent") // Should not panic
}

func TestSyncCache(t *testing.T) {
	c := NewSync(100, 10000)

	c.Set(&Item{
		Key:   "sync_key",
		Value: "sync_value",
	})

	val, ok := c.Get("sync_key")
	if !ok {
		t.Fatal("syncCache.Get(\"sync_key\") returned ok=false, want true")
	}
	if val.(string) != "sync_value" {
		t.Errorf("c.Get(sync_key)=%v, want %q", val, "sync_value")
	}

	c.Del("sync_key")

	_, ok = c.Get("sync_key")
	if ok {
		t.Error("syncCache.Get(\"sync_key\") returned ok=true after Del, want false")
	}
}

func TestSyncCacheConcurrentAccess(t *testing.T) {
	c := NewSync(1000, 10000)

	done := make(chan bool)

	// Run multiple goroutines concurrently
	for i := 0; i < 10; i++ {
		go func(n int) {
			key := fmt.Sprintf("key-%d", n)
			c.Set(&Item{
				Key:   key,
				Value: fmt.Sprintf("value-%d", n),
			})
			_, _ = c.Get(key)
			c.Del(key)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
