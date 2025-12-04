package caches

type Cache interface {
	Set(key string, value []byte, cost int64) bool
	Get(key string) ([]byte, bool)
	Close() error
	Metrics() Metrics
	Name() string
}

type Metrics struct {
	HitCount      int64
	MissCount     int64
	EvictionCount int64
}
