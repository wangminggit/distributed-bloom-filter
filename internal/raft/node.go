package raft

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"

	"github.com/wangminggit/distributed-bloom-filter/internal/metadata"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

// Node represents a Raft consensus node for the distributed Bloom filter.
// It implements the raft.FSM interface for state machine replication.
type Node struct {
	nodeID          string
	raftPort        int
	dataDir         string
	bloomFilter     *bloom.CountingBloomFilter
	walEncryptor    *wal.WALEncryptor
	metadataService *metadata.Service

	raftNode  *raft.Raft
	raftStore *raftboltdb.BoltStore
	transport *raft.NetworkTransport

	mu sync.RWMutex
}

// Command represents a Raft command.
type Command struct {
	Command string `json:"command"`
	Item    []byte `json:"item"`
}

// NewNode creates a new Raft node.
func NewNode(nodeID string, raftPort int, dataDir string,
	bloomFilter *bloom.CountingBloomFilter,
	walEncryptor *wal.WALEncryptor,
	metadataService *metadata.Service) *Node {

	return &Node{
		nodeID:          nodeID,
		raftPort:        raftPort,
		dataDir:         dataDir,
		bloomFilter:     bloomFilter,
		walEncryptor:    walEncryptor,
		metadataService: metadataService,
	}
}

// Start initializes and starts the Raft node.
func (n *Node) Start(bootstrap bool) error {
	return n.StartWithPeers(bootstrap, nil)
}

// StartWithPeers initializes and starts the Raft node with optional peer addresses.
// If bootstrap is true, this node bootstraps the cluster.
// If bootstrap is false and peers are provided, this node joins the existing cluster.
func (n *Node) StartWithPeers(bootstrap bool, peers []string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Create Raft configuration with optimized timeouts for fast failover
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(n.nodeID)
	
	// Optimized Raft configuration for fast failover (<500ms target)
	config.ElectionTimeout = 250 * time.Millisecond      // Very fast election
	config.HeartbeatTimeout = 125 * time.Millisecond     // Fast heartbeat
	config.LeaderLeaseTimeout = 125 * time.Millisecond   // Fast lease
	config.CommitTimeout = 25 * time.Millisecond         // Very fast commit
	config.SnapshotInterval = 60 * time.Second           // Snapshot every 60s
	config.SnapshotThreshold = 1024                      // Snapshot after 1024 ops

	// Create data directory for Raft
	raftDir := filepath.Join(n.dataDir, "raft")
	if err := os.MkdirAll(raftDir, 0755); err != nil {
		return fmt.Errorf("failed to create raft directory: %w", err)
	}

	// Create BoltDB store for Raft logs and stable storage
	boltStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "raft.db"))
	if err != nil {
		return fmt.Errorf("failed to create bolt store: %w", err)
	}
	n.raftStore = boltStore

	// Create log store and stable store
	logStore := boltStore
	stableStore := boltStore

	// Create snapshot store
	snapshotStore, err := raft.NewFileSnapshotStore(raftDir, 3, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create snapshot store: %w", err)
	}

	// Create transport - use localhost for local binding
	bindAddr := fmt.Sprintf("127.0.0.1:%d", n.raftPort)
	tcpAddr, err := net.ResolveTCPAddr("tcp", bindAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve TCP address: %w", err)
	}
	transport, err := raft.NewTCPTransport(bindAddr, tcpAddr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}
	n.transport = transport

	// Create Raft instance (pass n as the FSM)
	ra, err := raft.NewRaft(config, n, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return fmt.Errorf("failed to create raft: %w", err)
	}
	n.raftNode = ra

	// Bootstrap if this is the first node
	if bootstrap {
		advertiseAddr := fmt.Sprintf("127.0.0.1:%d", n.raftPort)
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: raft.ServerAddress(advertiseAddr),
				},
			},
		}

		future := ra.BootstrapCluster(configuration)
		if err := future.Error(); err != nil {
			return fmt.Errorf("failed to bootstrap cluster: %w", err)
		}

		log.Printf("Bootstrapped Raft cluster with node %s", n.nodeID)
	} else if len(peers) > 0 {
		// Join existing cluster - nodes will be added by the leader
		// The leader should call AddVoter for each peer
		log.Printf("Node %s starting to join cluster with peers: %v", n.nodeID, peers)
	}

	log.Printf("Raft node %s started on port %d (election_timeout=%v, heartbeat=%v)", 
		n.nodeID, n.raftPort, config.ElectionTimeout, config.HeartbeatTimeout)
	return nil
}

// JoinCluster adds this node to an existing Raft cluster.
// This should be called by the leader to add a new voter.
func (n *Node) JoinCluster(leaderAddr string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.raftNode == nil {
		return fmt.Errorf("raft node not started")
	}

	advertiseAddr := fmt.Sprintf("127.0.0.1:%d", n.raftPort)
	
	// Use Raft's AddVoter API to add this node as a voter
	future := n.raftNode.AddVoter(
		raft.ServerID(n.nodeID),
		raft.ServerAddress(advertiseAddr),
		0,      // prevIndex: 0 means any
		10*time.Second, // timeout
	)
	
	if err := future.Error(); err != nil {
		// Check if node is already in the cluster
		if err.Error() == "already in the cluster" {
			log.Printf("Node %s is already a cluster member", n.nodeID)
			return nil
		}
		return fmt.Errorf("failed to add voter: %w", err)
	}

	log.Printf("Node %s successfully joined cluster at %s", n.nodeID, advertiseAddr)
	return nil
}

// AddPeer adds a new peer to the Raft cluster (called by leader).
func (n *Node) AddPeer(peerID, peerAddr string) error {
	if n.raftNode == nil {
		return fmt.Errorf("raft node not started")
	}

	if !n.IsLeader() {
		return fmt.Errorf("only leader can add peers")
	}

	future := n.raftNode.AddVoter(
		raft.ServerID(peerID),
		raft.ServerAddress(peerAddr),
		0,
		10*time.Second,
	)

	if err := future.Error(); err != nil {
		if err.Error() == "already in the cluster" {
			log.Printf("Peer %s is already a cluster member", peerID)
			return nil
		}
		return fmt.Errorf("failed to add peer %s: %w", peerID, err)
	}

	log.Printf("Added peer %s at %s to cluster", peerID, peerAddr)
	return nil
}

// RemovePeer removes a peer from the Raft cluster (called by leader).
func (n *Node) RemovePeer(peerID string) error {
	if n.raftNode == nil {
		return fmt.Errorf("raft node not started")
	}

	if !n.IsLeader() {
		return fmt.Errorf("only leader can remove peers")
	}

	future := n.raftNode.RemoveServer(
		raft.ServerID(peerID),
		0,
		10*time.Second,
	)

	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to remove peer %s: %w", peerID, err)
	}

	log.Printf("Removed peer %s from cluster", peerID)
	return nil
}

// GetClusterMembers returns the current cluster configuration.
func (n *Node) GetClusterMembers() ([]raft.Server, error) {
	if n.raftNode == nil {
		return nil, fmt.Errorf("raft node not started")
	}

	future := n.raftNode.GetConfiguration()
	if err := future.Error(); err != nil {
		return nil, fmt.Errorf("failed to get configuration: %w", err)
	}

	config := future.Configuration()
	return config.Servers, nil
}

// Add adds an item to the Bloom filter through Raft consensus.
func (n *Node) Add(item []byte) error {
	if n.raftNode == nil {
		return fmt.Errorf("raft node not started")
	}

	// Create command
	cmd := Command{
		Command: "add",
		Item:    item,
	}

	// Encode command
	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	// Apply through Raft
	future := n.raftNode.Apply(data, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to apply command: %w", err)
	}

	return nil
}

// Remove removes an item from the Bloom filter through Raft consensus.
func (n *Node) Remove(item []byte) error {
	if n.raftNode == nil {
		return fmt.Errorf("raft node not started")
	}

	cmd := Command{
		Command: "remove",
		Item:    item,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	future := n.raftNode.Apply(data, 0)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to apply command: %w", err)
	}

	return nil
}

// Contains checks if an item is in the Bloom filter (local read).
func (n *Node) Contains(item []byte) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return n.bloomFilter.Contains(item)
}

// IsLeader returns true if this node is the Raft leader.
func (n *Node) IsLeader() bool {
	if n.raftNode == nil {
		return false
	}
	return n.raftNode.State() == raft.Leader
}

// Leader returns the address of the current Raft leader.
func (n *Node) Leader() raft.ServerAddress {
	if n.raftNode == nil {
		return ""
	}
	return n.raftNode.Leader()
}

// Shutdown gracefully shuts down the Raft node.
func (n *Node) Shutdown() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.transport != nil {
		n.transport.Close()
	}

	if n.raftNode != nil {
		future := n.raftNode.Shutdown()
		if err := future.Error(); err != nil {
			log.Printf("Error shutting down raft: %v", err)
		}
	}

	if n.raftStore != nil {
		n.raftStore.Close()
	}

	log.Printf("Raft node %s shut down", n.nodeID)
}

// Apply applies a Raft log entry to the FSM (called by Raft on leader).
// This implements the raft.FSM interface.
func (n *Node) Apply(log *raft.Log) interface{} {
	// Parse the command
	var cmd Command
	if err := json.Unmarshal(log.Data, &cmd); err != nil {
		return fmt.Errorf("failed to parse command: %w", err)
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// Execute the command on the Bloom filter
	switch cmd.Command {
	case "add":
		n.bloomFilter.Add(cmd.Item)
		return nil
	case "remove":
		n.bloomFilter.Remove(cmd.Item)
		return nil
	default:
		return fmt.Errorf("unknown command: %s", cmd.Command)
	}
}

// Snapshot returns a snapshot of the FSM state.
// This implements the raft.FSM interface.
func (n *Node) Snapshot() (raft.FSMSnapshot, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Create a snapshot of the Bloom filter state
	bloomData := n.bloomFilter.Serialize()

	return &fsmSnapshot{
		bloomData: bloomData,
	}, nil
}

// Restore restores the FSM state from a snapshot.
// This implements the raft.FSM interface.
func (n *Node) Restore(rc io.ReadCloser) error {
	defer rc.Close()

	n.mu.Lock()
	defer n.mu.Unlock()

	// Read the snapshot data
	data, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("failed to read snapshot: %w", err)
	}

	// Restore the Bloom filter state
	newFilter, err := bloom.Deserialize(data)
	if err != nil {
		return fmt.Errorf("failed to restore bloom filter: %w", err)
	}

	n.bloomFilter = newFilter
	log.Printf("Restored FSM state from snapshot (%d bytes)", len(data))
	return nil
}

// fsmSnapshot implements raft.FSMSnapshot for the Bloom filter state.
type fsmSnapshot struct {
	bloomData []byte
}

// Persist writes the snapshot data to the given sink.
func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	_, err := sink.Write(s.bloomData)
	if err != nil {
		sink.Cancel()
		return err
	}
	return sink.Close()
}

// Release is called when the snapshot is no longer needed.
func (s *fsmSnapshot) Release() {
	// No cleanup needed
}

// GetState returns the current state of the node.
func (n *Node) GetState() map[string]interface{} {
	n.mu.RLock()
	defer n.mu.RUnlock()

	state := map[string]interface{}{
		"node_id":    n.nodeID,
		"raft_port":  n.raftPort,
		"is_leader":  n.IsLeader(),
		"bloom_size": n.bloomFilter.Size(),
		"bloom_k":    n.bloomFilter.HashCount(),
	}

	if n.raftNode != nil {
		state["raft_state"] = n.raftNode.State().String()
		state["leader"] = string(n.Leader())
	}

	return state
}
