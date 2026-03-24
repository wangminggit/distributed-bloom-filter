package raft

import (
	"testing"

	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

// TestSnapshotManagerNew tests NewSnapshotManager.
func TestSnapshotManagerNew(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	sm := NewSnapshotManager(bf)
	
	if sm == nil {
		t.Fatal("Expected non-nil snapshot manager")
	}
	
	if sm.bloomFilter != bf {
		t.Error("Expected bloomFilter to be set")
	}
	
	t.Log("NewSnapshotManager test completed")
}

// TestSnapshotManagerNewWithEncryption tests NewSnapshotManagerWithEncryption.
func TestSnapshotManagerNewWithEncryption(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	tmpDir := t.TempDir()
	
	// Use empty secretPath to trigger test mode with random key
	encryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}
	
	sm := NewSnapshotManagerWithEncryption(bf, encryptor, tmpDir)
	
	if sm == nil {
		t.Fatal("Expected non-nil snapshot manager")
	}
	
	if sm.encryptor == nil {
		t.Error("Expected encryptor to be set")
	}
	
	t.Log("NewSnapshotManagerWithEncryption test completed")
}

// TestSnapshotManagerSetSnapshotStore tests SetSnapshotStore.
func TestSnapshotManagerSetSnapshotStore(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	sm := NewSnapshotManager(bf)
	
	// Set nil store
	sm.SetSnapshotStore(nil)
	
	// Should not panic
	t.Log("SetSnapshotStore completed")
}

// TestSnapshotManagerCreateSnapshotNoBloom tests CreateSnapshot without bloom filter.
func TestSnapshotManagerCreateSnapshotNoBloom(t *testing.T) {
	sm := NewSnapshotManager(nil)
	
	err := sm.CreateSnapshot(1, 1)
	
	if err != ErrBloomFilterNotInitialized {
		t.Errorf("Expected ErrBloomFilterNotInitialized, got %v", err)
	}
	
	t.Log("CreateSnapshot correctly rejected nil bloom filter")
}

// TestSnapshotManagerCreateSnapshot tests CreateSnapshot.
func TestSnapshotManagerCreateSnapshot(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	bf.Add([]byte("test-item"))
	
	sm := NewSnapshotManager(bf)
	
	err := sm.CreateSnapshot(1, 1)
	
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	
	t.Log("CreateSnapshot test completed")
}

// TestSnapshotManagerGetLastSnapshotIndex tests GetLastSnapshotIndex.
func TestSnapshotManagerGetLastSnapshotIndex(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	sm := NewSnapshotManager(bf)
	
	// Initial should be 0
	if sm.GetLastSnapshotIndex() != 0 {
		t.Error("Expected initial index 0")
	}
	
	// Create a snapshot
	sm.CreateSnapshot(100, 5)
	
	if sm.GetLastSnapshotIndex() != 100 {
		t.Errorf("Expected index 100, got %d", sm.GetLastSnapshotIndex())
	}
	
	t.Log("GetLastSnapshotIndex test completed")
}

// TestSnapshotManagerGetLastSnapshotTerm tests GetLastSnapshotTerm.
func TestSnapshotManagerGetLastSnapshotTerm(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	sm := NewSnapshotManager(bf)
	
	sm.CreateSnapshot(1, 5)
	
	if sm.GetLastSnapshotTerm() != 5 {
		t.Errorf("Expected term 5, got %d", sm.GetLastSnapshotTerm())
	}
	
	t.Log("GetLastSnapshotTerm test completed")
}

// TestSnapshotManagerGetLastSnapshotTime tests GetLastSnapshotTime.
func TestSnapshotManagerGetLastSnapshotTime(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	sm := NewSnapshotManager(bf)
	
	sm.CreateSnapshot(1, 1)
	
	if sm.GetLastSnapshotTime().IsZero() {
		t.Error("Expected non-zero snapshot time")
	}
	
	t.Log("GetLastSnapshotTime test completed")
}

// TestSnapshotManagerGetStats tests GetStats.
func TestSnapshotManagerGetStats(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	sm := NewSnapshotManager(bf)
	
	stats := sm.GetStats()
	
	// stats is a value type
	if stats.TotalSnapshots < 0 {
		t.Error("Expected non-negative TotalSnapshots")
	}
	
	t.Logf("Snapshot stats: %v", stats)
}

// TestSnapshotManagerGetSnapshotInfo tests GetSnapshotInfo.
func TestSnapshotManagerGetSnapshotInfo(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	sm := NewSnapshotManager(bf)
	
	info, err := sm.GetSnapshotInfo()
	
	if err != nil {
		t.Logf("GetSnapshotInfo returned: %v", err)
	}
	
	if info == nil {
		t.Log("GetSnapshotInfo returned nil (expected without snapshots)")
	}
	
	t.Log("GetSnapshotInfo test completed")
}

// TestSnapshotManagerDeleteOldSnapshots tests DeleteOldSnapshots.
func TestSnapshotManagerDeleteOldSnapshots(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	sm := NewSnapshotManager(bf)
	
	// Without snapshot dir, should handle gracefully
	err := sm.DeleteOldSnapshots(2)
	
	if err == nil {
		t.Log("DeleteOldSnapshots completed (no snapshots to delete)")
	} else {
		t.Logf("DeleteOldSnapshots returned: %v", err)
	}
}

// TestSnapshotManagerGetStatus tests GetStatus.
func TestSnapshotManagerGetStatus(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	sm := NewSnapshotManager(bf)
	
	status := sm.GetStatus()
	
	if status == nil {
		t.Error("Expected non-nil status")
	}
	
	t.Logf("Snapshot status: %v", status)
}

// TestSnapshotData tests SnapshotData structure.
func TestSnapshotData(t *testing.T) {
	data := &SnapshotData{
		BloomFilter: []byte("test-data"),
		Metadata:    map[string]interface{}{"version": "1.0"},
		Index:       100,
		Term:        5,
	}
	
	if len(data.BloomFilter) == 0 {
		t.Error("Expected non-empty BloomFilter")
	}
	
	t.Log("SnapshotData test completed")
}

// TestSnapshotStats tests SnapshotStats structure.
func TestSnapshotStats(t *testing.T) {
	stats := &SnapshotStats{
		TotalSnapshots:    10,
		TotalSnapshotSize: 1024,
		TotalRestores:     2,
	}
	
	if stats.TotalSnapshots != 10 {
		t.Errorf("Expected TotalSnapshots=10, got %d", stats.TotalSnapshots)
	}
	
	t.Log("SnapshotStats test completed")
}
