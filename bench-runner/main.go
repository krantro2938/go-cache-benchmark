// Command bench-runner reproduces the experiments of the paper
// "Адаптивное локальное кэширование в микросервисных системах с учетом
// особенностей сборки мусора".
//
// One invocation performs one run of every experiment series and writes
// RESULTS_DIR/results.csv (one row per configuration and cache) and
// RESULTS_DIR/env.json (runtime and machine description).
//
// Environment variables:
//
//	RUN_SEED     workload seed (default 1); run-benchmarks.sh uses the run number
//	RESULTS_DIR  output directory (default /app/results)
//	BENCH_SCALE  multiplies key-space sizes, request counts and cache budgets
//	             (default 1; e.g. 0.05 for a quick smoke test)
//	SERIES       comma-separated subset of: size,value,skew,mixed,lambda
//	             (default: all)
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"cache-bench/internal/bench"
	"cache-bench/internal/caches"
)

var (
	seed    = envInt("RUN_SEED", 1)
	scale   = envFloat("BENCH_SCALE", 1)
	outDir  = envStr("RESULTS_DIR", "/app/results")
	enabled = map[string]bool{}
)

func main() {
	for _, s := range strings.Split(envStr("SERIES", "size,value,skew,mixed,lambda"), ",") {
		enabled[strings.TrimSpace(s)] = true
	}
	var err error
	writer, err = bench.NewDataWriter(outDir)
	if err != nil {
		log.Fatal(err)
	}
	defer writer.Close()
	writeEnv()

	// 1. Cache budget: uniform keys, 1 KB objects (hit ratio vs. budget).
	if enabled["size"] {
		for _, mb := range []int64{64, 128, 256, 512, 1024} {
			cfg := bench.WorkloadConfig{KeySpaceSize: sc(5_000_000), TotalOps: sc(10_000_000), ValueSize: 1024, Skew: 0}
			run("size", fmt.Sprintf("size_%dmb", mb), cfg, scb(mb<<20), allCaches)
		}
	}

	// 2. Object size: Zipf s = 0.95, 512 MB budget.
	if enabled["value"] {
		for _, v := range []struct {
			id   string
			size int
		}{{"val_256b", 256}, {"val_1kb", 1024}, {"val_4kb", 4096}, {"val_16kb", 16384}, {"val_64kb", 65536}} {
			cfg := bench.WorkloadConfig{KeySpaceSize: sc(500_000), TotalOps: sc(2_000_000), ValueSize: v.size, Skew: 0.95}
			run("value", v.id, cfg, scb(512<<20), allCaches)
		}
	}

	// 3. Skew: 1 KB objects, 256 MB budget.
	if enabled["skew"] {
		for _, s := range []float64{0.80, 0.90, 0.95, 0.99} {
			cfg := bench.WorkloadConfig{KeySpaceSize: sc(1_000_000), TotalOps: sc(5_000_000), ValueSize: 1024, Skew: s}
			run("skew", fmt.Sprintf("skew_%.2f", s), cfg, scb(256<<20), allCaches)
		}
	}

	// 4. Heterogeneous object sizes, tight budget: the case the GC-aware
	// cost term is designed for.
	if enabled["mixed"] {
		run("mixed", "mixed_16mb", mixedConfig(), scb(16<<20), allCaches)
	}

	// 5. λ sweep of the proposed method on the heterogeneous workload.
	if enabled["lambda"] {
		for _, l := range []float64{0, 0.1, 0.3, 0.5, 0.7, 0.9, 1.0} {
			l := l
			run("lambda", fmt.Sprintf("lambda_%.1f", l), mixedConfig(), scb(16<<20),
				func(budget int64, wl *bench.Workload) []func() (caches.Cache, error) {
					return []func() (caches.Cache, error){
						func() (caches.Cache, error) { return caches.NewGCAwareCacheLambda(budget, l) },
					}
				})
		}
	}
}

func mixedConfig() bench.WorkloadConfig {
	return bench.WorkloadConfig{
		KeySpaceSize: sc(200_000), TotalOps: sc(3_000_000), Skew: 0.9,
		SizeMix: []int{256, 1024, 16384, 65536}, SizeWeights: []int{70, 20, 7, 3},
	}
}

// allCaches lists the compared systems. Go-Cache is not included: it has no
// size limit, so it cannot be compared under an equal memory budget (with
// realistic, separately allocated objects it simply grows with the key space).
func allCaches(budget int64, wl *bench.Workload) []func() (caches.Cache, error) {
	return []func() (caches.Cache, error){
		func() (caches.Cache, error) { return caches.NewRistrettoCache(budget, wl.MeanSize) },
		func() (caches.Cache, error) { return caches.NewAdaptiveTinyLFU(budget) },
		func() (caches.Cache, error) { return caches.NewGCAwareCache(budget) },
		func() (caches.Cache, error) { return caches.NewBigCache(budget, wl.MeanSize, wl.MaxSize) },
	}
}

func run(series, id string, cfg bench.WorkloadConfig, budget int64,
	list func(int64, *bench.Workload) []func() (caches.Cache, error)) {
	cfg.Seed = int64(seed)
	if cfg.ValueSize == 0 && len(cfg.SizeMix) == 0 {
		cfg.ValueSize = 1024
	}
	fmt.Printf("=== %s / %s (seed %d) ===\n", series, id, seed)
	wl := bench.GenerateWorkload(cfg)
	for _, mk := range list(budget, wl) {
		res, err := bench.RunBenchmark(mk, wl)
		if err != nil {
			log.Printf("  %s: %v", id, err)
			continue
		}
		fmt.Printf("  %-18s hit=%.4f ops/s=%.0f p99=%.2fus mem=%.1fMB alloc=%.1fMB/s pause=%.1fus\n",
			res.CacheName, float64(res.Hits)/float64(res.TotalOps),
			float64(res.TotalOps)/res.Duration.Seconds(), res.P99, res.MemoryMB, res.AllocRate, res.GCPause)
		if err := writer.Write(int64(seed), series, id, res); err != nil {
			log.Fatal(err)
		}
		runtime.GC()
	}
}

var writer *bench.DataWriter

func writeEnv() {
	env := map[string]any{
		"time_utc":        time.Now().UTC().Format(time.RFC3339),
		"go_version":      runtime.Version(),
		"goos":            runtime.GOOS,
		"goarch":          runtime.GOARCH,
		"num_cpu":         runtime.NumCPU(),
		"gomaxprocs":      runtime.GOMAXPROCS(0),
		"GOGC":            os.Getenv("GOGC"),
		"GOMEMLIMIT":      os.Getenv("GOMEMLIMIT"),
		"seed":            seed,
		"scale":           scale,
		"cpu_model":       firstMatch("/proc/cpuinfo", "model name"),
		"mem_total":       firstMatch("/proc/meminfo", "MemTotal"),
		"kernel":          readTrim("/proc/sys/kernel/osrelease"),
		"workload_series": envStr("SERIES", "size,value,skew,mixed,lambda"),
	}
	b, _ := json.MarshalIndent(env, "", "  ")
	_ = os.WriteFile(filepath.Join(outDir, "env.json"), b, 0o644)
}

func firstMatch(path, prefix string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, prefix) {
			if i := strings.Index(line, ":"); i >= 0 {
				return strings.TrimSpace(line[i+1:])
			}
		}
	}
	return ""
}

func readTrim(path string) string {
	b, _ := os.ReadFile(path)
	return strings.TrimSpace(string(b))
}

func sc(n int) int {
	v := int(float64(n) * scale)
	if v < 1000 {
		v = 1000
	}
	return v
}

func scb(b int64) int64 {
	v := int64(float64(b) * scale)
	if v < 1<<20 {
		v = 1 << 20
	}
	return v
}

func envStr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil {
		return v
	}
	return def
}

func envFloat(k string, def float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(k), 64); err == nil {
		return v
	}
	return def
}
