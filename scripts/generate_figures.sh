#!/bin/bash

set -euo pipefail

PYTHON="${PYTHON:-python3}"
if [ -x ".venv/bin/python" ]; then
    PYTHON=".venv/bin/python"
fi

RESULTS_DIR="${RESULTS_DIR:-benchmarks/results}"
PAPER_FIGURES_DIR="${PAPER_FIGURES_DIR:-docs/paper/figures}"

mkdir -p "$RESULTS_DIR/csv"

"$PYTHON" scripts/bench_to_csv.py \
    "$RESULTS_DIR/raw/inmemory-count10.txt" \
    > "$RESULTS_DIR/csv/inmemory-count10.csv"

"$PYTHON" scripts/bench_to_csv.py \
    "$RESULTS_DIR/raw/latency-count5.txt" \
    > "$RESULTS_DIR/csv/latency-count5.csv"

"$PYTHON" scripts/plot_benchmarks.py "$RESULTS_DIR/csv/latency-count5.csv"

mkdir -p "$PAPER_FIGURES_DIR"
cp "$RESULTS_DIR/csv/plots/"*.png "$PAPER_FIGURES_DIR"/
