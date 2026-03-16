package metadata

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestService(t *testing.T) (*Service, string) {
	t.Helper()

	// Create a temporary directory for test data
	tmpDir, err := os.MkdirTemp("", "metadata_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	s := NewService(tmpDir)
	return s, tmpDir
}

func TestNewService(t *testing.T) {
	s, tmpDir := setupTestService(t)
	defer os.RemoveAll(tmpDir)

	if s == nil {
		t.Fatal("Expected service to be created")
	}

	if s.dataDir != tmpDir {
		t.Errorf("Expected dataDir %s, got %s", tmpDir, s.dataDir)
	}

	if s.metadata == nil {
		t.Fatal("Expected metadata to be initialized")
	}

	if s.metadata.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", s.metadata.Version)
	}

	if s.metadata.Stats == nil {
		t.Fatal("Expected stats to be initialized")
	}
}

func TestServiceLoadSave(t *testing.T) {
	s, tmpDir := setupTestService(t)
	defer os.RemoveAll(tmpDir)

	// Set some values
	s.SetNodeID("test-node-1")
	s.AddClusterNode("node-2")
	s.AddClusterNode("node-3")
	s.SetConfig("key1", "value1")
	s.SetConfig("key2", 123)
	s.RecordAdd()
	s.RecordAdd()
	s.RecordQuery()

	// Save
	err := s.Save()
	if err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Verify file exists
	metadataPath := filepath.Join(tmpDir, "metadata.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Fatal("Expected metadata.json to exist")
	}

	// Create a new service and load
	s2 := NewService(tmpDir)
	err = s2.Load()
	if err != nil {
		t.Fatalf("Failed to load metadata: %v", err)
	}

	// Verify loaded values
	if s2.GetNodeID() != "test-node-1" {
		t.Errorf("Expected node ID test-node-1, got %s", s2.GetNodeID())
	}

	nodes := s2.GetClusterNodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 cluster nodes, got %d", len(nodes))
	}

	val, ok := s2.GetConfig("key1")
	if !ok || val != "value1" {
		t.Errorf("Expected key1=value1, got %v, %v", val, ok)
	}

	stats := s2.GetStats()
	if stats.TotalAdds != 2 {
		t.Errorf("Expected 2 adds, got %d", stats.TotalAdds)
	}
	if stats.TotalQueries != 1 {
		t.Errorf("Expected 1 query, got %d", stats.TotalQueries)
	}
}

func TestServiceLoadNonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "metadata_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create service in empty directory - should not fail
	s := NewService(tmpDir)
	if s == nil {
		t.Fatal("Expected service to be created even without existing metadata")
	}
}

func TestSetGetNodeID(t *testing.T) {
	s, tmpDir := setupTestService(t)
	defer os.RemoveAll(tmpDir)

	err := s.SetNodeID("node-123")
	if err != nil {
		t.Fatalf("Failed to set node ID: %v", err)
	}

	nodeID := s.GetNodeID()
	if nodeID != "node-123" {
		t.Errorf("Expected node-123, got %s", nodeID)
	}
}

func TestAddClusterNode(t *testing.T) {
	s, tmpDir := setupTestService(t)
	defer os.RemoveAll(tmpDir)

	// Add first node
	err := s.AddClusterNode("node-1")
	if err != nil {
		t.Fatalf("Failed to add cluster node: %v", err)
	}

	nodes := s.GetClusterNodes()
	if len(nodes) != 1 || nodes[0] != "node-1" {
		t.Errorf("Expected [node-1], got %v", nodes)
	}

	// Add second node
	err = s.AddClusterNode("node-2")
	if err != nil {
		t.Fatalf("Failed to add cluster node: %v", err)
	}

	nodes = s.GetClusterNodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}

	// Add duplicate node - should not error and not add again
	err = s.AddClusterNode("node-1")
	if err != nil {
		t.Fatalf("Adding duplicate node should not error: %v", err)
	}

	nodes = s.GetClusterNodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes after duplicate add, got %d", len(nodes))
	}
}

func TestRemoveClusterNode(t *testing.T) {
	s, tmpDir := setupTestService(t)
	defer os.RemoveAll(tmpDir)

	s.AddClusterNode("node-1")
	s.AddClusterNode("node-2")
	s.AddClusterNode("node-3")

	// Remove middle node
	err := s.RemoveClusterNode("node-2")
	if err != nil {
		t.Fatalf("Failed to remove cluster node: %v", err)
	}

	nodes := s.GetClusterNodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(nodes))
	}

	// Verify remaining nodes
	expected := map[string]bool{"node-1": true, "node-3": true}
	for _, n := range nodes {
		if !expected[n] {
			t.Errorf("Unexpected node %s in %v", n, nodes)
		}
	}

	// Remove non-existent node - should not error
	err = s.RemoveClusterNode("node-nonexistent")
	if err != nil {
		t.Fatalf("Removing non-existent node should not error: %v", err)
	}
}

func TestGetClusterNodesReturnsCopy(t *testing.T) {
	s, tmpDir := setupTestService(t)
	defer os.RemoveAll(tmpDir)

	s.AddClusterNode("node-1")

	// Get nodes and modify the returned slice
	nodes := s.GetClusterNodes()
	nodes = append(nodes, "node-2")

	// Get nodes again - should not include the modification
	nodes2 := s.GetClusterNodes()
	if len(nodes2) != 1 {
		t.Errorf("Expected GetClusterNodes to return a copy, original has %d nodes", len(nodes2))
	}
}

func TestSetGetConfig(t *testing.T) {
	s, tmpDir := setupTestService(t)
	defer os.RemoveAll(tmpDir)

	// Set various types of values
	s.SetConfig("string", "hello")
	s.SetConfig("int", 42)
	s.SetConfig("float", 3.14)
	s.SetConfig("bool", true)
	s.SetConfig("slice", []string{"a", "b", "c"})
	s.SetConfig("map", map[string]int{"x": 1, "y": 2})

	// Verify values
	val, ok := s.GetConfig("string")
	if !ok || val != "hello" {
		t.Errorf("Expected string=hello, got %v, %v", val, ok)
	}

	val, ok = s.GetConfig("int")
	if !ok || val != 42 {
		t.Errorf("Expected int=42, got %v, %v", val, ok)
	}

	val, ok = s.GetConfig("float")
	if !ok || val != 3.14 {
		t.Errorf("Expected float=3.14, got %v, %v", val, ok)
	}

	val, ok = s.GetConfig("bool")
	if !ok || val != true {
		t.Errorf("Expected bool=true, got %v, %v", val, ok)
	}

	// Get non-existent key
	_, ok = s.GetConfig("nonexistent")
	if ok {
		t.Error("Expected ok=false for non-existent key")
	}
}

func TestRecordOperations(t *testing.T) {
	s, tmpDir := setupTestService(t)
	defer os.RemoveAll(tmpDir)

	// Initial stats should be zero
	stats := s.GetStats()
	if stats.TotalAdds != 0 || stats.TotalRemoves != 0 || stats.TotalQueries != 0 {
		t.Errorf("Expected initial stats to be zero, got %+v", stats)
	}

	// Record some operations
	s.RecordAdd()
	s.RecordAdd()
	s.RecordAdd()
	s.RecordRemove()
	s.RecordRemove()
	s.RecordQuery()
	s.RecordQuery()
	s.RecordQuery()
	s.RecordQuery()

	stats = s.GetStats()
	if stats.TotalAdds != 3 {
		t.Errorf("Expected 3 adds, got %d", stats.TotalAdds)
	}
	if stats.TotalRemoves != 2 {
		t.Errorf("Expected 2 removes, got %d", stats.TotalRemoves)
	}
	if stats.TotalQueries != 4 {
		t.Errorf("Expected 4 queries, got %d", stats.TotalQueries)
	}
}

func TestSetLastBackup(t *testing.T) {
	s, tmpDir := setupTestService(t)
	defer os.RemoveAll(tmpDir)

	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	s.SetLastBackup(testTime)

	stats := s.GetStats()
	if stats.LastBackup != testTime {
		t.Errorf("Expected LastBackup %v, got %v", testTime, stats.LastBackup)
	}
}

func TestSetLastCompaction(t *testing.T) {
	s, tmpDir := setupTestService(t)
	defer os.RemoveAll(tmpDir)

	testTime := time.Date(2024, 2, 20, 14, 45, 0, 0, time.UTC)
	s.SetLastCompaction(testTime)

	stats := s.GetStats()
	if stats.LastCompaction != testTime {
		t.Errorf("Expected LastCompaction %v, got %v", testTime, stats.LastCompaction)
	}
}

func TestGetMetadata(t *testing.T) {
	s, tmpDir := setupTestService(t)
	defer os.RemoveAll(tmpDir)

	s.SetNodeID("test-node")
	s.AddClusterNode("node-1")
	s.AddClusterNode("node-2")
	s.SetConfig("key", "value")
	s.RecordAdd()
	s.RecordQuery()

	testTime := time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)
	s.SetLastBackup(testTime)

	metadata := s.GetMetadata()

	if metadata.NodeID != "test-node" {
		t.Errorf("Expected NodeID test-node, got %s", metadata.NodeID)
	}

	if len(metadata.ClusterNodes) != 2 {
		t.Errorf("Expected 2 cluster nodes, got %d", len(metadata.ClusterNodes))
	}

	if metadata.Stats.TotalAdds != 1 {
		t.Errorf("Expected 1 add, got %d", metadata.Stats.TotalAdds)
	}

	if metadata.Stats.LastBackup != testTime {
		t.Errorf("Expected LastBackup %v, got %v", testTime, metadata.Stats.LastBackup)
	}

	// Verify it's a copy - modify and check original
	metadata.NodeID = "modified-node"
	metadata.ClusterNodes[0] = "modified"
	metadata.Stats.TotalAdds = 999

	metadata2 := s.GetMetadata()
	if metadata2.NodeID == "modified-node" {
		t.Error("Expected GetMetadata to return a copy")
	}
	if metadata2.Stats.TotalAdds == 999 {
		t.Error("Expected GetMetadata stats to be a copy")
	}
}

func TestGetMetadataEmptyService(t *testing.T) {
	s, tmpDir := setupTestService(t)
	defer os.RemoveAll(tmpDir)

	metadata := s.GetMetadata()

	if metadata == nil {
		t.Fatal("Expected metadata to be returned")
	}

	if metadata.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	if metadata.Version != "1.0.0" {
		t.Errorf("Expected version 1.0.0, got %s", metadata.Version)
	}

	if metadata.Stats == nil {
		t.Error("Expected Stats to be initialized")
	}
}

func TestServiceSaveToReadOnlyDir(t *testing.T) {
	// Create a directory and make it read-only
	tmpDir, err := os.MkdirTemp("", "metadata_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Make directory read-only
	err = os.Chmod(tmpDir, 0555)
	if err != nil {
		t.Skip("Cannot change directory permissions on this system")
	}

	s := NewService(tmpDir)
	err = s.Save()

	// Restore permissions for cleanup
	os.Chmod(tmpDir, 0755)

	// Save should fail on read-only directory
	if err == nil {
		t.Error("Expected error when saving to read-only directory")
	}
}

func TestServiceLoadCorruptedJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "metadata_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write corrupted JSON
	metadataPath := filepath.Join(tmpDir, "metadata.json")
	err = os.WriteFile(metadataPath, []byte("{ invalid json }"), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	s := NewService(tmpDir)
	err = s.Load()

	if err == nil {
		t.Error("Expected error when loading corrupted JSON")
	}
}

func TestConcurrentAccess(t *testing.T) {
	s, tmpDir := setupTestService(t)
	defer os.RemoveAll(tmpDir)

	done := make(chan bool)

	// Start multiple goroutines that read and write
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				s.SetNodeID("node")
				s.GetNodeID()
				s.AddClusterNode("node")
				s.GetClusterNodes()
				s.SetConfig("key", "value")
				s.GetConfig("key")
				s.RecordAdd()
				s.GetStats()
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we got here without panicking, the test passed
}
