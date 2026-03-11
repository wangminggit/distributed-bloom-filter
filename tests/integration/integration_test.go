package integration

import (
	"context"
	"testing"
	"time"

	pb "github.com/wangminggit/distributed-bloom-filter/api/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	testServerAddr = "localhost:50051"
	testTimeout    = 10 * time.Second
)

// TestAddAndContains 验证 Add + Contains 功能
// 测试场景：添加元素后，查询应该返回存在
func TestAddAndContains(t *testing.T) {
	conn, client, ctx := createTestClient(t, testServerAddr)
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

	tests := []struct {
		name string
		item []byte
	}{
		{"AddAndContains_SingleItem", []byte("test-item-1")},
		{"AddAndContains_ChineseItem", []byte("测试元素")},
		{"AddAndContains_SpecialChars", []byte("item@#$%^&*()")},
		{"AddAndContains_LongItem", []byte("this-is-a-very-long-item-name-to-test-the-bloom-filter-capabilities")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Step 1: Add the item
			addReq := &pb.AddRequest{Item: tt.item}
			addResp, err := client.Add(ctx, addReq)
			require.NoError(t, err, "Add RPC should not return error")
			assert.True(t, addResp.Success, "Add should succeed for item: %s", string(tt.item))
			assert.Empty(t, addResp.Error, "Add should not have error message")

			// Wait for Raft to apply the command
			time.Sleep(200 * time.Millisecond)

			// Step 2: Verify the item exists using Contains
			containsReq := &pb.ContainsRequest{Item: tt.item}
			containsResp, err := client.Contains(ctx, containsReq)
			require.NoError(t, err, "Contains RPC should not return error")
			assert.True(t, containsResp.Exists, "Item should exist after Add: %s", string(tt.item))
			assert.Empty(t, containsResp.Error, "Contains should not have error message")
		})
	}

	// Test empty item should fail
	t.Run("AddAndContains_EmptyItem", func(t *testing.T) {
		addReq := &pb.AddRequest{Item: []byte("")}
		addResp, err := client.Add(ctx, addReq)
		require.NoError(t, err)
		assert.False(t, addResp.Success, "Add should fail for empty item")
		assert.NotEmpty(t, addResp.Error, "Add should return error for empty item")
	})
}

// TestAddAndRemove 验证 Add + Remove 功能
// 测试场景：添加元素后删除，查询应该返回不存在
func TestAddAndRemove(t *testing.T) {
	conn, client, ctx := createTestClient(t, testServerAddr)
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

	t.Run("AddAndRemove_SingleItem", func(t *testing.T) {
		item := []byte("remove-test-item-1")

		// Step 1: Add the item
		addReq := &pb.AddRequest{Item: item}
		addResp, err := client.Add(ctx, addReq)
		require.NoError(t, err)
		assert.True(t, addResp.Success, "Add should succeed")

		// Wait for Raft to apply
		time.Sleep(200 * time.Millisecond)

		// Step 2: Verify item exists
		containsReq := &pb.ContainsRequest{Item: item}
		containsResp, err := client.Contains(ctx, containsReq)
		require.NoError(t, err)
		assert.True(t, containsResp.Exists, "Item should exist after Add")

		// Step 3: Remove the item
		removeReq := &pb.RemoveRequest{Item: item}
		removeResp, err := client.Remove(ctx, removeReq)
		require.NoError(t, err)
		assert.True(t, removeResp.Success, "Remove should succeed")
		assert.Empty(t, removeResp.Error, "Remove should not have error message")

		// Wait for Raft to apply
		time.Sleep(200 * time.Millisecond)

		// Step 4: Verify item no longer exists
		containsResp2, err := client.Contains(ctx, containsReq)
		require.NoError(t, err)
		assert.False(t, containsResp2.Exists, "Item should not exist after Remove")
	})

	t.Run("AddAndRemove_MultipleItems", func(t *testing.T) {
		items := [][]byte{
			[]byte("remove-test-item-2"),
			[]byte("remove-test-item-3"),
			[]byte("remove-test-item-4"),
		}

		// Add all items
		for _, item := range items {
			addReq := &pb.AddRequest{Item: item}
			addResp, err := client.Add(ctx, addReq)
			require.NoError(t, err)
			assert.True(t, addResp.Success)
		}

		time.Sleep(200 * time.Millisecond)

		// Remove all items
		for _, item := range items {
			removeReq := &pb.RemoveRequest{Item: item}
			removeResp, err := client.Remove(ctx, removeReq)
			require.NoError(t, err)
			assert.True(t, removeResp.Success)
		}

		time.Sleep(200 * time.Millisecond)

		// Verify all items are removed
		for _, item := range items {
			containsReq := &pb.ContainsRequest{Item: item}
			containsResp, err := client.Contains(ctx, containsReq)
			require.NoError(t, err)
			assert.False(t, containsResp.Exists, "Item should not exist after Remove: %s", string(item))
		}
	})

	t.Run("AddAndRemove_RemoveEmptyItem", func(t *testing.T) {
		removeReq := &pb.RemoveRequest{Item: []byte("")}
		removeResp, err := client.Remove(ctx, removeReq)
		require.NoError(t, err)
		assert.False(t, removeResp.Success, "Remove should fail for empty item")
		assert.NotEmpty(t, removeResp.Error, "Remove should return error for empty item")
	})

	t.Run("AddAndRemove_RemoveNonExistentItem", func(t *testing.T) {
		removeReq := &pb.RemoveRequest{Item: []byte("non-existent-item")}
		removeResp, err := client.Remove(ctx, removeReq)
		require.NoError(t, err)
		// Remove on non-existent item should still succeed (no-op in counting bloom filter)
		assert.True(t, removeResp.Success, "Remove should succeed even for non-existent item")
	})
}

// TestBatchAdd 验证批量添加功能
// 测试场景：批量添加多个元素，验证成功数量
func TestBatchAdd(t *testing.T) {
	conn, client, ctx := createTestClient(t, testServerAddr)
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

	t.Run("BatchAdd_1000Items", func(t *testing.T) {
		// Prepare 1000 test items
		items := make([][]byte, 1000)
		for i := 0; i < 1000; i++ {
			items[i] = []byte("batch-item-" + string(rune(i)))
		}

		// Batch add all items
		req := &pb.BatchAddRequest{Items: items}
		resp, err := client.BatchAdd(ctx, req)
		require.NoError(t, err, "BatchAdd RPC should not return error")
		assert.Equal(t, int32(1000), resp.SuccessCount, "All 1000 items should be added successfully")
		assert.Equal(t, int32(0), resp.FailureCount, "No items should fail")
		// Note: Errors array may contain empty strings for successful items
		assert.Len(t, resp.Errors, 1000, "Errors array should have 1000 entries (one per item)")

		// Wait for Raft to apply
		time.Sleep(500 * time.Millisecond)

		// Verify a sample of items exist
		sampleIndices := []int{0, 100, 500, 999}
		for _, idx := range sampleIndices {
			containsReq := &pb.ContainsRequest{Item: items[idx]}
			containsResp, err := client.Contains(ctx, containsReq)
			require.NoError(t, err)
			assert.True(t, containsResp.Exists, "Sample item %d should exist", idx)
		}
	})

	t.Run("BatchAdd_WithEmptyItems", func(t *testing.T) {
		items := [][]byte{
			[]byte("valid-item-1"),
			[]byte(""), // empty item
			[]byte("valid-item-2"),
			[]byte(""), // empty item
			[]byte("valid-item-3"),
		}

		req := &pb.BatchAddRequest{Items: items}
		resp, err := client.BatchAdd(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, int32(3), resp.SuccessCount, "3 valid items should succeed")
		assert.Equal(t, int32(2), resp.FailureCount, "2 empty items should fail")
		assert.Len(t, resp.Errors, 5, "Errors array should have 5 entries")
		assert.NotEmpty(t, resp.Errors[1], "Error for empty item at index 1")
		assert.NotEmpty(t, resp.Errors[3], "Error for empty item at index 3")

		time.Sleep(200 * time.Millisecond)

		// Verify valid items were added
		for i, item := range items {
			if len(item) > 0 {
				containsReq := &pb.ContainsRequest{Item: item}
				containsResp, err := client.Contains(ctx, containsReq)
				require.NoError(t, err)
				assert.True(t, containsResp.Exists, "Valid item at index %d should exist", i)
			}
		}
	})

	t.Run("BatchAdd_EmptyList", func(t *testing.T) {
		req := &pb.BatchAddRequest{Items: [][]byte{}}
		resp, err := client.BatchAdd(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, int32(0), resp.SuccessCount)
		assert.Equal(t, int32(0), resp.FailureCount)
	})

	t.Run("BatchAdd_DuplicateItems", func(t *testing.T) {
		items := [][]byte{
			[]byte("duplicate-item"),
			[]byte("duplicate-item"),
			[]byte("duplicate-item"),
		}

		req := &pb.BatchAddRequest{Items: items}
		resp, err := client.BatchAdd(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, int32(3), resp.SuccessCount, "Duplicate items should all succeed (counting bloom filter)")

		time.Sleep(200 * time.Millisecond)

		// Verify item exists
		containsReq := &pb.ContainsRequest{Item: []byte("duplicate-item")}
		containsResp, err := client.Contains(ctx, containsReq)
		require.NoError(t, err)
		assert.True(t, containsResp.Exists)
	})
}

// TestBatchContains 验证批量查询功能
// 测试场景：批量查询多个元素，验证返回结果正确
func TestBatchContains(t *testing.T) {
	conn, client, ctx := createTestClient(t, testServerAddr)
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

	// Prepare test data: add some items first
	testItems := [][]byte{
		[]byte("exists-1"),
		[]byte("exists-2"),
		[]byte("exists-3"),
	}

	for _, item := range testItems {
		addReq := &pb.AddRequest{Item: item}
		addResp, err := client.Add(ctx, addReq)
		require.NoError(t, err)
		assert.True(t, addResp.Success)
	}

	time.Sleep(200 * time.Millisecond)

	t.Run("BatchContains_MixedResults", func(t *testing.T) {
		queryItems := [][]byte{
			[]byte("exists-1"),      // exists
			[]byte("not-exists-1"),  // doesn't exist
			[]byte("exists-2"),      // exists
			[]byte("not-exists-2"),  // doesn't exist
			[]byte("exists-3"),      // exists
		}

		req := &pb.BatchContainsRequest{Items: queryItems}
		resp, err := client.BatchContains(ctx, req)
		require.NoError(t, err)
		assert.Len(t, resp.Results, 5, "Should return 5 results")

		// Verify results: true, false, true, false, true
		expectedResults := []bool{true, false, true, false, true}
		for i, expected := range expectedResults {
			assert.Equal(t, expected, resp.Results[i], "Result %d should match expected", i)
		}
		assert.Empty(t, resp.Error)
	})

	t.Run("BatchContains_AllExist", func(t *testing.T) {
		req := &pb.BatchContainsRequest{Items: testItems}
		resp, err := client.BatchContains(ctx, req)
		require.NoError(t, err)
		assert.Len(t, resp.Results, 3)
		for i, result := range resp.Results {
			assert.True(t, result, "Item %d should exist", i)
		}
	})

	t.Run("BatchContains_NoneExist", func(t *testing.T) {
		queryItems := [][]byte{
			[]byte("never-added-1"),
			[]byte("never-added-2"),
			[]byte("never-added-3"),
		}

		req := &pb.BatchContainsRequest{Items: queryItems}
		resp, err := client.BatchContains(ctx, req)
		require.NoError(t, err)
		assert.Len(t, resp.Results, 3)
		for i, result := range resp.Results {
			assert.False(t, result, "Item %d should not exist", i)
		}
	})

	t.Run("BatchContains_EmptyList", func(t *testing.T) {
		req := &pb.BatchContainsRequest{Items: [][]byte{}}
		resp, err := client.BatchContains(ctx, req)
		require.NoError(t, err)
		assert.Empty(t, resp.Results)
		// Note: Server may return an informational message for empty list, which is acceptable
	})

	t.Run("BatchContains_WithEmptyItem", func(t *testing.T) {
		queryItems := [][]byte{
			[]byte("exists-1"),
			[]byte(""), // empty item
			[]byte("exists-2"),
		}

		req := &pb.BatchContainsRequest{Items: queryItems}
		resp, err := client.BatchContains(ctx, req)
		require.NoError(t, err)
		assert.Len(t, resp.Results, 3)
		// Empty item should return false (not found)
		assert.False(t, resp.Results[1], "Empty item should return false")
	})
}

// TestGetStats 验证统计接口功能
// 测试场景：获取服务器统计信息，验证字段正确
func TestGetStats(t *testing.T) {
	conn, client, ctx := createTestClient(t, testServerAddr)
	defer func() {
		if conn != nil {
			conn.Close()
		}
	}()

	t.Run("GetStats_BasicInfo", func(t *testing.T) {
		req := &pb.GetStatsRequest{}
		resp, err := client.GetStats(ctx, req)
		require.NoError(t, err, "GetStats RPC should not return error")
		assert.Empty(t, resp.Error, "GetStats should not have error message")

		// Verify basic fields
		assert.NotEmpty(t, resp.NodeId, "Node ID should not be empty")
		assert.Equal(t, "test-node1", resp.NodeId, "Node ID should match configured value")
		assert.True(t, resp.IsLeader, "Node should be the leader")
		assert.NotEmpty(t, resp.RaftState, "Raft state should not be empty")
		assert.Contains(t, []string{"Leader", "Follower", "Candidate"}, resp.RaftState, "Raft state should be valid")

		// Verify Bloom filter config
		assert.Equal(t, int64(10000), resp.BloomSize, "Bloom filter size should be 10000 bits")
		assert.Equal(t, int32(3), resp.BloomK, "Bloom filter K should be 3")

		// Verify ports
		assert.Equal(t, int32(50052), resp.RaftPort, "Raft port should be 50052")
	})

	t.Run("GetStats_AfterAddingItems", func(t *testing.T) {
		// Add some items first
		for i := 0; i < 100; i++ {
			addReq := &pb.AddRequest{Item: []byte("stats-test-item-" + string(rune(i)))}
			addResp, err := client.Add(ctx, addReq)
			require.NoError(t, err)
			assert.True(t, addResp.Success)
		}

		time.Sleep(300 * time.Millisecond)

		// Get stats
		req := &pb.GetStatsRequest{}
		resp, err := client.GetStats(ctx, req)
		require.NoError(t, err)

		// Bloom count should be greater than 0
		// Note: Counting Bloom Filter uses approximate counting, so count may be higher than actual
		assert.Greater(t, resp.BloomCount, int64(0), "Bloom count should be > 0 after adding items")
	})

	t.Run("GetStats_LeaderInfo", func(t *testing.T) {
		req := &pb.GetStatsRequest{}
		resp, err := client.GetStats(ctx, req)
		require.NoError(t, err)

		// Leader should be set (either address or node ID)
		// In single-node cluster, this will be the node's address
		assert.NotEmpty(t, resp.Leader, "Leader should be set")
	})
}

// Helper functions

// createTestClient creates a gRPC client connection to the server
func createTestClient(t *testing.T, addr string) (*grpc.ClientConn, pb.DBFServiceClient, context.Context) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err, "Failed to connect to server")

	client := pb.NewDBFServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)

	return conn, client, ctx
}
