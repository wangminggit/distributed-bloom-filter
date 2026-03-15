package raft

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/hashicorp/raft"
)

// LogEntry represents a single log entry in the Raft log.
type LogEntry struct {
	// Index is the position in the log.
	Index uint64

	// Term is the term in which this entry was created.
	Term uint64

	// Type is the type of log entry.
	Type LogType

	// Data is the log entry data.
	Data []byte

	// Timestamp is when the entry was created.
	Timestamp time.Time
}

// LogType represents the type of a log entry.
type LogType uint8

const (
	// LogCommand is a command that modifies the FSM state.
	LogCommand LogType = iota
	// LogNoop is a no-operation entry used for leader election and commits.
	LogNoop
)

// Command represents a command to be executed on the FSM.
type Command struct {
	// Type is the command type.
	Type string `json:"type"`

	// Item is the data item for the command.
	Item []byte `json:"item"`

	// Timestamp is when the command was created.
	Timestamp time.Time `json:"timestamp"`
}

// NewCommand creates a new command.
func NewCommand(cmdType string, item []byte) *Command {
	return &Command{
		Type:      cmdType,
		Item:      item,
		Timestamp: time.Now(),
	}
}

// Marshal serializes the command to JSON.
func (c *Command) Marshal() ([]byte, error) {
	return json.Marshal(c)
}

// UnmarshalCommand deserializes a command from JSON.
func UnmarshalCommand(data []byte) (*Command, error) {
	var cmd Command
	if err := json.Unmarshal(data, &cmd); err != nil {
		return nil, err
	}
	return &cmd, nil
}

// LogManager manages the Raft log with additional metadata.
type LogManager struct {
	mu sync.RWMutex

	// Reference to the Raft node
	raftNode *raft.Raft

	// Pending commands waiting for confirmation
	pendingCommands map[uint64]*pendingCommand

	// Statistics
	stats *LogStats
}

type pendingCommand struct {
	cmd       *Command
	timestamp time.Time
	done      chan error
}

// LogStats holds statistics about log operations.
type LogStats struct {
	TotalCommands    int64
	TotalAppends     int64
	TotalCommits     int64
	TotalFailures    int64
	LastCommandIndex uint64
	LastCommandTime  time.Time
}

// NewLogManager creates a new log manager.
func NewLogManager() *LogManager {
	return &LogManager{
		pendingCommands: make(map[uint64]*pendingCommand),
		stats: &LogStats{
			TotalCommands: 0,
		},
	}
}

// SetRaftNode sets the Raft node reference.
func (lm *LogManager) SetRaftNode(node *raft.Raft) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.raftNode = node
}

// ApplyCommand applies a command through Raft consensus.
func (lm *LogManager) ApplyCommand(cmd *Command, timeout time.Duration) error {
	lm.mu.Lock()
	if lm.raftNode == nil {
		lm.mu.Unlock()
		return ErrRaftNotStarted
	}
	lm.mu.Unlock()

	// Marshal the command
	data, err := cmd.Marshal()
	if err != nil {
		return err
	}

	// Apply through Raft
	future := lm.raftNode.Apply(data, timeout)
	if err := future.Error(); err != nil {
		lm.mu.Lock()
		lm.stats.TotalFailures++
		lm.mu.Unlock()
		return err
	}

	lm.mu.Lock()
	lm.stats.TotalCommits++
	lm.stats.LastCommandIndex = future.Index()
	lm.stats.LastCommandTime = time.Now()
	lm.mu.Unlock()

	return nil
}

// AddItem adds an item to the Bloom filter.
func (lm *LogManager) AddItem(item []byte, timeout time.Duration) error {
	cmd := NewCommand("add", item)
	return lm.ApplyCommand(cmd, timeout)
}

// RemoveItem removes an item from the Bloom filter.
func (lm *LogManager) RemoveItem(item []byte, timeout time.Duration) error {
	cmd := NewCommand("remove", item)
	return lm.ApplyCommand(cmd, timeout)
}

// GetLastIndex returns the last log index.
func (lm *LogManager) GetLastIndex() (uint64, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if lm.raftNode == nil {
		return 0, ErrRaftNotStarted
	}

	return lm.raftNode.LastIndex(), nil
}

// GetFirstIndex returns the first log index.
func (lm *LogManager) GetFirstIndex() (uint64, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if lm.raftNode == nil {
		return 0, ErrRaftNotStarted
	}

	// Note: FirstIndex is not exported in HashiCorp Raft
	// Return 0 as placeholder - can be retrieved from log store if needed
	return 0, nil
}

// GetStats returns a copy of the log statistics.
func (lm *LogManager) GetStats() LogStats {
	if lm == nil || lm.stats == nil {
		return LogStats{}
	}
	lm.mu.RLock()
	stats := *lm.stats
	lm.mu.RUnlock()
	return stats
}

// Errors for log management.
var (
	ErrRaftNotStarted  = errors.New("raft node not started")
	ErrInvalidLogEntry = errors.New("invalid log entry")
	ErrLogNotFound     = errors.New("log entry not found")
)

// ConvertLogType converts a HashiCorp Raft log type to our LogType.
func ConvertLogType(logType raft.LogType) LogType {
	switch logType {
	case raft.LogCommand:
		return LogCommand
	case raft.LogNoop:
		return LogNoop
	default:
		return LogCommand
	}
}
