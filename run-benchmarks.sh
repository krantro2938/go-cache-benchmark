#!/bin/bash

# Run benchmarks 30 times, storing results in separate folders

NUM_RUNS=30

for i in $(seq -w 1 $NUM_RUNS); do
    echo "=== Run $i/$NUM_RUNS ==="
    
    # Create results directory for this run
    RUN_DIR="./results/run_$i"
    mkdir -p "$RUN_DIR"
    
    # Run docker-compose with the run-specific volume mount
    docker compose run --rm -v "$(pwd)/results/run_$i:/app/results" bench-runner
    
    echo "Results saved to $RUN_DIR"
    echo ""
done

echo "All $NUM_RUNS runs completed!"
echo "Results are in ./results/run_01 through ./results/run_$(printf '%02d' $NUM_RUNS)"
