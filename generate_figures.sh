#!/bin/bash

set -euo pipefail

PYTHON="${PYTHON:-python3}"
if [ -x ".venv/bin/python" ]; then
    PYTHON=".venv/bin/python"
fi

mkdir -p bench-results/csv

"$PYTHON" scripts/bench_to_csv.py \
    bench-results/raw/inmemory-count10.txt \
    > bench-results/csv/inmemory-count10.csv

"$PYTHON" scripts/bench_to_csv.py \
    bench-results/raw/latency-count5.txt \
    > bench-results/csv/latency-count5.csv

"$PYTHON" scripts/plot_benchmarks.py bench-results/csv/latency-count5.csv
