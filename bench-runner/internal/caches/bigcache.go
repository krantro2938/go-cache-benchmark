// bench-runner/internal/caches/bigcache.go
package caches

import (
	"sync"
	"time"

	"github.com/allegro/bigcache/v3"
)

type BigCache struct {
	cache *bigcache.BigCache
	hits  int64
	miss  int64
	mu    sync.RWMutex
}

func NewBigCache(maxBytes int64) (*BigCache, error) {
	// Estimate max entries: assume 1KB per item
	maxEntries := int(maxBytes / 1024)
	if maxEntries < 1000 {
		maxEntries = 1000
	}

	cache, err := bigcache.NewBigCache(bigcache.Config{
		Shards:             1024,
		LifeWindow:         24 * time.Hour, // long TTL
		MaxEntriesInWindow: maxEntries,
		MaxEntrySize:       2048, // max item size
		Verbose:            false,
		HardMaxCacheSize:   int(maxBytes >> 20), // MB
	})
	if err != nil {
		return nil, err
	}
	return &BigCache{cache: cache}, nil
}

func (b *BigCache) Set(key string, value []byte, _ int64) bool {
	return b.cache.Set(key, value) == nil
}

func (b *BigCache) Get(key string) ([]byte, bool) {
	val, err := b.cache.Get(key)
	if err != nil {
		b.mu.Lock()
		b.miss++
		b.mu.Unlock()
		return nil, false
	}
	b.mu.Lock()
	b.hits++
	b.mu.Unlock()
	return val, true
}

func (b *BigCache) Close() error { return nil }

func (b *BigCache) Metrics() Metrics {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return Metrics{HitCount: b.hits, MissCount: b.miss}
}

func (b *BigCache) Name() string { return "BigCache" }
