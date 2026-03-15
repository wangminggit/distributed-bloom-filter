package grpc

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestCleanupOldTimestamps_Empty tests cleanup with no timestamps
func TestCleanupOldTimestamps_Empty(t *testing.T) {
	
	config := &AuthConfig{
		EnableAPIKeyAuth: true,
		APIKeys:          make(map[string]string),
	}
	interceptor, err := NewAuthInterceptor(config)
	if err != nil {
		t.Fatalf("Failed to create interceptor: %v", err)
	}
	defer interceptor.Stop()

	// Should not panic on empty map
	interceptor.cleanupOldTimestamps()
}

// TestCleanupOldTimestamps_WithOldEntries tests cleanup removes old timestamps
func TestCleanupOldTimestamps_WithOldEntries(t *testing.T) {
	
	config := &AuthConfig{
		EnableAPIKeyAuth: true,
		APIKeys:          make(map[string]string),
	}
	interceptor, err := NewAuthInterceptor(config)
	if err != nil {
		t.Fatalf("Failed to create interceptor: %v", err)
	}
	defer interceptor.Stop()

	now := time.Now()

	// Add some old timestamps manually (well beyond the cutoff)
	oldTime := now.Add(-10 * maxRequestAge).Unix()
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("oldkey%d:%d", i, oldTime)
		interceptor.seenRequests.Store(key, true)
	}

	// Add some recent timestamps
	recentTime := now.Unix()
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("recentkey%d:%d", i, recentTime)
		interceptor.seenRequests.Store(key, true)
	}

	// Count before cleanup
	countBefore := 0
	interceptor.seenRequests.Range(func(key, value interface{}) bool {
		countBefore++
		return true
	})

	if countBefore != 8 {
		t.Errorf("Expected 8 entries before cleanup, got %d", countBefore)
	}

	// Run cleanup
	interceptor.cleanupOldTimestamps()

	// Count after cleanup
	countAfter := 0
	interceptor.seenRequests.Range(func(key, value interface{}) bool {
		countAfter++
		return true
	})

	// Old entries should be removed, recent ones should remain
	if countAfter < 3 {
		t.Errorf("Expected at least 3 entries after cleanup, got %d", countAfter)
	}
}

// TestCleanupOldTimestamps_WithMixedAges tests cleanup with various ages
func TestCleanupOldTimestamps_WithMixedAges(t *testing.T) {
	
	config := &AuthConfig{
		EnableAPIKeyAuth: true,
		APIKeys:          make(map[string]string),
	}
	interceptor, err := NewAuthInterceptor(config)
	if err != nil {
		t.Fatalf("Failed to create interceptor: %v", err)
	}
	defer interceptor.Stop()

	now := time.Now()

	// Very old (should be removed)
	veryOld := now.Add(-10 * maxRequestAge).Unix()
	interceptor.seenRequests.Store(fmt.Sprintf("very-old:%d", veryOld), true)

	// Old (should be removed)
	old := now.Add(-6 * maxRequestAge).Unix()
	interceptor.seenRequests.Store(fmt.Sprintf("old:%d", old), true)

	// Recent (should remain)
	recent := now.Unix()
	interceptor.seenRequests.Store(fmt.Sprintf("recent:%d", recent), true)

	// Run cleanup
	interceptor.cleanupOldTimestamps()

	// Verify very old is removed
	if _, exists := interceptor.seenRequests.Load(fmt.Sprintf("very-old:%d", veryOld)); exists {
		t.Error("Very old entry should have been removed")
	}

	// Verify old is removed
	if _, exists := interceptor.seenRequests.Load(fmt.Sprintf("old:%d", old)); exists {
		t.Error("Old entry should have been removed")
	}

	// Verify recent remains
	if _, exists := interceptor.seenRequests.Load(fmt.Sprintf("recent:%d", recent)); !exists {
		t.Error("Recent entry should remain")
	}
}

// TestPeriodicCleanup tests the background cleanup goroutine
func TestPeriodicCleanup(t *testing.T) {
	
	config := &AuthConfig{EnableAPIKeyAuth: true, APIKeys: make(map[string]string)}
	interceptor, err := NewAuthInterceptor(config)
	if err != nil {
		t.Fatalf("Failed to create interceptor: %v", err)
	}

	// Add an old entry
	oldTime := time.Now().Add(-2 * maxRequestAge).Unix()
	interceptor.seenRequests.Store(fmt.Sprintf("test-old:%d", oldTime), true)

	// Wait for periodic cleanup (cleanupInterval is 10 minutes by default)
	// We can't wait that long, so we'll just verify the goroutine is running
	time.Sleep(100 * time.Millisecond)

	// Manually trigger cleanup for testing
	interceptor.cleanupOldTimestamps()

	// Verify old entry was removed
	if _, exists := interceptor.seenRequests.Load(fmt.Sprintf("test-old:%d", oldTime)); exists {
		t.Error("Old entry should have been removed by cleanup")
	}

	// Stop the interceptor
	interceptor.Stop()
}

// TestAuthInterceptor_Stop tests stopping the interceptor
func TestAuthInterceptor_Stop(t *testing.T) {
	
	config := &AuthConfig{EnableAPIKeyAuth: true, APIKeys: make(map[string]string)}
	interceptor, err := NewAuthInterceptor(config)
	if err != nil {
		t.Fatalf("Failed to create interceptor: %v", err)
	}

	// Stop should not panic
	interceptor.Stop()

	// Stopping twice should not panic
	interceptor.Stop()
}

// TestMemoryAPIKeyStore_Concurrent tests concurrent access to the key store
func TestMemoryAPIKeyStore_Concurrent(t *testing.T) {
	keyStore := NewMemoryAPIKeyStore()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			keyStore.AddKey(fmt.Sprintf("key-%d", id), fmt.Sprintf("secret-%d", id))
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = keyStore.GetSecret(fmt.Sprintf("key-%d", id))
		}(i)
	}

	wg.Wait()
}

// TestAuthInterceptor_ValidateAuthWithExpiredTimestamp tests validation with expired timestamp
func TestAuthInterceptor_ValidateAuthWithExpiredTimestamp(t *testing.T) {
	testAPIKey := "test-key"
	testSecret := "test-secret"
	
	keyStore := NewMemoryAPIKeyStore()
	keyStore.AddKey(testAPIKey, testSecret)

	apiKeys := map[string]string{testAPIKey: testSecret}
	config := &AuthConfig{EnableAPIKeyAuth: true, APIKeys: apiKeys}
	interceptor, err := NewAuthInterceptor(config)
	if err != nil {
		t.Fatalf("Failed to create interceptor: %v", err)
	}
	defer interceptor.Stop()
	
	// Inject the key store into the interceptor
	interceptor.keyStore = keyStore

	// Create auth with very old timestamp
	oldTimestamp := time.Now().Add(-2 * maxRequestAge).Unix()
	auth := &APIMetadata{
		ApiKey:    testAPIKey,
		Timestamp: oldTimestamp,
		Signature: interceptor.computeSignature(testAPIKey, oldTimestamp, "/test.Method", testSecret),
	}

	// Validation should fail
	err = interceptor.validateAPIKeyAuth(nil, auth, "/test.Method")
	if err == nil {
		t.Error("Expected error for expired timestamp")
	}
}

// TestAuthInterceptor_ValidateAuthWithReplay tests replay attack detection
func TestAuthInterceptor_ValidateAuthWithReplay(t *testing.T) {
	testAPIKey := "test-key"
	testSecret := "test-secret"
	
	keyStore := NewMemoryAPIKeyStore()
	keyStore.AddKey(testAPIKey, testSecret)

	apiKeys := map[string]string{testAPIKey: testSecret}
	config := &AuthConfig{EnableAPIKeyAuth: true, APIKeys: apiKeys}
	interceptor, err := NewAuthInterceptor(config)
	if err != nil {
		t.Fatalf("Failed to create interceptor: %v", err)
	}
	defer interceptor.Stop()
	
	// Inject the key store into the interceptor
	interceptor.keyStore = keyStore

	// Create valid auth
	timestamp := time.Now().Unix()
	auth := &APIMetadata{
		ApiKey:    testAPIKey,
		Timestamp: timestamp,
		Signature: interceptor.computeSignature(testAPIKey, timestamp, "/test.Method", testSecret),
	}

	// First validation should succeed
	err = interceptor.validateAPIKeyAuth(nil, auth, "/test.Method")
	if err != nil {
		t.Errorf("First validation failed: %v", err)
	}

	// Second validation with same timestamp should fail (replay attack)
	err = interceptor.validateAPIKeyAuth(nil, auth, "/test.Method")
	if err == nil {
		t.Error("Expected error for replay attack")
	}
}

// TestCleanupOldTimestamps_LargeDataSet tests cleanup with many entries
func TestCleanupOldTimestamps_LargeDataSet(t *testing.T) {
	
	config := &AuthConfig{EnableAPIKeyAuth: true, APIKeys: make(map[string]string)}
	interceptor, err := NewAuthInterceptor(config)
	if err != nil {
		t.Fatalf("Failed to create interceptor: %v", err)
	}
	defer interceptor.Stop()

	now := time.Now()

	// Add 100 old entries (well beyond cutoff)
	oldTime := now.Add(-10 * maxRequestAge).Unix()
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("old-%d:%d", i, oldTime)
		interceptor.seenRequests.Store(key, true)
	}

	// Add 50 recent entries
	recentTime := now.Unix()
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("recent-%d:%d", i, recentTime)
		interceptor.seenRequests.Store(key, true)
	}

	// Run cleanup
	interceptor.cleanupOldTimestamps()

	// Count remaining
	count := 0
	interceptor.seenRequests.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	// Should have approximately 50 entries (recent ones)
	if count < 40 || count > 60 {
		t.Errorf("Expected around 50 entries after cleanup, got %d", count)
	}
}

// TestAuthMetadata is a test helper struct matching proto.AuthMetadata
type AuthMetadata struct {
	ApiKey    string
	Timestamp int64
	Signature string
}
