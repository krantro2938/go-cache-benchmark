#!/bin/bash
# Runs the full benchmark NUM_RUNS times (default 30). Each run uses its own
# workload seed (the run number) and a fresh container, so runtime state is not
# carried over between runs. Results: ./results/run_XX/{results.csv,env.json}.
# Host description: ./results/host.txt. Summary: ./results/summary.csv.
#
#   ./run-benchmarks.sh               # 30 full runs
#   NUM_RUNS=3 ./run-benchmarks.sh    # fewer runs
#   BENCH_SCALE=0.05 NUM_RUNS=1 ./run-benchmarks.sh   # quick smoke test
set -euo pipefail

NUM_RUNS=${NUM_RUNS:-30}
mkdir -p results

{
  echo "date_utc: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "--- uname"; uname -a
  echo "--- os-release"; cat /etc/os-release 2>/dev/null || sw_vers 2>/dev/null || true
  echo "--- cpu"; lscpu 2>/dev/null || sysctl -n machdep.cpu.brand_string 2>/dev/null || true
  echo "--- memory"; free -h 2>/dev/null || sysctl hw.memsize 2>/dev/null || true
  echo "--- docker"; docker version --format 'client {{.Client.Version}} / server {{.Server.Version}}' || true
  echo "--- git"; git rev-parse HEAD 2>/dev/null || true
} > results/host.txt

docker compose build bench-runner

for i in $(seq 1 "$NUM_RUNS"); do
    run=$(printf '%02d' "$i")
    echo "=== Run $run/$NUM_RUNS ==="
    mkdir -p "results/run_$run"
    RUN_SEED=$i docker compose run --rm -e RUN_SEED="$i" \
        -v "$(pwd)/results/run_$run:/app/results" bench-runner
done

python3 analysis/aggregate.py results
echo "Done. Summary: results/summary.csv"
