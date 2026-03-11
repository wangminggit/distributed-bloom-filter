package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

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
	flag.Parse()

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Initialize WAL encryptor
	walEncryptor := wal.NewEncryptor([]byte("32-byte-secret-key-for-wal-enc"))

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

	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	log.Printf("Server started on port %d (Raft: %d)", *port, *raftPort)
	log.Printf("Node '%s' initialized with Bloom filter (m=%d, k=%d)", *nodeID, *m, *k)

	<-stop
	log.Println("Shutting down...")

	// Cleanup
	raftNode.Shutdown()

	log.Println("Server stopped")
}
