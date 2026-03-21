package raft

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/raft"
)

func TestElectionManager(t *testing.T) {
	t.Run("NewElectionManager", func(t *testing.T) {
		em := NewElectionManager()
		
		if em == nil {
			t.Fatal("Expected election manager to be created")
		}
		
		if em.stats == nil {
			t.Error("Expected stats to be initialized")
		}
		
		if em.currentLeader != "" {
			t.Error("Expected current leader to be empty initially")
		}
	})

	t.Run("SetRaftNode", func(t *testing.T) {
		em := NewElectionManager()
		
		// Mock raft node (we can't create a real one without full setup)
		em.SetRaftNode(nil)
		
		// Should not panic
		if em.raftNode != nil {
			t.Error("Expected raft node to be nil")
		}
	})

	t.Run("SetOnLeaderChange", func(t *testing.T) {
		em := NewElectionManager()
		
		called := false
		callback := func(leader string, addr raft.ServerAddress) {
			called = true
		}
		
		em.SetOnLeaderChange(callback)
		
		if em.onLeaderChange == nil {
			t.Error("Expected callback to be set")
		}
		
		// Trigger callback manually to test
		if em.onLeaderChange != nil {
			em.onLeaderChange("test-leader", "localhost:7000")
		}
		
		if !called {
			t.Error("Expected callback to be invoked")
		}
	})

	t.Run("IsLeader_WithoutRaftNode", func(t *testing.T) {
		em := NewElectionManager()
		
		if em.IsLeader() {
			t.Error("Expected false when raft node is not set")
		}
	})

	t.Run("GetLeader_WithoutRaftNode", func(t *testing.T) {
		em := NewElectionManager()
		
		leader, addr := em.GetLeader()
		
		if leader != "" {
			t.Errorf("Expected empty leader, got %s", leader)
		}
		
		if addr != "" {
			t.Errorf("Expected empty address, got %s", addr)
		}
	})

	t.Run("WaitForLeader_Timeout", func(t *testing.T) {
		em := NewElectionManager()
		
		ctx := context.Background()
		leader, addr, err := em.WaitForLeader(ctx, 100*time.Millisecond)
		
		if err == nil {
			t.Error("Expected timeout error")
		}
		
		if leader != "" {
			t.Errorf("Expected empty leader on timeout, got %s", leader)
		}
		
		if addr != "" {
			t.Errorf("Expected empty address on timeout, got %s", addr)
		}
	})

	t.Run("WaitForLeader_ContextCancelled", func(t *testing.T) {
		em := NewElectionManager()
		
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		leader, addr, err := em.WaitForLeader(ctx, 5*time.Second)
		
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled error, got %v", err)
		}
		
		if leader != "" {
			t.Errorf("Expected empty leader, got %s", leader)
		}
		
		if addr != "" {
			t.Errorf("Expected empty address, got %s", addr)
		}
	})

	t.Run("GetLeaderDuration", func(t *testing.T) {
		em := NewElectionManager()
		
		// Test with zero time (no leader set)
		duration := em.GetLeaderDuration()
		
		if duration < 0 {
			t.Errorf("Expected non-negative duration, got %v", duration)
		}
		
		// Set leader changed time
		em.leaderChangedAt = time.Now().Add(-2 * time.Second)
		duration = em.GetLeaderDuration()
		
		if duration < 2*time.Second {
			t.Errorf("Expected duration >= 2s, got %v", duration)
		}
	})

	t.Run("GetStats", func(t *testing.T) {
		em := NewElectionManager()
		
		// Record some stats
		em.stats.TotalElections = 5
		em.stats.TotalVotesReceived = 10
		em.stats.TotalVotesCast = 8
		em.stats.LeaderChanges = 2
		em.stats.LastElectionTerm = 3
		
		stats := em.GetStats()
		
		if stats.TotalElections != 5 {
			t.Errorf("Expected 5 total elections, got %d", stats.TotalElections)
		}
		
		if stats.TotalVotesReceived != 10 {
			t.Errorf("Expected 10 votes received, got %d", stats.TotalVotesReceived)
		}
		
		if stats.LeaderChanges != 2 {
			t.Errorf("Expected 2 leader changes, got %d", stats.LeaderChanges)
		}
	})

	t.Run("RecordVoteReceived", func(t *testing.T) {
		em := NewElectionManager()
		
		em.RecordVoteReceived()
		
		if em.stats.TotalVotesReceived != 1 {
			t.Errorf("Expected 1 vote received, got %d", em.stats.TotalVotesReceived)
		}
	})

	t.Run("RecordVoteCast", func(t *testing.T) {
		em := NewElectionManager()
		
		em.RecordVoteCast()
		
		if em.stats.TotalVotesCast != 1 {
			t.Errorf("Expected 1 vote cast, got %d", em.stats.TotalVotesCast)
		}
	})

	t.Run("RecordElection", func(t *testing.T) {
		em := NewElectionManager()
		
		em.RecordElection(2)
		
		if em.stats.TotalElections != 1 {
			t.Errorf("Expected 1 election, got %d", em.stats.TotalElections)
		}
		
		if em.stats.LastElectionTerm != 2 {
			t.Errorf("Expected term 2, got %d", em.stats.LastElectionTerm)
		}
	})

	t.Run("GetStatus", func(t *testing.T) {
		em := NewElectionManager()
		
		em.currentLeader = "leader-1"
		em.currentLeaderAddr = "localhost:7000"
		em.leaderChangedAt = time.Now().Add(-1 * time.Hour)
		em.stats.TotalElections = 3
		em.stats.LeaderChanges = 2
		
		status := em.GetStatus()
		
		if status["current_leader"] != "leader-1" {
			t.Errorf("Expected current_leader 'leader-1', got %v", status["current_leader"])
		}
		
		if status["leader_address"] != "localhost:7000" {
			t.Errorf("Expected leader_address 'localhost:7000', got %v", status["leader_address"])
		}
		
		if status["total_elections"] != int64(3) {
			t.Errorf("Expected 3 elections, got %v", status["total_elections"])
		}
		
		if status["leader_changes"] != int64(2) {
			t.Errorf("Expected 2 leader changes, got %v", status["leader_changes"])
		}
	})
}

func TestElectionStats(t *testing.T) {
	t.Run("InitialStats", func(t *testing.T) {
		stats := &ElectionStats{}
		
		if stats.TotalElections != 0 {
			t.Errorf("Expected 0 elections, got %d", stats.TotalElections)
		}
		
		if stats.LeaderChanges != 0 {
			t.Errorf("Expected 0 leader changes, got %d", stats.LeaderChanges)
		}
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		em := NewElectionManager()
		
		done := make(chan bool, 100)
		
		for i := 0; i < 100; i++ {
			go func(id int) {
				em.RecordVoteReceived()
				em.RecordVoteCast()
				em.GetStats()
				done <- true
			}(i)
		}
		
		for i := 0; i < 100; i++ {
			<-done
		}
		
		stats := em.GetStats()
		if stats.TotalVotesReceived != 100 {
			t.Errorf("Expected 100 votes received, got %d", stats.TotalVotesReceived)
		}
		if stats.TotalVotesCast != 100 {
			t.Errorf("Expected 100 votes cast, got %d", stats.TotalVotesCast)
		}
	})
}
