package sdk

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
)

// mockDBFServer for SDK testing
type mockDBFServer struct {
	proto.UnimplementedDBFServiceServer
	items map[string]bool
}

func newMockDBFServer() *mockDBFServer {
	return &mockDBFServer{
		items: make(map[string]bool),
	}
}

func (m *mockDBFServer) Add(ctx context.Context, req *proto.AddRequest) (*proto.AddResponse, error) {
	m.items[string(req.Item)] = true
	return &proto.AddResponse{Success: true}, nil
}

func (m *mockDBFServer) Remove(ctx context.Context, req *proto.RemoveRequest) (*proto.RemoveResponse, error) {
	delete(m.items, string(req.Item))
	return &proto.RemoveResponse{Success: true}, nil
}

func (m *mockDBFServer) Contains(ctx context.Context, req *proto.ContainsRequest) (*proto.ContainsResponse, error) {
	exists := m.items[string(req.Item)]
	return &proto.ContainsResponse{Exists: exists}, nil
}

func (m *mockDBFServer) BatchAdd(ctx context.Context, req *proto.BatchAddRequest) (*proto.BatchAddResponse, error) {
	successCount := 0
	errors := make([]string, len(req.Items))
	for i, item := range req.Items {
		if len(item) == 0 {
			errors[i] = "empty item"
		} else {
			m.items[string(item)] = true
			successCount++
		}
	}
	return &proto.BatchAddResponse{
		SuccessCount: int32(successCount),
		FailureCount: int32(len(req.Items) - successCount),
		Errors:       errors,
	}, nil
}

func (m *mockDBFServer) BatchContains(ctx context.Context, req *proto.BatchContainsRequest) (*proto.BatchContainsResponse, error) {
	results := make([]bool, len(req.Items))
	for i, item := range req.Items {
		results[i] = m.items[string(item)]
	}
	return &proto.BatchContainsResponse{Results: results}, nil
}

func (m *mockDBFServer) GetStats(ctx context.Context, req *proto.GetStatsRequest) (*proto.GetStatsResponse, error) {
	return &proto.GetStatsResponse{
		NodeId:     "test-node",
		IsLeader:   true,
		RaftState:  "Leader",
		Leader:     "test-node",
		BloomSize:  10000,
		BloomK:     7,
		BloomCount: 100,
		RaftPort:   7000,
	}, nil
}

func startMockServerT(t *testing.T) (string, func()) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	grpcServer := grpc.NewServer()
	mockServer := newMockDBFServer()
	proto.RegisterDBFServiceServer(grpcServer, mockServer)

	go func() {
		grpcServer.Serve(lis)
	}()

	return lis.Addr().String(), func() {
		grpcServer.Stop()
		lis.Close()
	}
}

func startMockServerB(b *testing.B) (string, func()) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("Failed to create listener: %v", err)
	}

	grpcServer := grpc.NewServer()
	mockServer := newMockDBFServer()
	proto.RegisterDBFServiceServer(grpcServer, mockServer)

	go func() {
		grpcServer.Serve(lis)
	}()

	return lis.Addr().String(), func() {
		grpcServer.Stop()
		lis.Close()
	}
}

func TestClient_Add(t *testing.T) {
	addr, cleanup := startMockServerT(t)
	defer cleanup()

	client, err := NewClient(ClientConfig{
		Addresses: []string{addr},
		APIKey:    "test-key",
		APISecret: "test-secret",
		EnableTLS: false,
		Timeout:   5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Test Add
	err = client.Add("test-item")
	if err != nil {
		t.Errorf("Add() error = %v, want nil", err)
	}

	// Test Contains
	exists, err := client.Contains("test-item")
	if err != nil {
		t.Errorf("Contains() error = %v, want nil", err)
	}
	if !exists {
		t.Errorf("Contains() exists = false, want true")
	}
}

func TestClient_Remove(t *testing.T) {
	addr, cleanup := startMockServerT(t)
	defer cleanup()

	client, err := NewClient(ClientConfig{
		Addresses: []string{addr},
		APIKey:    "test-key",
		APISecret: "test-secret",
		EnableTLS: false,
		Timeout:   5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Add item first
	_ = client.Add("test-item")

	// Test Remove
	err = client.Remove("test-item")
	if err != nil {
		t.Errorf("Remove() error = %v, want nil", err)
	}

	// Verify removal
	exists, err := client.Contains("test-item")
	if err != nil {
		t.Errorf("Contains() error = %v, want nil", err)
	}
	if exists {
		t.Errorf("Contains() after remove = true, want false")
	}
}

func TestClient_BatchAdd(t *testing.T) {
	addr, cleanup := startMockServerT(t)
	defer cleanup()

	client, err := NewClient(ClientConfig{
		Addresses: []string{addr},
		APIKey:    "test-key",
		APISecret: "test-secret",
		EnableTLS: false,
		Timeout:   5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Test BatchAdd
	items := []string{"item1", "item2", "item3"}
	err = client.BatchAdd(items)
	if err != nil {
		t.Errorf("BatchAdd() error = %v, want nil", err)
	}

	// Verify all items exist
	for _, item := range items {
		exists, err := client.Contains(item)
		if err != nil {
			t.Errorf("Contains(%s) error = %v, want nil", item, err)
		}
		if !exists {
			t.Errorf("Contains(%s) = false, want true", item)
		}
	}
}

func TestClient_BatchContains(t *testing.T) {
	addr, cleanup := startMockServerT(t)
	defer cleanup()

	client, err := NewClient(ClientConfig{
		Addresses: []string{addr},
		APIKey:    "test-key",
		APISecret: "test-secret",
		EnableTLS: false,
		Timeout:   5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Add some items
	_ = client.Add("item1")
	_ = client.Add("item2")

	// Test BatchContains
	items := []string{"item1", "item2", "item3"}
	results, err := client.BatchContains(items)
	if err != nil {
		t.Errorf("BatchContains() error = %v, want nil", err)
	}

	if !results["item1"] {
		t.Errorf("BatchContains(item1) = false, want true")
	}
	if !results["item2"] {
		t.Errorf("BatchContains(item2) = false, want true")
	}
	if results["item3"] {
		t.Errorf("BatchContains(item3) = true, want false")
	}
}

func TestClient_GetStatus(t *testing.T) {
	addr, cleanup := startMockServerT(t)
	defer cleanup()

	client, err := NewClient(ClientConfig{
		Addresses: []string{addr},
		APIKey:    "test-key",
		APISecret: "test-secret",
		EnableTLS: false,
		Timeout:   5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Test GetStatus
	status, err := client.GetStatus()
	if err != nil {
		t.Errorf("GetStatus() error = %v, want nil", err)
	}

	if status.NodeID != "test-node" {
		t.Errorf("GetStatus() NodeID = %s, want test-node", status.NodeID)
	}
	if !status.IsLeader {
		t.Errorf("GetStatus() IsLeader = false, want true")
	}
	if status.RaftState != "Leader" {
		t.Errorf("GetStatus() RaftState = %s, want Leader", status.RaftState)
	}
}

func TestClient_Close(t *testing.T) {
	addr, cleanup := startMockServerT(t)
	defer cleanup()

	client, err := NewClient(ClientConfig{
		Addresses: []string{addr},
		APIKey:    "test-key",
		APISecret: "test-secret",
		EnableTLS: false,
		Timeout:   5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test Close
	err = client.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestClient_DefaultConfig(t *testing.T) {
	config := DefaultClientConfig()

	if config.MaxRetries != 3 {
		t.Errorf("Default MaxRetries = %d, want 3", config.MaxRetries)
	}
	if config.RetryDelay != 100*time.Millisecond {
		t.Errorf("Default RetryDelay = %v, want 100ms", config.RetryDelay)
	}
	if config.Timeout != 5*time.Second {
		t.Errorf("Default Timeout = %v, want 5s", config.Timeout)
	}
}

func TestClient_NoAddresses(t *testing.T) {
	_, err := NewClient(ClientConfig{
		Addresses: []string{},
	})
	if err == nil {
		t.Errorf("NewClient() with no addresses should return error")
	}
}

func TestErrLeaderRedirect(t *testing.T) {
	err := ErrLeaderRedirect{Message: "not the leader, redirect to: node-1 (127.0.0.1:7000)"}

	if err.Error() == "" {
		t.Errorf("ErrLeaderRedirect.Error() returned empty string")
	}

	var target ErrLeaderRedirect
	if !As(err, &target) {
		t.Errorf("As() failed to extract ErrLeaderRedirect")
	}
	if target.Message != err.Message {
		t.Errorf("As() extracted wrong message: %s, want %s", target.Message, err.Message)
	}
}

func TestIsLeaderRedirect(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"not the leader, redirect to: node-1", true},
		{"Not the leader, redirect to: node-1", true},
		{"connection refused", false},
		{"", false},
		{"some other error", false},
	}

	for _, tt := range tests {
		got := isLeaderRedirect(tt.msg)
		if got != tt.want {
			t.Errorf("isLeaderRedirect(%q) = %v, want %v", tt.msg, got, tt.want)
		}
	}
}

func TestExtractLeaderAddress(t *testing.T) {
	tests := []struct {
		msg  string
		want string
	}{
		{"not the leader, redirect to: node-1 (127.0.0.1:7000)", "127.0.0.1:7000"},
		{"redirect to: node-2 (localhost:7001)", "localhost:7001"},
		{"no address here", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := extractLeaderAddress(tt.msg)
		if got != tt.want {
			t.Errorf("extractLeaderAddress(%q) = %q, want %q", tt.msg, got, tt.want)
		}
	}
}

// Benchmark tests
func BenchmarkClient_Add(b *testing.B) {
	addr, cleanup := startMockServerB(b)
	defer cleanup()

	client, err := NewClient(ClientConfig{
		Addresses: []string{addr},
		APIKey:    "test-key",
		APISecret: "test-secret",
		EnableTLS: false,
		Timeout:   5 * time.Second,
	})
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.Add("test-item")
	}
}

func BenchmarkClient_Contains(b *testing.B) {
	addr, cleanup := startMockServerB(b)
	defer cleanup()

	client, err := NewClient(ClientConfig{
		Addresses: []string{addr},
		APIKey:    "test-key",
		APISecret: "test-secret",
		EnableTLS: false,
		Timeout:   5 * time.Second,
	})
	if err != nil {
		b.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Add item first
	_ = client.Add("test-item")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = client.Contains("test-item")
	}
}
