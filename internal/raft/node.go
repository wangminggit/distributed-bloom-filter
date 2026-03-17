package raft

import (
	"crypto/tls"
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
	dbftls "github.com/wangminggit/distributed-bloom-filter/pkg/tls"
)

// RaftNode defines the interface for Raft node operations.
type RaftNode interface {
	Start(bootstrap bool) error
	Shutdown() error
	Add(item []byte) error
	Remove(item []byte) error
	Contains(item []byte) bool
	BatchAdd(items [][]byte) (successCount, failureCount int, errors []string)
	BatchContains(items [][]byte) []bool
	IsLeader() bool
	Leader() string
	GetState() map[string]interface{}
}

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
	tlsConfig *tls.Config

	mu sync.RWMutex
}

// RaftTLSConfig holds TLS configuration for Raft transport.
type RaftTLSConfig struct {
	EnableTLS      bool
	CertPath       string
	KeyPath        string
	CAPath         string
	MinVersion     uint16
	ReloadInterval time.Duration
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

// NewNodeWithTLS creates a new Raft node with TLS configuration.
func NewNodeWithTLS(nodeID string, raftPort int, dataDir string,
	bloomFilter *bloom.CountingBloomFilter,
	walEncryptor *wal.WALEncryptor,
	metadataService *metadata.Service,
	tlsConfig *RaftTLSConfig) (*Node, error) {

	node := &Node{
		nodeID:          nodeID,
		raftPort:        raftPort,
		dataDir:         dataDir,
		bloomFilter:     bloomFilter,
		walEncryptor:    walEncryptor,
		metadataService: metadataService,
	}

	// Build TLS config if enabled
	if tlsConfig != nil && tlsConfig.EnableTLS {
		dbfTLSConfig := &dbftls.Config{
			CertPath:   tlsConfig.CertPath,
			KeyPath:    tlsConfig.KeyPath,
			CAPath:     tlsConfig.CAPath,
			MinVersion: tlsConfig.MinVersion,
			ClientAuth: tls.RequireAndVerifyClientCert,
		}

		if dbfTLSConfig.MinVersion == 0 {
			dbfTLSConfig.MinVersion = tls.VersionTLS13
		}

		var err error
		if tlsConfig.ReloadInterval > 0 {
			// Use cert reloader for hot reload
			reloader, err := dbftls.NewCertReloader(dbfTLSConfig, tlsConfig.ReloadInterval)
			if err != nil {
				return nil, fmt.Errorf("failed to create cert reloader: %w", err)
			}
			node.tlsConfig, err = reloader.GetTLSConfig()
			if err != nil {
				return nil, fmt.Errorf("failed to get TLS config: %w", err)
			}
		} else {
			node.tlsConfig, err = dbftls.BuildTLSConfig(dbfTLSConfig)
			if err != nil {
				return nil, fmt.Errorf("failed to build TLS config: %w", err)
			}
		}
	}

	return node, nil
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

	// Create transport - use localhost for local binding
	bindAddr := fmt.Sprintf("127.0.0.1:%d", n.raftPort)
	tcpAddr, err := net.ResolveTCPAddr("tcp", bindAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve TCP address: %w", err)
	}

	// Create transport with or without TLS
	var transportImpl *raft.NetworkTransport
	if n.tlsConfig != nil {
		// Create TLS-wrapped transport
		transportImpl, err = n.createTLSTransport(tcpAddr)
	} else {
		// Create standard TCP transport
		transportImpl, err = raft.NewTCPTransport(bindAddr, tcpAddr, 3, 10*time.Second, os.Stderr)
	}
	if err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}
	n.transport = transportImpl

	// Create Raft instance (pass n as the FSM)
	ra, err := raft.NewRaft(config, n, logStore, stableStore, snapshotStore, transportImpl)
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
	cmd := Command{
		Type: "add",
		Item: item,
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
		Type: "remove",
		Item: item,
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
	switch cmd.Type {
	case "add":
		n.bloomFilter.Add(cmd.Item)
		return nil
	case "remove":
		n.bloomFilter.Remove(cmd.Item)
		return nil
	default:
		return fmt.Errorf("unknown command: %s", cmd.Type)
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
		"tls_enabled": n.tlsConfig != nil,
	}

	if n.raftNode != nil {
		state["raft_state"] = n.raftNode.State().String()
		state["leader"] = string(n.Leader())
	}

	return state
}

// createTLSTransport creates a TLS-wrapped Raft network transport.
// This uses a custom StreamLayer to wrap connections with TLS.
func (n *Node) createTLSTransport(tcpAddr *net.TCPAddr) (*raft.NetworkTransport, error) {
	// Create a TLS stream layer
	streamLayer := &tlsStreamLayer{
		tlsConfig: n.tlsConfig,
		tcpAddr:   tcpAddr,
	}

	// Create network transport with the TLS stream layer
	// Using nil logger for simplicity - can be customized
	transport := raft.NewNetworkTransportWithLogger(
		streamLayer,
		3,
		10*time.Second,
		nil,
	)

	return transport, nil
}

// tlsStreamLayer implements raft.StreamLayer with TLS support.
type tlsStreamLayer struct {
	tlsConfig *tls.Config
	tcpAddr   *net.TCPAddr
	listener  net.Listener
}

// Accept accepts incoming TLS connections.
func (s *tlsStreamLayer) Accept() (net.Conn, error) {
	if s.listener == nil {
		// Create listener on first accept
		baseLis, err := net.ListenTCP("tcp", s.tcpAddr)
		if err != nil {
			return nil, err
		}
		s.listener = tls.NewListener(baseLis, s.tlsConfig)
	}
	return s.listener.Accept()
}

// Close closes the listener.
func (s *tlsStreamLayer) Close() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// Addr returns the listener address.
func (s *tlsStreamLayer) Addr() net.Addr {
	if s.listener != nil {
		return s.listener.Addr()
	}
	return s.tcpAddr
}

// Dial creates an outgoing TLS connection.
func (s *tlsStreamLayer) Dial(address raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", string(address), timeout)
	if err != nil {
		return nil, err
	}

	// Wrap with TLS
	tlsConn := tls.Client(conn, s.tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	return tlsConn, nil
}
