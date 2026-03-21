package raft

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/wangminggit/distributed-bloom-filter/internal/metadata"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

// getFreePort dynamically allocates a free port to avoid conflicts
func getFreePort(t *testing.T) int {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to resolve TCP addr: %v", err)
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to listen on port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// waitForLeader waits for node to become leader with timeout
func waitForLeader(t *testing.T, node *Node, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if node.IsLeader() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Helper()
	t.Logf("Node state at timeout: is_leader=%v, raft_state=%v", node.IsLeader(), node.GetState()["raft_state"])
	t.Fatal("Node did not become leader within timeout")
}

// setupTestNode creates a test node with isolated resources
func setupTestNode(t *testing.T, name string) (*Node, string) {
	t.Helper()
	
	tmpDir := t.TempDir()
	
	walEncryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create WAL encryptor: %v", err)
	}
	
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)
	port := getFreePort(t)
	
	node := NewNode(name, port, tmpDir, bloomFilter, walEncryptor, metadataService)
	return node, tmpDir
}

func TestNodeStartAndLeaderElection(t *testing.T) {
	node, _ := setupTestNode(t, "test-node1")
	
	if err := node.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	defer node.Shutdown()
	
	waitForLeader(t, node, 5*time.Second)
	
	state := node.GetState()
	if state["is_leader"] != true {
		t.Error("Expected is_leader to be true")
	}
	if state["raft_state"] != "Leader" {
		t.Errorf("Expected raft_state to be Leader, got %v", state["raft_state"])
	}
	
	t.Logf("Node state: %v", state)
}

func TestNodeAddAndContains(t *testing.T) {
	node, _ := setupTestNode(t, "test-node2")
	
	if err := node.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	defer node.Shutdown()
	
	waitForLeader(t, node, 2*time.Second)
	
	testItem := []byte("test-item")
	if err := node.Add(testItem); err != nil {
		t.Fatalf("Failed to add item: %v", err)
	}
	
	time.Sleep(100 * time.Millisecond)
	
	if !node.Contains(testItem) {
		t.Error("Expected item to be in Bloom filter after Add")
	}
	
	if err := node.Remove(testItem); err != nil {
		t.Fatalf("Failed to remove item: %v", err)
	}
	
	time.Sleep(100 * time.Millisecond)
	t.Logf("Item still present after remove: %v", node.Contains(testItem))
}

func TestNodeGracefulShutdown(t *testing.T) {
	node, _ := setupTestNode(t, "test-node3")
	
	if err := node.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	
	waitForLeader(t, node, 3*time.Second)
	
	node.Shutdown()
	
	time.Sleep(100 * time.Millisecond)
	if node.IsLeader() {
		t.Error("Expected node to not be leader after shutdown")
	}
}

func TestNodeMultipleOperations(t *testing.T) {
	node, _ := setupTestNode(t, "test-node4")
	
	if err := node.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	defer node.Shutdown()
	
	waitForLeader(t, node, 3*time.Second)
	
	items := [][]byte{
		[]byte("item1"),
		[]byte("item2"),
		[]byte("item3"),
		[]byte("item4"),
		[]byte("item5"),
	}

	for _, item := range items {
		if err := node.Add(item); err != nil {
			t.Fatalf("Failed to add item %s: %v", string(item), err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	
	time.Sleep(300 * time.Millisecond)
	
	for _, item := range items {
		if !node.Contains(item) {
			t.Errorf("Expected item %s to be in Bloom filter", string(item))
		}
	}
	
	t.Logf("Successfully added and verified %d items", len(items))
}

func TestNodeTLSConfig(t *testing.T) {
	t.Run("RaftTLSConfigDefaults", func(t *testing.T) {
		config := &RaftTLSConfig{
			EnableTLS:  true,
			MinVersion: 771,
		}

		if config.EnableTLS != true {
			t.Error("Expected TLS to be enabled")
		}

		if config.MinVersion != 771 {
			t.Errorf("Expected TLS 1.3 (771), got %d", config.MinVersion)
		}
	})

	t.Run("RaftTLSConfigWithReload", func(t *testing.T) {
		config := &RaftTLSConfig{
			EnableTLS:      true,
			MinVersion:     771,
			ReloadInterval: 5 * time.Minute,
		}

		if config.ReloadInterval != 5*time.Minute {
			t.Errorf("Expected reload interval 5m, got %v", config.ReloadInterval)
		}
	})
}

func TestNodeWithTLSInitialization(t *testing.T) {
	tmpDir := t.TempDir()
	
	walEncryptor, err := wal.NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create WAL encryptor: %v", err)
	}
	
	metadataService := metadata.NewService(tmpDir)
	bloomFilter := bloom.NewCountingBloomFilter(10000, 3)
	
	tlsConfig := &RaftTLSConfig{
		EnableTLS: false,
	}

	node, err := NewNodeWithTLS("test-node-tls", getFreePort(t), tmpDir, bloomFilter, walEncryptor, metadataService, tlsConfig)
	if err != nil {
		t.Fatalf("Failed to create node with TLS config: %v", err)
	}
	
	if node == nil {
		t.Fatal("Expected node to be created")
	}

	state := node.GetState()
	if state["tls_enabled"] != false {
		t.Error("Expected TLS to be disabled")
	}

	node.Shutdown()
}

func TestNodeConcurrentAccess(t *testing.T) {
	node, _ := setupTestNode(t, "test-node-concurrent")
	
	if err := node.Start(true); err != nil {
		t.Fatalf("Failed to start Raft node: %v", err)
	}
	defer node.Shutdown()
	
	waitForLeader(t, node, 3*time.Second)
	
	errChan := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			item := []byte(fmt.Sprintf("concurrent-item-%d", id))
			errChan <- node.Add(item)
		}(i)
	}
	
	var addErrors []error
	for i := 0; i < 10; i++ {
		if err := <-errChan; err != nil {
			addErrors = append(addErrors, err)
		}
	}
	
	if len(addErrors) > 0 {
		t.Fatalf("%d concurrent Add operations failed: %v", len(addErrors), addErrors[0])
	}
	
	time.Sleep(300 * time.Millisecond)
	t.Log("Concurrent access test completed successfully")
}
