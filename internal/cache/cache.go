package cache

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
)

// Cache stores normalized-prompt → answer mappings.
// Thread-safe for concurrent access.
type Cache struct {
	mu    sync.RWMutex
	data  map[string]cacheEntry
	hits  int
	misses int
}

type cacheEntry struct {
	answer string
}

// New creates an empty cache.
func New() *Cache {
	return &Cache{data: make(map[string]cacheEntry)}
}

// normalize strips whitespace, lowers case, and removes punctuation
// so that semantically identical prompts match the same cache key.
func normalize(prompt string) string {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	var b strings.Builder
	b.Grow(len(lower))
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// Get returns a cached answer and true if found.
func (c *Cache) Get(prompt string) (string, bool) {
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(normalize(prompt))))
	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()
	if ok {
		c.mu.Lock()
		c.hits++
		c.mu.Unlock()
		return e.answer, true
	}
	c.mu.Lock()
	c.misses++
	c.mu.Unlock()
	return "", false
}

// Set stores an answer for a prompt.
func (c *Cache) Set(prompt, answer string) {
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(normalize(prompt))))
	c.mu.Lock()
	c.data[key] = cacheEntry{answer: answer}
	c.mu.Unlock()
}

// Stats returns hit/miss counts.
func (c *Cache) Stats() (int, int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses
}
