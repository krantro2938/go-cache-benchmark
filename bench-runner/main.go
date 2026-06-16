package main

import (
	"cache-bench/internal/bench"
	"cache-bench/internal/caches"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
)

const BaseValueSize = 1024

func main() {
	w, err := bench.NewDataWriter()
	if err != nil {
		log.Fatal(err)
	}
	defer w.Flush()

	// === 1. Vary CACHE SIZE (for hit ratio vs memory graph) ===
	cacheSizes := []struct {
		id   string
		size int64
	}{
		{"size_64mb", 64 << 20},
		{"size_128mb", 128 << 20},
		{"size_256mb", 256 << 20},
		{"size_512mb", 512 << 20},
		{"size_1gb", 1024 << 20},
	}

	for _, cs := range cacheSizes {
		fmt.Printf("=== Running config: %s ===\n", cs.id)
		workload := bench.GenerateWorkload(bench.WorkloadConfig{
			Seed:         42,
			KeySpaceSize: 5_000_000,   // 5M keys
			TotalOps:     10_000_000,  // 10M ops
			ValueSize:    BaseValueSize,
			Skew:         0.0, // Uniform distribution to force evictions
		})

		runBenchmarks(cs.id, cs.size, workload, w)
	}

	// === 2. Vary VALUE SIZE (for throughput vs object size) ===
	valueSizes := []struct {
		id   string
		size int
	}{
		{"val_256b", 256},
		{"val_1kb", 1024},
		{"val_4kb", 4096},
		{"val_16kb", 16384},
		{"val_64kb", 65536},
	}

	for _, vs := range valueSizes {
		fmt.Printf("=== Running config: %s ===\n", vs.id)
		workload := bench.GenerateWorkload(bench.WorkloadConfig{
			Seed:         42,
			KeySpaceSize: 500_000,     // 500k keys
			TotalOps:     2_000_000,   // 2M ops
			ValueSize:    vs.size,
			Skew:         0.95,
		})

		// Use fixed 512MB cache for this test
		runBenchmarks(vs.id, 512<<20, workload, w)
	}

	// === 3. Vary SKEW (for adaptivity analysis) ===
	skews := []struct {
		id   string
		skew float64
	}{
		{"skew_0.80", 0.80},
		{"skew_0.90", 0.90},
		{"skew_0.95", 0.95},
		{"skew_0.99", 0.99},
	}

	for _, s := range skews {
		fmt.Printf("=== Running config: %s ===\n", s.id)
		workload := bench.GenerateWorkload(bench.WorkloadConfig{
			Seed:         42,
			KeySpaceSize: 1_000_000,   // 1M keys
			TotalOps:     5_000_000,   // 5M ops
			ValueSize:    BaseValueSize,
			Skew:         s.skew,
		})

		runBenchmarks(s.id, 256<<20, workload, w)
	}

	// === 4. Lambda sweep (GCAware only) ===
	// One fixed config: 256MB cache, skew 0.99, standard skew-group key/op counts.
	runLambdaSweep()
}

// runLambdaSweep benchmarks GCAware across a range of λ values over a single
// fixed config and writes hit ratio / memory / alloc rate per λ to
// /app/results/lambda_sweep.csv.
func runLambdaSweep() {
	fmt.Printf("=== Running lambda sweep (GCAware, 256MB, skew 0.99) ===\n")

	const cacheSizeBytes = 256 << 20
	workload := bench.GenerateWorkload(bench.WorkloadConfig{
		Seed:         42,
		KeySpaceSize: 1_000_000, // 1M keys (standard skew-group counts)
		TotalOps:     5_000_000, // 5M ops
		ValueSize:    BaseValueSize,
		Skew:         0.99,
	})

	os.MkdirAll("/app/results", 0755)
	f, err := os.Create("/app/results/lambda_sweep.csv")
	if err != nil {
		log.Printf("lambda sweep: cannot create csv: %v", err)
		return
	}
	defer f.Close()
	cw := csv.NewWriter(f)
	defer cw.Flush()
	cw.Write([]string{"lambda", "hit_ratio", "memory_mb", "alloc_rate_mb_per_sec"})

	lambdas := []float64{0.0, 0.3, 0.5, 0.7, 0.9, 1.0}
	for _, lambda := range lambdas {
		c, err := caches.NewGCAwareCacheLambda(cacheSizeBytes, lambda)
		if err != nil {
			log.Printf("lambda sweep: cannot build cache for λ=%.1f: %v", lambda, err)
			continue
		}
		fmt.Printf("  → GCAware λ=%.1f\n", lambda)
		result := bench.RunBenchmark(c, workload)
		hitRatio := float64(result.Hits) / float64(result.Hits+result.Misses)
		cw.Write([]string{
			strconv.FormatFloat(lambda, 'f', 1, 64),
			strconv.FormatFloat(hitRatio, 'f', 4, 64),
			strconv.FormatFloat(result.MemoryMB, 'f', 2, 64),
			strconv.FormatFloat(result.AllocRate, 'f', 2, 64),
		})
		cw.Flush()
		c.Close()
		runtime.GC()
	}
}

func runBenchmarks(configID string, cacheSizeBytes int64, workload *bench.Workload, w *bench.DataWriter) {
	// Helper to run a single cache benchmark
	runCache := func(c caches.Cache) {
		// Ensure we clean up this cache before moving to the next one
		defer runtime.GC()
		defer c.Close()

		fmt.Printf("  → %s\n", c.Name())

		result := bench.RunBenchmark(c, workload)

		hitRatio := float64(result.Hits) / float64(result.Hits+result.Misses)
		tps := float64(result.TotalOps) / result.Duration.Seconds()
		p50 := bench.Percentile(result.Latencies, 0.50).Microseconds()
		p95 := bench.Percentile(result.Latencies, 0.95).Microseconds()
		p99 := bench.Percentile(result.Latencies, 0.99).Microseconds()

		w.WriteLatency(configID, c.Name(), float64(p50), float64(p95), float64(p99))
		w.WriteHitRatio(configID, c.Name(), hitRatio)
		w.WriteThroughput(configID, c.Name(), tps)
		w.WriteEvictions(configID, c.Name(), result.Evictions)
		w.WriteMemory(configID, c.Name(), result.MemoryMB)
		w.WriteGC(configID, c.Name(), result.AllocsPerOp, result.GCPause, result.GCCycles, result.AllocRate)
	}

	// Ristretto
	if r, err := caches.NewRistrettoCache(cacheSizeBytes); err == nil {
		runCache(r)
	}

	// Otter
	if o, err := caches.NewOtterCache(int(cacheSizeBytes)); err == nil { // Otter ignores size arg
		runCache(o)
	}

	// BigCache
	if b, err := caches.NewBigCache(cacheSizeBytes); err == nil {
		runCache(b)
	}

	// GoCache (approximate size)
	maxItems := int(cacheSizeBytes / int64(BaseValueSize))
	gc := caches.NewGoCache(maxItems)
	runCache(gc)

	// GCAware (Adaptive TinyLFU admission + GC-aware cost function)
	if g, err := caches.NewGCAwareCache(cacheSizeBytes); err == nil {
		runCache(g)
	}
}
