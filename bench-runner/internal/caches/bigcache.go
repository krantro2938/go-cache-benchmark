// bench-runner/internal/caches/bigcache.go
package caches

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/allegro/bigcache/v3"
)

type BigCache struct {
	cache     *bigcache.BigCache
	hits      int64
	miss      int64
	evictions atomic.Int64
	mu        sync.RWMutex
}

// NewBigCache builds a BigCache with a hard limit of maxBytes. The number of
// shards (a power of two, at most 1024) is chosen so that every shard can hold
// at least four objects of the largest size in the workload; otherwise
// BigCache cannot store large objects under small budgets at all.
func NewBigCache(maxBytes int64, meanItemSize float64, maxItemSize int) (*BigCache, error) {
	shards := 1024
	for shards > 16 && maxBytes/int64(shards) < int64(4*maxItemSize) {
		shards /= 2
	}
	maxEntries := int(float64(maxBytes) / meanItemSize)
	if maxEntries < 1000 {
		maxEntries = 1000
	}

	b := &BigCache{}
	cache, err := bigcache.NewBigCache(bigcache.Config{
		OnRemoveWithReason: func(_ string, _ []byte, reason bigcache.RemoveReason) {
			if reason == bigcache.NoSpace {
				b.evictions.Add(1)
			}
		},
		Shards:             shards,
		LifeWindow:         24 * time.Hour, // long TTL: eviction is by size only
		MaxEntriesInWindow: maxEntries,
		MaxEntrySize:       maxItemSize,
		Verbose:            false,
		HardMaxCacheSize:   int(maxBytes >> 20), // MB
	})
	if err != nil {
		return nil, err
	}
	b.cache = cache
	return b, nil
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
	return Metrics{HitCount: b.hits, MissCount: b.miss, EvictionCount: b.evictions.Load()}
}

func (b *BigCache) Name() string { return "BigCache" }
