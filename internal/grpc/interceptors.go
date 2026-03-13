package grpc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
)

const (
	// maxRequestAge is the maximum age of a request before it's considered a replay attack.
	maxRequestAge = 5 * time.Minute

	// defaultRateLimit is the default rate limit (requests per second).
	defaultRateLimit = 100

	// defaultBurstSize is the default burst size for rate limiting.
	defaultBurstSize = 200
)

// APIKeyStore defines the interface for API key validation.
// In production, this should be backed by a secure database or secret manager.
type APIKeyStore interface {
	// GetSecret returns the secret associated with an API key.
	// Returns empty string if the key is invalid.
	GetSecret(apiKey string) string
}

// MemoryAPIKeyStore is a simple in-memory API key store for testing.
// DO NOT use in production - secrets should be stored securely.
type MemoryAPIKeyStore struct {
	mu      sync.RWMutex
	secrets map[string]string
}

// NewMemoryAPIKeyStore creates a new in-memory API key store.
func NewMemoryAPIKeyStore() *MemoryAPIKeyStore {
	return &MemoryAPIKeyStore{
		secrets: make(map[string]string),
	}
}

// AddKey adds an API key and its associated secret.
func (s *MemoryAPIKeyStore) AddKey(apiKey, secret string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[apiKey] = secret
}

// GetSecret returns the secret for an API key.
func (s *MemoryAPIKeyStore) GetSecret(apiKey string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.secrets[apiKey]
}

// AuthInterceptor validates authentication metadata for incoming requests.
type AuthInterceptor struct {
	keyStore APIKeyStore
	// seenRequests tracks request timestamps to prevent replay attacks
	seenRequests sync.Map // map[string]bool where key is "apiKey:timestamp"
}

// NewAuthInterceptor creates a new authentication interceptor.
func NewAuthInterceptor(keyStore APIKeyStore) *AuthInterceptor {
	return &AuthInterceptor{
		keyStore: keyStore,
	}
}

// UnaryInterceptor returns a gRPC unary server interceptor that validates authentication.
func (a *AuthInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Extract auth metadata from the request
		authReq, ok := req.(interface{ GetAuth() *proto.AuthMetadata })
		if !ok {
			return nil, status.Error(codes.InvalidArgument, "request does not support authentication")
		}

		auth := authReq.GetAuth()
		if auth == nil {
			return nil, status.Error(codes.Unauthenticated, "missing authentication metadata")
		}

		// Validate the authentication
		if err := a.validateAuth(ctx, auth, info.FullMethod); err != nil {
			return nil, err
		}

		// Proceed with the handler
		return handler(ctx, req)
	}
}

// validateAuth validates the authentication metadata.
func (a *AuthInterceptor) validateAuth(ctx context.Context, auth *proto.AuthMetadata, method string) error {
	// Check if API key is provided
	if auth.ApiKey == "" {
		return status.Error(codes.Unauthenticated, "missing API key")
	}

	// Get the secret for this API key
	secret := a.keyStore.GetSecret(auth.ApiKey)
	if secret == "" {
		return status.Error(codes.Unauthenticated, "invalid API key")
	}

	// Check timestamp to prevent replay attacks
	requestTime := time.Unix(auth.Timestamp, 0)
	if time.Since(requestTime) > maxRequestAge {
		return status.Error(codes.Unauthenticated, "request timestamp too old")
	}

	// Check for replay attack using timestamp
	replayKey := fmt.Sprintf("%s:%d", auth.ApiKey, auth.Timestamp)
	if _, loaded := a.seenRequests.LoadOrStore(replayKey, true); loaded {
		return status.Error(codes.Unauthenticated, "replay attack detected")
	}

	// Clean up old timestamps periodically (simplified - in production use a more robust approach)
	go a.cleanupOldTimestamps()

	// Verify signature
	expectedSignature := a.computeSignature(auth.ApiKey, auth.Timestamp, method, secret)
	if !hmac.Equal([]byte(auth.Signature), []byte(expectedSignature)) {
		return status.Error(codes.Unauthenticated, "invalid signature")
	}

	return nil
}

// computeSignature computes the expected HMAC-SHA256 signature.
func (a *AuthInterceptor) computeSignature(apiKey string, timestamp int64, method, secret string) string {
	// Create the message to sign: apiKey + timestamp + method
	message := fmt.Sprintf("%s%d%s", apiKey, timestamp, method)

	// Compute HMAC-SHA256
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))

	// Return base64-encoded signature
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// cleanupOldTimestamps removes timestamps older than maxRequestAge.
func (a *AuthInterceptor) cleanupOldTimestamps() {
	// This is a simplified cleanup - in production, use a more efficient approach
	// like a bounded map with automatic expiration
	// For now, this is a no-op placeholder
	// In production, implement proper TTL-based cleanup using time.AfterFunc or similar
}

// RateLimitInterceptor implements rate limiting for gRPC requests.
type RateLimitInterceptor struct {
	limiter *rate.Limiter
}

// NewRateLimitInterceptor creates a new rate limiting interceptor.
func NewRateLimitInterceptor(requestsPerSecond int, burstSize int) *RateLimitInterceptor {
	// rate.Every expects a duration between requests
	// For requestsPerSecond=5, we want 1 request every 200ms (time.Second/5)
	limiter := rate.NewLimiter(rate.Limit(requestsPerSecond), burstSize)
	return &RateLimitInterceptor{
		limiter: limiter,
	}
}

// UnaryInterceptor returns a gRPC unary server interceptor that enforces rate limiting.
func (r *RateLimitInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Check if we're allowed to proceed
		if !r.limiter.Allow() {
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}

		// Proceed with the handler
		return handler(ctx, req)
	}
}

// StreamInterceptor returns a gRPC stream server interceptor that enforces rate limiting.
func (r *RateLimitInterceptor) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Check if we're allowed to proceed
		if !r.limiter.Allow() {
			return status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}

		// Proceed with the handler
		return handler(srv, ss)
	}
}

// GetClientIP extracts the client IP address from the context.
func GetClientIP(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	// Check for x-forwarded-for header (for proxied requests)
	if xff := md.Get("x-forwarded-for"); len(xff) > 0 {
		return xff[0]
	}

	// Check for x-real-ip header
	if xri := md.Get("x-real-ip"); len(xri) > 0 {
		return xri[0]
	}

	// Fall back to peer address
	peer, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	// Try to get the address from the peer
	if addr := peer.Get(":authority"); len(addr) > 0 {
		return addr[0]
	}

	return ""
}
