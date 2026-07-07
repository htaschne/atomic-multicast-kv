# Atomic Multicast KV

Atomic Multicast KV is a distributed key-value store implementing Skeen's atomic multicast protocol and an ACK-gated delivery variant. The repository includes deterministic correctness tests, in-memory benchmarks, artificial-latency benchmarks, and automated performance visualization.

## Features

- Original Skeen atomic multicast
- ACK-gated delivery variant
- Configurable N-partition routing
- Deterministic scheduled transport for correctness tests
- In-memory protocol benchmarks
- Artificial-latency benchmarks
- CSV export and plot generation
- Docker Compose deployment

## Repository layout

```text
.
├── bench-results/
│   ├── csv/
│   ├── plots/
│   └── raw/
├── docs/
├── references/
├── scripts/
├── *.go
├── docker-compose.yml
├── docker-compose.3.yml
└── README.md
```

## Running

```bash
go test ./...
```

```bash
go run .
```

Useful runtime flags:

```bash
go run . -id=0 -partitions=3 -mode=strengthened -peers='0=http://localhost:4000,1=http://localhost:4001,2=http://localhost:4002'
```

## Benchmarks

Run the in-memory protocol benchmark:

```bash
go test -bench='DestinationOverhead' -benchmem -run=^$ ./...
```

Run the artificial-latency benchmark:

```bash
go test -bench='ArtificialLatency|AckLatency' -benchmem -run=^$ -count=5 ./... > bench-results/raw/latency-count5.txt
```

Convert benchmark output to CSV:

```bash
python3 scripts/bench_to_csv.py bench-results/raw/latency-count5.txt > bench-results/csv/latency-count5.csv
```

Generate plots:

```bash
.venv/bin/python scripts/plot_benchmarks.py bench-results/csv/latency-count5.csv
```

Artificial latency is benchmark-only. It isolates protocol sensitivity to fixed per-message delay and ACK-specific delay without changing production transports or correctness tests.

## Results

![Latency vs delay](bench-results/plots/latency_vs_delay.png)

![Overhead by destination count](bench-results/plots/overhead_by_dst.png)

![ACK delay overhead](bench-results/plots/ack_delay_overhead.png)

## Reference

- [Strengthening Atomic Multicast for Partitioned State Machine Replication](references/non-blocking-skeen-atomic-multicast.pdf)
