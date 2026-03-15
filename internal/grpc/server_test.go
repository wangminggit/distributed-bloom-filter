package grpc

import (
	"context"
	"sync"
	"testing"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

// MockRaftNode is a mock implementation of the Raft node for testing.
// It avoids the bolt DB race detection issues.
type MockRaftNode struct {
	mu       sync.RWMutex
	filter   *bloom.CountingBloomFilter
	nodeID   string
	isLeader bool
}

// NewMockRaftNodeForTest creates a new mock Raft node for testing.
// This is exported for use in tls_test.go
func NewMockRaftNodeForTest(nodeID string) *MockRaftNode {
	return &MockRaftNode{
		filter:   bloom.NewCountingBloomFilter(10000, 3),
		nodeID:   nodeID,
		isLeader: true, // Assume leader for testing
	}
}

// NewMockRaftNode creates a new mock Raft node (alias for NewMockRaftNodeForTest).
func NewMockRaftNode(nodeID string) *MockRaftNode {
	return NewMockRaftNodeForTest(nodeID)
}

// Start starts the mock node (no-op).
func (m *MockRaftNode) Start() error {
	return nil
}

// Shutdown shuts down the mock node (no-op).
func (m *MockRaftNode) Shutdown() error {
	return nil
}

// IsLeader returns whether this node is the leader.
func (m *MockRaftNode) IsLeader() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isLeader
}

// Add adds an item to the Bloom filter.
func (m *MockRaftNode) Add(item []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(item) == 0 {
		return nil // Will be validated at gRPC layer
	}
	m.filter.Add(item)
	return nil
}

// Contains checks if an item exists in the Bloom filter.
func (m *MockRaftNode) Contains(item []byte) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.filter.Contains(item)
}

// Remove removes an item from the Bloom filter.
func (m *MockRaftNode) Remove(item []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(item) == 0 {
		return nil
	}
	m.filter.Remove(item)
	return nil
}

// BatchAdd adds multiple items.
func (m *MockRaftNode) BatchAdd(items [][]byte) (successCount int, failureCount int, errors []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	errors = make([]string, len(items))
	for i, item := range items {
		if len(item) == 0 {
			errors[i] = "empty item"
			failureCount++
		} else {
			m.filter.Add(item)
			successCount++
		}
	}
	return
}

// BatchContains checks multiple items.
func (m *MockRaftNode) BatchContains(items [][]byte) []bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make([]bool, len(items))
	for i, item := range items {
		results[i] = m.filter.Contains(item)
	}
	return results
}

// GetState returns the node state.
func (m *MockRaftNode) GetState() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]interface{}{
		"node_id":    m.nodeID,
		"is_leader":  m.isLeader,
		"raft_state": "Leader",
		"leader":     m.nodeID,
		"bloom_size": 10000,
		"bloom_k":    3,
		"raft_port":  18081,
	}
}

// GetConfig returns the node configuration.
func (m *MockRaftNode) GetConfig() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]interface{}{
		"node_id":     m.nodeID,
		"bloom_size":  10000,
		"bloom_k":     3,
		"raft_port":   18081,
		"data_dir":    "/tmp/test",
		"bootstrap":   true,
		"heartbeat":   "1s",
		"election":    "3s",
		"commit":      "50ms",
		"snapshot_int": "2m0s",
		"snapshot_thresh": 8192,
	}
}

// setupTestServer creates a test gRPC service with a mock Raft node.
// This avoids the bolt DB race detection issues.
func setupTestServer(t *testing.T) (*DBFService, func()) {
	// Use mock Raft node to avoid bolt DB race issues
	mockNode := NewMockRaftNode("test-node")

	// Create service
	service := NewDBFService(mockNode)

	cleanup := func() {
		// No cleanup needed for mock
	}

	return service, cleanup
}

// TestServerAdd tests the Add RPC method.
func TestServerAdd(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// Test 1: Add a valid item
	t.Run("AddValidItem", func(t *testing.T) {
		req := &proto.AddRequest{Item: []byte("test-item-1")}
		resp, err := server.Add(ctx, req)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if !resp.Success {
			t.Errorf("Expected success, got error: %s", resp.Error)
		}
	})

	// Test 2: Add empty item should fail
	t.Run("AddEmptyItem", func(t *testing.T) {
		req := &proto.AddRequest{Item: []byte("")}
		resp, err := server.Add(ctx, req)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if resp.Success {
			t.Error("Expected failure for empty item")
		}
		if resp.Error == "" {
			t.Error("Expected error message for empty item")
		}
	})

	// Test 3: Add nil item should fail
	t.Run("AddNilItem", func(t *testing.T) {
		req := &proto.AddRequest{Item: nil}
		resp, err := server.Add(ctx, req)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if resp.Success {
			t.Error("Expected failure for nil item")
		}
	})

	// Test 4: Verify item was added
	t.Run("VerifyItemAdded", func(t *testing.T) {
		containsReq := &proto.ContainsRequest{Item: []byte("test-item-1")}
		containsResp, err := server.Contains(ctx, containsReq)
		if err != nil {
			t.Fatalf("Contains failed: %v", err)
		}
		if !containsResp.Exists {
			t.Error("Expected item to exist after Add")
		}
	})
}

// TestServerContains tests the Contains RPC method.
func TestServerContains(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// Test 1: Check non-existent item
	t.Run("ContainsNonExistent", func(t *testing.T) {
		req := &proto.ContainsRequest{Item: []byte("non-existent-item")}
		resp, err := server.Contains(ctx, req)
		if err != nil {
			t.Fatalf("Contains failed: %v", err)
		}
		if resp.Error != "" {
			t.Errorf("Unexpected error: %s", resp.Error)
		}
	})

	// Test 2: Add then check
	t.Run("ContainsAfterAdd", func(t *testing.T) {
		// Add item
		addReq := &proto.AddRequest{Item: []byte("test-item-2")}
		addResp, err := server.Add(ctx, addReq)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if !addResp.Success {
			t.Fatalf("Add failed: %s", addResp.Error)
		}

		// Check item
		containsReq := &proto.ContainsRequest{Item: []byte("test-item-2")}
		containsResp, err := server.Contains(ctx, containsReq)
		if err != nil {
			t.Fatalf("Contains failed: %v", err)
		}
		if !containsResp.Exists {
			t.Error("Expected item to exist after Add")
		}
	})

	// Test 3: Empty item should return error
	t.Run("ContainsEmptyItem", func(t *testing.T) {
		req := &proto.ContainsRequest{Item: []byte("")}
		resp, err := server.Contains(ctx, req)
		if err != nil {
			t.Fatalf("Contains failed: %v", err)
		}
		if resp.Error == "" {
			t.Error("Expected error for empty item")
		}
	})
}

// TestServerBatchOperations tests the BatchAdd and BatchContains RPC methods.
func TestServerBatchOperations(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// Test 1: BatchAdd multiple items
	t.Run("BatchAddMultipleItems", func(t *testing.T) {
		items := [][]byte{
			[]byte("batch-item-1"),
			[]byte("batch-item-2"),
			[]byte("batch-item-3"),
		}

		req := &proto.BatchAddRequest{Items: items}
		resp, err := server.BatchAdd(ctx, req)
		if err != nil {
			t.Fatalf("BatchAdd failed: %v", err)
		}
		if resp.SuccessCount != 3 {
			t.Errorf("Expected 3 successes, got %d", resp.SuccessCount)
		}
		if resp.FailureCount != 0 {
			t.Errorf("Expected 0 failures, got %d", resp.FailureCount)
		}
	})

	// Test 2: BatchAdd with some empty items
	t.Run("BatchAddWithEmptyItems", func(t *testing.T) {
		items := [][]byte{
			[]byte("batch-item-4"),
			[]byte(""),
			[]byte("batch-item-5"),
		}

		req := &proto.BatchAddRequest{Items: items}
		resp, err := server.BatchAdd(ctx, req)
		if err != nil {
			t.Fatalf("BatchAdd failed: %v", err)
		}
		if resp.SuccessCount != 2 {
			t.Errorf("Expected 2 successes, got %d", resp.SuccessCount)
		}
		if resp.FailureCount != 1 {
			t.Errorf("Expected 1 failure, got %d", resp.FailureCount)
		}
		if len(resp.Errors) < 2 || resp.Errors[1] == "" {
			t.Error("Expected error for empty item")
		}
	})

	// Test 3: BatchAdd empty list
	t.Run("BatchAddEmptyList", func(t *testing.T) {
		req := &proto.BatchAddRequest{Items: [][]byte{}}
		resp, err := server.BatchAdd(ctx, req)
		if err != nil {
			t.Fatalf("BatchAdd failed: %v", err)
		}
		if resp.SuccessCount != 0 {
			t.Errorf("Expected 0 successes, got %d", resp.SuccessCount)
		}
	})

	// Test 4: BatchContains
	t.Run("BatchContains", func(t *testing.T) {
		items := [][]byte{
			[]byte("batch-item-1"),
			[]byte("batch-item-2"),
			[]byte("non-existent"),
		}

		req := &proto.BatchContainsRequest{Items: items}
		resp, err := server.BatchContains(ctx, req)
		if err != nil {
			t.Fatalf("BatchContains failed: %v", err)
		}
		if len(resp.Results) != 3 {
			t.Errorf("Expected 3 results, got %d", len(resp.Results))
		}
		// First two should exist (added in previous test)
		if !resp.Results[0] {
			t.Error("Expected batch-item-1 to exist")
		}
		if !resp.Results[1] {
			t.Error("Expected batch-item-2 to exist")
		}
	})

	// Test 5: BatchContains empty list
	t.Run("BatchContainsEmptyList", func(t *testing.T) {
		req := &proto.BatchContainsRequest{Items: [][]byte{}}
		resp, err := server.BatchContains(ctx, req)
		if err != nil {
			t.Fatalf("BatchContains failed: %v", err)
		}
		if len(resp.Results) != 0 {
			t.Errorf("Expected 0 results, got %d", len(resp.Results))
		}
	})
}

// TestServerRemove tests the Remove RPC method.
func TestServerRemove(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// Test 1: Remove valid item
	t.Run("RemoveValidItem", func(t *testing.T) {
		// First add the item
		addReq := &proto.AddRequest{Item: []byte("remove-test-item")}
		addResp, err := server.Add(ctx, addReq)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if !addResp.Success {
			t.Fatalf("Add failed: %s", addResp.Error)
		}

		// Now remove it
		removeReq := &proto.RemoveRequest{Item: []byte("remove-test-item")}
		removeResp, err := server.Remove(ctx, removeReq)
		if err != nil {
			t.Fatalf("Remove failed: %v", err)
		}
		if !removeResp.Success {
			t.Errorf("Expected success, got error: %s", removeResp.Error)
		}
	})

	// Test 2: Remove empty item should fail
	t.Run("RemoveEmptyItem", func(t *testing.T) {
		req := &proto.RemoveRequest{Item: []byte("")}
		resp, err := server.Remove(ctx, req)
		if err != nil {
			t.Fatalf("Remove failed: %v", err)
		}
		if resp.Success {
			t.Error("Expected failure for empty item")
		}
		if resp.Error == "" {
			t.Error("Expected error message for empty item")
		}
	})
}

// TestServerGetStats tests the GetStats RPC method.
func TestServerGetStats(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("GetStats", func(t *testing.T) {
		req := &proto.GetStatsRequest{}
		resp, err := server.GetStats(ctx, req)
		if err != nil {
			t.Fatalf("GetStats failed: %v", err)
		}
		if resp.Error != "" {
			t.Errorf("Unexpected error: %s", resp.Error)
		}
		if resp.NodeId != "test-node" {
			t.Errorf("Expected node_id 'test-node', got '%s'", resp.NodeId)
		}
		if resp.BloomSize != 10000 {
			t.Errorf("Expected bloom_size 10000, got %d", resp.BloomSize)
		}
		if resp.BloomK != 3 {
			t.Errorf("Expected bloom_k 3, got %d", resp.BloomK)
		}
		if resp.RaftPort != 18081 {
			t.Errorf("Expected raft_port 18081, got %d", resp.RaftPort)
		}
	})
}

// TestServer_ConcurrentRequests tests concurrent request handling.
// This is a P0 test case for race detection verification.
func TestServer_ConcurrentRequests(t *testing.T) {
	server, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// Run concurrent Add operations
	const concurrency = 10
	errChan := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			req := &proto.AddRequest{Item: []byte("concurrent-item-" + string(rune(idx)))}
			_, err := server.Add(ctx, req)
			errChan <- err
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < concurrency; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent Add failed: %v", err)
		}
	}

	// Verify all items were added
	for i := 0; i < concurrency; i++ {
		req := &proto.ContainsRequest{Item: []byte("concurrent-item-" + string(rune(i)))}
		resp, err := server.Contains(ctx, req)
		if err != nil {
			t.Errorf("Contains failed: %v", err)
		}
		if !resp.Exists {
			t.Errorf("Expected concurrent-item-%d to exist", i)
		}
	}
}
