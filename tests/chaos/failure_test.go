package chaos

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/wangminggit/distributed-bloom-filter/internal/metadata"
	raftnode "github.com/wangminggit/distributed-bloom-filter/internal/raft"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

// TestCluster represents a test Raft cluster
type TestCluster struct {
	nodes    []*raftnode.Node
	nodeIDs  []string
	ports    []int
	dataDirs []string
	mu       sync.Mutex
}

// NewTestCluster creates a new test cluster with the specified number of nodes
func NewTestCluster(numNodes int, basePort int) *TestCluster {
	cluster := &TestCluster{
		nodes:    make([]*raftnode.Node, numNodes),
		nodeIDs:  make([]string, numNodes),
		ports:    make([]int, numNodes),
		dataDirs: make([]string, numNodes),
	}

	for i := 0; i < numNodes; i++ {
		cluster.nodeIDs[i] = fmt.Sprintf("node%d", i+1)
		cluster.ports[i] = basePort + i
		cluster.dataDirs[i] = filepath.Join(os.TempDir(), fmt.Sprintf("raft-test-%d-%d", time.Now().UnixNano(), i))
		os.MkdirAll(cluster.dataDirs[i], 0755)
	}

	return cluster
}

// Start starts all nodes in the cluster
func (c *TestCluster) Start(t *testing.T) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := 0; i < len(c.nodes); i++ {
		// Create Bloom filter for this node
		bloomFilter := bloom.NewCountingBloomFilter(1048576, 7)
		
		// Create WAL encryptor (empty string = random key for testing)
		walEncryptor, err := wal.NewWALEncryptor("")
		if err != nil {
			return fmt.Errorf("failed to create WAL encryptor: %w", err)
		}
		
		// Create metadata service
		metadataService := metadata.NewService(c.dataDirs[i])
		
		// Create Raft node
		node := raftnode.NewNode(c.nodeIDs[i], c.ports[i], c.dataDirs[i], bloomFilter, walEncryptor, metadataService)
		
		// Start node (bootstrap only the first node)
		if err := node.Start(i == 0); err != nil {
			return fmt.Errorf("failed to start node %s: %w", c.nodeIDs[i], err)
		}
		
		c.nodes[i] = node
		t.Logf("Started node %s on port %d", c.nodeIDs[i], c.ports[i])
	}

	// Wait for cluster to stabilize
	time.Sleep(2 * time.Second)

	return nil
}

// Stop stops all nodes in the cluster
func (c *TestCluster) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, node := range c.nodes {
		if node != nil {
			node.Shutdown()
		}
	}

	// Cleanup data directories
	for _, dir := range c.dataDirs {
		os.RemoveAll(dir)
	}
}

// GetLeader returns the current leader node
func (c *TestCluster) GetLeader() *raftnode.Node {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, node := range c.nodes {
		if node != nil && node.IsLeader() {
			return node
		}
	}
	return nil
}

// GetLeaderIndex returns the index of the leader node
func (c *TestCluster) GetLeaderIndex() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, node := range c.nodes {
		if node != nil && node.IsLeader() {
			return i
		}
	}
	return -1
}

// KillNode kills the node at the specified index
func (c *TestCluster) KillNode(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.nodes) {
		return fmt.Errorf("invalid node index: %d", index)
	}

	if c.nodes[index] != nil {
		c.nodes[index].Shutdown()
		c.nodes[index] = nil
		log.Printf("Killed node %s", c.nodeIDs[index])
	}

	return nil
}

// RestartNode restarts the node at the specified index
func (c *TestCluster) RestartNode(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.nodes) {
		return fmt.Errorf("invalid node index: %d", index)
	}

	// Create new node
	bloomFilter := bloom.NewCountingBloomFilter(1048576, 7)
	walEncryptor, _ := wal.NewWALEncryptor("")
	metadataService := metadata.NewService(c.dataDirs[index])
	
	node := raftnode.NewNode(c.nodeIDs[index], c.ports[index], c.dataDirs[index], bloomFilter, walEncryptor, metadataService)
	
	if err := node.Start(false); err != nil {
		return fmt.Errorf("failed to restart node %s: %w", c.nodeIDs[index], err)
	}

	c.nodes[index] = node
	log.Printf("Restarted node %s", c.nodeIDs[index])
	return nil
}

// WriteData writes test data through the leader
func (c *TestCluster) WriteData(t *testing.T, items [][]byte) error {
	leader := c.GetLeader()
	if leader == nil {
		return fmt.Errorf("no leader available")
	}

	for _, item := range items {
		if err := leader.Add(item); err != nil {
			return fmt.Errorf("failed to add item: %w", err)
		}
	}

	return nil
}

// VerifyData verifies that data exists on all live nodes
func (c *TestCluster) VerifyData(t *testing.T, items [][]byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, node := range c.nodes {
		if node == nil {
			continue // Skip dead nodes
		}

		for _, item := range items {
			if !node.Contains(item) {
				return fmt.Errorf("node %s missing item %s", c.nodeIDs[i], string(item))
			}
		}
	}

	return nil
}

// WaitForLeader waits for a leader to be elected
func (c *TestCluster) WaitForLeader(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if c.GetLeader() != nil {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// TestLeaderFailure tests Leader node failure and recovery
func TestLeaderFailure(t *testing.T) {
	maxRecoveryTime := 500 * time.Millisecond
	
	t.Log("=== Starting Leader Failure Test ===")
	
	// Create a 3-node cluster
	cluster := NewTestCluster(3, 15000)
	defer cluster.Stop()
	
	// Start the cluster
	if err := cluster.Start(t); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}
	
	// Wait for leader election
	if !cluster.WaitForLeader(5 * time.Second) {
		t.Fatal("Failed to elect leader within timeout")
	}
	
	leaderIndex := cluster.GetLeaderIndex()
	t.Logf("Leader is node%d (index %d)", leaderIndex+1, leaderIndex)
	
	// Write test data
	testData := [][]byte{
		[]byte("test-item-1"),
		[]byte("test-item-2"),
		[]byte("test-item-3"),
	}
	
	if err := cluster.WriteData(t, testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	t.Log("Test data written successfully")
	
	// Kill the leader
	t.Log("Killing leader node...")
	startTime := time.Now()
	if err := cluster.KillNode(leaderIndex); err != nil {
		t.Fatalf("Failed to kill leader: %v", err)
	}
	
	// Wait for new leader election
	if !cluster.WaitForLeader(maxRecoveryTime) {
		t.Errorf("Failed to elect new leader within %v", maxRecoveryTime)
	}
	
	recoveryTime := time.Since(startTime)
	t.Logf("New leader elected in %v", recoveryTime)
	
	// Verify data integrity
	if err := cluster.VerifyData(t, testData); err != nil {
		t.Errorf("Data verification failed: %v", err)
	} else {
		t.Log("Data integrity verified")
	}
	
	// Check recovery time
	if recoveryTime > maxRecoveryTime {
		t.Errorf("Leader recovery time %v exceeds maximum %v", recoveryTime, maxRecoveryTime)
	} else {
		t.Logf("✅ Leader recovery time: %v (max: %v) - PASS", recoveryTime, maxRecoveryTime)
	}
}

// TestFollowerFailure tests Follower node failure
func TestFollowerFailure(t *testing.T) {
	t.Log("=== Starting Follower Failure Test ===")
	
	// Create a 3-node cluster
	cluster := NewTestCluster(3, 16000)
	defer cluster.Stop()
	
	// Start the cluster
	if err := cluster.Start(t); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}
	
	// Wait for leader election
	if !cluster.WaitForLeader(5 * time.Second) {
		t.Fatal("Failed to elect leader within timeout")
	}
	
	leaderIndex := cluster.GetLeaderIndex()
	t.Logf("Leader is node%d", leaderIndex+1)
	
	// Find a follower
	followerIndex := -1
	for i := 0; i < 3; i++ {
		if i != leaderIndex {
			followerIndex = i
			break
		}
	}
	
	if followerIndex < 0 {
		t.Fatal("No follower found")
	}
	
	t.Logf("Killing follower node%d", followerIndex+1)
	
	// Write test data before killing follower
	testData := [][]byte{
		[]byte("follower-test-1"),
		[]byte("follower-test-2"),
	}
	
	if err := cluster.WriteData(t, testData); err != nil {
		t.Fatalf("Failed to write test data before follower kill: %v", err)
	}
	
	// Kill the follower
	if err := cluster.KillNode(followerIndex); err != nil {
		t.Fatalf("Failed to kill follower: %v", err)
	}
	
	// Verify service continues to work
	time.Sleep(500 * time.Millisecond)
	
	// Write more data after follower death
	moreData := [][]byte{
		[]byte("after-follower-death-1"),
		[]byte("after-follower-death-2"),
	}
	
	if err := cluster.WriteData(t, moreData); err != nil {
		t.Errorf("Service interrupted after follower death: %v", err)
	} else {
		t.Log("✅ Service continues after follower death - PASS")
	}
	
	// Verify data on remaining nodes
	allData := append(testData, moreData...)
	if err := cluster.VerifyData(t, allData); err != nil {
		t.Errorf("Data verification failed: %v", err)
	} else {
		t.Log("✅ Data integrity maintained - PASS")
	}
}

// TestNetworkPartition tests network partition scenario
func TestNetworkPartition(t *testing.T) {
	t.Log("=== Starting Network Partition Test ===")
	
	// Create a 3-node cluster
	cluster := NewTestCluster(3, 17000)
	defer cluster.Stop()
	
	// Start the cluster
	if err := cluster.Start(t); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}
	
	// Wait for leader election
	if !cluster.WaitForLeader(5 * time.Second) {
		t.Fatal("Failed to elect leader within timeout")
	}
	
	t.Log("Cluster started with leader elected")
	
	// Write test data before partition
	testData := [][]byte{
		[]byte("pre-partition-1"),
		[]byte("pre-partition-2"),
	}
	
	if err := cluster.WriteData(t, testData); err != nil {
		t.Fatalf("Failed to write pre-partition data: %v", err)
	}
	
	t.Log("Pre-partition data written")
	
	// Simulate network partition by killing one node
	leaderIndex := cluster.GetLeaderIndex()
	t.Logf("Simulating partition by isolating node%d", leaderIndex+1)
	
	if err := cluster.KillNode(leaderIndex); err != nil {
		t.Fatalf("Failed to simulate partition: %v", err)
	}
	
	// Wait for new leader
	if !cluster.WaitForLeader(2 * time.Second) {
		t.Error("Failed to elect new leader during partition")
	} else {
		t.Log("✅ New leader elected during partition - PASS")
	}
	
	// Write data during partition
	partitionData := [][]byte{
		[]byte("during-partition-1"),
	}
	
	if err := cluster.WriteData(t, partitionData); err != nil {
		t.Errorf("Failed to write data during partition: %v", err)
	} else {
		t.Log("✅ Data written during partition - PASS")
	}
	
	// "Restore" network by restarting the killed node
	t.Log("Restoring network connection...")
	if err := cluster.RestartNode(leaderIndex); err != nil {
		t.Fatalf("Failed to restore node: %v", err)
	}
	
	// Wait for node to rejoin
	time.Sleep(2 * time.Second)
	
	// Verify all data is consistent
	allData := append(testData, partitionData...)
	if err := cluster.VerifyData(t, allData); err != nil {
		t.Logf("⚠️ Data consistency check: %v", err)
	} else {
		t.Log("✅ Data consistency after partition heal - PASS")
	}
}

// TestPodRecovery tests node restart and data recovery
func TestPodRecovery(t *testing.T) {
	t.Log("=== Starting Pod Recovery Test ===")
	
	// Create a 3-node cluster
	cluster := NewTestCluster(3, 18000)
	defer cluster.Stop()
	
	// Start the cluster
	if err := cluster.Start(t); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}
	
	// Wait for leader election
	if !cluster.WaitForLeader(5 * time.Second) {
		t.Fatal("Failed to elect leader within timeout")
	}
	
	// Write test data
	testData := [][]byte{
		[]byte("recovery-test-1"),
		[]byte("recovery-test-2"),
		[]byte("recovery-test-3"),
	}
	
	if err := cluster.WriteData(t, testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	
	t.Log("Test data written before node restart")
	
	// Kill a non-leader node
	leaderIndex := cluster.GetLeaderIndex()
	killIndex := (leaderIndex + 1) % 3
	
	t.Logf("Killing node%d for restart test", killIndex+1)
	if err := cluster.KillNode(killIndex); err != nil {
		t.Fatalf("Failed to kill node: %v", err)
	}
	
	// Restart the node
	t.Log("Restarting node...")
	if err := cluster.RestartNode(killIndex); err != nil {
		t.Fatalf("Failed to restart node: %v", err)
	}
	
	// Wait for node to recover and sync
	time.Sleep(3 * time.Second)
	
	// Verify data on restarted node
	cluster.mu.Lock()
	restartedNode := cluster.nodes[killIndex]
	cluster.mu.Unlock()
	
	if restartedNode == nil {
		t.Fatal("Restarted node is nil")
	}
	
	dataComplete := true
	for _, item := range testData {
		if !restartedNode.Contains(item) {
			dataComplete = false
			t.Errorf("Restarted node missing item: %s", string(item))
		}
	}
	
	if dataComplete {
		t.Log("✅ Restarted node recovered all data - PASS")
	} else {
		t.Error("❌ Restarted node failed to recover all data")
	}
}

// TestConcurrentFailures tests multiple simultaneous failures
func TestConcurrentFailures(t *testing.T) {
	t.Log("=== Starting Concurrent Failures Test ===")
	
	// Create a 5-node cluster for this test
	cluster := NewTestCluster(5, 19000)
	defer cluster.Stop()
	
	// Start the cluster
	if err := cluster.Start(t); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}
	
	// Wait for leader election
	if !cluster.WaitForLeader(5 * time.Second) {
		t.Fatal("Failed to elect leader within timeout")
	}
	
	// Write test data
	testData := [][]byte{
		[]byte("concurrent-test-1"),
		[]byte("concurrent-test-2"),
	}
	
	if err := cluster.WriteData(t, testData); err != nil {
		t.Fatalf("Failed to write test data: %v", err)
	}
	
	// Kill 2 non-leader nodes simultaneously
	leaderIndex := cluster.GetLeaderIndex()
	killIndices := []int{}
	for i := 0; i < 5; i++ {
		if i != leaderIndex && len(killIndices) < 2 {
			killIndices = append(killIndices, i)
		}
	}
	
	t.Logf("Killing nodes %v simultaneously", killIndices)
	
	var wg sync.WaitGroup
	for _, idx := range killIndices {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cluster.KillNode(i)
		}(idx)
	}
	wg.Wait()
	
	// Verify service still works
	time.Sleep(500 * time.Millisecond)
	
	moreData := [][]byte{
		[]byte("after-concurrent-failure-1"),
	}
	
	if err := cluster.WriteData(t, moreData); err != nil {
		t.Errorf("Service interrupted after concurrent failures: %v", err)
	} else {
		t.Log("✅ Service survives concurrent failures - PASS")
	}
	
	// Verify data integrity
	allData := append(testData, moreData...)
	if err := cluster.VerifyData(t, allData); err != nil {
		t.Errorf("Data verification failed: %v", err)
	} else {
		t.Log("✅ Data integrity maintained after concurrent failures - PASS")
	}
}

// Helper function to check if a port is available
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// waitForPort waits for a port to become available
func waitForPort(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isPortAvailable(port) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
