package bloom

import (
	"testing"
)

func TestGetHashIndices(t *testing.T) {
	item := []byte("test-item")
	m := 1000
	k := 7

	indices := getHashIndices(item, m, k)

	if len(indices) != k {
		t.Errorf("Expected %d indices, got %d", k, len(indices))
	}

	// All indices should be in range [0, m)
	for i, idx := range indices {
		if idx < 0 || idx >= m {
			t.Errorf("Index %d at position %d is out of range [0, %d)", idx, i, m)
		}
	}

	// Same item should produce same indices
	indices2 := getHashIndices(item, m, k)
	for i := range indices {
		if indices[i] != indices2[i] {
			t.Errorf("Indices differ for same input: %v vs %v", indices, indices2)
		}
	}

	// Different items should produce different indices (most of the time)
	item2 := []byte("different-item")
	indices3 := getHashIndices(item2, m, k)
	same := true
	for i := range indices {
		if indices[i] != indices3[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("Different items produced same indices")
	}
}

func TestGetHashIndicesEmptyItem(t *testing.T) {
	item := []byte("")
	m := 1000
	k := 7

	indices := getHashIndices(item, m, k)

	if len(indices) != k {
		t.Errorf("Expected %d indices, got %d", k, len(indices))
	}

	for i, idx := range indices {
		if idx < 0 || idx >= m {
			t.Errorf("Index %d at position %d is out of range [0, %d)", idx, i, m)
		}
	}
}

func TestGetHashIndicesLargeItem(t *testing.T) {
	// Create a large item
	item := make([]byte, 10000)
	for i := range item {
		item[i] = byte(i % 256)
	}

	m := 1000
	k := 7

	indices := getHashIndices(item, m, k)

	if len(indices) != k {
		t.Errorf("Expected %d indices, got %d", k, len(indices))
	}

	for i, idx := range indices {
		if idx < 0 || idx >= m {
			t.Errorf("Index %d at position %d is out of range [0, %d)", idx, i, m)
		}
	}
}

func TestGetHashIndicesSmallM(t *testing.T) {
	item := []byte("test")
	m := 10
	k := 7

	indices := getHashIndices(item, m, k)

	if len(indices) != k {
		t.Errorf("Expected %d indices, got %d", k, len(indices))
	}

	for i, idx := range indices {
		if idx < 0 || idx >= m {
			t.Errorf("Index %d at position %d is out of range [0, %d)", idx, i, m)
		}
	}
}

func TestMurmurHash3Provider(t *testing.T) {
	provider := NewMurmurHash3Provider()

	if provider == nil {
		t.Fatal("Expected provider to be created")
	}

	if provider.Hash32 == nil {
		t.Fatal("Expected Hash32 to be initialized")
	}
}

func TestMurmurHash3ProviderHash(t *testing.T) {
	provider := NewMurmurHash3Provider()

	data := []byte("test-data")
	hash1 := provider.Hash(data)
	hash2 := provider.Hash(data)

	if hash1 != hash2 {
		t.Errorf("Same data should produce same hash: %d vs %d", hash1, hash2)
	}

	// Different data should produce different hash
	hash3 := provider.Hash([]byte("different-data"))
	if hash1 == hash3 {
		t.Error("Different data should produce different hash")
	}
}

func TestMurmurHash3ProviderHashEmpty(t *testing.T) {
	provider := NewMurmurHash3Provider()

	hash := provider.Hash([]byte(""))
	
	// Empty data should produce a consistent hash (MurmurHash3 returns 0 for empty input)
	// The important thing is that it's deterministic
	hash2 := provider.Hash([]byte(""))
	if hash != hash2 {
		t.Error("Empty data should produce consistent hash")
	}
}

func TestMurmurHash3ProviderReset(t *testing.T) {
	provider := NewMurmurHash3Provider()

	data1 := []byte("first")
	hash1 := provider.Hash(data1)

	provider.Reset()

	data2 := []byte("second")
	hash2 := provider.Hash(data2)

	// After reset, hashing different data should produce different result
	if hash1 == hash2 {
		t.Error("After reset, different data should produce different hash")
	}
}

func TestMurmurHash3ProviderWrite(t *testing.T) {
	provider := NewMurmurHash3Provider()

	n, err := provider.Write([]byte("test"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != 4 {
		t.Errorf("Expected 4 bytes written, got %d", n)
	}

	// Write more data
	n, err = provider.Write([]byte(" more"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != 5 {
		t.Errorf("Expected 5 bytes written, got %d", n)
	}
}

func TestMurmurHash3ProviderSum(t *testing.T) {
	provider := NewMurmurHash3Provider()

	provider.Write([]byte("test-data"))
	sum := provider.Sum(nil)

	if len(sum) != 4 {
		t.Errorf("Expected 4-byte sum, got %d bytes", len(sum))
	}

	// Sum should be consistent
	provider2 := NewMurmurHash3Provider()
	provider2.Write([]byte("test-data"))
	sum2 := provider2.Sum(nil)

	for i := range sum {
		if sum[i] != sum2[i] {
			t.Errorf("Sum differs at byte %d: %d vs %d", i, sum[i], sum2[i])
		}
	}
}

func TestMurmurHash3ProviderWriteMultipleTimes(t *testing.T) {
	provider := NewMurmurHash3Provider()

	// Write in chunks
	provider.Write([]byte("hel"))
	provider.Write([]byte("lo"))
	sum1 := provider.Sum(nil)

	// Write all at once
	provider2 := NewMurmurHash3Provider()
	provider2.Write([]byte("hello"))
	sum2 := provider2.Sum(nil)

	// Should produce same result
	for i := range sum1 {
		if sum1[i] != sum2[i] {
			t.Errorf("Chunked write differs from single write at byte %d", i)
		}
	}
}

func TestDoubleHash(t *testing.T) {
	item := []byte("test-item")
	m := 1000

	h1, h2 := DoubleHash(item, m)

	// Both hashes should be in range [0, m)
	if h1 < 0 || h1 >= m {
		t.Errorf("h1 %d is out of range [0, %d)", h1, m)
	}
	if h2 < 0 || h2 >= m {
		t.Errorf("h2 %d is out of range [0, %d)", h2, m)
	}

	// h2 should be >= 1 (as per implementation)
	if h2 < 1 {
		t.Errorf("h2 should be >= 1, got %d", h2)
	}

	// Same item should produce same hashes
	h1_2, h2_2 := DoubleHash(item, m)
	if h1 != h1_2 || h2 != h2_2 {
		t.Errorf("Same item should produce same hashes: (%d,%d) vs (%d,%d)", h1, h2, h1_2, h2_2)
	}

	// Different items should produce different hashes (most of the time)
	h1_3, h2_3 := DoubleHash([]byte("different"), m)
	if h1 == h1_3 && h2 == h2_3 {
		t.Error("Different items should produce different hashes")
	}
}

func TestDoubleHashEmptyItem(t *testing.T) {
	item := []byte("")
	m := 1000

	h1, h2 := DoubleHash(item, m)

	if h1 < 0 || h1 >= m {
		t.Errorf("h1 %d is out of range [0, %d)", h1, m)
	}
	if h2 < 1 || h2 >= m {
		t.Errorf("h2 %d is out of range [1, %d)", h2, m)
	}
}

func TestDoubleHashSmallM(t *testing.T) {
	item := []byte("test")
	m := 10

	h1, h2 := DoubleHash(item, m)

	if h1 < 0 || h1 >= m {
		t.Errorf("h1 %d is out of range [0, %d)", h1, m)
	}
	if h2 < 1 || h2 >= m {
		t.Errorf("h2 %d is out of range [1, %d)", h2, m)
	}
}

func TestDoubleHashVariousSizes(t *testing.T) {
	testCases := []struct {
		name string
		m    int
	}{
		{"small", 10},
		{"medium", 100},
		{"large", 10000},
		{"very large", 100000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			item := []byte("test-item")
			h1, h2 := DoubleHash(item, tc.m)

			if h1 < 0 || h1 >= tc.m {
				t.Errorf("h1 %d is out of range [0, %d)", h1, tc.m)
			}
			if h2 < 1 || h2 >= tc.m {
				t.Errorf("h2 %d is out of range [1, %d)", h2, tc.m)
			}
		})
	}
}

func TestComputeIndices(t *testing.T) {
	item := []byte("test-item")
	m := 1000
	k := 7

	indices := ComputeIndices(item, m, k)

	if len(indices) != k {
		t.Errorf("Expected %d indices, got %d", k, len(indices))
	}

	// All indices should be in range [0, m)
	for i, idx := range indices {
		if idx < 0 || idx >= m {
			t.Errorf("Index %d at position %d is out of range [0, %d)", idx, i, m)
		}
	}

	// Same item should produce same indices
	indices2 := ComputeIndices(item, m, k)
	for i := range indices {
		if indices[i] != indices2[i] {
			t.Errorf("Indices differ for same input: %v vs %v", indices, indices2)
		}
	}

	// Different items should produce different indices
	indices3 := ComputeIndices([]byte("different"), m, k)
	same := true
	for i := range indices {
		if indices[i] != indices3[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("Different items produced same indices")
	}
}

func TestComputeIndicesEmptyItem(t *testing.T) {
	item := []byte("")
	m := 1000
	k := 7

	indices := ComputeIndices(item, m, k)

	if len(indices) != k {
		t.Errorf("Expected %d indices, got %d", k, len(indices))
	}

	for i, idx := range indices {
		if idx < 0 || idx >= m {
			t.Errorf("Index %d at position %d is out of range [0, %d)", idx, i, m)
		}
	}
}

func TestComputeIndicesKValues(t *testing.T) {
	item := []byte("test")
	m := 1000

	testCases := []int{1, 3, 5, 7, 10, 20}

	for _, k := range testCases {
		indices := ComputeIndices(item, m, k)
		if len(indices) != k {
			t.Errorf("k=%d: Expected %d indices, got %d", k, k, len(indices))
		}
	}
}

func TestComputeIndicesDeterministic(t *testing.T) {
	item := []byte("deterministic-test")
	m := 1000
	k := 7

	// Run multiple times to ensure determinism
	allIndices := make([][]int, 10)
	for i := 0; i < 10; i++ {
		allIndices[i] = ComputeIndices(item, m, k)
	}

	for i := 1; i < 10; i++ {
		for j := range allIndices[0] {
			if allIndices[0][j] != allIndices[i][j] {
				t.Errorf("Non-deterministic: run 0 has %d at pos %d, run %d has %d",
					allIndices[0][j], j, i, allIndices[i][j])
			}
		}
	}
}

func TestHashDistribution(t *testing.T) {
	// Test that hash indices are reasonably distributed
	m := 1000
	k := 7
	n := 1000

	buckets := make(map[int]int)

	for i := 0; i < n; i++ {
		item := []byte("item-" + string(rune(i)))
		indices := ComputeIndices(item, m, k)
		for _, idx := range indices {
			buckets[idx]++
		}
	}

	// Check that we have good coverage (at least 50% of buckets used)
	usedBuckets := len(buckets)
	coverage := float64(usedBuckets) / float64(m)
	if coverage < 0.5 {
		t.Errorf("Poor hash distribution: only %.2f%% of buckets used", coverage*100)
	}
}
