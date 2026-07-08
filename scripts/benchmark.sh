#!/bin/bash

mkdir -p benchmarks/results/raw

for i in {1..30}; do
  go test -bench=. -benchmem ./... >> benchmarks/results/raw/inmemory-30runs.txt
done
