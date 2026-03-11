package integration

import (
	"context"
	"testing"
	"time"

	pb "github.com/wangminggit/distributed-bloom-filter/api/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestAddAndContains 验证 Add + Contains 操作
// 优先级：P0 - 最基础功能测试
func TestAddAndContains(t *testing.T) {
	// 1. 连接服务器
	conn, err := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	defer conn.Close()

	client := pb.NewDBFServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 2. 添加元素
	item := []byte("test-item-1")
	addResp, err := client.Add(ctx, &pb.AddRequest{Item: item})
	assert.NoError(t, err)
	assert.True(t, addResp.Success, "Add operation should succeed")

	// 3. 验证元素存在
	containsResp, err := client.Contains(ctx, &pb.ContainsRequest{Item: item})
	assert.NoError(t, err)
	assert.True(t, containsResp.Exists, "Element should exist after Add")

	t.Log("✅ TestAddAndContains passed")
}

// TestAddAndRemove 验证 Add + Remove 操作
// 优先级：P0 - 基础删除功能
func TestAddAndRemove(t *testing.T) {
	// 1. 连接服务器
	conn, err := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	defer conn.Close()

	client := pb.NewDBFServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 2. 添加元素
	item := []byte("test-item-2")
	addResp, err := client.Add(ctx, &pb.AddRequest{Item: item})
	assert.NoError(t, err)
	assert.True(t, addResp.Success)

	// 3. 验证添加成功
	containsResp, err := client.Contains(ctx, &pb.ContainsRequest{Item: item})
	assert.NoError(t, err)
	assert.True(t, containsResp.Exists)

	// 4. 删除元素
	removeResp, err := client.Remove(ctx, &pb.RemoveRequest{Item: item})
	assert.NoError(t, err)
	assert.True(t, removeResp.Success, "Remove operation should succeed")

	// 5. 验证元素不存在（注意：Bloom Filter 删除后可能仍有假阳性）
	// 对于 Counting Bloom Filter，删除后应该返回 false
	containsResp2, err := client.Contains(ctx, &pb.ContainsRequest{Item: item})
	assert.NoError(t, err)
	assert.False(t, containsResp2.Exists, "Element should not exist after Remove")

	t.Log("✅ TestAddAndRemove passed")
}

// TestBatchAdd 验证批量添加
// 优先级：P0 - 批量操作基础功能
func TestBatchAdd(t *testing.T) {
	// 1. 连接服务器
	conn, err := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	defer conn.Close()

	client := pb.NewDBFServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 2. 准备批量数据（100 个元素）
	items := make([][]byte, 100)
	for i := 0; i < 100; i++ {
		items[i] = []byte("batch-item-" + string(rune('a'+i%26)))
	}

	// 3. 批量添加
	batchResp, err := client.BatchAdd(ctx, &pb.BatchAddRequest{Items: items})
	assert.NoError(t, err)
	assert.Equal(t, int32(100), batchResp.SuccessCount, "All items should be added successfully")
	assert.Equal(t, int32(0), batchResp.FailureCount, "No items should fail")

	t.Logf("✅ BatchAdd: %d items added successfully", batchResp.SuccessCount)
}

// TestBatchContains 验证批量查询
// 优先级：P0 - 批量查询功能
func TestBatchContains(t *testing.T) {
	// 1. 连接服务器
	conn, err := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	defer conn.Close()

	client := pb.NewDBFServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 2. 先添加一些元素
	items := [][]byte{
		[]byte("query-item-1"),
		[]byte("query-item-2"),
		[]byte("query-item-3"),
	}

	_, err = client.BatchAdd(ctx, &pb.BatchAddRequest{Items: items})
	assert.NoError(t, err)

	// 3. 批量查询（包含存在和不存在的元素）
	queryItems := [][]byte{
		[]byte("query-item-1"),      // 存在
		[]byte("query-item-2"),      // 存在
		[]byte("query-item-3"),      // 存在
		[]byte("nonexistent-item"),  // 不存在
	}

	batchResp, err := client.BatchContains(ctx, &pb.BatchContainsRequest{Items: queryItems})
	assert.NoError(t, err)
	assert.Len(t, batchResp.Results, 4, "Should return 4 results")

	// 4. 验证结果
	assert.True(t, batchResp.Results[0], "query-item-1 should exist")
	assert.True(t, batchResp.Results[1], "query-item-2 should exist")
	assert.True(t, batchResp.Results[2], "query-item-3 should exist")
	assert.False(t, batchResp.Results[3], "nonexistent-item should not exist")

	t.Log("✅ TestBatchContains passed")
}

// TestGetStats 验证统计接口
// 优先级：P0 - 监控和调试功能
func TestGetStats(t *testing.T) {
	// 1. 连接服务器
	conn, err := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	defer conn.Close()

	client := pb.NewDBFServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 2. 获取统计信息
	statsResp, err := client.GetStats(ctx, &pb.GetStatsRequest{})
	assert.NoError(t, err)

	// 3. 验证统计信息
	assert.NotEmpty(t, statsResp.NodeId, "Node ID should not be empty")
	assert.True(t, statsResp.IsLeader, "Node should be leader in single-node mode")
	assert.Equal(t, "Leader", statsResp.RaftState, "Raft state should be Leader")
	assert.NotEmpty(t, statsResp.Leader, "Leader address should not be empty")
	assert.Greater(t, statsResp.BloomSize, int64(0), "Bloom filter size should be > 0")
	assert.Greater(t, statsResp.BloomK, int32(0), "Bloom filter K should be > 0")
	assert.GreaterOrEqual(t, statsResp.BloomCount, int64(0), "Bloom count should be >= 0")
	assert.Equal(t, int32(8081), statsResp.RaftPort, "Raft port should be 8081")

	t.Logf("✅ Server Stats: Node=%s, Leader=%v, BloomSize=%d, Count=~%d",
		statsResp.NodeId, statsResp.IsLeader, statsResp.BloomSize, statsResp.BloomCount)
}
