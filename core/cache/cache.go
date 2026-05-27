// Package cache provides a SQLite-backed caching layer for the tau multi-agent system.
//
// It offers middleware wrappers for Provider (LLM), Embedder, and Tool execution
// that transparently cache results, reducing API calls and improving latency.
// All middleware is provider-agnostic and injection-based — zero changes to core/loop.go.
package cache

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"
)

// CacheBackend defines the interface for arbitrary key-value caching.
type CacheBackend interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte, ttl time.Duration)
	Delete(key string)
	Flush()
	Stats() CacheStats
}

// CacheStats holds aggregated cache metrics suitable for Prometheus-style monitoring.
type CacheStats struct {
	Hits      int64 `json:"hits"`
	Misses    int64 `json:"misses"`
	ItemCount int   `json:"item_count"`
	MaxSize   int   `json:"max_size"`
}

// SQLiteCacheBackend stores cached items in a SQLite table, reusing the
// same *sql.DB connection that backs the main tau session store.
type SQLiteCacheBackend struct {
	db      *sql.DB
	mu      sync.RWMutex
	maxSize int
	hits    int64
	misses  int64
}

// NewSQLiteCache creates the cache table (if needed) and starts a
// background goroutine that cleans expired items every 5 minutes.
// maxSize ≤ 0 disables the size cap.
func NewSQLiteCache(db *sql.DB, maxSize int) (*SQLiteCacheBackend, error) {
	if db == nil {
		return nil, fmt.Errorf("cache: db must not be nil")
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS cache (
		key        TEXT PRIMARY KEY,
		value      BLOB NOT NULL,
		expires_at INTEGER NOT NULL
	)`); err != nil {
		return nil, fmt.Errorf("cache: create table: %w", err)
	}
	c := &SQLiteCacheBackend{db: db, maxSize: maxSize}
	go c.periodicCleanup(5 * time.Minute)
	return c, nil
}

// periodicCleanup removes expired rows on a ticker.
func (c *SQLiteCacheBackend) periodicCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		c.mu.Lock()
		_, _ = c.db.Exec("DELETE FROM cache WHERE expires_at < ?", time.Now().UnixMilli())
		c.mu.Unlock()
	}
}

// Get retrieves a value from the cache. Expired entries are treated as a miss
// and are lazily deleted.
func (c *SQLiteCacheBackend) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var val []byte
	var expires int64
	err := c.db.QueryRow("SELECT value, expires_at FROM cache WHERE key = ?", key).
		Scan(&val, &expires)
	if err != nil || expires < time.Now().UnixMilli() {
		c.misses++
		if err == nil {
			// expired in DB but we still found the row — clean it
			_, _ = c.db.Exec("DELETE FROM cache WHERE key = ?", key)
		}
		return nil, false
	}
	c.hits++
	return val, true
}

// Set stores a value with a time-to-live. Insert or replace semantics.
func (c *SQLiteCacheBackend) Set(key string, value []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expires := time.Now().Add(ttl).UnixMilli()
	_, err := c.db.Exec(
		"INSERT OR REPLACE INTO cache(key, value, expires_at) VALUES(?, ?, ?)",
		key, value, expires,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[cache] set %s: %v\n", key[:min(8, len(key))], err)
	}
	c.enforceMaxSize()
}

// Delete removes a single key.
func (c *SQLiteCacheBackend) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, _ = c.db.Exec("DELETE FROM cache WHERE key = ?", key)
}

// Flush deletes every cached entry and resets counters.
func (c *SQLiteCacheBackend) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, _ = c.db.Exec("DELETE FROM cache")
	c.hits, c.misses = 0, 0
}

// Stats returns current cache metrics (only non-expired items are counted).
func (c *SQLiteCacheBackend) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var count int
	_ = c.db.QueryRow(
		"SELECT COUNT(*) FROM cache WHERE expires_at > ?", time.Now().UnixMilli(),
	).Scan(&count)

	// Keep a snapshot of the volatile counters.
	hits := c.hits
	misses := c.misses

	return CacheStats{
		Hits:      hits,
		Misses:    misses,
		ItemCount: count,
		MaxSize:   c.maxSize,
	}
}

// enforceMaxSize evicts the soonest-to-expire entries when count > maxSize.
func (c *SQLiteCacheBackend) enforceMaxSize() {
	if c.maxSize <= 0 {
		return
	}
	var count int
	_ = c.db.QueryRow("SELECT COUNT(*) FROM cache").Scan(&count)
	if excess := count - c.maxSize; excess > 0 {
		_, _ = c.db.Exec(
			"DELETE FROM cache WHERE key IN (SELECT key FROM cache ORDER BY expires_at ASC LIMIT ?)",
			excess,
		)
	}
}

// ---- Key helpers -----------------------------------------------------------

// HashKey produces a short hex digest from concatenated parts.
func HashKey(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// LLMCacheKey returns a namespaced key for LLM request/response caching.
func LLMCacheKey(model string, messagesDigest string) string {
	return "llm:" + HashKey(model, messagesDigest)
}

// EmbeddingCacheKey returns a namespaced key for embedding caching.
func EmbeddingCacheKey(embedder string, text string) string {
	return "emb:" + HashKey(embedder, text)
}

// ToolCacheKey returns a namespaced key for tool result caching.
func ToolCacheKey(toolName string, paramsDigest string) string {
	return "tool:" + HashKey(toolName, paramsDigest)
}
