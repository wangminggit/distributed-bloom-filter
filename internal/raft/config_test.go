package raft

import (
	"testing"
	"time"
)

// TestConfig_Validate_Basic tests basic config validation.
func TestConfig_Validate_Basic(t *testing.T) {
	// Valid config
	validConfig := &Config{
		NodeID:   "test-node",
		RaftPort: 7000,
		DataDir:  "/tmp/test",
	}

	err := validConfig.Validate()
	if err != nil {
		t.Errorf("Valid config should not error: %v", err)
	}
}

// TestConfig_Validate_EmptyNodeID tests empty node ID validation.
func TestConfig_Validate_EmptyNodeID(t *testing.T) {
	invalidConfig := &Config{
		NodeID:   "",
		RaftPort: 7000,
	}
	err := invalidConfig.Validate()
	if err == nil {
		t.Error("Expected error for empty node ID")
	}
}

// TestConfig_Validate_ZeroPort tests zero port validation.
func TestConfig_Validate_ZeroPort(t *testing.T) {
	invalidConfig := &Config{
		NodeID:   "test",
		RaftPort: 0,
	}
	err := invalidConfig.Validate()
	if err == nil {
		t.Error("Expected error for zero port")
	}
}

// TestConfig_Defaults tests default config values.
func TestConfig_Defaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("Expected non-nil config")
	}
	if cfg.NodeID != "" {
		t.Error("Expected empty default NodeID")
	}
}

// TestConfig_Validate_TimeoutRelationship tests timeout relationships.
func TestConfig_Validate_TimeoutRelationship(t *testing.T) {
	config := &Config{
		NodeID:           "test",
		RaftPort:         7000,
		HeartbeatTimeout: time.Second * 5,
		ElectionTimeout:  time.Second, // Too short
	}
	err := config.Validate()
	if err == nil {
		t.Error("Expected error when election timeout < heartbeat timeout")
	}
}
