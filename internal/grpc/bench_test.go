package grpc

import (
	"context"
	"fmt"
	"testing"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
)

// BenchmarkGRPCAdd measures gRPC Add operation performance
func BenchmarkGRPCAdd(b *testing.B) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := []byte(fmt.Sprintf("benchmark-item-%d", i))
		_, _ = service.Add(ctx, &proto.AddRequest{
			Auth: &proto.AuthMetadata{},
			Item: item,
		})
	}
}

// BenchmarkGRPCContains measures gRPC Contains operation performance
func BenchmarkGRPCContains(b *testing.B) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)
	ctx := context.Background()

	// Pre-populate with some items
	for i := 0; i < 1000; i++ {
		_, _ = service.Add(ctx, &proto.AddRequest{
			Auth: &proto.AuthMetadata{},
			Item: []byte(fmt.Sprintf("item-%d", i)),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.Contains(ctx, &proto.ContainsRequest{
			Auth: &proto.AuthMetadata{},
			Item: []byte(fmt.Sprintf("item-%d", i%1000)),
		})
	}
}

// BenchmarkGRPCRemove measures gRPC Remove operation performance
func BenchmarkGRPCRemove(b *testing.B) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := []byte(fmt.Sprintf("benchmark-remove-item-%d", i))
		
		// Add first
		_, _ = service.Add(ctx, &proto.AddRequest{
			Auth: &proto.AuthMetadata{},
			Item: item,
		})
		
		// Then remove
		_, _ = service.Remove(ctx, &proto.RemoveRequest{
			Auth: &proto.AuthMetadata{},
			Item: item,
		})
	}
}

// BenchmarkGRPCBatchAdd measures gRPC BatchAdd operation performance
func BenchmarkGRPCBatchAdd(b *testing.B) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)
	ctx := context.Background()

	items := make([][]byte, 100)
	for i := 0; i < 100; i++ {
		items[i] = []byte(fmt.Sprintf("batch-item-%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.BatchAdd(ctx, &proto.BatchAddRequest{
			Auth:  &proto.AuthMetadata{},
			Items: items,
		})
	}
}

// BenchmarkGRPCBatchContains measures gRPC BatchContains operation performance
func BenchmarkGRPCBatchContains(b *testing.B) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)
	ctx := context.Background()

	// Pre-populate
	items := make([][]byte, 100)
	for i := 0; i < 100; i++ {
		items[i] = []byte(fmt.Sprintf("batch-item-%d", i))
		_, _ = service.Add(ctx, &proto.AddRequest{
			Auth: &proto.AuthMetadata{},
			Item: items[i],
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.BatchContains(ctx, &proto.BatchContainsRequest{
			Auth:  &proto.AuthMetadata{},
			Items: items,
		})
	}
}

// BenchmarkGRPCGetStats measures gRPC GetStats operation performance
func BenchmarkGRPCGetStats(b *testing.B) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.GetStats(ctx, &proto.GetStatsRequest{
			Auth: &proto.AuthMetadata{},
		})
	}
}

// BenchmarkGRPCAdd_WithAuth measures Add performance with authentication
func BenchmarkGRPCAdd_WithAuth(b *testing.B) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)
	ctx := context.Background()

	// Create valid auth metadata
	auth := &proto.AuthMetadata{
		ApiKey:    "test-key",
		Timestamp: 1234567890,
		Signature: "test-signature",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := []byte(fmt.Sprintf("auth-item-%d", i))
		_, _ = service.Add(ctx, &proto.AddRequest{
			Auth: auth,
			Item: item,
		})
	}
}

// BenchmarkGRPCParallelAdd measures concurrent Add performance
func BenchmarkGRPCParallelAdd(b *testing.B) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		i := 0
		for pb.Next() {
			item := []byte(fmt.Sprintf("parallel-item-%d", i))
			_, _ = service.Add(ctx, &proto.AddRequest{
				Auth: &proto.AuthMetadata{},
				Item: item,
			})
			i++
		}
	})
}

// BenchmarkGRPCParallelContains measures concurrent Contains performance
func BenchmarkGRPCParallelContains(b *testing.B) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)
	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		_, _ = service.Add(ctx, &proto.AddRequest{
			Auth: &proto.AuthMetadata{},
			Item: []byte(fmt.Sprintf("item-%d", i)),
		})
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, _ = service.Contains(ctx, &proto.ContainsRequest{
				Auth: &proto.AuthMetadata{},
				Item: []byte(fmt.Sprintf("item-%d", i%1000)),
			})
			i++
		}
	})
}

// BenchmarkGRPCAdd_NotLeader measures Add performance when not leader (redirect case)
func BenchmarkGRPCAdd_NotLeader(b *testing.B) {
	mockNode := newMockRaftNode()
	mockNode.isLeader = false
	service := NewDBFService(mockNode)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := []byte(fmt.Sprintf("not-leader-item-%d", i))
		_, _ = service.Add(ctx, &proto.AddRequest{
			Auth: &proto.AuthMetadata{},
			Item: item,
		})
	}
}

// BenchmarkGRPC_EmptyItem measures performance with empty item validation
func BenchmarkGRPC_EmptyItem(b *testing.B) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.Add(ctx, &proto.AddRequest{
			Auth: &proto.AuthMetadata{},
			Item: []byte{}, // Empty item
		})
	}
}

// BenchmarkGRPC_NilItem measures performance with nil item validation
func BenchmarkGRPC_NilItem(b *testing.B) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.Add(ctx, &proto.AddRequest{
			Auth: &proto.AuthMetadata{},
			Item: nil, // Nil item
		})
	}
}

// BenchmarkGRPCBatchAdd_Empty measures BatchAdd with empty items list
func BenchmarkGRPCBatchAdd_Empty(b *testing.B) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.BatchAdd(ctx, &proto.BatchAddRequest{
			Auth:  &proto.AuthMetadata{},
			Items: [][]byte{}, // Empty list
		})
	}
}

// BenchmarkGRPCBatchContains_Empty measures BatchContains with empty items list
func BenchmarkGRPCBatchContains_Empty(b *testing.B) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.BatchContains(ctx, &proto.BatchContainsRequest{
			Auth:  &proto.AuthMetadata{},
			Items: [][]byte{}, // Empty list
		})
	}
}

// BenchmarkGRPC_MixedWorkload measures mixed gRPC operation performance
func BenchmarkGRPC_MixedWorkload(b *testing.B) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		item := []byte(fmt.Sprintf("mixed-item-%d", i))
		
		// 40% Add, 40% Contains, 10% Remove, 10% GetStats
		switch i % 10 {
		case 0, 1, 2, 3:
			_, _ = service.Add(ctx, &proto.AddRequest{
				Auth: &proto.AuthMetadata{},
				Item: item,
			})
		case 4, 5, 6, 7:
			_, _ = service.Contains(ctx, &proto.ContainsRequest{
				Auth: &proto.AuthMetadata{},
				Item: item,
			})
		case 8:
			_, _ = service.Remove(ctx, &proto.RemoveRequest{
				Auth: &proto.AuthMetadata{},
				Item: item,
			})
		case 9:
			_, _ = service.GetStats(ctx, &proto.GetStatsRequest{
				Auth: &proto.AuthMetadata{},
			})
		}
	}
}
