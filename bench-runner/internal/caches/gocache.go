// bench-runner/internal/caches/gocache.go
package caches

import (
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

type GoCache struct {
	cache *cache.Cache
	hits  int64
	miss  int64
	mu    sync.RWMutex
}

func NewGoCache(maxItems int) *GoCache {
	// No default expiration, cleanup every 10 mins
	c := cache.New(24*time.Hour, 10*time.Minute)
	return &GoCache{cache: c}
}

func (g *GoCache) Set(key string, value []byte, _ int64) bool {
	g.cache.Set(key, value, cache.DefaultExpiration)
	return true
}

func (g *GoCache) Get(key string) ([]byte, bool) {
	val, found := g.cache.Get(key)
	if !found {
		g.mu.Lock()
		g.miss++
		g.mu.Unlock()
		return nil, false
	}
	if b, ok := val.([]byte); ok {
		g.mu.Lock()
		g.hits++
		g.mu.Unlock()
		return b, true
	}
	return nil, false
}

func (g *GoCache) Close() error { return nil }

func (g *GoCache) Metrics() Metrics {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return Metrics{HitCount: g.hits, MissCount: g.miss}
}

func (g *GoCache) Name() string { return "GoCache" }
