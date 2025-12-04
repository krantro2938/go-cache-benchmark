package caches

import "github.com/dgraph-io/ristretto"

type RistrettoCache struct {
	cache *ristretto.Cache
}

func NewRistrettoCache(maxCost int64) (*RistrettoCache, error) {
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 10 * 1_000_000,
		MaxCost:     maxCost,
		BufferItems: 64,
		Metrics:     true,
	})
	if err != nil {
		return nil, err
	}
	return &RistrettoCache{cache: cache}, nil
}

func (r *RistrettoCache) Set(key string, value []byte, cost int64) bool {
	return r.cache.Set(key, value, cost)
}

func (r *RistrettoCache) Get(key string) ([]byte, bool) {
	val, ok := r.cache.Get(key)
	if !ok {
		return nil, false
	}
	if b, ok := val.([]byte); ok {
		return b, true
	}
	return nil, false
}

func (r *RistrettoCache) Close() error {
	r.cache.Close()
	return nil
}

func (r *RistrettoCache) Metrics() Metrics {
	m := r.cache.Metrics
	return Metrics{
		HitCount:  int64(m.Hits()),
		MissCount: int64(m.Misses()),
		// EvictionCount: int64(m.KeysEvicted), if you want
	}
}

func (r *RistrettoCache) Name() string { return "Ristretto" }
