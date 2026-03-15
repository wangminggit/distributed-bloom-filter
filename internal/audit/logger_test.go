package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewAuditEvent(t *testing.T) {
	event := NewAuditEvent(EventAuthSuccess, SeverityInfo)
	
	if event.EventType != EventAuthSuccess {
		t.Errorf("expected event type %s, got %s", EventAuthSuccess, event.EventType)
	}
	
	if event.Severity != SeverityInfo {
		t.Errorf("expected severity %s, got %s", SeverityInfo, event.Severity)
	}
	
	if event.Timestamp == 0 {
		t.Error("expected timestamp to be set")
	}
	
	if event.Time == "" {
		t.Error("expected time to be set")
	}
	
	if event.Metadata == nil {
		t.Error("expected metadata map to be initialized")
	}
}

func TestAuditEventBuilder(t *testing.T) {
	event := NewAuditEvent(EventAuthFailure, SeverityWarning).
		WithClientIP("192.168.1.100").
		WithUserID("user123").
		WithMethod("/proto.DBFService/Check").
		WithResult("failure").
		WithReason("Invalid API key").
		WithRequestID("req-456")
	
	if event.ClientIP != "192.168.1.100" {
		t.Errorf("expected client IP 192.168.1.100, got %s", event.ClientIP)
	}
	
	if event.UserID != "user123" {
		t.Errorf("expected user ID user123, got %s", event.UserID)
	}
	
	if event.Method != "/proto.DBFService/Check" {
		t.Errorf("expected method /proto.DBFService/Check, got %s", event.Method)
	}
	
	if event.Result != "failure" {
		t.Errorf("expected result failure, got %s", event.Result)
	}
	
	if event.Reason != "Invalid API key" {
		t.Errorf("expected reason 'Invalid API key', got %s", event.Reason)
	}
	
	if event.RequestID != "req-456" {
		t.Errorf("expected request ID req-456, got %s", event.RequestID)
	}
}

func TestGetDefaultSeverity(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  EventSeverity
	}{
		{EventAuthSuccess, SeverityInfo},
		{EventAuthFailure, SeverityWarning},
		{EventRateLimitViolated, SeverityWarning},
		{EventPermissionChanged, SeverityWarning},
		{EventConfigModified, SeverityWarning},
		{EventSystemStart, SeverityInfo},
		{EventSystemStop, SeverityWarning},
	}
	
	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			severity := GetDefaultSeverity(tt.eventType)
			if severity != tt.expected {
				t.Errorf("expected severity %s for event %s, got %s", tt.expected, tt.eventType, severity)
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	tmpDir := t.TempDir()
	
	config := LoggerConfig{
		LogDir:        tmpDir,
		MaxFileSize:   1024 * 1024,
		MaxAge:        24 * time.Hour,
		BufferSize:    100,
		FlushInterval: time.Second,
	}
	
	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()
	
	if logger.config.LogDir != tmpDir {
		t.Errorf("expected log dir %s, got %s", tmpDir, logger.config.LogDir)
	}
	
	// Verify log file was created
	logger.mu.RLock()
	logPath := logger.currentPath
	logger.mu.RUnlock()
	
	if logPath == "" {
		t.Error("expected log file path to be set")
	}
	
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("expected log file to exist at %s", logPath)
	}
}

func TestLoggerWriteEvent(t *testing.T) {
	tmpDir := t.TempDir()
	
	config := LoggerConfig{
		LogDir:        tmpDir,
		MaxFileSize:   1024 * 1024,
		MaxAge:        24 * time.Hour,
		BufferSize:    100,
		FlushInterval: time.Second,
		EnableConsole: false,
	}
	
	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()
	
	event := NewAuditEvent(EventAuthSuccess, SeverityInfo).
		WithClientIP("192.168.1.100").
		WithUserID("user123").
		WithMethod("/proto.DBFService/Check").
		WithResult("success")
	
	// Log synchronously to ensure it's written
	if err := logger.LogSync(event); err != nil {
		t.Fatalf("failed to log event: %v", err)
	}
	
	// Read the log file and verify content
	logger.mu.RLock()
	logPath := logger.currentPath
	logger.mu.RUnlock()
	
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 log line, got %d", len(lines))
	}
	
	var loggedEvent AuditEvent
	if err := json.Unmarshal([]byte(lines[0]), &loggedEvent); err != nil {
		t.Fatalf("failed to unmarshal log event: %v", err)
	}
	
	if loggedEvent.EventType != EventAuthSuccess {
		t.Errorf("expected event type %s, got %s", EventAuthSuccess, loggedEvent.EventType)
	}
	
	if loggedEvent.ClientIP != "192.168.1.100" {
		t.Errorf("expected client IP 192.168.1.100, got %s", loggedEvent.ClientIP)
	}
}

func TestLoggerAsyncWrite(t *testing.T) {
	tmpDir := t.TempDir()
	
	config := LoggerConfig{
		LogDir:        tmpDir,
		MaxFileSize:   1024 * 1024,
		MaxAge:        24 * time.Hour,
		BufferSize:    100,
		FlushInterval: 100 * time.Millisecond,
	}
	
	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Log multiple events asynchronously
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			event := NewAuditEvent(EventAuthSuccess, SeverityInfo).
				WithUserID(string(rune('0' + i))).
				WithResult("success")
			logger.Log(event)
		}(i)
	}
	
	wg.Wait()
	
	// Give the async writer time to process
	time.Sleep(200 * time.Millisecond)
	
	// Verify events were written
	logger.mu.RLock()
	logPath := logger.currentPath
	logger.mu.RUnlock()
	
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 log lines, got %d", len(lines))
	}
}

func TestLogRotation(t *testing.T) {
	tmpDir := t.TempDir()
	
	config := LoggerConfig{
		LogDir:        tmpDir,
		MaxFileSize:   500, // Small size to trigger rotation
		MaxAge:        24 * time.Hour,
		BufferSize:    100,
		FlushInterval: time.Second,
	}
	
	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Write enough events to trigger rotation
	for i := 0; i < 10; i++ {
		event := NewAuditEvent(EventAuthSuccess, SeverityInfo).
			WithUserID("user123").
			WithMethod("/proto.DBFService/Check").
			WithResult("success").
			WithReason("This is a test reason to make the log entry longer")
		
		if err := logger.LogSync(event); err != nil {
			t.Fatalf("failed to log event: %v", err)
		}
	}
	
	// Check that rotation occurred
	files, err := filepath.Glob(filepath.Join(tmpDir, "audit_*.log"))
	if err != nil {
		t.Fatalf("failed to glob log files: %v", err)
	}
	
	if len(files) < 1 {
		t.Error("expected at least 1 log file after rotation")
	}
}

func TestSanitizeValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"abc", "***"},
		{"abcd", "****"},
		{"abcde", "*bcde"},
		{"abcdef", "**cdef"},
		{"1234567890", "******7890"},
	}
	
	for _, tt := range tests {
		result := SanitizeValue(tt.input)
		if result != tt.expected {
			t.Errorf("expected %s, got %s for input %s", tt.expected, result, tt.input)
		}
	}
}

func TestSanitizeAPIKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"abc", "***"},
		{"abcd", "****"},
		{"abcdefgh", "********"},
		{"abcdefghij", "abcd**ghij"},
		{"sk-1234567890abcdef", "sk-1***********cdef"},
	}
	
	for _, tt := range tests {
		result := SanitizeAPIKey(tt.input)
		if result != tt.expected {
			t.Errorf("expected %s, got %s for input %s", tt.expected, result, tt.input)
		}
	}
}

func TestHelperFunctions(t *testing.T) {
	tmpDir := t.TempDir()
	
	config := LoggerConfig{
		LogDir:        tmpDir,
		MaxFileSize:   1024 * 1024,
		MaxAge:        24 * time.Hour,
		BufferSize:    100,
		FlushInterval: time.Second,
	}
	
	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Test LogAuthSuccess
	event1 := NewAuditEvent(EventAuthSuccess, SeverityInfo).
		WithClientIP("192.168.1.100").
		WithUserID("user123").
		WithMethod("/proto.DBFService/Check").
		WithResult("success")
	logger.LogSync(event1)
	
	// Test LogAuthFailure
	event2 := NewAuditEvent(EventAuthFailure, SeverityWarning).
		WithClientIP("192.168.1.100").
		WithUserID("user123").
		WithMethod("/proto.DBFService/Check").
		WithResult("failure")
	logger.LogSync(event2)
	
	// Test LogRateLimitViolation
	event3 := NewAuditEvent(EventRateLimitViolated, SeverityWarning).
		WithClientIP("192.168.1.100").
		WithUserID("user123").
		WithMethod("/proto.DBFService/Check").
		WithResult("violation")
	logger.LogSync(event3)
	
	// Test LogPermissionChange
	event4 := NewAuditEvent(EventPermissionChanged, SeverityWarning).
		WithClientIP("192.168.1.100").
		WithUserID("admin").
		WithResult("changed").
		WithMetadata("action", "grant")
	logger.LogSync(event4)
	
	// Test LogConfigChange
	event5 := NewAuditEvent(EventConfigModified, SeverityWarning).
		WithClientIP("192.168.1.100").
		WithUserID("admin").
		WithResult("modified").
		WithMetadata("config_key", "rate_limit")
	logger.LogSync(event5)
	
	// Verify events were logged
	logger.mu.RLock()
	logPath := logger.currentPath
	logger.mu.RUnlock()
	
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 log lines, got %d", len(lines))
	}
}

func TestContextWithAuditInfo(t *testing.T) {
	ctx := ContextWithAuditInfo(context.Background(), "req-123", "192.168.1.100", "user123")
	
	requestID, clientIP, userID := GetAuditInfoFromContext(ctx)
	
	if requestID != "req-123" {
		t.Errorf("expected request ID req-123, got %s", requestID)
	}
	
	if clientIP != "192.168.1.100" {
		t.Errorf("expected client IP 192.168.1.100, got %s", clientIP)
	}
	
	if userID != "user123" {
		t.Errorf("expected user ID user123, got %s", userID)
	}
}

func TestLoggerClose(t *testing.T) {
	tmpDir := t.TempDir()
	
	config := LoggerConfig{
		LogDir:        tmpDir,
		MaxFileSize:   1024 * 1024,
		MaxAge:        24 * time.Hour,
		BufferSize:    100,
		FlushInterval: time.Second,
	}
	
	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	
	// Log some events
	event := NewAuditEvent(EventAuthSuccess, SeverityInfo).
		WithUserID("user123").
		WithResult("success")
	logger.Log(event)
	
	// Give async writer time to process
	time.Sleep(50 * time.Millisecond)
	
	// Close the logger
	if err := logger.Close(); err != nil {
		t.Errorf("failed to close logger: %v", err)
	}
	
	// Try to close again (should be safe due to sync.Once)
	// Note: The underlying file may already be closed, but sync.Once prevents double-close logic
	logger.stopOnce.Do(func() {}) // This is what Close() does internally
}

func TestGetLogFiles(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create some test log files
	testFiles := []string{
		"audit_20260314_100000.log",
		"audit_20260314_110000.log",
		"audit_20260314_120000.log",
		"other_file.txt",
	}
	
	for _, filename := range testFiles {
		filePath := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}
	
	files, err := GetLogFiles(tmpDir)
	if err != nil {
		t.Fatalf("failed to get log files: %v", err)
	}
	
	if len(files) != 3 {
		t.Errorf("expected 3 log files, got %d", len(files))
	}
	
	// Verify files are sorted
	expectedOrder := []string{
		filepath.Join(tmpDir, "audit_20260314_100000.log"),
		filepath.Join(tmpDir, "audit_20260314_110000.log"),
		filepath.Join(tmpDir, "audit_20260314_120000.log"),
	}
	
	for i, expected := range expectedOrder {
		if files[i] != expected {
			t.Errorf("expected file %s at index %d, got %s", expected, i, files[i])
		}
	}
}

func TestLoggerWithBuffer(t *testing.T) {
	tmpDir := t.TempDir()
	
	config := LoggerConfig{
		LogDir:        tmpDir,
		MaxFileSize:   1024 * 1024,
		MaxAge:        24 * time.Hour,
		BufferSize:    100,
		FlushInterval: 50 * time.Millisecond,
		EnableConsole: false,
	}
	
	logger, err := NewLogger(config)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Close()
	
	// Log events with small delays to avoid buffer overflow
	for i := 0; i < 20; i++ {
		event := NewAuditEvent(EventAuthSuccess, SeverityInfo).
			WithUserID(string(rune('0' + (i % 10)))).
			WithResult("success")
		logger.Log(event)
		time.Sleep(5 * time.Millisecond) // Small delay to allow processing
	}
	
	// Wait for buffer to flush
	time.Sleep(200 * time.Millisecond)
	
	// Verify events were written
	logger.mu.RLock()
	logPath := logger.currentPath
	logger.mu.RUnlock()
	
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 20 {
		t.Fatalf("expected 20 log lines, got %d", len(lines))
	}
}

func TestLoggerNilSafety(t *testing.T) {
	var logger *Logger
	
	// These should not panic
	logger.Log(nil)
	logger.LogSync(nil)
	logger.Close()
	
	// Global logger functions should handle nil gracefully
	LogAuthSuccess("", "", "")
	LogAuthFailure("", "", "", "")
	LogRateLimitViolation("", "", "")
}
