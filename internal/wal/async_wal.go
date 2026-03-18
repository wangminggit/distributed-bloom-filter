package wal

import (
	"sync"
	"time"
)

// AsyncWALWriter wraps WALWriter with async write capabilities.
// This reduces write latency by batching and deferring disk writes.
type AsyncWALWriter struct {
	*WALWriter

	// Async write buffer
	mu         sync.Mutex
	buffer     []WalEntry
	flushCh    chan struct{}
	stopCh     chan struct{}
	flushDone  chan struct{}

	// Configuration
	maxBatchSize int
	maxFlushTime time.Duration
	flushTimer   *time.Timer

	// Statistics
	stats struct {
		mu            sync.RWMutex
		totalWrites   int64
		totalFlushes  int64
		avgBatchSize  float64
		lastFlushTime time.Time
	}
}

// WalEntry represents a WAL entry for async writing.
type WalEntry struct {
	Data      []byte
	Timestamp time.Time
	Callback  func(error) // Optional callback when flushed
}

// NewAsyncWALWriter creates a new async WAL writer.
func NewAsyncWALWriter(baseDir string, secretPath string, maxBatchSize int, maxFlushTime time.Duration) (*AsyncWALWriter, error) {
	encryptor, err := NewWALEncryptor(secretPath)
	if err != nil {
		return nil, err
	}

	writer, err := NewWALWriter(baseDir, encryptor)
	if err != nil {
		return nil, err
	}

	async := &AsyncWALWriter{
		WALWriter:    writer,
		buffer:       make([]WalEntry, 0, maxBatchSize),
		flushCh:      make(chan struct{}, 1),
		stopCh:       make(chan struct{}),
		flushDone:    make(chan struct{}),
		maxBatchSize: maxBatchSize,
		maxFlushTime: maxFlushTime,
	}

	// Start background flush goroutine
	go async.flushLoop()

	return async, nil
}

// WriteAsync writes an entry asynchronously.
// Returns immediately, with actual write happening in background.
func (a *AsyncWALWriter) WriteAsync(data []byte, callback func(error)) error {
	entry := WalEntry{
		Data:      data,
		Timestamp: time.Now(),
		Callback:  callback,
	}

	a.mu.Lock()
	a.buffer = append(a.buffer, entry)
	shouldFlush := len(a.buffer) >= a.maxBatchSize
	a.mu.Unlock()

	// Update stats
	a.stats.mu.Lock()
	a.stats.totalWrites++
	a.stats.mu.Unlock()

	// Trigger flush if buffer is full
	if shouldFlush {
		select {
		case a.flushCh <- struct{}{}:
		default:
			// Flush already pending
		}
	}

	return nil
}

// Write writes an entry synchronously (for compatibility).
func (a *AsyncWALWriter) Write(data []byte) error {
	return a.WALWriter.Write(data)
}

// Flush forces a flush of all pending writes.
func (a *AsyncWALWriter) Flush() error {
	a.mu.Lock()
	if len(a.buffer) == 0 {
		a.mu.Unlock()
		return nil
	}

	// Copy buffer and clear
	entries := make([]WalEntry, len(a.buffer))
	copy(entries, a.buffer)
	a.buffer = a.buffer[:0]
	a.mu.Unlock()

	// Perform flush
	return a.doFlush(entries)
}

// flushLoop is the background flush goroutine.
func (a *AsyncWALWriter) flushLoop() {
	a.flushTimer = time.NewTimer(a.maxFlushTime)
	defer a.flushTimer.Stop()

	for {
		select {
		case <-a.flushCh:
			// Flush triggered by buffer full
			if !a.flushTimer.Stop() {
				<-a.flushTimer.C
			}
			a.flushTimer.Reset(a.maxFlushTime)
			a.doFlushFromBuffer()

		case <-a.flushTimer.C:
			// Flush triggered by timeout
			a.doFlushFromBuffer()
			a.flushTimer.Reset(a.maxFlushTime)

		case <-a.stopCh:
			// Shutdown - flush remaining
			a.doFlushFromBuffer()
			close(a.flushDone)
			return
		}
	}
}

// doFlushFromBuffer flushes the current buffer.
func (a *AsyncWALWriter) doFlushFromBuffer() {
	a.mu.Lock()
	if len(a.buffer) == 0 {
		a.mu.Unlock()
		return
	}

	entries := make([]WalEntry, len(a.buffer))
	copy(entries, a.buffer)
	a.buffer = a.buffer[:0]
	a.mu.Unlock()

	a.doFlush(entries)
}

// doFlush performs the actual flush operation.
func (a *AsyncWALWriter) doFlush(entries []WalEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Batch write all entries
	var lastErr error
	batchData := make([]byte, 0)
	for _, entry := range entries {
		// Simple batching - in production, would need proper framing
		batchData = append(batchData, entry.Data...)
	}

	// Write batch
	if len(batchData) > 0 {
		lastErr = a.WALWriter.Write(batchData)
	}

	// Update stats
	a.stats.mu.Lock()
	a.stats.totalFlushes++
	a.stats.avgBatchSize = float64(len(entries))
	a.stats.lastFlushTime = time.Now()
	a.stats.mu.Unlock()

	// Call callbacks
	for _, entry := range entries {
		if entry.Callback != nil {
			entry.Callback(lastErr)
		}
	}

	return lastErr
}

// Close shuts down the async writer and flushes remaining data.
func (a *AsyncWALWriter) Close() error {
	close(a.stopCh)
	<-a.flushDone
	return a.WALWriter.Close()
}

// Stats returns async write statistics.
func (a *AsyncWALWriter) Stats() (totalWrites, totalFlushes int64, avgBatchSize float64, lastFlush time.Time) {
	a.stats.mu.RLock()
	defer a.stats.mu.RUnlock()

	return a.stats.totalWrites, a.stats.totalFlushes, a.stats.avgBatchSize, a.stats.lastFlushTime
}

// SyncWALWriter provides synchronous write with batching.
// This is a middle ground between sync and async.
type SyncWALWriter struct {
	*WALWriter
	mu        sync.Mutex
	buffer    [][]byte
	maxSize   int
}

// NewSyncWALWriter creates a new sync WAL with batching.
func NewSyncWALWriter(base *WALWriter, maxBatchSize int) *SyncWALWriter {
	return &SyncWALWriter{
		WALWriter: base,
		buffer:    make([][]byte, 0, maxBatchSize),
		maxSize:   maxBatchSize,
	}
}

// Write batches writes and flushes when buffer is full.
func (s *SyncWALWriter) Write(data []byte) error {
	s.mu.Lock()
	s.buffer = append(s.buffer, data)
	shouldFlush := len(s.buffer) >= s.maxSize
	s.mu.Unlock()

	if shouldFlush {
		return s.Flush()
	}

	return nil
}

// Flush writes all buffered data.
func (s *SyncWALWriter) Flush() error {
	s.mu.Lock()
	if len(s.buffer) == 0 {
		s.mu.Unlock()
		return nil
	}

	data := make([][]byte, len(s.buffer))
	copy(data, s.buffer)
	s.buffer = s.buffer[:0]
	s.mu.Unlock()

	// Batch write
	for _, d := range data {
		if err := s.WALWriter.Write(d); err != nil {
			return err
		}
	}

	return nil
}
