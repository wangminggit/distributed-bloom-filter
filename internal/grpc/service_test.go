package grpc

import (
	"context"
	"testing"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
	"github.com/wangminggit/distributed-bloom-filter/internal/raft"
)

// mockRaftNode is a mock implementation of raft.RaftNode for testing.
type mockRaftNode struct {
	isLeader    bool
	leaderID    string
	leaderAddr  string
	items       map[string]bool
	nodeID      string
	raftState   string
	bloomSize   int
	bloomK      int
	raftPort    int
}

func newMockRaftNode() *mockRaftNode {
	return &mockRaftNode{
		isLeader:   true,
		leaderID:   "node-1",
		leaderAddr: "127.0.0.1:7000",
		items:      make(map[string]bool),
		nodeID:     "node-1",
		raftState:  "Leader",
		bloomSize:  10000,
		bloomK:     7,
		raftPort:   7000,
	}
}

func (m *mockRaftNode) Start() error                              { return nil }
func (m *mockRaftNode) Shutdown() error                           { return nil }
func (m *mockRaftNode) IsLeader() bool                            { return m.isLeader }
func (m *mockRaftNode) Add(item []byte) error                     { m.items[string(item)] = true; return nil }
func (m *mockRaftNode) Remove(item []byte) error                  { delete(m.items, string(item)); return nil }
func (m *mockRaftNode) Contains(item []byte) bool                 { return m.items[string(item)] }
func (m *mockRaftNode) BatchAdd(items [][]byte) (int, int, []string) {
	success := 0
	errors := make([]string, len(items))
	for i, item := range items {
		if len(item) == 0 {
			errors[i] = "empty item"
		} else {
			m.items[string(item)] = true
			success++
		}
	}
	return success, len(items) - success, errors
}
func (m *mockRaftNode) BatchContains(items [][]byte) []bool {
	results := make([]bool, len(items))
	for i, item := range items {
		results[i] = m.items[string(item)]
	}
	return results
}
func (m *mockRaftNode) GetState() map[string]interface{} {
	return map[string]interface{}{
		"node_id":        m.nodeID,
		"is_leader":      m.isLeader,
		"raft_state":     m.raftState,
		"leader":         m.leaderID,
		"leader_address": m.leaderAddr,
		"bloom_size":     m.bloomSize,
		"bloom_k":        m.bloomK,
		"raft_port":      m.raftPort,
	}
}
func (m *mockRaftNode) GetConfig() map[string]interface{} { return nil }

// Ensure mockRaftNode implements raft.RaftNode
var _ raft.RaftNode = (*mockRaftNode)(nil)

func TestDBFService_Add(t *testing.T) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)

	ctx := context.Background()

	// Test successful add
	resp, err := service.Add(ctx, &proto.AddRequest{
		Auth: &proto.AuthMetadata{},
		Item: []byte("test-item"),
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if !resp.Success {
		t.Errorf("Add() success = false, want true")
	}

	// Test empty item
	resp, err = service.Add(ctx, &proto.AddRequest{
		Auth: &proto.AuthMetadata{},
		Item: []byte{},
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if resp.Success {
		t.Errorf("Add() success = true, want false for empty item")
	}
}

func TestDBFService_Add_NotLeader(t *testing.T) {
	mockNode := newMockRaftNode()
	mockNode.isLeader = false
	service := NewDBFService(mockNode)

	ctx := context.Background()

	resp, err := service.Add(ctx, &proto.AddRequest{
		Auth: &proto.AuthMetadata{},
		Item: []byte("test-item"),
	})
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if resp.Success {
		t.Errorf("Add() success = true, want false when not leader")
	}
	if resp.Error == "" {
		t.Errorf("Add() error = empty, want redirect message")
	}
}

func TestDBFService_Remove(t *testing.T) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)

	ctx := context.Background()

	// Add item first
	_, _ = service.Add(ctx, &proto.AddRequest{
		Auth: &proto.AuthMetadata{},
		Item: []byte("test-item"),
	})

	// Test successful remove
	resp, err := service.Remove(ctx, &proto.RemoveRequest{
		Auth: &proto.AuthMetadata{},
		Item: []byte("test-item"),
	})
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if !resp.Success {
		t.Errorf("Remove() success = false, want true")
	}
}

func TestDBFService_Contains(t *testing.T) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)

	ctx := context.Background()

	// Add item first
	_, _ = service.Add(ctx, &proto.AddRequest{
		Auth: &proto.AuthMetadata{},
		Item: []byte("test-item"),
	})

	// Test contains - should exist
	resp, err := service.Contains(ctx, &proto.ContainsRequest{
		Auth: &proto.AuthMetadata{},
		Item: []byte("test-item"),
	})
	if err != nil {
		t.Fatalf("Contains() error = %v", err)
	}
	if !resp.Exists {
		t.Errorf("Contains() exists = false, want true")
	}

	// Test contains - should not exist
	resp, err = service.Contains(ctx, &proto.ContainsRequest{
		Auth: &proto.AuthMetadata{},
		Item: []byte("non-existent"),
	})
	if err != nil {
		t.Fatalf("Contains() error = %v", err)
	}
	if resp.Exists {
		t.Errorf("Contains() exists = true, want false")
	}
}

func TestDBFService_BatchAdd(t *testing.T) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)

	ctx := context.Background()

	// Test successful batch add
	resp, err := service.BatchAdd(ctx, &proto.BatchAddRequest{
		Auth:  &proto.AuthMetadata{},
		Items: [][]byte{[]byte("item1"), []byte("item2"), []byte("item3")},
	})
	if err != nil {
		t.Fatalf("BatchAdd() error = %v", err)
	}
	if resp.SuccessCount != 3 {
		t.Errorf("BatchAdd() success_count = %d, want 3", resp.SuccessCount)
	}
	if resp.FailureCount != 0 {
		t.Errorf("BatchAdd() failure_count = %d, want 0", resp.FailureCount)
	}
}

func TestDBFService_BatchContains(t *testing.T) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)

	ctx := context.Background()

	// Add items first
	_, _ = service.Add(ctx, &proto.AddRequest{
		Auth: &proto.AuthMetadata{},
		Item: []byte("item1"),
	})
	_, _ = service.Add(ctx, &proto.AddRequest{
		Auth: &proto.AuthMetadata{},
		Item: []byte("item2"),
	})

	// Test batch contains
	resp, err := service.BatchContains(ctx, &proto.BatchContainsRequest{
		Auth:  &proto.AuthMetadata{},
		Items: [][]byte{[]byte("item1"), []byte("item2"), []byte("item3")},
	})
	if err != nil {
		t.Fatalf("BatchContains() error = %v", err)
	}
	if len(resp.Results) != 3 {
		t.Errorf("BatchContains() results length = %d, want 3", len(resp.Results))
	}
	if !resp.Results[0] || !resp.Results[1] || resp.Results[2] {
		t.Errorf("BatchContains() results = %v, want [true, true, false]", resp.Results)
	}
}

func TestDBFService_GetStats(t *testing.T) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)

	ctx := context.Background()

	resp, err := service.GetStats(ctx, &proto.GetStatsRequest{
		Auth: &proto.AuthMetadata{},
	})
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if resp.NodeId != "node-1" {
		t.Errorf("GetStats() node_id = %s, want node-1", resp.NodeId)
	}
	if !resp.IsLeader {
		t.Errorf("GetStats() is_leader = false, want true")
	}
	if resp.RaftState != "Leader" {
		t.Errorf("GetStats() raft_state = %s, want Leader", resp.RaftState)
	}
	if resp.BloomSize != 10000 {
		t.Errorf("GetStats() bloom_size = %d, want 10000", resp.BloomSize)
	}
}

// Benchmark tests
func BenchmarkDBFService_Add(b *testing.B) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.Add(ctx, &proto.AddRequest{
			Auth: &proto.AuthMetadata{},
			Item: []byte("test-item"),
		})
	}
}

func BenchmarkDBFService_Contains(b *testing.B) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)
	ctx := context.Background()

	// Add item first
	_, _ = service.Add(ctx, &proto.AddRequest{
		Auth: &proto.AuthMetadata{},
		Item: []byte("test-item"),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.Contains(ctx, &proto.ContainsRequest{
			Auth: &proto.AuthMetadata{},
			Item: []byte("test-item"),
		})
	}
}
