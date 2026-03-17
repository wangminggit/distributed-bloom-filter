package bloom

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"sync"
)

// MaxFilterSize 最大过滤器大小限制 (100MB)，防止反序列化时 OOM
const MaxFilterSize = 100 * 1024 * 1024

// MaxHashFunctions 最大哈希函数数量
const MaxHashFunctions = 20

// ErrCounterOverflow 计数器溢出错误
var ErrCounterOverflow = errors.New("counter overflow: maximum value 255 reached")

// ErrChecksumMismatch 校验和不匹配错误，表示数据已损坏
var ErrChecksumMismatch = errors.New("checksum mismatch: data may be corrupted")

// CountingBloomFilter implements a counting Bloom filter that supports deletions
// by using counters instead of single bits.
type CountingBloomFilter struct {
	m        int          // Number of counters
	k        int          // Number of hash functions
	counters []uint8      // Counter array (4 bits per counter is typical, using uint8 for simplicity)
	mu       sync.RWMutex // Mutex for thread-safe operations
}

// NewCountingBloomFilter creates a new counting Bloom filter with the specified
// size (m) and number of hash functions (k).
func NewCountingBloomFilter(m, k int) *CountingBloomFilter {
	return &CountingBloomFilter{
		m:        m,
		k:        k,
		counters: make([]uint8, m),
	}
}

// Add adds an item to the Bloom filter.
// Returns ErrCounterOverflow if any counter has reached its maximum value (255).
func (cbf *CountingBloomFilter) Add(item []byte) error {
	cbf.mu.Lock()
	defer cbf.mu.Unlock()

	indices := getHashIndices(item, cbf.m, cbf.k)
	for _, idx := range indices {
		if cbf.counters[idx] >= 255 {
			return ErrCounterOverflow
		}
		cbf.counters[idx]++
	}
	return nil
}

// Remove removes an item from the Bloom filter (decrements counters).
//
// ⚠️ SECURITY WARNING: This method can cause false negatives if:
//   - The same item was never added (malicious removal)
//   - There are hash collisions with other items
//
// For security-critical applications, consider:
//   - Tracking which items have been added before allowing removal
//   - Using an allowlist to validate removal requests
//   - Implementing audit logging for removal operations
//
// Note: This can cause false negatives if the same item was never added
// or if there are hash collisions.
func (cbf *CountingBloomFilter) Remove(item []byte) {
	cbf.mu.Lock()
	defer cbf.mu.Unlock()

	indices := getHashIndices(item, cbf.m, cbf.k)
	for _, idx := range indices {
		if cbf.counters[idx] > 0 {
			cbf.counters[idx]--
		}
	}
}

// BatchAdd adds multiple items to the Bloom filter efficiently.
// This is optimized for bulk operations with reduced lock contention.
// Returns results for each item: success count, failure count, and error messages.
func (cbf *CountingBloomFilter) BatchAdd(items [][]byte) (successCount, failureCount int, errors []string) {
	cbf.mu.Lock()
	defer cbf.mu.Unlock()

	errors = make([]string, len(items))
	successCount = 0
	failureCount = 0

	for i, item := range items {
		indices := getHashIndices(item, cbf.m, cbf.k)
		hasOverflow := false
		
		// First pass: check for overflow
		for _, idx := range indices {
			if cbf.counters[idx] >= 255 {
				hasOverflow = true
				break
			}
		}

		if hasOverflow {
			errors[i] = ErrCounterOverflow.Error()
			failureCount++
		} else {
			// Second pass: increment counters
			for _, idx := range indices {
				cbf.counters[idx]++
			}
			successCount++
			errors[i] = ""
		}
	}

	return successCount, failureCount, errors
}

// BatchRemove removes multiple items from the Bloom filter efficiently.
func (cbf *CountingBloomFilter) BatchRemove(items [][]byte) int {
	cbf.mu.Lock()
	defer cbf.mu.Unlock()

	removedCount := 0
	for _, item := range items {
		indices := getHashIndices(item, cbf.m, cbf.k)
		for _, idx := range indices {
			if cbf.counters[idx] > 0 {
				cbf.counters[idx]--
			}
		}
		removedCount++
	}

	return removedCount
}

// Contains checks if an item might be in the Bloom filter.
// Returns true if the item might be present (possible false positive),
// false if the item is definitely not present.
func (cbf *CountingBloomFilter) Contains(item []byte) bool {
	cbf.mu.RLock()
	defer cbf.mu.RUnlock()

	indices := getHashIndices(item, cbf.m, cbf.k)
	for _, idx := range indices {
		if cbf.counters[idx] == 0 {
			return false
		}
	}
	return true
}

// BatchContains checks if multiple items might be in the Bloom filter.
// This is optimized for bulk queries with reduced lock contention.
// Returns a slice of bool indicating presence for each item.
func (cbf *CountingBloomFilter) BatchContains(items [][]byte) []bool {
	cbf.mu.RLock()
	defer cbf.mu.RUnlock()

	results := make([]bool, len(items))
	for i, item := range items {
		indices := getHashIndices(item, cbf.m, cbf.k)
		found := true
		for _, idx := range indices {
			if cbf.counters[idx] == 0 {
				found = false
				break
			}
		}
		results[i] = found
	}

	return results
}

// Count returns the approximate count of how many times an item has been added.
// This is the minimum counter value across all hash positions.
func (cbf *CountingBloomFilter) Count(item []byte) uint8 {
	cbf.mu.RLock()
	defer cbf.mu.RUnlock()

	indices := getHashIndices(item, cbf.m, cbf.k)
	minCount := uint8(255)
	for _, idx := range indices {
		if cbf.counters[idx] < minCount {
			minCount = cbf.counters[idx]
		}
	}
	return minCount
}

// Reset clears all counters in the Bloom filter.
func (cbf *CountingBloomFilter) Reset() {
	cbf.mu.Lock()
	defer cbf.mu.Unlock()

	for i := range cbf.counters {
		cbf.counters[i] = 0
	}
}

// Size returns the number of counters in the filter.
func (cbf *CountingBloomFilter) Size() int {
	return cbf.m
}

// HashCount returns the number of hash functions used.
func (cbf *CountingBloomFilter) HashCount() int {
	return cbf.k
}

// Serialize returns a byte representation of the Bloom filter for persistence.
// Format: [m(4 bytes)][k(4 bytes)][counters(m bytes)][CRC32 checksum(4 bytes)]
func (cbf *CountingBloomFilter) Serialize() []byte {
	cbf.mu.RLock()
	defer cbf.mu.RUnlock()

	// Data without checksum: header (8 bytes) + counters
	dataLen := 8 + len(cbf.counters)
	data := make([]byte, dataLen+4) // +4 for CRC32 checksum
	
	binary.BigEndian.PutUint32(data[0:4], uint32(cbf.m))
	binary.BigEndian.PutUint32(data[4:8], uint32(cbf.k))
	copy(data[8:dataLen], cbf.counters)
	
	// Calculate CRC32 checksum of the data (excluding checksum itself)
	checksum := crc32.Checksum(data[:dataLen], crc32.IEEETable)
	binary.BigEndian.PutUint32(data[dataLen:dataLen+4], checksum)
	
	return data
}

// Deserialize loads a Bloom filter from a byte representation.
// Validates data size, parameters, and CRC32 checksum to prevent OOM, invalid configurations, and corrupted data.
// Expected format: [m(4 bytes)][k(4 bytes)][counters(m bytes)][CRC32 checksum(4 bytes)]
func Deserialize(data []byte) (*CountingBloomFilter, error) {
	// Minimum size: header (8) + checksum (4) = 12 bytes
	if len(data) < 12 {
		return nil, ErrInvalidData
	}

	m := int(binary.BigEndian.Uint32(data[0:4]))
	k := int(binary.BigEndian.Uint32(data[4:8]))

	// P1-1: 边界检查 - 防止恶意数据导致 OOM
	if m > MaxFilterSize {
		return nil, ErrInvalidData
	}

	// P1-1: 边界检查 - 验证哈希函数数量合理性
	if k < 1 || k > MaxHashFunctions {
		return nil, ErrInvalidData
	}

	// Expected total size: header (8) + counters (m) + checksum (4)
	expectedLen := 8 + m + 4
	if len(data) != expectedLen {
		return nil, ErrInvalidData
	}

	// Verify CRC32 checksum before using the data
	dataWithoutChecksum := data[:expectedLen-4]
	storedChecksum := binary.BigEndian.Uint32(data[expectedLen-4 : expectedLen])
	calculatedChecksum := crc32.Checksum(dataWithoutChecksum, crc32.IEEETable)
	
	if storedChecksum != calculatedChecksum {
		return nil, ErrChecksumMismatch
	}

	cbf := &CountingBloomFilter{
		m:        m,
		k:        k,
		counters: make([]uint8, m),
	}
	copy(cbf.counters, data[8:8+m])

	return cbf, nil
}
