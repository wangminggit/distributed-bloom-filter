package raft

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hashicorp/raft"
)

// ReplicationManager manages log replication operations.
type ReplicationManager struct {
	mu sync.RWMutex

	// Reference to the Raft node
	raftNode *raft.Raft

	// Replication statistics
	stats *ReplicationStats

	// Peer information
	peers map[string]*PeerInfo

	// Replication callbacks
	onReplicationComplete func(uint64, error)
}

// PeerInfo holds information about a replication peer.
type PeerInfo struct {
	ID              string
	Address         raft.ServerAddress
	LastContact     time.Time
	LastLogIndex    uint64
	LastLogTerm     uint64
	IsVoter         bool
	IsHealthy       bool
	ReplicationLag  uint64
}

// ReplicationStats holds statistics about replication.
type ReplicationStats struct {
	TotalReplications     int64
	SuccessfulReplications int64
	FailedReplications    int64
	AverageLatency        time.Duration
	LastReplicationTime   time.Time
	BytesReplicated       int64
}

// NewReplicationManager creates a new replication manager.
func NewReplicationManager() *ReplicationManager {
	return &ReplicationManager{
		stats: &ReplicationStats{},
		peers: make(map[string]*PeerInfo),
	}
}

// SetRaftNode sets the Raft node reference.
func (rm *ReplicationManager) SetRaftNode(node *raft.Raft) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.raftNode = node
}

// SetOnReplicationComplete sets the callback for replication completion.
func (rm *ReplicationManager) SetOnReplicationComplete(callback func(uint64, error)) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.onReplicationComplete = callback
}

// GetConfiguration returns the current cluster configuration.
func (rm *ReplicationManager) GetConfiguration() (raft.Configuration, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if rm.raftNode == nil {
		return raft.Configuration{}, ErrRaftNotStarted
	}

	// Get the latest configuration
	configFuture := rm.raftNode.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return raft.Configuration{}, err
	}

	return configFuture.Configuration(), nil
}

// AddPeer adds a new peer to the cluster.
func (rm *ReplicationManager) AddPeer(serverID string, address raft.ServerAddress, voter bool) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.raftNode == nil {
		return ErrRaftNotStarted
	}

	// Add the server to the cluster
	addFuture := rm.raftNode.AddVoter(raft.ServerID(serverID), address, 0, 10*time.Second)
	if err := addFuture.Error(); err != nil {
		rm.stats.FailedReplications++
		return err
	}

	// Update peer information
	rm.peers[serverID] = &PeerInfo{
		ID:          serverID,
		Address:     address,
		IsVoter:     voter,
		IsHealthy:   true,
		LastContact: time.Now(),
	}

	rm.stats.TotalReplications++
	rm.stats.SuccessfulReplications++

	return nil
}

// RemovePeer removes a peer from the cluster.
func (rm *ReplicationManager) RemovePeer(serverID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.raftNode == nil {
		return ErrRaftNotStarted
	}

	// Remove the server from the cluster
	removeFuture := rm.raftNode.RemoveServer(raft.ServerID(serverID), 0, 10*time.Second)
	if err := removeFuture.Error(); err != nil {
		rm.stats.FailedReplications++
		return err
	}

	// Remove peer information
	delete(rm.peers, serverID)

	rm.stats.TotalReplications++
	rm.stats.SuccessfulReplications++

	return nil
}

// GetPeerInfo returns information about a specific peer.
func (rm *ReplicationManager) GetPeerInfo(serverID string) (*PeerInfo, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	peer, ok := rm.peers[serverID]
	if !ok {
		return nil, ErrPeerNotFound
	}

	return peer, nil
}

// GetAllPeers returns information about all peers.
func (rm *ReplicationManager) GetAllPeers() map[string]*PeerInfo {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	peers := make(map[string]*PeerInfo)
	for id, peer := range rm.peers {
		peers[id] = peer
	}

	return peers
}

// GetReplicationLag returns the replication lag for a peer.
func (rm *ReplicationManager) GetReplicationLag(serverID string) (uint64, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	peer, ok := rm.peers[serverID]
	if !ok {
		return 0, ErrPeerNotFound
	}

	if rm.raftNode == nil {
		return 0, ErrRaftNotStarted
	}

	lastIndex := rm.raftNode.LastIndex()
	if lastIndex > peer.LastLogIndex {
		return lastIndex - peer.LastLogIndex, nil
	}

	return 0, nil
}

// WaitForReplication waits for replication to complete up to the given index.
func (rm *ReplicationManager) WaitForReplication(ctx context.Context, index uint64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Until(deadline)):
			return ErrReplicationTimeout
		case <-ticker.C:
			rm.mu.RLock()
			if rm.raftNode == nil {
				rm.mu.RUnlock()
				return ErrRaftNotStarted
			}
			commitIndex := rm.raftNode.LastIndex()
			rm.mu.RUnlock()

			if commitIndex >= index {
				return nil
			}
		}
	}
}

// GetStats returns a copy of the replication statistics.
func (rm *ReplicationManager) GetStats() ReplicationStats {
	if rm == nil || rm.stats == nil {
		return ReplicationStats{}
	}
	rm.mu.RLock()
	stats := *rm.stats
	rm.mu.RUnlock()
	return stats
}

// UpdatePeerContact updates the last contact time for a peer.
func (rm *ReplicationManager) UpdatePeerContact(serverID string, logIndex uint64, logTerm uint64) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if peer, ok := rm.peers[serverID]; ok {
		peer.LastContact = time.Now()
		peer.LastLogIndex = logIndex
		peer.LastLogTerm = logTerm
		peer.IsHealthy = true

		// Calculate replication lag
		if rm.raftNode != nil {
			lastIndex := rm.raftNode.LastIndex()
			if lastIndex > logIndex {
				peer.ReplicationLag = lastIndex - logIndex
			} else {
				peer.ReplicationLag = 0
			}
		}
	}
}

// MarkPeerUnhealthy marks a peer as unhealthy.
func (rm *ReplicationManager) MarkPeerUnhealthy(serverID string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if peer, ok := rm.peers[serverID]; ok {
		peer.IsHealthy = false
	}
}

// GetStatus returns comprehensive replication status.
func (rm *ReplicationManager) GetStatus() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	peers := make([]map[string]interface{}, 0, len(rm.peers))
	for _, peer := range rm.peers {
		peers = append(peers, map[string]interface{}{
			"id":              peer.ID,
			"address":         string(peer.Address),
			"last_contact":    peer.LastContact.String(),
			"last_log_index":  peer.LastLogIndex,
			"last_log_term":   peer.LastLogTerm,
			"is_voter":        peer.IsVoter,
			"is_healthy":      peer.IsHealthy,
			"replication_lag": peer.ReplicationLag,
		})
	}

	return map[string]interface{}{
		"total_replications":      rm.stats.TotalReplications,
		"successful_replications": rm.stats.SuccessfulReplications,
		"failed_replications":     rm.stats.FailedReplications,
		"average_latency":         rm.stats.AverageLatency.String(),
		"peers":                   peers,
	}
}

// Errors for replication management.
var (
	ErrReplicationTimeout = errors.New("replication timeout")
	ErrPeerNotFound       = errors.New("peer not found")
)
