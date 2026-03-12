package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/golang-jwt/jwt/v5"
)

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	// mTLS Configuration
	EnableMTLS bool
	CACertPath string
	ServerCertPath string
	ServerKeyPath  string
	
	// Token Configuration
	EnableTokenAuth bool
	JWTSecretKey   string
	TokenExpiry    time.Duration
}

// TokenClaims represents JWT token claims.
type TokenClaims struct {
	NodeID     string   `json:"node_id"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

// AuthInterceptor handles authentication for gRPC requests.
type AuthInterceptor struct {
	config *AuthConfig
	caCert *x509.Certificate
}

// NewAuthInterceptor creates a new authentication interceptor.
func NewAuthInterceptor(config *AuthConfig) (*AuthInterceptor, error) {
	interceptor := &AuthInterceptor{
		config: config,
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

	return interceptor, nil
}

// UnaryInterceptor returns a gRPC unary server interceptor for authentication.
func (a *AuthInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if err := a.authenticate(ctx); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamInterceptor returns a gRPC stream server interceptor for authentication.
func (a *AuthInterceptor) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if err := a.authenticate(ss.Context()); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}

// authenticate validates the request credentials.
func (a *AuthInterceptor) authenticate(ctx context.Context) error {
	// Skip authentication if both methods are disabled
	if !a.config.EnableMTLS && !a.config.EnableTokenAuth {
		return nil
	}

	// Check if TLS is present (mTLS)
	if a.config.EnableMTLS {
		if err := a.validateMTLS(ctx); err == nil {
			return nil // mTLS validation passed
		}
		// If mTLS validation fails but is required, return error
		// Otherwise, continue to token validation
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
