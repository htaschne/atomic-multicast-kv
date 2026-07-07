#!/bin/bash

mkdir -p bench-results

for i in {1..30}; do
  go test -bench=. -benchmem ./... >> bench-results/inmemory-30runs.txt
done