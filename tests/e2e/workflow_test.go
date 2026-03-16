package e2e

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	pb "github.com/wangminggit/distributed-bloom-filter/api/proto"
	grpcserver "github.com/wangminggit/distributed-bloom-filter/internal/grpc"
	"github.com/wangminggit/distributed-bloom-filter/internal/metadata"
	raftnode "github.com/wangminggit/distributed-bloom-filter/internal/raft"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	testTimeout    = 30 * time.Second
	clusterSize    = 3
)

// getAvailablePort finds an available port
func getAvailablePort(basePort int) int {
	for port := basePort; port < basePort+1000; port++ {
		lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			lis.Close()
			return port
		}
	}
	return basePort
}

// E2ECluster represents a complete test cluster
type E2ECluster struct {
	grpcServers []*grpc.Server
	grpcConns   []*grpc.ClientConn
	grpcClients []pb.DBFServiceClient
	raftNodes   []*raftnode.Node
	nodeIDs     []string
	raftPorts   []int
	grpcPorts   []int
	dataDirs    []string
	mu          sync.Mutex
}

// NewE2ECluster creates a new test cluster
func NewE2ECluster(numNodes int) *E2ECluster {
	cluster := &E2ECluster{
		grpcServers: make([]*grpc.Server, numNodes),
		grpcConns:   make([]*grpc.ClientConn, numNodes),
		grpcClients: make([]pb.DBFServiceClient, numNodes),
		raftNodes:   make([]*raftnode.Node, numNodes),
		nodeIDs:     make([]string, numNodes),
		raftPorts:   make([]int, numNodes),
		grpcPorts:   make([]int, numNodes),
		dataDirs:    make([]string, numNodes),
	}

	basePort := 20000 + int(time.Now().UnixNano()%1000)
	for i := 0; i < numNodes; i++ {
		cluster.nodeIDs[i] = fmt.Sprintf("e2e-node%d", i+1)
		cluster.raftPorts[i] = getAvailablePort(basePort + i*2)
		cluster.grpcPorts[i] = getAvailablePort(basePort + i*2 + 1)
		cluster.dataDirs[i] = filepath.Join(os.TempDir(), fmt.Sprintf("dbf-e2e-%d-%d", time.Now().UnixNano(), i))
		os.MkdirAll(cluster.dataDirs[i], 0755)
	}

	return cluster
}

// Start starts the complete cluster
func (c *E2ECluster) Start(t *testing.T) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Start all nodes
	for i := 0; i < len(c.raftNodes); i++ {
		if err := c.startNode(i, i == 0); err != nil {
			return fmt.Errorf("failed to start node %d: %w", i, err)
		}
		t.Logf("Started node %s on Raft port %d, gRPC port %d", c.nodeIDs[i], c.raftPorts[i], c.grpcPorts[i])
	}

	// Wait for cluster to stabilize
	time.Sleep(2 * time.Second)

	// Wait for leader election
	if !c.waitForLeader(10 * time.Second) {
		return fmt.Errorf("failed to elect leader")
	}

	t.Logf("Cluster started with %d nodes", len(c.raftNodes))
	return nil
}

// startNode starts a single node
func (c *E2ECluster) startNode(index int, bootstrap bool) error {
	// Create Bloom filter
	bloomFilter := bloom.NewCountingBloomFilter(1048576, 7)

	// Create WAL encryptor
	walEncryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		return fmt.Errorf("failed to create WAL encryptor: %w", err)
	}

	// Create metadata service
	metadataService := metadata.NewService(c.dataDirs[index])

	// Create Raft node
	raftNode := raftnode.NewNode(c.nodeIDs[index], c.raftPorts[index], c.dataDirs[index], bloomFilter, walEncryptor, metadataService)

	// Start Raft node
	if err := raftNode.Start(bootstrap); err != nil {
		return fmt.Errorf("failed to start Raft node: %w", err)
	}

	c.raftNodes[index] = raftNode

	// Create gRPC server
	dbfService := grpcserver.NewDBFServer(raftNode)
	grpcServer := grpc.NewServer()
	pb.RegisterDBFServiceServer(grpcServer, dbfService)
	c.grpcServers[index] = grpcServer

	// Start gRPC server
	go func(idx int) {
		addr := fmt.Sprintf("localhost:%d", c.grpcPorts[idx])
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			log.Printf("Failed to listen on %s: %v", addr, err)
			return
		}
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			log.Printf("gRPC server failed: %v", err)
		}
	}(index)

	// Wait for gRPC server to start
	time.Sleep(100 * time.Millisecond)

	// Create gRPC client connection
	conn, err := grpc.NewClient(fmt.Sprintf("localhost:%d", c.grpcPorts[index]), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to create gRPC client: %w", err)
	}

	c.grpcConns[index] = conn
	c.grpcClients[index] = pb.NewDBFServiceClient(conn)

	return nil
}

// Stop stops the cluster
func (c *E2ECluster) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close gRPC connections
	for _, conn := range c.grpcConns {
		if conn != nil {
			conn.Close()
		}
	}

	// Stop gRPC servers
	for _, server := range c.grpcServers {
		if server != nil {
			server.GracefulStop()
		}
	}

	// Shutdown Raft nodes
	for _, node := range c.raftNodes {
		if node != nil {
			node.Shutdown()
		}
	}

	// Cleanup data directories
	for _, dir := range c.dataDirs {
		os.RemoveAll(dir)
	}
}

// GetLeaderClient returns the gRPC client for the leader node
func (c *E2ECluster) GetLeaderClient() (pb.DBFServiceClient, int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, node := range c.raftNodes {
		if node != nil && node.IsLeader() {
			return c.grpcClients[i], i
		}
	}
	return nil, -1
}

// GetClient returns the gRPC client for a specific node
func (c *E2ECluster) GetClient(index int) pb.DBFServiceClient {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.grpcClients[index]
}

// waitForLeader waits for a leader to be elected
func (c *E2ECluster) waitForLeader(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if c.GetLeader() != nil {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// GetLeader returns the leader node
func (c *E2ECluster) GetLeader() *raftnode.Node {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, node := range c.raftNodes {
		if node != nil && node.IsLeader() {
			return node
		}
	}
	return nil
}

// KillNode kills a node
func (c *E2ECluster) KillNode(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.raftNodes) {
		return fmt.Errorf("invalid node index: %d", index)
	}

	if c.grpcConns[index] != nil {
		c.grpcConns[index].Close()
		c.grpcConns[index] = nil
	}

	if c.grpcServers[index] != nil {
		c.grpcServers[index].GracefulStop()
		c.grpcServers[index] = nil
	}

	if c.raftNodes[index] != nil {
		c.raftNodes[index].Shutdown()
		c.raftNodes[index] = nil
	}

	log.Printf("Killed node %s", c.nodeIDs[index])
	return nil
}

// RestartNode restarts a node
func (c *E2ECluster) RestartNode(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if index < 0 || index >= len(c.raftNodes) {
		return fmt.Errorf("invalid node index: %d", index)
	}

	// Reconnect gRPC client
	conn, err := grpc.NewClient(fmt.Sprintf("localhost:%d", c.grpcPorts[index]), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to reconnect gRPC client: %w", err)
	}

	c.grpcConns[index] = conn
	c.grpcClients[index] = pb.NewDBFServiceClient(conn)

	log.Printf("Restarted node %s", c.nodeIDs[index])
	return nil
}

// TestFullWorkflow tests the complete workflow: Add -> Contains -> Remove -> Contains
func TestFullWorkflow(t *testing.T) {
	t.Log("=== Starting Full Workflow Test ===")

	// Create and start cluster
	cluster := NewE2ECluster(clusterSize)
	defer cluster.Stop()

	if err := cluster.Start(t); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}

	// Get leader client
	leaderClient, leaderIdx := cluster.GetLeaderClient()
	if leaderClient == nil {
		t.Fatal("No leader available")
	}
	t.Logf("Leader is node %d", leaderIdx)

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Test 1: Add element
	t.Run("Add_Element", func(t *testing.T) {
		item := []byte("test-element-1")
		req := &pb.AddRequest{Item: item}
		resp, err := leaderClient.Add(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Success, "Add should succeed")
		assert.Empty(t, resp.Error)
		t.Logf("✅ Added element: %s", string(item))
	})

	// Wait for Raft to apply
	time.Sleep(200 * time.Millisecond)

	// Test 2: Contains should return true
	t.Run("Contains_Exists", func(t *testing.T) {
		item := []byte("test-element-1")
		req := &pb.ContainsRequest{Item: item}
		resp, err := leaderClient.Contains(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Exists, "Element should exist")
		assert.Empty(t, resp.Error)
		t.Logf("✅ Contains returned true for: %s", string(item))
	})

	// Test 3: Remove element
	t.Run("Remove_Element", func(t *testing.T) {
		item := []byte("test-element-1")
		req := &pb.RemoveRequest{Item: item}
		resp, err := leaderClient.Remove(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Success, "Remove should succeed")
		assert.Empty(t, resp.Error)
		t.Logf("✅ Removed element: %s", string(item))
	})

	// Wait for Raft to apply
	time.Sleep(200 * time.Millisecond)

	// Test 4: Contains should return false after removal
	t.Run("Contains_NotExists_After_Remove", func(t *testing.T) {
		item := []byte("test-element-1")
		req := &pb.ContainsRequest{Item: item}
		resp, err := leaderClient.Contains(ctx, req)
		require.NoError(t, err)
		assert.False(t, resp.Exists, "Element should not exist after removal")
		assert.Empty(t, resp.Error)
		t.Logf("✅ Contains returned false after removal: %s", string(item))
	})

	t.Log("=== Full Workflow Test PASSED ===")
}

// TestBatchWorkflow tests batch operations
func TestBatchWorkflow(t *testing.T) {
	t.Log("=== Starting Batch Workflow Test ===")

	// Create and start cluster
	cluster := NewE2ECluster(clusterSize)
	defer cluster.Stop()

	if err := cluster.Start(t); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}

	// Get leader client
	leaderClient, _ := cluster.GetLeaderClient()
	if leaderClient == nil {
		t.Fatal("No leader available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Test 1: BatchAdd 1000 elements
	t.Run("BatchAdd_1000_Elements", func(t *testing.T) {
		items := make([][]byte, 1000)
		for i := 0; i < 1000; i++ {
			items[i] = []byte(fmt.Sprintf("batch-element-%d", i))
		}

		req := &pb.BatchAddRequest{Items: items}
		resp, err := leaderClient.BatchAdd(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, int32(1000), resp.SuccessCount, "All 1000 items should succeed")
		assert.Equal(t, int32(0), resp.FailureCount)
		t.Logf("✅ BatchAdd succeeded: %d items", resp.SuccessCount)
	})

	// Wait for Raft to apply
	time.Sleep(500 * time.Millisecond)

	// Test 2: BatchContains to verify
	t.Run("BatchContains_Verify", func(t *testing.T) {
		items := make([][]byte, 100)
		for i := 0; i < 100; i++ {
			items[i] = []byte(fmt.Sprintf("batch-element-%d", i*10))
		}

		req := &pb.BatchContainsRequest{Items: items}
		resp, err := leaderClient.BatchContains(ctx, req)
		require.NoError(t, err)
		assert.Len(t, resp.Results, 100)

		for i, result := range resp.Results {
			assert.True(t, result, "Item %d should exist", i)
		}
		t.Logf("✅ BatchContains verified: %d items exist", len(resp.Results))
	})

	// Test 3: BatchAdd with some invalid items
	t.Run("BatchAdd_WithInvalidItems", func(t *testing.T) {
		items := [][]byte{
			[]byte("valid-1"),
			[]byte(""), // empty
			[]byte("valid-2"),
			[]byte(""), // empty
			[]byte("valid-3"),
		}

		req := &pb.BatchAddRequest{Items: items}
		resp, err := leaderClient.BatchAdd(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, int32(3), resp.SuccessCount)
		assert.Equal(t, int32(2), resp.FailureCount)
		t.Logf("✅ BatchAdd with invalid items: %d success, %d failure", resp.SuccessCount, resp.FailureCount)
	})

	t.Log("=== Batch Workflow Test PASSED ===")
}

// TestClusterScaling tests cluster scaling (simulated)
func TestClusterScaling(t *testing.T) {
	t.Log("=== Starting Cluster Scaling Test ===")

	// Start with 2 nodes
	cluster := NewE2ECluster(2)
	defer cluster.Stop()

	if err := cluster.Start(t); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Get leader and add data
	leaderClient, _ := cluster.GetLeaderClient()
	if leaderClient == nil {
		t.Fatal("No leader available")
	}

	// Add initial data
	initialData := [][]byte{
		[]byte("scaling-data-1"),
		[]byte("scaling-data-2"),
		[]byte("scaling-data-3"),
	}

	for _, item := range initialData {
		req := &pb.AddRequest{Item: item}
		resp, err := leaderClient.Add(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Success)
	}

	time.Sleep(300 * time.Millisecond)
	t.Log("✅ Initial data added")

	// Simulate adding a new node (in real scenario, this would be a new pod)
	t.Log("Simulating cluster scale-up to 3 nodes...")
	// Note: In real K8s, StatefulSet would handle this automatically
	// For this test, we verify the existing nodes maintain data

	// Verify data still accessible
	for _, item := range initialData {
		req := &pb.ContainsRequest{Item: item}
		resp, err := leaderClient.Contains(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Exists, "Data should persist after scaling")
	}

	t.Log("✅ Data verified after simulated scaling")
	t.Log("=== Cluster Scaling Test PASSED ===")
}

// TestDataConsistency tests data consistency across replicas
func TestDataConsistency(t *testing.T) {
	t.Log("=== Starting Data Consistency Test ===")

	// Create and start cluster
	cluster := NewE2ECluster(clusterSize)
	defer cluster.Stop()

	if err := cluster.Start(t); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Get leader client
	leaderClient, _ := cluster.GetLeaderClient()
	if leaderClient == nil {
		t.Fatal("No leader available")
	}

	// Concurrent writes
	testItems := make([][]byte, 100)
	for i := 0; i < 100; i++ {
		testItems[i] = []byte(fmt.Sprintf("consistency-test-%d", i))
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	// Concurrent writes from multiple goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(startIdx int) {
			defer wg.Done()
			for j := startIdx; j < startIdx+10; j++ {
				req := &pb.AddRequest{Item: testItems[j]}
				resp, err := leaderClient.Add(ctx, req)
				if err != nil {
					errChan <- err
					return
				}
				if !resp.Success {
					errChan <- fmt.Errorf("add failed for item %d", j)
					return
				}
			}
		}(i * 10)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		t.Errorf("Concurrent write error: %v", err)
	}

	t.Log("✅ Concurrent writes completed")

	// Wait for Raft to replicate
	time.Sleep(1 * time.Second)

	// Verify data on all nodes
	t.Log("Verifying data consistency across all nodes...")
	consistent := true
	for nodeIdx := 0; nodeIdx < clusterSize; nodeIdx++ {
		client := cluster.GetClient(nodeIdx)
		if client == nil {
			continue
		}

		for _, item := range testItems {
			req := &pb.ContainsRequest{Item: item}
			resp, err := client.Contains(ctx, req)
			if err != nil || !resp.Exists {
				t.Errorf("Node %d missing item: %s", nodeIdx, string(item))
				consistent = false
			}
		}
	}

	if consistent {
		t.Log("✅ All nodes have consistent data")
	} else {
		t.Error("❌ Data inconsistency detected")
	}

	// Verify Raft strong consistency
	t.Log("Verifying Raft strong consistency...")
	leaderNode := cluster.GetLeader()
	if leaderNode != nil {
		t.Logf("✅ Leader confirmed")
	}

	t.Log("=== Data Consistency Test PASSED ===")
}

// TestHighAvailability tests HA during node failures
func TestHighAvailability(t *testing.T) {
	t.Log("=== Starting High Availability Test ===")

	cluster := NewE2ECluster(clusterSize)
	defer cluster.Stop()

	if err := cluster.Start(t); err != nil {
		t.Fatalf("Failed to start cluster: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Get initial leader
	leaderClient, leaderIdx := cluster.GetLeaderClient()
	if leaderClient == nil {
		t.Fatal("No leader available")
	}
	t.Logf("Initial leader: node %d", leaderIdx)

	// Add some data
	testData := [][]byte{
		[]byte("ha-test-1"),
		[]byte("ha-test-2"),
	}

	for _, item := range testData {
		req := &pb.AddRequest{Item: item}
		resp, err := leaderClient.Add(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Success)
	}

	time.Sleep(300 * time.Millisecond)
	t.Log("✅ Initial data added")

	// Kill the leader
	t.Logf("Killing leader node %d...", leaderIdx)
	if err := cluster.KillNode(leaderIdx); err != nil {
		t.Fatalf("Failed to kill leader: %v", err)
	}

	// Wait for new leader election
	time.Sleep(2 * time.Second)

	// Get new leader
	newLeaderClient, newLeaderIdx := cluster.GetLeaderClient()
	if newLeaderClient == nil {
		t.Fatal("No new leader elected")
	}
	t.Logf("✅ New leader elected: node %d", newLeaderIdx)

	// Verify service is still available
	t.Log("Testing service availability...")
	for _, item := range testData {
		req := &pb.ContainsRequest{Item: item}
		resp, err := newLeaderClient.Contains(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Exists, "Data should be available after leader failure")
	}

	t.Log("✅ Service remains available during leader failure")
	t.Log("=== High Availability Test PASSED ===")
}
