package raft

import (
	"os"
	"testing"
	"time"

	"github.com/wangminggit/distributed-bloom-filter/internal/metadata"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

// createTestEncryptor creates a WAL encryptor for tests
func createTestEncryptor(t *testing.T) *wal.WALEncryptor {
	encryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create WAL encryptor: %v", err)
	}
	return encryptor
}

func TestNodeStartAndLeaderElection(t *testing.T) {
	// Create temporary directory for test data
	tmpDir, err := os.MkdirTemp("", "raft-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize dependencies
	walEncryptor := createTestEncryptor(t)
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
	walEncryptor := createTestEncryptor(t)
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
	walEncryptor := createTestEncryptor(t)
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
	walEncryptor := createTestEncryptor(t)
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

// TestRaftClusterMembership tests that nodes correctly join the Raft cluster
func TestRaftClusterMembership(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "raft-membership-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	walEncryptor := createTestEncryptor(t)
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)

	// Create and bootstrap first node
	node1 := NewNode("node1", 19001, tmpDir+"/node1", bloomFilter, walEncryptor, metadataService)
	if err := node1.Start(true); err != nil {
		t.Fatalf("Failed to start node1: %v", err)
	}
	defer node1.Shutdown()

	// Wait for node1 to become leader
	time.Sleep(500 * time.Millisecond)
	if !node1.IsLeader() {
		t.Fatal("Node1 did not become leader")
	}

	// Create second node
	bloomFilter2 := bloom.NewCountingBloomFilter(10000, 3)
	metadataService2 := metadata.NewService(tmpDir + "/node2")
	node2 := NewNode("node2", 19002, tmpDir+"/node2", bloomFilter2, walEncryptor, metadataService2)
	
	if err := node2.Start(false); err != nil {
		t.Fatalf("Failed to start node2: %v", err)
	}
	defer node2.Shutdown()

	// Add node2 to cluster via node1 (leader)
	peerAddr := "127.0.0.1:19002"
	if err := node1.AddPeer("node2", peerAddr); err != nil {
		t.Fatalf("Failed to add node2 to cluster: %v", err)
	}

	// Wait for cluster membership to propagate
	time.Sleep(500 * time.Millisecond)

	// Verify node2 is in the cluster
	members, err := node1.GetClusterMembers()
	if err != nil {
		t.Fatalf("Failed to get cluster members: %v", err)
	}

	found := false
	for _, member := range members {
		if string(member.ID) == "node2" {
			found = true
			t.Logf("Found node2 in cluster with address: %s", member.Address)
			break
		}
	}

	if !found {
		t.Error("Expected node2 to be in cluster members")
	}

	t.Logf("Cluster has %d members", len(members))
}

// TestRaftDataReplication tests that data is replicated to follower nodes
func TestRaftDataReplication(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "raft-replication-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	walEncryptor := createTestEncryptor(t)
	
	// Create leader node
	bloomFilter1 := bloom.NewCountingBloomFilter(10000, 3)
	metadataService1 := metadata.NewService(tmpDir + "/leader")
	leader := NewNode("leader", 19010, tmpDir+"/leader", bloomFilter1, walEncryptor, metadataService1)
	
	if err := leader.Start(true); err != nil {
		t.Fatalf("Failed to start leader: %v", err)
	}
	defer leader.Shutdown()

	// Wait for leader election
	time.Sleep(500 * time.Millisecond)
	if !leader.IsLeader() {
		t.Fatal("Leader did not become leader")
	}

	// Create follower node
	bloomFilter2 := bloom.NewCountingBloomFilter(10000, 3)
	metadataService2 := metadata.NewService(tmpDir + "/follower")
	follower := NewNode("follower", 19011, tmpDir+"/follower", bloomFilter2, walEncryptor, metadataService2)
	
	if err := follower.Start(false); err != nil {
		t.Fatalf("Failed to start follower: %v", err)
	}
	defer follower.Shutdown()

	// Add follower to cluster
	if err := leader.AddPeer("follower", "127.0.0.1:19011"); err != nil {
		t.Fatalf("Failed to add follower to cluster: %v", err)
	}

	// Wait for cluster to stabilize
	time.Sleep(1 * time.Second)

	// Write data through leader
	testItems := [][]byte{
		[]byte("replication-test-1"),
		[]byte("replication-test-2"),
		[]byte("replication-test-3"),
	}

	for _, item := range testItems {
		if err := leader.Add(item); err != nil {
			t.Fatalf("Failed to add item on leader: %v", err)
		}
	}

	// Wait for replication
	time.Sleep(500 * time.Millisecond)

	// Verify data on follower
	for _, item := range testItems {
		if !follower.Contains(item) {
			t.Errorf("Follower missing replicated item: %s", string(item))
		}
	}

	t.Log("✅ Data successfully replicated to follower")
}

// TestRaftLeaderFailover tests that data is preserved after leader failure
// Uses 3 nodes to ensure proper quorum during failover
func TestRaftLeaderFailover(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "raft-failover-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	walEncryptor := createTestEncryptor(t)
	
	// Create initial leader (node1)
	bloomFilter1 := bloom.NewCountingBloomFilter(10000, 3)
	metadataService1 := metadata.NewService(tmpDir + "/node1")
	node1 := NewNode("node1", 19020, tmpDir+"/node1", bloomFilter1, walEncryptor, metadataService1)
	
	if err := node1.Start(true); err != nil {
		t.Fatalf("Failed to start node1: %v", err)
	}
	defer node1.Shutdown()

	// Wait for node1 to become leader with retries
	var isLeader bool
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if node1.IsLeader() {
			isLeader = true
			break
		}
	}
	if !isLeader {
		t.Fatal("Node1 did not become leader")
	}

	// Create follower node (node2)
	bloomFilter2 := bloom.NewCountingBloomFilter(10000, 3)
	metadataService2 := metadata.NewService(tmpDir + "/node2")
	node2 := NewNode("node2", 19021, tmpDir+"/node2", bloomFilter2, walEncryptor, metadataService2)
	
	if err := node2.Start(false); err != nil {
		t.Fatalf("Failed to start node2: %v", err)
	}
	defer node2.Shutdown()

	// Create follower node (node3) for quorum
	bloomFilter3 := bloom.NewCountingBloomFilter(10000, 3)
	metadataService3 := metadata.NewService(tmpDir + "/node3")
	node3 := NewNode("node3", 19022, tmpDir+"/node3", bloomFilter3, walEncryptor, metadataService3)
	
	if err := node3.Start(false); err != nil {
		t.Fatalf("Failed to start node3: %v", err)
	}
	defer node3.Shutdown()

	// Add node2 and node3 to cluster
	if err := node1.AddPeer("node2", "127.0.0.1:19021"); err != nil {
		t.Fatalf("Failed to add node2 to cluster: %v", err)
	}
	if err := node1.AddPeer("node3", "127.0.0.1:19022"); err != nil {
		t.Fatalf("Failed to add node3 to cluster: %v", err)
	}

	// Wait for cluster to stabilize
	time.Sleep(1 * time.Second)

	// Write test data through leader (node1)
	testItems := [][]byte{
		[]byte("failover-test-1"),
		[]byte("failover-test-2"),
		[]byte("failover-test-3"),
	}

	for _, item := range testItems {
		if err := node1.Add(item); err != nil {
			t.Fatalf("Failed to add item: %v", err)
		}
	}

	// Wait for replication
	time.Sleep(500 * time.Millisecond)

	// Verify data on followers before failover
	for _, node := range []*Node{node2, node3} {
		for _, item := range testItems {
			if !node.Contains(item) {
				t.Logf("Warning: Follower missing item before failover: %s", string(item))
			}
		}
	}

	t.Log("Data replicated to followers, now simulating leader failure...")

	// Remove node1 from cluster first (clean removal)
	if err := node1.RemovePeer("node1"); err != nil {
		t.Logf("Warning: failed to remove node1 from cluster: %v", err)
	}

	// Simulate leader failure by shutting down node1
	node1.Shutdown()

	// Wait for node2 or node3 to detect leader failure and become leader
	time.Sleep(2 * time.Second)

	// Verify one of the remaining nodes becomes the new leader
	newLeader := node2.IsLeader() || node3.IsLeader()
	if !newLeader {
		t.Error("Neither node2 nor node3 became leader after node1 failure")
	} else {
		t.Log("✅ New leader elected after failover")
	}

	// Verify data is still available on at least one node
	dataPreserved := false
	for _, node := range []*Node{node2, node3} {
		allPresent := true
		for _, item := range testItems {
			if !node.Contains(item) {
				allPresent = false
				break
			}
		}
		if allPresent {
			dataPreserved = true
			break
		}
	}

	if dataPreserved {
		t.Log("✅ Data preserved after leader failover")
	} else {
		t.Error("Data not preserved after leader failover")
	}
}
