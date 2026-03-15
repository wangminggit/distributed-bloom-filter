package grpc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
)

// TestAuthInterceptor_BoundaryTimestamp tests timestamp boundary conditions
func TestAuthInterceptor_BoundaryTimestamp(t *testing.T) {
	keyStore := NewMemoryAPIKeyStore()
	testAPIKey := "test-key-boundary"
	testSecret := "test-secret-boundary"
	keyStore.AddKey(testAPIKey, testSecret)

	apiKeys := map[string]string{testAPIKey: testSecret}
	config := &AuthConfig{EnableAPIKeyAuth: true, APIKeys: apiKeys}
	interceptor, err := NewAuthInterceptor(config)
	if err != nil { t.Fatalf("Failed: %v", err) }
	defer interceptor.Stop()
	interceptor.keyStore = keyStore

	t.Run("TimestampJustWithinLimit", func(t *testing.T) {
		// Create timestamp just within the limit (4 minutes 59 seconds ago)
		timestamp := time.Now().Add(-maxRequestAge + time.Second).Unix()
		method := "/dbf.DBFService/Add"
		
		message := fmt.Sprintf("%s%d%s", testAPIKey, timestamp, method)
		h := hmac.New(sha256.New, []byte(testSecret))
		h.Write([]byte(message))
		signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

		req := &proto.AddRequest{
			Auth: &proto.AuthMetadata{
				ApiKey:    testAPIKey,
				Timestamp: timestamp,
				Signature: signature,
			},
			Item: []byte("test-item"),
		}

		ctx := context.Background()
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return &proto.AddResponse{Success: true}, nil
		}

		info := &grpc.UnaryServerInfo{
			FullMethod: method,
		}

		// Should succeed (just within the boundary)
		_, err := interceptor.UnaryInterceptor()(ctx, req, info, handler)
		if err != nil {
			t.Errorf("Expected success within boundary, got: %v", err)
		}
	})

	t.Run("TimestampJustOverLimit", func(t *testing.T) {
		// Create timestamp just over the limit (5 minutes + 1 second ago)
		timestamp := time.Now().Add(-maxRequestAge - time.Second).Unix()
		
		req := &proto.AddRequest{
			Auth: &proto.AuthMetadata{
				ApiKey:    testAPIKey,
				Timestamp: timestamp,
				Signature: "any-signature",
			},
			Item: []byte("test-item"),
		}

		ctx := context.Background()
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return &proto.AddResponse{Success: true}, nil
		}

		info := &grpc.UnaryServerInfo{
			FullMethod: "/dbf.DBFService/Add",
		}

		// Should fail (just over the limit)
		_, err := interceptor.UnaryInterceptor()(ctx, req, info, handler)
		if err == nil {
			t.Error("Expected error for timestamp just over limit, got nil")
		}
	})

	t.Run("FutureTimestamp", func(t *testing.T) {
		// Create timestamp in the future
		timestamp := time.Now().Add(1 * time.Hour).Unix()
		
		message := fmt.Sprintf("%s%d%s", testAPIKey, timestamp, "/dbf.DBFService/Add")
		h := hmac.New(sha256.New, []byte(testSecret))
		h.Write([]byte(message))
		signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

		req := &proto.AddRequest{
			Auth: &proto.AuthMetadata{
				ApiKey:    testAPIKey,
				Timestamp: timestamp,
				Signature: signature,
			},
			Item: []byte("test-item"),
		}

		ctx := context.Background()
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return &proto.AddResponse{Success: true}, nil
		}

		info := &grpc.UnaryServerInfo{
			FullMethod: "/dbf.DBFService/Add",
		}

		// Future timestamps should succeed (they're within maxRequestAge)
		_, err := interceptor.UnaryInterceptor()(ctx, req, info, handler)
		if err != nil {
			t.Errorf("Expected success for future timestamp, got: %v", err)
		}
	})
}

// TestRateLimitInterceptor_TokenRecovery tests token bucket recovery
func TestRateLimitInterceptor_TokenRecovery(t *testing.T) {
	// Create limiter with small burst for testing
	limiter := NewRateLimitInterceptor(10, 3) // 10 req/s, burst of 3

	ctx := context.Background()
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return &proto.AddResponse{Success: true}, nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/dbf.DBFService/Add",
	}

	// Exhaust the burst (3 requests should succeed)
	for i := 0; i < 3; i++ {
		_, err := limiter.UnaryInterceptor()(ctx, &proto.AddRequest{}, info, handler)
		if err != nil {
			t.Fatalf("Request %d failed unexpectedly: %v", i, err)
		}
	}

	// Next request should fail (burst exhausted)
	_, err := limiter.UnaryInterceptor()(ctx, &proto.AddRequest{}, info, handler)
	if err == nil {
		t.Error("Expected rate limit error after exhausting burst, got nil")
	}

	// Wait for token recovery (100ms should give us 1 token at 10 req/s)
	time.Sleep(150 * time.Millisecond)

	// Now should succeed again
	_, err = limiter.UnaryInterceptor()(ctx, &proto.AddRequest{}, info, handler)
	if err != nil {
		t.Errorf("Expected success after token recovery, got: %v", err)
	}
}

// TestStreamInterceptor_Auth tests stream interceptor (placeholder for future auth)
func TestStreamInterceptor_Auth(t *testing.T) {
	// Note: Current implementation doesn't have stream auth interceptor
	// This test documents the expected behavior for future implementation
	
	keyStore := NewMemoryAPIKeyStore()
	testAPIKey := "test-stream-key"
	testSecret := "test-stream-secret"
	keyStore.AddKey(testAPIKey, testSecret)

	config := &AuthConfig{EnableAPIKeyAuth: true, APIKeys: make(map[string]string)}
	interceptor, err := NewAuthInterceptor(config)
	if err != nil { t.Fatalf("Failed: %v", err) }
	defer interceptor.Stop()

	// Verify interceptor exists
	if interceptor == nil {
		t.Error("AuthInterceptor should not be nil")
	}

	// Future: Test stream authentication
	// For now, just verify the interceptor can be created
	t.Log("Stream interceptor auth test - interceptor created successfully")
}

// TestRateLimitInterceptor_StreamInterceptor tests stream rate limiting
func TestRateLimitInterceptor_StreamInterceptor(t *testing.T) {
	limiter := NewRateLimitInterceptor(5, 5)

	ctx := context.Background()
	
	// Create a mock stream
	mockStream := &mockServerStream{ctx: ctx}

	info := &grpc.StreamServerInfo{
		FullMethod: "/dbf.DBFService/Stream",
	}

	handler := func(srv interface{}, ss grpc.ServerStream) error {
		return nil
	}

	// Make requests within limit
	for i := 0; i < 3; i++ {
		err := limiter.StreamInterceptor()(nil, mockStream, info, handler)
		if err != nil {
			t.Fatalf("Stream request %d failed unexpectedly: %v", i, err)
		}
	}

	// Exhaust the limit
	for i := 0; i < 5; i++ {
		err := limiter.StreamInterceptor()(nil, mockStream, info, handler)
		if i < 2 {
			// These should succeed (remaining burst)
			if err != nil {
				t.Fatalf("Stream request %d failed unexpectedly: %v", i+3, err)
			}
		} else {
			// These should fail
			if err == nil {
				t.Errorf("Stream request %d should have been rate limited", i+3)
			} else {
				st, ok := status.FromError(err)
				if !ok {
					t.Fatalf("Expected gRPC status error, got: %v", err)
				}
				if st.Code() != codes.ResourceExhausted {
					t.Errorf("Expected code ResourceExhausted, got: %v", st.Code())
				}
			}
		}
	}
}

// TestServer_ConcurrentRequests_HeavyLoad tests concurrent gRPC service requests with heavy load
// This is an extended version with more goroutines than the basic test
func TestServer_ConcurrentRequests_HeavyLoad(t *testing.T) {
	mockNode := newMockRaftNode()
	service := NewDBFService(mockNode)

	var wg sync.WaitGroup
	numGoroutines := 20
	requestsPerGoroutine := 50

	errors := make(chan error, numGoroutines*requestsPerGoroutine)

	// Start multiple goroutines making concurrent requests
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ctx := context.Background()
			
			for j := 0; j < requestsPerGoroutine; j++ {
				item := []byte(fmt.Sprintf("concurrent-item-%d-%d", id, j))
				
				// Mix of Add and Contains operations
				if j%2 == 0 {
					_, err := service.Add(ctx, &proto.AddRequest{
						Auth: &proto.AuthMetadata{},
						Item: item,
					})
					if err != nil {
						errors <- err
					}
				} else {
					_, err := service.Contains(ctx, &proto.ContainsRequest{
						Auth: &proto.AuthMetadata{},
						Item: item,
					})
					if err != nil {
						errors <- err
					}
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		errorCount++
		t.Errorf("Concurrent request error: %v", err)
	}

	if errorCount > 0 {
		t.Errorf("Total errors: %d", errorCount)
	}
}

// TestServer_RaftFailure tests service behavior when Raft operations fail
func TestServer_RaftFailure(t *testing.T) {
	// Create mock that simulates Raft failures
	mockNode := &mockRaftNodeWithFailures{
		failAdd:      true,
		failRemove:   true,
		failContains: false,
	}
	
	service := NewDBFService(mockNode)
	ctx := context.Background()

	t.Run("AddFailure", func(t *testing.T) {
		resp, err := service.Add(ctx, &proto.AddRequest{
			Auth: &proto.AuthMetadata{},
			Item: []byte("test-item"),
		})
		if err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		if resp.Success {
			t.Error("Expected Add to fail, got success=true")
		}
		if resp.Error == "" {
			t.Error("Expected error message, got empty")
		}
	})

	t.Run("RemoveFailure", func(t *testing.T) {
		resp, err := service.Remove(ctx, &proto.RemoveRequest{
			Auth: &proto.AuthMetadata{},
			Item: []byte("test-item"),
		})
		if err != nil {
			t.Fatalf("Remove() error = %v", err)
		}
		if resp.Success {
			t.Error("Expected Remove to fail, got success=true")
		}
	})

	t.Run("ContainsSuccess", func(t *testing.T) {
		resp, err := service.Contains(ctx, &proto.ContainsRequest{
			Auth: &proto.AuthMetadata{},
			Item: []byte("test-item"),
		})
		if err != nil {
			t.Fatalf("Contains() error = %v", err)
		}
		// Contains should work even if Add/Remove fail
		// Since mockRaftNodeWithFailures.failContains=false, Contains returns !false = true
		if !resp.Exists {
			t.Error("Expected Contains to return true")
		}
	})
}

// TestGetClientIP_WithMetadata tests GetClientIP with various metadata
// Note: These tests use a trusted proxy IP (localhost) to allow X-Forwarded-For headers
func TestGetClientIP_WithMetadata(t *testing.T) {
	// Create a peer from localhost (trusted by default)
	peerAddr := &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 54321,
	}

	t.Run("XForwardedFor", func(t *testing.T) {
		ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: peerAddr})
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			"x-forwarded-for", "192.168.1.100",
		))
		
		ip := GetClientIP(ctx)
		if ip != "192.168.1.100" {
			t.Errorf("Expected IP 192.168.1.100, got %q", ip)
		}
	})

	t.Run("XRealIP", func(t *testing.T) {
		ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: peerAddr})
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			"x-real-ip", "10.0.0.50",
		))
		
		ip := GetClientIP(ctx)
		if ip != "10.0.0.50" {
			t.Errorf("Expected IP 10.0.0.50, got %q", ip)
		}
	})

	t.Run("XForwardedForTakesPrecedence", func(t *testing.T) {
		ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: peerAddr})
		ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(
			"x-forwarded-for", "192.168.1.100",
			"x-real-ip", "10.0.0.50",
		))
		
		ip := GetClientIP(ctx)
		if ip != "192.168.1.100" {
			t.Errorf("Expected IP 192.168.1.100 (x-forwarded-for takes precedence), got %q", ip)
		}
	})
}

// TestAuthInterceptor_ReplayAttack tests replay attack detection
func TestAuthInterceptor_ReplayAttack(t *testing.T) {
	keyStore := NewMemoryAPIKeyStore()
	testAPIKey := "test-replay-key"
	testSecret := "test-replay-secret"
	keyStore.AddKey(testAPIKey, testSecret)

	apiKeys := map[string]string{testAPIKey: testSecret}
	config := &AuthConfig{EnableAPIKeyAuth: true, APIKeys: apiKeys}
	interceptor, err := NewAuthInterceptor(config)
	if err != nil { t.Fatalf("Failed: %v", err) }
	defer interceptor.Stop()
	interceptor.keyStore = keyStore

	timestamp := time.Now().Unix()
	method := "/dbf.DBFService/Add"
	
	message := fmt.Sprintf("%s%d%s", testAPIKey, timestamp, method)
	h := hmac.New(sha256.New, []byte(testSecret))
	h.Write([]byte(message))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	req := &proto.AddRequest{
		Auth: &proto.AuthMetadata{
			ApiKey:    testAPIKey,
			Timestamp: timestamp,
			Signature: signature,
		},
		Item: []byte("test-item"),
	}

	ctx := context.Background()
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return &proto.AddResponse{Success: true}, nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: method,
	}

	// First request should succeed
	_, err = interceptor.UnaryInterceptor()(ctx, req, info, handler)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}

	// Second request with same timestamp should fail (replay attack)
	_, err = interceptor.UnaryInterceptor()(ctx, req, info, handler)
	if err == nil {
		t.Error("Expected replay attack detection, got nil error")
	} else {
		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Expected gRPC status error, got: %v", err)
		}
		if st.Code() != codes.Unauthenticated {
			t.Errorf("Expected code Unauthenticated, got: %v", st.Code())
		}
		if st.Message() != "replay attack detected" {
			t.Errorf("Expected 'replay attack detected', got: %s", st.Message())
		}
	}
}

// TestRateLimitInterceptor_ZeroConfig tests rate limiter with zero configuration
func TestRateLimitInterceptor_ZeroConfig(t *testing.T) {
	// Create limiter with 0 rate (should allow nothing or use defaults)
	limiter := NewRateLimitInterceptor(0, 0)
	
	ctx := context.Background()
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return &proto.AddResponse{Success: true}, nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/dbf.DBFService/Add",
	}

	// With 0 rate, behavior depends on rate.Limiter implementation
	// Just verify it doesn't crash
	_, err := limiter.UnaryInterceptor()(ctx, &proto.AddRequest{}, info, handler)
	// We don't assert on success/failure as it's implementation-dependent
	t.Logf("Zero config rate limiter returned: %v", err)
}

// mockServerStream is a mock implementation of grpc.ServerStream for testing
type mockServerStream struct {
	ctx context.Context
}

func (m *mockServerStream) SetHeader(md metadata.MD) error { return nil }
func (m *mockServerStream) SendHeader(md metadata.MD) error { return nil }
func (m *mockServerStream) SetTrailer(md metadata.MD) {}
func (m *mockServerStream) Context() context.Context { return m.ctx }
func (m *mockServerStream) SendMsg(msg interface{}) error { return nil }
func (m *mockServerStream) RecvMsg(msg interface{}) error { return nil }

// mockRaftNodeWithFailures is a mock that can simulate failures
type mockRaftNodeWithFailures struct {
	failAdd      bool
	failRemove   bool
	failContains bool
}

func (m *mockRaftNodeWithFailures) Start() error { return nil }
func (m *mockRaftNodeWithFailures) Shutdown() error { return nil }
func (m *mockRaftNodeWithFailures) IsLeader() bool { return true }
func (m *mockRaftNodeWithFailures) Add(item []byte) error {
	if m.failAdd {
		return fmt.Errorf("simulated Raft add failure")
	}
	return nil
}
func (m *mockRaftNodeWithFailures) Remove(item []byte) error {
	if m.failRemove {
		return fmt.Errorf("simulated Raft remove failure")
	}
	return nil
}
func (m *mockRaftNodeWithFailures) Contains(item []byte) bool {
	return !m.failContains
}
func (m *mockRaftNodeWithFailures) BatchAdd(items [][]byte) (int, int, []string) {
	if m.failAdd {
		return 0, len(items), []string{"simulated Raft batch add failure"}
	}
	return len(items), 0, nil
}
func (m *mockRaftNodeWithFailures) BatchContains(items [][]byte) []bool {
	results := make([]bool, len(items))
	for i := range results {
		results[i] = !m.failContains
	}
	return results
}
func (m *mockRaftNodeWithFailures) GetState() map[string]interface{} {
	return map[string]interface{}{
		"node_id":        "test-node",
		"is_leader":      true,
		"raft_state":     "Leader",
		"leader":         "test-node",
		"leader_address": "127.0.0.1:7000",
		"bloom_size":     10000,
		"bloom_k":        7,
		"raft_port":      7000,
	}
}
func (m *mockRaftNodeWithFailures) GetConfig() map[string]interface{} { return nil }

// Ensure mockRaftNodeWithFailures implements raft.RaftNode
var _ interface{ IsLeader() bool } = (*mockRaftNodeWithFailures)(nil)
