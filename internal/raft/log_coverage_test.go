package raft

import (
	"testing"
	"time"
)

// TestLogManagerBasic tests basic LogManager operations.
func TestLogManagerBasic(t *testing.T) {
	lm := NewLogManager()
	
	if lm == nil {
		t.Fatal("Expected non-nil log manager")
	}
	
	if lm.pendingCommands == nil {
		t.Error("Expected pendingCommands to be initialized")
	}
	
	if lm.stats == nil {
		t.Error("Expected stats to be initialized")
	}
	
	if len(lm.pendingCommands) != 0 {
		t.Errorf("Expected empty pendingCommands, got %d", len(lm.pendingCommands))
	}
	
	t.Log("LogManager created successfully")
}

// TestLogManagerSetRaftNode tests SetRaftNode method.
func TestLogManagerSetRaftNode(t *testing.T) {
	lm := NewLogManager()
	
	// Set nil raft node (for testing without full setup)
	lm.SetRaftNode(nil)
	
	// Should not panic
	t.Log("SetRaftNode completed without panic")
}

// TestLogManagerApplyCommandNoNode tests ApplyCommand when no raft node is set.
func TestLogManagerApplyCommandNoNode(t *testing.T) {
	lm := NewLogManager()
	
	cmd := NewCommand("add", []byte("test-item"))
	
	err := lm.ApplyCommand(cmd, 1*time.Second)
	
	if err != ErrRaftNotStarted {
		t.Errorf("Expected ErrRaftNotStarted, got %v", err)
	}
	
	t.Log("ApplyCommand correctly rejected command without raft node")
}

// TestLogManagerAddItemNoNode tests AddItem when no raft node is set.
func TestLogManagerAddItemNoNode(t *testing.T) {
	lm := NewLogManager()
	
	err := lm.AddItem([]byte("test-item"), 1*time.Second)
	
	if err != ErrRaftNotStarted {
		t.Errorf("Expected ErrRaftNotStarted, got %v", err)
	}
	
	t.Log("AddItem correctly rejected item without raft node")
}

// TestLogManagerRemoveItemNoNode tests RemoveItem when no raft node is set.
func TestLogManagerRemoveItemNoNode(t *testing.T) {
	lm := NewLogManager()
	
	err := lm.RemoveItem([]byte("test-item"), 1*time.Second)
	
	if err != ErrRaftNotStarted {
		t.Errorf("Expected ErrRaftNotStarted, got %v", err)
	}
	
	t.Log("RemoveItem correctly rejected item without raft node")
}

// TestLogManagerGetLastIndexNoNode tests GetLastIndex when no raft node is set.
func TestLogManagerGetLastIndexNoNode(t *testing.T) {
	lm := NewLogManager()
	
	index, err := lm.GetLastIndex()
	
	if err != ErrRaftNotStarted {
		t.Errorf("Expected ErrRaftNotStarted, got %v", err)
	}
	
	if index != 0 {
		t.Errorf("Expected index 0, got %d", index)
	}
	
	t.Log("GetLastIndex returned expected error")
}

// TestLogManagerGetFirstIndexNoNode tests GetFirstIndex when no raft node is set.
func TestLogManagerGetFirstIndexNoNode(t *testing.T) {
	lm := NewLogManager()
	
	index, err := lm.GetFirstIndex()
	
	if err != ErrRaftNotStarted {
		t.Errorf("Expected ErrRaftNotStarted, got %v", err)
	}
	
	if index != 0 {
		t.Errorf("Expected index 0, got %d", index)
	}
	
	t.Log("GetFirstIndex returned expected error")
}

// TestLogManagerGetStats tests GetStats method.
func TestLogManagerGetStats(t *testing.T) {
	lm := NewLogManager()
	
	stats := lm.GetStats()
	
	// stats is a value type, so it won't be nil
	if stats.TotalCommands != 0 {
		t.Errorf("Expected 0 total commands, got %d", stats.TotalCommands)
	}
	
	t.Logf("Log stats: %v", stats)
}

// TestLogStats tests LogStats structure.
func TestLogStats(t *testing.T) {
	stats := &LogStats{
		TotalCommands:    100,
		TotalAppends:     95,
		TotalCommits:     90,
		TotalFailures:    5,
		LastCommandIndex: 1000,
	}
	
	if stats.TotalCommands != 100 {
		t.Errorf("Expected TotalCommands=100, got %d", stats.TotalCommands)
	}
	
	if stats.TotalFailures != 5 {
		t.Errorf("Expected TotalFailures=5, got %d", stats.TotalFailures)
	}
	
	t.Log("LogStats structure test completed")
}

// TestNewCommand tests NewCommand function.
func TestNewCommand(t *testing.T) {
	cmd := NewCommand("add", []byte("test-item"))
	
	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}
	
	if cmd.Type != "add" {
		t.Errorf("Expected Type=add, got %s", cmd.Type)
	}
	
	if string(cmd.Item) != "test-item" {
		t.Errorf("Expected Item=test-item, got %s", string(cmd.Item))
	}
	
	if cmd.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
	
	t.Log("NewCommand test completed successfully")
}

// TestCommandMarshal tests Command.Marshal method.
func TestCommandMarshal(t *testing.T) {
	cmd := NewCommand("add", []byte("test-item"))
	
	data, err := cmd.Marshal()
	
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	
	if len(data) == 0 {
		t.Error("Expected non-empty marshaled data")
	}
	
	t.Logf("Marshaled command: %d bytes", len(data))
}

// TestCommandMarshalUnmarshalRoundTrip tests Marshal/Unmarshal round-trip.
func TestCommandMarshalUnmarshalRoundTrip(t *testing.T) {
	original := NewCommand("remove", []byte("remove-item"))
	
	// Marshal
	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	
	// Unmarshal
	unmarshaled, err := UnmarshalCommand(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	
	if unmarshaled.Type != original.Type {
		t.Errorf("Expected Type=%s, got %s", original.Type, unmarshaled.Type)
	}
	
	if string(unmarshaled.Item) != string(original.Item) {
		t.Errorf("Expected Item=%s, got %s", string(original.Item), string(unmarshaled.Item))
	}
	
	t.Log("Marshal/Unmarshal round-trip test completed successfully")
}

// TestUnmarshalCommandInvalidData tests UnmarshalCommand with invalid data.
func TestUnmarshalCommandInvalidData(t *testing.T) {
	invalidData := []byte("this is not valid JSON")
	
	_, err := UnmarshalCommand(invalidData)
	
	if err == nil {
		t.Error("Expected error when unmarshaling invalid data")
	}
	
	t.Logf("UnmarshalCommand correctly rejected invalid data: %v", err)
}

// TestUnmarshalCommandEmptyData tests UnmarshalCommand with empty data.
func TestUnmarshalCommandEmptyData(t *testing.T) {
	_, err := UnmarshalCommand([]byte{})
	
	if err == nil {
		t.Error("Expected error when unmarshaling empty data")
	}
	
	t.Logf("UnmarshalCommand correctly rejected empty data: %v", err)
}



// TestCommandDifferentTypes tests commands with different types.
func TestCommandDifferentTypes(t *testing.T) {
	tests := []struct {
		name string
		cmdType string
		item    []byte
	}{
		{"Add", "add", []byte("item1")},
		{"Remove", "remove", []byte("item2")},
		{"BatchAdd", "batch_add", []byte("batch-data")},
		{"Custom", "custom_op", []byte("data")},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewCommand(tt.cmdType, tt.item)
			
			if cmd.Type != tt.cmdType {
				t.Errorf("Expected Type=%s, got %s", tt.cmdType, cmd.Type)
			}
		})
	}
}

// TestLogType tests LogType constants.
func TestLogType(t *testing.T) {
	if LogCommand != 0 {
		t.Errorf("Expected LogCommand=0, got %d", LogCommand)
	}
	
	if LogNoop != 1 {
		t.Errorf("Expected LogNoop=1, got %d", LogNoop)
	}
	
	t.Log("LogType constants test completed")
}

// TestPendingCommand tests pendingCommand structure.
func TestPendingCommand(t *testing.T) {
	cmd := NewCommand("add", []byte("test"))
	done := make(chan error, 1)
	
	pc := &pendingCommand{
		cmd:       cmd,
		timestamp: time.Now(),
		done:      done,
	}
	
	if pc.cmd == nil {
		t.Error("Expected non-nil command")
	}
	
	if pc.done == nil {
		t.Error("Expected non-nil done channel")
	}
	
	t.Log("pendingCommand structure test completed")
}
