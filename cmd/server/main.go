package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/wangminggit/distributed-bloom-filter/internal/grpc"
	"github.com/wangminggit/distributed-bloom-filter/internal/metadata"
	"github.com/wangminggit/distributed-bloom-filter/internal/raft"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

func main() {
	// Parse command-line flags
	port := flag.Int("port", 8080, "gRPC server port")
	raftPort := flag.Int("raft-port", 8081, "Raft consensus port")
	dataDir := flag.String("data-dir", "./data", "Directory for storing data")
	nodeID := flag.String("node-id", "node1", "Unique node identifier")
	k := flag.Int("k", 3, "Number of hash functions for Bloom filter")
	m := flag.Int("m", 10000, "Size of Bloom filter in bits")
	bootstrap := flag.Bool("bootstrap", false, "Bootstrap as first node in cluster")
	_ = bootstrap // Reserved for future multi-node cluster support
	
	// Security flags
	enableTLS := flag.Bool("enable-tls", false, "Enable TLS encryption")
	tlsCertFile := flag.String("tls-cert", "", "Path to TLS certificate file")
	tlsKeyFile := flag.String("tls-key", "", "Path to TLS private key file")
	apiKey := flag.String("api-key", "", "API key for authentication (if provided, enables auth)")
	rateLimit := flag.Int("rate-limit", 100, "Rate limit (requests per second)")
	flag.Parse()

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Initialize WAL encryptor
	walEncryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		log.Fatalf("Failed to initialize WAL encryptor: %v", err)
	}

	// Initialize metadata service
	metadataService := metadata.NewService(*dataDir)

	// Initialize Bloom filter
	bloomFilter := bloom.NewCountingBloomFilter(*m, *k)

	// Initialize Raft node
	raftNode, err := raft.NewNodeWithDefaults(*nodeID, *raftPort, *dataDir, bloomFilter, walEncryptor, metadataService)
	if err != nil {
		log.Fatalf("Failed to create Raft node: %v", err)
	}

	// Start Raft node
	if err := raftNode.Start(); err != nil {
		log.Fatalf("Failed to start Raft node: %v", err)
	}

	// Create and start gRPC server
	grpcServer := grpc.NewGRPCServer(raftNode)
	
	// Setup server configuration with security features
	config := grpc.ServerConfig{
		Port:               *port,
		EnableTLS:          *enableTLS,
		TLSCertFile:        *tlsCertFile,
		TLSKeyFile:         *tlsKeyFile,
		RateLimitPerSecond: *rateLimit,
		RateLimitBurstSize: *rateLimit * 2,
	}
	
	// Setup API key authentication if provided
	if *apiKey != "" {
		keyStore := grpc.NewMemoryAPIKeyStore()
		// In production, use a secure secret, not a hardcoded one
		keyStore.AddKey(*apiKey, "secure-secret-key-change-in-production")
		config.APIKeyStore = keyStore
		log.Printf("Authentication enabled with API key")
	}
	
	if *enableTLS {
		if *tlsCertFile == "" || *tlsKeyFile == "" {
			log.Fatalf("TLS enabled but certificate or key file not specified")
		}
		log.Printf("TLS encryption enabled with cert: %s, key: %s", *tlsCertFile, *tlsKeyFile)
	}
	
	go func() {
		if err := grpcServer.Start(config); err != nil {
			log.Fatalf("Failed to start gRPC server: %v", err)
		}
	}()

	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	log.Printf("gRPC server started on port %d (Raft: %d)", *port, *raftPort)
	log.Printf("Node '%s' initialized with Bloom filter (m=%d, k=%d)", *nodeID, *m, *k)

	<-stop
	log.Println("Shutting down...")

	// Cleanup
	grpcServer.Stop()
	raftNode.Shutdown()

	log.Println("Server stopped")
}
