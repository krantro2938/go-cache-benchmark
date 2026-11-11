#!/usr/bin/env python3
"""Aggregate results/run_*/results.csv into results/summary.csv.

For every (series, config_id, cache) and every metric the summary contains the
number of runs, the mean, the sample standard deviation and the half-width of
the 95 % confidence interval of the mean (Student's t). Standard library only.

Usage: python3 analysis/aggregate.py [results_dir]
"""
import csv
import glob
import math
import os
import statistics
import sys

METRICS = ["hit_ratio", "evictions", "ops_per_sec", "p50_us", "p95_us", "p99_us",
           "memory_mb", "live_mb", "alloc_rate_mb_per_sec", "allocs_per_op",
           "gc_cycles_per_sec", "gc_pause_us"]

# two-sided 95 % Student t quantiles for df = 1..30 (df > 30 -> 1.96)
T95 = [12.706, 4.303, 3.182, 2.776, 2.571, 2.447, 2.365, 2.306, 2.262, 2.228,
       2.201, 2.179, 2.160, 2.145, 2.131, 2.120, 2.110, 2.101, 2.093, 2.086,
       2.080, 2.074, 2.069, 2.064, 2.060, 2.056, 2.052, 2.048, 2.045, 2.042]


def main():
    root = sys.argv[1] if len(sys.argv) > 1 else "results"
    groups, order = {}, []
    for path in sorted(glob.glob(os.path.join(root, "run_*", "results.csv"))):
        with open(path, newline="") as f:
            for row in csv.DictReader(f):
                key = (row["series"], row["config_id"], row["cache"])
                if key not in groups:
                    groups[key] = []
                    order.append(key)
                groups[key].append(row)
    if not groups:
        sys.exit(f"no results found under {root}/run_*/results.csv")

    out = os.path.join(root, "summary.csv")
    with open(out, "w", newline="") as f:
        w = csv.writer(f)
        head = ["series", "config_id", "cache", "runs"]
        for m in METRICS:
            head += [f"{m}_mean", f"{m}_std", f"{m}_ci95"]
        w.writerow(head)
        for key in order:
            rows = groups[key]
            line = list(key) + [len(rows)]
            for m in METRICS:
                xs = [float(r[m]) for r in rows if r.get(m, "") != ""]
                mean = statistics.fmean(xs) if xs else float("nan")
                sd = statistics.stdev(xs) if len(xs) > 1 else 0.0
                t = T95[len(xs) - 2] if 1 < len(xs) <= 31 else 1.96
                ci = t * sd / math.sqrt(len(xs)) if len(xs) > 1 else 0.0
                line += [f"{mean:.6g}", f"{sd:.6g}", f"{ci:.6g}"]
            w.writerow(line)
    print(f"{len(order)} groups from {len(next(iter(groups.values())))} runs -> {out}")


if __name__ == "__main__":
    main()
