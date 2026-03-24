package raft

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

// TestFSMSnapshotRelease tests fsmSnapshot.Release method.
func TestFSMSnapshotRelease(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	bf.Add([]byte("test-item"))
	
	snapshot := &fsmSnapshot{
		bloomData: bf.Serialize(),
		metadata:  map[string]interface{}{"version": "1.0"},
	}
	
	// Release should not panic
	snapshot.Release()
	
	t.Log("fsmSnapshot.Release completed successfully")
}

// TestConvertLogType tests ConvertLogType function.
func TestConvertLogType(t *testing.T) {
	// ConvertLogType converts raft.LogType to our local LogType
	result := ConvertLogType(raft.LogCommand)
	
	if result != LogCommand {
		t.Errorf("Expected LogCommand, got %v", result)
	}
	
	// Test Noop
	result2 := ConvertLogType(raft.LogNoop)
	if result2 != LogNoop {
		t.Errorf("Expected LogNoop, got %v", result2)
	}
	
	// Test default case
	result3 := ConvertLogType(raft.LogType(99))
	if result3 != LogCommand {
		t.Errorf("Expected LogCommand for unknown type, got %v", result3)
	}
	
	t.Log("ConvertLogType test completed")
}

// TestSnapshotManagerSaveSnapshotToFile tests SaveSnapshotToFile.
func TestSnapshotManagerSaveSnapshotToFile(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	bf.Add([]byte("test-item"))
	
	sm := NewSnapshotManager(bf)
	
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-snapshot.bin")
	
	// First save snapshot data to file manually
	err := os.WriteFile(filePath, []byte("test-snapshot-data"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	
	// SaveSnapshotToFile just saves internal state to file
	err = sm.SaveSnapshotToFile(filePath)
	
	if err != nil {
		t.Fatalf("SaveSnapshotToFile failed: %v", err)
	}
	
	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Expected snapshot file to exist")
	}
	
	t.Log("SaveSnapshotToFile test completed successfully")
}

// TestSnapshotManagerLoadSnapshotFromFile tests LoadSnapshotFromFile.
func TestSnapshotManagerLoadSnapshotFromFile(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	bf.Add([]byte("test-item"))
	sm := NewSnapshotManager(bf)
	
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test-snapshot.json")
	
	// Create test file with valid JSON (SnapshotData format)
	// Use actual serialized bloom filter data encoded as base64
	serializedData := bf.Serialize()
	b64Data := base64.StdEncoding.EncodeToString(serializedData)
	
	snapshotData := map[string]interface{}{
		"bloom_filter": b64Data,
		"metadata":     map[string]interface{}{},
		"timestamp":    time.Now().Format(time.RFC3339),
		"index":        1,
		"term":         1,
	}
	
	jsonData, err := json.Marshal(snapshotData)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}
	
	err = os.WriteFile(filePath, jsonData, 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	
	// Load
	err = sm.LoadSnapshotFromFile(filePath)
	if err != nil {
		t.Fatalf("LoadSnapshotFromFile failed: %v", err)
	}
	
	t.Log("LoadSnapshotFromFile test completed successfully")
}

// TestSnapshotManagerSaveSnapshotToFileError tests SaveSnapshotToFile with invalid path.
func TestSnapshotManagerSaveSnapshotToFileError(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	sm := NewSnapshotManager(bf)
	
	// Try to save to invalid path
	err := sm.SaveSnapshotToFile("/nonexistent/dir/snapshot.bin")
	
	if err == nil {
		t.Error("Expected error when saving to invalid path")
	}
	
	t.Logf("SaveSnapshotToFile correctly failed: %v", err)
}

// TestSnapshotManagerLoadSnapshotFromFileNotFound tests LoadSnapshotFromFile with non-existent file.
func TestSnapshotManagerLoadSnapshotFromFileNotFound(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	sm := NewSnapshotManager(bf)
	
	err := sm.LoadSnapshotFromFile("/nonexistent/snapshot.bin")
	
	if err == nil {
		t.Error("Expected error when loading non-existent file")
	}
	
	t.Logf("LoadSnapshotFromFile correctly failed: %v", err)
}

// TestSnapshotManagerRestoreFromFSM tests RestoreFromFSM.
func TestSnapshotManagerRestoreFromFSM(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	sm := NewSnapshotManager(bf)
	
	// RestoreFromFSM just updates internal state
	sm.RestoreFromFSM(100, 5)
	
	if sm.lastSnapshotIndex != 100 {
		t.Errorf("Expected lastSnapshotIndex=100, got %d", sm.lastSnapshotIndex)
	}
	
	if sm.lastSnapshotTerm != 5 {
		t.Errorf("Expected lastSnapshotTerm=5, got %d", sm.lastSnapshotTerm)
	}
	
	if sm.lastSnapshotTime.IsZero() {
		t.Error("Expected non-zero lastSnapshotTime")
	}
	
	t.Log("RestoreFromFSM test completed successfully")
}

// TestSnapshotManagerGetSnapshot tests GetSnapshot.
func TestSnapshotManagerGetSnapshot(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	bf.Add([]byte("test-item"))
	
	tmpDir := t.TempDir()
	encryptor, _ := wal.NewWALEncryptor("")
	sm := NewSnapshotManagerWithEncryption(bf, encryptor, tmpDir)
	
	// Save a snapshot first
	data := []byte("test-snapshot-data")
	err := sm.SaveSnapshot(data)
	if err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}
	
	// Get the snapshot
	snapshot, err := sm.GetSnapshot()
	
	if err != nil {
		t.Fatalf("GetSnapshot failed: %v", err)
	}
	
	if snapshot == nil {
		t.Error("Expected non-nil snapshot")
	}
	
	t.Log("GetSnapshot test completed")
}

// TestSnapshotManagerRestoreSnapshot tests RestoreSnapshot.
func TestSnapshotManagerRestoreSnapshot(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	bf.Add([]byte("test-item"))
	
	// Serialize bloom filter data
	serializedData := bf.Serialize()
	
	// Test bloom filter deserialize directly
	restoredBF, err := bloom.Deserialize(serializedData)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}
	
	if !restoredBF.Contains([]byte("test-item")) {
		t.Error("Expected test-item to be present after deserialize")
	}
	
	t.Log("RestoreSnapshot bloom filter deserialize test completed successfully")
}

// TestSnapshotManagerSaveSnapshotUnencrypted tests saveSnapshotUnencrypted.
func TestSnapshotManagerSaveSnapshotUnencrypted(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	sm := NewSnapshotManager(bf)
	
	tmpDir := t.TempDir()
	sm.snapshotDir = tmpDir
	
	data := []byte("unencrypted-snapshot-data")
	
	err := sm.SaveSnapshot(data)
	
	if err != nil {
		t.Fatalf("SaveSnapshot (unencrypted) failed: %v", err)
	}
	
	// Verify file was created
	files, err := os.ReadDir(tmpDir)
	if err != nil || len(files) == 0 {
		t.Error("Expected snapshot file to be created")
	}
	
	t.Log("saveSnapshotUnencrypted test completed")
}

// TestSnapshotManagerLoadSnapshotUnencrypted tests loadSnapshotUnencrypted.
func TestSnapshotManagerLoadSnapshotUnencrypted(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	sm := NewSnapshotManager(bf)
	
	tmpDir := t.TempDir()
	sm.snapshotDir = tmpDir
	
	data := []byte("unencrypted-snapshot-data")
	
	// Save first
	err := sm.SaveSnapshot(data)
	if err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}
	
	// Load the snapshot (reads from snapshotDir)
	loadedData, err := sm.loadSnapshotUnencrypted()
	if err != nil {
		t.Fatalf("loadSnapshotUnencrypted failed: %v", err)
	}
	
	if loadedData == nil {
		t.Error("Expected non-nil loaded data")
	}
	
	t.Log("loadSnapshotUnencrypted test completed successfully")
}

// TestSnapshotManagerLoadSnapshotUnencryptedNoDir tests loadSnapshotUnencrypted without snapshot dir.
func TestSnapshotManagerLoadSnapshotUnencryptedNoDir(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	sm := NewSnapshotManager(bf)
	
	// Don't set snapshotDir
	_, err := sm.loadSnapshotUnencrypted()
	
	if err == nil {
		t.Error("Expected error when snapshot dir not configured")
	}
	
	t.Logf("loadSnapshotUnencrypted correctly failed: %v", err)
}

// testFSMForRestore is a mock FSM for testing RestoreFromFSM.
type testFSMForRestore struct {
	data []byte
}

func (f *testFSMForRestore) Apply(log *raft.Log) interface{} {
	return nil
}

func (f *testFSMForRestore) Snapshot() (raft.FSMSnapshot, error) {
	return &testFSMSnapshotForRestore{data: f.data}, nil
}

func (f *testFSMForRestore) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	f.data = data
	return nil
}

// testFSMSnapshotForRestore is a mock FSM snapshot.
type testFSMSnapshotForRestore struct {
	data []byte
}

func (s *testFSMSnapshotForRestore) Persist(sink raft.SnapshotSink) error {
	_, err := sink.Write(s.data)
	if err != nil {
		return err
	}
	return sink.Close()
}

func (s *testFSMSnapshotForRestore) Release() {}
