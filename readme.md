# go-cache-benchmark

Benchmark accompanying the paper «Адаптивное локальное кэширование в микросервисных
системах с учетом особенностей сборки мусора» (Adaptive TinyLFU + GC-Aware Cost).

## What is measured

Compared systems (all under the same byte budget):

| Name in results     | System |
|---------------------|--------|
| `GCAware`           | proposed method, λ = 0.7 (`internal/gcaware`) |
| `Adaptive_TinyLFU`  | the same implementation with λ = 1 (no cost term in V(x)) |
| `Ristretto`         | dgraph-io/ristretto v0.2.0, cost = object size |
| `BigCache`          | allegro/bigcache v3, hard limit = budget |

Go-Cache is not part of the comparison: it has no size limit and cannot be
compared under an equal memory budget.

Series (one row per configuration and cache in `results.csv`):

| Series   | Configurations | Workload |
|----------|----------------|----------|
| `size`   | budget 64 MB … 1 GB | 5M keys, uniform, 10M requests, 1 KB objects |
| `value`  | object 256 B … 64 KB | 500k keys, Zipf s = 0.95, 2M requests, 512 MB |
| `skew`   | s = 0.80 … 0.99 | 1M keys, 5M requests, 1 KB objects, 256 MB |
| `mixed`  | — | 200k keys, Zipf s = 0.9, 3M requests, 16 MB; sizes 256 B / 1 KB / 16 KB / 64 KB with weights 70/20/7/3 |
| `lambda` | λ = 0 … 1 (GCAware only) | same as `mixed` |

Protocol (see `internal/bench/runner.go`): look-aside access — `Get`, and on a
miss a fresh object of the key's size is allocated (simulated fetch from the
source) and `Set`. Latency covers the cache calls only. GC/allocation metrics
are `runtime.MemStats` deltas over the request loop; `gc_pause_us` is the mean
pause per GC cycle (ΔPauseTotalNs / ΔNumGC). `memory_mb` is the HeapInuse
delta after a forced GC with the cache alive, relative to a baseline taken
before the cache is constructed (`live_mb` is the same for HeapAlloc).
Zipf ranks are sampled exactly (inverse CDF) and mapped to keys by a random
permutation. Each run uses its own seed (the run number).

## Running

Requires Docker with Compose. A full run takes several minutes per seed and
needs about 6 GB of free RAM (the 1 GB budget configuration).

```bash
./run-benchmarks.sh                       # 30 runs -> results/run_XX, results/summary.csv
NUM_RUNS=3 ./run-benchmarks.sh            # fewer runs
BENCH_SCALE=0.05 NUM_RUNS=1 ./run-benchmarks.sh   # 1-minute smoke test
```

`results/host.txt` records the machine (CPU, RAM, OS, Docker); each
`results/run_XX/env.json` records the Go version, GOMAXPROCS and GOGC.

Unit tests: `cd bench-runner && go test ./...`
