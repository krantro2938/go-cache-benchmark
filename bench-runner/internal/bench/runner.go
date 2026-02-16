package bench

import (
	"runtime"
	"sort"
	"sync"
	"time"

	"cache-bench/internal/caches"
)

type BenchmarkResult struct {
	CacheName   string
	Latencies   []time.Duration
	Hits        int64
	Misses      int64
	Evictions   int64
	TotalOps    int
	MemoryMB    float64
	P50         time.Duration
	P95         time.Duration
	P99         time.Duration
	AllocsPerOp float64   // Allocations per operation (thousands)
	GCPause     float64   // Total GC pause in microseconds
	GCCycles    float64   // GC cycles per second
	AllocRate   float64   // Allocation rate in MB/s
	Duration    time.Duration
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

	// Capture initial memory stats before benchmark
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	startTime := time.Now()

	var wg sync.WaitGroup
	// Run sequentially
	concurrency := 1
	chunkSize := len(workload.Operations) / concurrency

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for j := start; j < end; j++ {
				op := workload.Operations[j]
				// Generate key on the fly to save memory
				key := GenerateKey(op.KeyID)
				
				opStart := time.Now()
				if _, ok := cache.Get(key); ok {
					mu.Lock()
					hits++
					mu.Unlock()
				} else {
					cache.Set(key, workload.SharedValue, int64(len(workload.SharedValue)))
					mu.Lock()
					misses++
					mu.Unlock()
				}
				lat := time.Since(opStart)
				mu.Lock()
				latencies = append(latencies, lat)
				mu.Unlock()
			}
		}(i*chunkSize, (i+1)*chunkSize)
	}
	wg.Wait()

	duration := time.Since(startTime)

	// Capture memory stats after benchmark
	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	memoryMB := float64(memAfter.Alloc) / 1024 / 1024

	// Calculate GC and allocation metrics
	totalOps := len(workload.Operations)
	allocsPerOp := float64(memAfter.Mallocs-memBefore.Mallocs) / float64(totalOps) / 1000 // in thousands
	gcPause := float64(memAfter.PauseTotalNs-memBefore.PauseTotalNs) / 1000               // in microseconds
	gcCycles := float64(memAfter.NumGC-memBefore.NumGC) / duration.Seconds()              // cycles per second
	allocRate := float64(memAfter.TotalAlloc-memBefore.TotalAlloc) / 1024 / 1024 / duration.Seconds() // MB/s

	// Sort latencies for percentiles
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p50 := percentile(latencies, 0.50)
	p95 := percentile(latencies, 0.95)
	p99 := percentile(latencies, 0.99)

	metrics := cache.Metrics()
	return &BenchmarkResult{
		CacheName:   cache.Name(),
		Latencies:   latencies,
		Hits:        hits,
		Misses:      misses,
		Evictions:   metrics.EvictionCount,
		TotalOps:    totalOps,
		MemoryMB:    memoryMB,
		P50:         p50,
		P95:         p95,
		P99:         p99,
		AllocsPerOp: allocsPerOp,
		GCPause:     gcPause,
		GCCycles:    gcCycles,
		AllocRate:   allocRate,
		Duration:    duration,
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
