package raft

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"

	"distributed-bloom-filter/pkg/bloom"
	"distributed-bloom-filter/internal/wal"
	"distributed-bloom-filter/internal/metadata"
)

// Node represents a Raft consensus node for the distributed Bloom filter.
type Node struct {
	nodeID          string
	raftPort        int
	dataDir         string
	bloomFilter     *bloom.CountingBloomFilter
	walEncryptor    *wal.Encryptor
	metadataService *metadata.Service
	
	raftNode        *raft.Raft
	raftStore       *raftboltdb.BoltStore
	transport       *raft.NetworkTransport
	
	mu sync.RWMutex
}

// NewNode creates a new Raft node.
func NewNode(nodeID string, raftPort int, dataDir string, 
	bloomFilter *bloom.CountingBloomFilter,
	walEncryptor *wal.Encryptor,
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
	n.mu.Lock()
	defer n.mu.Unlock()

	// Create Raft configuration
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(n.nodeID)

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

	// Create transport
	addr := fmt.Sprintf(":%d", n.raftPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to create transport listener: %w", err)
	}

	transport := raft.NewNetworkTransport(listener, 3, 0, os.Stderr)
	n.transport = transport

	// Create Raft instance
	ra, err := raft.NewRaft(config, nil, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return fmt.Errorf("failed to create raft: %w", err)
	}
	n.raftNode = ra

	// Bootstrap if this is the first node
	if bootstrap {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: raft.ServerAddress(listener.Addr().String()),
				},
			},
		}
		
		future := ra.BootstrapCluster(configuration)
		if err := future.Error(); err != nil {
			return fmt.Errorf("failed to bootstrap cluster: %w", err)
		}
		
		log.Printf("Bootstrapped Raft cluster with node %s", n.nodeID)
	}

	log.Printf("Raft node %s started on port %d", n.nodeID, n.raftPort)
	return nil
}

// Add adds an item to the Bloom filter through Raft consensus.
func (n *Node) Add(item []byte) error {
	if n.raftNode == nil {
		return fmt.Errorf("raft node not started")
	}

	// Create command
	cmd := map[string]interface{}{
		"command": "add",
		"item":    item,
	}

	// Encode command (in production, use proper serialization)
	data := []byte(fmt.Sprintf("%v", cmd))

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

	cmd := map[string]interface{}{
		"command": "remove",
		"item":    item,
	}

	data := []byte(fmt.Sprintf("%v", cmd))
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

// ApplyCommand applies a command to the FSM (called by Raft on leader).
// This is a skeleton - in production, implement the FSM interface properly.
func (n *Node) ApplyCommand(command []byte) error {
	// TODO: Parse and execute command
	// This would typically be implemented as part of the FSM interface
	return nil
}

// GetState returns the current state of the node.
func (n *Node) GetState() map[string]interface{} {
	n.mu.RLock()
	defer n.mu.RUnlock()

	state := map[string]interface{}{
		"node_id":      n.nodeID,
		"raft_port":    n.raftPort,
		"is_leader":    n.IsLeader(),
		"bloom_size":   n.bloomFilter.Size(),
		"bloom_k":      n.bloomFilter.HashCount(),
	}

	if n.raftNode != nil {
		state["raft_state"] = n.raftNode.State().String()
		state["leader"] = string(n.Leader())
	}

	return state
}
