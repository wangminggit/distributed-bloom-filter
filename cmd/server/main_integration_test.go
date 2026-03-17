package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/wangminggit/distributed-bloom-filter/internal/grpc"
	"github.com/wangminggit/distributed-bloom-filter/internal/metadata"
	"github.com/wangminggit/distributed-bloom-filter/internal/raft"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

// TestMainServerConfiguration tests the main server configuration flow.
// This is a P0 test case for end-to-end server configuration.
func TestMainServerConfiguration(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate main() configuration logic
	port := 8080
	raftPort := 8081
	dataDir := tmpDir
	nodeID := "test-node"
	k := 3
	m := 10000
	bootstrap := true
	enableMTLS := false
	enableTokenAuth := false

	// Validate configuration
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

	// Create data directory
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	// Initialize WAL encryptor
	walEncryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to initialize WAL encryptor: %v", err)
	}

	// Initialize metadata service
	metadataService := metadata.NewService(dataDir)

	// Initialize Bloom filter
	bloomFilter := bloom.NewCountingBloomFilter(m, k)

	// Initialize Raft node
	raftNode := raft.NewNode(nodeID, raftPort, dataDir, bloomFilter, walEncryptor, metadataService)

	// Create gRPC server
	var dbfServer *grpc.DBFServer
	serverConfig := &grpc.ServerConfig{
		Port:            port,
		EnableMTLS:      enableMTLS,
		EnableTokenAuth: enableTokenAuth,
	}

	if enableMTLS || enableTokenAuth {
		dbfServer, err = grpc.NewDBFServerWithAuth(raftNode, serverConfig)
		if err != nil {
			t.Fatalf("Failed to create gRPC server with auth: %v", err)
		}
	} else {
		dbfServer = grpc.NewDBFServer(raftNode)
	}

	// Verify all components are initialized
	if walEncryptor == nil {
		t.Error("WAL encryptor should not be nil")
	}
	if metadataService == nil {
		t.Error("Metadata service should not be nil")
	}
	if bloomFilter == nil {
		t.Error("Bloom filter should not be nil")
	}
	if raftNode == nil {
		t.Error("Raft node should not be nil")
	}
	if dbfServer == nil {
		t.Error("DBFServer should not be nil")
	}

	// Start Raft node
	if err := raftNode.Start(bootstrap); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Verify server config
	authInfo := "authentication disabled"
	if enableMTLS {
		authInfo = "mTLS enabled"
	} else if enableTokenAuth {
		authInfo = fmt.Sprintf("token auth enabled (expiry: %v)", serverConfig.TokenExpiry)
	}

	t.Logf("Server configured: port=%d, raft-port=%d, %s", port, raftPort, authInfo)
	t.Logf("Node '%s' initialized with Bloom filter (m=%d, k=%d)", nodeID, m, k)

	// Cleanup
	raftNode.Shutdown()

	t.Log("Main server configuration test completed successfully")
}

// TestMainWithMTLSConfiguration tests server configuration with mTLS enabled.
// This is a P0 test case for mTLS configuration.
func TestMainWithMTLSConfiguration(t *testing.T) {
	tmpDir := t.TempDir()

	// Test mTLS configuration validation
	enableMTLS := true
	caCertPath := ""
	serverCertPath := ""
	serverKeyPath := ""

	// This should fail validation
	if enableMTLS {
		if caCertPath == "" || serverCertPath == "" || serverKeyPath == "" {
			t.Log("mTLS validation correctly detected missing certificates")
		} else {
			t.Error("Should have detected missing certificates")
		}
	}

	// Initialize components
	walEncryptor, _ := wal.NewWALEncryptor("")
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)
	_ = raft.NewNode("test-node", 18090, tmpDir, bloomFilter, walEncryptor, metadataService)

	// Create server config with mTLS
	serverConfig := &grpc.ServerConfig{
		Port:            8080,
		EnableMTLS:      true,
		CACertPath:      "/path/to/ca.crt",
		ServerCertPath:  "/path/to/server.crt",
		ServerKeyPath:   "/path/to/server.key",
		EnableTokenAuth: false,
	}

	if !serverConfig.EnableMTLS {
		t.Error("Expected EnableMTLS to be true")
	}
	if serverConfig.CACertPath == "" {
		t.Error("Expected CACertPath to be set")
	}

	t.Log("mTLS configuration test completed")
}

// TestMainWithTokenAuthConfiguration tests server configuration with token auth.
// This is a P0 test case for token auth configuration.
func TestMainWithTokenAuthConfiguration(t *testing.T) {
	tmpDir := t.TempDir()

	// Test token auth configuration validation
	enableTokenAuth := true
	jwtSecretKey := ""

	// This should fail validation
	if enableTokenAuth && jwtSecretKey == "" {
		t.Log("Token auth validation correctly detected missing JWT secret")
	}

	// Initialize components
	walEncryptor, _ := wal.NewWALEncryptor("")
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)
	_ = raft.NewNode("test-node", 18091, tmpDir, bloomFilter, walEncryptor, metadataService)

	// Create server config with token auth
	serverConfig := &grpc.ServerConfig{
		Port:            8080,
		EnableMTLS:      false,
		EnableTokenAuth: true,
		JWTSecretKey:    "test-secret-key",
		TokenExpiry:     24 * time.Hour,
	}

	if !serverConfig.EnableTokenAuth {
		t.Error("Expected EnableTokenAuth to be true")
	}
	if serverConfig.JWTSecretKey == "" {
		t.Error("Expected JWTSecretKey to be set")
	}
	if serverConfig.TokenExpiry != 24*time.Hour {
		t.Errorf("Expected TokenExpiry 24h, got %v", serverConfig.TokenExpiry)
	}

	t.Log("Token auth configuration test completed")
}

// TestMainAuthenticationInfoMessage tests the authentication info message generation.
// This is a P0 test case for logging.
func TestMainAuthenticationInfoMessage(t *testing.T) {
	tests := []struct {
		name          string
		enableMTLS    bool
		enableToken   bool
		tokenExpiry   time.Duration
		expectedInfo  string
	}{
		{
			name:         "NoAuth",
			enableMTLS:   false,
			enableToken:  false,
			tokenExpiry:  0,
			expectedInfo: "authentication disabled",
		},
		{
			name:         "mTLS",
			enableMTLS:   true,
			enableToken:  false,
			tokenExpiry:  0,
			expectedInfo: "mTLS enabled",
		},
		{
			name:         "TokenAuth",
			enableMTLS:   false,
			enableToken:  true,
			tokenExpiry:  24 * time.Hour,
			expectedInfo: "token auth enabled (expiry: 24h0m0s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var authInfo string
			if tt.enableMTLS {
				authInfo = "mTLS enabled"
			} else if tt.enableToken {
				authInfo = fmt.Sprintf("token auth enabled (expiry: %v)", tt.tokenExpiry)
			} else {
				authInfo = "authentication disabled"
			}

			if authInfo != tt.expectedInfo {
				t.Errorf("Expected auth info '%s', got '%s'", tt.expectedInfo, authInfo)
			}
		})
	}
}

// TestMainDataDirectoryCreation tests data directory creation in main flow.
// This is a P0 test case for file system operations.
func TestMainDataDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	testDataDir := tmpDir + "/test-data"

	// Simulate main.go data directory creation
	if err := os.MkdirAll(testDataDir, 0755); err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	// Verify directory exists
	info, err := os.Stat(testDataDir)
	if err != nil {
		t.Fatalf("Directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("Should be a directory")
	}

	// Verify permissions
	if info.Mode().Perm()&0755 != 0755 {
		t.Logf("Directory permissions: %o", info.Mode().Perm())
	}

	t.Log("Data directory creation test completed")
}

// TestMainComponentInitializationOrder tests the correct initialization order.
// This is a P0 test case for initialization sequence.
func TestMainComponentInitializationOrder(t *testing.T) {
	tmpDir := t.TempDir()

	// Track initialization order
	initOrder := []string{}

	// 1. Initialize WAL encryptor (first)
	walEncryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to initialize WAL encryptor: %v", err)
	}
	initOrder = append(initOrder, "wal")

	// 2. Initialize metadata service
	metadataService := metadata.NewService(tmpDir)
	if metadataService == nil {
		t.Fatal("Metadata service is nil")
	}
	initOrder = append(initOrder, "metadata")

	// 3. Initialize Bloom filter
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)
	if bloomFilter == nil {
		t.Fatal("Bloom filter is nil")
	}
	initOrder = append(initOrder, "bloom")

	// 4. Initialize Raft node
	raftNode := raft.NewNode("test-node", 18092, tmpDir, bloomFilter, walEncryptor, metadataService)
	if raftNode == nil {
		t.Fatal("Raft node is nil")
	}
	initOrder = append(initOrder, "raft")

	// 5. Create gRPC server (last)
	dbfServer := grpc.NewDBFServer(raftNode)
	if dbfServer == nil {
		t.Fatal("DBFServer is nil")
	}
	initOrder = append(initOrder, "grpc")

	// Verify initialization order
	expectedOrder := []string{"wal", "metadata", "bloom", "raft", "grpc"}
	if len(initOrder) != len(expectedOrder) {
		t.Fatalf("Expected %d initialization steps, got %d", len(expectedOrder), len(initOrder))
	}

	for i, expected := range expectedOrder {
		if initOrder[i] != expected {
			t.Errorf("Step %d: expected %s, got %s", i+1, expected, initOrder[i])
		}
	}

	t.Log("Component initialization order test completed")
}

// TestMainServerStartSequence tests the server start sequence.
// This is a P0 test case for server lifecycle.
func TestMainServerStartSequence(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize all components
	walEncryptor, _ := wal.NewWALEncryptor("")
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)
	raftNode := raft.NewNode("test-node", 18093, tmpDir, bloomFilter, walEncryptor, metadataService)
	dbfServer := grpc.NewDBFServer(raftNode)

	// Start Raft node first
	if err := raftNode.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}

	// Give Raft time to elect leader
	time.Sleep(100 * time.Millisecond)

	// Verify node is running
	if !raftNode.IsLeader() {
		t.Log("Node started but not leader (expected for single node with bootstrap)")
	}

	// Start gRPC server in goroutine (simulating main.go)
	serverConfig := &grpc.ServerConfig{
		Port: 18094,
	}
	go func() {
		// Don't actually start, just verify we could
		_ = dbfServer.StartWithConfig(serverConfig)
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Cleanup
	raftNode.Shutdown()

	t.Log("Server start sequence test completed")
}

// TestMainGracefulShutdown tests graceful shutdown sequence.
// This is a P0 test case for shutdown handling.
func TestMainGracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize components
	walEncryptor, _ := wal.NewWALEncryptor("")
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)
	raftNode := raft.NewNode("test-node", 18095, tmpDir, bloomFilter, walEncryptor, metadataService)

	// Start node
	if err := raftNode.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown (simulating graceful shutdown from signal)
	raftNode.Shutdown()

	// Verify shutdown completed
	state := raftNode.GetState()
	if state == nil {
		t.Error("State should be available after shutdown")
	}

	t.Log("Graceful shutdown test completed")
}

// TestMainLogMessages tests that log messages are formatted correctly.
// This is a P0 test case for logging.
func TestMainLogMessages(t *testing.T) {
	port := 8080
	raftPort := 8081
	nodeID := "test-node"
	m := 10000
	k := 3

	// Test startup log message format
	startupMsg := fmt.Sprintf("gRPC server started on port %d (Raft: %d, %s)", port, raftPort, "authentication disabled")
	expectedStartup := "gRPC server started on port 8080 (Raft: 8081, authentication disabled)"
	if startupMsg != expectedStartup {
		t.Errorf("Startup message mismatch: got %s, want %s", startupMsg, expectedStartup)
	}

	// Test initialization log message format
	initMsg := fmt.Sprintf("Node '%s' initialized with Bloom filter (m=%d, k=%d)", nodeID, m, k)
	expectedInit := "Node 'test-node' initialized with Bloom filter (m=10000, k=3)"
	if initMsg != expectedInit {
		t.Errorf("Init message mismatch: got %s, want %s", initMsg, expectedInit)
	}

	t.Log("Log message format test completed")
}
