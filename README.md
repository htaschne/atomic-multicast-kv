# SkeenKV

SkeenKV is an experimental distributed key-value store implementing the original Skeen atomic multicast protocol and the ACK-gated extension proposed by Pacheco et al. The project accompanies an academic paper and provides deterministic validation and reproducible benchmarks for evaluating the protocol variants.

## Features

- Original Skeen implementation
- ACK-gated delivery variant
- Configurable N-partition clusters
- Deterministic protocol validation
- In-memory benchmarks
- Artificial latency benchmarks
- Docker deployment
- Automated CSV and plot generation

## Repository layout

```text
benchmarks/results/
deployments/
docs/paper/
docs/postman/
references/
scripts/
README.md
*.go
```

## Quick start

### Run tests

```bash
go test ./...
```

### Run benchmarks

```bash
go test -bench=. -benchmem -run=^$ ./...
```

### Generate figures

```bash
scripts/generate_figures.sh
```

The script reads raw benchmark output from `benchmarks/results/raw/`, writes CSV and plot outputs under `benchmarks/results/csv/`, and refreshes the curated paper figures in `docs/paper/figures/`.

### Docker deployment

```bash
docker compose -f deployments/docker-compose.yml up --build
docker compose -f deployments/docker-compose.3.yml up --build
```

## Paper

This repository accompanies the paper "Implementing and Evaluating Skeen's Atomic Multicast Protocol and the ACK-Gated Extension for Atomic Global Order". Paper sources live under `docs/paper/`; generated PDFs and arXiv bundles are not tracked.

## References

The project builds on Skeen's original atomic multicast paper and Pacheco et al.'s ACK-gated extension. Reference material is collected in `references/`.

## Repository goals

SkeenKV prioritizes correctness, reproducibility, and experimental evaluation. It is intended as an academic artifact rather than a production deployment.
