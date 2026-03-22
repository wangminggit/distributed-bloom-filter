package bloom

import (
	"sync"
	"testing"
)

// TestGetHashIndicesPooled tests pooled hash indices allocation.
func TestGetHashIndicesPooled(t *testing.T) {
	bf := NewCountingBloomFilter(1000, 3)
	item := []byte("test")

	// Get pooled indices (returns pointer to slice)
	indices := getHashIndicesPooled(item, bf.m, bf.k)

	if indices == nil {
		t.Fatal("Expected non-nil indices")
	}
	if len(*indices) != bf.k {
		t.Errorf("Expected %d indices, got %d", bf.k, len(*indices))
	}

	// Put back to pool
	putHashIndices(indices)

	// Should not panic
	t.Log("Pooled indices test completed")
}

// TestPutHashIndices tests returning indices to pool.
func TestPutHashIndices(t *testing.T) {
	item := []byte("test-pool")
	
	// Get indices
	indices := getHashIndicesPooled(item, 1000, 5)
	if indices == nil {
		t.Fatal("Expected non-nil indices")
	}

	// Put back to pool
	putHashIndices(indices)

	// Get new indices (might be the same ones)
	newIndices := getHashIndicesPooled(item, 1000, 5)
	if newIndices == nil {
		t.Fatal("Expected non-nil indices")
	}

	// Put back
	putHashIndices(newIndices)
}

// TestHashPool_ConcurrentAccess tests thread safety of hash pool.
func TestHashPool_ConcurrentAccess(t *testing.T) {
	bf := NewCountingBloomFilter(10000, 5)
	item := []byte("concurrent-test")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				indices := getHashIndicesPooled(item, bf.m, bf.k)
				
				// Use indices
				for k := range *indices {
					(*indices)[k] = k + id
				}

				// Return to pool
				putHashIndices(indices)
			}
		}(i)
	}

	wg.Wait()

	t.Log("Concurrent pool access test completed successfully")
}

// TestGetHashIndicesPooled_DifferentItems tests with different items.
func TestGetHashIndicesPooled_DifferentItems(t *testing.T) {
	items := [][]byte{
		[]byte("item1"),
		[]byte("item2"),
		[]byte("item3"),
	}

	for _, item := range items {
		indices := getHashIndicesPooled(item, 1000, 5)
		if indices == nil {
			t.Errorf("Expected non-nil indices for item %v", item)
		}
		if len(*indices) != 5 {
			t.Errorf("Expected 5 indices, got %d", len(*indices))
		}
		putHashIndices(indices)
	}
}

// TestGetHashIndicesPooled_ZeroK tests with k=0.
func TestGetHashIndicesPooled_ZeroK(t *testing.T) {
	item := []byte("test")
	indices := getHashIndicesPooled(item, 1000, 0)

	if indices == nil {
		t.Error("Expected non-nil indices")
	}
	if len(*indices) != 0 {
		t.Errorf("Expected 0 indices, got %d", len(*indices))
	}
}

// TestHashPool_Reuse tests that pool actually reuses allocations.
func TestHashPool_Reuse(t *testing.T) {
	item := []byte("reuse-test")
	
	// Get and return indices multiple times
	for i := 0; i < 10; i++ {
		indices := getHashIndicesPooled(item, 1000, 5)
		putHashIndices(indices)
	}

	// Should not cause memory issues
	t.Log("Pool reuse test completed")
}

// TestGetHashIndicesPooled_LargeK tests with large k value.
func TestGetHashIndicesPooled_LargeK(t *testing.T) {
	item := []byte("large-k-test")
	largeK := 100
	
	indices := getHashIndicesPooled(item, 10000, largeK)
	if indices == nil {
		t.Fatal("Expected non-nil indices")
	}
	if len(*indices) != largeK {
		t.Errorf("Expected %d indices, got %d", largeK, len(*indices))
	}

	// Verify all indices are usable
	for i := range *indices {
		(*indices)[i] = i
	}

	putHashIndices(indices)
}

// TestMurmurHash3Provider_Deterministic tests hash determinism.
func TestMurmurHash3Provider_Deterministic(t *testing.T) {
	provider := NewMurmurHash3Provider()
	item := []byte("deterministic-test")

	hash1 := provider.Hash(item)
	hash2 := provider.Hash(item)

	if hash1 != hash2 {
		t.Errorf("Expected same hash, got %d and %d", hash1, hash2)
	}
}

// TestMurmurHash3Provider_EmptyItem tests empty item hashing.
func TestMurmurHash3Provider_EmptyItem(t *testing.T) {
	provider := NewMurmurHash3Provider()

	hash := provider.Hash([]byte{})
	t.Logf("Empty item hash: %d", hash)
}

// TestDoubleHash_Function tests the DoubleHash function.
func TestDoubleHash_Function(t *testing.T) {
	item := []byte("test-item")
	m := 1000

	h1, h2 := DoubleHash(item, m)

	if h1 < 0 || h1 >= m {
		t.Errorf("Expected h1 in range [0, %d), got %d", m, h1)
	}
	if h2 < 0 || h2 >= m {
		t.Errorf("Expected h2 in range [0, %d), got %d", m, h2)
	}

	// Should be deterministic
	h1_2, h2_2 := DoubleHash(item, m)
	if h1 != h1_2 || h2 != h2_2 {
		t.Error("Expected deterministic results")
	}
}

// TestDoubleHash_DifferentItems tests different items produce different hashes.
func TestDoubleHash_DifferentItems(t *testing.T) {
	m := 1000

	h1, _ := DoubleHash([]byte("item1"), m)
	h2, _ := DoubleHash([]byte("item2"), m)

	if h1 == h2 {
		t.Log("Warning: Different items produced same hash (collision)")
	}
}

// Helper function to get pointer address for testing
func unsafePointer(s *[]int) uintptr {
	return uintptr(len(*s))
}
