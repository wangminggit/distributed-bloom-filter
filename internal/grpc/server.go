package grpc

import (
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
	"github.com/wangminggit/distributed-bloom-filter/internal/audit"
	"github.com/wangminggit/distributed-bloom-filter/internal/raft"
)

// ServerConfig holds configuration for the gRPC server.
type ServerConfig struct {
	// Port is the port to listen on.
	Port int

	// TLSCertFile is the path to the TLS certificate file.
	TLSCertFile string

	// TLSKeyFile is the path to the TLS private key file.
	TLSKeyFile string

	// EnableTLS enables TLS encryption. If false, the server will use insecure connection.
	EnableTLS bool

	// APIKeyStore is the store for API key validation.
	APIKeyStore APIKeyStore

	// RateLimitPerSecond is the maximum number of requests per second.
	RateLimitPerSecond int

	// RateLimitBurstSize is the maximum burst size for rate limiting.
	RateLimitBurstSize int

	// AuditLogDir is the directory for audit logs (empty disables audit logging).
	AuditLogDir string

	// AuditMaxFileSize is the maximum size of audit log files before rotation.
	AuditMaxFileSize int64

	// AuditMaxAge is the maximum age of audit log files before cleanup.
	AuditMaxAge int

	// readyCh is an optional channel that is closed when the server is ready to serve.
	// This is used for test synchronization to avoid race conditions between Start and Stop.
	// Internal use only - not part of the public API.
	readyCh chan struct{}
}

// GRPCServer wraps the gRPC server and service.
type GRPCServer struct {
	service *DBFService
	server  *grpc.Server
	config  ServerConfig
}

// NewGRPCServer creates a new gRPC server.
func NewGRPCServer(raftNode raft.RaftNode) *GRPCServer {
	return &GRPCServer{
		service: NewDBFService(raftNode),
	}
}

// Start starts the gRPC server with the given configuration.
func (s *GRPCServer) Start(config ServerConfig) error {
	s.config = config
	// Create gRPC server options
	var opts []grpc.ServerOption

	// Configure TLS if enabled
	if config.EnableTLS {
		if config.TLSCertFile == "" || config.TLSKeyFile == "" {
			return fmt.Errorf("TLS enabled but certificate or key file not specified")
		}

		creds, err := credentials.NewServerTLSFromFile(config.TLSCertFile, config.TLSKeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
		log.Printf("TLS encryption enabled with cert: %s, key: %s", config.TLSCertFile, config.TLSKeyFile)
	} else {
		log.Printf("WARNING: TLS is disabled - using insecure connection")
	}

	// Collect all unary interceptors for chaining
	var unaryInterceptors []grpc.UnaryServerInterceptor
	var streamInterceptors []grpc.StreamServerInterceptor

	// Initialize audit logging if configured
	var auditLogger *audit.Logger
	if config.AuditLogDir != "" {
		maxAge := audit.DefaultMaxAge
		if config.AuditMaxAge > 0 {
			maxAge = time.Duration(config.AuditMaxAge) * 24 * time.Hour
		}
		
		auditConfig := audit.LoggerConfig{
			LogDir:        config.AuditLogDir,
			MaxFileSize:   config.AuditMaxFileSize,
			MaxAge:        maxAge,
			BufferSize:    audit.DefaultBufferSize,
			FlushInterval: audit.DefaultFlushInterval,
			EnableConsole: false,
		}
		
		var err error
		auditLogger, err = audit.NewLogger(auditConfig)
		if err != nil {
			log.Printf("WARNING: failed to initialize audit logger: %v", err)
		} else {
			log.Printf("Audit logging enabled: dir=%s", config.AuditLogDir)
			
			// Add audit interceptor first (for request tracking)
			auditInterceptor := NewAuditInterceptor(auditLogger)
			unaryInterceptors = append(unaryInterceptors, auditInterceptor.UnaryInterceptor())
			streamInterceptors = append(streamInterceptors, auditInterceptor.StreamInterceptor())
		}
	}

	// Add authentication interceptor if API key store is provided
	if config.APIKeyStore != nil {
		// Convert APIKeyStore to map for AuthConfig
		apiKeys := make(map[string]string)
		// Note: This is a temporary workaround - the APIKeyStore interface should be used directly
		if memStore, ok := config.APIKeyStore.(*MemoryAPIKeyStore); ok {
			memStore.mu.RLock()
			for k, v := range memStore.secrets {
				apiKeys[k] = v
			}
			memStore.mu.RUnlock()
		}
		
		authConfig := &AuthConfig{
			EnableAPIKeyAuth: true,
			APIKeys:          apiKeys,
		}
		authInterceptor, err := NewAuthInterceptor(authConfig)
		if err != nil {
			return fmt.Errorf("failed to create auth interceptor: %w", err)
		}
		
		// Wrap with audit logging if available
		if auditLogger != nil {
			auditAuthInterceptor := NewAuditAuthInterceptor(authInterceptor, auditLogger)
			unaryInterceptors = append(unaryInterceptors, auditAuthInterceptor.UnaryInterceptor())
			streamInterceptors = append(streamInterceptors, authInterceptor.StreamInterceptor())
		} else {
			unaryInterceptors = append(unaryInterceptors, authInterceptor.UnaryInterceptor())
			streamInterceptors = append(streamInterceptors, authInterceptor.StreamInterceptor())
		}
		log.Printf("Authentication interceptor enabled")
	}

	// Add rate limiting interceptor if configured
	if config.RateLimitPerSecond > 0 {
		burstSize := config.RateLimitBurstSize
		if burstSize == 0 {
			burstSize = defaultBurstSize
		}
		rateLimiter := NewRateLimitInterceptor(config.RateLimitPerSecond, burstSize)
		
		// Wrap with audit logging if available
		if auditLogger != nil {
			auditRateInterceptor := NewAuditRateLimitInterceptor(rateLimiter, auditLogger)
			unaryInterceptors = append(unaryInterceptors, auditRateInterceptor.UnaryInterceptor())
			streamInterceptors = append(streamInterceptors, rateLimiter.StreamInterceptor())
		} else {
			unaryInterceptors = append(unaryInterceptors, rateLimiter.UnaryInterceptor())
			streamInterceptors = append(streamInterceptors, rateLimiter.StreamInterceptor())
		}
		log.Printf("Rate limiting enabled: %d requests/sec, burst: %d", config.RateLimitPerSecond, burstSize)
	}

	// Chain all unary interceptors together - order matters!
	// Order: Audit -> Auth -> RateLimit
	if len(unaryInterceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(unaryInterceptors...))
	}
	if len(streamInterceptors) > 0 {
		opts = append(opts, grpc.ChainStreamInterceptor(streamInterceptors...))
	}

	// Create the gRPC server
	grpcServer := grpc.NewServer(opts...)
	s.server = grpcServer

	// Register the service
	proto.RegisterDBFServiceServer(grpcServer, s.service)

	// Create listener
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	log.Printf("gRPC server starting on port %d (TLS: %v)", config.Port, config.EnableTLS)
	
	// Signal that server is ready (for test synchronization)
	// This is used by tests to avoid race conditions between Start and Stop
	if config.readyCh != nil {
		close(config.readyCh)
	}
	
	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

// Stop gracefully stops the gRPC server.
func (s *GRPCServer) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
	}
}

// StartInsecure starts the gRPC server without TLS (for development only).
// DEPRECATED: Use Start() with ServerConfig instead.
func (s *GRPCServer) StartInsecure(port int) error {
	config := ServerConfig{
		Port:      port,
		EnableTLS: false,
	}
	return s.Start(config)
}

// GenerateSelfSignedCert generates a self-signed certificate for development.
// In production, use certificates from a trusted CA.
func GenerateSelfSignedCert(certFile, keyFile string) error {
	// For production, generate certificates using:
	// openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes
	//
	// This is a placeholder - actual implementation would use crypto/x509
	// For now, we'll log instructions
	log.Printf("To generate self-signed certificates for development:")
	log.Printf("  openssl req -x509 -newkey rsa:4096 -keyout %s -out %s -days 365 -nodes", keyFile, certFile)
	return nil
}
