package grpc

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
	"github.com/wangminggit/distributed-bloom-filter/internal/metadata"
	"github.com/wangminggit/distributed-bloom-filter/internal/raft"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

// setupTestServer creates a test gRPC server with a mock Raft node.
func setupTestServer(t *testing.T) (*DBFServer, func()) {
	// Create test data directory
	dataDir := t.TempDir()

	// Initialize components
	walEncryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create WAL encryptor: %v", err)
	}

	metadataService := metadata.NewService(dataDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)
	raftNode := raft.NewNode("test-node", 18081, dataDir, bloomFilter, walEncryptor, metadataService)

	// Start Raft node with bootstrap
	if err := raftNode.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}

	// Wait for node to become leader (up to 2 seconds)
	for i := 0; i < 20; i++ {
		if raftNode.IsLeader() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !raftNode.IsLeader() {
		t.Fatal("Raft node did not become leader")
	}

	server := NewDBFServer(raftNode)

	cleanup := func() {
		raftNode.Shutdown()
	}

	return server, cleanup
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
		// Give Raft time to apply the command
		time.Sleep(100 * time.Millisecond)

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

		// Give Raft time to apply
		time.Sleep(100 * time.Millisecond)

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

		// Give Raft time to apply
		time.Sleep(100 * time.Millisecond)
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
		if resp.Errors[1] == "" {
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

		// Give Raft time to apply
		time.Sleep(100 * time.Millisecond)

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

// TestServerTLSConfig tests TLS configuration for the gRPC server.
func TestServerTLSConfig(t *testing.T) {
	t.Run("TLSConfigDefaults", func(t *testing.T) {
		config := &ServerConfig{
			Port:         50051,
			EnableTLS:    true,
			TLSMinVersion: tls.VersionTLS13,
		}

		if config.EnableTLS != true {
			t.Error("Expected TLS to be enabled")
		}

		if config.TLSMinVersion != tls.VersionTLS13 {
			t.Errorf("Expected TLS 1.3, got %d", config.TLSMinVersion)
		}
	})

	t.Run("TLSConfigWithReload", func(t *testing.T) {
		config := &ServerConfig{
			Port:              50051,
			EnableTLS:         true,
			TLSMinVersion:     tls.VersionTLS13,
			TLSReloadInterval: 5 * time.Minute,
		}

		if config.TLSReloadInterval != 5*time.Minute {
			t.Errorf("Expected reload interval 5m, got %v", config.TLSReloadInterval)
		}
	})
}
