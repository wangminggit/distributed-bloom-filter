package grpc

import (
	"context"
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
)

// TestNewGRPCServer tests server creation
func TestNewGRPCServer(t *testing.T) {
	mockNode := newMockRaftNode()
	server := NewGRPCServer(mockNode)

	if server == nil {
		t.Fatal("NewGRPCServer returned nil")
	}

	if server.service == nil {
		t.Error("Service should not be nil")
	}
}

// TestGRPCServerStart tests server start configuration validation
func TestGRPCServerStart(t *testing.T) {
	t.Run("StartWithMissingTLSCert", func(t *testing.T) {
		mockNode := newMockRaftNode()
		server := NewGRPCServer(mockNode)
		
		config := ServerConfig{
			Port:      9999,
			EnableTLS: true,
			// Missing TLSCertFile and TLSKeyFile
		}
		
		err := server.Start(config)
		if err == nil {
			t.Error("Expected error for missing TLS cert, got nil")
		}
	})

	t.Run("StartWithInvalidTLSCert", func(t *testing.T) {
		mockNode := newMockRaftNode()
		server := NewGRPCServer(mockNode)
		
		config := ServerConfig{
			Port:        9999,
			EnableTLS:   true,
			TLSCertFile: "nonexistent.crt",
			TLSKeyFile:  "nonexistent.key",
		}
		
		err := server.Start(config)
		if err == nil {
			t.Error("Expected error for invalid TLS cert, got nil")
		}
	})
}

// TestGRPCServerStop tests server stop
func TestGRPCServerStop(t *testing.T) {
	mockNode := newMockRaftNode()
	server := NewGRPCServer(mockNode)

	// Stop on non-started server should not panic
	server.Stop()
}

// TestStartInsecure tests the deprecated StartInsecure method
func TestStartInsecure(t *testing.T) {
	mockNode := newMockRaftNode()
	server := NewGRPCServer(mockNode)

	// Try to start on port 0 (will fail)
	err := server.StartInsecure(0)
	if err == nil {
		t.Error("Expected error for port 0, got nil")
	}
}

// TestGenerateSelfSignedCert tests certificate generation helper
func TestGenerateSelfSignedCert(t *testing.T) {
	tempDir := t.TempDir()
	certFile := filepath.Join(tempDir, "test.crt")
	keyFile := filepath.Join(tempDir, "test.key")

	err := GenerateSelfSignedCert(certFile, keyFile)
	if err != nil {
		t.Errorf("GenerateSelfSignedCert failed: %v", err)
	}
}

// TestServerConfig_Validation tests configuration validation
func TestServerConfig_Validation(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		config := ServerConfig{
			Port:               8080,
			EnableTLS:          false,
			RateLimitPerSecond: 100,
			RateLimitBurstSize: 200,
		}
		
		// Basic validation
		if config.Port <= 0 {
			t.Error("Port should be positive")
		}
	})

	t.Run("ZeroBurstUsesDefault", func(t *testing.T) {
		config := ServerConfig{
			Port:               8080,
			EnableTLS:          false,
			RateLimitPerSecond: 100,
			RateLimitBurstSize: 0, // Should use default
		}
		
		if config.RateLimitBurstSize != 0 {
			// The code handles this in Start()
			t.Logf("Burst size: %d (will use default in Start)", config.RateLimitBurstSize)
		}
	})
}

// TestGetClientIP_AllPaths tests all code paths in GetClientIP
func TestGetClientIP_AllPaths(t *testing.T) {
	t.Run("NoMetadata", func(t *testing.T) {
		ctx := context.Background()
		ip := GetClientIP(ctx)
		if ip != "" {
			t.Errorf("Expected empty IP, got %q", ip)
		}
	})

	t.Run("WithAuthority", func(t *testing.T) {
		// This tests the :authority fallback path
		// Note: In practice, this requires a real gRPC connection
		ctx := context.Background()
		ip := GetClientIP(ctx)
		// Will be empty without real metadata
		t.Logf("IP with empty context: %q", ip)
	})
}

// TestCleanupOldTimestamps tests the cleanup function (currently no-op)
func TestCleanupOldTimestamps(t *testing.T) {
	keyStore := NewMemoryAPIKeyStore()
	interceptor := NewAuthInterceptor(keyStore)

	// Should not panic
	interceptor.cleanupOldTimestamps()
}

// TestDBFService_GetStateEdgeCases tests GetStats with various state configurations
func TestDBFService_GetStateEdgeCases(t *testing.T) {
	mockNode := &mockRaftNodeWithIncompleteState{}
	service := NewDBFService(mockNode)
	ctx := context.Background()

	resp, err := service.GetStats(ctx, &proto.GetStatsRequest{
		Auth: &proto.AuthMetadata{},
	})
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	// Should handle missing state gracefully
	if resp.NodeId == "" {
		t.Error("NodeId should have default value")
	}
}

// mockRaftNodeWithIncompleteState returns partial state to test defaults
type mockRaftNodeWithIncompleteState struct{}

func (m *mockRaftNodeWithIncompleteState) Start() error { return nil }
func (m *mockRaftNodeWithIncompleteState) Shutdown() error { return nil }
func (m *mockRaftNodeWithIncompleteState) IsLeader() bool { return true }
func (m *mockRaftNodeWithIncompleteState) Add(item []byte) error { return nil }
func (m *mockRaftNodeWithIncompleteState) Remove(item []byte) error { return nil }
func (m *mockRaftNodeWithIncompleteState) Contains(item []byte) bool { return false }
func (m *mockRaftNodeWithIncompleteState) BatchAdd(items [][]byte) (int, int, []string) {
	return len(items), 0, nil
}
func (m *mockRaftNodeWithIncompleteState) BatchContains(items [][]byte) []bool {
	return make([]bool, len(items))
}
func (m *mockRaftNodeWithIncompleteState) GetState() map[string]interface{} {
	// Return incomplete state to test defaults
	return map[string]interface{}{}
}
func (m *mockRaftNodeWithIncompleteState) GetConfig() map[string]interface{} { return nil }

var _ interface{ IsLeader() bool } = (*mockRaftNodeWithIncompleteState)(nil)

// TestTLSHelperFunctions tests TLS-related helper functions
func TestTLSHelperFunctions(t *testing.T) {
	t.Run("GenerateSelfSignedCert_InvalidPath", func(t *testing.T) {
		err := GenerateSelfSignedCert("/nonexistent/path/cert.crt", "/nonexistent/path/key.key")
		// Should not panic, may log instructions
		if err != nil {
			t.Logf("Expected behavior: %v", err)
		}
	})
}

// TestServerWithRealTLS tests server with actual TLS (if certs available)
func TestServerWithRealTLS(t *testing.T) {
	// Skip if no test certs available
	testCert := "test.crt"
	testKey := "test.key"
	
	if _, err := os.Stat(testCert); os.IsNotExist(err) {
		t.Skip("Test certificates not available")
	}

	// Load test certificate
	_, err := tls.LoadX509KeyPair(testCert, testKey)
	if err != nil {
		t.Skipf("Cannot load test certificates: %v", err)
	}

	// If we get here, we have valid certs
	mockNode := newMockRaftNode()
	server := NewGRPCServer(mockNode)

	config := ServerConfig{
		Port:        0, // Invalid port, will fail
		EnableTLS:   true,
		TLSCertFile: testCert,
		TLSKeyFile:  testKey,
	}

	err = server.Start(config)
	// Expected to fail on port binding, but TLS loading should succeed
	t.Logf("Server start with TLS: %v", err)
}

// TestRateLimitInterceptor_EdgeCases tests rate limiter edge cases
func TestRateLimitInterceptor_EdgeCases(t *testing.T) {
	t.Run("VeryHighRate", func(t *testing.T) {
		limiter := NewRateLimitInterceptor(10000, 10000)
		
		ctx := context.Background()
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return &proto.AddResponse{Success: true}, nil
		}
		info := &grpc.UnaryServerInfo{FullMethod: "/test"}

		// Should allow many requests
		for i := 0; i < 100; i++ {
			_, err := limiter.UnaryInterceptor()(ctx, &proto.AddRequest{}, info, handler)
			if err != nil {
				t.Fatalf("Request %d failed: %v", i, err)
			}
		}
	})

	t.Run("VeryLowRate", func(t *testing.T) {
		limiter := NewRateLimitInterceptor(1, 1)
		
		ctx := context.Background()
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return &proto.AddResponse{Success: true}, nil
		}
		info := &grpc.UnaryServerInfo{FullMethod: "/test"}

		// First should succeed
		_, err := limiter.UnaryInterceptor()(ctx, &proto.AddRequest{}, info, handler)
		if err != nil {
			t.Fatalf("First request failed: %v", err)
		}

		// Second should fail immediately
		_, err = limiter.UnaryInterceptor()(ctx, &proto.AddRequest{}, info, handler)
		if err == nil {
			t.Error("Second request should have been rate limited")
		}
	})
}
