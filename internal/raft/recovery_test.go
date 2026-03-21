package raft

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

// setupFSM creates an isolated FSM for testing
func setupFSM(t *testing.T) (*BloomFSM, string) {
	t.Helper()
	
	tempDir := t.TempDir()
	walDir := filepath.Join(tempDir, "wal")
	
	bloomFilter := bloom.NewCountingBloomFilter(1000, 3)
	encryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create WAL encryptor: %v", err)
	}
	
	fsm, err := NewBloomFSM(bloomFilter, encryptor, walDir)
	if err != nil {
		t.Fatalf("Failed to create FSM: %v", err)
	}
	
	return fsm, walDir
}

// TestFSMApplyWALIntegration tests that FSM Apply operations are written to WAL.
func TestFSMApplyWALIntegration(t *testing.T) {
	fsm, walDir := setupFSM(t)
	defer fsm.Close()
	
	item := base64.StdEncoding.EncodeToString([]byte("test-item-1"))
	addCmd := []byte(`{"type":"add","item":"` + item + `","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`)
	raftLog := &raft.Log{
		Index: 1,
		Term:  1,
		Data:  addCmd,
	}
	
	result := fsm.Apply(raftLog)
	if result != nil {
		t.Errorf("Apply failed: %v", result)
	}
	
	if !fsm.GetBloomFilter().Contains([]byte("test-item-1")) {
		t.Error("Bloom filter should contain test-item-1 after Apply")
	}
	
	walFiles, err := filepath.Glob(filepath.Join(walDir, "*.wal.enc"))
	if err != nil {
		t.Fatalf("Failed to glob WAL files: %v", err)
	}
	
	if len(walFiles) == 0 {
		t.Error("WAL file should be created after Apply")
	}
}

// TestFSMSnapshotEncryption tests that snapshots are encrypted with AES-256-GCM.
func TestFSMSnapshotEncryption(t *testing.T) {
	tempDir := t.TempDir()
	snapshotDir := filepath.Join(tempDir, "snapshots")
	
	bloomFilter := bloom.NewCountingBloomFilter(1000, 3)
	bloomFilter.Add([]byte("item-1"))
	bloomFilter.Add([]byte("item-2"))
	
	encryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create WAL encryptor: %v", err)
	}
	
	sm := NewSnapshotManagerWithEncryption(bloomFilter, encryptor, snapshotDir)
	sm.lastSnapshotIndex = 100
	sm.lastSnapshotTerm = 5
	
	snapshotData := &SnapshotData{
		BloomFilter: bloomFilter.Serialize(),
		Metadata:    map[string]interface{}{"version": "1.0"},
		Timestamp:   time.Now(),
		Index:       100,
		Term:        5,
	}
	
	data, err := json.Marshal(snapshotData)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot data: %v", err)
	}
	
	err = sm.SaveSnapshot(data)
	if err != nil {
		t.Fatalf("Failed to save encrypted snapshot: %v", err)
	}
	
	files, err := filepath.Glob(filepath.Join(snapshotDir, "*.enc"))
	if err != nil {
		t.Fatalf("Failed to glob snapshot files: %v", err)
	}
	
	if len(files) == 0 {
		t.Fatal("Encrypted snapshot file should be created")
	}
	
	fileData, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("Failed to read snapshot file: %v", err)
	}
	
	if len(fileData) < 65 {
		t.Fatal("Snapshot file too short - should contain checksum")
	}
}

// TestSnapshotLoadAndVerify tests loading and verifying encrypted snapshots.
func TestSnapshotLoadAndVerify(t *testing.T) {
	tempDir := t.TempDir()
	snapshotDir := filepath.Join(tempDir, "snapshots")
	
	bloomFilter := bloom.NewCountingBloomFilter(1000, 3)
	testItems := []string{"item-1", "item-2", "item-3"}
	for _, item := range testItems {
		bloomFilter.Add([]byte(item))
	}
	
	encryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create WAL encryptor: %v", err)
	}
	
	sm := NewSnapshotManagerWithEncryption(bloomFilter, encryptor, snapshotDir)
	sm.lastSnapshotIndex = 200
	sm.lastSnapshotTerm = 10
	
	snapshotData := &SnapshotData{
		BloomFilter: bloomFilter.Serialize(),
		Metadata:    map[string]interface{}{},
		Timestamp:   time.Now(),
		Index:       200,
		Term:        10,
	}
	
	data, err := json.Marshal(snapshotData)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot data: %v", err)
	}
	
	if err := sm.SaveSnapshot(data); err != nil {
		t.Fatalf("Failed to save snapshot: %v", err)
	}
	
	loadedData, err := sm.LoadSnapshot()
	if err != nil {
		t.Fatalf("Failed to load snapshot: %v", err)
	}
	
	if len(loadedData) != len(data) {
		t.Errorf("Loaded data size mismatch: expected %d, got %d", len(data), len(loadedData))
	}
	
	t.Log("Snapshot checksum verification passed")
}

// TestSnapshotTamperingDetection tests that tampered snapshots are detected.
func TestSnapshotTamperingDetection(t *testing.T) {
	tempDir := t.TempDir()
	snapshotDir := filepath.Join(tempDir, "snapshots")
	
	bloomFilter := bloom.NewCountingBloomFilter(1000, 3)
	bloomFilter.Add([]byte("original-item"))
	
	encryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create WAL encryptor: %v", err)
	}
	
	sm := NewSnapshotManagerWithEncryption(bloomFilter, encryptor, snapshotDir)
	sm.lastSnapshotIndex = 300
	sm.lastSnapshotTerm = 15
	
	snapshotData := &SnapshotData{
		BloomFilter: bloomFilter.Serialize(),
		Metadata:    map[string]interface{}{},
		Timestamp:   time.Now(),
		Index:       300,
		Term:        15,
	}
	
	data, err := json.Marshal(snapshotData)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot data: %v", err)
	}
	
	if err := sm.SaveSnapshot(data); err != nil {
		t.Fatalf("Failed to save snapshot: %v", err)
	}
	
	files, err := filepath.Glob(filepath.Join(snapshotDir, "*.enc"))
	if err != nil {
		t.Fatalf("Failed to glob snapshot files: %v", err)
	}
	
	if len(files) == 0 {
		t.Fatal("Snapshot file not found")
	}
	
	fileData, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("Failed to read snapshot file: %v", err)
	}
	
	if len(fileData) > 100 {
		fileData[100] ^= 0xFF
	}
	
	if err := os.WriteFile(files[0], fileData, 0644); err != nil {
		t.Fatalf("Failed to write tampered file: %v", err)
	}
	
	_, err = sm.LoadSnapshot()
	if err == nil {
		t.Error("Expected error when loading tampered snapshot, got nil")
	} else {
		t.Logf("Correctly detected tampering: %v", err)
	}
}

// TestWALReplayAfterCrash tests WAL replay after a simulated crash.
func TestWALReplayAfterCrash(t *testing.T) {
	tempDir := t.TempDir()
	walDir := filepath.Join(tempDir, "wal")
	
	bloomFilter := bloom.NewCountingBloomFilter(1000, 3)
	
	encryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create WAL encryptor: %v", err)
	}
	
	fsm, err := NewBloomFSM(bloomFilter, encryptor, walDir)
	if err != nil {
		t.Fatalf("Failed to create FSM: %v", err)
	}
	
	operations := []struct {
		cmdType string
		item    string
		index   uint64
	}{
		{"add", "wal-item-1", 1},
		{"add", "wal-item-2", 2},
		{"add", "wal-item-3", 3},
		{"remove", "wal-item-1", 4},
	}
	
	for _, op := range operations {
		encodedItem := base64.StdEncoding.EncodeToString([]byte(op.item))
		cmd := []byte(`{"type":"` + op.cmdType + `","item":"` + encodedItem + `","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`)
		raftLog := &raft.Log{Index: op.index, Term: 1, Data: cmd}
		fsm.Apply(raftLog)
	}
	
	fsm.Close()
	
	newBloomFilter := bloom.NewCountingBloomFilter(1000, 3)
	
	reader, err := wal.NewWALReader(walDir, encryptor)
	if err != nil {
		t.Fatalf("Failed to create WAL reader: %v", err)
	}
	defer reader.Close()
	
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read WAL: %v", err)
	}
	
	for i, record := range records {
		var cmd FSMCommand
		if err := json.Unmarshal(record, &cmd); err != nil {
			t.Errorf("Failed to parse WAL record %d: %v", i, err)
			continue
		}
		
		switch cmd.Type {
		case "add":
			newBloomFilter.Add(cmd.Item)
		case "remove":
			newBloomFilter.Remove(cmd.Item)
		}
	}
	
	if newBloomFilter.Contains([]byte("wal-item-1")) {
		t.Error("wal-item-1 should have been removed")
	}
	
	if !newBloomFilter.Contains([]byte("wal-item-2")) {
		t.Error("wal-item-2 should exist after replay")
	}
	
	if !newBloomFilter.Contains([]byte("wal-item-3")) {
		t.Error("wal-item-3 should exist after replay")
	}
	
	t.Logf("Successfully replayed %d WAL operations", len(records))
}

// TestSnapshotManagerIsolation tests that each test gets isolated snapshot manager
func TestSnapshotManagerIsolation(t *testing.T) {
	tempDir := t.TempDir()
	snapshotDir := filepath.Join(tempDir, "snapshots")
	
	bloomFilter := bloom.NewCountingBloomFilter(1000, 3)
	encryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create WAL encryptor: %v", err)
	}
	
	sm := NewSnapshotManagerWithEncryption(bloomFilter, encryptor, snapshotDir)
	sm.lastSnapshotIndex = 500
	sm.lastSnapshotTerm = 25
	
	snapshotData := &SnapshotData{
		BloomFilter: bloomFilter.Serialize(),
		Metadata:    map[string]interface{}{"test": "isolation"},
		Timestamp:   time.Now(),
		Index:       500,
		Term:        25,
	}
	
	data, err := json.Marshal(snapshotData)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot data: %v", err)
	}
	
	if err := sm.SaveSnapshot(data); err != nil {
		t.Fatalf("Failed to save snapshot: %v", err)
	}
	
	loadedData, err := sm.LoadSnapshot()
	if err != nil {
		t.Fatalf("Failed to load snapshot: %v", err)
	}
	
	var loaded SnapshotData
	if err := json.Unmarshal(loadedData, &loaded); err != nil {
		t.Fatalf("Failed to unmarshal loaded data: %v", err)
	}
	
	if loaded.Metadata["test"] != "isolation" {
		t.Error("Snapshot metadata should match")
	}
	
	t.Log("Snapshot manager isolation test passed")
}
