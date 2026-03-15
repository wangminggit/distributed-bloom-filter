package raft

import (
	"errors"
	"sync"
	"time"

	"github.com/hashicorp/raft"
)

// NodeState represents the current state of a Raft node.
type NodeState int

const (
	// StateFollower indicates the node is a follower.
	StateFollower NodeState = iota
	// StateCandidate indicates the node is a candidate in an election.
	StateCandidate
	// StateLeader indicates the node is the cluster leader.
	StateLeader
	// StateShutdown indicates the node is shutting down.
	StateShutdown
)

// String returns a string representation of the node state.
func (s NodeState) String() string {
	switch s {
	case StateFollower:
		return "Follower"
	case StateCandidate:
		return "Candidate"
	case StateLeader:
		return "Leader"
	case StateShutdown:
		return "Shutdown"
	default:
		return "Unknown"
	}
}

// StateManager manages the state of a Raft node.
type StateManager struct {
	mu sync.RWMutex

	// Current state
	state NodeState

	// State transition timestamp
	stateChangedAt time.Time

	// Term number (from Raft)
	currentTerm uint64

	// Voted for in current term
	votedFor string

	// Commit index
	commitIndex uint64

	// Last applied index
	lastApplied uint64

	// Log index of the last snapshot
	lastSnapshotIndex uint64

	// Log term of the last snapshot
	lastSnapshotTerm uint64
}

// NewStateManager creates a new state manager.
func NewStateManager() *StateManager {
	return &StateManager{
		state:            StateFollower,
		stateChangedAt:   time.Now(),
		currentTerm:      0,
		votedFor:         "",
		commitIndex:      0,
		lastApplied:      0,
		lastSnapshotIndex: 0,
		lastSnapshotTerm:  0,
	}
}

// GetState returns the current state.
func (sm *StateManager) GetState() NodeState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

// SetState sets the current state.
func (sm *StateManager) SetState(state NodeState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.state = state
	sm.stateChangedAt = time.Now()
}

// GetStateDuration returns how long the node has been in the current state.
func (sm *StateManager) GetStateDuration() time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return time.Since(sm.stateChangedAt)
}

// GetCurrentTerm returns the current term.
func (sm *StateManager) GetCurrentTerm() uint64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentTerm
}

// SetCurrentTerm sets the current term.
func (sm *StateManager) SetCurrentTerm(term uint64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.currentTerm = term
}

// GetVotedFor returns who this node voted for in the current term.
func (sm *StateManager) GetVotedFor() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.votedFor
}

// SetVotedFor sets who this node voted for.
func (sm *StateManager) SetVotedFor(candidateID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.votedFor = candidateID
}

// GetCommitIndex returns the commit index.
func (sm *StateManager) GetCommitIndex() uint64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.commitIndex
}

// SetCommitIndex sets the commit index.
func (sm *StateManager) SetCommitIndex(index uint64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.commitIndex = index
}

// GetLastApplied returns the last applied index.
func (sm *StateManager) GetLastApplied() uint64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastApplied
}

// SetLastApplied sets the last applied index.
func (sm *StateManager) SetLastApplied(index uint64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.lastApplied = index
}

// GetLastSnapshotIndex returns the last snapshot index.
func (sm *StateManager) GetLastSnapshotIndex() uint64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastSnapshotIndex
}

// SetLastSnapshotIndex sets the last snapshot index.
func (sm *StateManager) SetLastSnapshotIndex(index uint64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.lastSnapshotIndex = index
}

// GetLastSnapshotTerm returns the last snapshot term.
func (sm *StateManager) GetLastSnapshotTerm() uint64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastSnapshotTerm
}

// SetLastSnapshotTerm sets the last snapshot term.
func (sm *StateManager) SetLastSnapshotTerm(term uint64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.lastSnapshotTerm = term
}

// GetStatus returns a comprehensive status snapshot.
func (sm *StateManager) GetStatus() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return map[string]interface{}{
		"state":             sm.state.String(),
		"state_duration":    time.Since(sm.stateChangedAt).String(),
		"current_term":      sm.currentTerm,
		"voted_for":         sm.votedFor,
		"commit_index":      sm.commitIndex,
		"last_applied":      sm.lastApplied,
		"last_snapshot_index": sm.lastSnapshotIndex,
		"last_snapshot_term":  sm.lastSnapshotTerm,
	}
}

// ConvertRaftState converts a HashiCorp Raft state to our NodeState.
func ConvertRaftState(state raft.RaftState) NodeState {
	switch state {
	case raft.Follower:
		return StateFollower
	case raft.Candidate:
		return StateCandidate
	case raft.Leader:
		return StateLeader
	case raft.Shutdown:
		return StateShutdown
	default:
		return StateFollower
	}
}

// Errors for state management.
var (
	ErrInvalidStateTransition = errors.New("invalid state transition")
	ErrNotLeader              = errors.New("node is not the leader")
)
