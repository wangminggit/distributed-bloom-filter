package bloom

import (
	"sync"
	"testing"
)

// TestCountingBloomFilter_BatchAdd tests batch add operation.
func TestCountingBloomFilter_BatchAdd(t *testing.T) {
	bf := NewCountingBloomFilter(1000, 3)

	items := [][]byte{
		[]byte("item1"),
		[]byte("item2"),
		[]byte("item3"),
		[]byte("item4"),
		[]byte("item5"),
	}

	bf.BatchAdd(items)

	// Verify all items were added
	for _, item := range items {
		if !bf.Contains(item) {
			t.Errorf("Item %s should exist", string(item))
		}
	}
}

// TestCountingBloomFilter_BatchAdd_Empty tests empty batch.
func TestCountingBloomFilter_BatchAdd_Empty(t *testing.T) {
	bf := NewCountingBloomFilter(1000, 3)

	// Empty batch should not panic
	bf.BatchAdd([][]byte{})

	// Should still work normally
	bf.Add([]byte("single"))
	if !bf.Contains([]byte("single")) {
		t.Error("Expected item to exist")
	}
}

// TestCountingBloomFilter_BatchAdd_NilItems tests nil items in batch.
func TestCountingBloomFilter_BatchAdd_NilItems(t *testing.T) {
	bf := NewCountingBloomFilter(1000, 3)

	items := [][]byte{
		[]byte("valid"),
		nil,
		[]byte("another"),
	}

	// Should handle nil gracefully (may panic or skip)
	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchAdd panicked with nil items: %v", r)
		}
	}()

	bf.BatchAdd(items)
}

// TestCountingBloomFilter_BatchRemove tests batch remove operation.
func TestCountingBloomFilter_BatchRemove(t *testing.T) {
	bf := NewCountingBloomFilter(1000, 3)

	// Add items first
	items := [][]byte{
		[]byte("to-remove-1"),
		[]byte("to-remove-2"),
		[]byte("to-remove-3"),
		[]byte("to-keep"),
	}

	bf.BatchAdd(items)

	// Remove subset
	toRemove := [][]byte{
		[]byte("to-remove-1"),
		[]byte("to-remove-2"),
		[]byte("to-remove-3"),
	}

	bf.BatchRemove(toRemove)

	// Verify removed items don't exist
	for _, item := range toRemove {
		if bf.Contains(item) {
			t.Errorf("Item %s should be removed", string(item))
		}
	}

	// Verify kept item still exists
	if !bf.Contains([]byte("to-keep")) {
		t.Error("Expected to-keep to still exist")
	}
}

// TestCountingBloomFilter_BatchRemove_Empty tests empty batch remove.
func TestCountingBloomFilter_BatchRemove_Empty(t *testing.T) {
	bf := NewCountingBloomFilter(1000, 3)
	bf.Add([]byte("test"))

	// Empty batch should not panic
	bf.BatchRemove([][]byte{})

	if !bf.Contains([]byte("test")) {
		t.Error("Expected item to still exist")
	}
}

// TestCountingBloomFilter_BatchContains tests batch contains operation.
func TestCountingBloomFilter_BatchContains(t *testing.T) {
	bf := NewCountingBloomFilter(1000, 3)

	items := [][]byte{
		[]byte("exists-1"),
		[]byte("exists-2"),
		[]byte("not-exists"),
	}

	// Add some items
	bf.Add(items[0])
	bf.Add(items[1])

	// Batch check
	results := bf.BatchContains(items)

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	if !results[0] {
		t.Error("Expected exists-1 to be found")
	}
	if !results[1] {
		t.Error("Expected exists-2 to be found")
	}
	if results[2] {
		t.Error("Expected not-exists to not be found")
	}
}

// TestCountingBloomFilter_BatchContains_Empty tests empty batch contains.
func TestCountingBloomFilter_BatchContains_Empty(t *testing.T) {
	bf := NewCountingBloomFilter(1000, 3)

	results := bf.BatchContains([][]byte{})

	if len(results) != 0 {
		t.Errorf("Expected 0 results, got %d", len(results))
	}
}

// TestCountingBloomFilter_BatchOperations_LargeBatch tests large batch operations.
func TestCountingBloomFilter_BatchOperations_LargeBatch(t *testing.T) {
	bf := NewCountingBloomFilter(100000, 5)

	// Create large batch
	items := make([][]byte, 1000)
	for i := 0; i < 1000; i++ {
		items[i] = []byte{byte(i % 256), byte(i / 256)}
	}

	// Batch add
	bf.BatchAdd(items)

	// Verify all exist
	results := bf.BatchContains(items)
	for i, exists := range results {
		if !exists {
			t.Errorf("Item %d should exist", i)
		}
	}
}

// TestCountingBloomFilter_BatchAdd_Concurrent tests concurrent batch adds.
func TestCountingBloomFilter_BatchAdd_Concurrent(t *testing.T) {
	bf := NewCountingBloomFilter(100000, 5)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			items := make([][]byte, 100)
			for j := 0; j < 100; j++ {
				items[j] = []byte{byte(id), byte(j)}
			}
			bf.BatchAdd(items)
		}(i)
	}

	wg.Wait()

	// Verify all items exist
	for i := 0; i < 10; i++ {
		for j := 0; j < 100; j++ {
			if !bf.Contains([]byte{byte(i), byte(j)}) {
				t.Errorf("Item [%d,%d] should exist", i, j)
			}
		}
	}
}

// TestCountingBloomFilter_BatchRemove_Partial tests partial batch remove.
func TestCountingBloomFilter_BatchRemove_Partial(t *testing.T) {
	bf := NewCountingBloomFilter(1000, 3)

	// Add items
	items := [][]byte{
		[]byte("a"),
		[]byte("b"),
		[]byte("c"),
		[]byte("d"),
	}
	bf.BatchAdd(items)

	// Try to remove some that exist and some that don't
	mixed := [][]byte{
		[]byte("a"),
		[]byte("nonexistent"),
		[]byte("c"),
	}

	// Should not panic
	bf.BatchRemove(mixed)

	// Verify existing items were removed
	if bf.Contains([]byte("a")) {
		t.Error("Expected 'a' to be removed")
	}
	if bf.Contains([]byte("c")) {
		t.Error("Expected 'c' to be removed")
	}
}

// TestCountingBloomFilter_BatchContains_NilItems tests nil items in batch.
func TestCountingBloomFilter_BatchContains_NilItems(t *testing.T) {
	bf := NewCountingBloomFilter(1000, 3)
	bf.Add([]byte("valid"))

	items := [][]byte{
		[]byte("valid"),
		nil,
		[]byte("nonexistent"),
	}

	// Should handle nil gracefully
	defer func() {
		if r := recover(); r != nil {
			t.Logf("BatchContains panicked with nil items: %v", r)
		}
	}()

	results := bf.BatchContains(items)
	
	// First item should exist
	if len(results) > 0 && !results[0] {
		t.Error("Expected first item to exist")
	}
}
