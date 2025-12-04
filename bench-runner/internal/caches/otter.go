// bench-runner/internal/caches/otter.go
package caches

import (
	"github.com/maypok86/otter"
)

type OtterCache struct {
	cache otter.Cache[string, []byte]
}

func NewOtterCache(maxCost int) (*OtterCache, error) {
	builder, err := otter.NewBuilder[string, []byte](maxCost)
	if err != nil {
		return nil, err
	}
	// Configure cost function to track bytes, not items
	builder.Cost(func(key string, value []byte) uint32 {
		return uint32(len(value))
	})
	
	c, err := builder.Build()
	if err != nil {
		return nil, err
	}
	return &OtterCache{cache: c}, nil
}

func (o *OtterCache) Set(key string, value []byte, cost int64) bool {
	return o.cache.Set(key, value)
}

func (o *OtterCache) Get(key string) ([]byte, bool) {
	return o.cache.Get(key)
}

func (o *OtterCache) Close() error {
	o.cache.Close()
	return nil
}

func (o *OtterCache) Metrics() Metrics {
	stats := o.cache.Stats()
	return Metrics{
		HitCount:  int64(stats.Hits()),
		MissCount: int64(stats.Misses()),
	}
}

func (o *OtterCache) Name() string {
	return "Otter"
}
