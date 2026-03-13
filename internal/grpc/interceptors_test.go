package grpc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
)

// TestAuthInterceptor tests the authentication interceptor.
func TestAuthInterceptor(t *testing.T) {
	keyStore := NewMemoryAPIKeyStore()
	testAPIKey := "test-key-123"
	testSecret := "test-secret-456"
	keyStore.AddKey(testAPIKey, testSecret)

	interceptor := NewAuthInterceptor(keyStore)

	t.Run("ValidAuth", func(t *testing.T) {
		// Create a valid request with proper authentication
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

		// Create a mock context and handler
		ctx := context.Background()
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return &proto.AddResponse{Success: true}, nil
		}

		info := &grpc.UnaryServerInfo{
			FullMethod: method,
		}

		// Call the interceptor
		resp, err := interceptor.UnaryInterceptor()(ctx, req, info, handler)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}

		addResp, ok := resp.(*proto.AddResponse)
		if !ok {
			t.Fatalf("Expected AddResponse, got %T", resp)
		}
		if !addResp.Success {
			t.Error("Expected success=true")
		}
	})

	t.Run("MissingAuth", func(t *testing.T) {
		req := &proto.AddRequest{
			Auth: nil,
			Item: []byte("test-item"),
		}

		ctx := context.Background()
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return &proto.AddResponse{Success: true}, nil
		}

		info := &grpc.UnaryServerInfo{
			FullMethod: "/dbf.DBFService/Add",
		}

		_, err := interceptor.UnaryInterceptor()(ctx, req, info, handler)
		if err == nil {
			t.Fatal("Expected error for missing auth, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Expected gRPC status error, got: %v", err)
		}
		if st.Code() != codes.Unauthenticated {
			t.Errorf("Expected code Unauthenticated, got: %v", st.Code())
		}
	})

	t.Run("InvalidAPIKey", func(t *testing.T) {
		timestamp := time.Now().Unix()
		
		req := &proto.AddRequest{
			Auth: &proto.AuthMetadata{
				ApiKey:    "invalid-key",
				Timestamp: timestamp,
				Signature: "invalid-signature",
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

		_, err := interceptor.UnaryInterceptor()(ctx, req, info, handler)
		if err == nil {
			t.Fatal("Expected error for invalid API key, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Expected gRPC status error, got: %v", err)
		}
		if st.Code() != codes.Unauthenticated {
			t.Errorf("Expected code Unauthenticated, got: %v", st.Code())
		}
	})

	t.Run("ExpiredTimestamp", func(t *testing.T) {
		// Create a timestamp from 10 minutes ago (older than maxRequestAge)
		timestamp := time.Now().Add(-10 * time.Minute).Unix()
		
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

		_, err := interceptor.UnaryInterceptor()(ctx, req, info, handler)
		if err == nil {
			t.Fatal("Expected error for expired timestamp, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Expected gRPC status error, got: %v", err)
		}
		if st.Code() != codes.Unauthenticated {
			t.Errorf("Expected code Unauthenticated, got: %v", st.Code())
		}
	})

	t.Run("InvalidSignature", func(t *testing.T) {
		timestamp := time.Now().Unix()
		
		req := &proto.AddRequest{
			Auth: &proto.AuthMetadata{
				ApiKey:    testAPIKey,
				Timestamp: timestamp,
				Signature: "invalid-signature",
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

		_, err := interceptor.UnaryInterceptor()(ctx, req, info, handler)
		if err == nil {
			t.Fatal("Expected error for invalid signature, got nil")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatalf("Expected gRPC status error, got: %v", err)
		}
		if st.Code() != codes.Unauthenticated {
			t.Errorf("Expected code Unauthenticated, got: %v", st.Code())
		}
	})
}

// TestRateLimitInterceptor tests the rate limiting interceptor.
func TestRateLimitInterceptor(t *testing.T) {
	t.Run("WithinLimit", func(t *testing.T) {
		// Create a fresh limiter for this test
		limiter := NewRateLimitInterceptor(5, 5)
		
		ctx := context.Background()
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return &proto.AddResponse{Success: true}, nil
		}

		info := &grpc.UnaryServerInfo{
			FullMethod: "/dbf.DBFService/Add",
		}

		// Make a few requests within the burst limit
		for i := 0; i < 3; i++ {
			_, err := limiter.UnaryInterceptor()(ctx, &proto.AddRequest{}, info, handler)
			if err != nil {
				t.Fatalf("Request %d failed unexpectedly: %v", i, err)
			}
		}
	})

	t.Run("ExceedsLimit", func(t *testing.T) {
		// Create a fresh limiter for this test
		limiter := NewRateLimitInterceptor(5, 5)
		
		ctx := context.Background()
		handler := func(ctx context.Context, req interface{}) (interface{}, error) {
			return &proto.AddResponse{Success: true}, nil
		}

		info := &grpc.UnaryServerInfo{
			FullMethod: "/dbf.DBFService/Add",
		}

		// Exhaust the burst - with burst=5, first 5 should succeed
		for i := 0; i < 8; i++ {
			_, err := limiter.UnaryInterceptor()(ctx, &proto.AddRequest{}, info, handler)
			if i < 5 {
				// First 5 should succeed (burst size)
				if err != nil {
					t.Fatalf("Request %d failed unexpectedly: %v", i, err)
				}
			} else {
				// Remaining should fail
				if err == nil {
					t.Errorf("Request %d should have been rate limited", i)
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
	})
}

// TestMemoryAPIKeyStore tests the in-memory API key store.
func TestMemoryAPIKeyStore(t *testing.T) {
	store := NewMemoryAPIKeyStore()

	t.Run("AddAndGet", func(t *testing.T) {
		apiKey := "test-key"
		secret := "test-secret"

		store.AddKey(apiKey, secret)
		retrieved := store.GetSecret(apiKey)

		if retrieved != secret {
			t.Errorf("Expected secret %q, got %q", secret, retrieved)
		}
	})

	t.Run("InvalidKey", func(t *testing.T) {
		retrieved := store.GetSecret("non-existent-key")
		if retrieved != "" {
			t.Errorf("Expected empty string for non-existent key, got %q", retrieved)
		}
	})
}

// TestGetClientIP tests the client IP extraction.
func TestGetClientIP(t *testing.T) {
	t.Run("EmptyContext", func(t *testing.T) {
		ctx := context.Background()
		ip := GetClientIP(ctx)
		if ip != "" {
			t.Errorf("Expected empty IP for empty context, got %q", ip)
		}
	})
}
