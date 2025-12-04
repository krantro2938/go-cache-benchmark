package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"cache-bench/internal/bench"
	"cache-bench/internal/caches"
)

const CacheSizeBytes = 512 * 1024 * 1024 // 512 MB

func main() {
	workload := bench.GenerateZipfWorkload(42)

	w, err := bench.NewDataWriter()
	if err != nil {
		log.Fatal(err)
	}
	defer w.Flush()

	cachesList := []caches.Cache{}

	// 1. Redis
	if redisAddr := os.Getenv("REDIS_ADDR"); redisAddr != "" {
		redisCache, err := caches.NewRedisCache(redisAddr)
		if err != nil {
			log.Printf("Redis skipped: %v", err)
		} else {
			cachesList = append(cachesList, redisCache)
		}
	}

	// // 2. Badger (in-memory)
	// badgerCache, err := caches.NewBadgerCache()
	// if err != nil {
	// 	log.Printf("Badger skipped: %v", err)
	// } else {
	// 	cachesList = append(cachesList, badgerCache)
	// 	defer badgerCache.Close()
	// }

	// 3. Ristretto
	ristrettoCache, err := caches.NewRistrettoCache(CacheSizeBytes)
	if err != nil {
		log.Printf("Ristretto skipped: %v", err)
	} else {
		cachesList = append(cachesList, ristrettoCache)
		defer ristrettoCache.Close()
	}

	// 4. Otter (your "custom" cache)
	otterCache, err := caches.NewOtterCache(CacheSizeBytes)
	if err != nil {
		log.Printf("Otter skipped: %v", err)
	} else {
		cachesList = append(cachesList, otterCache)
		defer otterCache.Close()
	}

	// BigCache (~512MB)
	bigCache, err := caches.NewBigCache(512 * 1024 * 1024)
	if err != nil {
		log.Printf("BigCache skipped: %v", err)
	} else {
		cachesList = append(cachesList, bigCache)
		defer bigCache.Close()
	}

	// GoCache (~512MB worth of 1KB items)
	goCache := caches.NewGoCache(524288)
	cachesList = append(cachesList, goCache)

	for _, cache := range cachesList {
		fmt.Printf("Running benchmark for %s...\n", cache.Name())
		start := time.Now()
		result := bench.RunBenchmark(cache, workload)
		duration := time.Since(start)

		hitRatio := float64(result.Hits) / float64(result.Hits+result.Misses)
		tps := float64(result.TotalOps) / duration.Seconds()
		p50 := bench.Percentile(result.Latencies, 0.50).Microseconds()
		p95 := bench.Percentile(result.Latencies, 0.95).Microseconds()
		p99 := bench.Percentile(result.Latencies, 0.99).Microseconds()

		w.WriteLatency(cache.Name(), float64(p50), float64(p95), float64(p99))

		w.WriteHitRatio(cache.Name(), hitRatio)
		w.WriteThroughput(cache.Name(), tps)
		w.WriteEvictions(cache.Name(), result.Evictions)
		w.WriteMemory(cache.Name(), result.MemoryMB)

		fmt.Printf("→ Hit ratio: %.4f, TPS: %.0f, p99: %d ms\n", hitRatio, tps, p99)
	}

	fmt.Println("Benchmark complete. Results saved to ./results/")
}
