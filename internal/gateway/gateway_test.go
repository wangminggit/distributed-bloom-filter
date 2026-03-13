package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
)

// mockDBFServer is a mock gRPC server for testing.
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
		NodeId:    "test-node",
		IsLeader:  true,
		RaftState: "Leader",
		Leader:    "test-node",
	}, nil
}

func TestGateway_handleAdd(t *testing.T) {
	// Start mock gRPC server
	grpcServer := grpc.NewServer()
	mockServer := newMockDBFServer()
	proto.RegisterDBFServiceServer(grpcServer, mockServer)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	// Create gateway
	gateway, err := NewGateway(GatewayConfig{
		Port:        0,
		GRPCAddress: lis.Addr().String(),
		APIKey:      "test-key",
		APISecret:   "test-secret",
		EnableTLS:   false,
	})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}
	defer gateway.Stop()

	// Test add endpoint
	reqBody := `{"item": "test-item"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/add", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	gateway.handleAdd(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handleAdd() status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp APIResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("handleAdd() success = false, want true")
	}
}

func TestGateway_handleContains(t *testing.T) {
	// Start mock gRPC server
	grpcServer := grpc.NewServer()
	mockServer := newMockDBFServer()
	proto.RegisterDBFServiceServer(grpcServer, mockServer)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	// Create gateway
	gateway, err := NewGateway(GatewayConfig{
		Port:        0,
		GRPCAddress: lis.Addr().String(),
		APIKey:      "test-key",
		APISecret:   "test-secret",
		EnableTLS:   false,
	})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}
	defer gateway.Stop()

	// Add item first
	addBody := `{"item": "test-item"}`
	addReq := httptest.NewRequest(http.MethodPost, "/api/v1/add", bytes.NewBufferString(addBody))
	addReq.Header.Set("Content-Type", "application/json")
	addRR := httptest.NewRecorder()
	gateway.handleAdd(addRR, addReq)

	// Test contains endpoint
	reqBody := `{"item": "test-item"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/contains", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	gateway.handleContains(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handleContains() status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp APIResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("handleContains() success = false, want true")
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("handleContains() data is not a map")
	}
	if exists, ok := data["exists"].(bool); !ok || !exists {
		t.Errorf("handleContains() exists = %v, want true", exists)
	}
}

func TestGateway_handleBatchAdd(t *testing.T) {
	// Start mock gRPC server
	grpcServer := grpc.NewServer()
	mockServer := newMockDBFServer()
	proto.RegisterDBFServiceServer(grpcServer, mockServer)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	// Create gateway
	gateway, err := NewGateway(GatewayConfig{
		Port:        0,
		GRPCAddress: lis.Addr().String(),
		APIKey:      "test-key",
		APISecret:   "test-secret",
		EnableTLS:   false,
	})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}
	defer gateway.Stop()

	// Test batch add endpoint
	reqBody := `{"items": ["item1", "item2", "item3"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/batch/add", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	gateway.handleBatchAdd(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handleBatchAdd() status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp APIResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("handleBatchAdd() success = false, want true")
	}
}

func TestGateway_handleStatus(t *testing.T) {
	// Start mock gRPC server
	grpcServer := grpc.NewServer()
	mockServer := newMockDBFServer()
	proto.RegisterDBFServiceServer(grpcServer, mockServer)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	// Create gateway
	gateway, err := NewGateway(GatewayConfig{
		Port:        0,
		GRPCAddress: lis.Addr().String(),
		APIKey:      "test-key",
		APISecret:   "test-secret",
		EnableTLS:   false,
	})
	if err != nil {
		t.Fatalf("Failed to create gateway: %v", err)
	}
	defer gateway.Stop()

	// Test status endpoint
	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	rr := httptest.NewRecorder()

	gateway.handleStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handleStatus() status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp APIResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Errorf("handleStatus() success = false, want true")
	}
}

func TestGateway_corsMiddleware(t *testing.T) {
	gateway := &Gateway{}
	handler := gateway.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test OPTIONS request
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("CORS OPTIONS status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	headers := rr.Header()
	if headers.Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("CORS Allow-Origin = %s, want *", headers.Get("Access-Control-Allow-Origin"))
	}
}

func TestAPIResponse_JSON(t *testing.T) {
	resp := APIResponse{
		Success: true,
		Data:    map[string]bool{"exists": true},
		Leader:  "node-1:7000",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	var decoded APIResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !decoded.Success {
		t.Errorf("Unmarshaled success = false, want true")
	}
	if decoded.Leader != "node-1:7000" {
		t.Errorf("Unmarshaled leader = %s, want node-1:7000", decoded.Leader)
	}
}
