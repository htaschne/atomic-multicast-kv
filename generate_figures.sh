#!/bin/bash

python3 scripts/bench_to_csv.py \
    bench-results/inmemory-count10.txt \
    > bench-results/inmemory-count10.csv

python3 scripts/bench_to_csv.py \
    bench-results/latency-count5.txt \
    > bench-results/latency-count5.csv

python3 scripts/plot_benchmarks.py bench-results/latency-count5.csv
