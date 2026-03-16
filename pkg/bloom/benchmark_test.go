package bloom

import (
	"fmt"
	"math/rand"
	"testing"
)

// BenchmarkAdd benchmarks the Add operation.
func BenchmarkAdd(b *testing.B) {
	bf := NewCountingBloomFilter(10000, 3)
	item := []byte("test-item")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.Add(item)
	}
}

// BenchmarkContains benchmarks the Contains operation.
func BenchmarkContains(b *testing.B) {
	bf := NewCountingBloomFilter(10000, 3)
	item := []byte("test-item")
	bf.Add(item)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.Contains(item)
	}
}

// BenchmarkRemove benchmarks the Remove operation.
func BenchmarkRemove(b *testing.B) {
	bf := NewCountingBloomFilter(10000, 3)
	item := []byte("test-item")
	bf.Add(item)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.Remove(item)
		bf.Add(item) // Re-add for next iteration
	}
}

// BenchmarkAddLargeFilter benchmarks Add on a large filter.
func BenchmarkAddLargeFilter(b *testing.B) {
	bf := NewCountingBloomFilter(1000000, 7)
	item := []byte("benchmark-item")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.Add(item)
	}
}

// BenchmarkContainsLargeFilter benchmarks Contains on a large filter.
func BenchmarkContainsLargeFilter(b *testing.B) {
	bf := NewCountingBloomFilter(1000000, 7)
	item := []byte("benchmark-item")
	bf.Add(item)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.Contains(item)
	}
}

// BenchmarkAddManyItems benchmarks adding many unique items.
func BenchmarkAddManyItems(b *testing.B) {
	bf := NewCountingBloomFilter(100000, 5)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		item := []byte(fmt.Sprintf("item-%d", i))
		bf.Add(item)
	}
}

// BenchmarkContainsManyItems benchmarks Contains with many items.
func BenchmarkContainsManyItems(b *testing.B) {
	bf := NewCountingBloomFilter(100000, 5)
	
	// Pre-populate
	for i := 0; i < 10000; i++ {
		item := []byte(fmt.Sprintf("item-%d", i))
		bf.Add(item)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := []byte(fmt.Sprintf("item-%d", i%10000))
		bf.Contains(item)
	}
}

// BenchmarkAddConcurrent benchmarks concurrent Add operations.
func BenchmarkAddConcurrent(b *testing.B) {
	bf := NewCountingBloomFilter(100000, 5)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			item := []byte(fmt.Sprintf("concurrent-item-%d", i))
			bf.Add(item)
			i++
		}
	})
}

// BenchmarkContainsConcurrent benchmarks concurrent Contains operations.
func BenchmarkContainsConcurrent(b *testing.B) {
	bf := NewCountingBloomFilter(100000, 5)
	
	// Pre-populate
	for i := 0; i < 1000; i++ {
		item := []byte(fmt.Sprintf("item-%d", i))
		bf.Add(item)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			item := []byte(fmt.Sprintf("item-%d", rand.Intn(1000)))
			bf.Contains(item)
		}
	})
}

// BenchmarkSerialize benchmarks serialization.
func BenchmarkSerialize(b *testing.B) {
	bf := NewCountingBloomFilter(10000, 3)
	
	// Add some items
	for i := 0; i < 100; i++ {
		item := []byte(fmt.Sprintf("item-%d", i))
		bf.Add(item)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bf.Serialize()
	}
}

// BenchmarkDeserialize benchmarks deserialization.
func BenchmarkDeserialize(b *testing.B) {
	bf := NewCountingBloomFilter(10000, 3)
	
	// Add some items
	for i := 0; i < 100; i++ {
		item := []byte(fmt.Sprintf("item-%d", i))
		bf.Add(item)
	}

	data := bf.Serialize()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Deserialize(data)
	}
}

// BenchmarkHashIndices benchmarks hash index computation.
func BenchmarkHashIndices(b *testing.B) {
	item := []byte("test-item-for-hashing")
	m := 10000
	k := 7

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getHashIndices(item, m, k)
	}
}

// BenchmarkDoubleHash benchmarks double hashing.
func BenchmarkDoubleHash(b *testing.B) {
	item := []byte("test-item-for-double-hash")
	m := 10000

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DoubleHash(item, m)
	}
}

// BenchmarkComputeIndices benchmarks index computation.
func BenchmarkComputeIndices(b *testing.B) {
	item := []byte("test-item-for-indices")
	m := 10000
	k := 7

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeIndices(item, m, k)
	}
}

// BenchmarkMurmurHash3Provider benchmarks MurmurHash3.
func BenchmarkMurmurHash3Provider(b *testing.B) {
	provider := NewMurmurHash3Provider()
	data := []byte("test-data-for-murmur3")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		provider.Hash(data)
	}
}

// BenchmarkFalsePositiveRate benchmarks false positive rate measurement.
func BenchmarkFalsePositiveRate(b *testing.B) {
	bf := NewCountingBloomFilter(100000, 7)
	
	// Add items
	for i := 0; i < 10000; i++ {
		item := []byte(fmt.Sprintf("item-%d", i))
		bf.Add(item)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Check non-existent items
		item := []byte(fmt.Sprintf("nonexistent-%d", i+10000))
		bf.Contains(item)
	}
}

// BenchmarkMemoryUsage benchmarks memory allocation.
func BenchmarkMemoryUsage(b *testing.B) {
	b.ReportAllocs()
	
	for i := 0; i < b.N; i++ {
		bf := NewCountingBloomFilter(10000, 3)
		_ = bf
	}
}

// BenchmarkAddContainsCycle benchmarks add-contains cycle.
func BenchmarkAddContainsCycle(b *testing.B) {
	bf := NewCountingBloomFilter(10000, 3)
	item := []byte("cycle-item")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bf.Add(item)
		bf.Contains(item)
		bf.Remove(item)
	}
}

// BenchmarkBatchOperations benchmarks batch operations.
func BenchmarkBatchAdd(b *testing.B) {
	bf := NewCountingBloomFilter(100000, 5)
	items := make([][]byte, 100)
	for i := range items {
		items[i] = []byte(fmt.Sprintf("batch-item-%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, item := range items {
			bf.Add(item)
		}
	}
}

// BenchmarkBatchContains benchmarks batch contains.
func BenchmarkBatchContains(b *testing.B) {
	bf := NewCountingBloomFilter(100000, 5)
	items := make([][]byte, 100)
	for i := range items {
		items[i] = []byte(fmt.Sprintf("batch-item-%d", i))
		bf.Add(items[i])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, item := range items {
			bf.Contains(item)
		}
	}
}
