package raft

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hashicorp/raft"
)

// ElectionManager manages leader election operations.
type ElectionManager struct {
	mu sync.RWMutex

	// Reference to the Raft node
	raftNode *raft.Raft

	// Election statistics
	stats *ElectionStats

	// Current leader information
	currentLeader     string
	currentLeaderAddr raft.ServerAddress
	leaderChangedAt   time.Time

	// Election callbacks
	onLeaderChange func(string, raft.ServerAddress)
}

// ElectionStats holds statistics about elections.
type ElectionStats struct {
	TotalElections      int64
	TotalVotesReceived  int64
	TotalVotesCast      int64
	LastElectionTime    time.Time
	LastElectionTerm    uint64
	LeaderChanges       int64
	AverageElectionTime time.Duration
}

// NewElectionManager creates a new election manager.
func NewElectionManager() *ElectionManager {
	return &ElectionManager{
		stats: &ElectionStats{},
	}
}

// SetRaftNode sets the Raft node reference.
func (em *ElectionManager) SetRaftNode(node *raft.Raft) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.raftNode = node
}

// SetOnLeaderChange sets the callback for leader changes.
func (em *ElectionManager) SetOnLeaderChange(callback func(string, raft.ServerAddress)) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.onLeaderChange = callback
}

// IsLeader returns true if this node is the current leader.
func (em *ElectionManager) IsLeader() bool {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if em.raftNode == nil {
		return false
	}

	return em.raftNode.State() == raft.Leader
}

// GetLeader returns the current leader's ID and address.
func (em *ElectionManager) GetLeader() (string, raft.ServerAddress) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if em.raftNode == nil {
		return "", ""
	}

	addr := em.raftNode.Leader()
	return string(addr), addr
}

// WaitForLeader waits for a leader to be elected.
func (em *ElectionManager) WaitForLeader(ctx context.Context, timeout time.Duration) (string, raft.ServerAddress, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-time.After(time.Until(deadline)):
			return "", "", ErrLeaderElectionTimeout
		case <-ticker.C:
			leaderID, leaderAddr := em.GetLeader()
			if leaderID != "" {
				return leaderID, leaderAddr, nil
			}
		}
	}
}

// MonitorLeaderChanges starts monitoring for leader changes.
func (em *ElectionManager) MonitorLeaderChanges(ctx context.Context, checkInterval time.Duration) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	var lastLeader string

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			leaderID, leaderAddr := em.GetLeader()

			if leaderID != lastLeader {
				em.mu.Lock()
				em.currentLeader = leaderID
				em.currentLeaderAddr = leaderAddr
				em.leaderChangedAt = time.Now()
				em.stats.LeaderChanges++

				callback := em.onLeaderChange
				em.mu.Unlock()

				if callback != nil && leaderID != "" {
					callback(leaderID, leaderAddr)
				}

				lastLeader = leaderID
			}
		}
	}
}

// GetLeaderDuration returns how long the current leader has been in power.
func (em *ElectionManager) GetLeaderDuration() time.Duration {
	em.mu.RLock()
	defer em.mu.RUnlock()

	if em.leaderChangedAt.IsZero() {
		return 0
	}

	return time.Since(em.leaderChangedAt)
}

// GetStats returns a copy of the election statistics.
func (em *ElectionManager) GetStats() ElectionStats {
	if em == nil || em.stats == nil {
		return ElectionStats{}
	}
	em.mu.RLock()
	stats := *em.stats
	em.mu.RUnlock()
	return stats
}

// RecordVoteReceived records a vote received during an election.
func (em *ElectionManager) RecordVoteReceived() {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.stats.TotalVotesReceived++
}

// RecordVoteCast records a vote cast during an election.
func (em *ElectionManager) RecordVoteCast() {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.stats.TotalVotesCast++
}

// RecordElection records an election event.
func (em *ElectionManager) RecordElection(term uint64) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.stats.TotalElections++
	em.stats.LastElectionTime = time.Now()
	em.stats.LastElectionTerm = term
}

// GetStatus returns comprehensive election status.
func (em *ElectionManager) GetStatus() map[string]interface{} {
	em.mu.RLock()
	defer em.mu.RUnlock()

	status := map[string]interface{}{
		"is_leader":       em.raftNode != nil && em.raftNode.State() == raft.Leader,
		"current_leader":  em.currentLeader,
		"leader_address":  string(em.currentLeaderAddr),
		"leader_duration": em.leaderChangedAt.String(),
		"total_elections": em.stats.TotalElections,
		"leader_changes":  em.stats.LeaderChanges,
	}

	if em.raftNode != nil {
		status["raft_state"] = em.raftNode.State().String()
		// Note: GetCurrentTerm is not exported in HashiCorp Raft
		status["current_term"] = "unavailable"
	}

	return status
}

// Errors for election management.
var (
	ErrLeaderElectionTimeout = errors.New("leader election timeout")
	ErrNoLeader              = errors.New("no leader elected")
)
