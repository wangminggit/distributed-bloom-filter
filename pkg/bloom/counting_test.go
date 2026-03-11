package bloom

import (
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
	cbf.Add(item)
	
	// Item should now be present
	if !cbf.Contains(item) {
		t.Error("Item should be present after adding")
	}
}

func TestRemove(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)
	
	item := []byte("test-item")
	
	// Add the item twice
	cbf.Add(item)
	cbf.Add(item)
	
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
		cbf.Add(item)
	}
	
	// Count should be 5
	if count := cbf.Count(item); count != 5 {
		t.Errorf("Expected count=5, got %d", count)
	}
}

func TestReset(t *testing.T) {
	cbf := NewCountingBloomFilter(1000, 5)
	
	item := []byte("test-item")
	cbf.Add(item)
	cbf.Add(item)
	
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
	cbf.Add(item)
	cbf.Add(item)
	cbf.Add(item)
	
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
		cbf.Add(item)
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
				cbf.Add(item)
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
