package raft

import (
	"encoding/json"
	"fmt"
	"io"
	stdlog "log"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

// BloomFSM implements the raft.FSM interface for the distributed Bloom filter.
// It manages the state machine with WAL integration for persistence.
type BloomFSM struct {
	mu sync.RWMutex

	// Bloom filter state
	bloom *bloom.CountingBloomFilter

	// WAL encryptor for persistence
	wal *wal.WALEncryptor

	// WAL writer for appending logs
	walWriter *wal.WALWriter

	// Metadata
	lastAppliedIndex uint64
	lastAppliedTerm  uint64
	startTime        time.Time
}

// FSMCommand represents a command to be applied to the FSM.
type FSMCommand struct {
	// Type is the command type: "add" | "remove"
	Type string `json:"type"`

	// Item is the data item for the command.
	Item []byte `json:"item"`

	// Timestamp is when the command was created.
	Timestamp int64 `json:"timestamp"`

	// Index is the log index (for auditing).
	Index uint64 `json:"index"`

	// Term is the log term (for auditing).
	Term uint64 `json:"term"`
}

// NewBloomFSM creates a new BloomFSM with WAL integration.
func NewBloomFSM(bloomFilter *bloom.CountingBloomFilter, walEncryptor *wal.WALEncryptor, walDir string) (*BloomFSM, error) {
	fsm := &BloomFSM{
		bloom:    bloomFilter,
		wal:      walEncryptor,
		startTime: time.Now(),
	}

	// Create WAL writer for persistence
	if walDir != "" {
		walWriter, err := wal.NewWALWriter(walDir, walEncryptor)
		if err != nil {
			return nil, fmt.Errorf("failed to create WAL writer: %w", err)
		}
		fsm.walWriter = walWriter
	}

	return fsm, nil
}

// Apply applies a Raft log entry to the FSM.
// This implements the raft.FSM interface.
//
// IMPORTANT: This is the single source of truth for FSM state changes.
// All state modifications MUST go through this method to ensure consistency.
//
// Integration with WAL:
// 1. Parse the command from log data
// 2. Apply to Bloom filter (with lock protection)
// 3. Write to WAL for persistence (if enabled)
// 4. Return result
func (f *BloomFSM) Apply(log *raft.Log) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Parse the command - try both formats (Command and FSMCommand)
	var cmd Command
	if err := json.Unmarshal(log.Data, &cmd); err != nil {
		// Try FSMCommand format
		var fsmCmd FSMCommand
		if err2 := json.Unmarshal(log.Data, &fsmCmd); err2 != nil {
			return fmt.Errorf("failed to parse command: %w", err)
		}
		// Convert FSMCommand to Command
		cmd = Command{
			Type:      fsmCmd.Type,
			Item:      fsmCmd.Item,
			Timestamp: time.Unix(0, fsmCmd.Timestamp),
		}
	}

	// Execute the command on the Bloom filter
	var result interface{}
	switch cmd.Type {
	case "add":
		if err := f.bloom.Add(cmd.Item); err != nil {
			result = err
		} else {
			result = nil
		}
	case "remove":
		f.bloom.Remove(cmd.Item)
		result = nil
	default:
		result = fmt.Errorf("unknown command type: %s", cmd.Type)
	}

	// Write to WAL for persistence (if enabled)
	if f.walWriter != nil && result == nil {
		// Create WAL entry with full command context
		walEntry := FSMCommand{
			Type:      cmd.Type,
			Item:      cmd.Item,
			Timestamp: time.Now().UnixNano(),
			Index:     log.Index,
			Term:      log.Term,
		}

		walData, err := json.Marshal(walEntry)
		if err != nil {
			stdlog.Printf("WARNING: Failed to marshal WAL entry: %v", err)
			// Continue anyway - WAL is for durability, not correctness
		} else {
			if err := f.walWriter.Write(walData); err != nil {
				stdlog.Printf("WARNING: Failed to write WAL entry: %v", err)
				// Continue anyway - WAL is for durability, not correctness
			}
		}
	}

	// Update metadata
	f.lastAppliedIndex = log.Index
	f.lastAppliedTerm = log.Term

	return result
}

// Snapshot returns a snapshot of the FSM state.
// This implements the raft.FSMSnapshot interface.
//
// Integration with WAL encryption:
// 1. Get current Bloom filter state
// 2. Serialize to bytes
// 3. Return snapshot object (encryption happens in SnapshotManager)
func (f *BloomFSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Serialize Bloom filter state
	bloomData := f.bloom.Serialize()

	// Create snapshot with metadata
	snapshot := &fsmSnapshot{
		bloomData: bloomData,
		metadata: map[string]interface{}{
			"version":          "1.0",
			"created_by":       "distributed-bloom-filter",
			"bloom_size":       f.bloom.Size(),
			"bloom_k":          f.bloom.HashCount(),
			"last_applied_index": f.lastAppliedIndex,
			"last_applied_term":  f.lastAppliedTerm,
			"timestamp":        time.Now().UnixNano(),
			"start_time":       f.startTime.UnixNano(),
		},
	}

	return snapshot, nil
}

// Restore restores the FSM state from a snapshot.
// This implements the raft.FSM interface.
func (f *BloomFSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()

	f.mu.Lock()
	defer f.mu.Unlock()

	// Read the snapshot data
	data, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("failed to read snapshot: %w", err)
	}

	// Try to deserialize as SnapshotData first (new format with metadata)
	var snapshotData SnapshotData
	if err := json.Unmarshal(data, &snapshotData); err == nil {
		// New format with metadata
		newFilter, err := bloom.Deserialize(snapshotData.BloomFilter)
		if err != nil {
			return fmt.Errorf("failed to restore bloom filter: %w", err)
		}

		f.bloom = newFilter
		f.lastAppliedIndex = snapshotData.Index
		f.lastAppliedTerm = snapshotData.Term
		f.startTime = snapshotData.Timestamp

		stdlog.Printf("Restored FSM from snapshot at index %d, term %d", 
			snapshotData.Index, snapshotData.Term)
		return nil
	}

	// Old format: raw Bloom filter data
	newFilter, err := bloom.Deserialize(data)
	if err != nil {
		return fmt.Errorf("failed to restore bloom filter: %w", err)
	}

	f.bloom = newFilter
	stdlog.Printf("Restored FSM from snapshot (%d bytes)", len(data))
	return nil
}

// GetBloomFilter returns the Bloom filter instance.
func (f *BloomFSM) GetBloomFilter() *bloom.CountingBloomFilter {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.bloom
}

// GetLastAppliedIndex returns the last applied log index.
func (f *BloomFSM) GetLastAppliedIndex() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.lastAppliedIndex
}

// GetLastAppliedTerm returns the last applied log term.
func (f *BloomFSM) GetLastAppliedTerm() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.lastAppliedTerm
}

// GetStats returns FSM statistics.
func (f *BloomFSM) GetStats() map[string]interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return map[string]interface{}{
		"bloom_size":         f.bloom.Size(),
		"bloom_k":            f.bloom.HashCount(),
		"last_applied_index": f.lastAppliedIndex,
		"last_applied_term":  f.lastAppliedTerm,
		"start_time":         f.startTime.String(),
		"uptime":             time.Since(f.startTime).String(),
	}
}

// Close closes the FSM and releases resources.
func (f *BloomFSM) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.walWriter != nil {
		return f.walWriter.Close()
	}

	return nil
}

// fsmSnapshot implements raft.FSMSnapshot.
type fsmSnapshot struct {
	bloomData []byte
	metadata  map[string]interface{}
}

// Persist writes the snapshot to the given sink.
// This implements the raft.FSMSnapshot interface.
func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	// Create SnapshotData with metadata
	snapshotData := SnapshotData{
		BloomFilter: s.bloomData,
		Metadata:    s.metadata,
		Timestamp:   time.Now(),
		Index:       0, // Will be set by Raft
		Term:        0, // Will be set by Raft
	}

	// Serialize to JSON
	data, err := json.Marshal(snapshotData)
	if err != nil {
		sink.Cancel()
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	// Write to sink
	_, err = sink.Write(data)
	if err != nil {
		sink.Cancel()
		return fmt.Errorf("failed to write snapshot: %w", err)
	}

	return sink.Close()
}

// Release is called when the snapshot is no longer needed.
func (s *fsmSnapshot) Release() {
	// No cleanup needed
}
