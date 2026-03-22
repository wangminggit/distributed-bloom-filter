package raft

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

// TestBloomFSM_Apply tests FSM Apply method.
func TestBloomFSM_Apply(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	walEncryptor, _ := wal.NewWALEncryptor("")
	fsm, err := NewBloomFSM(bf, walEncryptor, t.TempDir())
	if err != nil {
		t.Fatalf("NewBloomFSM failed: %v", err)
	}

	// Create command using NewCommand
	cmd := NewCommand("add", []byte("test-item"))
	data, _ := cmd.Marshal()

	log := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  data,
	}

	result := fsm.Apply(log)
	// Result may be nil or error, just verify the operation worked
	_ = result

	// Verify item was added
	if !bf.Contains([]byte("test-item")) {
		t.Error("Expected item to be added")
	}
}

// TestBloomFSM_Apply_Remove tests FSM Apply with Remove command.
func TestBloomFSM_Apply_Remove(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	walEncryptor, _ := wal.NewWALEncryptor("")
	fsm, err := NewBloomFSM(bf, walEncryptor, t.TempDir())
	if err != nil {
		t.Fatalf("NewBloomFSM failed: %v", err)
	}

	// Add first
	bf.Add([]byte("to-remove"))

	// Remove command
	cmd := NewCommand("remove", []byte("to-remove"))
	data, _ := cmd.Marshal()

	log := &raft.Log{
		Index: 2,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  data,
	}

	result := fsm.Apply(log)
	_ = result

	// Verify item was removed
	if bf.Contains([]byte("to-remove")) {
		t.Error("Expected item to be removed")
	}
}

// TestBloomFSM_Apply_InvalidCommand tests invalid command handling.
func TestBloomFSM_Apply_InvalidCommand(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	walEncryptor, _ := wal.NewWALEncryptor("")
	fsm, err := NewBloomFSM(bf, walEncryptor, t.TempDir())
	if err != nil {
		t.Fatalf("NewBloomFSM failed: %v", err)
	}

	// Invalid log type
	log := &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogBarrier, // Not a command
	}

	result := fsm.Apply(log)
	if result == nil {
		t.Error("Expected non-nil result for invalid command")
	}
}

// TestBloomFSM_Snapshot tests FSM snapshot creation.
func TestBloomFSM_Snapshot(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	
	// Add some items
	for i := 0; i < 10; i++ {
		bf.Add([]byte{byte(i)})
	}

	walEncryptor, _ := wal.NewWALEncryptor("")
	fsm, err := NewBloomFSM(bf, walEncryptor, t.TempDir())
	if err != nil {
		t.Fatalf("NewBloomFSM failed: %v", err)
	}

	snapshot, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	if snapshot == nil {
		t.Fatal("Expected non-nil snapshot")
	}
}

// TestBloomFSM_Restore tests FSM restore from snapshot.
func TestBloomFSM_Restore(t *testing.T) {
	originalBF := bloom.NewCountingBloomFilter(1000, 3)
	
	// Add some items
	for i := 0; i < 10; i++ {
		originalBF.Add([]byte{byte(i)})
	}

	walEncryptor, _ := wal.NewWALEncryptor("")
	fsm, err := NewBloomFSM(originalBF, walEncryptor, t.TempDir())
	if err != nil {
		t.Fatalf("NewBloomFSM failed: %v", err)
	}

	// Create snapshot
	snapshot, _ := fsm.Snapshot()
	
	// Create buffer to store snapshot data
	var buf bytes.Buffer
	
	// Create mock sink
	sink := &mockSnapshotSink{
		Buffer: &buf,
	}
	
	// Persist snapshot
	err = snapshot.Persist(sink)
	if err != nil {
		t.Fatalf("Persist failed: %v", err)
	}
	snapshot.Release()

	// Create new FSM for restore
	newBF := bloom.NewCountingBloomFilter(1000, 3)
	fsm2, err := NewBloomFSM(newBF, walEncryptor, t.TempDir())
	if err != nil {
		t.Fatalf("NewBloomFSM failed: %v", err)
	}

	// Restore from buffer
	rc := io.NopCloser(&buf)
	err = fsm2.Restore(rc)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify data was restored
	for i := 0; i < 10; i++ {
		if !newBF.Contains([]byte{byte(i)}) {
			t.Errorf("Item %d should exist after restore", i)
		}
	}
}

// TestBloomFSM_GetBloomFilter tests Bloom filter getter.
func TestBloomFSM_GetBloomFilter(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	walEncryptor, _ := wal.NewWALEncryptor("")
	fsm, err := NewBloomFSM(bf, walEncryptor, t.TempDir())
	if err != nil {
		t.Fatalf("NewBloomFSM failed: %v", err)
	}

	gotBF := fsm.GetBloomFilter()
	if gotBF != bf {
		t.Error("Expected same Bloom filter instance")
	}
}

// TestBloomFSM_GetLastAppliedIndex tests last applied index getter.
func TestBloomFSM_GetLastAppliedIndex(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	walEncryptor, _ := wal.NewWALEncryptor("")
	fsm, err := NewBloomFSM(bf, walEncryptor, t.TempDir())
	if err != nil {
		t.Fatalf("NewBloomFSM failed: %v", err)
	}

	// Initially zero
	if fsm.GetLastAppliedIndex() != 0 {
		t.Errorf("Expected initial index 0, got %d", fsm.GetLastAppliedIndex())
	}
}

// TestBloomFSM_GetLastAppliedTerm tests last applied term getter.
func TestBloomFSM_GetLastAppliedTerm(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	walEncryptor, _ := wal.NewWALEncryptor("")
	fsm, err := NewBloomFSM(bf, walEncryptor, t.TempDir())
	if err != nil {
		t.Fatalf("NewBloomFSM failed: %v", err)
	}

	// Initially zero
	if fsm.GetLastAppliedTerm() != 0 {
		t.Errorf("Expected initial term 0, got %d", fsm.GetLastAppliedTerm())
	}
}

// TestBloomFSM_GetStats tests FSM statistics.
func TestBloomFSM_GetStats(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	walEncryptor, _ := wal.NewWALEncryptor("")
	fsm, err := NewBloomFSM(bf, walEncryptor, t.TempDir())
	if err != nil {
		t.Fatalf("NewBloomFSM failed: %v", err)
	}

	stats := fsm.GetStats()

	if stats == nil {
		t.Fatal("Expected non-nil stats")
	}
	if stats["bloom_size"] != 1000 {
		t.Errorf("Expected bloom_size 1000, got %v", stats["bloom_size"])
	}
	if stats["bloom_k"] != 3 {
		t.Errorf("Expected bloom_k 3, got %v", stats["bloom_k"])
	}
}

// TestBloomFSM_Close tests FSM close.
func TestBloomFSM_Close(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	walEncryptor, _ := wal.NewWALEncryptor("")
	fsm, err := NewBloomFSM(bf, walEncryptor, t.TempDir())
	if err != nil {
		t.Fatalf("NewBloomFSM failed: %v", err)
	}

	// Close should not panic
	err = fsm.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// TestBloomFSM_Snapshot_Release tests snapshot release.
func TestBloomFSM_Snapshot_Release(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	walEncryptor, _ := wal.NewWALEncryptor("")
	fsm, err := NewBloomFSM(bf, walEncryptor, t.TempDir())
	if err != nil {
		t.Fatalf("NewBloomFSM failed: %v", err)
	}

	snapshot, _ := fsm.Snapshot()
	
	// Release should not panic
	snapshot.Release()
}

// TestFsmSnapshot_Persist tests snapshot persist.
func TestFsmSnapshot_Persist(t *testing.T) {
	bf := bloom.NewCountingBloomFilter(1000, 3)
	walEncryptor, _ := wal.NewWALEncryptor("")
	fsm, err := NewBloomFSM(bf, walEncryptor, t.TempDir())
	if err != nil {
		t.Fatalf("NewBloomFSM failed: %v", err)
	}

	snapshot, _ := fsm.Snapshot()
	
	buf := &bytes.Buffer{}
	sink := &mockSnapshotSink{
		Buffer: buf,
	}

	err = snapshot.Persist(sink)
	if err != nil {
		t.Errorf("Persist failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("Expected non-zero snapshot data")
	}
}

// TestLogManager_BasicOperations tests basic log manager operations.
func TestLogManager_BasicOperations(t *testing.T) {
	lm := NewLogManager()

	if lm == nil {
		t.Fatal("Expected non-nil LogManager")
	}

	// GetLastIndex should return 0 initially
	idx, _ := lm.GetLastIndex()
	if idx != 0 {
		t.Errorf("Expected initial index 0, got %d", idx)
	}

	// GetFirstIndex should return 0 initially
	first, _ := lm.GetFirstIndex()
	if first != 0 {
		t.Errorf("Expected first index 0, got %d", first)
	}
}

// TestCommand_Marshal tests command marshaling.
func TestCommand_Marshal(t *testing.T) {
	cmd := NewCommand("add", []byte("test"))
	
	data, err := cmd.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	
	if len(data) == 0 {
		t.Error("Expected non-zero marshaled data")
	}
}

// TestCommand_Unmarshal tests command unmarshaling.
func TestCommand_Unmarshal(t *testing.T) {
	original := NewCommand("remove", []byte("test-item"))
	
	data, _ := original.Marshal()
	
	cmd, err := UnmarshalCommand(data)
	if err != nil {
		t.Fatalf("UnmarshalCommand failed: %v", err)
	}
	
	if cmd.Type != original.Type {
		t.Errorf("Expected type %s, got %s", original.Type, cmd.Type)
	}
	if string(cmd.Item) != string(original.Item) {
		t.Errorf("Expected item %s, got %s", string(original.Item), string(cmd.Item))
	}
}

// TestCommand_JSONMarshal tests command JSON marshaling.
func TestCommand_JSONMarshal(t *testing.T) {
	cmd := &Command{
		Type: "add",
		Item: []byte("test"),
	}
	
	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}
	
	if len(data) == 0 {
		t.Error("Expected non-zero JSON data")
	}
}

// mockSnapshotSink implements raft.SnapshotSink for testing.
type mockSnapshotSink struct {
	Buffer  *bytes.Buffer
	Closed  bool
	Canceled bool
}

func (m *mockSnapshotSink) Write(p []byte) (int, error) {
	return m.Buffer.Write(p)
}

func (m *mockSnapshotSink) Close() error {
	m.Closed = true
	return nil
}

func (m *mockSnapshotSink) ID() string {
	return "mock-snapshot"
}

func (m *mockSnapshotSink) Cancel() error {
	m.Canceled = true
	return nil
}
