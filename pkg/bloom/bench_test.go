package bloom

import (
	"sync"
	"testing"
)

// BenchmarkAdd measures the performance of Add operation
func BenchmarkAdd(b *testing.B) {
	cbf := NewCountingBloomFilter(10000, 7)
	item := []byte("benchmark-test-item")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cbf.Add(item)
	}
}

// BenchmarkContains measures the performance of Contains operation
func BenchmarkContains(b *testing.B) {
	cbf := NewCountingBloomFilter(10000, 7)
	item := []byte("benchmark-test-item")
	
	// Add item first
	_ = cbf.Add(item)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cbf.Contains(item)
	}
}

// BenchmarkParallelAdd measures concurrent Add performance
func BenchmarkParallelAdd(b *testing.B) {
	cbf := NewCountingBloomFilter(10000, 7)
	item := []byte("benchmark-parallel-item")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = cbf.Add(item)
		}
	})
}

// BenchmarkParallelContains measures concurrent Contains performance
func BenchmarkParallelContains(b *testing.B) {
	cbf := NewCountingBloomFilter(10000, 7)
	item := []byte("benchmark-parallel-item")
	
	// Pre-populate with some items
	for i := 0; i < 1000; i++ {
		_ = cbf.Add([]byte("item-" + string(rune(i))))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = cbf.Contains(item)
		}
	})
}

// BenchmarkAdd_DifferentSizes measures Add performance with different filter sizes
func BenchmarkAdd_SmallFilter(b *testing.B) {
	cbf := NewCountingBloomFilter(100, 3)
	item := []byte("small-filter-item")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cbf.Add(item)
	}
}

func BenchmarkAdd_LargeFilter(b *testing.B) {
	cbf := NewCountingBloomFilter(1000000, 10)
	item := []byte("large-filter-item")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cbf.Add(item)
	}
}

// BenchmarkRemove measures Remove performance
func BenchmarkRemove(b *testing.B) {
	cbf := NewCountingBloomFilter(10000, 7)
	item := []byte("benchmark-remove-item")
	
	// Add item first
	_ = cbf.Add(item)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cbf.Remove(item)
		_ = cbf.Add(item) // Re-add for next iteration
	}
}

// BenchmarkCount measures Count performance
func BenchmarkCount(b *testing.B) {
	cbf := NewCountingBloomFilter(10000, 7)
	item := []byte("benchmark-count-item")
	
	// Add item multiple times
	for i := 0; i < 10; i++ {
		_ = cbf.Add(item)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cbf.Count(item)
	}
}

// BenchmarkSerialize measures serialization performance
func BenchmarkSerialize(b *testing.B) {
	cbf := NewCountingBloomFilter(10000, 7)
	
	// Add some items
	for i := 0; i < 100; i++ {
		_ = cbf.Add([]byte("item-" + string(rune(i))))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cbf.Serialize()
	}
}

// BenchmarkDeserialize measures deserialization performance
func BenchmarkDeserialize(b *testing.B) {
	cbf := NewCountingBloomFilter(10000, 7)
	
	// Add some items
	for i := 0; i < 100; i++ {
		_ = cbf.Add([]byte("item-" + string(rune(i))))
	}
	
	data := cbf.Serialize()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Deserialize(data)
	}
}

// BenchmarkMixedOperations measures mixed workload performance
func BenchmarkMixedOperations(b *testing.B) {
	cbf := NewCountingBloomFilter(10000, 7)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := []byte("mixed-item-" + string(rune(i%100)))
		
		// 50% Add, 40% Contains, 10% Remove
		switch i % 10 {
		case 0, 1, 2, 3, 4:
			_ = cbf.Add(item)
		case 5, 6, 7, 8:
			_ = cbf.Contains(item)
		case 9:
			cbf.Remove(item)
		}
	}
}

// BenchmarkParallelMixed measures concurrent mixed operations
func BenchmarkParallelMixed(b *testing.B) {
	cbf := NewCountingBloomFilter(10000, 7)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			item := []byte("parallel-mixed-item-" + string(rune(i%100)))
			
			switch i % 10 {
			case 0, 1, 2, 3, 4:
				_ = cbf.Add(item)
			case 5, 6, 7, 8:
				_ = cbf.Contains(item)
			case 9:
				cbf.Remove(item)
			}
			i++
		}
	})
}

// BenchmarkHashIndices measures hash computation performance
func BenchmarkHashIndices(b *testing.B) {
	item := []byte("benchmark-hash-item")
	m := 10000
	k := 7

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getHashIndices(item, m, k)
	}
}

// BenchmarkConcurrency_Stress tests performance under high contention
func BenchmarkConcurrency_Stress(b *testing.B) {
	cbf := NewCountingBloomFilter(10000, 7)
	numGoroutines := 100
	opsPerGoroutine := b.N / numGoroutines
	
	var wg sync.WaitGroup
	item := []byte("stress-test-item")
	
	b.ResetTimer()
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				_ = cbf.Add(item)
			}
		}()
	}
	
	wg.Wait()
}
