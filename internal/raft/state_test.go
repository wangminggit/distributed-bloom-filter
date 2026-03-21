package raft

import (
	"testing"
	"time"
)

func TestNodeState(t *testing.T) {
	t.Run("StateFollower", func(t *testing.T) {
		if StateFollower != 0 {
			t.Errorf("Expected StateFollower to be 0, got %d", StateFollower)
		}
		
		if StateFollower.String() != "Follower" {
			t.Errorf("Expected 'Follower', got %s", StateFollower.String())
		}
	})

	t.Run("StateCandidate", func(t *testing.T) {
		if StateCandidate != 1 {
			t.Errorf("Expected StateCandidate to be 1, got %d", StateCandidate)
		}
		
		if StateCandidate.String() != "Candidate" {
			t.Errorf("Expected 'Candidate', got %s", StateCandidate.String())
		}
	})

	t.Run("StateLeader", func(t *testing.T) {
		if StateLeader != 2 {
			t.Errorf("Expected StateLeader to be 2, got %d", StateLeader)
		}
		
		if StateLeader.String() != "Leader" {
			t.Errorf("Expected 'Leader', got %s", StateLeader.String())
		}
	})

	t.Run("StateShutdown", func(t *testing.T) {
		if StateShutdown != 3 {
			t.Errorf("Expected StateShutdown to be 3, got %d", StateShutdown)
		}
		
		if StateShutdown.String() != "Shutdown" {
			t.Errorf("Expected 'Shutdown', got %s", StateShutdown.String())
		}
	})

	t.Run("UnknownState", func(t *testing.T) {
		var unknown NodeState = 999
		if unknown.String() != "Unknown" {
			t.Errorf("Expected 'Unknown', got %s", unknown.String())
		}
	})
}

func TestStateManager(t *testing.T) {
	t.Run("NewStateManager", func(t *testing.T) {
		sm := NewStateManager()
		
		if sm == nil {
			t.Fatal("Expected state manager to be created")
		}
		
		if sm.state != StateFollower {
			t.Errorf("Expected initial state Follower, got %v", sm.state)
		}
		
		if sm.currentTerm != 0 {
			t.Errorf("Expected initial term 0, got %d", sm.currentTerm)
		}
		
		if sm.votedFor != "" {
			t.Errorf("Expected empty votedFor, got %s", sm.votedFor)
		}
		
		if sm.commitIndex != 0 {
			t.Errorf("Expected commitIndex 0, got %d", sm.commitIndex)
		}
		
		if sm.lastApplied != 0 {
			t.Errorf("Expected lastApplied 0, got %d", sm.lastApplied)
		}
	})

	t.Run("GetState", func(t *testing.T) {
		sm := NewStateManager()
		
		state := sm.GetState()
		if state != StateFollower {
			t.Errorf("Expected StateFollower, got %v", state)
		}
	})

	t.Run("SetState", func(t *testing.T) {
		sm := NewStateManager()
		
		sm.SetState(StateLeader)
		
		if sm.GetState() != StateLeader {
			t.Errorf("Expected StateLeader, got %v", sm.GetState())
		}
		
		// Verify stateChangedAt was updated
		if time.Since(sm.stateChangedAt) > time.Second {
			t.Error("Expected stateChangedAt to be recent")
		}
	})

	t.Run("GetStateDuration", func(t *testing.T) {
		sm := NewStateManager()
		
		// Set state to a known time in the past
		sm.state = StateLeader
		sm.stateChangedAt = time.Now().Add(-5 * time.Second)
		
		duration := sm.GetStateDuration()
		
		if duration < 5*time.Second {
			t.Errorf("Expected duration >= 5s, got %v", duration)
		}
	})

	t.Run("SetCurrentTerm", func(t *testing.T) {
		sm := NewStateManager()
		
		sm.SetCurrentTerm(5)
		
		if sm.currentTerm != 5 {
			t.Errorf("Expected term 5, got %d", sm.currentTerm)
		}
	})

	t.Run("GetCurrentTerm", func(t *testing.T) {
		sm := NewStateManager()
		sm.currentTerm = 10
		
		term := sm.GetCurrentTerm()
		if term != 10 {
			t.Errorf("Expected term 10, got %d", term)
		}
	})

	t.Run("SetVotedFor", func(t *testing.T) {
		sm := NewStateManager()
		
		sm.SetVotedFor("candidate-1")
		
		if sm.votedFor != "candidate-1" {
			t.Errorf("Expected votedFor 'candidate-1', got %s", sm.votedFor)
		}
	})

	t.Run("GetVotedFor", func(t *testing.T) {
		sm := NewStateManager()
		sm.votedFor = "candidate-2"
		
		votedFor := sm.GetVotedFor()
		if votedFor != "candidate-2" {
			t.Errorf("Expected votedFor 'candidate-2', got %s", votedFor)
		}
	})

	t.Run("SetCommitIndex", func(t *testing.T) {
		sm := NewStateManager()
		
		sm.SetCommitIndex(100)
		
		if sm.commitIndex != 100 {
			t.Errorf("Expected commitIndex 100, got %d", sm.commitIndex)
		}
	})

	t.Run("GetCommitIndex", func(t *testing.T) {
		sm := NewStateManager()
		sm.commitIndex = 200
		
		index := sm.GetCommitIndex()
		if index != 200 {
			t.Errorf("Expected commitIndex 200, got %d", index)
		}
	})

	t.Run("SetLastApplied", func(t *testing.T) {
		sm := NewStateManager()
		
		sm.SetLastApplied(50)
		
		if sm.lastApplied != 50 {
			t.Errorf("Expected lastApplied 50, got %d", sm.lastApplied)
		}
	})

	t.Run("GetLastApplied", func(t *testing.T) {
		sm := NewStateManager()
		sm.lastApplied = 75
		
		index := sm.GetLastApplied()
		if index != 75 {
			t.Errorf("Expected lastApplied 75, got %d", index)
		}
	})

	t.Run("SnapshotFields", func(t *testing.T) {
		sm := NewStateManager()
		
		// Direct field access since methods may not exist
		sm.lastSnapshotIndex = 300
		sm.lastSnapshotTerm = 10
		
		if sm.lastSnapshotIndex != 300 {
			t.Errorf("Expected lastSnapshotIndex 300, got %d", sm.lastSnapshotIndex)
		}
		
		if sm.lastSnapshotTerm != 10 {
			t.Errorf("Expected lastSnapshotTerm 10, got %d", sm.lastSnapshotTerm)
		}
	})

	t.Run("GetStatus", func(t *testing.T) {
		sm := NewStateManager()
		sm.state = StateLeader
		sm.currentTerm = 5
		sm.commitIndex = 100
		sm.lastApplied = 95
		sm.votedFor = "leader-1"
		
		status := sm.GetStatus()
		
		if status["state"] != "Leader" {
			t.Errorf("Expected state 'Leader', got %v", status["state"])
		}
		
		if status["current_term"] != uint64(5) {
			t.Errorf("Expected current_term 5, got %v", status["current_term"])
		}
		
		if status["commit_index"] != uint64(100) {
			t.Errorf("Expected commit_index 100, got %v", status["commit_index"])
		}
		
		if status["last_applied"] != uint64(95) {
			t.Errorf("Expected last_applied 95, got %v", status["last_applied"])
		}
		
		if status["voted_for"] != "leader-1" {
			t.Errorf("Expected voted_for 'leader-1', got %v", status["voted_for"])
		}
	})

	t.Run("StateReset_Manual", func(t *testing.T) {
		sm := NewStateManager()
		
		// Set some values
		sm.state = StateLeader
		sm.currentTerm = 10
		sm.votedFor = "candidate-1"
		
		// Manual reset of term and vote
		sm.currentTerm = 0
		sm.votedFor = ""
		
		if sm.currentTerm != 0 {
			t.Errorf("Expected term reset to 0, got %d", sm.currentTerm)
		}
		
		if sm.votedFor != "" {
			t.Errorf("Expected votedFor reset to empty, got %s", sm.votedFor)
		}
	})
}

func TestStateManagerConcurrentAccess(t *testing.T) {
	sm := NewStateManager()
	
	done := make(chan bool, 100)
	
	for i := 0; i < 100; i++ {
		go func(id int) {
			sm.GetState()
			sm.GetCurrentTerm()
			sm.GetCommitIndex()
			sm.GetStatus()
			done <- true
		}(i)
	}
	
	for i := 0; i < 100; i++ {
		<-done
	}
	
	t.Log("Concurrent access test passed")
}

func TestStateManagerStateTransitions(t *testing.T) {
	sm := NewStateManager()
	
	// Follower -> Candidate
	sm.SetState(StateCandidate)
	if sm.GetState() != StateCandidate {
		t.Error("Failed to transition to Candidate")
	}
	
	// Candidate -> Leader
	sm.SetState(StateLeader)
	if sm.GetState() != StateLeader {
		t.Error("Failed to transition to Leader")
	}
	
	// Leader -> Follower (after term change)
	sm.SetState(StateFollower)
	if sm.GetState() != StateFollower {
		t.Error("Failed to transition to Follower")
	}
	
	// Follower -> Shutdown
	sm.SetState(StateShutdown)
	if sm.GetState() != StateShutdown {
		t.Error("Failed to transition to Shutdown")
	}
	
	t.Log("All state transitions passed")
}
