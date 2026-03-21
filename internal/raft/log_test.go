package raft

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCommand(t *testing.T) {
	t.Run("NewCommand", func(t *testing.T) {
		cmd := NewCommand("add", []byte("test-item"))
		
		if cmd.Type != "add" {
			t.Errorf("Expected type 'add', got %s", cmd.Type)
		}
		
		if string(cmd.Item) != "test-item" {
			t.Errorf("Expected item 'test-item', got %s", string(cmd.Item))
		}
		
		if cmd.Timestamp.IsZero() {
			t.Error("Expected timestamp to be set")
		}
		
		if time.Since(cmd.Timestamp) > time.Second {
			t.Error("Expected timestamp to be recent")
		}
	})

	t.Run("Command_Marshal", func(t *testing.T) {
		cmd := NewCommand("remove", []byte("item-to-remove"))
		
		data, err := cmd.Marshal()
		if err != nil {
			t.Fatalf("Failed to marshal command: %v", err)
		}
		
		if len(data) == 0 {
			t.Error("Expected non-empty marshaled data")
		}
		
		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Errorf("Marshaled data is not valid JSON: %v", err)
		}
	})

	t.Run("UnmarshalCommand", func(t *testing.T) {
		original := &Command{
			Type:      "add",
			Item:      []byte("test-item"),
			Timestamp: time.Now(),
		}
		
		data, _ := original.Marshal()
		
		cmd, err := UnmarshalCommand(data)
		if err != nil {
			t.Fatalf("Failed to unmarshal command: %v", err)
		}
		
		if cmd.Type != original.Type {
			t.Errorf("Expected type %s, got %s", original.Type, cmd.Type)
		}
		
		if string(cmd.Item) != string(original.Item) {
			t.Errorf("Expected item %s, got %s", string(original.Item), string(cmd.Item))
		}
	})

	t.Run("UnmarshalCommand_InvalidJSON", func(t *testing.T) {
		_, err := UnmarshalCommand([]byte("invalid json"))
		
		if err == nil {
			t.Error("Expected error for invalid JSON")
		}
	})

	t.Run("UnmarshalCommand_EmptyData", func(t *testing.T) {
		_, err := UnmarshalCommand([]byte{})
		
		if err == nil {
			t.Error("Expected error for empty data")
		}
	})
}

func TestLogManager(t *testing.T) {
	t.Run("NewLogManager", func(t *testing.T) {
		lm := NewLogManager()
		
		if lm == nil {
			t.Fatal("Expected log manager to be created")
		}
		
		if lm.pendingCommands == nil {
			t.Error("Expected pending commands map to be initialized")
		}
		
		if lm.stats == nil {
			t.Error("Expected stats to be initialized")
		}
		
		if lm.raftNode != nil {
			t.Error("Expected raft node to be nil initially")
		}
	})

	t.Run("SetRaftNode", func(t *testing.T) {
		lm := NewLogManager()
		
		lm.SetRaftNode(nil)
		
		if lm.raftNode != nil {
			t.Error("Expected raft node to be nil")
		}
	})

	t.Run("GetStats", func(t *testing.T) {
		lm := NewLogManager()
		
		lm.stats.TotalCommands = 10
		lm.stats.TotalAppends = 8
		lm.stats.TotalCommits = 7
		lm.stats.TotalFailures = 1
		
		stats := lm.GetStats()
		
		if stats.TotalCommands != 10 {
			t.Errorf("Expected 10 total commands, got %d", stats.TotalCommands)
		}
		
		if stats.TotalAppends != 8 {
			t.Errorf("Expected 8 appends, got %d", stats.TotalAppends)
		}
		
		if stats.TotalFailures != 1 {
			t.Errorf("Expected 1 failure, got %d", stats.TotalFailures)
		}
	})

	t.Run("LogStats", func(t *testing.T) {
		stats := &LogStats{}
		
		if stats.TotalCommands != 0 {
			t.Errorf("Expected 0 commands, got %d", stats.TotalCommands)
		}
		
		stats.TotalCommands = 100
		stats.LastCommandIndex = 99
		
		if stats.TotalCommands != 100 {
			t.Errorf("Expected 100 commands, got %d", stats.TotalCommands)
		}
		
		if stats.LastCommandIndex != 99 {
			t.Errorf("Expected last index 99, got %d", stats.LastCommandIndex)
		}
	})
}

func TestLogEntry(t *testing.T) {
	t.Run("LogEntry_Creation", func(t *testing.T) {
		entry := &LogEntry{
			Index:     1,
			Term:      2,
			Type:      LogCommand,
			Data:      []byte("test-data"),
			Timestamp: time.Now(),
		}
		
		if entry.Index != 1 {
			t.Errorf("Expected index 1, got %d", entry.Index)
		}
		
		if entry.Term != 2 {
			t.Errorf("Expected term 2, got %d", entry.Term)
		}
		
		if entry.Type != LogCommand {
			t.Errorf("Expected LogCommand type, got %d", entry.Type)
		}
		
		if string(entry.Data) != "test-data" {
			t.Errorf("Expected test-data, got %s", string(entry.Data))
		}
	})

	t.Run("LogType_Constants", func(t *testing.T) {
		if LogCommand != 0 {
			t.Errorf("Expected LogCommand to be 0, got %d", LogCommand)
		}
		
		if LogNoop != 1 {
			t.Errorf("Expected LogNoop to be 1, got %d", LogNoop)
		}
	})
}

func TestLogManagerConcurrentAccess(t *testing.T) {
	lm := NewLogManager()
	
	done := make(chan bool, 100)
	
	for i := 0; i < 100; i++ {
		go func(id int) {
			lm.GetStats()
			done <- true
		}(i)
	}
	
	for i := 0; i < 100; i++ {
		<-done
	}
	
	t.Log("Concurrent access test passed")
}
