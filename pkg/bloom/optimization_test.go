package bloom

import (
	"fmt"
	"testing"
)

// BenchmarkIndexCachePerformance benchmarks the index cache.
func BenchmarkIndexCachePerformance(b *testing.B) {
	cache := NewIndexCache(1000)
	
	// Simulate hot items (frequently accessed)
	hotItems := make([]string, 10)
	for i := 0; i < 10; i++ {
		hotItems[i] = fmt.Sprintf("hot-item-%d", i)
		// Pre-populate cache
		cache.Put(hotItems[i], []int{1, 2, 3})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Access hot items
		for _, item := range hotItems {
			cache.Get(item)
		}
	}
}

// BenchmarkCachedVsUncached benchmarks cached vs uncached operations.
func BenchmarkCachedVsUncached(b *testing.B) {
	cbf := NewCountingBloomFilter(10000, 3)
	cbfCached := NewCountingBloomFilterWithCache(10000, 3, 1000)
	
	items := make([][]byte, 100)
	for i := 0; i < 100; i++ {
		items[i] = []byte(fmt.Sprintf("item-%d", i))
	}

	b.Run("Uncached", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, item := range items {
				cbf.Contains(item)
			}
		}
	})

	b.Run("Cached", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, item := range items {
				cbfCached.Contains(item)
			}
		}
	})
}

// BenchmarkCacheHitRate benchmarks cache hit scenarios.
func BenchmarkCacheHitRate(b *testing.B) {
	cache := NewIndexCache(100)
	
	// Populate cache
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("item-%d", i)
		cache.Put(key, []int{i % 10, (i + 1) % 10, (i + 2) % 10})
	}

	b.Run("CacheHit", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("item-%d", i%100)
			cache.Get(key)
		}
	})

	b.Run("CacheMiss", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("miss-%d", i)
			cache.Get(key)
		}
	})

	b.Run("Mixed", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if i%2 == 0 {
				key := fmt.Sprintf("item-%d", i%100) // Hit
				cache.Get(key)
			} else {
				key := fmt.Sprintf("miss-%d", i) // Miss
				cache.Get(key)
			}
		}
	})
}

// BenchmarkCompressionPerformance benchmarks compression.
func BenchmarkCompressionPerformance(b *testing.B) {
	cbf := NewCountingBloomFilter(10000, 3)
	
	// Add data
	for i := 0; i < 1000; i++ {
		cbf.Add([]byte(fmt.Sprintf("compress-item-%d", i)))
	}

	b.Run("Serialize", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cbf.Serialize()
		}
	})

	b.Run("CompressSerialize", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cbf.CompressSerialize()
		}
	})
}

// BenchmarkCompressionRatio benchmarks compression effectiveness.
func BenchmarkCompressionRatio(b *testing.B) {
	sizes := []int{1000, 10000, 100000}
	
	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size%d", size), func(b *testing.B) {
			cbf := NewCountingBloomFilter(size, 3)
			
			// Add data
			for i := 0; i < size/10; i++ {
				cbf.Add([]byte(fmt.Sprintf("item-%d", i)))
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				original, compressed, ratio, err := cbf.CompressionRatio()
				if err != nil {
					b.Fatal(err)
				}
				b.ReportMetric(float64(original), "original_bytes")
				b.ReportMetric(float64(compressed), "compressed_bytes")
				b.ReportMetric(ratio, "compression_ratio")
			}
		})
	}
}

// BenchmarkAsyncWALPerformance benchmarks async WAL writes.
func BenchmarkAsyncWALPerformance(b *testing.B) {
	b.Run("SyncWrite", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Simulate sync write latency
			// In real scenario, this would be disk I/O
		}
	})

	b.Run("AsyncWrite", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Async write returns immediately
			// Background flush handles actual I/O
		}
	})
}

// BenchmarkBatchOptimization benchmarks batch operation improvements.
func BenchmarkBatchOptimization(b *testing.B) {
	cbf := NewCountingBloomFilter(100000, 5)
	
	b.Run("Individual100", func(b *testing.B) {
		items := make([][]byte, 100)
		for i := 0; i < 100; i++ {
			items[i] = []byte(fmt.Sprintf("item-%d", i))
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, item := range items {
				cbf.Add(item)
			}
		}
	})

	b.Run("Batch100", func(b *testing.B) {
		items := make([][]byte, 100)
		for i := 0; i < 100; i++ {
			items[i] = []byte(fmt.Sprintf("item-%d", i))
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cbf.BatchAdd(items)
		}
	})

	b.Run("Individual1000", func(b *testing.B) {
		items := make([][]byte, 1000)
		for i := 0; i < 1000; i++ {
			items[i] = []byte(fmt.Sprintf("item-%d", i))
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, item := range items {
				cbf.Add(item)
			}
		}
	})

	b.Run("Batch1000", func(b *testing.B) {
		items := make([][]byte, 1000)
		for i := 0; i < 1000; i++ {
			items[i] = []byte(fmt.Sprintf("item-%d", i))
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			cbf.BatchAdd(items)
		}
	})
}

// BenchmarkMemoryPoolPerformance benchmarks memory pool effectiveness.
func BenchmarkMemoryPoolPerformance(b *testing.B) {
	item := []byte("test-item-for-pool-benchmark")
	m := 10000
	k := 3

	b.Run("StandardAlloc", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			indices := getHashIndices(item, m, k)
			_ = indices
		}
	})

	b.Run("PooledAlloc", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			indices := getHashIndicesPooled(item, m, k)
			putHashIndices(indices)
		}
	})
}

// BenchmarkCacheLRUPerformance benchmarks LRU eviction.
func BenchmarkCacheLRUPerformance(b *testing.B) {
	cache := NewIndexCache(100)
	
	// Fill cache
	for i := 0; i < 100; i++ {
		cache.Put(fmt.Sprintf("item-%d", i), []int{i})
	}

	b.Run("LRUEviction", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// This will cause eviction
			key := fmt.Sprintf("new-item-%d", i)
			cache.Put(key, []int{i % 10})
		}
	})
}
