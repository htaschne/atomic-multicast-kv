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
docs/
references/
scripts/
bench-results/
Dockerfile
docker-compose*.yml
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
./generate_figures.sh
```

## Paper

This repository accompanies the paper "Implementing and Evaluating Skeen's Atomic Multicast Protocol and the ACK-Gated Extension for Atomic Global Order". The paper PDF is expected at `docs/paper.pdf`.

## References

The project builds on Skeen's original atomic multicast paper and Pacheco et al.'s ACK-gated extension. Reference material is collected in `references/`.

## Repository goals

SkeenKV prioritizes correctness, reproducibility, and experimental evaluation. It is intended as an academic artifact rather than a production deployment.
