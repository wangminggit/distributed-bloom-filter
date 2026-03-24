package raft

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

// TestNodeSnapshot tests the Node.Snapshot method.
func TestNodeSnapshot(t *testing.T) {
	// Create a minimal node for testing
	node := &Node{
		nodeID:   "test-node",
		raftPort: 9000,
		bloomFilter: bloom.NewCountingBloomFilter(1000, 3),
	}
	
	// Add some data to the bloom filter
	node.bloomFilter.Add([]byte("test-item"))
	
	// Call Snapshot
	fsmSnap, err := node.Snapshot()
	
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	
	if fsmSnap == nil {
		t.Fatal("Expected non-nil snapshot")
	}
	
	// Verify it's the right type
	snap, ok := fsmSnap.(*fsmSnapshot)
	if !ok {
		t.Fatal("Expected fsmSnapshot type")
	}
	
	if len(snap.bloomData) == 0 {
		t.Error("Expected non-empty bloom data in snapshot")
	}
	
	t.Logf("Snapshot created successfully (%d bytes)", len(snap.bloomData))
}

// TestNodeRestore tests the Node.Restore method.
func TestNodeRestore(t *testing.T) {
	// Create original node with data
	originalNode := &Node{
		nodeID:   "test-node",
		raftPort: 9000,
		bloomFilter: bloom.NewCountingBloomFilter(1000, 3),
	}
	originalNode.bloomFilter.Add([]byte("test-item-1"))
	originalNode.bloomFilter.Add([]byte("test-item-2"))
	
	// Serialize the bloom filter
	data := originalNode.bloomFilter.Serialize()
	
	// Create a new node to restore into
	newNode := &Node{
		nodeID:   "test-node",
		raftPort: 9000,
		bloomFilter: bloom.NewCountingBloomFilter(1000, 3),
	}
	
	// Create a read closer from the data
	rc := io.NopCloser(bytes.NewReader(data))
	
	// Restore
	err := newNode.Restore(rc)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
	
	// Verify data was restored
	if !newNode.bloomFilter.Contains([]byte("test-item-1")) {
		t.Error("Expected test-item-1 to be present after restore")
	}
	
	if !newNode.bloomFilter.Contains([]byte("test-item-2")) {
		t.Error("Expected test-item-2 to be present after restore")
	}
	
	t.Log("Node restore completed successfully")
}

// TestNodeRestoreInvalidData tests Node.Restore with invalid data.
func TestNodeRestoreInvalidData(t *testing.T) {
	node := &Node{
		nodeID:   "test-node",
		raftPort: 9000,
		bloomFilter: bloom.NewCountingBloomFilter(1000, 3),
	}
	
	// Invalid data
	invalidData := []byte("this is not valid bloom filter data")
	rc := io.NopCloser(bytes.NewReader(invalidData))
	
	err := node.Restore(rc)
	if err == nil {
		t.Error("Expected error when restoring invalid data")
	}
	
	t.Logf("Correctly detected invalid data: %v", err)
}

// TestNodeRestoreReadError tests Node.Restore with read error.
func TestNodeRestoreReadError(t *testing.T) {
	node := &Node{
		nodeID:   "test-node",
		raftPort: 9000,
		bloomFilter: bloom.NewCountingBloomFilter(1000, 3),
	}
	
	// Create a reader that will fail
	rc := &failingReadCloser{}
	
	err := node.Restore(rc)
	if err == nil {
		t.Error("Expected error when read fails")
	}
	
	t.Logf("Correctly detected read error: %v", err)
}

// TestNodeGetState tests the Node.GetState method.
func TestNodeGetState(t *testing.T) {
	node := &Node{
		nodeID:   "test-node",
		raftPort: 9000,
		bloomFilter: bloom.NewCountingBloomFilter(1000, 3),
	}
	
	// Add some data
	node.bloomFilter.Add([]byte("test"))
	
	state := node.GetState()
	
	// Verify state fields
	if state["node_id"] != "test-node" {
		t.Errorf("Expected node_id=test-node, got %v", state["node_id"])
	}
	
	if state["raft_port"] != 9000 {
		t.Errorf("Expected raft_port=9000, got %v", state["raft_port"])
	}
	
	if state["is_leader"] != false {
		t.Error("Expected is_leader=false (no raft node)")
	}
	
	if state["tls_enabled"] != false {
		t.Error("Expected tls_enabled=false")
	}
	
	bloomSize := state["bloom_size"].(int)
	if bloomSize <= 0 {
		t.Errorf("Expected positive bloom_size, got %d", bloomSize)
	}
	
	t.Logf("Node state: %v", state)
}

// TestNodeGetStateWithRaftNode tests GetState when raft node exists.
func TestNodeGetStateWithRaftNode(t *testing.T) {
	node := &Node{
		nodeID:   "test-node",
		raftPort: 9000,
		bloomFilter: bloom.NewCountingBloomFilter(1000, 3),
	}
	
	// Note: We can't easily create a real raft.Raft for testing,
	// so we skip the raft_state and leader fields verification
	state := node.GetState()
	
	if state["node_id"] != "test-node" {
		t.Errorf("Expected node_id=test-node, got %v", state["node_id"])
	}
	
	t.Logf("Node state (no raft): %v", state)
}

// TestTlsStreamLayer tests the tlsStreamLayer methods.
func TestTlsStreamLayer(t *testing.T) {
	layer := &tlsStreamLayer{
		tcpAddr: nil, // Will be set on first accept
	}
	
	// Test Addr with no listener
	addr := layer.Addr()
	if addr == nil {
		t.Error("Expected non-nil address")
	}
	
	// Test Close with no listener
	err := layer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
	
	t.Log("tlsStreamLayer basic methods tested")
}

// TestTlsStreamLayerDial tests the Dial method with invalid address.
func TestTlsStreamLayerDial(t *testing.T) {
	layer := &tlsStreamLayer{}
	
	// Try to dial an invalid address
	_, err := layer.Dial("invalid", 100*time.Millisecond)
	
	if err == nil {
		t.Error("Expected error when dialing invalid address")
	}
	
	t.Logf("Dial correctly failed: %v", err)
}

// TestTlsStreamLayerAcceptError tests Accept with invalid address.
func TestTlsStreamLayerAcceptError(t *testing.T) {
	addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	layer := &tlsStreamLayer{
		tcpAddr: addr,
	}
	
	// Accept should work but timeout quickly
	// We just test that the method doesn't panic
	done := make(chan error, 1)
	go func() {
		_, err := layer.Accept()
		done <- err
	}()
	
	// Give it a moment then close
	time.Sleep(100 * time.Millisecond)
	layer.Close()
	
	err := <-done
	t.Logf("Accept completed (may be closed error): %v", err)
}

// TestNodeSnapshotRestoreRoundTrip tests snapshot and restore round-trip.
func TestNodeSnapshotRestoreRoundTrip(t *testing.T) {
	// Create original node
	original := &Node{
		nodeID:   "test-node",
		raftPort: 9000,
		bloomFilter: bloom.NewCountingBloomFilter(10000, 5),
	}
	
	// Add multiple items
	items := [][]byte{
		[]byte("item1"), []byte("item2"), []byte("item3"),
		[]byte("item4"), []byte("item5"),
	}
	
	for _, item := range items {
		original.bloomFilter.Add(item)
	}
	
	// Create snapshot
	snap, err := original.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	
	// Create a buffer to simulate persistence
	var buf bytes.Buffer
	fsmSnap := snap.(*fsmSnapshot)
	buf.Write(fsmSnap.bloomData)
	
	// Create new node and restore
	newNode := &Node{
		nodeID:   "test-node",
		raftPort: 9000,
		bloomFilter: bloom.NewCountingBloomFilter(10000, 5),
	}
	
	rc := io.NopCloser(&buf)
	err = newNode.Restore(rc)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
	
	// Verify all items are present
	for _, item := range items {
		if !newNode.bloomFilter.Contains(item) {
			t.Errorf("Item %s not found after restore", string(item))
		}
	}
	
	t.Log("Snapshot/restore round-trip completed successfully")
}

// TestNodeConcurrentStateAccess tests concurrent access to GetState.
func TestNodeConcurrentStateAccess(t *testing.T) {
	node := &Node{
		nodeID:   "test-node",
		raftPort: 9000,
		bloomFilter: bloom.NewCountingBloomFilter(1000, 3),
	}
	
	done := make(chan bool)
	
	// Spawn multiple goroutines accessing state concurrently
	for i := 0; i < 10; i++ {
		go func() {
			state := node.GetState()
			if state["node_id"] != "test-node" {
				t.Error("State access returned incorrect data")
			}
			done <- true
		}()
	}
	
	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
	
	t.Log("Concurrent state access test completed")
}

// TestNodeSnapshotEmptyBloomFilter tests Snapshot with empty bloom filter.
func TestNodeSnapshotEmptyBloomFilter(t *testing.T) {
	node := &Node{
		nodeID:   "test-node",
		raftPort: 9000,
		bloomFilter: bloom.NewCountingBloomFilter(1000, 3),
	}
	
	snap, err := node.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}
	
	fsmSnap := snap.(*fsmSnapshot)
	if len(fsmSnap.bloomData) == 0 {
		t.Error("Expected non-empty snapshot data even for empty bloom filter")
	}
	
	t.Logf("Empty bloom filter snapshot: %d bytes", len(fsmSnap.bloomData))
}

// failingReadCloser is a mock that always fails on Read.
type failingReadCloser struct{}

func (f *failingReadCloser) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func (f *failingReadCloser) Close() error {
	return nil
}
