# analysis

`aggregate.py` — merges `results/run_*/results.csv` into `results/summary.csv`
with mean, standard deviation and 95 % confidence interval per configuration,
cache and metric (standard library only):

    python3 analysis/aggregate.py results

`merge_results.py` — legacy script for the previous CSV layout.
