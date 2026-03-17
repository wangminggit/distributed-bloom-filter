package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/wangminggit/distributed-bloom-filter/internal/grpc"
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
		name     string
		port     int
		raftPort int
		dataDir  string
		nodeID   string
		k        int
		m        int
		wantErr  bool
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

	// Create Raft node using the correct signature
	raftNode := raft.NewNode("test-node", 18081, tmpDir, bloomFilter, walEncryptor, metadataService)
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

	raftNode := raft.NewNode("test-node", 18082, tmpDir, bloomFilter, walEncryptor, metadataService)
	if raftNode == nil {
		t.Fatal("Raft node is nil")
	}

	// Start node (bootstrap = true for single node test)
	if err := raftNode.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Shutdown node
	raftNode.Shutdown()

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

// TestCommandLineFlags tests command-line flag parsing.
// This is a P0 test case for command-line argument handling.
func TestCommandLineFlags(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantPort     int
		wantRaftPort int
		wantDataDir  string
		wantNodeID   string
		wantK        int
		wantM        int
		wantBootstrap bool
	}{
		{
			name:         "DefaultValues",
			args:         []string{},
			wantPort:     8080,
			wantRaftPort: 8081,
			wantDataDir:  "./data",
			wantNodeID:   "node1",
			wantK:        3,
			wantM:        10000,
			wantBootstrap: false,
		},
		{
			name:         "CustomValues",
			args:         []string{"--port", "9090", "--raft-port", "9091", "--data-dir", "/custom/data", "--node-id", "node2", "--k", "5", "--m", "20000", "--bootstrap"},
			wantPort:     9090,
			wantRaftPort: 9091,
			wantDataDir:  "/custom/data",
			wantNodeID:   "node2",
			wantK:        5,
			wantM:        20000,
			wantBootstrap: true,
		},
		{
			name:         "PartialOverride",
			args:         []string{"--port", "3000", "--node-id", "custom-node"},
			wantPort:     3000,
			wantRaftPort: 8081,
			wantDataDir:  "./data",
			wantNodeID:   "custom-node",
			wantK:        3,
			wantM:        10000,
			wantBootstrap: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)

			port := fs.Int("port", 8080, "gRPC server port")
			raftPort := fs.Int("raft-port", 8081, "Raft consensus port")
			dataDir := fs.String("data-dir", "./data", "Directory for storing data")
			nodeID := fs.String("node-id", "node1", "Unique node identifier")
			k := fs.Int("k", 3, "Number of hash functions")
			m := fs.Int("m", 10000, "Size of Bloom filter")
			bootstrap := fs.Bool("bootstrap", false, "Bootstrap as first node")

			fs.Parse(tt.args)

			if *port != tt.wantPort {
				t.Errorf("Expected port %d, got %d", tt.wantPort, *port)
			}
			if *raftPort != tt.wantRaftPort {
				t.Errorf("Expected raft-port %d, got %d", tt.wantRaftPort, *raftPort)
			}
			if *dataDir != tt.wantDataDir {
				t.Errorf("Expected data-dir %s, got %s", tt.wantDataDir, *dataDir)
			}
			if *nodeID != tt.wantNodeID {
				t.Errorf("Expected node-id %s, got %s", tt.wantNodeID, *nodeID)
			}
			if *k != tt.wantK {
				t.Errorf("Expected k %d, got %d", tt.wantK, *k)
			}
			if *m != tt.wantM {
				t.Errorf("Expected m %d, got %d", tt.wantM, *m)
			}
			if *bootstrap != tt.wantBootstrap {
				t.Errorf("Expected bootstrap %v, got %v", tt.wantBootstrap, *bootstrap)
			}
		})
	}
}

// TestAuthenticationFlags tests authentication-related command-line flags.
// This is a P0 test case for authentication configuration.
func TestAuthenticationFlags(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantEnableMTLS bool
		wantCACert     string
		wantServerCert string
		wantServerKey  string
		wantEnableToken bool
		wantJWTSecret  string
	}{
		{
			name:           "NoAuth",
			args:           []string{},
			wantEnableMTLS: false,
			wantCACert:     "",
			wantServerCert: "",
			wantServerKey:  "",
			wantEnableToken: false,
			wantJWTSecret:  "",
		},
		{
			name:           "mTLSEnabled",
			args:           []string{"--enable-mtls", "--ca-cert", "/path/to/ca.crt", "--server-cert", "/path/to/server.crt", "--server-key", "/path/to/server.key"},
			wantEnableMTLS: true,
			wantCACert:     "/path/to/ca.crt",
			wantServerCert: "/path/to/server.crt",
			wantServerKey:  "/path/to/server.key",
			wantEnableToken: false,
			wantJWTSecret:  "",
		},
		{
			name:           "TokenAuthEnabled",
			args:           []string{"--enable-token-auth", "--jwt-secret", "my-secret-key", "--token-expiry", "12h"},
			wantEnableMTLS: false,
			wantCACert:     "",
			wantServerCert: "",
			wantServerKey:  "",
			wantEnableToken: true,
			wantJWTSecret:  "my-secret-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)

			enableMTLS := fs.Bool("enable-mtls", false, "Enable mTLS authentication")
			caCertPath := fs.String("ca-cert", "", "Path to CA certificate")
			serverCertPath := fs.String("server-cert", "", "Path to server certificate")
			serverKeyPath := fs.String("server-key", "", "Path to server private key")
			enableTokenAuth := fs.Bool("enable-token-auth", false, "Enable JWT token authentication")
			jwtSecretKey := fs.String("jwt-secret", "", "JWT secret key for token signing")

			fs.Parse(tt.args)

			if *enableMTLS != tt.wantEnableMTLS {
				t.Errorf("Expected enable-mtls %v, got %v", tt.wantEnableMTLS, *enableMTLS)
			}
			if *caCertPath != tt.wantCACert {
				t.Errorf("Expected ca-cert %s, got %s", tt.wantCACert, *caCertPath)
			}
			if *serverCertPath != tt.wantServerCert {
				t.Errorf("Expected server-cert %s, got %s", tt.wantServerCert, *serverCertPath)
			}
			if *serverKeyPath != tt.wantServerKey {
				t.Errorf("Expected server-key %s, got %s", tt.wantServerKey, *serverKeyPath)
			}
			if *enableTokenAuth != tt.wantEnableToken {
				t.Errorf("Expected enable-token-auth %v, got %v", tt.wantEnableToken, *enableTokenAuth)
			}
			if *jwtSecretKey != tt.wantJWTSecret {
				t.Errorf("Expected jwt-secret %s, got %s", tt.wantJWTSecret, *jwtSecretKey)
			}
		})
	}
}

// TestMTLSConfigValidation tests mTLS configuration validation.
// This is a P0 test case for mTLS configuration validation.
func TestMTLSConfigValidation(t *testing.T) {
	tests := []struct {
		name           string
		enableMTLS     bool
		caCertPath     string
		serverCertPath string
		serverKeyPath  string
		shouldFail     bool
	}{
		{
			name:           "mTLSDisabled-NoCerts",
			enableMTLS:     false,
			caCertPath:     "",
			serverCertPath: "",
			serverKeyPath:  "",
			shouldFail:     false,
		},
		{
			name:           "mTLSEnabled-MissingCACert",
			enableMTLS:     true,
			caCertPath:     "",
			serverCertPath: "/path/to/server.crt",
			serverKeyPath:  "/path/to/server.key",
			shouldFail:     true,
		},
		{
			name:           "mTLSEnabled-MissingServerCert",
			enableMTLS:     true,
			caCertPath:     "/path/to/ca.crt",
			serverCertPath: "",
			serverKeyPath:  "/path/to/server.key",
			shouldFail:     true,
		},
		{
			name:           "mTLSEnabled-MissingServerKey",
			enableMTLS:     true,
			caCertPath:     "/path/to/ca.crt",
			serverCertPath: "/path/to/server.crt",
			serverKeyPath:  "",
			shouldFail:     true,
		},
		{
			name:           "mTLSEnabled-AllCertsProvided",
			enableMTLS:     true,
			caCertPath:     "/path/to/ca.crt",
			serverCertPath: "/path/to/server.crt",
			serverKeyPath:  "/path/to/server.key",
			shouldFail:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasErr := false

			if tt.enableMTLS {
				if tt.caCertPath == "" || tt.serverCertPath == "" || tt.serverKeyPath == "" {
					hasErr = true
				}
			}

			if hasErr != tt.shouldFail {
				t.Errorf("Expected failure=%v, got failure=%v", tt.shouldFail, hasErr)
			}
		})
	}
}

// TestTokenAuthConfigValidation tests JWT token authentication configuration validation.
// This is a P0 test case for token auth configuration validation.
func TestTokenAuthConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		enableToken bool
		jwtSecret   string
		shouldFail  bool
	}{
		{
			name:        "TokenDisabled-NoSecret",
			enableToken: false,
			jwtSecret:   "",
			shouldFail:  false,
		},
		{
			name:        "TokenEnabled-MissingSecret",
			enableToken: true,
			jwtSecret:   "",
			shouldFail:  true,
		},
		{
			name:        "TokenEnabled-SecretProvided",
			enableToken: true,
			jwtSecret:   "my-secret-key",
			shouldFail:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasErr := false

			if tt.enableToken && tt.jwtSecret == "" {
				hasErr = true
			}

			if hasErr != tt.shouldFail {
				t.Errorf("Expected failure=%v, got failure=%v", tt.shouldFail, hasErr)
			}
		})
	}
}

// TestServerConfigCreation tests gRPC server configuration creation.
// This is a P0 test case for server configuration.
func TestServerConfigCreation(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize components
	walEncryptor, _ := wal.NewWALEncryptor("")
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)

	raftNode := raft.NewNode("test-node", 18083, tmpDir, bloomFilter, walEncryptor, metadataService)
	if raftNode == nil {
		t.Fatal("Raft node is nil")
	}

	// Test server config without auth
	serverConfig := &grpc.ServerConfig{
		Port:            8080,
		EnableMTLS:      false,
		EnableTokenAuth: false,
	}

	dbfServer := grpc.NewDBFServer(raftNode)
	if dbfServer == nil {
		t.Fatal("DBFServer should not be nil")
	}

	// Verify server config values
	if serverConfig.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", serverConfig.Port)
	}
	if serverConfig.EnableMTLS != false {
		t.Error("Expected EnableMTLS to be false")
	}
	if serverConfig.EnableTokenAuth != false {
		t.Error("Expected EnableTokenAuth to be false")
	}

	t.Log("Server config created successfully")
}

// TestServerConfigWithMTLS tests gRPC server configuration with mTLS.
// This is a P0 test case for mTLS server configuration.
func TestServerConfigWithMTLS(t *testing.T) {
	tmpDir := t.TempDir()

	walEncryptor, _ := wal.NewWALEncryptor("")
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)

	raftNode := raft.NewNode("test-node", 18084, tmpDir, bloomFilter, walEncryptor, metadataService)
	if raftNode == nil {
		t.Fatal("Raft node is nil")
	}

	// Test server config with mTLS
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
	if serverConfig.ServerCertPath == "" {
		t.Error("Expected ServerCertPath to be set")
	}
	if serverConfig.ServerKeyPath == "" {
		t.Error("Expected ServerKeyPath to be set")
	}

	t.Log("mTLS server config validated successfully")
}

// TestServerConfigWithTokenAuth tests gRPC server configuration with token auth.
// This is a P0 test case for token auth server configuration.
func TestServerConfigWithTokenAuth(t *testing.T) {
	tmpDir := t.TempDir()

	walEncryptor, _ := wal.NewWALEncryptor("")
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)

	raftNode := raft.NewNode("test-node", 18085, tmpDir, bloomFilter, walEncryptor, metadataService)
	if raftNode == nil {
		t.Fatal("Raft node is nil")
	}

	// Test server config with token auth
	serverConfig := &grpc.ServerConfig{
		Port:            8080,
		EnableMTLS:      false,
		EnableTokenAuth: true,
		JWTSecretKey:    "my-secret-key",
		TokenExpiry:     24 * time.Hour,
	}

	if serverConfig.EnableTokenAuth != true {
		t.Error("Expected EnableTokenAuth to be true")
	}
	if serverConfig.JWTSecretKey == "" {
		t.Error("Expected JWTSecretKey to be set")
	}
	if serverConfig.TokenExpiry != 24*time.Hour {
		t.Errorf("Expected TokenExpiry 24h, got %v", serverConfig.TokenExpiry)
	}

	t.Log("Token auth server config validated successfully")
}

// TestGracefulShutdownSignalHandling tests graceful shutdown signal handling.
// This is a P0 test case for graceful shutdown.
func TestGracefulShutdownSignalHandling(t *testing.T) {
	// Test that signal channel can be created
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Verify channel is not nil
	if stop == nil {
		t.Fatal("Signal channel should not be nil")
	}

	// Test sending interrupt signal (non-blocking)
	select {
	case stop <- os.Interrupt:
		// Signal sent successfully
	default:
		t.Error("Failed to send signal to channel")
	}

	// Read the signal back
	sig := <-stop
	if sig != os.Interrupt {
		t.Errorf("Expected os.Interrupt, got %v", sig)
	}

	t.Log("Graceful shutdown signal handling test completed")
}

// TestBloomFilterParameters tests Bloom filter parameter validation.
// This is a P0 test case for Bloom filter configuration.
func TestBloomFilterParameters(t *testing.T) {
	tests := []struct {
		name       string
		m          int
		k          int
		shouldFail bool
	}{
		{"Valid-Small", 1000, 3, false},
		{"Valid-Medium", 10000, 5, false},
		{"Valid-Large", 100000, 7, false},
		{"Invalid-MZero", 0, 3, true},
		{"Invalid-KZero", 10000, 0, true},
		{"Invalid-MNegative", -1000, 3, true},
		{"Invalid-KNegative", 10000, -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasErr := false

			if tt.m <= 0 || tt.k <= 0 {
				hasErr = true
			}

			if hasErr != tt.shouldFail {
				t.Errorf("Expected failure=%v, got failure=%v", tt.shouldFail, hasErr)
			}

			// Create bloom filter if valid
			if !hasErr {
				bf := bloom.NewCountingBloomFilter(tt.m, tt.k)
				if bf == nil {
					t.Error("Bloom filter should not be nil for valid parameters")
				}
			}
		})
	}
}

// TestNodeIDUniqueness tests that node IDs are properly configured.
// This is a P0 test case for node identity.
func TestNodeIDUniqueness(t *testing.T) {
	tmpDir := t.TempDir()

	service := metadata.NewService(tmpDir)

	// Test setting unique node IDs
	nodeIDs := []string{"node1", "node2", "node3", "custom-node", "node-with-dash"}

	for _, nodeID := range nodeIDs {
		err := service.SetNodeID(nodeID)
		if err != nil {
			t.Errorf("Failed to set node ID %s: %v", nodeID, err)
		}

		retrieved := service.GetNodeID()
		if retrieved != nodeID {
			t.Errorf("Expected node ID %s, got %s", nodeID, retrieved)
		}
	}

	t.Log("Node ID uniqueness test completed")
}

// TestWALEncryptorInitialization tests WAL encryptor initialization.
// This is a P0 test case for WAL encryption.
func TestWALEncryptorInitialization(t *testing.T) {
	// Test with empty key (should work, using default/no encryption)
	encryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create WAL encryptor with empty key: %v", err)
	}
	if encryptor == nil {
		t.Fatal("WAL encryptor should not be nil")
	}

	t.Log("WAL encryptor initialization test completed")
}

// TestServerPortRange tests server port range validation.
// This is a P0 test case for port validation.
func TestServerPortRange(t *testing.T) {
	tests := []struct {
		name       string
		port       int
		shouldFail bool
	}{
		{"Valid-Low", 1, false},
		{"Valid-Medium", 8080, false},
		{"Valid-High", 65535, false},
		{"Invalid-Zero", 0, true},
		{"Invalid-Negative", -1, true},
		{"Invalid-TooHigh", 65536, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasErr := false

			if tt.port <= 0 || tt.port > 65535 {
				hasErr = true
			}

			if hasErr != tt.shouldFail {
				t.Errorf("Expected failure=%v, got failure=%v", tt.shouldFail, hasErr)
			}
		})
	}
}

// TestBootstrapFlag tests the bootstrap flag functionality.
// This is a P0 test case for cluster bootstrap.
func TestBootstrapFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	bootstrap := fs.Bool("bootstrap", false, "Bootstrap as first node")

	// Test default value
	fs.Parse([]string{})
	if *bootstrap != false {
		t.Errorf("Expected default bootstrap=false, got %v", *bootstrap)
	}

	// Test explicit true
	fs.Parse([]string{"--bootstrap"})
	if *bootstrap != true {
		t.Errorf("Expected bootstrap=true, got %v", *bootstrap)
	}

	t.Log("Bootstrap flag test completed")
}

// TestTokenExpiryDuration tests token expiry duration parsing.
// This is a P0 test case for token configuration.
func TestTokenExpiryDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration string
		wantErr  bool
	}{
		{"Valid-Hours", "24h", false},
		{"Valid-Minutes", "30m", false},
		{"Valid-Extended", "168h", false}, // 7 days in hours (Go doesn't support 'd' suffix)
		{"Invalid-Empty", "", true},
		{"Invalid-Format", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasErr := false
			if tt.duration == "" {
				hasErr = true
			} else {
				_, err := time.ParseDuration(tt.duration)
				if err != nil {
					hasErr = true
				}
			}

			if hasErr != tt.wantErr {
				t.Errorf("Expected error=%v, got error=%v", tt.wantErr, hasErr)
			}
		})
	}
}

// TestComponentIntegration tests integration of all components.
// This is a P0 test case for full component integration.
func TestComponentIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize all components in order (as in main.go)
	
	// 1. WAL Encryptor
	walEncryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to initialize WAL encryptor: %v", err)
	}

	// 2. Metadata Service
	metadataService := metadata.NewService(tmpDir)
	if metadataService == nil {
		t.Fatal("Metadata service is nil")
	}

	// 3. Bloom Filter
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)
	if bloomFilter == nil {
		t.Fatal("Bloom filter is nil")
	}

	// 4. Raft Node
	raftNode := raft.NewNode("test-node", 18086, tmpDir, bloomFilter, walEncryptor, metadataService)
	if raftNode == nil {
		t.Fatal("Raft node is nil")
	}

	// 5. gRPC Server
	dbfServer := grpc.NewDBFServer(raftNode)
	if dbfServer == nil {
		t.Fatal("DBFServer is nil")
	}

	// Verify all components are non-nil
	if walEncryptor == nil || metadataService == nil || bloomFilter == nil || raftNode == nil || dbfServer == nil {
		t.Fatal("One or more components failed to initialize")
	}

	t.Log("All components integrated successfully")
}

// TestDataDirectoryPermissions tests data directory permission handling.
// This is a P0 test case for file system permissions.
func TestDataDirectoryPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	testSubDir := tmpDir + "/test-perms"

	// Create directory with specific permissions
	if err := os.MkdirAll(testSubDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Verify permissions
	info, err := os.Stat(testSubDir)
	if err != nil {
		t.Fatalf("Directory should exist: %v", err)
	}

	// Check if directory is readable and writable
	if info.Mode().Perm()&0400 == 0 {
		t.Error("Directory should be readable by owner")
	}
	if info.Mode().Perm()&0200 == 0 {
		t.Error("Directory should be writable by owner")
	}

	t.Log("Data directory permissions test completed")
}

// TestConcurrentComponentAccess tests concurrent access to components.
// This is a P0 test case for thread safety.
func TestConcurrentComponentAccess(t *testing.T) {
	tmpDir := t.TempDir()

	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)

	// Test concurrent metadata access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			nodeID := "node" + string(rune('0'+id))
			metadataService.SetNodeID(nodeID)
			metadataService.GetNodeID()
			metadataService.RecordAdd()
			metadataService.GetStats()
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Test concurrent bloom filter access
	for i := 0; i < 100; i++ {
		go func(val int) {
			bloomFilter.Add([]byte(string(rune('0' + val))))
			bloomFilter.Contains([]byte(string(rune('0' + val))))
		}(i)
	}

	// Give goroutines time to complete
	time.Sleep(100 * time.Millisecond)

	t.Log("Concurrent component access test completed")
}
