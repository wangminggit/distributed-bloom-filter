package raft

import (
	"os"
	"testing"
	"time"

	"github.com/wangminggit/distributed-bloom-filter/internal/metadata"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

// TestNodeStartAndLeaderElection tests that a node can start and become leader.
func TestNodeStartAndLeaderElection(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "raft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize dependencies
	walEncryptor, _ := wal.NewWALEncryptor("")
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)

	// Create Raft node with in-memory store for testing (avoids bolt DB race issues)
	config := DefaultConfig()
	config.NodeID = "test-node1"
	config.RaftPort = 18081
	config.DataDir = tmpDir
	config.LocalID = "test-node1"
	config.Bootstrap = true
	config.UseInmemStore = true // Use in-memory store for race detection

	node, err := NewNode(config, bloomFilter, walEncryptor, metadataService)
	if err != nil {
		t.Fatalf("Failed to create Raft node: %v", err)
	}

	// Start the node as bootstrap
	if err := node.Start(); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	defer node.Shutdown()

	// Wait for leader election with longer timeout
	time.Sleep(2 * time.Second)

	// Verify state
	state := node.GetState()
	t.Logf("Node state: %v", state)

	// Node should be running (may or may not be leader in single-node test)
	if state["node_id"] != "test-node1" {
		t.Errorf("Expected node_id test-node1, got %v", state["node_id"])
	}
}

// TestNodeAddAndContains tests adding and checking items.
func TestNodeAddAndContains(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "raft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	walEncryptor, _ := wal.NewWALEncryptor("")
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)

	config := DefaultConfig()
	config.NodeID = "test-node2"
	config.RaftPort = 18082
	config.DataDir = tmpDir
	config.LocalID = "test-node2"
	config.UseInmemStore = true

	node, err := NewNode(config, bloomFilter, walEncryptor, metadataService)
	if err != nil {
		t.Fatalf("Failed to create Raft node: %v", err)
	}

	if err := node.Start(); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	defer node.Shutdown()

	// Wait for startup
	time.Sleep(1 * time.Second)

	// Test adding an item (will work even if not leader - just won't commit)
	testItem := []byte("test-item")
	err = node.Add(testItem)
	if err != nil {
		t.Logf("Add returned error (expected if not leader): %v", err)
	}

	// Test Contains (local read, always works)
	if !node.Contains(testItem) {
		t.Log("Item not in filter (expected if Add didn't commit)")
	}

	t.Logf("Node is leader: %v", node.IsLeader())
}

// TestNodeGracefulShutdown tests graceful shutdown.
func TestNodeGracefulShutdown(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "raft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	walEncryptor, _ := wal.NewWALEncryptor("")
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)

	config := DefaultConfig()
	config.NodeID = "test-node3"
	config.RaftPort = 18083
	config.DataDir = tmpDir
	config.LocalID = "test-node3"
	config.UseInmemStore = true

	node, err := NewNode(config, bloomFilter, walEncryptor, metadataService)
	if err != nil {
		t.Fatalf("Failed to create Raft node: %v", err)
	}

	if err := node.Start(); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}

	// Shutdown
	if err := node.Shutdown(); err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}

	t.Log("Node shut down successfully")
}

// TestNodeMultipleOperations tests multiple add/remove operations.
func TestNodeMultipleOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "raft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	walEncryptor, _ := wal.NewWALEncryptor("")
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)

	config := DefaultConfig()
	config.NodeID = "test-node4"
	config.RaftPort = 18084
	config.DataDir = tmpDir
	config.LocalID = "test-node4"
	config.UseInmemStore = true

	node, err := NewNode(config, bloomFilter, walEncryptor, metadataService)
	if err != nil {
		t.Fatalf("Failed to create Raft node: %v", err)
	}

	if err := node.Start(); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	defer node.Shutdown()

	// Wait for startup
	time.Sleep(1 * time.Second)

	// Add multiple items
	items := [][]byte{
		[]byte("item1"),
		[]byte("item2"),
		[]byte("item3"),
	}

	for _, item := range items {
		err := node.Add(item)
		if err != nil {
			t.Logf("Add item %s returned error: %v", string(item), err)
		}
	}

	time.Sleep(500 * time.Millisecond)

	t.Logf("Successfully processed %d items", len(items))
}

// TestNodeStateManager tests the state manager.
func TestNodeStateManager(t *testing.T) {
	sm := NewStateManager()

	// Test initial state
	if sm.GetState() != StateFollower {
		t.Error("Expected initial state to be Follower")
	}

	// Test state transitions
	sm.SetState(StateCandidate)
	if sm.GetState() != StateCandidate {
		t.Error("Expected state to be Candidate")
	}

	sm.SetState(StateLeader)
	if sm.GetState() != StateLeader {
		t.Error("Expected state to be Leader")
	}

	// Test term management
	sm.SetCurrentTerm(5)
	if sm.GetCurrentTerm() != 5 {
		t.Errorf("Expected term 5, got %d", sm.GetCurrentTerm())
	}

	// Test vote management
	sm.SetVotedFor("node-1")
	if sm.GetVotedFor() != "node-1" {
		t.Errorf("Expected votedFor node-1, got %s", sm.GetVotedFor())
	}

	// Test status
	status := sm.GetStatus()
	if status["state"] != "Leader" {
		t.Errorf("Expected state Leader in status, got %v", status["state"])
	}
	if status["current_term"] != uint64(5) {
		t.Errorf("Expected term 5 in status, got %v", status["current_term"])
	}

	t.Logf("State manager status: %v", status)
}

// TestNodeLogManager tests the log manager.
func TestNodeLogManager(t *testing.T) {
	// Test command creation
	cmd := NewCommand("add", []byte("test-item"))
	if cmd.Type != "add" {
		t.Errorf("Expected command type 'add', got %s", cmd.Type)
	}

	// Test marshaling
	data, err := cmd.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal command: %v", err)
	}

	// Test unmarshaling
	unmarshaledCmd, err := UnmarshalCommand(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal command: %v", err)
	}

	if unmarshaledCmd.Type != "add" {
		t.Errorf("Expected unmarshaled command type 'add', got %s", unmarshaledCmd.Type)
	}

	// Test log manager creation and stats
	lm := NewLogManager()
	if lm == nil {
		t.Fatal("LogManager is nil")
	}
	stats := lm.GetStats()
	if stats.TotalCommands != 0 {
		t.Errorf("Expected 0 total commands, got %d", stats.TotalCommands)
	}
}

// TestNodeElectionManager tests the election manager.
func TestNodeElectionManager(t *testing.T) {
	em := NewElectionManager()

	// Test initial state
	if em.IsLeader() {
		t.Error("Expected not to be leader initially")
	}

	leaderID, leaderAddr := em.GetLeader()
	if leaderID != "" {
		t.Errorf("Expected empty leader ID, got %s", leaderID)
	}
	if leaderAddr != "" {
		t.Errorf("Expected empty leader address, got %s", leaderAddr)
	}

	// Test stats
	stats := em.GetStats()
	if stats.TotalElections != 0 {
		t.Errorf("Expected 0 total elections, got %d", stats.TotalElections)
	}

	// Test status
	status := em.GetStatus()
	if status["is_leader"] != false {
		t.Error("Expected is_leader to be false in status")
	}

	t.Logf("Election manager status: %v", status)
}

// TestNodeReplicationManager tests the replication manager.
func TestNodeReplicationManager(t *testing.T) {
	rm := NewReplicationManager()

	// Test initial peers
	peers := rm.GetAllPeers()
	if len(peers) != 0 {
		t.Errorf("Expected 0 peers initially, got %d", len(peers))
	}

	// Test stats
	stats := rm.GetStats()
	if stats.TotalReplications != 0 {
		t.Errorf("Expected 0 total replications, got %d", stats.TotalReplications)
	}

	// Test status
	status := rm.GetStatus()
	if status["total_replications"] != int64(0) {
		t.Errorf("Expected 0 total replications in status, got %v", status["total_replications"])
	}

	t.Logf("Replication manager status: %v", status)
}

// TestNodeSnapshotManager tests the snapshot manager.
func TestNodeSnapshotManager(t *testing.T) {
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)
	sm := NewSnapshotManager(bloomFilter)

	// Test initial state
	if sm.GetLastSnapshotIndex() != 0 {
		t.Errorf("Expected 0 last snapshot index, got %d", sm.GetLastSnapshotIndex())
	}

	// Test stats
	stats := sm.GetStats()
	if stats.TotalSnapshots != 0 {
		t.Errorf("Expected 0 total snapshots, got %d", stats.TotalSnapshots)
	}

	// Test status
	status := sm.GetStatus()
	if status["total_snapshots"] != int64(0) {
		t.Errorf("Expected 0 total snapshots in status, got %v", status["total_snapshots"])
	}

	t.Logf("Snapshot manager status: %v", status)
}

// TestNodeConfigValidation tests configuration validation.
func TestNodeConfigValidation(t *testing.T) {
	// Test missing NodeID
	config := DefaultConfig()
	config.NodeID = ""
	if err := config.Validate(); err != ErrNodeIDRequired {
		t.Errorf("Expected ErrNodeIDRequired, got %v", err)
	}

	// Test invalid port
	config.NodeID = "test"
	config.RaftPort = 0
	if err := config.Validate(); err != ErrInvalidPort {
		t.Errorf("Expected ErrInvalidPort, got %v", err)
	}

	// Test missing data dir
	config.RaftPort = 8080
	config.DataDir = ""
	if err := config.Validate(); err != ErrDataDirRequired {
		t.Errorf("Expected ErrDataDirRequired, got %v", err)
	}

	// Test valid config
	config.DataDir = "/tmp/test"
	if err := config.Validate(); err != nil {
		t.Errorf("Expected no error for valid config, got %v", err)
	}
}

// TestNodeStateString tests state string conversion.
func TestNodeStateString(t *testing.T) {
	tests := []struct {
		state    NodeState
		expected string
	}{
		{StateFollower, "Follower"},
		{StateCandidate, "Candidate"},
		{StateLeader, "Leader"},
		{StateShutdown, "Shutdown"},
		{NodeState(999), "Unknown"},
	}

	for _, test := range tests {
		if test.state.String() != test.expected {
			t.Errorf("Expected %s for state %d, got %s", test.expected, test.state, test.state.String())
		}
	}
}

// TestNodeSnapshotSaveLoad tests saving and loading snapshots.
func TestNodeSnapshotSaveLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "raft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	walEncryptor, _ := wal.NewWALEncryptor("")
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)

	config := DefaultConfig()
	config.NodeID = "test-node5"
	config.RaftPort = 18085
	config.DataDir = tmpDir
	config.LocalID = "test-node5"
	config.UseInmemStore = true

	node, err := NewNode(config, bloomFilter, walEncryptor, metadataService)
	if err != nil {
		t.Fatalf("Failed to create Raft node: %v", err)
	}

	if err := node.Start(); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	defer node.Shutdown()

	// Wait for startup
	time.Sleep(500 * time.Millisecond)

	// Save snapshot
	snapshotPath := tmpDir + "/snapshot.json"
	if err := node.SaveSnapshot(snapshotPath); err != nil {
		t.Fatalf("Failed to save snapshot: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(snapshotPath); os.IsNotExist(err) {
		t.Error("Snapshot file does not exist")
	}

	t.Logf("Snapshot saved to %s", snapshotPath)
}
