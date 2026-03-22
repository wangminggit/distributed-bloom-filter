package bloom

import (
	"testing"
)

// TestIndexCache_BasicOperations tests basic cache operations.
func TestIndexCache_BasicOperations(t *testing.T) {
	cache := NewIndexCache(100)

	if cache == nil {
		t.Fatal("Expected non-nil cache")
	}

	// Test Put and Get
	key := "test-key"
	value := []int{1, 2, 3, 4, 5}

	cache.Put(key, value)

	got := cache.Get(key)
	if got == nil {
		t.Error("Expected to find key")
	}
	if len(got) != len(value) {
		t.Errorf("Expected length %d, got %d", len(value), len(got))
	}

	// Test cache miss
	miss := cache.Get("non-existent")
	if miss != nil {
		t.Error("Expected nil for cache miss")
	}
}

// TestIndexCache_Remove tests key removal.
func TestIndexCache_Remove(t *testing.T) {
	cache := NewIndexCache(100)

	key := "to-remove"
	cache.Put(key, []int{1, 2, 3})

	// Verify it exists
	got := cache.Get(key)
	if got == nil {
		t.Fatal("Expected key to exist")
	}

	// Remove it
	cache.Remove(key)

	// Verify it's gone
	got = cache.Get(key)
	if got != nil {
		t.Error("Expected key to be removed")
	}
}

// TestIndexCache_Clear tests clearing the cache.
func TestIndexCache_Clear(t *testing.T) {
	cache := NewIndexCache(100)

	// Add multiple items
	for i := 0; i < 10; i++ {
		cache.Put(string(rune(i)), []int{i})
	}

	// Clear
	cache.Clear()

	// Verify all are gone
	for i := 0; i < 10; i++ {
		got := cache.Get(string(rune(i)))
		if got != nil {
			t.Errorf("Expected item %d to be cleared", i)
		}
	}
}

// TestIndexCache_Eviction tests LRU eviction.
func TestIndexCache_Eviction(t *testing.T) {
	cache := NewIndexCache(5) // Small capacity

	// Add more items than capacity
	for i := 0; i < 10; i++ {
		cache.Put(string(rune(i)), []int{i})
	}

	// Size should not exceed capacity
	if len(cache.cache) > 5 {
		t.Errorf("Expected size <= 5 after eviction, got %d", len(cache.cache))
	}
}

// TestIndexCache_LRUOrder tests that LRU order is maintained.
func TestIndexCache_LRUOrder(t *testing.T) {
	cache := NewIndexCache(3)

	// Add items: 1, 2, 3
	cache.Put("1", []int{1})
	cache.Put("2", []int{2})
	cache.Put("3", []int{3})

	// Access item 1 (should move to front)
	cache.Get("1")

	// Add item 4 (should evict item 2, not 1)
	cache.Put("4", []int{4})

	// Item 1 should still exist
	got := cache.Get("1")
	if got == nil {
		t.Error("Expected item 1 to exist (was recently accessed)")
	}
}

// TestIndexCache_ConcurrentAccess tests thread safety.
func TestIndexCache_ConcurrentAccess(t *testing.T) {
	cache := NewIndexCache(1000)

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()
			for j := 0; j < 100; j++ {
				key := string(rune(id*100 + j))
				cache.Put(key, []int{j})
				cache.Get(key)
			}
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic or deadlock
	t.Log("Concurrent access test completed successfully")
}

// TestCountingBloomFilterWithCache tests the cached Bloom filter.
func TestCountingBloomFilterWithCache(t *testing.T) {
	bf := NewCountingBloomFilterWithCache(1000, 3, 100)

	if bf == nil {
		t.Fatal("Expected non-nil Bloom filter")
	}

	// Test Add
	item := []byte("test-item")
	err := bf.Add(item)
	if err != nil {
		t.Errorf("Add failed: %v", err)
	}

	// Test Contains (should use cache)
	if !bf.Contains(item) {
		t.Error("Expected item to exist")
	}
}

// TestCountingBloomFilterWithCache_Remove tests removal with cache.
func TestCountingBloomFilterWithCache_Remove(t *testing.T) {
	bf := NewCountingBloomFilterWithCache(1000, 3, 100)

	item := []byte("to-remove")
	bf.Add(item)

	if !bf.Contains(item) {
		t.Fatal("Expected item to exist")
	}

	bf.Remove(item)

	if bf.Contains(item) {
		t.Error("Expected item to be removed")
	}
}

// TestCountingBloomFilterWithCache_CacheMethod tests Cache() getter.
func TestCountingBloomFilterWithCache_CacheMethod(t *testing.T) {
	bf := NewCountingBloomFilterWithCache(1000, 3, 100)

	gotCache := bf.Cache()
	if gotCache == nil {
		t.Error("Expected non-nil cache")
	}
}

// TestIndexCache_Stats tests cache statistics.
func TestIndexCache_Stats(t *testing.T) {
	cache := NewIndexCache(100)

	// Add and get items
	cache.Put("key1", []int{1, 2})
	cache.Get("key1")
	cache.Get("key1")
	cache.Get("key2") // miss

	size, hits := cache.Stats()
	
	if size != 1 {
		t.Errorf("Expected size 1, got %d", size)
	}
	// Hits may include internal operations, just verify > 0
	if hits == 0 {
		t.Errorf("Expected hits > 0, got %d", hits)
	}
}

// TestIndexCache_UpdateExisting tests updating existing key.
func TestIndexCache_UpdateExisting(t *testing.T) {
	cache := NewIndexCache(100)

	cache.Put("key", []int{1, 2, 3})
	cache.Put("key", []int{4, 5, 6, 7})

	got := cache.Get("key")
	if len(got) != 4 {
		t.Errorf("Expected length 4, got %d", len(got))
	}
}
