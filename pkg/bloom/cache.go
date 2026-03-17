package bloom

import (
	"sync"
	"time"
)

// CacheEntry represents a cached hash indices entry.
type CacheEntry struct {
	indices   []int
	timestamp time.Time
	hits      uint64
}

// IndexCache is an LRU cache for hot item indices.
// This reduces hash computation for frequently accessed items.
type IndexCache struct {
	mu       sync.RWMutex
	capacity int
	cache    map[string]*CacheEntry
	lru      []string // Simple LRU list
}

// NewIndexCache creates a new index cache with the given capacity.
func NewIndexCache(capacity int) *IndexCache {
	return &IndexCache{
		capacity: capacity,
		cache:    make(map[string]*CacheEntry, capacity),
		lru:      make([]string, 0, capacity),
	}
}

// Get retrieves cached indices for an item.
// Returns nil if not found (cache miss).
func (c *IndexCache) Get(key string) []int {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, found := c.cache[key]
	if !found {
		return nil
	}

	// Update hit count and timestamp
	entry.hits++
	entry.timestamp = time.Now()

	// Move to front of LRU list
	c.moveToFront(key)

	// Return a copy to prevent modification
	indices := make([]int, len(entry.indices))
	copy(indices, entry.indices)
	return indices
}

// Put stores indices in the cache.
func (c *IndexCache) Put(key string, indices []int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists
	if entry, found := c.cache[key]; found {
		entry.indices = indices
		entry.timestamp = time.Now()
		entry.hits++
		c.moveToFront(key)
		return
	}

	// Evict if at capacity
	if len(c.cache) >= c.capacity {
		c.evictOldest()
	}

	// Add new entry
	c.cache[key] = &CacheEntry{
		indices:   indices,
		timestamp: time.Now(),
		hits:      0,
	}
	c.lru = append(c.lru, key)
}

// Remove removes an item from the cache.
func (c *IndexCache) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, found := c.cache[key]; found {
		delete(c.cache, key)
		c.removeFromLRU(key)
	}
}

// Clear clears all entries from the cache.
func (c *IndexCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*CacheEntry, c.capacity)
	c.lru = make([]string, 0, c.capacity)
}

// Stats returns cache statistics.
func (c *IndexCache) Stats() (size int, capacity int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.cache), c.capacity
}

// moveToFront moves a key to the front of the LRU list.
func (c *IndexCache) moveToFront(key string) {
	c.removeFromLRU(key)
	c.lru = append(c.lru, key)
}

// removeFromLRU removes a key from the LRU list.
func (c *IndexCache) removeFromLRU(key string) {
	for i, k := range c.lru {
		if k == key {
			c.lru = append(c.lru[:i], c.lru[i+1:]...)
			break
		}
	}
}

// evictOldest removes the oldest entry from the cache.
func (c *IndexCache) evictOldest() {
	if len(c.lru) == 0 {
		return
	}

	oldest := c.lru[0]
	delete(c.cache, oldest)
	c.lru = c.lru[1:]
}

// CountingBloomFilterWithCache is a Bloom filter with index caching.
// This provides better performance for hot items.
type CountingBloomFilterWithCache struct {
	*CountingBloomFilter
	cache *IndexCache
}

// NewCountingBloomFilterWithCache creates a new Bloom filter with caching.
func NewCountingBloomFilterWithCache(m, k, cacheSize int) *CountingBloomFilterWithCache {
	return &CountingBloomFilterWithCache{
		CountingBloomFilter: NewCountingBloomFilter(m, k),
		cache:               NewIndexCache(cacheSize),
	}
}

// Add adds an item to the Bloom filter with caching.
func (cbf *CountingBloomFilterWithCache) Add(item []byte) error {
	key := string(item)

	// Get indices from cache or compute
	indices := cbf.cache.Get(key)
	if indices == nil {
		indices = getHashIndices(item, cbf.m, cbf.k)
		cbf.cache.Put(key, indices)
	}

	cbf.mu.Lock()
	defer cbf.mu.Unlock()

	for _, idx := range indices {
		if cbf.counters[idx] >= 255 {
			return ErrCounterOverflow
		}
		cbf.counters[idx]++
	}

	return nil
}

// Contains checks if an item might be in the Bloom filter with caching.
func (cbf *CountingBloomFilterWithCache) Contains(item []byte) bool {
	key := string(item)

	// Get indices from cache or compute
	indices := cbf.cache.Get(key)
	if indices == nil {
		indices = getHashIndices(item, cbf.m, cbf.k)
		cbf.cache.Put(key, indices)
	}

	cbf.mu.RLock()
	defer cbf.mu.RUnlock()

	for _, idx := range indices {
		if cbf.counters[idx] == 0 {
			return false
		}
	}

	return true
}

// Remove removes an item and invalidates cache.
func (cbf *CountingBloomFilterWithCache) Remove(item []byte) {
	key := string(item)
	cbf.cache.Remove(key) // Invalidate cache
	cbf.CountingBloomFilter.Remove(item)
}

// Cache returns the underlying cache for monitoring.
func (cbf *CountingBloomFilterWithCache) Cache() *IndexCache {
	return cbf.cache
}
