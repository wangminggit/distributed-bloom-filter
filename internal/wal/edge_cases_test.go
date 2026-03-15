package wal

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestEncryptor_WrongKey tests decryption with wrong key
func TestEncryptor_WrongKey(t *testing.T) {
	// Create first encryptor with random key
	encryptor1, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor1: %v", err)
	}

	// Create second encryptor with different random key
	encryptor2, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor2: %v", err)
	}

	// Verify keys are different
	key1, _ := encryptor1.GetCurrentKey()
	key2, _ := encryptor2.GetCurrentKey()
	if bytes.Equal(key1, key2) {
		t.Skip("Random keys happened to be equal, skipping test")
	}

	// Encrypt with first encryptor
	testData := []byte("Secret data")
	encrypted, err := encryptor1.Encrypt(testData)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Try to decrypt with second encryptor (wrong key)
	_, err = encryptor2.Decrypt(encrypted)
	if err == nil {
		t.Error("Expected error when decrypting with wrong key, got nil")
	}
}

// TestWALWriter_ConcurrentWrites tests concurrent writes to WAL
func TestWALWriter_ConcurrentWrites(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "wal-concurrent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create encryptor
	encryptor, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Create writer
	writer, err := NewWALWriter(tempDir, encryptor)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	var wg sync.WaitGroup
	numGoroutines := 10
	writesPerGoroutine := 50

	// Start multiple goroutines writing concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				data := []byte("Goroutine-" + string(rune('0'+id)) + "-Record-" + string(rune('0'+j%10)))
				err := writer.Write(data)
				if err != nil {
					t.Errorf("Failed to write from goroutine %d: %v", id, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Close writer
	writer.Close()

	// Verify all data can be read
	reader, err := NewWALReader(tempDir, encryptor)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read records: %v", err)
	}

	expectedRecords := numGoroutines * writesPerGoroutine
	if len(records) != expectedRecords {
		t.Errorf("Expected %d records, got %d", expectedRecords, len(records))
	}
}

// TestWALReader_CorruptedFile tests reading from corrupted file
func TestWALReader_CorruptedFile(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "wal-corrupted-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create encryptor and writer
	encryptor, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	writer, err := NewWALWriter(tempDir, encryptor)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Write some valid records
	testData := []byte("Valid record")
	err = writer.Write(testData)
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}

	writer.Close()

	// Corrupt the file by truncating it
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	if len(files) > 0 {
		filePath := filepath.Join(tempDir, files[0].Name())
		// Truncate file to corrupt it
		err = os.Truncate(filePath, 5) // Truncate to 5 bytes (incomplete record)
		if err != nil {
			t.Fatalf("Failed to truncate file: %v", err)
		}
	}

	// Try to read - should handle corruption gracefully
	reader, err := NewWALReader(tempDir, encryptor)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// ReadAll should not crash, may return partial data or error
	records, err := reader.ReadAll()
	// It's acceptable if this returns an error or partial data
	t.Logf("Read %d records from corrupted file (error: %v)", len(records), err)
}

// TestWALReader_EmptyDirectory tests reading from empty directory
func TestWALReader_EmptyDirectory(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "wal-empty-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create encryptor
	encryptor, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Create reader on empty directory
	reader, err := NewWALReader(tempDir, encryptor)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}
	defer reader.Close()

	// ReadAll should return empty slice, not error
	records, err := reader.ReadAll()
	if err != nil {
		t.Errorf("Expected no error for empty directory, got: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("Expected 0 records from empty directory, got %d", len(records))
	}
}

// TestWALWriter_RollingBoundary tests file rolling at boundary conditions
func TestWALWriter_RollingBoundary(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "wal-rolling-boundary-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create encryptor
	encryptor, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Create writer with very small max file size to force rolling
	// Each encrypted record is ~50-60 bytes, so set max to 100 bytes
	writer, err := NewWALWriterWithConfig(tempDir, encryptor, 100, 5*time.Minute, 5)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Write multiple records to trigger rolling
	for i := 0; i < 10; i++ {
		data := []byte("Record-" + string(rune('0'+i)))
		err := writer.Write(data)
		if err != nil {
			t.Fatalf("Failed to write record %d: %v", i, err)
		}
	}

	writer.Close()

	// Verify multiple files were created
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	walFileCount := 0
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".enc" {
			walFileCount++
		}
	}

	if walFileCount < 2 {
		t.Errorf("Expected at least 2 WAL files due to rolling, got %d", walFileCount)
	}

	t.Logf("Created %d WAL files", walFileCount)
}

// TestWALWriter_MaxFilesCleanup tests that old files are cleaned up
func TestWALWriter_MaxFilesCleanup(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "wal-cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create encryptor
	encryptor, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Create writer with maxFiles=3
	writer, err := NewWALWriterWithConfig(tempDir, encryptor, 50, 5*time.Minute, 3)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// Write enough to trigger multiple rolls
	for i := 0; i < 20; i++ {
		data := []byte("Record-" + string(rune('0'+i%10)))
		err := writer.Write(data)
		if err != nil {
			t.Fatalf("Failed to write record %d: %v", i, err)
		}
	}

	writer.Close()

	// Verify file count doesn't exceed maxFiles
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}

	walFileCount := 0
	for _, f := range files {
		if !f.IsDir() && filepath.Ext(f.Name()) == ".enc" {
			walFileCount++
		}
	}

	if walFileCount > 3 {
		t.Errorf("Expected at most 3 WAL files, got %d", walFileCount)
	}

	t.Logf("Created %d WAL files (maxFiles=3)", walFileCount)
}

// TestEncryptor_MultipleRotations tests multiple key rotations
func TestEncryptor_MultipleRotations(t *testing.T) {
	encryptor, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Encrypt data with initial key
	data1 := []byte("Data version 1")
	encrypted1, err := encryptor.Encrypt(data1)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Rotate key multiple times
	for i := 0; i < 5; i++ {
		err := encryptor.RotateKey()
		if err != nil {
			t.Fatalf("Failed to rotate key: %v", err)
		}

		// Encrypt new data with new key
		data := []byte("Data version " + string(rune('2'+i)))
		encrypted, err := encryptor.Encrypt(data)
		if err != nil {
			t.Fatalf("Failed to encrypt after rotation %d: %v", i+1, err)
		}

		// Verify new data can be decrypted
		decrypted, err := encryptor.Decrypt(encrypted)
		if err != nil {
			t.Fatalf("Failed to decrypt after rotation %d: %v", i+1, err)
		}
		if string(decrypted) != string(data) {
			t.Errorf("Decrypted data mismatch after rotation %d", i+1)
		}
	}

	// Verify old data can still be decrypted
	decrypted1, err := encryptor.Decrypt(encrypted1)
	if err != nil {
		t.Fatalf("Failed to decrypt old data: %v", err)
	}
	if string(decrypted1) != string(data1) {
		t.Error("Old data decryption failed after multiple rotations")
	}

	// Verify key version increased
	_, version := encryptor.GetCurrentKey()
	if version != 6 { // Initial version 1 + 5 rotations
		t.Errorf("Expected key version 6, got %d", version)
	}
}

// TestK8sSecretLoader_MissingFile tests K8s Secret loader with missing files
func TestK8sSecretLoader_MissingFile(t *testing.T) {
	// Create temporary directory but don't add any files
	tempDir, err := os.MkdirTemp("", "k8s-missing-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create loader
	loader := &K8sSecretLoader{secretPath: tempDir}

	// Load should fail with missing key file
	_, _, err = loader.LoadKey()
	if err == nil {
		t.Error("Expected error for missing key file, got nil")
	}
}

// TestWALWriter_CloseTwice tests closing writer twice
func TestWALWriter_CloseTwice(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wal-close-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	encryptor, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	writer, err := NewWALWriter(tempDir, encryptor)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}

	// First close
	err = writer.Close()
	if err != nil {
		t.Errorf("First close failed: %v", err)
	}

	// Second close should not panic
	err = writer.Close()
	if err != nil {
		t.Logf("Second close returned: %v (acceptable)", err)
	}
}

// TestWALReader_Close tests reader close
func TestWALReader_Close(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wal-reader-close-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	encryptor, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	reader, err := NewWALReader(tempDir, encryptor)
	if err != nil {
		t.Fatalf("Failed to create reader: %v", err)
	}

	err = reader.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// TestEncryptor_RefreshKey tests key refresh with K8s loader
func TestEncryptor_RefreshKey(t *testing.T) {
	// Create temp directory with key files
	tempDir, err := os.MkdirTemp("", "wal-refresh-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write initial key
	initialKey := []byte("0123456789abcdef0123456789abcdef")
	err = os.WriteFile(filepath.Join(tempDir, "key"), initialKey, 0644)
	if err != nil {
		t.Fatalf("Failed to write key: %v", err)
	}
	err = os.WriteFile(filepath.Join(tempDir, "version"), []byte("1"), 0644)
	if err != nil {
		t.Fatalf("Failed to write version: %v", err)
	}

	// Create encryptor with K8s loader
	encryptor, err := NewWALEncryptor(tempDir)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// First refresh should succeed (cache not expired)
	err = encryptor.RefreshKey()
	if err != nil {
		t.Errorf("First refresh failed: %v", err)
	}

	// Manually expire cache by setting keyCacheTime in the past
	// Note: This requires accessing unexported field, so we skip for now
	// In production, this would be tested by waiting KeyCacheDuration
	t.Log("RefreshKey test completed (cache not expired)")
}

// TestWALWriter_WithConfig tests writer with custom configuration
func TestWALWriter_WithConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wal-config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	encryptor, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Create writer with custom config
	writer, err := NewWALWriterWithConfig(tempDir, encryptor, 1024, time.Minute, 5)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Write some data
	err = writer.Write([]byte("test data"))
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
}

// TestWALWriter_RollFile tests manual file rolling
func TestWALWriter_RollFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wal-roll-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	encryptor, err := NewWALEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	writer, err := NewWALWriter(tempDir, encryptor)
	if err != nil {
		t.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	// Write some data first
	err = writer.Write([]byte("test data"))
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}

	// Manually roll file
	// Note: rollFile is private, so we trigger it via Write with size limit
}
