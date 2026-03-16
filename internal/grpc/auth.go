package grpc

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/golang-jwt/jwt/v5"
)

// APIMetadata contains authentication information for API key-based auth.
type APIMetadata struct {
	ApiKey    string
	Timestamp int64
	Signature string
}

const (
	// maxRequestAge is the maximum age of a request before it's considered a replay attack.
	maxRequestAge = 5 * time.Minute

	// cleanupInterval is how often to clean up old timestamps.
	cleanupInterval = 10 * time.Minute

	// maxTimestampMapSize is the maximum number of entries in the seenRequests map.
	// When exceeded, the oldest 50% of entries are removed.
	maxTimestampMapSize = 100000

	// cleanupPercentage is the percentage of entries to remove when map exceeds max size.
	cleanupPercentage = 0.5
)

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	// mTLS Configuration
	EnableMTLS     bool
	CACertPath     string
	ServerCertPath string
	ServerKeyPath  string

	// Token Configuration
	EnableTokenAuth bool
	JWTSecretKey    string
	TokenExpiry     time.Duration

	// API Key Configuration
	EnableAPIKeyAuth bool
	APIKeys          map[string]string // map[apiKey]secret
}

// TokenClaims represents JWT token claims.
type TokenClaims struct {
	NodeID      string   `json:"node_id"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

// APIKeyStore defines the interface for API key validation.
type APIKeyStore interface {
	// GetSecret returns the secret associated with an API key.
	// Returns empty string if the key is invalid.
	GetSecret(apiKey string) string
}

// MemoryAPIKeyStore is a simple in-memory API key store for testing.
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

// AuthInterceptor handles authentication for gRPC requests.
type AuthInterceptor struct {
	config         *AuthConfig
	caCert         *x509.Certificate
	keyStore       APIKeyStore
	seenRequests   sync.Map // map[string]bool where key is "apiKey:timestamp"
	stopChan       chan struct{}
	stopOnce       sync.Once      // ensures Stop is only called once
	timestampCount int64          // atomic counter for tracking map size (metrics)
	apiKeyEnabled  bool           // cache for whether API key auth is enabled
}

// NewAuthInterceptor creates a new authentication interceptor.
func NewAuthInterceptor(config *AuthConfig) (*AuthInterceptor, error) {
	interceptor := &AuthInterceptor{
		config:        config,
		stopChan:      make(chan struct{}),
		apiKeyEnabled: config.EnableAPIKeyAuth && len(config.APIKeys) > 0,
	}

	// Load CA certificate if mTLS is enabled
	if config.EnableMTLS && config.CACertPath != "" {
		caCertPEM, err := os.ReadFile(config.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert: %w", err)
		}

		// Decode PEM block
		block, _ := pem.Decode(caCertPEM)
		if block == nil || block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("failed to decode PEM block containing certificate")
		}

		// Parse the certificate
		caCert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CA certificate: %w", err)
		}
		interceptor.caCert = caCert
	}

	// Setup API key store if enabled
	if interceptor.apiKeyEnabled {
		keyStore := NewMemoryAPIKeyStore()
		for apiKey, secret := range config.APIKeys {
			keyStore.AddKey(apiKey, secret)
		}
		interceptor.keyStore = keyStore

		// Start periodic cleanup of old timestamps
		go interceptor.periodicCleanup()
	}

	return interceptor, nil
}

// Stop stops the background cleanup goroutine.
// Safe to call multiple times.
func (a *AuthInterceptor) Stop() {
	a.stopOnce.Do(func() {
		close(a.stopChan)
	})
}

// GetTimestampCount returns the current number of tracked timestamps.
// This can be used for monitoring and metrics.
func (a *AuthInterceptor) GetTimestampCount() int64 {
	return atomic.LoadInt64(&a.timestampCount)
}

// periodicCleanup periodically removes old timestamps to prevent memory leaks.
func (a *AuthInterceptor) periodicCleanup() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.cleanupOldTimestamps()
		case <-a.stopChan:
			return
		}
	}
}

// UnaryInterceptor returns a gRPC unary server interceptor for authentication.
func (a *AuthInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Try API key authentication first if enabled
		if a.apiKeyEnabled {
			if authReq, ok := req.(interface{ GetAuth() *APIMetadata }); ok {
				auth := authReq.GetAuth()
				if auth != nil && auth.ApiKey != "" {
					if err := a.validateAPIKeyAuth(ctx, auth, info.FullMethod); err != nil {
						return nil, err
					}
					return handler(ctx, req)
				}
			}
		}

		// Fall back to other authentication methods
		if err := a.authenticate(ctx); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamInterceptor returns a gRPC stream server interceptor for authentication.
func (a *AuthInterceptor) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// For streams, validate via context (token or mTLS)
		if err := a.authenticate(ss.Context()); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

// authenticate validates the request credentials.
func (a *AuthInterceptor) authenticate(ctx context.Context) error {
	// Skip authentication if all methods are disabled
	if !a.config.EnableMTLS && !a.config.EnableTokenAuth && !a.apiKeyEnabled {
		return nil
	}

	// Check if TLS is present (mTLS)
	if a.config.EnableMTLS {
		if err := a.validateMTLS(ctx); err == nil {
			return nil // mTLS validation passed
		}
	}

	// Check Token authentication
	if a.config.EnableTokenAuth {
		if err := a.validateToken(ctx); err != nil {
			return err
		}
		return nil
	}

	// If mTLS is enabled but validation failed
	if a.config.EnableMTLS {
		return status.Error(codes.Unauthenticated, "mTLS authentication required")
	}

	return nil
}

// validateAPIKeyAuth validates API key + timestamp + signature authentication.
func (a *AuthInterceptor) validateAPIKeyAuth(ctx context.Context, auth *APIMetadata, method string) error {
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

	// Increment timestamp counter for metrics
	atomic.AddInt64(&a.timestampCount, 1)

	// Verify signature using constant-time comparison
	expectedSignature := a.computeSignature(auth.ApiKey, auth.Timestamp, method, secret)
	if subtle.ConstantTimeCompare([]byte(auth.Signature), []byte(expectedSignature)) != 1 {
		return status.Error(codes.Unauthenticated, "invalid signature")
	}

	return nil
}

// validateMTLS validates mTLS credentials from the context.
func (a *AuthInterceptor) validateMTLS(ctx context.Context) error {
	// Extract peer info from context
	p, ok := peer.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "TLS connection required")
	}

	// Check if TLS is being used
	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return status.Error(codes.Unauthenticated, "TLS connection required")
	}

	// Verify that client provided a certificate
	if len(tlsInfo.State.PeerCertificates) == 0 {
		return status.Error(codes.Unauthenticated, "client certificate required")
	}

	// Validate client certificate against CA
	clientCert := tlsInfo.State.PeerCertificates[0]
	if a.caCert != nil {
		// Create a cert pool with our CA
		caPool := x509.NewCertPool()
		caPool.AddCert(a.caCert)

		// Verify the client certificate against our CA
		_, err := clientCert.Verify(x509.VerifyOptions{
			Roots: caPool,
		})
		if err != nil {
			return status.Errorf(codes.Unauthenticated, "invalid client certificate: %v", err)
		}
	}

	return nil
}

// validateToken validates JWT token from metadata.
func (a *AuthInterceptor) validateToken(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	// Extract authorization header
	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return status.Error(codes.Unauthenticated, "missing authorization header")
	}

	authHeader := authHeaders[0]
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return status.Error(codes.Unauthenticated, "invalid authorization format")
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")

	// Parse and validate JWT token
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.config.JWTSecretKey), nil
	})

	if err != nil {
		return status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}

	if !token.Valid {
		return status.Error(codes.Unauthenticated, "invalid token")
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok {
		return status.Error(codes.Unauthenticated, "invalid token claims")
	}

	// Validate token expiry
	if claims.ExpiresAt != nil && time.Now().After(claims.ExpiresAt.Time) {
		return status.Error(codes.Unauthenticated, "token expired")
	}

	// Validate node_id is present
	if claims.NodeID == "" {
		return status.Error(codes.Unauthenticated, "missing node_id in token")
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

// cleanupOldTimestamps removes timestamps older than maxRequestAge and enforces size limits.
// This prevents memory leaks from the growing seenRequests map.
func (a *AuthInterceptor) cleanupOldTimestamps() {
	now := time.Now()
	cutoff := now.Add(-maxRequestAge).Unix()

	// Track entries for potential removal
	type timestampEntry struct {
		key       string
		timestamp int64
	}

	var oldEntries []timestampEntry
	var totalCount int64

	// First pass: collect all entries and identify old ones
	a.seenRequests.Range(func(key, value interface{}) bool {
		keyStr := key.(string)
		totalCount++

		// Extract timestamp from key (format: "apiKey:timestamp")
		// Find the last colon to handle API keys that might contain colons
		lastColon := strings.LastIndex(keyStr, ":")
		if lastColon == -1 {
			return true
		}

		var timestamp int64
		if _, err := fmt.Sscanf(keyStr[lastColon+1:], "%d", &timestamp); err != nil {
			// If parsing fails, skip this entry
			return true
		}

		if timestamp < cutoff {
			oldEntries = append(oldEntries, timestampEntry{key: keyStr, timestamp: timestamp})
		}
		return true
	})

	// Update metrics counter
	atomic.StoreInt64(&a.timestampCount, totalCount)

	// Remove old entries (older than maxRequestAge)
	for _, entry := range oldEntries {
		a.seenRequests.Delete(entry.key)
	}

	// Check if we need to enforce size limit
	if totalCount > maxTimestampMapSize {
		// Collect all entries with their timestamps for sorting
		var allEntries []timestampEntry
		a.seenRequests.Range(func(key, value interface{}) bool {
			keyStr := key.(string)
			lastColon := strings.LastIndex(keyStr, ":")
			if lastColon == -1 {
				return true
			}

			var timestamp int64
			if _, err := fmt.Sscanf(keyStr[lastColon+1:], "%d", &timestamp); err == nil {
				allEntries = append(allEntries, timestampEntry{key: keyStr, timestamp: timestamp})
			}
			return true
		})

		// Sort by timestamp (oldest first)
		sort.Slice(allEntries, func(i, j int) bool {
			return allEntries[i].timestamp < allEntries[j].timestamp
		})

		// Remove the oldest entries (cleanupPercentage of total)
		removeCount := int(float64(len(allEntries)) * cleanupPercentage)
		if removeCount > 0 {
			for i := 0; i < removeCount && i < len(allEntries); i++ {
				a.seenRequests.Delete(allEntries[i].key)
			}

			// Update metrics for monitoring
			remainingCount := totalCount - int64(removeCount)
			atomic.StoreInt64(&a.timestampCount, remainingCount)

			// In production, you would emit metrics here:
			// metrics.Gauge("grpc_auth_timestamp_map_size", remainingCount)
			// metrics.Gauge("grpc_auth_timestamp_map_cleaned", int64(removeCount))
		}
	}
}

// LoadTLSCredentials creates TLS credentials for gRPC server with mTLS.
func LoadTLSCredentials(config *AuthConfig) (credentials.TransportCredentials, error) {
	// Load server certificate
	cert, err := tls.LoadX509KeyPair(config.ServerCertPath, config.ServerKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Load CA certificate for client verification
	caCertPEM, err := os.ReadFile(config.CACertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCertPEM) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	// Create TLS config with client authentication
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(tlsConfig), nil
}

// GenerateToken creates a JWT token for a node.
func GenerateToken(config *AuthConfig, nodeID string, permissions []string) (string, error) {
	claims := TokenClaims{
		NodeID:      nodeID,
		Permissions: permissions,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(config.TokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "dbf-auth",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.JWTSecretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// GenerateAPIKeyToken generates a signed authentication token for API key auth.
func GenerateAPIKeyToken(apiKey string, secret string, method string) *APIMetadata {
	timestamp := time.Now().Unix()

	// Create message to sign
	message := fmt.Sprintf("%s%d%s", apiKey, timestamp, method)

	// Compute HMAC-SHA256
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return &APIMetadata{
		ApiKey:    apiKey,
		Timestamp: timestamp,
		Signature: signature,
	}
}
