package bench

import (
	"runtime"
	"sort"
	"time"

	"cache-bench/internal/caches"
)

// BenchmarkResult holds the metrics of one cache on one workload.
type BenchmarkResult struct {
	CacheName   string
	Hits        int64
	Misses      int64
	HitBytes    int64 // bytes served from the cache
	ReqBytes    int64 // bytes requested in total
	Evictions   int64
	TotalOps    int
	MemoryMB    float64 // heap held by the cache: HeapInuse delta (incl. span fragmentation), MB
	LiveMB      float64 // live objects retained by the cache: HeapAlloc delta, MB
	P50, P95    float64 // latency percentiles, µs
	P99         float64
	AllocsPerOp float64 // heap allocations per operation
	GCPause     float64 // mean stop-the-world pause per GC cycle, µs
	GCCycles    float64 // GC cycles per second
	AllocRate   float64 // MB allocated per second
	AllocPerOp  float64 // bytes allocated per operation (independent of speed)
	Duration    time.Duration
}

// RunBenchmark builds a cache with newCache and replays the workload against
// it in look-aside mode: Get; on a miss, load the object (allocate a fresh
// buffer of the key's size) and Set it.
//
// Measurement protocol:
//   - the latency buffer is allocated and a forced GC is run BEFORE the cache
//     is constructed, and HeapInuse is recorded as the baseline;
//   - per-operation latency covers the cache calls only (Get, and Set on a
//     miss), not the simulated load;
//   - GC and allocation metrics are the difference of runtime.MemStats taken
//     right before and right after the request loop (before any forced GC);
//   - memory is HeapInuse after a forced GC with the cache still alive,
//     minus the baseline, i.e. the heap spans held by the cache and its data
//     (including fragmentation of partially used spans); live memory is the
//     same difference for HeapAlloc (bytes of live objects only).
func RunBenchmark(newCache func() (caches.Cache, error), w *Workload) (*BenchmarkResult, error) {
	n := len(w.Keys)
	lat := make([]time.Duration, n)

	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)

	cache, err := newCache()
	if err != nil {
		return nil, err
	}
	defer cache.Close()

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	var hits, misses, hitBytes, reqBytes int64
	start := time.Now()
	for i, key := range w.Keys {
		t0 := time.Now()
		_, ok := cache.Get(key)
		lat[i] = time.Since(t0)
		reqBytes += int64(w.Sizes[i])
		if ok {
			hits++
			hitBytes += int64(w.Sizes[i])
			continue
		}
		misses++
		// Simulated fetch from the source: a fresh object of the key's size.
		v := make([]byte, w.Sizes[i])
		copy(v, w.Source)
		t1 := time.Now()
		cache.Set(key, v, int64(len(v)))
		lat[i] += time.Since(t1)
	}
	duration := time.Since(start)

	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	runtime.GC()
	var retained runtime.MemStats
	runtime.ReadMemStats(&retained)
	memoryMB := (float64(retained.HeapInuse) - float64(base.HeapInuse)) / (1 << 20)
	liveMB := (float64(retained.HeapAlloc) - float64(base.HeapAlloc)) / (1 << 20)
	metrics := cache.Metrics()
	runtime.KeepAlive(cache)

	cycles := after.NumGC - before.NumGC
	pause := 0.0
	if cycles > 0 {
		pause = float64(after.PauseTotalNs-before.PauseTotalNs) / float64(cycles) / 1000
	}

	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	us := func(p float64) float64 {
		idx := int(float64(n-1) * p)
		return float64(lat[idx].Nanoseconds()) / 1000
	}

	return &BenchmarkResult{
		CacheName:   cache.Name(),
		Hits:        hits,
		Misses:      misses,
		HitBytes:    hitBytes,
		ReqBytes:    reqBytes,
		Evictions:   metrics.EvictionCount,
		TotalOps:    n,
		MemoryMB:    memoryMB,
		LiveMB:      liveMB,
		P50:         us(0.50),
		P95:         us(0.95),
		P99:         us(0.99),
		AllocsPerOp: float64(after.Mallocs-before.Mallocs) / float64(n),
		GCPause:     pause,
		GCCycles:    float64(cycles) / duration.Seconds(),
		AllocRate:   float64(after.TotalAlloc-before.TotalAlloc) / (1 << 20) / duration.Seconds(),
		AllocPerOp:  float64(after.TotalAlloc-before.TotalAlloc) / float64(n),
		Duration:    duration,
	}, nil
}
