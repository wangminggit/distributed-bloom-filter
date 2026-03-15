package bloom

import (
	"encoding/binary"
	"testing"
)

func TestNewCountingBloomFilter(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)

	if cbf.m != 1000 {
		t.Errorf("Expected m=1000, got %d", cbf.m)
	}

	if cbf.k != 5 {
		t.Errorf("Expected k=5, got %d", cbf.k)
	}

	if len(cbf.counters) != 1000 {
		t.Errorf("Expected 1000 counters, got %d", len(cbf.counters))
	}
}

func TestAddAndContains(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)

	item := []byte("test-item")

	// Item should not be present initially
	if cbf.Contains(item) {
		t.Error("Item should not be present before adding")
	}

	// Add the item
	if err := cbf.Add(item); err != nil {
		t.Errorf("Add failed: %v", err)
	}

	// Item should now be present
	if !cbf.Contains(item) {
		t.Error("Item should be present after adding")
	}
}

func TestRemove(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)

	item := []byte("test-item")

	// Add the item twice
	if err := cbf.Add(item); err != nil {
		t.Errorf("Add failed: %v", err)
	}
	if err := cbf.Add(item); err != nil {
		t.Errorf("Add failed: %v", err)
	}

	// Count should be 2
	if count := cbf.Count(item); count != 2 {
		t.Errorf("Expected count=2, got %d", count)
	}

	// Remove once
	cbf.Remove(item)

	// Count should be 1
	if count := cbf.Count(item); count != 1 {
		t.Errorf("Expected count=1, got %d", count)
	}

	// Remove again
	cbf.Remove(item)

	// Item should not be present
	if cbf.Contains(item) {
		t.Error("Item should not be present after removing twice")
	}
}

func TestCount(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)

	item := []byte("test-item")

	// Add the item 5 times
	for i := 0; i < 5; i++ {
		if err := cbf.Add(item); err != nil {
			t.Errorf("Add failed: %v", err)
		}
	}

	// Count should be 5
	if count := cbf.Count(item); count != 5 {
		t.Errorf("Expected count=5, got %d", count)
	}
}

func TestReset(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)

	item := []byte("test-item")
	if err := cbf.Add(item); err != nil {
		t.Errorf("Add failed: %v", err)
	}
	if err := cbf.Add(item); err != nil {
		t.Errorf("Add failed: %v", err)
	}

	if !cbf.Contains(item) {
		t.Error("Item should be present before reset")
	}

	cbf.Reset()

	if cbf.Contains(item) {
		t.Error("Item should not be present after reset")
	}

	if count := cbf.Count(item); count != 0 {
		t.Errorf("Expected count=0 after reset, got %d", count)
	}
}

func TestSerializeDeserialize(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)

	item := []byte("test-item")
	if err := cbf.Add(item); err != nil {
		t.Errorf("Add failed: %v", err)
	}
	if err := cbf.Add(item); err != nil {
		t.Errorf("Add failed: %v", err)
	}
	if err := cbf.Add(item); err != nil {
		t.Errorf("Add failed: %v", err)
	}

	// Serialize
	data := cbf.Serialize()

	// Deserialize
	cbf2, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	// Check that the deserialized filter has the same state
	if cbf2.m != cbf.m {
		t.Errorf("Expected m=%d, got %d", cbf.m, cbf2.m)
	}

	if cbf2.k != cbf.k {
		t.Errorf("Expected k=%d, got %d", cbf.k, cbf2.k)
	}

	if !cbf2.Contains(item) {
		t.Error("Deserialized filter should contain the item")
	}

	if count := cbf2.Count(item); count != 3 {
		t.Errorf("Expected count=3, got %d", count)
	}
}

func TestDeserializeInvalidData(t *testing.T) {
	// Test with too short data
	_, err := Deserialize([]byte{1, 2, 3})
	if err != ErrInvalidData {
		t.Errorf("Expected ErrInvalidData, got %v", err)
	}

	// Test with incomplete counter data
	data := make([]byte, 12) // Header says 1000 counters but we only have 4 bytes
	data[0] = 0x00
	data[1] = 0x00
	data[2] = 0x03
	data[3] = 0xE8 // 1000 in big-endian
	data[4] = 0x00
	data[5] = 0x00
	data[6] = 0x00
	data[7] = 0x05 // 5 hash functions

	_, err = Deserialize(data)
	if err != ErrInvalidData {
		t.Errorf("Expected ErrInvalidData, got %v", err)
	}
}

func TestMultipleItems(t *testing.T) {
	cbf := NewCountingBloomFilter(10000, 5)

	items := [][]byte{
		[]byte("item1"),
		[]byte("item2"),
		[]byte("item3"),
		[]byte("item4"),
		[]byte("item5"),
	}

	// Add all items
	for _, item := range items {
		if err := cbf.Add(item); err != nil {
			t.Errorf("Add failed: %v", err)
		}
	}

	// All items should be present
	for _, item := range items {
		if !cbf.Contains(item) {
			t.Errorf("Item %s should be present", string(item))
		}
	}

	// Non-existent items should (mostly) not be present
	// Note: There's a small probability of false positives
	falsePositives := 0
	for i := 0; i < 100; i++ {
		nonExistent := []byte("non-existent-" + string(rune(i)))
		if cbf.Contains(nonExistent) {
			falsePositives++
		}
	}

	// Allow up to 5% false positives
	if falsePositives > 5 {
		t.Errorf("Too many false positives: %d/100", falsePositives)
	}
}

func TestConcurrency(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)

	item := []byte("concurrent-item")

	// Run concurrent adds (10 goroutines * 10 adds = 100 total, well under 255 limit)
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				if err := cbf.Add(item); err != nil {
					// Ignore overflow errors in concurrency test
					if err != ErrCounterOverflow {
						t.Errorf("Add failed: %v", err)
					}
				}
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Count should be 100 (10 goroutines * 10 adds)
	// Note: Due to counter saturation at 255, we keep the test under that limit
	if count := cbf.Count(item); count != 100 {
		t.Errorf("Expected count=100, got %d", count)
	}
}

func TestHashIndices(t *testing.T) {
	item := []byte("test-hash-indices")
	m := 1000
	k := 5

	indices := getHashIndices(item, m, k)

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
	indices2 := getHashIndices(item, m, k)
	for i := range indices {
		if indices[i] != indices2[i] {
			t.Errorf("Indices not deterministic: %v vs %v", indices, indices2)
		}
	}
}

// TestDeserialize_ChecksumVerification tests that corrupted data is detected via CRC32 checksum verification.
func TestDeserialize_ChecksumVerification(t *testing.T) {
	// Create a valid Bloom filter and serialize it
	cbf := NewCountingBloomFilter(1000, 5)
	item := []byte("test-item")
	if err := cbf.Add(item); err != nil {
		t.Errorf("Add failed: %v", err)
	}

	data := cbf.Serialize()

	// Test 1: Corrupt a byte in the counter data (early position)
	corruptedData := make([]byte, len(data))
	copy(corruptedData, data)
	corruptedData[10] ^= 0xFF // Flip bits in counter data

	_, err := Deserialize(corruptedData)
	if err != ErrChecksumMismatch {
		t.Errorf("Expected ErrChecksumMismatch for corrupted counter data, got: %v", err)
	}

	// Test 2: Corrupt a byte in the counter region (later position)
	corruptedData2 := make([]byte, len(data))
	copy(corruptedData2, data)
	corruptedData2[500] ^= 0xFF // Flip bits in counter data at different position

	_, err = Deserialize(corruptedData2)
	if err != ErrChecksumMismatch {
		t.Errorf("Expected ErrChecksumMismatch for corrupted counter data (position 500), got: %v", err)
	}

	// Test 3: Corrupt the checksum itself
	corruptedData3 := make([]byte, len(data))
	copy(corruptedData3, data)
	corruptedData3[len(data)-1] ^= 0xFF // Flip bits in checksum

	_, err = Deserialize(corruptedData3)
	if err != ErrChecksumMismatch {
		t.Errorf("Expected ErrChecksumMismatch for corrupted checksum, got: %v", err)
	}

	// Test 4: Truncate data (remove last byte of checksum)
	truncatedData := data[:len(data)-1]
	_, err = Deserialize(truncatedData)
	if err != ErrInvalidData {
		t.Errorf("Expected ErrInvalidData for truncated data, got: %v", err)
	}

	// Test 5: Valid data should still work
	cbf2, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Failed to deserialize valid data: %v", err)
	}
	if !cbf2.Contains(item) {
		t.Error("Deserialized filter should contain the item")
	}
}

// TestCounterOverflow tests P1-2: Bloom 计数器溢出处理
func TestCounterOverflow(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)

	item := []byte("overflow-test")

	// Add 255 times (should succeed)
	for i := 0; i < 255; i++ {
		if err := cbf.Add(item); err != nil {
			t.Errorf("Add failed at iteration %d: %v", i, err)
		}
	}

	// 256th add should fail with ErrCounterOverflow
	if err := cbf.Add(item); err != ErrCounterOverflow {
		t.Errorf("Expected ErrCounterOverflow on 256th add, got: %v", err)
	}

	// Count should still be 255
	if count := cbf.Count(item); count != 255 {
		t.Errorf("Expected count=255, got %d", count)
	}
}

// TestDeserializeMaxFilterSize tests P1-1: 反序列化边界检查 - 过滤器大小
func TestDeserializeMaxFilterSize(t *testing.T) {
	// Create data with size > MaxFilterSize
	data := make([]byte, 8)
	binary.BigEndian.PutUint32(data[0:4], uint32(MaxFilterSize+1)) // m > 100MB
	binary.BigEndian.PutUint32(data[4:8], 5)                       // k = 5

	_, err := Deserialize(data)
	if err != ErrInvalidData {
		t.Errorf("Expected ErrInvalidData for oversized filter, got: %v", err)
	}
}

// TestDeserializeInvalidK tests P1-1: 反序列化边界检查 - 哈希函数数量
func TestDeserializeInvalidK(t *testing.T) {
	tests := []struct {
		name string
		k    int
	}{
		{"k=0", 0},
		{"k=-1", -1},
		{"k=21", 21},
		{"k=100", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]byte, 8)
			binary.BigEndian.PutUint32(data[0:4], 1000)          // m = 1000
			binary.BigEndian.PutUint32(data[4:8], uint32(tt.k)) // invalid k

			_, err := Deserialize(data)
			if err != ErrInvalidData {
				t.Errorf("Expected ErrInvalidData for %s, got: %v", tt.name, err)
			}
		})
	}
}
