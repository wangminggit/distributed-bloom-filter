package raft

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/raft"
)

// TestElectionManagerMonitorLeaderChanges tests MonitorLeaderChanges.
func TestElectionManagerMonitorLeaderChanges(t *testing.T) {
	em := NewElectionManager()
	
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	
	em.SetOnLeaderChange(func(leader string, addr raft.ServerAddress) {
		t.Logf("Leader changed to: %s (%s)", leader, addr)
	})
	
	// Start monitoring
	go em.MonitorLeaderChanges(ctx, 50*time.Millisecond)
	
	// Wait for context to expire
	<-ctx.Done()
	
	t.Log("MonitorLeaderChanges test completed")
}

// TestElectionManagerMonitorLeaderChangesNoCallback tests without callback.
func TestElectionManagerMonitorLeaderChangesNoCallback(t *testing.T) {
	em := NewElectionManager()
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// Start monitoring without callback
	go em.MonitorLeaderChanges(ctx, 50*time.Millisecond)
	
	<-ctx.Done()
	
	t.Log("MonitorLeaderChanges without callback test completed")
}

// TestElectionManagerGetLeaderDuration tests GetLeaderDuration.
func TestElectionManagerGetLeaderDuration(t *testing.T) {
	em := NewElectionManager()
	
	// Initial duration should be 0
	duration := em.GetLeaderDuration()
	if duration != 0 {
		t.Errorf("Expected 0 duration, got %v", duration)
	}
	
	// Simulate leader change by setting leaderChangedAt
	em.mu.Lock()
	em.leaderChangedAt = time.Now().Add(-1 * time.Second)
	em.currentLeader = "test-leader"
	em.mu.Unlock()
	
	duration = em.GetLeaderDuration()
	if duration < 1*time.Second {
		t.Errorf("Expected duration >= 1s, got %v", duration)
	}
	
	t.Logf("Leader duration: %v", duration)
}

// TestElectionManagerGetStats tests GetStats.
func TestElectionManagerGetStats(t *testing.T) {
	em := NewElectionManager()
	
	// Record some events
	em.RecordVoteReceived()
	em.RecordVoteReceived()
	em.RecordVoteCast()
	em.RecordElection(5)
	
	stats := em.GetStats()
	
	if stats.TotalVotesReceived != 2 {
		t.Errorf("Expected 2 votes received, got %d", stats.TotalVotesReceived)
	}
	
	if stats.TotalVotesCast != 1 {
		t.Errorf("Expected 1 vote cast, got %d", stats.TotalVotesCast)
	}
	
	if stats.TotalElections != 1 {
		t.Errorf("Expected 1 election, got %d", stats.TotalElections)
	}
	
	if stats.LastElectionTerm != 5 {
		t.Errorf("Expected term 5, got %d", stats.LastElectionTerm)
	}
	
	t.Logf("Election stats: %v", stats)
}

// TestElectionManagerGetStatsNil tests GetStats with nil receiver.
func TestElectionManagerGetStatsNil(t *testing.T) {
	var em *ElectionManager
	
	stats := em.GetStats()
	
	if stats.TotalElections != 0 {
		t.Errorf("Expected 0 elections, got %d", stats.TotalElections)
	}
	
	t.Log("GetStats with nil receiver handled correctly")
}

// TestElectionManagerRecordVoteReceived tests RecordVoteReceived.
func TestElectionManagerRecordVoteReceived(t *testing.T) {
	em := NewElectionManager()
	
	for i := 0; i < 5; i++ {
		em.RecordVoteReceived()
	}
	
	stats := em.GetStats()
	if stats.TotalVotesReceived != 5 {
		t.Errorf("Expected 5 votes received, got %d", stats.TotalVotesReceived)
	}
	
	t.Log("RecordVoteReceived test completed")
}

// TestElectionManagerRecordVoteCast tests RecordVoteCast.
func TestElectionManagerRecordVoteCast(t *testing.T) {
	em := NewElectionManager()
	
	for i := 0; i < 3; i++ {
		em.RecordVoteCast()
	}
	
	stats := em.GetStats()
	if stats.TotalVotesCast != 3 {
		t.Errorf("Expected 3 votes cast, got %d", stats.TotalVotesCast)
	}
	
	t.Log("RecordVoteCast test completed")
}

// TestElectionManagerRecordElection tests RecordElection.
func TestElectionManagerRecordElection(t *testing.T) {
	em := NewElectionManager()
	
	em.RecordElection(10)
	
	stats := em.GetStats()
	if stats.TotalElections != 1 {
		t.Errorf("Expected 1 election, got %d", stats.TotalElections)
	}
	
	if stats.LastElectionTerm != 10 {
		t.Errorf("Expected term 10, got %d", stats.LastElectionTerm)
	}
	
	if stats.LastElectionTime.IsZero() {
		t.Error("Expected non-zero election time")
	}
	
	t.Log("RecordElection test completed")
}

// TestElectionManagerGetStatus tests GetStatus.
func TestElectionManagerGetStatus(t *testing.T) {
	em := NewElectionManager()
	
	// Set some state
	em.mu.Lock()
	em.currentLeader = "node-1"
	em.currentLeaderAddr = "127.0.0.1:9000"
	em.leaderChangedAt = time.Now()
	em.mu.Unlock()
	
	status := em.GetStatus()
	
	if status["current_leader"] != "node-1" {
		t.Errorf("Expected current_leader=node-1, got %v", status["current_leader"])
	}
	
	if status["leader_address"] != "127.0.0.1:9000" {
		t.Errorf("Expected leader_address=127.0.0.1:9000, got %v", status["leader_address"])
	}
	
	if status["is_leader"] != false {
		t.Error("Expected is_leader=false (no raft node)")
	}
	
	t.Logf("Election status: %v", status)
}

// TestElectionManagerGetStatusWithLeader tests GetStatus with leader info.
func TestElectionManagerGetStatusWithLeader(t *testing.T) {
	em := NewElectionManager()
	
	em.mu.Lock()
	em.currentLeader = "leader-node"
	em.currentLeaderAddr = "192.168.1.10:9000"
	em.leaderChangedAt = time.Now().Add(-1 * time.Hour)
	em.mu.Unlock()
	
	status := em.GetStatus()
	
	if status["current_leader"] != "leader-node" {
		t.Errorf("Expected leader-node, got %v", status["current_leader"])
	}
	
	t.Logf("Leader status: %v", status)
}

// TestElectionManagerSetOnLeaderChange tests SetOnLeaderChange.
func TestElectionManagerSetOnLeaderChange(t *testing.T) {
	em := NewElectionManager()
	
	em.SetOnLeaderChange(func(leader string, addr raft.ServerAddress) {
		t.Logf("Leader changed to: %s (%s)", leader, addr)
	})
	
	if em.onLeaderChange == nil {
		t.Error("Expected callback to be set")
	}
	
	t.Log("SetOnLeaderChange test completed")
}

// TestElectionManagerSetRaftNode tests SetRaftNode.
func TestElectionManagerSetRaftNode(t *testing.T) {
	em := NewElectionManager()
	
	// Set nil raft node
	em.SetRaftNode(nil)
	
	// Should not panic
	t.Log("SetRaftNode completed without panic")
}

// TestElectionManagerGetLeader tests GetLeader.
func TestElectionManagerGetLeader(t *testing.T) {
	em := NewElectionManager()
	
	// Without raft node, should return empty
	leaderID, _ := em.GetLeader()
	
	if leaderID != "" {
		t.Logf("Got leader: %s", leaderID)
	}
	
	t.Log("GetLeader test completed")
}

// TestElectionManagerWaitForLeader tests WaitForLeader.
func TestElectionManagerWaitForLeader(t *testing.T) {
	em := NewElectionManager()
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// Should timeout since no leader
	leaderID, leaderAddr, err := em.WaitForLeader(ctx, 50*time.Millisecond)
	
	if err == nil {
		t.Logf("Got leader: %s (%s)", leaderID, leaderAddr)
	} else {
		t.Logf("WaitForLeader returned: %v", err)
	}
}

// TestElectionManagerWaitForLeaderImmediate tests WaitForLeader with immediate return.
func TestElectionManagerWaitForLeaderImmediate(t *testing.T) {
	// This test requires a real raft node to work properly
	// Without raft node, GetLeader always returns empty
	// Skipping this test as it requires full raft setup
	t.Skip("Skipping - requires full raft node setup")
}

// TestElectionManagerConcurrentAccess tests concurrent access.
func TestElectionManagerConcurrentAccess(t *testing.T) {
	em := NewElectionManager()
	
	done := make(chan bool)
	
	// Spawn multiple goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			em.RecordVoteReceived()
			em.RecordVoteCast()
			_ = em.GetStats()
			done <- true
		}(i)
	}
	
	// Wait for all
	for i := 0; i < 10; i++ {
		<-done
	}
	
	stats := em.GetStats()
	if stats.TotalVotesReceived != 10 {
		t.Errorf("Expected 10 votes received, got %d", stats.TotalVotesReceived)
	}
	
	if stats.TotalVotesCast != 10 {
		t.Errorf("Expected 10 votes cast, got %d", stats.TotalVotesCast)
	}
	
	t.Log("Concurrent access test completed")
}
