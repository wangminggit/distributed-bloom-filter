package grpc

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
	"github.com/wangminggit/distributed-bloom-filter/internal/raft"
)

// DBFServer implements the gRPC DBFService.
type DBFServer struct {
	proto.UnimplementedDBFServiceServer
	raftNode *raft.Node
}

// NewDBFServer creates a new gRPC server.
func NewDBFServer(raftNode *raft.Node) *DBFServer {
	return &DBFServer{
		raftNode: raftNode,
	}
}

// Add adds an item to the Bloom filter.
func (s *DBFServer) Add(ctx context.Context, req *proto.AddRequest) (*proto.AddResponse, error) {
	if req.Item == nil || len(req.Item) == 0 {
		return &proto.AddResponse{
			Success: false,
			Error:   "item cannot be empty",
		}, nil
	}

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
func (s *DBFServer) Remove(ctx context.Context, req *proto.RemoveRequest) (*proto.RemoveResponse, error) {
	if req.Item == nil || len(req.Item) == 0 {
		return &proto.RemoveResponse{
			Success: false,
			Error:   "item cannot be empty",
		}, nil
	}

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
func (s *DBFServer) Contains(ctx context.Context, req *proto.ContainsRequest) (*proto.ContainsResponse, error) {
	if req.Item == nil || len(req.Item) == 0 {
		return &proto.ContainsResponse{
			Exists: false,
			Error:  "item cannot be empty",
		}, nil
	}

	exists := s.raftNode.Contains(req.Item)
	return &proto.ContainsResponse{
		Exists: exists,
		Error:  "",
	}, nil
}

// BatchAdd adds multiple items to the Bloom filter.
func (s *DBFServer) BatchAdd(ctx context.Context, req *proto.BatchAddRequest) (*proto.BatchAddResponse, error) {
	if len(req.Items) == 0 {
		return &proto.BatchAddResponse{
			SuccessCount: 0,
			FailureCount: 0,
			Errors:       []string{"no items provided"},
		}, nil
	}

	successCount := 0
	failureCount := 0
	errors := make([]string, len(req.Items))

	for i, item := range req.Items {
		if item == nil || len(item) == 0 {
			errors[i] = "item cannot be empty"
			failureCount++
			continue
		}

		err := s.raftNode.Add(item)
		if err != nil {
			errors[i] = fmt.Sprintf("failed to add item: %v", err)
			failureCount++
		} else {
			successCount++
		}
	}

	return &proto.BatchAddResponse{
		SuccessCount: int32(successCount),
		FailureCount: int32(failureCount),
		Errors:       errors,
	}, nil
}

// BatchContains checks if multiple items exist in the Bloom filter.
func (s *DBFServer) BatchContains(ctx context.Context, req *proto.BatchContainsRequest) (*proto.BatchContainsResponse, error) {
	if len(req.Items) == 0 {
		return &proto.BatchContainsResponse{
			Results: []bool{},
			Error:   "no items provided",
		}, nil
	}

	results := make([]bool, len(req.Items))
	for i, item := range req.Items {
		if item == nil || len(item) == 0 {
			results[i] = false
		} else {
			results[i] = s.raftNode.Contains(item)
		}
	}

	return &proto.BatchContainsResponse{
		Results: results,
		Error:   "",
	}, nil
}

// GetStats returns statistics about the Bloom filter and node.
func (s *DBFServer) GetStats(ctx context.Context, req *proto.GetStatsRequest) (*proto.GetStatsResponse, error) {
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

	leader, ok := state["leader"].(string)
	if !ok {
		leader = ""
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
	if bloomSizeVal, ok := state["bloom_size"].(int); ok && bloomSizeVal > 0 {
		// This is a simplified estimation; the actual count depends on the filter implementation
		bloomCount = int64(bloomSizeVal / 8) // Rough estimate
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

// Start starts the gRPC server on the specified port.
func (s *DBFServer) Start(port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer()
	proto.RegisterDBFServiceServer(grpcServer, s)

	log.Printf("gRPC server starting on port %d", port)
	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}
