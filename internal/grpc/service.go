package grpc

import (
	"context"
	"fmt"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
	"github.com/wangminggit/distributed-bloom-filter/internal/raft"
)

// DBFService implements the gRPC DBFService with Leader redirection support.
type DBFService struct {
	proto.UnimplementedDBFServiceServer
	raftNode raft.RaftNode
}

// NewDBFService creates a new gRPC service.
func NewDBFService(raftNode raft.RaftNode) *DBFService {
	return &DBFService{
		raftNode: raftNode,
	}
}

// Add adds an item to the Bloom filter.
// If this node is not the leader, returns a redirect response.
func (s *DBFService) Add(ctx context.Context, req *proto.AddRequest) (*proto.AddResponse, error) {
	// Check if item is valid
	if req.Item == nil || len(req.Item) == 0 {
		return &proto.AddResponse{
			Success: false,
			Error:   "item cannot be empty",
		}, nil
	}

	// Check if we're the leader
	if !s.raftNode.IsLeader() {
		leaderID, leaderAddr := s.getLeaderInfo()
		return &proto.AddResponse{
			Success: false,
			Error:   fmt.Sprintf("not the leader, redirect to: %s (%s)", leaderID, leaderAddr),
		}, nil
	}

	// Execute on leader
	err := s.raftNode.Add(req.Item)
	if err != nil {
		return &proto.AddResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to add item: %v", err),
		}, nil
	}

	return &proto.AddResponse{
		Success: true,
		Error:   "",
	}, nil
}

// Remove removes an item from the Bloom filter.
// If this node is not the leader, returns a redirect response.
func (s *DBFService) Remove(ctx context.Context, req *proto.RemoveRequest) (*proto.RemoveResponse, error) {
	// Check if item is valid
	if req.Item == nil || len(req.Item) == 0 {
		return &proto.RemoveResponse{
			Success: false,
			Error:   "item cannot be empty",
		}, nil
	}

	// Check if we're the leader
	if !s.raftNode.IsLeader() {
		leaderID, leaderAddr := s.getLeaderInfo()
		return &proto.RemoveResponse{
			Success: false,
			Error:   fmt.Sprintf("not the leader, redirect to: %s (%s)", leaderID, leaderAddr),
		}, nil
	}

	// Execute on leader
	err := s.raftNode.Remove(req.Item)
	if err != nil {
		return &proto.RemoveResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to remove item: %v", err),
		}, nil
	}

	return &proto.RemoveResponse{
		Success: true,
		Error:   "",
	}, nil
}

// Contains checks if an item exists in the Bloom filter.
// This is a read operation and can be served by any node.
func (s *DBFService) Contains(ctx context.Context, req *proto.ContainsRequest) (*proto.ContainsResponse, error) {
	// Check if item is valid
	if req.Item == nil || len(req.Item) == 0 {
		return &proto.ContainsResponse{
			Exists: false,
			Error:  "item cannot be empty",
		}, nil
	}

	// Read operations can be served by any node
	exists := s.raftNode.Contains(req.Item)
	return &proto.ContainsResponse{
		Exists: exists,
		Error:  "",
	}, nil
}

// BatchAdd adds multiple items to the Bloom filter.
// If this node is not the leader, returns a redirect response.
func (s *DBFService) BatchAdd(ctx context.Context, req *proto.BatchAddRequest) (*proto.BatchAddResponse, error) {
	// Check if we have items
	if len(req.Items) == 0 {
		return &proto.BatchAddResponse{
			SuccessCount: 0,
			FailureCount: 0,
			Errors:       []string{"no items provided"},
		}, nil
	}

	// Check if we're the leader
	if !s.raftNode.IsLeader() {
		leaderID, leaderAddr := s.getLeaderInfo()
		return &proto.BatchAddResponse{
			SuccessCount: 0,
			FailureCount: int32(len(req.Items)),
			Errors:       []string{fmt.Sprintf("not the leader, redirect to: %s (%s)", leaderID, leaderAddr)},
		}, nil
	}

	// Execute on leader
	successCount, failureCount, errors := s.raftNode.BatchAdd(req.Items)
	return &proto.BatchAddResponse{
		SuccessCount: int32(successCount),
		FailureCount: int32(failureCount),
		Errors:       errors,
	}, nil
}

// BatchContains checks if multiple items exist in the Bloom filter.
// This is a read operation and can be served by any node.
func (s *DBFService) BatchContains(ctx context.Context, req *proto.BatchContainsRequest) (*proto.BatchContainsResponse, error) {
	// Check if we have items
	if len(req.Items) == 0 {
		return &proto.BatchContainsResponse{
			Results: []bool{},
			Error:   "no items provided",
		}, nil
	}

	// Read operations can be served by any node
	results := s.raftNode.BatchContains(req.Items)
	return &proto.BatchContainsResponse{
		Results: results,
		Error:   "",
	}, nil
}

// GetStats returns statistics about the Bloom filter and node.
func (s *DBFService) GetStats(ctx context.Context, req *proto.GetStatsRequest) (*proto.GetStatsResponse, error) {
	state := s.raftNode.GetState()

	nodeID, ok := state["node_id"].(string)
	if !ok {
		nodeID = "unknown"
	}

	isLeader, ok := state["is_leader"].(bool)
	if !ok {
		isLeader = false
	}

	raftState, ok := state["raft_state"].(string)
	if !ok {
		raftState = "unknown"
	}

	leader, ok := state["leader_address"].(string)
	if !ok {
		leader, ok = state["leader"].(string)
		if !ok {
			leader = ""
		}
	}

	bloomSize, ok := state["bloom_size"].(int)
	if !ok {
		bloomSize = 0
	}

	bloomK, ok := state["bloom_k"].(int)
	if !ok {
		bloomK = 0
	}

	raftPort, ok := state["raft_port"].(int)
	if !ok {
		raftPort = 0
	}

	// Calculate approximate count from Bloom filter
	bloomCount := int64(0)
	if n, ok := state["bloom_count"].(int64); ok {
		bloomCount = n
	}

	return &proto.GetStatsResponse{
		NodeId:     nodeID,
		IsLeader:   isLeader,
		RaftState:  raftState,
		Leader:     leader,
		BloomSize:  int64(bloomSize),
		BloomK:     int32(bloomK),
		BloomCount: bloomCount,
		RaftPort:   int32(raftPort),
		Error:      "",
	}, nil
}

// getLeaderInfo returns the leader ID and address.
func (s *DBFService) getLeaderInfo() (string, string) {
	state := s.raftNode.GetState()
	
	leaderID, ok := state["leader"].(string)
	if !ok {
		leaderID = "unknown"
	}
	
	leaderAddr, ok := state["leader_address"].(string)
	if !ok {
		leaderAddr = ""
	}
	
	return leaderID, leaderAddr
}
