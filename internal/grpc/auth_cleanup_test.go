package grpc

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthInterceptor_TimestampCleanup(t *testing.T) {
	t.Run("cleanup removes old timestamps", func(t *testing.T) {
		config := &AuthConfig{
			EnableAPIKeyAuth: true,
			APIKeys: map[string]string{
				"test-key": "test-secret",
			},
		}

		interceptor, err := NewAuthInterceptor(config)
		require.NoError(t, err)
		defer interceptor.Stop()

		// Manually add some old timestamps
		oldTimestamp := time.Now().Add(-10 * time.Minute).Unix() // Older than maxRequestAge (5 min)
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("old-key-%d:%d", i, oldTimestamp)
			interceptor.seenRequests.Store(key, true)
		}

		// Add some new timestamps
		newTimestamp := time.Now().Unix()
		for i := 0; i < 50; i++ {
			key := fmt.Sprintf("new-key-%d:%d", i, newTimestamp)
			interceptor.seenRequests.Store(key, true)
		}

		// Update the counter
		_ = interceptor.GetTimestampCount()

		// Manually trigger cleanup
		interceptor.cleanupOldTimestamps()

		// Verify old entries were removed
		oldCount := 0
		interceptor.seenRequests.Range(func(key, value interface{}) bool {
			keyStr := key.(string)
			if len(keyStr) >= 8 && keyStr[:8] == "old-key-" {
				oldCount++
			}
			return true
		})

		assert.Equal(t, 0, oldCount, "All old timestamps should be removed")

		// Verify new entries remain
		newCount := 0
		interceptor.seenRequests.Range(func(key, value interface{}) bool {
			keyStr := key.(string)
			if len(keyStr) >= 8 && keyStr[:8] == "new-key-" {
				newCount++
			}
			return true
		})

		assert.Equal(t, 50, newCount, "New timestamps should remain")
	})

	t.Run("cleanup enforces size limit", func(t *testing.T) {
		config := &AuthConfig{
			EnableAPIKeyAuth: true,
			APIKeys: map[string]string{
				"test-key": "test-secret",
			},
		}

		interceptor, err := NewAuthInterceptor(config)
		require.NoError(t, err)
		defer interceptor.Stop()

		// Temporarily reduce the max size for testing
		originalMaxSize := maxTimestampMapSize
		maxTimestampMapSize = 100 // Set low for testing
		defer func() { maxTimestampMapSize = originalMaxSize }()

		// Add entries exceeding the limit with recent timestamps
		now := time.Now().Unix()
		for i := 0; i < 200; i++ {
			key := fmt.Sprintf("test-key-%d:%d", i, now+int64(i))
			interceptor.seenRequests.Store(key, true)
		}

		// Manually trigger cleanup
		interceptor.cleanupOldTimestamps()

		// Count remaining entries
		count := 0
		interceptor.seenRequests.Range(func(key, value interface{}) bool {
			count++
			return true
		})

		// Should have removed approximately 50% (cleanupPercentage)
		// Expected: around 100 entries remaining (200 * 0.5)
		assert.LessOrEqual(t, count, 100, "Should enforce size limit")
		assert.Greater(t, count, 0, "Should have some entries remaining")
	})

	t.Run("cleanup handles mixed old and new entries", func(t *testing.T) {
		config := &AuthConfig{
			EnableAPIKeyAuth: true,
			APIKeys: map[string]string{
				"test-key": "test-secret",
			},
		}

		interceptor, err := NewAuthInterceptor(config)
		require.NoError(t, err)
		defer interceptor.Stop()

		now := time.Now()

		// Add very old entries (should be removed by age)
		veryOldTimestamp := now.Add(-20 * time.Minute).Unix()
		for i := 0; i < 50; i++ {
			key := fmt.Sprintf("very-old-%d:%d", i, veryOldTimestamp)
			interceptor.seenRequests.Store(key, true)
		}

		// Add moderately old entries (should be removed by age)
		oldTimestamp := now.Add(-8 * time.Minute).Unix()
		for i := 0; i < 50; i++ {
			key := fmt.Sprintf("old-%d:%d", i, oldTimestamp)
			interceptor.seenRequests.Store(key, true)
		}

		// Add recent entries (should remain)
		recentTimestamp := now.Add(-2 * time.Minute).Unix()
		for i := 0; i < 50; i++ {
			key := fmt.Sprintf("recent-%d:%d", i, recentTimestamp)
			interceptor.seenRequests.Store(key, true)
		}

		// Add very new entries (should remain)
		newTimestamp := now.Unix()
		for i := 0; i < 50; i++ {
			key := fmt.Sprintf("new-%d:%d", i, newTimestamp)
			interceptor.seenRequests.Store(key, true)
		}

		// Manually trigger cleanup
		interceptor.cleanupOldTimestamps()

		// Count entries by category
		var veryOldCount, oldCount, recentCount, newCount int
		interceptor.seenRequests.Range(func(key, value interface{}) bool {
			keyStr := key.(string)
			if len(keyStr) >= 5 {
				switch keyStr[:5] {
				case "very-":
					veryOldCount++
				case "old-k":
					oldCount++
				case "recen":
					recentCount++
				case "new-k":
					newCount++
				}
			}
			return true
		})

		assert.Equal(t, 0, veryOldCount, "Very old entries should be removed")
		assert.Equal(t, 0, oldCount, "Old entries should be removed")
		assert.Equal(t, 50, recentCount, "Recent entries should remain")
		assert.Equal(t, 50, newCount, "New entries should remain")
	})

	t.Run("GetTimestampCount returns accurate count", func(t *testing.T) {
		config := &AuthConfig{
			EnableAPIKeyAuth: true,
			APIKeys: map[string]string{
				"test-key": "test-secret",
			},
		}

		interceptor, err := NewAuthInterceptor(config)
		require.NoError(t, err)
		defer interceptor.Stop()

		// Add some entries
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("count-test-%d:%d", i, time.Now().Unix())
			interceptor.seenRequests.Store(key, true)
		}

		count := interceptor.GetTimestampCount()
		assert.Equal(t, int64(100), count, "Timestamp count should match stored entries")
	})

	t.Run("cleanup is thread-safe", func(t *testing.T) {
		config := &AuthConfig{
			EnableAPIKeyAuth: true,
			APIKeys: map[string]string{
				"test-key": "test-secret",
			},
		}

		interceptor, err := NewAuthInterceptor(config)
		require.NoError(t, err)
		defer interceptor.Stop()

		var wg sync.WaitGroup

		// Concurrently add entries and trigger cleanup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(start int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					key := fmt.Sprintf("concurrent-%d-%d:%d", start, j, time.Now().Unix())
					interceptor.seenRequests.Store(key, true)
				}
			}(i)

			wg.Add(1)
			go func() {
				defer wg.Done()
				interceptor.cleanupOldTimestamps()
			}()
		}

		wg.Wait()

		// Should not panic and should have some entries
		count := interceptor.GetTimestampCount()
		assert.Greater(t, count, int64(0), "Should have entries after concurrent operations")
	})
}

func TestAuthInterceptor_ReplayAttackPrevention(t *testing.T) {
	t.Run("detects replay attack", func(t *testing.T) {
		config := &AuthConfig{
			EnableAPIKeyAuth: true,
			APIKeys: map[string]string{
				"test-key": "test-secret",
			},
		}

		interceptor, err := NewAuthInterceptor(config)
		require.NoError(t, err)
		defer interceptor.Stop()

		// Create a valid auth request
		auth := &APIMetadata{
			ApiKey:    "test-key",
			Timestamp: time.Now().Unix(),
			Signature: "", // Will be set below
		}

		// Generate valid signature
		auth.Signature = interceptor.computeSignature(auth.ApiKey, auth.Timestamp, "/test.Method", "test-secret")

		// First request should succeed
		err = interceptor.validateAPIKeyAuth(context.Background(), auth, "/test.Method")
		assert.NoError(t, err)

		// Same request again should be detected as replay attack
		err = interceptor.validateAPIKeyAuth(context.Background(), auth, "/test.Method")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "replay attack detected")
	})
}

func TestAuthInterceptor_ConcurrentCleanup(t *testing.T) {
	t.Run("periodic cleanup runs without issues", func(t *testing.T) {
		config := &AuthConfig{
			EnableAPIKeyAuth: true,
			APIKeys: map[string]string{
				"test-key": "test-secret",
			},
		}

		interceptor, err := NewAuthInterceptor(config)
		require.NoError(t, err)
		defer interceptor.Stop()

		// Simulate traffic while cleanup runs
		done := make(chan bool)

		go func() {
			for {
				select {
				case <-done:
					return
				default:
					auth := &proto.AuthMetadata{
						ApiKey:    "test-key",
						Timestamp: time.Now().Unix(),
					}
					auth.Signature = interceptor.computeSignature(auth.ApiKey, auth.Timestamp, "/test.Method", "test-secret")
					interceptor.validateAPIKeyAuth(context.Background(), auth, "/test.Method")
				}
			}
		}()

		// Let it run for a bit
		time.Sleep(100 * time.Millisecond)
		close(done)

		// Should not panic
		count := interceptor.GetTimestampCount()
		assert.Greater(t, count, int64(0), "Should have processed some requests")
	})
}

func TestAuthInterceptor_APIKeyValidation(t *testing.T) {
	t.Run("valid API key with signature", func(t *testing.T) {
		config := &AuthConfig{
			EnableAPIKeyAuth: true,
			APIKeys: map[string]string{
				"valid-key": "valid-secret",
			},
		}

		interceptor, err := NewAuthInterceptor(config)
		require.NoError(t, err)
		defer interceptor.Stop()

		auth := GenerateAPIKeyToken("valid-key", "valid-secret", "/test.Method")
		err = interceptor.validateAPIKeyAuth(context.Background(), auth, "/test.Method")
		assert.NoError(t, err)
	})

	t.Run("invalid API key", func(t *testing.T) {
		config := &AuthConfig{
			EnableAPIKeyAuth: true,
			APIKeys: map[string]string{
				"valid-key": "valid-secret",
			},
		}

		interceptor, err := NewAuthInterceptor(config)
		require.NoError(t, err)
		defer interceptor.Stop()

		auth := &proto.AuthMetadata{
			ApiKey:    "invalid-key",
			Timestamp: time.Now().Unix(),
			Signature: "invalid",
		}

		err = interceptor.validateAPIKeyAuth(context.Background(), auth, "/test.Method")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid API key")
	})

	t.Run("old timestamp rejected", func(t *testing.T) {
		config := &AuthConfig{
			EnableAPIKeyAuth: true,
			APIKeys: map[string]string{
				"test-key": "test-secret",
			},
		}

		interceptor, err := NewAuthInterceptor(config)
		require.NoError(t, err)
		defer interceptor.Stop()

		auth := &proto.AuthMetadata{
			ApiKey:    "test-key",
			Timestamp: time.Now().Add(-10 * time.Minute).Unix(), // Too old
			Signature: "invalid",
		}

		err = interceptor.validateAPIKeyAuth(context.Background(), auth, "/test.Method")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "timestamp too old")
	})

	t.Run("invalid signature rejected", func(t *testing.T) {
		config := &AuthConfig{
			EnableAPIKeyAuth: true,
			APIKeys: map[string]string{
				"test-key": "test-secret",
			},
		}

		interceptor, err := NewAuthInterceptor(config)
		require.NoError(t, err)
		defer interceptor.Stop()

		auth := &proto.AuthMetadata{
			ApiKey:    "test-key",
			Timestamp: time.Now().Unix(),
			Signature: "invalid-signature",
		}

		err = interceptor.validateAPIKeyAuth(context.Background(), auth, "/test.Method")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid signature")
	})
}
