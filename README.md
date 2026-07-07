# Atomic Multicast KV Store

This project is a small Go implementation of atomic multicast for a partitioned key-value store. It implements original Skeen atomic multicast and the strengthened ACK-gated variant from *Strengthening Atomic Multicast for Partitioned State Machine Replication* (LADC 2022).

The implementation is intentionally explicit: timestamps, proposals, final decisions, ACKs, pending request state, and delivery checks are all represented directly in code.

## Architecture

- `config.go` defines `ClusterConfig`: local partition id, total partition count, peer addresses, and protocol mode.
- `routing.go` defines configurable N-partition routing using `partition = key % partitionCount`.
- `kvstore.go` contains a per-partition in-memory `KVStore`.
- `skeen.go` contains request types, protocol message types, timestamps, request state, original Skeen delivery, and strengthened delivery.
- `transport.go` contains `InMemoryTransport` for tests/benchmarks and `HTTPTransport` for local clusters.
- `partition.go` exposes client endpoints and the internal protocol endpoint.

Each partition process owns a local store and a `Skeen` instance. Client requests may enter through any partition. `Submit` multicasts `START` to the destination partitions for that request.

## Cluster Configuration

Runtime config comes from flags or equivalent environment variables:

| Flag | Env var | Default | Description |
| --- | --- | --- | --- |
| `-id` | `PARTITION_ID` | `0` | Local partition id. |
| `-partitions` | `PARTITION_COUNT` | `2` | Total number of partitions. |
| `-mode` | `PROTOCOL_MODE` | `original` | `original` or `strengthened`. |
| `-peers` | `PEERS` | `0=http://localhost:4000,1=http://localhost:4001` | Comma-separated peer map. |

Peer map format:

```text
0=http://host0:4000,1=http://host1:4001,2=http://host2:4002
```

## Routing Model

Default routing is modulo based:

```text
partition = key % partitionCount
```

`PUT(k,v)` targets exactly one partition. `RANGE(start,end)` computes every partition touched by keys in `[start,end]`, preserving first-touch order and deduplicating partitions. Destination sets can be single partition, multi-partition, all partitions, or arbitrary subsets when constructing `Request` values directly in tests/benchmarks.

## Protocol

### Original Skeen

1. The submitting partition sends `START` to every destination in `req.Dst`.
2. Each destination assigns a local timestamp `(logical clock, partition id)`.
3. Each destination sends `LOCAL_TS` proposals to all destinations.
4. Once a destination has proposals from every destination, it decides the final timestamp as the maximum proposal.
5. The final timestamp is broadcast as `FINAL_TS`.
6. The logical clock advances to at least the final timestamp clock.
7. Delivery uses hold-back queue semantics: a request with final timestamp `ts` is delivered only when every pending local request is known to have a larger final timestamp or a larger local timestamp.

### Strengthened Variant

The strengthened mode adds the paper's ACK gate:

1. After learning a final timestamp and advancing its clock, a destination records/sends `ACK`.
2. A request is deliverable only after every destination has ACKed it.

This tests the paper's key assumption: original Skeen can allow delivery at one destination before another destination has advanced its clock past that request's final timestamp, while the ACK-gated variant delays delivery until that condition is known.

## HTTP API

Client endpoints:

```bash
POST /put
GET /range?start=<int>&end=<int>
```

Internal protocol endpoint:

```bash
POST /internal/protocol
```

The internal endpoint is used by peer partitions for `START`, `LOCAL_TS`, `FINAL_TS`, and `ACK`.

## Run Locally Without Docker

Two partitions:

```bash
go run . -id=0 -partitions=2 -mode=strengthened -peers='0=http://localhost:4000,1=http://localhost:4001'
go run . -id=1 -partitions=2 -mode=strengthened -peers='0=http://localhost:4000,1=http://localhost:4001'
```

Three partitions:

```bash
go run . -id=0 -partitions=3 -mode=strengthened -peers='0=http://localhost:4000,1=http://localhost:4001,2=http://localhost:4002'
go run . -id=1 -partitions=3 -mode=strengthened -peers='0=http://localhost:4000,1=http://localhost:4001,2=http://localhost:4002'
go run . -id=2 -partitions=3 -mode=strengthened -peers='0=http://localhost:4000,1=http://localhost:4001,2=http://localhost:4002'
```

Use `-mode=original` to run original Skeen.

Example requests:

```bash
curl -X POST http://localhost:4000/put \
  -H "Content-Type: application/json" \
  -d '{"key": 0, "value": 42}'

curl -X POST http://localhost:4000/put \
  -H "Content-Type: application/json" \
  -d '{"key": 1, "value": 99}'

curl "http://localhost:4000/range?start=0&end=2"
```

The range response merges results from every destination partition involved in the range.

## Docker

Build:

```bash
docker compose build
```

Run the default 2-partition strengthened cluster:

```bash
docker compose up
```

Run the 2-partition original Skeen cluster:

```bash
PROTOCOL_MODE=original docker compose up
```

Run the 3-partition strengthened cluster:

```bash
docker compose -f docker-compose.3.yml up
```

Run the 3-partition original Skeen cluster:

```bash
PROTOCOL_MODE=original docker compose -f docker-compose.3.yml up
```

Ports:

- partition `0`: `localhost:4000`
- partition `1`: `localhost:4001`
- partition `2`: `localhost:4002` when using `docker-compose.3.yml`

For N partitions manually, start partition ids `0..N-1`, set `PARTITION_COUNT=N`, and pass a `PEERS` entry for every partition.

## Tests

```bash
go test ./...
```

The test suite covers:

- request validation;
- N-partition routing, including negative keys and all-partition ranges;
- cluster config validation;
- local store behavior;
- rejection of internal `START` at non-destination partitions;
- single-partition and cross-partition execution through in-memory transport;
- 5-partition all-range execution;
- non-overlapping destination sets delivering independently;
- overlapping destination sets preserving order at their intersection;
- original Skeen's atomic-global-order counterexample from the paper;
- strengthened Skeen delaying delivery until destination ACKs;
- the same unsafe/delayed scenario with 4 partitions.

## Benchmarks

Run all benchmarks:

```bash
go test -bench=. -benchmem ./...
```

Or:

```bash
sh scripts/bench.sh
```

Benchmarks compare original and strengthened modes across:

- N = 2, 3, and 5 partitions;
- 1 destination;
- 2 destinations;
- 3 destinations where N permits it;
- all N destinations.

Go's benchmark output reports latency as `ns/op` and allocation overhead as `B/op` and `allocs/op`. Use `-benchtime` to lengthen runs:

```bash
go test -bench=. -benchmem -benchtime=5s ./...
```

## Limitations

- The implementation models one process per partition and does not replicate within a partition group.
- Original Skeen is non-fault-tolerant, matching the paper's discussion.
- There is no retry, membership change, persistence, or crash recovery.
- The HTTP transport is intended for local experiments, tests, and benchmarks, not production deployment.
