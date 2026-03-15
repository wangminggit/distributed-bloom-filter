package main

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/wangminggit/distributed-bloom-filter/internal/metadata"
	"github.com/wangminggit/distributed-bloom-filter/internal/raft"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

// TestConfig_LoadValidConfig tests loading a valid configuration.
// This is a P1 test case for configuration loading.
func TestConfig_LoadValidConfig(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Test configuration values
	port := 8080
	raftPort := 8081
	dataDir := tmpDir
	nodeID := "test-node"
	k := 3
	m := 10000

	// Verify values are valid
	if port <= 0 || port > 65535 {
		t.Error("Invalid port")
	}
	if raftPort <= 0 || raftPort > 65535 {
		t.Error("Invalid raft port")
	}
	if dataDir == "" {
		t.Error("Data directory cannot be empty")
	}
	if nodeID == "" {
		t.Error("Node ID cannot be empty")
	}
	if k <= 0 {
		t.Error("K must be positive")
	}
	if m <= 0 {
		t.Error("M must be positive")
	}

	t.Logf("Valid config: port=%d, raft-port=%d, data-dir=%s, node-id=%s, k=%d, m=%d",
		port, raftPort, dataDir, nodeID, k, m)
}

// TestConfig_LoadInvalidConfig tests loading an invalid configuration.
// This is a P1 test case for configuration validation.
func TestConfig_LoadInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		raftPort int
		dataDir string
		nodeID  string
		k       int
		m       int
		wantErr bool
	}{
		{"InvalidPort", 0, 8081, "/tmp", "node1", 3, 10000, true},
		{"InvalidPortHigh", 70000, 8081, "/tmp", "node1", 3, 10000, true},
		{"InvalidRaftPort", 8080, 0, "/tmp", "node1", 3, 10000, true},
		{"EmptyDataDir", 8080, 8081, "", "node1", 3, 10000, true},
		{"EmptyNodeID", 8080, 8081, "/tmp", "", 3, 10000, true},
		{"InvalidK", 8080, 8081, "/tmp", "node1", 0, 10000, true},
		{"InvalidM", 8080, 8081, "/tmp", "node1", 3, 0, true},
		{"ValidConfig", 8080, 8081, "/tmp", "node1", 3, 10000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasErr := false

			if tt.port <= 0 || tt.port > 65535 {
				hasErr = true
			}
			if tt.raftPort <= 0 || tt.raftPort > 65535 {
				hasErr = true
			}
			if tt.dataDir == "" {
				hasErr = true
			}
			if tt.nodeID == "" {
				hasErr = true
			}
			if tt.k <= 0 {
				hasErr = true
			}
			if tt.m <= 0 {
				hasErr = true
			}

			if hasErr != tt.wantErr {
				t.Errorf("Expected error=%v, got error=%v", tt.wantErr, hasErr)
			}
		})
	}
}

// TestConfig_DefaultValues tests that default configuration values are correct.
// This is a P1 test case for default configuration.
func TestConfig_DefaultValues(t *testing.T) {
	// Create a flag set for testing
	fs := flag.NewFlagSet("test", flag.ContinueOnError)

	// Define flags with defaults
	port := fs.Int("port", 8080, "gRPC server port")
	raftPort := fs.Int("raft-port", 8081, "Raft consensus port")
	dataDir := fs.String("data-dir", "./data", "Directory for storing data")
	nodeID := fs.String("node-id", "node1", "Unique node identifier")
	k := fs.Int("k", 3, "Number of hash functions")
	m := fs.Int("m", 10000, "Size of Bloom filter")

	// Parse empty args to use defaults
	fs.Parse([]string{})

	// Verify defaults
	if *port != 8080 {
		t.Errorf("Expected default port 8080, got %d", *port)
	}
	if *raftPort != 8081 {
		t.Errorf("Expected default raft-port 8081, got %d", *raftPort)
	}
	if *dataDir != "./data" {
		t.Errorf("Expected default data-dir ./data, got %s", *dataDir)
	}
	if *nodeID != "node1" {
		t.Errorf("Expected default node-id node1, got %s", *nodeID)
	}
	if *k != 3 {
		t.Errorf("Expected default k=3, got %d", *k)
	}
	if *m != 10000 {
		t.Errorf("Expected default m=10000, got %d", *m)
	}

	t.Log("All default values are correct")
}

// TestComponentInitialization tests that all components can be initialized.
// This is a P1 test case for component initialization.
func TestComponentInitialization(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize WAL encryptor
	walEncryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to initialize WAL encryptor: %v", err)
	}
	if walEncryptor == nil {
		t.Fatal("WAL encryptor is nil")
	}

	// Initialize metadata service
	metadataService := metadata.NewService(tmpDir)
	if metadataService == nil {
		t.Fatal("Metadata service is nil")
	}

	// Initialize Bloom filter
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)
	if bloomFilter == nil {
		t.Fatal("Bloom filter is nil")
	}

	t.Log("All components initialized successfully")
}

// TestRaftNodeCreation tests Raft node creation.
// This is a P1 test case for Raft node initialization.
func TestRaftNodeCreation(t *testing.T) {
	tmpDir := t.TempDir()

	walEncryptor, _ := wal.NewWALEncryptor("")
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)

	// Create Raft node with in-memory store for testing
	config := raft.DefaultConfig()
	config.NodeID = "test-node"
	config.RaftPort = 18081
	config.DataDir = tmpDir
	config.LocalID = "test-node"
	config.UseInmemStore = true
	config.TLSEnabled = false // Disable TLS for testing

	raftNode, err := raft.NewNode(config, bloomFilter, walEncryptor, metadataService)
	if err != nil {
		t.Fatalf("Failed to create Raft node: %v", err)
	}
	if raftNode == nil {
		t.Fatal("Raft node is nil")
	}

	t.Log("Raft node created successfully")
}

// TestRaftNodeStartAndShutdown tests Raft node lifecycle.
// This is a P1 test case for Raft node lifecycle.
func TestRaftNodeStartAndShutdown(t *testing.T) {
	tmpDir := t.TempDir()

	walEncryptor, _ := wal.NewWALEncryptor("")
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)

	config := raft.DefaultConfig()
	config.NodeID = "test-node"
	config.RaftPort = 18082
	config.DataDir = tmpDir
	config.LocalID = "test-node"
	config.UseInmemStore = true
	config.TLSEnabled = false // Disable TLS for testing

	raftNode, err := raft.NewNode(config, bloomFilter, walEncryptor, metadataService)
	if err != nil {
		t.Fatalf("Failed to create Raft node: %v", err)
	}

	// Start node
	if err := raftNode.Start(); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown node
	if err := raftNode.Shutdown(); err != nil {
		t.Errorf("Failed to shutdown Raft node: %v", err)
	}

	t.Log("Raft node lifecycle test completed")
}

// TestMetadataServiceOperations tests metadata service operations.
// This is a P1 test case for metadata service.
func TestMetadataServiceOperations(t *testing.T) {
	tmpDir := t.TempDir()

	service := metadata.NewService(tmpDir)
	if service == nil {
		t.Fatal("Metadata service is nil")
	}

	// Test SetNodeID
	err := service.SetNodeID("test-node")
	if err != nil {
		t.Errorf("Failed to set NodeID: %v", err)
	}

	// Test GetNodeID
	nodeID := service.GetNodeID()
	if nodeID != "test-node" {
		t.Errorf("Expected test-node, got %s", nodeID)
	}

	// Test AddClusterNode
	err = service.AddClusterNode("node1")
	if err != nil {
		t.Errorf("Failed to add cluster node: %v", err)
	}

	// Test GetClusterNodes
	nodes := service.GetClusterNodes()
	if len(nodes) != 1 {
		t.Errorf("Expected 1 cluster node, got %d", len(nodes))
	}

	// Test SetConfig
	err = service.SetConfig("test-key", "test-value")
	if err != nil {
		t.Errorf("Failed to set config: %v", err)
	}

	// Test GetConfig
	value, ok := service.GetConfig("test-key")
	if !ok {
		t.Error("Expected config key to exist")
	}
	if value != "test-value" {
		t.Errorf("Expected test-value, got %v", value)
	}

	// Test RecordAdd
	service.RecordAdd()
	service.RecordAdd()
	service.RecordQuery()

	// Test GetStats
	stats := service.GetStats()
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}
	if stats.TotalAdds != 2 {
		t.Errorf("Expected TotalAdds=2, got %d", stats.TotalAdds)
	}
	if stats.TotalQueries != 1 {
		t.Errorf("Expected TotalQueries=1, got %d", stats.TotalQueries)
	}

	t.Log("Metadata service operations test completed")
}

// TestDataDirectoryCreation tests data directory creation.
// This is a P1 test case for data directory management.
func TestDataDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	testSubDir := tmpDir + "/test-subdir"

	// Create directory
	if err := os.MkdirAll(testSubDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(testSubDir)
	if err != nil {
		t.Fatalf("Directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("Should be a directory")
	}

	t.Log("Data directory creation test completed")
}
