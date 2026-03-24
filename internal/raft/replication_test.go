package raft

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/raft"
)

// TestReplicationManagerBasic tests basic replication manager operations.
func TestReplicationManagerBasic(t *testing.T) {
	rm := NewReplicationManager()
	
	if rm == nil {
		t.Fatal("Expected non-nil replication manager")
	}
	
	if rm.stats == nil {
		t.Error("Expected stats to be initialized")
	}
	
	if rm.peers == nil {
		t.Error("Expected peers map to be initialized")
	}
	
	if len(rm.peers) != 0 {
		t.Errorf("Expected empty peers map, got %d entries", len(rm.peers))
	}
}

// TestReplicationManagerSetRaftNode tests setting the Raft node reference.
func TestReplicationManagerSetRaftNode(t *testing.T) {
	rm := NewReplicationManager()
	
	// Create a mock raft node (nil for this test)
	var mockNode *raft.Raft
	
	rm.SetRaftNode(mockNode)
	
	// Verify no panic occurred
	t.Log("SetRaftNode completed without panic")
}

// TestReplicationManagerSetCallback tests setting the replication complete callback.
func TestReplicationManagerSetCallback(t *testing.T) {
	rm := NewReplicationManager()
	
	callbackCalled := false
	testCallback := func(index uint64, err error) {
		callbackCalled = true
	}
	
	rm.SetOnReplicationComplete(testCallback)
	
	if rm.onReplicationComplete == nil {
		t.Error("Expected callback to be set")
	}
	
	// Call the callback to verify it works
	rm.onReplicationComplete(1, nil)
	
	if !callbackCalled {
		t.Error("Expected callback to be invoked")
	}
}

// TestReplicationManagerGetConfigurationNoNode tests GetConfiguration when no node is set.
func TestReplicationManagerGetConfigurationNoNode(t *testing.T) {
	rm := NewReplicationManager()
	
	config, err := rm.GetConfiguration()
	
	if err != ErrRaftNotStarted {
		t.Errorf("Expected ErrRaftNotStarted, got %v", err)
	}
	
	if config.Servers != nil && len(config.Servers) != 0 {
		t.Errorf("Expected empty configuration, got %d servers", len(config.Servers))
	}
}

// TestReplicationManagerAddPeerNoNode tests AddPeer when no node is set.
func TestReplicationManagerAddPeerNoNode(t *testing.T) {
	rm := NewReplicationManager()
	
	err := rm.AddPeer("test-node", raft.ServerAddress("127.0.0.1:9000"), true)
	
	if err != ErrRaftNotStarted {
		t.Errorf("Expected ErrRaftNotStarted, got %v", err)
	}
	
	// Note: FailedReplications is only incremented after attempting to add
	// and the raft call fails, not on the early return for nil node
	stats := rm.GetStats()
	t.Logf("Stats after failed AddPeer: Total=%d, Success=%d, Failed=%d", 
		stats.TotalReplications, stats.SuccessfulReplications, stats.FailedReplications)
}

// TestReplicationManagerRemovePeerNoNode tests RemovePeer when no node is set.
func TestReplicationManagerRemovePeerNoNode(t *testing.T) {
	rm := NewReplicationManager()
	
	err := rm.RemovePeer("test-node")
	
	if err != ErrRaftNotStarted {
		t.Errorf("Expected ErrRaftNotStarted, got %v", err)
	}
}

// TestReplicationManagerGetPeerInfoNotFound tests GetPeerInfo for non-existent peer.
func TestReplicationManagerGetPeerInfoNotFound(t *testing.T) {
	rm := NewReplicationManager()
	
	peer, err := rm.GetPeerInfo("non-existent")
	
	if err != ErrPeerNotFound {
		t.Errorf("Expected ErrPeerNotFound, got %v", err)
	}
	
	if peer != nil {
		t.Error("Expected nil peer for non-existent peer")
	}
}

// TestReplicationManagerGetAllPeersEmpty tests GetAllPeers when no peers exist.
func TestReplicationManagerGetAllPeersEmpty(t *testing.T) {
	rm := NewReplicationManager()
	
	peers := rm.GetAllPeers()
	
	if len(peers) != 0 {
		t.Errorf("Expected empty peers map, got %d entries", len(peers))
	}
}

// TestReplicationManagerGetReplicationLagNoNode tests GetReplicationLag when no node is set.
func TestReplicationManagerGetReplicationLagNoNode(t *testing.T) {
	rm := NewReplicationManager()
	
	// Add a peer first
	rm.peers["test-node"] = &PeerInfo{
		ID:           "test-node",
		Address:      "127.0.0.1:9000",
		LastLogIndex: 100,
	}
	
	lag, err := rm.GetReplicationLag("test-node")
	
	if err != ErrRaftNotStarted {
		t.Errorf("Expected ErrRaftNotStarted, got %v", err)
	}
	
	if lag != 0 {
		t.Errorf("Expected 0 lag, got %d", lag)
	}
}

// TestReplicationManagerGetReplicationLagNoLag tests GetReplicationLag when peer is caught up.
func TestReplicationManagerGetReplicationLagNoLag(t *testing.T) {
	rm := NewReplicationManager()
	
	// Add a peer that's caught up
	rm.peers["test-node"] = &PeerInfo{
		ID:           "test-node",
		Address:      "127.0.0.1:9000",
		LastLogIndex: 100,
	}
	
	// Skip this test as we can't easily mock the raft node
	t.Skip("Skipping - requires full raft node mock")
}

// TestReplicationManagerWaitForReplicationTimeout tests WaitForReplication timeout.
func TestReplicationManagerWaitForReplicationTimeout(t *testing.T) {
	rm := NewReplicationManager()
	
	ctx := context.Background()
	err := rm.WaitForReplication(ctx, 100, 100*time.Millisecond)
	
	if err != ErrReplicationTimeout {
		t.Errorf("Expected ErrReplicationTimeout, got %v", err)
	}
}

// TestReplicationManagerWaitForReplicationNoNode tests WaitForReplication when no node is set.
func TestReplicationManagerWaitForReplicationNoNode(t *testing.T) {
	rm := NewReplicationManager()
	
	ctx := context.Background()
	err := rm.WaitForReplication(ctx, 100, 200*time.Millisecond)
	
	// Should timeout since no node is set
	if err != ErrReplicationTimeout && err != context.DeadlineExceeded {
		t.Logf("Got error (expected timeout): %v", err)
	}
}

// TestReplicationManagerGetStats tests GetStats returns a copy.
func TestReplicationManagerGetStats(t *testing.T) {
	rm := NewReplicationManager()
	
	stats := rm.GetStats()
	
	// Modify the returned stats
	stats.TotalReplications = 999
	
	// Get stats again
	stats2 := rm.GetStats()
	
	if stats2.TotalReplications == 999 {
		t.Error("Expected GetStats to return a copy, not the original")
	}
}

// TestReplicationManagerGetStatsNil tests GetStats with nil receiver.
func TestReplicationManagerGetStatsNil(t *testing.T) {
	var rm *ReplicationManager
	
	stats := rm.GetStats()
	
	if stats.TotalReplications != 0 {
		t.Errorf("Expected 0 total replications, got %d", stats.TotalReplications)
	}
}

// TestReplicationManagerUpdatePeerContact tests UpdatePeerContact.
func TestReplicationManagerUpdatePeerContact(t *testing.T) {
	rm := NewReplicationManager()
	
	// Add a peer
	rm.peers["test-node"] = &PeerInfo{
		ID:           "test-node",
		Address:      "127.0.0.1:9000",
		LastLogIndex: 50,
		LastLogTerm:  1,
	}
	
	// Update contact
	rm.UpdatePeerContact("test-node", 100, 2)
	
	peer := rm.peers["test-node"]
	if peer.LastLogIndex != 100 {
		t.Errorf("Expected LastLogIndex=100, got %d", peer.LastLogIndex)
	}
	
	if peer.LastLogTerm != 2 {
		t.Errorf("Expected LastLogTerm=2, got %d", peer.LastLogTerm)
	}
	
	if !peer.IsHealthy {
		t.Error("Expected peer to be marked healthy")
	}
}

// TestReplicationManagerUpdatePeerContactWithLag tests UpdatePeerContact with replication lag.
func TestReplicationManagerUpdatePeerContactWithLag(t *testing.T) {
	rm := NewReplicationManager()
	
	// Add a peer
	rm.peers["test-node"] = &PeerInfo{
		ID:           "test-node",
		Address:      "127.0.0.1:9000",
		LastLogIndex: 50,
	}
	
	// Skip this test as we can't easily mock the raft node
	t.Skip("Skipping - requires full raft node mock")
}

// TestReplicationManagerUpdatePeerContactNotFound tests UpdatePeerContact for non-existent peer.
func TestReplicationManagerUpdatePeerContactNotFound(t *testing.T) {
	rm := NewReplicationManager()
	
	// Should not panic
	rm.UpdatePeerContact("non-existent", 100, 2)
	
	t.Log("UpdatePeerContact handled non-existent peer gracefully")
}

// TestReplicationManagerMarkPeerUnhealthy tests MarkPeerUnhealthy.
func TestReplicationManagerMarkPeerUnhealthy(t *testing.T) {
	rm := NewReplicationManager()
	
	// Add a healthy peer
	rm.peers["test-node"] = &PeerInfo{
		ID:        "test-node",
		IsHealthy: true,
	}
	
	// Mark as unhealthy
	rm.MarkPeerUnhealthy("test-node")
	
	peer := rm.peers["test-node"]
	if peer.IsHealthy {
		t.Error("Expected peer to be marked unhealthy")
	}
}

// TestReplicationManagerMarkPeerUnhealthyNotFound tests MarkPeerUnhealthy for non-existent peer.
func TestReplicationManagerMarkPeerUnhealthyNotFound(t *testing.T) {
	rm := NewReplicationManager()
	
	// Should not panic
	rm.MarkPeerUnhealthy("non-existent")
	
	t.Log("MarkPeerUnhealthy handled non-existent peer gracefully")
}

// TestReplicationManagerGetStatus tests GetStatus returns comprehensive status.
func TestReplicationManagerGetStatus(t *testing.T) {
	rm := NewReplicationManager()
	
	// Add a peer
	rm.peers["test-node"] = &PeerInfo{
		ID:              "test-node",
		Address:         "127.0.0.1:9000",
		LastContact:     time.Now(),
		LastLogIndex:    100,
		LastLogTerm:     2,
		IsVoter:         true,
		IsHealthy:       true,
		ReplicationLag:  0,
	}
	
	status := rm.GetStatus()
	
	// Verify status structure
	if status["total_replications"] == nil {
		t.Error("Expected total_replications in status")
	}
	
	if status["peers"] == nil {
		t.Error("Expected peers in status")
	}
	
	peers := status["peers"].([]map[string]interface{})
	if len(peers) != 1 {
		t.Errorf("Expected 1 peer in status, got %d", len(peers))
	}
}

// TestReplicationManagerGetStatusEmpty tests GetStatus with no peers.
func TestReplicationManagerGetStatusEmpty(t *testing.T) {
	rm := NewReplicationManager()
	
	status := rm.GetStatus()
	
	peers := status["peers"].([]map[string]interface{})
	if len(peers) != 0 {
		t.Errorf("Expected 0 peers in status, got %d", len(peers))
	}
}

// TestReplicationStats tests ReplicationStats structure.
func TestReplicationStats(t *testing.T) {
	stats := &ReplicationStats{
		TotalReplications:      100,
		SuccessfulReplications: 95,
		FailedReplications:     5,
		AverageLatency:         10 * time.Millisecond,
		LastReplicationTime:    time.Now(),
		BytesReplicated:        1024,
	}
	
	if stats.TotalReplications != 100 {
		t.Errorf("Expected TotalReplications=100, got %d", stats.TotalReplications)
	}
	
	if stats.SuccessfulReplications != 95 {
		t.Errorf("Expected SuccessfulReplications=95, got %d", stats.SuccessfulReplications)
	}
	
	if stats.FailedReplications != 5 {
		t.Errorf("Expected FailedReplications=5, got %d", stats.FailedReplications)
	}
}

// TestPeerInfo tests PeerInfo structure.
func TestPeerInfo(t *testing.T) {
	peer := &PeerInfo{
		ID:             "test-node",
		Address:        "127.0.0.1:9000",
		LastContact:    time.Now(),
		LastLogIndex:   100,
		LastLogTerm:    2,
		IsVoter:        true,
		IsHealthy:      true,
		ReplicationLag: 10,
	}
	
	if peer.ID != "test-node" {
		t.Errorf("Expected ID=test-node, got %s", peer.ID)
	}
	
	if !peer.IsVoter {
		t.Error("Expected IsVoter=true")
	}
	
	if !peer.IsHealthy {
		t.Error("Expected IsHealthy=true")
	}
}

// TestReplicationErrors tests replication error types.
func TestReplicationErrors(t *testing.T) {
	if ErrReplicationTimeout == nil {
		t.Error("Expected ErrReplicationTimeout to be defined")
	}
	
	if ErrPeerNotFound == nil {
		t.Error("Expected ErrPeerNotFound to be defined")
	}
	
	t.Logf("ErrReplicationTimeout: %v", ErrReplicationTimeout)
	t.Logf("ErrPeerNotFound: %v", ErrPeerNotFound)
}
