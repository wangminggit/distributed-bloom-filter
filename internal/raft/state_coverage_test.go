package raft

import (
	"testing"

	"github.com/hashicorp/raft"
)

// TestConvertRaftState tests ConvertRaftState function.
func TestConvertRaftState(t *testing.T) {
	tests := []struct {
		name     string
		input    raft.RaftState
		expected NodeState
	}{
		{"Follower", raft.Follower, StateFollower},
		{"Candidate", raft.Candidate, StateCandidate},
		{"Leader", raft.Leader, StateLeader},
		{"Shutdown", raft.Shutdown, StateShutdown},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertRaftState(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestConvertRaftStateUnknown tests ConvertRaftState with unknown state.
func TestConvertRaftStateUnknown(t *testing.T) {
	// Default case - use an invalid RaftState value
	result := ConvertRaftState(raft.RaftState(999))
	
	if result != StateFollower {
		t.Errorf("Expected StateFollower for unknown state, got %v", result)
	}
	
	t.Log("ConvertRaftState correctly handled unknown state")
}

// TestNodeStateString tests NodeState.String method.
func TestNodeStateStringCoverage(t *testing.T) {
	tests := []struct {
		state    NodeState
		expected string
	}{
		{StateFollower, "Follower"},
		{StateCandidate, "Candidate"},
		{StateLeader, "Leader"},
		{StateShutdown, "Shutdown"},
		{NodeState(999), "Unknown"},
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
