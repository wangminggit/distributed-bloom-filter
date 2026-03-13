package bloom

import (
	"testing"
)

// TestMurmurHash3Provider tests the hash provider implementation
func TestMurmurHash3Provider(t *testing.T) {
	provider := NewMurmurHash3Provider()

	t.Run("Hash", func(t *testing.T) {
		data := []byte("test-hash-data")
		hash1 := provider.Hash(data)
		hash2 := provider.Hash(data)

		// Hash should be deterministic
		if hash1 != hash2 {
			t.Errorf("Hash not deterministic: %d != %d", hash1, hash2)
		}

		// Different data should produce different hash (with high probability)
		hash3 := provider.Hash([]byte("different-data"))
		if hash1 == hash3 {
			t.Error("Different data produced same hash")
		}
	})

	t.Run("Reset", func(t *testing.T) {
		provider.Reset()
		// Reset should not panic
	})

	t.Run("Write", func(t *testing.T) {
		data := []byte("test-write")
		n, err := provider.Write(data)
		if err != nil {
			t.Errorf("Write failed: %v", err)
		}
		if n != len(data) {
			t.Errorf("Write returned %d, expected %d", n, len(data))
		}
	})

	t.Run("Sum", func(t *testing.T) {
		data := []byte("test-sum")
		provider.Write(data)
		sum := provider.Sum(nil)
		if len(sum) == 0 {
			t.Error("Sum returned empty result")
		}
	})
}

// TestDoubleHash tests the double hashing function
func TestDoubleHash(t *testing.T) {
	item := []byte("test-double-hash")
	m := 1000

	h1, h2 := DoubleHash(item, m)

	// Both hashes should be in valid range
	if h1 < 0 || h1 >= m {
		t.Errorf("h1 out of range: %d", h1)
	}
	if h2 < 0 || h2 >= m {
		t.Errorf("h2 out of range: %d", h2)
	}

	// h2 should be coprime to m (for double hashing to work well)
	// We just verify it's not 0
	if h2 == 0 {
		t.Error("h2 should not be 0")
	}

	// Hashes should be deterministic
	h1_2, h2_2 := DoubleHash(item, m)
	if h1 != h1_2 || h2 != h2_2 {
		t.Error("DoubleHash not deterministic")
	}

	// Different items should produce different hashes
	h1_3, h2_3 := DoubleHash([]byte("different-item"), m)
	if h1 == h1_3 && h2 == h2_3 {
		t.Error("Different items produced same hashes")
	}
}

// TestComputeIndices tests the ComputeIndices function
func TestComputeIndices(t *testing.T) {
	item := []byte("test-compute-indices")
	m := 100
	k := 5

	indices := ComputeIndices(item, m, k)

	if len(indices) != k {
		t.Errorf("Expected %d indices, got %d", k, len(indices))
	}

	// All indices should be in valid range
	for _, idx := range indices {
		if idx < 0 || idx >= m {
			t.Errorf("Index %d out of range [0, %d)", idx, m)
		}
	}

	// Indices should be deterministic
	indices2 := ComputeIndices(item, m, k)
	for i := range indices {
		if indices[i] != indices2[i] {
			t.Errorf("Indices not deterministic at position %d", i)
		}
	}
}

// TestDoubleHash_EdgeCases tests edge cases for double hashing
func TestDoubleHash_EdgeCases(t *testing.T) {
	t.Run("SmallM", func(t *testing.T) {
		item := []byte("test")
		m := 10

		h1, h2 := DoubleHash(item, m)
		if h1 < 0 || h1 >= m {
			t.Errorf("h1 out of range for small m: %d", h1)
		}
		if h2 < 0 || h2 >= m {
			t.Errorf("h2 out of range for small m: %d", h2)
		}
	})

	t.Run("EmptyItem", func(t *testing.T) {
		item := []byte{}
		m := 100

		h1, h2 := DoubleHash(item, m)
		if h1 < 0 || h1 >= m {
			t.Errorf("h1 out of range for empty item: %d", h1)
		}
		if h2 < 0 || h2 >= m {
			t.Errorf("h2 out of range for empty item: %d", h2)
		}
	})

	t.Run("NilItem", func(t *testing.T) {
		var item []byte = nil
		m := 100

		h1, h2 := DoubleHash(item, m)
		if h1 < 0 || h1 >= m {
			t.Errorf("h1 out of range for nil item: %d", h1)
		}
		if h2 < 0 || h2 >= m {
			t.Errorf("h2 out of range for nil item: %d", h2)
		}
	})
}

// TestComputeIndices_EdgeCases tests edge cases for index computation
func TestComputeIndices_EdgeCases(t *testing.T) {
	t.Run("K=0", func(t *testing.T) {
		item := []byte("test")
		m := 100
		k := 0

		indices := ComputeIndices(item, m, k)
		if len(indices) != 0 {
			t.Errorf("Expected 0 indices for k=0, got %d", len(indices))
		}
	})

	t.Run("K=1", func(t *testing.T) {
		item := []byte("test")
		m := 100
		k := 1

		indices := ComputeIndices(item, m, k)
		if len(indices) != 1 {
			t.Errorf("Expected 1 index for k=1, got %d", len(indices))
		}
	})

	t.Run("LargeK", func(t *testing.T) {
		item := []byte("test")
		m := 10
		k := 100

		indices := ComputeIndices(item, m, k)
		if len(indices) != 100 {
			t.Errorf("Expected 100 indices, got %d", len(indices))
		}

		// All should still be in valid range
		for _, idx := range indices {
			if idx < 0 || idx >= m {
				t.Errorf("Index %d out of range [0, %d)", idx, m)
			}
		}
	})
}
