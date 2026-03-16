package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	
	// Authentication flags
	enableMTLS := flag.Bool("enable-mtls", false, "Enable mTLS authentication")
	caCertPath := flag.String("ca-cert", "", "Path to CA certificate")
	serverCertPath := flag.String("server-cert", "", "Path to server certificate")
	serverKeyPath := flag.String("server-key", "", "Path to server private key")
	enableTokenAuth := flag.Bool("enable-token-auth", false, "Enable JWT token authentication")
	jwtSecretKey := flag.String("jwt-secret", "", "JWT secret key for token signing")
	tokenExpiry := flag.Duration("token-expiry", 24*time.Hour, "JWT token expiry duration")
	
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
	raftNode := raft.NewNode(*nodeID, *raftPort, *dataDir, bloomFilter, walEncryptor, metadataService)

	// Start Raft node
	if err := raftNode.Start(*bootstrap); err != nil {
		log.Fatalf("Failed to start Raft node: %v", err)
	}

	// Create gRPC server with authentication
	var dbfServer *grpc.DBFServer
	serverConfig := &grpc.ServerConfig{
		Port:            *port,
		EnableMTLS:      *enableMTLS,
		CACertPath:      *caCertPath,
		ServerCertPath:  *serverCertPath,
		ServerKeyPath:   *serverKeyPath,
		EnableTokenAuth: *enableTokenAuth,
		JWTSecretKey:    *jwtSecretKey,
		TokenExpiry:     *tokenExpiry,
	}

	// Validate authentication configuration
	if *enableMTLS {
		if *caCertPath == "" || *serverCertPath == "" || *serverKeyPath == "" {
			log.Fatal("mTLS enabled but certificate paths not provided. Use --ca-cert, --server-cert, --server-key")
		}
		dbfServer, err = grpc.NewDBFServerWithAuth(raftNode, serverConfig)
		if err != nil {
			log.Fatalf("Failed to create gRPC server with auth: %v", err)
		}
	} else if *enableTokenAuth {
		if *jwtSecretKey == "" {
			log.Fatal("Token auth enabled but JWT secret not provided. Use --jwt-secret")
		}
		dbfServer, err = grpc.NewDBFServerWithAuth(raftNode, serverConfig)
		if err != nil {
			log.Fatalf("Failed to create gRPC server with auth: %v", err)
		}
	} else {
		dbfServer = grpc.NewDBFServer(raftNode)
	}

	// Start gRPC server
	go func() {
		if err := dbfServer.StartWithConfig(serverConfig); err != nil {
			log.Fatalf("Failed to start gRPC server: %v", err)
		}
	}()

	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	authInfo := "authentication disabled"
	if *enableMTLS {
		authInfo = "mTLS enabled"
	} else if *enableTokenAuth {
		authInfo = fmt.Sprintf("token auth enabled (expiry: %v)", *tokenExpiry)
	}
	
	log.Printf("gRPC server started on port %d (Raft: %d, %s)", *port, *raftPort, authInfo)
	log.Printf("Node '%s' initialized with Bloom filter (m=%d, k=%d)", *nodeID, *m, *k)

	<-stop
	log.Println("Shutting down...")

	// Cleanup
	raftNode.Shutdown()

	log.Println("Server stopped")
}
