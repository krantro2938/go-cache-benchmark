package bench

import (
	"runtime"
	"sort"
	"sync"
	"time"

	"cache-bench/internal/caches"
)

type BenchmarkResult struct {
	CacheName string
	Latencies []time.Duration
	Hits      int64
	Misses    int64
	Evictions int64
	TotalOps  int
	MemoryMB  float64
	P50       time.Duration
	P95       time.Duration
	P99       time.Duration
}

// Helper to get percentile
func percentile(latencies []time.Duration, p float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	index := int(float64(len(latencies)-1) * p)
	if index < 0 {
		index = 0
	}
	return latencies[index]
}

func RunBenchmark(cache caches.Cache, workload *Workload) *BenchmarkResult {
	var hits, misses int64
	var latencies []time.Duration
	var mu sync.Mutex

	// Pre-allocate to avoid GC during test
	latencies = make([]time.Duration, 0, len(workload.Operations))

	var wg sync.WaitGroup
	// Run in 4 parallel goroutines (simulate concurrency)
	concurrency := 4
	chunkSize := len(workload.Operations) / concurrency

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for j := start; j < end; j++ {
				op := workload.Operations[j]
				startTime := time.Now()
				if _, ok := cache.Get(op.Key); ok {
					mu.Lock()
					hits++
					mu.Unlock()
				} else {
					cache.Set(op.Key, op.Value, int64(len(op.Value)))
					mu.Lock()
					misses++
					mu.Unlock()
				}
				lat := time.Since(startTime)
				mu.Lock()
				latencies = append(latencies, lat)
				mu.Unlock()
			}
		}(i*chunkSize, (i+1)*chunkSize)
	}
	wg.Wait()

	// Capture memory
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	memoryMB := float64(m.Alloc) / 1024 / 1024

	// Sort latencies for percentiles
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p50 := percentile(latencies, 0.50)
	p95 := percentile(latencies, 0.95)
	p99 := percentile(latencies, 0.99)

	metrics := cache.Metrics()
	return &BenchmarkResult{
		CacheName: cache.Name(),
		Latencies: latencies,
		Hits:      hits + metrics.HitCount,
		Misses:    misses + metrics.MissCount,
		Evictions: metrics.EvictionCount,
		TotalOps:  len(workload.Operations),
		MemoryMB:  memoryMB,
		P50:       p50,
		P95:       p95,
		P99:       p99,
	}
}

func CalculateThroughput(result *BenchmarkResult, duration time.Duration) float64 {
	return float64(result.TotalOps) / duration.Seconds()
}

func Percentile(latencies []time.Duration, p float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	index := int(float64(len(latencies)-1) * p)
	if index < 0 {
		index = 0
	}
	if index >= len(latencies) {
		index = len(latencies) - 1
	}
	return latencies[index]
}
