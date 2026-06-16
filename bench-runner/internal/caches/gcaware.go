package caches

import "cache-bench/internal/gcaware"

// GCAwareCache adapts the gcaware.Cache (Adaptive TinyLFU admission + GC-aware
// cost function) to the benchmark's Cache interface. It is used look-aside:
// Get records the access (via Lookup) and Set admits under the policy.
type GCAwareCache struct {
	cache *gcaware.Cache
}

// NewGCAwareCache builds a GC-aware cache with a byte budget of maxBytes,
// using the paper's default parameters (λ=0.7, k_h=5, k_s=5, adaptive window).
func NewGCAwareCache(maxBytes int64) (Cache, error) {
	c := gcaware.New(gcaware.DefaultConfig(uintptr(maxBytes)))
	return &GCAwareCache{cache: c}, nil
}

// NewGCAwareCacheLambda is like NewGCAwareCache but overrides λ (the
// utility/allocation-cost balance) for the lambda sweep.
func NewGCAwareCacheLambda(maxBytes int64, lambda float64) (Cache, error) {
	cfg := gcaware.DefaultConfig(uintptr(maxBytes))
	cfg.Lambda = lambda
	c := gcaware.New(cfg)
	return &GCAwareCache{cache: c}, nil
}

// Set admits value under the GC-aware admission policy. The cost argument is
// ignored: gcaware computes its own GC-aware cost C(x) = s(x) + m_gc(x).
// It does NOT record an access (look-aside ordering: Get/Lookup records it).
func (g *GCAwareCache) Set(key string, value []byte, cost int64) bool {
	g.cache.Set(key, value)
	return true
}

// Get is a look-aside read: it records the access and returns the cached value,
// without loading or admitting on a miss.
func (g *GCAwareCache) Get(key string) ([]byte, bool) {
	return g.cache.Lookup(key)
}

func (g *GCAwareCache) Close() error { return nil }

func (g *GCAwareCache) Metrics() Metrics {
	st := g.cache.Stats()
	return Metrics{
		HitCount:      int64(st.Hits),
		MissCount:     int64(st.Misses),
		EvictionCount: int64(st.Evictions),
	}
}

func (g *GCAwareCache) Name() string { return "GCAware" }
