package bloom

import (
	"sync"
	"testing"
)

// TestNewCountingBloomFilter_EdgeCases tests boundary conditions for initialization
func TestNewCountingBloomFilter_EdgeCases(t *testing.T) {
	t.Run("M=0", func(t *testing.T) {
		// m=0 should create a filter with zero counters
		cbf := NewCountingBloomFilter(0, 5)
		if cbf.m != 0 {
			t.Errorf("Expected m=0, got %d", cbf.m)
		}
		if len(cbf.counters) != 0 {
			t.Errorf("Expected 0 counters, got %d", len(cbf.counters))
		}
	})

	t.Run("K=0", func(t *testing.T) {
		// k=0 should create a filter with zero hash functions
		cbf := NewCountingBloomFilter(1000, 0)
		if cbf.k != 0 {
			t.Errorf("Expected k=0, got %d", cbf.k)
		}
	})

	t.Run("M=1_K=1", func(t *testing.T) {
		// Minimum valid configuration
		cbf := NewCountingBloomFilter(1, 1)
		if cbf.m != 1 {
			t.Errorf("Expected m=1, got %d", cbf.m)
		}
		if cbf.k != 1 {
			t.Errorf("Expected k=1, got %d", cbf.k)
		}
		if len(cbf.counters) != 1 {
			t.Errorf("Expected 1 counter, got %d", len(cbf.counters))
		}
	})

	t.Run("LargeM", func(t *testing.T) {
		// Large filter should work
		cbf := NewCountingBloomFilter(1000000, 10)
		if cbf.m != 1000000 {
			t.Errorf("Expected m=1000000, got %d", cbf.m)
		}
	})
}

// TestAdd_NilItem tests adding nil item
func TestAdd_NilItem(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)

	// Adding nil should not panic
	err := cbf.Add(nil)
	if err != nil {
		t.Errorf("Add(nil) should not return error, got: %v", err)
	}

	// Verify it doesn't crash Contains either
	_ = cbf.Contains(nil)
}

// TestRemove_NonExistentItem tests removing an item that was never added
func TestRemove_NonExistentItem(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)

	item := []byte("never-added-item")

	// Remove should not panic on non-existent item
	cbf.Remove(item)

	// Count should be 0
	if count := cbf.Count(item); count != 0 {
		t.Errorf("Expected count=0 for non-existent item, got %d", count)
	}

	// Contains should return false
	if cbf.Contains(item) {
		t.Error("Contains should return false for non-existent item")
	}
}

// TestRemove_NilItem tests removing nil item
func TestRemove_NilItem(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)

	// Remove nil should not panic
	cbf.Remove(nil)
}

// TestDeserialize_CorruptedData tests deserializing corrupted data
func TestDeserialize_CorruptedData(t *testing.T) {
	t.Run("EmptyData", func(t *testing.T) {
		_, err := Deserialize([]byte{})
		if err != ErrInvalidData {
			t.Errorf("Expected ErrInvalidData for empty data, got: %v", err)
		}
	})

	t.Run("OnlyHeader", func(t *testing.T) {
		// 8 bytes header but no counter data
		data := make([]byte, 8)
		data[0] = 0x00
		data[1] = 0x00
		data[2] = 0x03
		data[3] = 0xE8 // m = 1000
		data[4] = 0x00
		data[5] = 0x00
		data[6] = 0x00
		data[7] = 0x05 // k = 5

		_, err := Deserialize(data)
		if err != ErrInvalidData {
			t.Errorf("Expected ErrInvalidData for incomplete data, got: %v", err)
		}
	})

	t.Run("NegativeM", func(t *testing.T) {
		// Negative m value (interpreted as large positive due to uint32)
		data := make([]byte, 8)
		data[0] = 0xFF
		data[1] = 0xFF
		data[2] = 0xFF
		data[3] = 0xFF // m = 4294967295 (way over MaxFilterSize)
		data[4] = 0x00
		data[5] = 0x00
		data[6] = 0x00
		data[7] = 0x05 // k = 5

		_, err := Deserialize(data)
		if err != ErrInvalidData {
			t.Errorf("Expected ErrInvalidData for oversized m, got: %v", err)
		}
	})

	t.Run("CorruptedCounterData", func(t *testing.T) {
		// Valid header but truncated counter data
		data := make([]byte, 10) // Header says 1000 counters but we only have 2 bytes
		data[0] = 0x00
		data[1] = 0x00
		data[2] = 0x03
		data[3] = 0xE8 // m = 1000
		data[4] = 0x00
		data[5] = 0x00
		data[6] = 0x00
		data[7] = 0x05 // k = 5
		// bytes 8-9 are zeros (incomplete counter data)

		_, err := Deserialize(data)
		if err != ErrInvalidData {
			t.Errorf("Expected ErrInvalidData for truncated counter data, got: %v", err)
		}
	})
}

// TestConcurrency_MixedOperations tests concurrent Add/Contains/Remove operations
func TestConcurrency_MixedOperations(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)

	var wg sync.WaitGroup
	numGoroutines := 10
	opsPerGoroutine := 100

	item := []byte("concurrent-mixed-item")

	// Start multiple goroutines doing mixed operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				if id%3 == 0 {
					_ = cbf.Add(item)
				} else if id%3 == 1 {
					_ = cbf.Contains(item)
				} else {
					cbf.Remove(item)
				}
			}
		}(i)
	}

	wg.Wait()

	// Should not crash - verify basic functionality still works
	_ = cbf.Count(item)
}

// TestSizeAndHashCount tests Size() and HashCount() methods
func TestSizeAndHashCount(t *testing.T) {
	cbf := NewCountingBloomFilter(500, 7)

	if cbf.Size() != 500 {
		t.Errorf("Expected Size()=500, got %d", cbf.Size())
	}

	if cbf.HashCount() != 7 {
		t.Errorf("Expected HashCount()=7, got %d", cbf.HashCount())
	}
}

// TestReset_EmptyFilter tests resetting an already empty filter
func TestReset_EmptyFilter(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)

	// Reset on empty filter should not panic
	cbf.Reset()

	// Should still work after reset
	item := []byte("test")
	if err := cbf.Add(item); err != nil {
		t.Errorf("Add after reset failed: %v", err)
	}
}

// TestCount_NeverAdded tests Count on item never added
func TestCount_NeverAdded(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)

	item := []byte("never-added")
	count := cbf.Count(item)

	if count != 0 {
		t.Errorf("Expected count=0 for never-added item, got %d", count)
	}
}

// TestContains_AfterRemoveAll tests Contains after removing all instances
func TestContains_AfterRemoveAll(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)

	item := []byte("test-item")

	// Add twice
	_ = cbf.Add(item)
	_ = cbf.Add(item)

	// Should be present
	if !cbf.Contains(item) {
		t.Error("Expected Contains=true after adding twice")
	}

	// Remove twice
	cbf.Remove(item)
	cbf.Remove(item)

	// Should not be present
	if cbf.Contains(item) {
		t.Error("Expected Contains=false after removing all instances")
	}
}

// TestSerialize_EmptyFilter tests serializing an empty filter
func TestSerialize_EmptyFilter(t *testing.T) {
	cbf := NewCountingBloomFilter(100, 5)

	data := cbf.Serialize()

	// Should have 8 bytes header + 100 bytes counters + 4 bytes CRC32 checksum
	expectedLen := 8 + 100 + 4
	if len(data) != expectedLen {
		t.Errorf("Expected serialized length=%d, got %d", expectedLen, len(data))
	}

	// Deserialize should work
	cbf2, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Failed to deserialize empty filter: %v", err)
	}

	if cbf2.m != 100 {
		t.Errorf("Expected m=100, got %d", cbf2.m)
	}
	if cbf2.k != 5 {
		t.Errorf("Expected k=5, got %d", cbf2.k)
	}
}

// TestHashIndices_EdgeCases tests hash index computation edge cases
func TestHashIndices_EdgeCases(t *testing.T) {
	t.Run("SmallM", func(t *testing.T) {
		item := []byte("test")
		m := 10
		k := 3

		indices := getHashIndices(item, m, k)

		if len(indices) != k {
			t.Errorf("Expected %d indices, got %d", k, len(indices))
		}

		for _, idx := range indices {
			if idx < 0 || idx >= m {
				t.Errorf("Index %d out of range [0, %d)", idx, m)
			}
		}
	})

	t.Run("KGreaterThanM", func(t *testing.T) {
		item := []byte("test")
		m := 5
		k := 10

		indices := getHashIndices(item, m, k)

		if len(indices) != k {
			t.Errorf("Expected %d indices, got %d", k, len(indices))
		}

		// All indices should still be in valid range
		for _, idx := range indices {
			if idx < 0 || idx >= m {
				t.Errorf("Index %d out of range [0, %d)", idx, m)
			}
		}
	})
}
