package raft

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	"github.com/wangminggit/distributed-bloom-filter/internal/wal"
	"github.com/wangminggit/distributed-bloom-filter/pkg/bloom"
)

// SnapshotManager manages Raft snapshots with encryption support.
type SnapshotManager struct {
	mu sync.RWMutex

	// Snapshot store reference
	snapshotStore raft.SnapshotStore

	// Bloom filter reference
	bloomFilter *bloom.CountingBloomFilter

	// WAL encryptor for snapshot encryption
	encryptor *wal.WALEncryptor

	// Snapshot directory
	snapshotDir string

	// Snapshot statistics
	stats *SnapshotStats

	// Last snapshot information
	lastSnapshotIndex uint64
	lastSnapshotTerm  uint64
	lastSnapshotTime  time.Time
}

// SnapshotStats holds statistics about snapshots.
type SnapshotStats struct {
	TotalSnapshots      int64
	TotalSnapshotSize   int64
	LastSnapshotIndex   uint64
	LastSnapshotTerm    uint64
	LastSnapshotTime    time.Time
	TotalRestores       int64
	AverageSnapshotSize int64
}

// SnapshotData contains the data to be snapshotted.
type SnapshotData struct {
	// BloomFilter is the serialized Bloom filter state.
	BloomFilter []byte `json:"bloom_filter"`

	// Metadata contains additional metadata.
	Metadata map[string]interface{} `json:"metadata"`

	// Timestamp is when the snapshot was created.
	Timestamp time.Time `json:"timestamp"`

	// Index is the log index of this snapshot.
	Index uint64 `json:"index"`

	// Term is the term of this snapshot.
	Term uint64 `json:"term"`
}

// NewSnapshotManager creates a new snapshot manager with encryption support.
func NewSnapshotManager(bloomFilter *bloom.CountingBloomFilter) *SnapshotManager {
	return &SnapshotManager{
		bloomFilter: bloomFilter,
		stats:       &SnapshotStats{},
	}
}

// NewSnapshotManagerWithEncryption creates a new snapshot manager with WAL encryption.
func NewSnapshotManagerWithEncryption(bloomFilter *bloom.CountingBloomFilter, 
	encryptor *wal.WALEncryptor, snapshotDir string) *SnapshotManager {
	
	return &SnapshotManager{
		bloomFilter: bloomFilter,
		encryptor:   encryptor,
		snapshotDir: snapshotDir,
		stats:       &SnapshotStats{},
	}
}

// SetSnapshotStore sets the snapshot store reference.
func (sm *SnapshotManager) SetSnapshotStore(store raft.SnapshotStore) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.snapshotStore = store
}

// CreateSnapshot creates a new snapshot.
func (sm *SnapshotManager) CreateSnapshot(index uint64, term uint64) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.bloomFilter == nil {
		return ErrBloomFilterNotInitialized
	}

	// Serialize the Bloom filter
	bloomData := sm.bloomFilter.Serialize()

	// Create snapshot data
	snapshotData := &SnapshotData{
		BloomFilter: bloomData,
		Metadata: map[string]interface{}{
			"version":     "1.0",
			"created_by":  "distributed-bloom-filter",
			"bloom_size":  sm.bloomFilter.Size(),
			"bloom_k":     sm.bloomFilter.HashCount(),
		},
		Timestamp: time.Now(),
		Index:     index,
		Term:      term,
	}

	// Serialize snapshot data
	data, err := json.Marshal(snapshotData)
	if err != nil {
		return err
	}

	// Update statistics
	sm.stats.TotalSnapshots++
	sm.stats.TotalSnapshotSize += int64(len(data))
	sm.stats.LastSnapshotIndex = index
	sm.stats.LastSnapshotTerm = term
	sm.stats.LastSnapshotTime = time.Now()
	sm.stats.AverageSnapshotSize = sm.stats.TotalSnapshotSize / sm.stats.TotalSnapshots

	sm.lastSnapshotIndex = index
	sm.lastSnapshotTerm = term
	sm.lastSnapshotTime = time.Now()

	log.Printf("Created snapshot at index %d, term %d, size %d bytes", index, term, len(data))

	return nil
}

// SaveSnapshot saves a snapshot with encryption and checksum.
// This implements the encryption workflow:
// 1. Calculate SHA-256 checksum
// 2. Encrypt data using AES-256-GCM
// 3. Write to file with checksum prefix
func (sm *SnapshotManager) SaveSnapshot(data []byte) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.encryptor == nil {
		// No encryption, save as-is
		return sm.saveSnapshotUnencrypted(data)
	}

	// Step 1: Calculate SHA-256 checksum
	checksum := sha256.Sum256(data)
	checksumHex := hex.EncodeToString(checksum[:])

	// Step 2: Encrypt data using AES-256-GCM
	encryptedData, err := sm.encryptor.Encrypt(data)
	if err != nil {
		return fmt.Errorf("failed to encrypt snapshot: %w", err)
	}

	// Step 3: Create snapshot file with checksum
	// Format: [checksum (64 bytes hex)][encrypted data]
	checksumBytes := []byte(checksumHex + "\n")
	fileData := append(checksumBytes, encryptedData...)

	// Write to file
	if err := sm.writeSnapshotFile(fileData); err != nil {
		return err
	}

	log.Printf("Saved encrypted snapshot with checksum %s (%d bytes)", 
		checksumHex[:16], len(fileData))

	return nil
}

// saveSnapshotUnencrypted saves a snapshot without encryption (fallback).
func (sm *SnapshotManager) saveSnapshotUnencrypted(data []byte) error {
	if sm.snapshotDir == "" {
		return ErrSnapshotDirNotConfigured
	}

	// Ensure directory exists
	if err := os.MkdirAll(sm.snapshotDir, 0755); err != nil {
		return err
	}

	// Write to file
	filename := fmt.Sprintf("snapshot_%d_%d.json", 
		sm.lastSnapshotIndex, time.Now().UnixNano())
	filepath := filepath.Join(sm.snapshotDir, filename)

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return err
	}

	log.Printf("Saved unencrypted snapshot to %s (%d bytes)", filepath, len(data))
	return nil
}

// writeSnapshotFile writes snapshot data to a file.
func (sm *SnapshotManager) writeSnapshotFile(data []byte) error {
	if sm.snapshotDir == "" {
		return ErrSnapshotDirNotConfigured
	}

	// Ensure directory exists
	if err := os.MkdirAll(sm.snapshotDir, 0755); err != nil {
		return err
	}

	// Write to file
	filename := fmt.Sprintf("snapshot_%d_%d.enc", 
		sm.lastSnapshotIndex, time.Now().UnixNano())
	filepath := filepath.Join(sm.snapshotDir, filename)

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return err
	}

	return nil
}

// LoadSnapshot loads a snapshot with decryption and verification.
// This implements the decryption workflow:
// 1. Read file
// 2. Parse checksum and encrypted data
// 3. Decrypt data
// 4. Verify checksum
func (sm *SnapshotManager) LoadSnapshot() ([]byte, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.encryptor == nil {
		// No encryption, load as-is
		return sm.loadSnapshotUnencrypted()
	}

	// Step 1: Read latest snapshot file
	fileData, err := sm.readLatestSnapshotFile()
	if err != nil {
		return nil, err
	}

	// Step 2: Parse checksum and encrypted data
	// Format: [checksum (64 bytes hex)][newline][encrypted data]
	newlineIdx := bytes.Index(fileData, []byte("\n"))
	if newlineIdx == -1 || newlineIdx != 64 {
		return nil, fmt.Errorf("invalid snapshot format: missing checksum separator")
	}

	checksumHex := string(fileData[:64])
	encryptedData := fileData[65:]

	// Step 3: Decrypt data
	decryptedData, err := sm.encryptor.Decrypt(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt snapshot: %w", err)
	}

	// Step 4: Verify checksum
	actualChecksum := sha256.Sum256(decryptedData)
	actualChecksumHex := hex.EncodeToString(actualChecksum[:])

	if actualChecksumHex != checksumHex {
		return nil, fmt.Errorf("checksum verification failed: expected %s, got %s", 
			checksumHex, actualChecksumHex)
	}

	log.Printf("Loaded and verified encrypted snapshot (%d bytes)", len(decryptedData))
	return decryptedData, nil
}

// loadSnapshotUnencrypted loads a snapshot without encryption (fallback).
func (sm *SnapshotManager) loadSnapshotUnencrypted() ([]byte, error) {
	if sm.snapshotDir == "" {
		return nil, ErrSnapshotDirNotConfigured
	}

	// Find latest snapshot file
	files, err := filepath.Glob(filepath.Join(sm.snapshotDir, "snapshot_*.json"))
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, ErrNoSnapshotFound
	}

	// Use the latest file (sorted by name which includes timestamp)
	latestFile := files[len(files)-1]

	data, err := os.ReadFile(latestFile)
	if err != nil {
		return nil, err
	}

	log.Printf("Loaded unencrypted snapshot from %s (%d bytes)", latestFile, len(data))
	return data, nil
}

// readLatestSnapshotFile reads the latest encrypted snapshot file.
func (sm *SnapshotManager) readLatestSnapshotFile() ([]byte, error) {
	if sm.snapshotDir == "" {
		return nil, ErrSnapshotDirNotConfigured
	}

	// Find latest snapshot file
	files, err := filepath.Glob(filepath.Join(sm.snapshotDir, "snapshot_*.enc"))
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, ErrNoSnapshotFound
	}

	// Use the latest file (sorted by name which includes timestamp)
	latestFile := files[len(files)-1]

	data, err := os.ReadFile(latestFile)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// GetSnapshot creates a snapshot for Raft.
// This implements the raft.FSMSnapshot interface.
func (sm *SnapshotManager) GetSnapshot() (raft.FSMSnapshot, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.bloomFilter == nil {
		return nil, ErrBloomFilterNotInitialized
	}

	// Serialize the Bloom filter
	bloomData := sm.bloomFilter.Serialize()

	return &fsmSnapshot{
		bloomData: bloomData,
		metadata: map[string]interface{}{
			"timestamp": time.Now().UnixNano(),
			"size":      len(bloomData),
		},
	}, nil
}

// RestoreSnapshot restores from a Raft snapshot.
func (sm *SnapshotManager) RestoreSnapshot(rc io.ReadCloser) error {
	defer rc.Close()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.bloomFilter == nil {
		return ErrBloomFilterNotInitialized
	}

	// Read the snapshot data
	data, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("failed to read snapshot: %w", err)
	}

	// Try to deserialize as SnapshotData first (new format)
	var snapshotData SnapshotData
	if err := json.Unmarshal(data, &snapshotData); err == nil {
		// New format with metadata
		newFilter, err := bloom.Deserialize(snapshotData.BloomFilter)
		if err != nil {
			return fmt.Errorf("failed to restore bloom filter: %w", err)
		}

		sm.bloomFilter = newFilter
		sm.lastSnapshotIndex = snapshotData.Index
		sm.lastSnapshotTerm = snapshotData.Term
		sm.lastSnapshotTime = snapshotData.Timestamp

		sm.stats.TotalRestores++

		log.Printf("Restored snapshot from index %d, term %d (%d bytes)", 
			snapshotData.Index, snapshotData.Term, len(data))
		return nil
	}

	// Old format: raw Bloom filter data
	newFilter, err := bloom.Deserialize(data)
	if err != nil {
		return fmt.Errorf("failed to restore bloom filter: %w", err)
	}

	sm.bloomFilter = newFilter
	sm.stats.TotalRestores++

	log.Printf("Restored snapshot (%d bytes)", len(data))
	return nil
}

// GetLastSnapshotIndex returns the last snapshot index.
func (sm *SnapshotManager) GetLastSnapshotIndex() uint64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastSnapshotIndex
}

// GetLastSnapshotTerm returns the last snapshot term.
func (sm *SnapshotManager) GetLastSnapshotTerm() uint64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastSnapshotTerm
}

// GetLastSnapshotTime returns the last snapshot time.
func (sm *SnapshotManager) GetLastSnapshotTime() time.Time {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastSnapshotTime
}

// RestoreFromFSM updates snapshot manager state after FSM restore.
// This is called after Restore() to keep snapshot manager in sync with FSM.
func (sm *SnapshotManager) RestoreFromFSM(index, term uint64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.lastSnapshotIndex = index
	sm.lastSnapshotTerm = term
	sm.lastSnapshotTime = time.Now()
}

// GetStats returns a copy of the snapshot statistics.
func (sm *SnapshotManager) GetStats() SnapshotStats {
	if sm == nil || sm.stats == nil {
		return SnapshotStats{}
	}
	sm.mu.RLock()
	stats := *sm.stats
	sm.mu.RUnlock()
	return stats
}

// GetSnapshotInfo returns information about available snapshots.
func (sm *SnapshotManager) GetSnapshotInfo() ([]map[string]interface{}, error) {
	sm.mu.RLock()
	snapshotStore := sm.snapshotStore
	sm.mu.RUnlock()

	if snapshotStore == nil {
		return nil, ErrSnapshotStoreNotInitialized
	}

	// List snapshots (only supported by FileSnapshotStore)
	fileStore, ok := snapshotStore.(*raft.FileSnapshotStore)
	if !ok {
		return nil, nil // In-memory store doesn't support listing
	}
	snapshots, err := fileStore.List()
	if err != nil {
		return nil, err
	}

	info := make([]map[string]interface{}, 0, len(snapshots))
	for _, snapshot := range snapshots {
		info = append(info, map[string]interface{}{
			"id":    snapshot.ID,
			"index": snapshot.Index,
			"term":  snapshot.Term,
			"size":  snapshot.Size,
			// Note: HashiCorp Raft SnapshotMeta uses Timestamp field
		})
	}

	return info, nil
}

// DeleteOldSnapshots deletes snapshots older than the specified retention period.
// Note: HashiCorp Raft FileSnapshotStore doesn't expose a Delete method directly.
// This is a placeholder for future implementation.
func (sm *SnapshotManager) DeleteOldSnapshots(retainCount int) error {
	sm.mu.RLock()
	snapshotStore := sm.snapshotStore
	sm.mu.RUnlock()

	if snapshotStore == nil {
		return ErrSnapshotStoreNotInitialized
	}

	// List snapshots (only supported by FileSnapshotStore)
	fileStore, ok := snapshotStore.(*raft.FileSnapshotStore)
	if !ok {
		return nil // In-memory store doesn't need cleanup
	}
	snapshots, err := fileStore.List()
	if err != nil {
		return err
	}

	// Note: HashiCorp Raft doesn't expose Delete method on FileSnapshotStore
	// Old snapshots are automatically cleaned up based on retention settings
	log.Printf("Snapshot cleanup: %d snapshots found, retaining %d", len(snapshots), retainCount)

	return nil
}

// GetStatus returns comprehensive snapshot status.
func (sm *SnapshotManager) GetStatus() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return map[string]interface{}{
		"last_snapshot_index": sm.lastSnapshotIndex,
		"last_snapshot_term":  sm.lastSnapshotTerm,
		"last_snapshot_time":  sm.lastSnapshotTime.String(),
		"total_snapshots":     sm.stats.TotalSnapshots,
		"total_restores":      sm.stats.TotalRestores,
		"total_snapshot_size": sm.stats.TotalSnapshotSize,
		"average_snapshot_size": sm.stats.AverageSnapshotSize,
	}
}

// SaveSnapshotToFile saves a snapshot to a file for backup purposes.
func (sm *SnapshotManager) SaveSnapshotToFile(filePath string) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if sm.bloomFilter == nil {
		return ErrBloomFilterNotInitialized
	}

	// Create snapshot data
	snapshotData := &SnapshotData{
		BloomFilter: sm.bloomFilter.Serialize(),
		Metadata: map[string]interface{}{
			"version":    "1.0",
			"created_by": "distributed-bloom-filter",
		},
		Timestamp: time.Now(),
		Index:     sm.lastSnapshotIndex,
		Term:      sm.lastSnapshotTerm,
	}

	// Serialize
	data, err := json.MarshalIndent(snapshotData, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	log.Printf("Saved snapshot to %s (%d bytes)", filePath, len(data))
	return nil
}

// LoadSnapshotFromFile loads a snapshot from a file.
func (sm *SnapshotManager) LoadSnapshotFromFile(filePath string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.bloomFilter == nil {
		return ErrBloomFilterNotInitialized
	}

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Deserialize
	var snapshotData SnapshotData
	if err := json.Unmarshal(data, &snapshotData); err != nil {
		return err
	}

	// Restore Bloom filter
	newFilter, err := bloom.Deserialize(snapshotData.BloomFilter)
	if err != nil {
		return err
	}

	sm.bloomFilter = newFilter
	sm.lastSnapshotIndex = snapshotData.Index
	sm.lastSnapshotTerm = snapshotData.Term
	sm.lastSnapshotTime = snapshotData.Timestamp

	log.Printf("Loaded snapshot from %s", filePath)
	return nil
}

// Errors for snapshot management.
var (
	ErrBloomFilterNotInitialized   = errors.New("bloom filter not initialized")
	ErrSnapshotStoreNotInitialized = errors.New("snapshot store not initialized")
	ErrSnapshotDirNotConfigured    = errors.New("snapshot directory not configured")
	ErrNoSnapshotFound             = errors.New("no snapshot found")
)
