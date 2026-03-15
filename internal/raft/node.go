package raft

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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

// RaftNode defines the interface for Raft node operations.
// This allows mocking for testing.
type RaftNode interface {
	Start() error
	Shutdown() error
	IsLeader() bool
	Add(item []byte) error
	Contains(item []byte) bool
	Remove(item []byte) error
	BatchAdd(items [][]byte) (successCount int, failureCount int, errors []string)
	BatchContains(items [][]byte) []bool
	GetState() map[string]interface{}
	GetConfig() map[string]interface{}
}

// Node represents a Raft consensus node for the distributed Bloom filter.
// It implements the raft.FSM interface for state machine replication.
type Node struct {
	// Configuration
	config *Config

	// Dependencies
	bloomFilter     *bloom.CountingBloomFilter
	walEncryptor    *wal.WALEncryptor
	metadataService *metadata.Service

	// Raft components
	raftNode      *raft.Raft
	raftStore     raft.LogStore
	raftStable    raft.StableStore
	transport     *raft.NetworkTransport
	snapshotStore raft.SnapshotStore

	// Managers
	stateManager      *StateManager
	logManager        *LogManager
	electionManager   *ElectionManager
	replicationManager *ReplicationManager
	snapshotManager   *SnapshotManager

	// FSM - embedded state machine for Raft
	// All state changes go through this single FSM to ensure consistency
	fsm *BloomFSM

	// Runtime state
	mu sync.RWMutex
}

// NewNode creates a new Raft node with the given configuration.
func NewNode(config *Config,
	bloomFilter *bloom.CountingBloomFilter,
	walEncryptor *wal.WALEncryptor,
	metadataService *metadata.Service) (*Node, error) {

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Create the FSM - this is the single source of truth for state changes
	fsm, err := NewBloomFSM(bloomFilter, walEncryptor, filepath.Join(config.DataDir, "wal"))
	if err != nil {
		return nil, fmt.Errorf("failed to create FSM: %w", err)
	}

	node := &Node{
		config:            config,
		bloomFilter:       bloomFilter,
		walEncryptor:      walEncryptor,
		metadataService:   metadataService,
		stateManager:      NewStateManager(),
		logManager:        NewLogManager(),
		electionManager:   NewElectionManager(),
		replicationManager: NewReplicationManager(),
		snapshotManager:   NewSnapshotManager(bloomFilter),
		fsm:               fsm,
	}

	return node, nil
}

// NewNodeWithDefaults creates a new Raft node with default configuration.
func NewNodeWithDefaults(nodeID string, raftPort int, dataDir string,
	bloomFilter *bloom.CountingBloomFilter,
	walEncryptor *wal.WALEncryptor,
	metadataService *metadata.Service) (*Node, error) {

	config := DefaultConfig()
	config.NodeID = nodeID
	config.RaftPort = raftPort
	config.DataDir = dataDir
	config.LocalID = nodeID

	return NewNode(config, bloomFilter, walEncryptor, metadataService)
}

// Start initializes and starts the Raft node.
func (n *Node) Start() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Create Raft configuration
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(n.config.LocalID)
	raftConfig.HeartbeatTimeout = n.config.HeartbeatTimeout
	raftConfig.ElectionTimeout = n.config.ElectionTimeout
	raftConfig.CommitTimeout = n.config.CommitTimeout
	raftConfig.SnapshotThreshold = n.config.SnapshotThreshold
	raftConfig.SnapshotInterval = n.config.SnapshotInterval
	// Note: MaxPool and Timeout are not in HashiCorp Raft Config

	// Create data directory for Raft
	raftDir := filepath.Join(n.config.DataDir, "raft")
	if err := os.MkdirAll(raftDir, 0755); err != nil {
		return fmt.Errorf("failed to create raft directory: %w", err)
	}

	var logStore raft.LogStore
	var stableStore raft.StableStore
	var snapshotStore raft.SnapshotStore

	// Use in-memory store for testing (avoids bolt DB race detection issues)
	if n.config.UseInmemStore {
		inmemStore := raft.NewInmemStore()
		n.raftStore = inmemStore
		logStore = inmemStore
		stableStore = inmemStore
		// Use in-memory snapshot store for testing
		snapshotStore = raft.NewInmemSnapshotStore()
		if fs, ok := snapshotStore.(*raft.InmemSnapshotStore); ok {
			n.snapshotStore = fs
			n.snapshotManager.SetSnapshotStore(fs)
		}
	} else {
		// Create BoltDB store for Raft logs and stable storage
		boltStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "raft.db"))
		if err != nil {
			return fmt.Errorf("failed to create bolt store: %w", err)
		}
		n.raftStore = boltStore
		n.raftStable = boltStore
		logStore = boltStore
		stableStore = boltStore

		// Create snapshot store
		var err2 error
		snapshotStore, err2 = raft.NewFileSnapshotStore(raftDir, 3, os.Stderr)
		if err2 != nil {
			return fmt.Errorf("failed to create snapshot store: %w", err2)
		}
		if fs, ok := snapshotStore.(*raft.FileSnapshotStore); ok {
			n.snapshotStore = fs
			n.snapshotManager.SetSnapshotStore(fs)
		}
	}

	// Create transport (with TLS if enabled)
	bindAddr := fmt.Sprintf("127.0.0.1:%d", n.config.RaftPort)
	tcpAddr, err := net.ResolveTCPAddr("tcp", bindAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve TCP address: %w", err)
	}

	var transport *raft.NetworkTransport
	if n.config.TLSEnabled && n.config.TLSConfig != nil {
		// Create TLS-encrypted transport
		transport, err = n.createTLSTransport(bindAddr, tcpAddr)
		if err != nil {
			return fmt.Errorf("failed to create TLS transport: %w", err)
		}
		log.Printf("Raft node %s: TLS transport enabled on %s", n.config.NodeID, bindAddr)
	} else {
		// Create plain TCP transport (development/testing only)
		transport, err = raft.NewTCPTransport(bindAddr, tcpAddr, 3, 10*time.Second, os.Stderr)
		if err != nil {
			return fmt.Errorf("failed to create transport: %w", err)
		}
		log.Printf("Raft node %s: Plain TCP transport enabled on %s (INSECURE)", n.config.NodeID, bindAddr)
	}
	n.transport = transport

	// Create Raft instance (pass n as the FSM)
	ra, err := raft.NewRaft(raftConfig, n, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return fmt.Errorf("failed to create raft: %w", err)
	}
	n.raftNode = ra

	// Set up managers with Raft node reference
	n.logManager.SetRaftNode(ra)
	n.electionManager.SetRaftNode(ra)
	n.replicationManager.SetRaftNode(ra)

	// Bootstrap if this is the first node
	if n.config.Bootstrap {
		if err := n.bootstrapCluster(ra); err != nil {
			return err
		}
	}

	// Update state manager
	n.stateManager.SetState(ConvertRaftState(ra.State()))

	log.Printf("Raft node %s started on port %d", n.config.NodeID, n.config.RaftPort)
	return nil
}

// createTLSTransport creates a TLS-encrypted transport for Raft.
func (n *Node) createTLSTransport(bindAddr string, advertise net.Addr) (*raft.NetworkTransport, error) {
	tlsConfig := n.config.TLSConfig

	// Validate TLS configuration
	if tlsConfig.CertFile == "" || tlsConfig.KeyFile == "" {
		return nil, fmt.Errorf("TLS certificate and key files are required")
	}

	// Load server certificate
	cert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Create CA certificate pool for client verification
	caCertPool := x509.NewCertPool()
	if tlsConfig.CAFile != "" {
		caCertPEM, err := os.ReadFile(tlsConfig.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load CA certificate: %w", err)
		}
		if !caCertPool.AppendCertsFromPEM(caCertPEM) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
	}

	// Create TLS configuration for server
	serverTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert, // Enforce mTLS
		ClientCAs:    caCertPool,
		MinVersion:   tlsConfig.MinVersion,
		ServerName:   tlsConfig.ServerName,
	}

	// For development, allow insecure connections
	if tlsConfig.InsecureSkipVerify {
		serverTLSConfig.ClientAuth = tls.RequireAnyClientCert
	}

	// Listen on the bind address
	listener, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	// Wrap with TLS
	tlsListener := tls.NewListener(listener, serverTLSConfig)

	// Create TLS stream layer
	streamLayer := &tlsStreamLayer{
		listener:   tlsListener,
		tlsConfig:  serverTLSConfig,
		advertise:  advertise,
		serverName: tlsConfig.ServerName,
		caPool:     caCertPool,
	}

	// Create network transport config
	config := &raft.NetworkTransportConfig{
		Stream:          streamLayer,
		MaxPool:         n.config.MaxPool,
		Timeout:         n.config.Timeout,
		MaxRPCsInFlight: 2,
	}

	// Create network transport
	transport := raft.NewNetworkTransportWithConfig(config)

	return transport, nil
}

// tlsStreamLayer implements raft.StreamLayer for TLS-encrypted connections.
type tlsStreamLayer struct {
	listener   net.Listener
	tlsConfig  *tls.Config
	advertise  net.Addr
	serverName string
	caPool     *x509.CertPool
}

func (t *tlsStreamLayer) Accept() (net.Conn, error) {
	return t.listener.Accept()
}

func (t *tlsStreamLayer) Close() error {
	return t.listener.Close()
}

func (t *tlsStreamLayer) Addr() net.Addr {
	if t.advertise != nil {
		return t.advertise
	}
	return t.listener.Addr()
}

func (t *tlsStreamLayer) Dial(address raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	// Create TLS config for client
	clientTLSConfig := t.tlsConfig.Clone()
	clientTLSConfig.ServerName = t.serverName
	clientTLSConfig.InsecureSkipVerify = false

	// Load client certificate if available (for mTLS)
	// In production, these would be configured separately
	if t.tlsConfig.Certificates != nil && len(t.tlsConfig.Certificates) > 0 {
		clientTLSConfig.Certificates = t.tlsConfig.Certificates
	}

	// Dial the remote address
	conn, err := net.DialTimeout("tcp", string(address), timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}

	// Wrap with TLS
	tlsConn := tls.Client(conn, clientTLSConfig)

	// Set deadline for handshake
	if err := tlsConn.SetDeadline(time.Now().Add(timeout)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to set deadline: %w", err)
	}

	// Perform TLS handshake
	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("TLS handshake failed: %w", err)
	}

	// Clear deadline
	if err := tlsConn.SetDeadline(time.Time{}); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to clear deadline: %w", err)
	}

	return tlsConn, nil
}

// bootstrapCluster bootstraps a new Raft cluster.
func (n *Node) bootstrapCluster(ra *raft.Raft) error {
	advertiseAddr := fmt.Sprintf("127.0.0.1:%d", n.config.RaftPort)
	configuration := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      raft.ServerID(n.config.LocalID),
				Address: raft.ServerAddress(advertiseAddr),
			},
		},
	}

	future := ra.BootstrapCluster(configuration)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to bootstrap cluster: %w", err)
	}

	// Update metadata service
	if n.metadataService != nil {
		n.metadataService.SetNodeID(n.config.NodeID)
		n.metadataService.AddClusterNode(n.config.NodeID)
		n.metadataService.Save()
	}

	log.Printf("Bootstrapped Raft cluster with node %s", n.config.NodeID)
	return nil
}

// Add adds an item to the Bloom filter through Raft consensus.
func (n *Node) Add(item []byte) error {
	return n.logManager.AddItem(item, n.config.Timeout)
}

// Remove removes an item from the Bloom filter through Raft consensus.
func (n *Node) Remove(item []byte) error {
	return n.logManager.RemoveItem(item, n.config.Timeout)
}

// Contains checks if an item is in the Bloom filter (local read).
func (n *Node) Contains(item []byte) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.bloomFilter == nil {
		return false
	}
	return n.bloomFilter.Contains(item)
}

// BatchAdd adds multiple items to the Bloom filter.
func (n *Node) BatchAdd(items [][]byte) (successCount int, failureCount int, errors []string) {
	errors = make([]string, len(items))
	for i, item := range items {
		if len(item) == 0 {
			errors[i] = "empty item"
			failureCount++
		} else {
			if err := n.Add(item); err != nil {
				errors[i] = err.Error()
				failureCount++
			} else {
				successCount++
			}
		}
	}
	return
}

// BatchContains checks multiple items in the Bloom filter.
func (n *Node) BatchContains(items [][]byte) []bool {
	results := make([]bool, len(items))
	for i, item := range items {
		results[i] = n.Contains(item)
	}
	return results
}

// GetConfig returns the node configuration.
func (n *Node) GetConfig() map[string]interface{} {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return map[string]interface{}{
		"node_id":         n.config.NodeID,
		"raft_port":       n.config.RaftPort,
		"data_dir":        n.config.DataDir,
		"bloom_size":      n.bloomFilter.Size(),
		"bloom_k":         n.bloomFilter.HashCount(),
		"bootstrap":       n.config.Bootstrap,
		"heartbeat":       n.config.HeartbeatTimeout.String(),
		"election":        n.config.ElectionTimeout.String(),
		"commit":          n.config.CommitTimeout.String(),
		"snapshot_int":    n.config.SnapshotInterval.String(),
		"snapshot_thresh": n.config.SnapshotThreshold,
	}
}

// IsLeader returns true if this node is the Raft leader.
func (n *Node) IsLeader() bool {
	return n.electionManager.IsLeader()
}

// GetLeader returns the current leader's ID and address.
func (n *Node) GetLeader() (string, string) {
	id, addr := n.electionManager.GetLeader()
	return id, string(addr)
}

// GetState returns the current state of the node.
func (n *Node) GetState() map[string]interface{} {
	n.mu.RLock()
	defer n.mu.RUnlock()

	state := map[string]interface{}{
		"node_id":    n.config.NodeID,
		"raft_port":  n.config.RaftPort,
		"is_leader":  n.electionManager.IsLeader(),
		"bloom_size": 0,
		"bloom_k":    0,
	}

	if n.bloomFilter != nil {
		state["bloom_size"] = n.bloomFilter.Size()
		state["bloom_k"] = n.bloomFilter.HashCount()
	}

	if n.raftNode != nil {
		state["raft_state"] = n.raftNode.State().String()
		state["last_index"] = n.raftNode.LastIndex()
		state["commit_index"] = n.raftNode.LastIndex()
		
		leaderID, leaderAddr := n.electionManager.GetLeader()
		state["leader"] = leaderID
		state["leader_address"] = string(leaderAddr)
	}

	return state
}

// Shutdown gracefully shuts down the Raft node.
func (n *Node) Shutdown() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.stateManager.SetState(StateShutdown)

	var lastErr error

	// Close FSM first to flush any pending WAL writes
	if n.fsm != nil {
		if err := n.fsm.Close(); err != nil {
			log.Printf("Error closing FSM: %v", err)
			lastErr = err
		}
	}

	if n.transport != nil {
		if err := n.transport.Close(); err != nil {
			lastErr = err
		}
	}

	if n.raftNode != nil {
		future := n.raftNode.Shutdown()
		if err := future.Error(); err != nil {
			log.Printf("Error shutting down raft: %v", err)
			lastErr = err
		}
	}

	// Close BoltDB store explicitly to prevent resource泄漏
	// This is only needed when using persistent storage (not inmem store)
	if n.raftStore != nil && !n.config.UseInmemStore {
		if boltStore, ok := n.raftStore.(*raftboltdb.BoltStore); ok {
			if err := boltStore.Close(); err != nil {
				log.Printf("Error closing BoltDB: %v", err)
				lastErr = err
			} else {
				log.Printf("BoltDB closed successfully for node %s", n.config.NodeID)
			}
		}
	}

	log.Printf("Raft node %s shut down", n.config.NodeID)
	return lastErr
}

// Apply applies a Raft log entry to the FSM (called by Raft on leader).
// This implements the raft.FSM interface.
//
// IMPORTANT: This is the ONLY path for FSM state changes.
// All state modifications must go through this method to ensure consistency.
// The actual state change logic is delegated to BloomFSM.Apply() to avoid
// duplicate implementations and potential data races.
func (n *Node) Apply(log *raft.Log) interface{} {
	// Delegate to the embedded FSM - this ensures a single unified Apply path
	result := n.fsm.Apply(log)

	// Update metadata service (non-critical, doesn't affect FSM state)
	if result == nil && n.metadataService != nil {
		var cmd Command
		if err := json.Unmarshal(log.Data, &cmd); err == nil {
			switch cmd.Type {
			case "add":
				n.metadataService.RecordAdd()
			case "remove":
				n.metadataService.RecordRemove()
			}
		}
	}

	// Update state manager
	n.stateManager.SetLastApplied(log.Index)

	return result
}

// Snapshot returns a snapshot of the FSM state.
// This implements the raft.FSM interface.
//
// Delegates to BloomFSM to ensure consistency with Apply().
func (n *Node) Snapshot() (raft.FSMSnapshot, error) {
	return n.fsm.Snapshot()
}

// Restore restores the FSM state from a snapshot.
// This implements the raft.FSM interface.
//
// Delegates to BloomFSM to ensure consistency with Apply().
func (n *Node) Restore(rc io.ReadCloser) error {
	err := n.fsm.Restore(rc)
	if err != nil {
		return err
	}

	// Update snapshot manager after restore
	n.snapshotManager.RestoreFromFSM(n.fsm.GetLastAppliedIndex(), n.fsm.GetLastAppliedTerm())
	return nil
}

// GetBloomFilter returns the Bloom filter instance.
func (n *Node) GetBloomFilter() *bloom.CountingBloomFilter {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.bloomFilter
}

// GetStateManager returns the state manager.
func (n *Node) GetStateManager() *StateManager {
	return n.stateManager
}

// GetLogManager returns the log manager.
func (n *Node) GetLogManager() *LogManager {
	return n.logManager
}

// GetElectionManager returns the election manager.
func (n *Node) GetElectionManager() *ElectionManager {
	return n.electionManager
}

// GetReplicationManager returns the replication manager.
func (n *Node) GetReplicationManager() *ReplicationManager {
	return n.replicationManager
}

// GetSnapshotManager returns the snapshot manager.
func (n *Node) GetSnapshotManager() *SnapshotManager {
	return n.snapshotManager
}

// WaitForLeader waits for a leader to be elected.
func (n *Node) WaitForLeader(timeout time.Duration) (string, string, error) {
	ctx := &timeoutContext{deadline: time.Now().Add(timeout)}
	id, addr, err := n.electionManager.WaitForLeader(ctx, timeout)
	return id, string(addr), err
}

// AddPeer adds a new peer to the cluster.
func (n *Node) AddPeer(serverID string, address string, voter bool) error {
	return n.replicationManager.AddPeer(serverID, raft.ServerAddress(address), voter)
}

// RemovePeer removes a peer from the cluster.
func (n *Node) RemovePeer(serverID string) error {
	return n.replicationManager.RemovePeer(serverID)
}

// GetPeers returns information about all peers.
func (n *Node) GetPeers() map[string]*PeerInfo {
	return n.replicationManager.GetAllPeers()
}

// SaveSnapshot saves a snapshot to a file.
func (n *Node) SaveSnapshot(filePath string) error {
	return n.snapshotManager.SaveSnapshotToFile(filePath)
}

// LoadSnapshot loads a snapshot from a file.
func (n *Node) LoadSnapshot(filePath string) error {
	return n.snapshotManager.LoadSnapshotFromFile(filePath)
}

// timeoutContext is a simple context implementation for timeouts.
type timeoutContext struct {
	deadline time.Time
}

func (c *timeoutContext) Deadline() (time.Time, bool) {
	return c.deadline, true
}

func (c *timeoutContext) Done() <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		<-time.After(time.Until(c.deadline))
		close(ch)
	}()
	return ch
}

func (c *timeoutContext) Err() error {
	if time.Now().After(c.deadline) {
		return context.DeadlineExceeded
	}
	return nil
}

func (c *timeoutContext) Value(key interface{}) interface{} {
	return nil
}
