package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultLogDir is the default directory for audit logs.
	DefaultLogDir = "logs/audit"
	
	// DefaultLogFileName is the default log file name pattern.
	DefaultLogFileName = "audit.log"
	
	// DefaultMaxFileSize is the default maximum file size before rotation (10MB).
	DefaultMaxFileSize = 10 * 1024 * 1024
	
	// DefaultMaxAge is the default maximum age of log files before cleanup (30 days).
	DefaultMaxAge = 30 * 24 * time.Hour
	
	// DefaultBufferSize is the default size of the async write buffer.
	DefaultBufferSize = 1000
	
	// DefaultFlushInterval is the default interval for flushing buffered logs.
	DefaultFlushInterval = 5 * time.Second
)

// LoggerConfig holds configuration for the audit logger.
type LoggerConfig struct {
	// LogDir is the directory where audit logs are stored.
	LogDir string
	
	// MaxFileSize is the maximum size of a log file before rotation (bytes).
	MaxFileSize int64
	
	// MaxAge is the maximum age of log files before cleanup.
	MaxAge time.Duration
	
	// BufferSize is the size of the async write buffer.
	BufferSize int
	
	// FlushInterval is the interval for flushing buffered logs.
	FlushInterval time.Duration
	
	// EnableConsole enables logging to console in addition to file.
	EnableConsole bool
	
	// CompressionEnabled enables gzip compression for rotated logs.
	CompressionEnabled bool
}

// Logger is the main audit logger that handles asynchronous writing and rotation.
type Logger struct {
	config       LoggerConfig
	logChan      chan *AuditEvent
	stopChan     chan struct{}
	stopOnce     sync.Once
	wg           sync.WaitGroup
	mu           sync.RWMutex
	currentFile  *os.File
	currentSize  int64
	currentPath  string
	encoder      *json.Encoder
	writeMu      sync.Mutex // ensures sequential file writes
}

// globalLogger is the package-level logger instance.
var (
	globalLogger *Logger
	initOnce     sync.Once
)

// Init initializes the global audit logger with default configuration.
func Init() error {
	return InitWithConfig(LoggerConfig{
		LogDir:              DefaultLogDir,
		MaxFileSize:         DefaultMaxFileSize,
		MaxAge:              DefaultMaxAge,
		BufferSize:          DefaultBufferSize,
		FlushInterval:       DefaultFlushInterval,
		EnableConsole:       false,
		CompressionEnabled:  false,
	})
}

// InitWithConfig initializes the global audit logger with custom configuration.
func InitWithConfig(config LoggerConfig) error {
	var initErr error
	initOnce.Do(func() {
		logger, err := NewLogger(config)
		if err != nil {
			initErr = err
			return
		}
		globalLogger = logger
	})
	return initErr
}

// GetLogger returns the global audit logger.
func GetLogger() *Logger {
	return globalLogger
}

// NewLogger creates a new audit logger with the given configuration.
func NewLogger(config LoggerConfig) (*Logger, error) {
	// Set defaults
	if config.LogDir == "" {
		config.LogDir = DefaultLogDir
	}
	if config.MaxFileSize <= 0 {
		config.MaxFileSize = DefaultMaxFileSize
	}
	if config.MaxAge <= 0 {
		config.MaxAge = DefaultMaxAge
	}
	if config.BufferSize <= 0 {
		config.BufferSize = DefaultBufferSize
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = DefaultFlushInterval
	}
	
	// Create log directory
	if err := os.MkdirAll(config.LogDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}
	
	logger := &Logger{
		config:   config,
		logChan:  make(chan *AuditEvent, config.BufferSize),
		stopChan: make(chan struct{}),
	}
	
	// Open initial log file
	if err := logger.openLogFile(); err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	
	// Start async writer goroutine
	logger.wg.Add(1)
	go logger.asyncWriter()
	
	// Start cleanup goroutine
	logger.wg.Add(1)
	go logger.cleanupOldLogs()
	
	return logger, nil
}

// openLogFile opens a new log file for writing.
func (l *Logger) openLogFile() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Close existing file if open
	if l.currentFile != nil {
		l.currentFile.Close()
	}
	
	// Generate log file path with timestamp
	timestamp := time.Now().Format("20060102_150405")
	logPath := filepath.Join(l.config.LogDir, fmt.Sprintf("audit_%s.log", timestamp))
	
	// Open file for appending (create if doesn't exist)
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	
	l.currentFile = file
	l.currentPath = logPath
	l.currentSize = 0
	l.encoder = json.NewEncoder(file)
	
	return nil
}

// asyncWriter processes log events from the channel and writes them to file.
func (l *Logger) asyncWriter() {
	defer l.wg.Done()
	
	flushTicker := time.NewTicker(l.config.FlushInterval)
	defer flushTicker.Stop()
	
	for {
		select {
		case event := <-l.logChan:
			if err := l.writeEvent(event); err != nil {
				log.Printf("audit: failed to write event: %v", err)
			}
			
		case <-flushTicker.C:
			// Flush buffered writes
			l.mu.RLock()
			if l.currentFile != nil {
				l.currentFile.Sync()
			}
			l.mu.RUnlock()
			
		case <-l.stopChan:
			// Drain remaining events before stopping
			for {
				select {
				case event := <-l.logChan:
					if err := l.writeEvent(event); err != nil {
						log.Printf("audit: failed to write event during shutdown: %v", err)
					}
				default:
					l.mu.RLock()
					if l.currentFile != nil {
						l.currentFile.Sync()
					}
					l.mu.RUnlock()
					return
				}
			}
		}
	}
}

// writeEvent writes a single audit event to the log file.
func (l *Logger) writeEvent(event *AuditEvent) error {
	l.writeMu.Lock()
	defer l.writeMu.Unlock()
	
	l.mu.RLock()
	if l.currentFile == nil || l.encoder == nil {
		l.mu.RUnlock()
		return fmt.Errorf("log file not initialized")
	}
	
	// Check if rotation is needed
	if l.currentSize >= l.config.MaxFileSize {
		l.mu.RUnlock()
		if err := l.rotateLog(); err != nil {
			return fmt.Errorf("failed to rotate log: %w", err)
		}
		l.mu.RLock()
	}
	
	// Encode event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		l.mu.RUnlock()
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	
	// Write to file
	n, err := l.currentFile.Write(append(eventJSON, '\n'))
	if err != nil {
		l.mu.RUnlock()
		return fmt.Errorf("failed to write event: %w", err)
	}
	
	l.currentSize += int64(n)
	l.mu.RUnlock()
	
	// Also log to console if enabled
	if l.config.EnableConsole {
		consoleJSON, _ := json.Marshal(event)
		log.Printf("AUDIT: %s", string(consoleJSON))
	}
	
	return nil
}

// rotateLog rotates the current log file and opens a new one.
func (l *Logger) rotateLog() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.currentFile != nil {
		l.currentFile.Close()
	}
	
	// Rename current file with timestamp
	timestamp := time.Now().Format("20060102_150405")
	rotatedPath := filepath.Join(l.config.LogDir, fmt.Sprintf("audit_%s.log", timestamp))
	
	if err := os.Rename(l.currentPath, rotatedPath); err != nil {
		return fmt.Errorf("failed to rename log file: %w", err)
	}
	
	// Open new log file
	file, err := os.OpenFile(l.currentPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open new log file: %w", err)
	}
	
	l.currentFile = file
	l.currentSize = 0
	l.encoder = json.NewEncoder(file)
	
	return nil
}

// cleanupOldLogs periodically removes log files older than MaxAge.
func (l *Logger) cleanupOldLogs() {
	defer l.wg.Done()
	
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			l.performCleanup()
		case <-l.stopChan:
			l.performCleanup() // Final cleanup on shutdown
			return
		}
	}
}

// performCleanup removes old log files.
func (l *Logger) performCleanup() {
	cutoff := time.Now().Add(-l.config.MaxAge)
	
	entries, err := os.ReadDir(l.config.LogDir)
	if err != nil {
		log.Printf("audit: failed to read log directory: %v", err)
		return
	}
	
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "audit_") {
			continue
		}
		
		info, err := entry.Info()
		if err != nil {
			continue
		}
		
		if info.ModTime().Before(cutoff) {
			filePath := filepath.Join(l.config.LogDir, entry.Name())
			if err := os.Remove(filePath); err != nil {
				log.Printf("audit: failed to remove old log file %s: %v", filePath, err)
			} else {
				log.Printf("audit: removed old log file: %s", filePath)
			}
		}
	}
}

// Log records an audit event asynchronously.
func (l *Logger) Log(event *AuditEvent) {
	if l == nil || event == nil {
		return
	}
	
	select {
	case l.logChan <- event:
		// Successfully queued
	default:
		// Buffer full, log warning
		log.Printf("audit: buffer full, dropping event: %v", event.EventType)
	}
}

// LogSync records an audit event synchronously (blocks until written).
// Use this only for critical events that must be persisted.
func (l *Logger) LogSync(event *AuditEvent) error {
	if l == nil || event == nil {
		return nil
	}
	return l.writeEvent(event)
}

// Close gracefully shuts down the logger.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	
	l.stopOnce.Do(func() {
		close(l.stopChan)
	})
	
	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		// Clean shutdown
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout waiting for logger to shut down")
	}
	
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.currentFile != nil {
		return l.currentFile.Close()
	}
	
	return nil
}

// SanitizeValue masks sensitive information in a string.
func SanitizeValue(value string) string {
	if len(value) == 0 {
		return value
	}
	
	// Mask all but last 4 characters
	if len(value) <= 4 {
		return strings.Repeat("*", len(value))
	}
	
	return strings.Repeat("*", len(value)-4) + value[len(value)-4:]
}

// SanitizeAPIKey masks an API key for logging.
func SanitizeAPIKey(apiKey string) string {
	if len(apiKey) == 0 {
		return apiKey
	}
	
	// Show first 4 and last 4 characters
	if len(apiKey) <= 8 {
		return strings.Repeat("*", len(apiKey))
	}
	
	return apiKey[:4] + strings.Repeat("*", len(apiKey)-8) + apiKey[len(apiKey)-4:]
}

// LogAuthSuccess logs a successful authentication event.
func LogAuthSuccess(clientIP, userID, method string) {
	if globalLogger == nil {
		return
	}
	event := NewAuditEvent(EventAuthSuccess, SeverityInfo).
		WithClientIP(clientIP).
		WithUserID(userID).
		WithMethod(method).
		WithResult("success").
		WithReason("Authentication successful")
	globalLogger.Log(event)
}

// LogAuthFailure logs a failed authentication event.
func LogAuthFailure(clientIP, userID, method, reason string) {
	if globalLogger == nil {
		return
	}
	event := NewAuditEvent(EventAuthFailure, SeverityWarning).
		WithClientIP(clientIP).
		WithUserID(SanitizeValue(userID)).
		WithMethod(method).
		WithResult("failure").
		WithReason(reason)
	globalLogger.Log(event)
}

// LogRateLimitViolation logs a rate limit violation event.
func LogRateLimitViolation(clientIP, userID, method string) {
	if globalLogger == nil {
		return
	}
	event := NewAuditEvent(EventRateLimitViolated, SeverityWarning).
		WithClientIP(clientIP).
		WithUserID(userID).
		WithMethod(method).
		WithResult("violation").
		WithReason("Rate limit exceeded")
	globalLogger.Log(event)
}

// LogPermissionChange logs a permission change event.
func LogPermissionChange(clientIP, userID, action, targetUser, permission string) {
	if globalLogger == nil {
		return
	}
	event := NewAuditEvent(EventPermissionChanged, SeverityWarning).
		WithClientIP(clientIP).
		WithUserID(userID).
		WithResult("changed").
		WithReason(fmt.Sprintf("Permission %s for user %s", action, targetUser)).
		WithMetadata("action", action).
		WithMetadata("target_user", SanitizeValue(targetUser)).
		WithMetadata("permission", permission)
	globalLogger.Log(event)
}

// LogConfigChange logs a configuration modification event.
func LogConfigChange(clientIP, userID, configKey string, oldValue, newValue interface{}) {
	if globalLogger == nil {
		return
	}
	event := NewAuditEvent(EventConfigModified, SeverityWarning).
		WithClientIP(clientIP).
		WithUserID(userID).
		WithResult("modified").
		WithReason(fmt.Sprintf("Configuration key %s modified", configKey)).
		WithMetadata("config_key", configKey).
		WithMetadata("old_value", SanitizeConfigValue(oldValue)).
		WithMetadata("new_value", SanitizeConfigValue(newValue))
	globalLogger.Log(event)
}

// SanitizeConfigValue sanitizes configuration values that might contain secrets.
func SanitizeConfigValue(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		// Check if it looks like a secret
		lower := strings.ToLower(v)
		if strings.Contains(lower, "secret") || 
		   strings.Contains(lower, "password") || 
		   strings.Contains(lower, "token") ||
		   strings.Contains(lower, "key") {
			return SanitizeValue(v)
		}
		return v
	default:
		return value
	}
}

// Middleware returns a context key for storing audit information.
type contextKey string

const (
	auditRequestIDKey contextKey = "audit_request_id"
	auditClientIPKey  contextKey = "audit_client_ip"
	auditUserIDKey    contextKey = "audit_user_id"
)

// ContextWithAuditInfo returns a context with audit information.
func ContextWithAuditInfo(ctx context.Context, requestID, clientIP, userID string) context.Context {
	ctx = context.WithValue(ctx, auditRequestIDKey, requestID)
	ctx = context.WithValue(ctx, auditClientIPKey, clientIP)
	ctx = context.WithValue(ctx, auditUserIDKey, userID)
	return ctx
}

// GetAuditInfoFromContext extracts audit information from context.
func GetAuditInfoFromContext(ctx context.Context) (requestID, clientIP, userID string) {
	if v := ctx.Value(auditRequestIDKey); v != nil {
		requestID = v.(string)
	}
	if v := ctx.Value(auditClientIPKey); v != nil {
		clientIP = v.(string)
	}
	if v := ctx.Value(auditUserIDKey); v != nil {
		userID = v.(string)
	}
	return
}

// SetLogWriter allows setting a custom io.Writer for testing.
func (l *Logger) SetLogWriter(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.encoder != nil {
		l.encoder = json.NewEncoder(w)
	}
}

// GetLogFiles returns a list of all audit log files sorted by modification time.
func GetLogFiles(logDir string) ([]string, error) {
	if logDir == "" {
		logDir = DefaultLogDir
	}
	
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}
	
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "audit_") {
			files = append(files, filepath.Join(logDir, entry.Name()))
		}
	}
	
	// Sort by filename (which includes timestamp)
	sort.Strings(files)
	
	return files, nil
}
