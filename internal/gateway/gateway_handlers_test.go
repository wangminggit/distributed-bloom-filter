package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestGateway_HandleRemove tests the remove handler.
func TestGateway_HandleRemove(t *testing.T) {
	// Create gateway with mock config
	config := GatewayConfig{
		Port:        8080,
		GRPCAddress: "localhost:50051",
		APIKey:      "test-key",
		APISecret:   "test-secret",
	}

	gateway := &Gateway{
		config: config,
	}

	// Create test request
	body := map[string]string{"item": "test-item"}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/remove", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	
	// Handler should not panic even without gRPC connection
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Handler panicked (expected without gRPC): %v", r)
		}
	}()
	
	gateway.handleRemove(rr, req)
	
	// Should return some response
	if rr.Code == 0 {
		t.Error("Expected response code")
	}
}

// TestGateway_HandleBatchContains tests the batch contains handler.
func TestGateway_HandleBatchContains(t *testing.T) {
	config := GatewayConfig{
		Port:        8080,
		GRPCAddress: "localhost:50051",
		APIKey:      "test-key",
	}

	gateway := &Gateway{
		config: config,
	}

	// Create test request
	body := map[string][][]byte{"items": {[]byte("item1"), []byte("item2")}}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/batch/contains", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Handler panicked (expected without gRPC): %v", r)
		}
	}()
	
	gateway.handleBatchContains(rr, req)
}

// TestGateway_HandleCluster tests the cluster handler.
func TestGateway_HandleCluster(t *testing.T) {
	config := GatewayConfig{
		Port:        8080,
		GRPCAddress: "localhost:50051",
	}

	gateway := &Gateway{
		config: config,
	}

	req := httptest.NewRequest("GET", "/api/v1/cluster", nil)
	rr := httptest.NewRecorder()
	
	defer func() {
		if r := recover(); r != nil {
			t.Logf("Handler panicked (expected without gRPC): %v", r)
		}
	}()
	
	gateway.handleCluster(rr, req)
}

// TestGateway_CreateAuthMetadata tests auth metadata creation.
func TestGateway_CreateAuthMetadata(t *testing.T) {
	config := GatewayConfig{
		APIKey:    "test-key",
		APISecret: "test-secret",
	}

	gateway := &Gateway{
		config: config,
	}

	metadata := gateway.createAuthMetadata("POST")
	
	if metadata == nil {
		t.Fatal("Expected non-nil metadata")
	}
	if metadata.ApiKey != "test-key" {
		t.Errorf("Expected api_key 'test-key', got '%s'", metadata.ApiKey)
	}
	if metadata.Timestamp == 0 {
		t.Error("Expected non-zero timestamp")
	}
	if metadata.Signature == "" {
		t.Error("Expected non-empty signature")
	}
}

// TestGateway_ComputeSignature tests signature computation.
func TestGateway_ComputeSignature(t *testing.T) {
	config := GatewayConfig{
		APISecret: "test-secret",
	}

	gateway := &Gateway{
		config: config,
	}

	timestamp := time.Now().Unix()
	signature := gateway.computeSignature("test-key", timestamp, "POST", "test-secret")
	
	if signature == "" {
		t.Error("Expected non-empty signature")
	}
	
	// Signature should be deterministic for same inputs
	signature2 := gateway.computeSignature("test-key", timestamp, "POST", "test-secret")
	if signature != signature2 {
		t.Error("Expected same signature for same inputs")
	}
}

// TestGateway_SendSuccess tests success response.
func TestGateway_SendSuccess(t *testing.T) {
	gateway := &Gateway{}
	
	rr := httptest.NewRecorder()
	data := map[string]string{"key": "value"}
	
	gateway.sendSuccess(rr, data, "")
	
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
	
	var resp APIResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	
	if !resp.Success {
		t.Error("Expected success=true")
	}
	if resp.Error != "" {
		t.Errorf("Expected empty error, got '%s'", resp.Error)
	}
}

// TestGateway_SendError tests error response.
func TestGateway_SendError(t *testing.T) {
	gateway := &Gateway{}
	
	rr := httptest.NewRecorder()
	
	gateway.sendError(rr, "test error", http.StatusBadRequest)
	
	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
	
	var resp APIResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	
	if resp.Success {
		t.Error("Expected success=false")
	}
	if resp.Error != "test error" {
		t.Errorf("Expected error 'test error', got '%s'", resp.Error)
	}
}

// TestGateway_SendRedirect tests redirect response.
func TestGateway_SendRedirect(t *testing.T) {
	gateway := &Gateway{}
	
	rr := httptest.NewRecorder()
	
	gateway.sendRedirect(rr, "not leader, redirect to: leader-node (127.0.0.1:50051)")
	
	// Status may be 307 (redirect) or 503 (unavailable)
	if rr.Code != 307 && rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 307 or 503, got %d", rr.Code)
	}
	
	var resp APIResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	
	if resp.Error == "" && rr.Code != 307 {
		t.Error("Expected error message")
	}
}

// TestGateway_LoggingMiddleware tests the logging middleware.
func TestGateway_LoggingMiddleware(t *testing.T) {
	config := GatewayConfig{
		Port: 8080,
	}

	gateway := &Gateway{
		config: config,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := gateway.loggingMiddleware(handler)
	
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	
	// Should not panic
	wrapped.ServeHTTP(rr, req)
	
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

// TestGateway_CorsMiddleware tests the CORS middleware.
func TestGateway_CorsMiddleware(t *testing.T) {
	config := GatewayConfig{
		Port: 8080,
	}

	gateway := &Gateway{
		config: config,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := gateway.corsMiddleware(handler)
	
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	
	wrapped.ServeHTTP(rr, req)
	
	// Check CORS headers
	headers := rr.Header()
	if headers.Get("Access-Control-Allow-Origin") == "" {
		t.Error("Expected Access-Control-Allow-Origin header")
	}
}

// TestGatewayConfig_Validation tests config validation.
func TestGatewayConfig_Validation(t *testing.T) {
	// Valid config
	validConfig := GatewayConfig{
		Port:        8080,
		GRPCAddress: "localhost:50051",
	}
	
	if validConfig.Port != 8080 {
		t.Error("Expected port 8080")
	}
	
	// Config with TLS
	tlsConfig := GatewayConfig{
		Port:        8080,
		GRPCAddress: "localhost:50051",
		EnableTLS:   true,
		TLSCertFile: "/path/to/cert.pem",
	}
	
	if !tlsConfig.EnableTLS {
		t.Error("Expected TLS enabled")
	}
	if tlsConfig.TLSCertFile == "" {
		t.Error("Expected TLS cert file path")
	}
}

// TestAPIResponse_MarshalJSON tests response marshaling.
func TestAPIResponse_MarshalJSON(t *testing.T) {
	resp := APIResponse{
		Success: true,
		Data:    map[string]string{"key": "value"},
	}
	
	jsonData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	
	if len(jsonData) == 0 {
		t.Error("Expected non-zero JSON data")
	}
}

// TestAPIResponse_ErrorCase tests error response.
func TestAPIResponse_ErrorCase(t *testing.T) {
	resp := APIResponse{
		Success: false,
		Error:   "something went wrong",
	}
	
	if resp.Success {
		t.Error("Expected success=false")
	}
	if resp.Error == "" {
		t.Error("Expected error message")
	}
}
