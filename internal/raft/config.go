package raft

import (
	"crypto/tls"
	"errors"
	"time"
)

// Config holds the configuration for a Raft node.
type Config struct {
	// NodeID is the unique identifier for this node.
	NodeID string

	// RaftPort is the port used for Raft consensus communication.
	RaftPort int

	// DataDir is the directory for storing Raft data (logs, snapshots, etc.).
	DataDir string

	// HeartbeatTimeout is the time without contact from the leader before
	// a follower attempts to become leader.
	HeartbeatTimeout time.Duration

	// ElectionTimeout is the time without contact from the leader before
	// a follower attempts to become leader.
	ElectionTimeout time.Duration

	// CommitTimeout is the time without an Apply before sending an AppendEntries
	// to replicate a no-op to trigger a commit.
	CommitTimeout time.Duration

	// SnapshotThreshold is the number of outstanding logs that can exist before
	// triggering a snapshot.
	SnapshotThreshold uint64

	// SnapshotInterval is the interval at which snapshots are checked for creation.
	SnapshotInterval time.Duration

	// MaxPool is the maximum number of connections to maintain in the connection pool.
	MaxPool int

	// Timeout is the RPC timeout for Raft communications.
	Timeout time.Duration

	// LocalID is the local server ID.
	LocalID string

	// Bootstrap should be true if this is the first node in the cluster.
	Bootstrap bool

	// ClusterNodes is the list of initial cluster nodes for bootstrap.
	ClusterNodes []ServerConfig

	// UseInmemStore uses in-memory storage instead of BoltDB (for testing).
	UseInmemStore bool

	// TLSEnabled enables TLS encryption for Raft node communication.
	TLSEnabled bool

	// TLSConfig holds the TLS configuration for encrypted communication.
	TLSConfig *TLSRaftConfig
}

// TLSRaftConfig holds TLS configuration for Raft.
type TLSRaftConfig struct {
	// CAFile is the path to the CA certificate file
	CAFile string

	// CertFile is the path to the server certificate file
	CertFile string

	// KeyFile is the path to the server private key file
	KeyFile string

	// ClientCertFile is the path to the client certificate file (for outbound connections)
	ClientCertFile string

	// ClientKeyFile is the path to the client private key file (for outbound connections)
	ClientKeyFile string

	// ServerName is the expected server name for certificate verification
	ServerName string

	// MinVersion is the minimum TLS version (default: TLS 1.2)
	MinVersion uint16

	// InsecureSkipVerify disables certificate verification (development only)
	InsecureSkipVerify bool
}

// ServerConfig holds the configuration for a cluster server.
type ServerConfig struct {
	// ID is the unique identifier for the server.
	ID string

	// Address is the network address for Raft communication.
	Address string

	// Voter is true if this server is a voting member of the cluster.
	Voter bool
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		HeartbeatTimeout:  1000 * time.Millisecond,
		ElectionTimeout:   1000 * time.Millisecond,
		CommitTimeout:     50 * time.Millisecond,
		SnapshotThreshold: 8192,
		SnapshotInterval:  120 * time.Second,
		MaxPool:           3,
		Timeout:           10 * time.Second,
		Bootstrap:         false,
		UseInmemStore:     false,
		TLSEnabled:        true, // Enable TLS by default for security
		TLSConfig: &TLSRaftConfig{
			MinVersion:         tls.VersionTLS12,
			ServerName:         "localhost",
			InsecureSkipVerify: false,
		},
	}
}

// Errors for configuration validation.
var (
	ErrNodeIDRequired = errors.New("node ID is required")
	ErrInvalidPort    = errors.New("invalid port number")
	ErrDataDirRequired = errors.New("data directory is required")
)

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.NodeID == "" {
		return ErrNodeIDRequired
	}
	if c.RaftPort <= 0 || c.RaftPort > 65535 {
		return ErrInvalidPort
	}
	if c.DataDir == "" {
		return ErrDataDirRequired
	}
	return nil
}
