package metadata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Service manages metadata for the distributed Bloom filter.
type Service struct {
	dataDir  string
	metadata *Metadata
	mu       sync.RWMutex
}

// Metadata contains all metadata for the Bloom filter service.
type Metadata struct {
	NodeID       string                 `json:"node_id"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	Version      string                 `json:"version"`
	ClusterNodes []string               `json:"cluster_nodes"`
	Config       map[string]interface{} `json:"config"`
	Stats        *Stats                 `json:"stats"`
}

// Stats contains operational statistics.
type Stats struct {
	TotalAdds      int64     `json:"total_adds"`
	TotalRemoves   int64     `json:"total_removes"`
	TotalQueries   int64     `json:"total_queries"`
	LastBackup     time.Time `json:"last_backup"`
	LastCompaction time.Time `json:"last_compaction"`
}

// NewService creates a new metadata service.
func NewService(dataDir string) *Service {
	s := &Service{
		dataDir: dataDir,
		metadata: &Metadata{
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
			Version:      "1.0.0",
			ClusterNodes: make([]string, 0),
			Config:       make(map[string]interface{}),
			Stats: &Stats{
				TotalAdds:    0,
				TotalRemoves: 0,
				TotalQueries: 0,
			},
		},
	}

	// Try to load existing metadata
	if err := s.Load(); err != nil {
		// If file doesn't exist, that's fine - we'll create it on first save
		if !os.IsNotExist(err) {
			fmt.Printf("Warning: failed to load metadata: %v\n", err)
		}
	}

	return s
}

// Load loads metadata from disk.
func (s *Service) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	metadataPath := filepath.Join(s.dataDir, "metadata.json")

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return err
	}

	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	s.metadata = &metadata
	return nil
}

// Save saves metadata to disk.
func (s *Service) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metadata.UpdatedAt = time.Now()

	metadataPath := filepath.Join(s.dataDir, "metadata.json")

	// Ensure directory exists
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(s.metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// SetNodeID sets the node ID.
func (s *Service) SetNodeID(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metadata.NodeID = nodeID
	s.metadata.UpdatedAt = time.Now()
	return nil
}

// GetNodeID returns the node ID.
func (s *Service) GetNodeID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.metadata.NodeID
}

// AddClusterNode adds a node to the cluster.
func (s *Service) AddClusterNode(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already exists
	for _, n := range s.metadata.ClusterNodes {
		if n == nodeID {
			return nil
		}
	}

	s.metadata.ClusterNodes = append(s.metadata.ClusterNodes, nodeID)
	s.metadata.UpdatedAt = time.Now()
	return nil
}

// RemoveClusterNode removes a node from the cluster.
func (s *Service) RemoveClusterNode(nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	nodes := make([]string, 0)
	for _, n := range s.metadata.ClusterNodes {
		if n != nodeID {
			nodes = append(nodes, n)
		}
	}

	s.metadata.ClusterNodes = nodes
	s.metadata.UpdatedAt = time.Now()
	return nil
}

// GetClusterNodes returns all cluster nodes.
func (s *Service) GetClusterNodes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]string, len(s.metadata.ClusterNodes))
	copy(nodes, s.metadata.ClusterNodes)
	return nodes
}

// SetConfig sets a configuration value.
func (s *Service) SetConfig(key string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metadata.Config[key] = value
	s.metadata.UpdatedAt = time.Now()
	return nil
}

// GetConfig gets a configuration value.
func (s *Service) GetConfig(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.metadata.Config[key]
	return value, ok
}

// RecordAdd records an add operation in stats.
func (s *Service) RecordAdd() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metadata.Stats.TotalAdds++
}

// RecordRemove records a remove operation in stats.
func (s *Service) RecordRemove() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metadata.Stats.TotalRemoves++
}

// RecordQuery records a query operation in stats.
func (s *Service) RecordQuery() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metadata.Stats.TotalQueries++
}

// GetStats returns a copy of the current stats.
func (s *Service) GetStats() *Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &Stats{
		TotalAdds:      s.metadata.Stats.TotalAdds,
		TotalRemoves:   s.metadata.Stats.TotalRemoves,
		TotalQueries:   s.metadata.Stats.TotalQueries,
		LastBackup:     s.metadata.Stats.LastBackup,
		LastCompaction: s.metadata.Stats.LastCompaction,
	}
}

// SetLastBackup updates the last backup timestamp.
func (s *Service) SetLastBackup(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metadata.Stats.LastBackup = t
}

// SetLastCompaction updates the last compaction timestamp.
func (s *Service) SetLastCompaction(t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.metadata.Stats.LastCompaction = t
}

// GetMetadata returns a copy of the full metadata.
func (s *Service) GetMetadata() *Metadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a deep copy
	metadataCopy := *s.metadata
	if s.metadata.Stats != nil {
		statsCopy := *s.metadata.Stats
		metadataCopy.Stats = &statsCopy
	}
	configCopy := make(map[string]interface{})
	for k, v := range s.metadata.Config {
		configCopy[k] = v
	}
	metadataCopy.Config = configCopy
	nodesCopy := make([]string, len(s.metadata.ClusterNodes))
	copy(nodesCopy, s.metadata.ClusterNodes)
	metadataCopy.ClusterNodes = nodesCopy

	return &metadataCopy
}
