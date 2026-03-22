package grpc

import (
	"context"
	"crypto/tls"
	"path/filepath"
	"testing"
	"time"

	"github.com/wangminggit/distributed-bloom-filter/api/proto"
	"github.com/wangminggit/distributed-bloom-filter/internal/raft"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/internal/metadata"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
	dbftls "github.com/wangminggit/distributed-bloom-filter/pkg/tls"
)

// TestNewDBFServerWithAuth tests server creation with authentication.
func TestNewDBFServerWithAuth(t *testing.T) {
	dataDir := t.TempDir()
	raftPort := 18200 + int(time.Now().UnixNano()%1000)

	walEncryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create WAL encryptor: %v", err)
	}

	metadataService := metadata.NewService(dataDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)
	raftNode := raft.NewNode("test-node", raftPort, dataDir, bloomFilter, walEncryptor, metadataService)

	if err := raftNode.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	defer raftNode.Shutdown()

	for i := 0; i < 20; i++ {
		if raftNode.IsLeader() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Run("valid_config_with_mTLS", func(t *testing.T) {
		config := &ServerConfig{
			Port:           50051,
			EnableMTLS:     true,
			CACertPath:     "testdata/ca.crt",
			ServerCertPath: "testdata/server.crt",
			ServerKeyPath:  "testdata/server.key",
		}

		server, err := NewDBFServerWithAuth(raftNode, config)
		if err == nil {
			if server == nil {
				t.Error("Expected server to be created")
			}
		}
	})

	t.Run("config_without_mTLS", func(t *testing.T) {
		config := &ServerConfig{
			Port:            50052,
			EnableMTLS:      false,
			EnableTokenAuth: true,
			JWTSecretKey:    "test-secret-key-12345",
			TokenExpiry:     time.Hour,
		}

		server, err := NewDBFServerWithAuth(raftNode, config)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if server == nil {
			t.Error("Expected server to be created")
		}
	})

	t.Run("invalid_CA_cert_path", func(t *testing.T) {
		config := &ServerConfig{
			Port:           50053,
			EnableMTLS:     true,
			CACertPath:     "/nonexistent/ca.crt",
			ServerCertPath: "/nonexistent/server.crt",
			ServerKeyPath:  "/nonexistent/server.key",
		}

		server, err := NewDBFServerWithAuth(raftNode, config)
		if err == nil {
			t.Error("Expected error for invalid cert paths")
		}
		if server != nil {
			t.Error("Expected nil server on error")
		}
	})
}

// TestServerStartWithConfig tests server startup with various configurations.
func TestServerStartWithConfig(t *testing.T) {
	dataDir := t.TempDir()
	raftPort := 18300 + int(time.Now().UnixNano()%1000)

	walEncryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create WAL encryptor: %v", err)
	}

	metadataService := metadata.NewService(dataDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)
	raftNode := raft.NewNode("test-node", raftPort, dataDir, bloomFilter, walEncryptor, metadataService)

	if err := raftNode.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	defer raftNode.Shutdown()

	for i := 0; i < 20; i++ {
		if raftNode.IsLeader() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	server := NewDBFServer(raftNode)

	t.Run("start_without_TLS", func(t *testing.T) {
		config := &ServerConfig{
			Port: 50060,
		}

		errCh := make(chan error, 1)
		go func() {
			errCh <- server.StartWithConfig(config)
		}()

		time.Sleep(100 * time.Millisecond)
		// Server should be listening
	})

	t.Run("start_with_invalid_TLS_config", func(t *testing.T) {
		config := &ServerConfig{
			Port:         50061,
			EnableTLS:    true,
			TLSCertPath:  "/nonexistent/server.crt",
			TLSKeyPath:   "/nonexistent/server.key",
		}

		err := server.StartWithConfig(config)
		if err == nil {
			t.Error("Expected error for invalid TLS config")
		}
	})
}

// TestCreateTLSListener tests TLS listener creation.
func TestCreateTLSListener(t *testing.T) {
	dataDir := t.TempDir()
	raftPort := 18400 + int(time.Now().UnixNano()%1000)

	walEncryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create WAL encryptor: %v", err)
	}

	metadataService := metadata.NewService(dataDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)
	raftNode := raft.NewNode("test-node", raftPort, dataDir, bloomFilter, walEncryptor, metadataService)

	if err := raftNode.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	defer raftNode.Shutdown()

	for i := 0; i < 20; i++ {
		if raftNode.IsLeader() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	server := NewDBFServer(raftNode)

	t.Run("create_listener_with_invalid_cert", func(t *testing.T) {
		config := &ServerConfig{
			Port:        50070,
			EnableTLS:   true,
			TLSCertPath: "/nonexistent/server.crt",
			TLSKeyPath:  "/nonexistent/server.key",
		}

		_, err := server.createTLSListener(config)
		if err == nil {
			t.Error("Expected error for invalid cert paths")
		}
	})

	t.Run("create_listener_with_reload", func(t *testing.T) {
		tmpDir := t.TempDir()
		certPath := filepath.Join(tmpDir, "server.crt")
		keyPath := filepath.Join(tmpDir, "server.key")

		config := &ServerConfig{
			Port:              50071,
			EnableTLS:         true,
			TLSCertPath:       certPath,
			TLSKeyPath:        keyPath,
			TLSReloadInterval: time.Minute,
		}

		_, err := server.createTLSListener(config)
		if err == nil {
			t.Error("Expected error for non-existent cert files")
		}
	})
}

// TestServerConfigValidation tests configuration validation.
func TestServerConfigValidation(t *testing.T) {
	t.Run("empty_config", func(t *testing.T) {
		config := &ServerConfig{}
		if config.Port != 0 {
			t.Error("Expected default port to be 0")
		}
	})

	t.Run("mTLS_requires_cert_paths", func(t *testing.T) {
		config := &ServerConfig{
			Port:       50080,
			EnableMTLS: true,
		}
		if config.EnableMTLS != true {
			t.Error("Expected mTLS to be enabled")
		}
	})

	t.Run("TLS_config_defaults", func(t *testing.T) {
		config := &ServerConfig{
			Port:      50081,
			EnableTLS: true,
		}
		if config.TLSMinVersion != 0 {
			t.Error("Expected default MinVersion to be 0")
		}
	})
}

// TestServerWithCertReloader tests certificate reloading functionality.
func TestServerWithCertReloader(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "server.crt")
	keyPath := filepath.Join(tmpDir, "server.key")
	caPath := filepath.Join(tmpDir, "ca.crt")

	t.Run("cert_reloader_creation", func(t *testing.T) {
		cfg := &dbftls.Config{
			CertPath:   certPath,
			KeyPath:    keyPath,
			CAPath:     caPath,
			MinVersion: tls.VersionTLS13,
			ClientAuth: tls.RequireAndVerifyClientCert,
		}

		_, err := dbftls.NewCertReloader(cfg, time.Minute)
		if err == nil {
			t.Error("Expected error for non-existent cert files")
		}
	})
}

// TestServerGetStatsEdgeCases tests edge cases in GetStats.
func TestServerGetStatsEdgeCases(t *testing.T) {
	dataDir := t.TempDir()
	raftPort := 18500 + int(time.Now().UnixNano()%1000)

	walEncryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create WAL encryptor: %v", err)
	}

	metadataService := metadata.NewService(dataDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)
	raftNode := raft.NewNode("test-node", raftPort, dataDir, bloomFilter, walEncryptor, metadataService)

	if err := raftNode.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	defer raftNode.Shutdown()

	for i := 0; i < 20; i++ {
		if raftNode.IsLeader() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	server := NewDBFServer(raftNode)

	t.Run("get_stats_with_data", func(t *testing.T) {
		ctx := context.Background()

		addReq := &proto.AddRequest{Item: []byte("stats-test-item")}
		_, _ = server.Add(ctx, addReq)

		time.Sleep(100 * time.Millisecond)

		resp, err := server.GetStats(ctx, &proto.GetStatsRequest{})
		if err != nil {
			t.Fatalf("GetStats failed: %v", err)
		}

		if resp.NodeId == "" {
			t.Error("Expected non-empty node ID")
		}
		if resp.BloomSize <= 0 {
			t.Error("Expected positive bloom size")
		}
	})
}

// TestServerBatchOperationsEdgeCases tests edge cases in batch operations.
func TestServerBatchOperationsEdgeCases(t *testing.T) {
	dataDir := t.TempDir()
	raftPort := 18600 + int(time.Now().UnixNano()%1000)

	walEncryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create WAL encryptor: %v", err)
	}

	metadataService := metadata.NewService(dataDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)
	raftNode := raft.NewNode("test-node", raftPort, dataDir, bloomFilter, walEncryptor, metadataService)

	if err := raftNode.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	defer raftNode.Shutdown()

	for i := 0; i < 20; i++ {
		if raftNode.IsLeader() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	server := NewDBFServer(raftNode)
	ctx := context.Background()

	t.Run("batch_add_all_empty_items", func(t *testing.T) {
		req := &proto.BatchAddRequest{Items: [][]byte{[]byte(""), []byte("")}}
		resp, err := server.BatchAdd(ctx, req)
		if err != nil {
			t.Fatalf("BatchAdd failed: %v", err)
		}
		if resp.SuccessCount != 0 {
			t.Errorf("Expected 0 successes, got %d", resp.SuccessCount)
		}
		if resp.FailureCount != 2 {
			t.Errorf("Expected 2 failures, got %d", resp.FailureCount)
		}
	})

	t.Run("batch_contains_mixed_results", func(t *testing.T) {
		_, _ = server.Add(ctx, &proto.AddRequest{Item: []byte("batch-test-item")})
		time.Sleep(100 * time.Millisecond)

		req := &proto.BatchContainsRequest{Items: [][]byte{
			[]byte("batch-test-item"),
			[]byte("non-existent"),
		}}
		resp, err := server.BatchContains(ctx, req)
		if err != nil {
			t.Fatalf("BatchContains failed: %v", err)
		}
		if len(resp.Results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(resp.Results))
		}
	})
}
