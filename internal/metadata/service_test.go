package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestService_GetNodeID tests NodeID management.
// This is a P1 test case for NodeID management.
func TestService_GetNodeID(t *testing.T) {
	tmpDir := t.TempDir()

	service := NewService(tmpDir)

	// Initial NodeID should be empty
	if service.GetNodeID() != "" {
		t.Errorf("Expected empty NodeID initially, got %s", service.GetNodeID())
	}

	// Set NodeID
	err := service.SetNodeID("test-node-123")
	if err != nil {
		t.Fatalf("Failed to set NodeID: %v", err)
	}

	// Verify NodeID was set
	if service.GetNodeID() != "test-node-123" {
		t.Errorf("Expected test-node-123, got %s", service.GetNodeID())
	}

	t.Log("GetNodeID test passed")
}

// TestService_ClusterNodeManagement tests cluster node management.
// This is a P1 test case for cluster management.
func TestService_ClusterNodeManagement(t *testing.T) {
	tmpDir := t.TempDir()

	service := NewService(tmpDir)

	// Add cluster nodes
	err := service.AddClusterNode("node1")
	if err != nil {
		t.Fatalf("Failed to add cluster node: %v", err)
	}
	err = service.AddClusterNode("node2")
	if err != nil {
		t.Fatalf("Failed to add cluster node: %v", err)
	}
	err = service.AddClusterNode("node3")
	if err != nil {
		t.Fatalf("Failed to add cluster node: %v", err)
	}

	// Verify nodes were added
	nodes := service.GetClusterNodes()
	if len(nodes) != 3 {
		t.Errorf("Expected 3 cluster nodes, got %d", len(nodes))
	}

	// Remove a node
	err = service.RemoveClusterNode("node2")
	if err != nil {
		t.Fatalf("Failed to remove cluster node: %v", err)
	}

	nodes = service.GetClusterNodes()
	if len(nodes) != 2 {
		t.Errorf("Expected 2 cluster nodes after removal, got %d", len(nodes))
	}

	t.Log("ClusterNodeManagement test passed")
}

// TestService_ConfigManagement tests configuration management.
// This is a P1 test case for configuration management.
func TestService_ConfigManagement(t *testing.T) {
	tmpDir := t.TempDir()

	service := NewService(tmpDir)

	// Set config values
	err := service.SetConfig("key1", "value1")
	if err != nil {
		t.Fatalf("Failed to set config: %v", err)
	}
	err = service.SetConfig("key2", 123)
	if err != nil {
		t.Fatalf("Failed to set config: %v", err)
	}

	// Get config value
	value, ok := service.GetConfig("key1")
	if !ok {
		t.Error("Expected key1 to exist")
	}
	if value != "value1" {
		t.Errorf("Expected value1, got %v", value)
	}

	value2, ok := service.GetConfig("key2")
	if !ok {
		t.Error("Expected key2 to exist")
	}
	if value2 != 123 {
		t.Errorf("Expected 123, got %v", value2)
	}

	// Get non-existent key
	_, ok = service.GetConfig("non-existent")
	if ok {
		t.Error("Expected non-existent key to return false")
	}

	t.Log("ConfigManagement test passed")
}

// TestService_StatsRecording tests statistics recording.
// This is a P1 test case for statistics management.
func TestService_StatsRecording(t *testing.T) {
	tmpDir := t.TempDir()

	service := NewService(tmpDir)

	// Record operations
	service.RecordAdd()
	service.RecordAdd()
	service.RecordAdd()
	service.RecordRemove()
	service.RecordQuery()
	service.RecordQuery()

	// Verify stats
	stats := service.GetStats()
	if stats == nil {
		t.Fatal("Stats should not be nil")
	}
	if stats.TotalAdds != 3 {
		t.Errorf("Expected TotalAdds=3, got %d", stats.TotalAdds)
	}
	if stats.TotalRemoves != 1 {
		t.Errorf("Expected TotalRemoves=1, got %d", stats.TotalRemoves)
	}
	if stats.TotalQueries != 2 {
		t.Errorf("Expected TotalQueries=2, got %d", stats.TotalQueries)
	}

	t.Log("StatsRecording test passed")
}

// TestService_Persistence tests metadata persistence to disk.
// This is a P1 test case for persistence.
func TestService_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	service := NewService(tmpDir)

	// Set some values
	service.SetNodeID("persist-test-node")
	service.AddClusterNode("node1")
	service.SetConfig("test-key", "test-value")
	service.RecordAdd()

	// Save to disk
	err := service.Save()
	if err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Verify file exists
	metadataPath := filepath.Join(tmpDir, "metadata.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Fatal("Metadata file should exist")
	}

	// Read file and verify content
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata file: %v", err)
	}
	if len(data) == 0 {
		t.Error("Metadata file should not be empty")
	}

	t.Log("Persistence test passed")
}

// TestService_Recovery tests metadata recovery from disk.
// This is a P1 test case for recovery.
func TestService_Recovery(t *testing.T) {
	tmpDir := t.TempDir()

	// Create first service and set values
	service1 := NewService(tmpDir)
	service1.SetNodeID("recover-test-node")
	service1.AddClusterNode("recover-node1")
	service1.RecordAdd()
	service1.RecordAdd()

	err := service1.Save()
	if err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Create second service (simulating restart)
	service2 := NewService(tmpDir)

	// Load should recover the data
	err = service2.Load()
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	// Verify data was recovered
	if service2.GetNodeID() != "recover-test-node" {
		t.Errorf("Expected recover-test-node, got %s", service2.GetNodeID())
	}

	nodes := service2.GetClusterNodes()
	if len(nodes) != 1 || nodes[0] != "recover-node1" {
		t.Errorf("Expected recover-node1, got %v", nodes)
	}

	stats := service2.GetStats()
	if stats.TotalAdds != 2 {
		t.Errorf("Expected TotalAdds=2, got %d", stats.TotalAdds)
	}

	t.Log("Recovery test passed")
}

// TestService_Load tests the Load method.
// This is a P1 test case for loading metadata.
func TestService_Load(t *testing.T) {
	tmpDir := t.TempDir()

	service := NewService(tmpDir)

	// Load from non-existent file (should not error)
	err := service.Load()
	if err != nil && !os.IsNotExist(err) {
		t.Errorf("Load should not error on non-existent file: %v", err)
	}

	// Create a valid metadata file
	metadata := &Metadata{
		NodeID:    "test-node",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   "1.0.0",
		ClusterNodes: []string{"node1", "node2"},
		Stats: &Stats{
			TotalAdds:    100,
			TotalRemoves: 50,
			TotalQueries: 200,
		},
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("Failed to marshal metadata: %v", err)
	}

	metadataPath := filepath.Join(tmpDir, "metadata.json")
	err = os.WriteFile(metadataPath, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write metadata file: %v", err)
	}

	// Load the metadata
	err = service.Load()
	if err != nil {
		t.Fatalf("Failed to load metadata: %v", err)
	}

	t.Log("Load test passed")
}

// TestService_Save tests the Save method.
// This is a P1 test case for saving metadata.
func TestService_Save(t *testing.T) {
	tmpDir := t.TempDir()

	service := NewService(tmpDir)

	// Set some values
	service.SetNodeID("save-test-node")
	service.AddClusterNode("save-node1")

	// Save
	err := service.Save()
	if err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Verify file exists and is valid JSON
	metadataPath := filepath.Join(tmpDir, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata file: %v", err)
	}

	var loaded Metadata
	err = json.Unmarshal(data, &loaded)
	if err != nil {
		t.Fatalf("Metadata file should be valid JSON: %v", err)
	}

	t.Log("Save test passed")
}

// TestService_ConcurrentAccess tests concurrent access to metadata.
// This is a P1 test case for thread safety.
func TestService_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()

	service := NewService(tmpDir)

	// Run concurrent operations
	var wg sync.WaitGroup
	const goroutines = 10

	// Concurrent writes
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			nodeID := "node-" + string(rune(idx))
			service.AddClusterNode(nodeID)
		}(i)
	}

	wg.Wait()

	// Verify nodes were added
	nodes := service.GetClusterNodes()
	if len(nodes) != goroutines {
		t.Errorf("Expected %d cluster nodes, got %d", goroutines, len(nodes))
	}

	t.Log("Concurrent access test passed")
}

// TestService_BackupCompaction tests backup and compaction timestamp management.
// This is a P1 test case for backup/compaction management.
func TestService_BackupCompaction(t *testing.T) {
	tmpDir := t.TempDir()

	service := NewService(tmpDir)

	// Set backup time
	backupTime := time.Now()
	service.SetLastBackup(backupTime)

	// Set compaction time
	compactionTime := time.Now()
	service.SetLastCompaction(compactionTime)

	// Get metadata and verify
	metadata := service.GetMetadata()
	if metadata == nil {
		t.Fatal("Metadata should not be nil")
	}
	if metadata.Stats.LastBackup.IsZero() {
		t.Error("LastBackup should be set")
	}
	if metadata.Stats.LastCompaction.IsZero() {
		t.Error("LastCompaction should be set")
	}

	t.Log("BackupCompaction test passed")
}

// TestService_GetMetadata tests GetMetadata method.
// This is a P1 test case for metadata retrieval.
func TestService_GetMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	service := NewService(tmpDir)

	// Set some values
	service.SetNodeID("metadata-test-node")
	service.AddClusterNode("meta-node1")
	service.RecordAdd()

	// Get metadata
	metadata := service.GetMetadata()
	if metadata == nil {
		t.Fatal("Metadata should not be nil")
	}
	if metadata.NodeID != "metadata-test-node" {
		t.Errorf("Expected metadata-test-node, got %s", metadata.NodeID)
	}
	if len(metadata.ClusterNodes) != 1 {
		t.Errorf("Expected 1 cluster node, got %d", len(metadata.ClusterNodes))
	}
	if metadata.Stats.TotalAdds != 1 {
		t.Errorf("Expected TotalAdds=1, got %d", metadata.Stats.TotalAdds)
	}

	t.Log("GetMetadata test passed")
}

// TestMetadataService_ConcurrentWrites tests that concurrent writes are atomic and safe.
// This is a P1 test case for atomic write safety.
func TestMetadataService_ConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()

	service := NewService(tmpDir)

	// Set initial node ID
	err := service.SetNodeID("concurrent-test-node")
	if err != nil {
		t.Fatalf("Failed to set node ID: %v", err)
	}

	// Run concurrent save operations
	var wg sync.WaitGroup
	const goroutines = 20
	const savesPerGoroutine = 10

	errors := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < savesPerGoroutine; j++ {
				// Update some data
				service.SetConfig(fmt.Sprintf("goroutine-%d-key-%d", idx, j), j)
				
				// Save concurrently
				if err := service.Save(); err != nil {
					errors <- err
					return
				}
				
				// Small delay to increase contention
				time.Sleep(time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent write error: %v", err)
		errorCount++
	}
	if errorCount > 0 {
		t.Fatalf("Had %d concurrent write errors", errorCount)
	}

	// Verify the metadata file is valid JSON (not corrupted)
	metadataPath := filepath.Join(tmpDir, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata file: %v", err)
	}

	var loaded Metadata
	err = json.Unmarshal(data, &loaded)
	if err != nil {
		t.Fatalf("Metadata file should be valid JSON after concurrent writes: %v", err)
	}

	// Verify file is not empty
	if len(data) == 0 {
		t.Error("Metadata file should not be empty")
	}

	t.Logf("ConcurrentWrites test passed: %d goroutines, %d saves each, file size: %d bytes",
		goroutines, savesPerGoroutine, len(data))
}

// TestMetadataService_AtomicWrite verifies that writes are atomic by checking
// that no partial writes occur even under heavy concurrent load.
func TestMetadataService_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()

	service := NewService(tmpDir)
	service.SetNodeID("atomic-test-node")

	// Run very rapid concurrent writes
	var wg sync.WaitGroup
	const goroutines = 50

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			service.SetConfig("key", idx)
			service.Save()
		}(i)
	}

	wg.Wait()

	// Read the file multiple times to ensure it's always valid
	for i := 0; i < 10; i++ {
		metadataPath := filepath.Join(tmpDir, "metadata.json")
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			t.Fatalf("Failed to read metadata file: %v", err)
		}

		var loaded Metadata
		err = json.Unmarshal(data, &loaded)
		if err != nil {
			t.Fatalf("Metadata file should always be valid JSON (attempt %d): %v", i+1, err)
		}
	}

	t.Log("AtomicWrite test passed: file remained valid under concurrent writes")
}
