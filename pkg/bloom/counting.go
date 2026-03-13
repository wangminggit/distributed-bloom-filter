package bloom

import (
	"encoding/binary"
	"errors"
	"sync"
)

// MaxFilterSize 最大过滤器大小限制 (100MB)，防止反序列化时 OOM
const MaxFilterSize = 100 * 1024 * 1024

// MaxHashFunctions 最大哈希函数数量
const MaxHashFunctions = 20

// ErrCounterOverflow 计数器溢出错误
var ErrCounterOverflow = errors.New("counter overflow: maximum value 255 reached")

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
func (cbf *CountingBloomFilter) Serialize() []byte {
	cbf.mu.RLock()
	defer cbf.mu.RUnlock()

	data := make([]byte, 8+len(cbf.counters))
	binary.BigEndian.PutUint32(data[0:4], uint32(cbf.m))
	binary.BigEndian.PutUint32(data[4:8], uint32(cbf.k))
	copy(data[8:], cbf.counters)
	return data
}

// Deserialize loads a Bloom filter from a byte representation.
// Validates data size and parameters to prevent OOM and invalid configurations.
func Deserialize(data []byte) (*CountingBloomFilter, error) {
	if len(data) < 8 {
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

	if len(data) < 8+m {
		return nil, ErrInvalidData
	}

	cbf := &CountingBloomFilter{
		m:        m,
		k:        k,
		counters: make([]uint8, m),
	}
	copy(cbf.counters, data[8:])

	return cbf, nil
}
