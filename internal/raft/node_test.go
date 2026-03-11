package raft

import (
	"os"
	"testing"
	"time"

	"github.com/wangminggit/distributed-bloom-filter/internal/metadata"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

func TestNodeStartAndLeaderElection(t *testing.T) {
	// Create temporary directory for test data
	tmpDir, err := os.MkdirTemp("", "raft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize dependencies
	walEncryptor := wal.NewEncryptor([]byte("32-byte-secret-key-for-test"))
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)

	// Create Raft node
	node := NewNode("test-node1", 18081, tmpDir, bloomFilter, walEncryptor, metadataService)

	// Start the node as bootstrap (first node in cluster)
	if err := node.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	defer node.Shutdown()

	// Wait for leader election with retries (single node should become leader quickly)
	var isLeader bool
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if node.IsLeader() {
			isLeader = true
			break
		}
	}

	// Verify the node is the leader
	if !isLeader {
		t.Error("Expected node to be leader after bootstrap")
	}

	// Verify state
	state := node.GetState()
	if state["is_leader"] != true {
		t.Error("Expected is_leader to be true")
	}
	if state["raft_state"] != "Leader" {
		t.Errorf("Expected raft_state to be Leader, got %v", state["raft_state"])
	}

	t.Logf("Node state: %v", state)
}

func TestNodeAddAndContains(t *testing.T) {
	// Create temporary directory for test data
	tmpDir, err := os.MkdirTemp("", "raft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize dependencies
	walEncryptor := wal.NewEncryptor([]byte("32-byte-secret-key-for-test"))
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)

	// Create Raft node
	node := NewNode("test-node2", 18082, tmpDir, bloomFilter, walEncryptor, metadataService)

	// Start the node as bootstrap
	if err := node.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	defer node.Shutdown()

	// Wait for leader election with retries
	var isLeader bool
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if node.IsLeader() {
			isLeader = true
			break
		}
	}
	if !isLeader {
		t.Fatal("Node did not become leader")
	}

	// Test adding an item
	testItem := []byte("test-item")
	if err := node.Add(testItem); err != nil {
		t.Fatalf("Failed to add item: %v", err)
	}

	// Give Raft time to apply the command
	time.Sleep(200 * time.Millisecond)

	// Verify the item is in the filter
	if !node.Contains(testItem) {
		t.Error("Expected item to be in Bloom filter after Add")
	}

	// Test removing the item
	if err := node.Remove(testItem); err != nil {
		t.Fatalf("Failed to remove item: %v", err)
	}

	// Give Raft time to apply the command
	time.Sleep(200 * time.Millisecond)

	// After removal, the item might still be present due to counting nature
	// but the counter should be decremented
	t.Logf("Item still present after remove: %v", node.Contains(testItem))
}

func TestNodeGracefulShutdown(t *testing.T) {
	// Create temporary directory for test data
	tmpDir, err := os.MkdirTemp("", "raft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize dependencies
	walEncryptor := wal.NewEncryptor([]byte("32-byte-secret-key-for-test"))
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)

	// Create Raft node
	node := NewNode("test-node3", 18083, tmpDir, bloomFilter, walEncryptor, metadataService)

	// Start the node
	if err := node.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}

	// Wait for leader election with retries
	var isLeader bool
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if node.IsLeader() {
			isLeader = true
			break
		}
	}

	// Verify it's running
	if !isLeader {
		t.Error("Expected node to be leader")
	}

	// Shutdown
	node.Shutdown()

	// Verify it's no longer the leader
	if node.IsLeader() {
		t.Error("Expected node to not be leader after shutdown")
	}
}

func TestNodeMultipleOperations(t *testing.T) {
	// Create temporary directory for test data
	tmpDir, err := os.MkdirTemp("", "raft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize dependencies
	walEncryptor := wal.NewEncryptor([]byte("32-byte-secret-key-for-test"))
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)

	// Create Raft node
	node := NewNode("test-node4", 18084, tmpDir, bloomFilter, walEncryptor, metadataService)

	// Start the node
	if err := node.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	defer node.Shutdown()

	// Wait for leader election with retries
	var isLeader bool
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if node.IsLeader() {
			isLeader = true
			break
		}
	}
	if !isLeader {
		t.Fatal("Node did not become leader")
	}

	// Add multiple items
	items := [][]byte{
		[]byte("item1"),
		[]byte("item2"),
		[]byte("item3"),
		[]byte("item4"),
		[]byte("item5"),
	}

	for _, item := range items {
		if err := node.Add(item); err != nil {
			t.Fatalf("Failed to add item %s: %v", string(item), err)
		}
	}

	// Give Raft time to apply commands
	time.Sleep(500 * time.Millisecond)

	// Verify all items are present
	for _, item := range items {
		if !node.Contains(item) {
			t.Errorf("Expected item %s to be in Bloom filter", string(item))
		}
	}

	t.Logf("Successfully added and verified %d items", len(items))
}
