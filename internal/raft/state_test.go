package raft

import (
	"testing"
	"time"
)

// TestStateManagerBasic tests basic state manager operations.
func TestStateManagerBasic(t *testing.T) {
	sm := NewStateManager()
	
	if sm == nil {
		t.Fatal("Expected non-nil state manager")
	}
	
	if sm.state != StateFollower {
		t.Errorf("Expected initial state Follower, got %v", sm.state)
	}
}

// TestStateManagerGetState tests GetState.
func TestStateManagerGetState(t *testing.T) {
	sm := NewStateManager()
	
	state := sm.GetState()
	
	if state != StateFollower {
		t.Errorf("Expected Follower state, got %v", state)
	}
}

// TestStateManagerSetState tests SetState and state transitions.
func TestStateManagerSetState(t *testing.T) {
	sm := NewStateManager()
	
	// Transition to Candidate
	sm.SetState(StateCandidate)
	if sm.GetState() != StateCandidate {
		t.Error("Expected Candidate state")
	}
	
	// Transition to Leader
	sm.SetState(StateLeader)
	if sm.GetState() != StateLeader {
		t.Error("Expected Leader state")
	}
	
	// Back to Follower
	sm.SetState(StateFollower)
	if sm.GetState() != StateFollower {
		t.Error("Expected Follower state")
	}
}

// TestStateManagerGetStateDuration tests GetStateDuration.
func TestStateManagerGetStateDuration(t *testing.T) {
	sm := NewStateManager()
	
	// Initial duration should be very small
	duration := sm.GetStateDuration()
	if duration < 0 {
		t.Errorf("Expected non-negative duration, got %v", duration)
	}
	
	// Wait a bit and check again
	time.Sleep(10 * time.Millisecond)
	duration2 := sm.GetStateDuration()
	
	if duration2 < duration {
		t.Error("Duration should increase over time")
	}
	
	t.Logf("State duration: %v", duration2)
}

// TestStateManagerGetCurrentTerm tests term management.
func TestStateManagerGetCurrentTerm(t *testing.T) {
	sm := NewStateManager()
	
	// Initial term should be 0
	term := sm.GetCurrentTerm()
	if term != 0 {
		t.Errorf("Expected initial term 0, got %d", term)
	}
	
	// Set term
	sm.SetCurrentTerm(5)
	term = sm.GetCurrentTerm()
	if term != 5 {
		t.Errorf("Expected term 5, got %d", term)
	}
	
	// Increment term
	sm.SetCurrentTerm(10)
	term = sm.GetCurrentTerm()
	if term != 10 {
		t.Errorf("Expected term 10, got %d", term)
	}
}

// TestStateManagerGetVotedFor tests voted for management.
func TestStateManagerGetVotedFor(t *testing.T) {
	sm := NewStateManager()
	
	// Initial votedFor should be empty
	votedFor := sm.GetVotedFor()
	if votedFor != "" {
		t.Errorf("Expected empty votedFor, got %s", votedFor)
	}
	
	// Set votedFor
	sm.SetVotedFor("node-1")
	votedFor = sm.GetVotedFor()
	if votedFor != "node-1" {
		t.Errorf("Expected votedFor=node-1, got %s", votedFor)
	}
	
	// Change votedFor
	sm.SetVotedFor("node-2")
	votedFor = sm.GetVotedFor()
	if votedFor != "node-2" {
		t.Errorf("Expected votedFor=node-2, got %s", votedFor)
	}
}

// TestStateManagerGetCommitIndex tests commit index management.
func TestStateManagerGetCommitIndex(t *testing.T) {
	sm := NewStateManager()
	
	// Initial commit index should be 0
	commitIndex := sm.GetCommitIndex()
	if commitIndex != 0 {
		t.Errorf("Expected initial commitIndex 0, got %d", commitIndex)
	}
	
	// Set commit index
	sm.SetCommitIndex(100)
	commitIndex = sm.GetCommitIndex()
	if commitIndex != 100 {
		t.Errorf("Expected commitIndex 100, got %d", commitIndex)
	}
}

// TestStateManagerGetLastApplied tests last applied management.
func TestStateManagerGetLastApplied(t *testing.T) {
	sm := NewStateManager()
	
	// Initial last applied should be 0
	lastApplied := sm.GetLastApplied()
	if lastApplied != 0 {
		t.Errorf("Expected initial lastApplied 0, got %d", lastApplied)
	}
	
	// Set last applied
	sm.SetLastApplied(50)
	lastApplied = sm.GetLastApplied()
	if lastApplied != 50 {
		t.Errorf("Expected lastApplied 50, got %d", lastApplied)
	}
}

// TestStateManagerGetLastSnapshotIndex tests last snapshot index management.
func TestStateManagerGetLastSnapshotIndex(t *testing.T) {
	sm := NewStateManager()
	
	// Initial values should be 0
	if sm.GetLastSnapshotIndex() != 0 {
		t.Error("Expected initial lastSnapshotIndex 0")
	}
	
	if sm.GetLastSnapshotTerm() != 0 {
		t.Error("Expected initial lastSnapshotTerm 0")
	}
	
	// Set values
	sm.SetLastSnapshotIndex(100)
	sm.SetLastSnapshotTerm(5)
	
	if sm.GetLastSnapshotIndex() != 100 {
		t.Errorf("Expected lastSnapshotIndex 100, got %d", sm.GetLastSnapshotIndex())
	}
	
	if sm.GetLastSnapshotTerm() != 5 {
		t.Errorf("Expected lastSnapshotTerm 5, got %d", sm.GetLastSnapshotTerm())
	}
}

// TestStateManagerGetStatus tests GetStatus.
func TestStateManagerGetStatus(t *testing.T) {
	sm := NewStateManager()
	
	// Set some state
	sm.SetState(StateLeader)
	sm.SetCurrentTerm(3)
	sm.SetVotedFor("node-1")
	sm.SetCommitIndex(100)
	sm.SetLastApplied(95)
	sm.SetLastSnapshotIndex(50)
	sm.SetLastSnapshotTerm(2)
	
	status := sm.GetStatus()
	
	// Verify status contains expected fields
	if status["state"] == nil {
		t.Error("Expected state in status")
	}
	
	if status["current_term"] == nil {
		t.Error("Expected current_term in status")
	}
	
	if status["voted_for"] == nil {
		t.Error("Expected voted_for in status")
	}
	
	if status["commit_index"] == nil {
		t.Error("Expected commit_index in status")
	}
	
	t.Logf("State manager status: %v", status)
}

// TestNodeStateString tests NodeState.String method.
func TestNodeStateString(t *testing.T) {
	tests := []struct {
		state    NodeState
		expected string
	}{
		{StateFollower, "Follower"},
		{StateCandidate, "Candidate"},
		{StateLeader, "Leader"},
		{StateShutdown, "Shutdown"},
		{NodeState(999), "Unknown"}, // Unknown state
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.state.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestStateManagerConcurrentStateAccess tests concurrent state access.
func TestStateManagerConcurrentStateAccess(t *testing.T) {
	sm := NewStateManager()
	
	done := make(chan bool)
	
	// Spawn multiple goroutines
	for i := 0; i < 10; i++ {
		go func(id int) {
			sm.SetState(StateLeader)
			sm.SetCurrentTerm(uint64(id))
			_ = sm.GetState()
			_ = sm.GetCurrentTerm()
			done <- true
		}(i)
	}
	
	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
	
	t.Log("Concurrent state access test completed")
}

// TestStateManagerStateTransitions tests valid state transitions.
func TestStateManagerStateTransitions(t *testing.T) {
	sm := NewStateManager()
	
	// Follower -> Candidate
	sm.SetState(StateFollower)
	sm.SetState(StateCandidate)
	if sm.GetState() != StateCandidate {
		t.Error("Follower -> Candidate transition failed")
	}
	
	// Candidate -> Leader
	sm.SetState(StateLeader)
	if sm.GetState() != StateLeader {
		t.Error("Candidate -> Leader transition failed")
	}
	
	// Leader -> Follower (e.g., lost election)
	sm.SetState(StateFollower)
	if sm.GetState() != StateFollower {
		t.Error("Leader -> Follower transition failed")
	}
	
	// Follower -> Shutdown
	sm.SetState(StateShutdown)
	if sm.GetState() != StateShutdown {
		t.Error("Follower -> Shutdown transition failed")
	}
	
	t.Log("All state transitions completed successfully")
}
