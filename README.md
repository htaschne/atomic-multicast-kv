# Atomic Multicast KV Store

This project is a small Go implementation of atomic multicast for a partitioned key-value store. It implements:

- original Skeen atomic multicast;
- the strengthened ACK-gated variant from *Strengthening Atomic Multicast for Partitioned State Machine Replication* (LADC 2022);
- in-memory deterministic tests and HTTP peer transport for local Docker clusters.

The implementation is intentionally small and explicit so the protocol state is easy to inspect.

## Architecture

- `routing.go` maps even keys to partition `0` and odd keys to partition `1`.
- `kvstore.go` contains a per-partition in-memory `KVStore`.
- `skeen.go` contains request types, protocol message types, timestamps, request state, and Skeen delivery rules.
- `transport.go` contains `InMemoryTransport` for tests and `HTTPTransport` for local clusters.
- `partition.go` exposes client endpoints and the internal protocol endpoint.

Each partition process owns a local store and a `Skeen` instance. Client requests enter through any partition, then `Submit` multicasts `START` to the destination partitions for that request.

## Protocol

### Original Skeen

For each request:

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

This prevents a partition from delivering a request before another destination has advanced its clock past that request's final timestamp.

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

Terminal 1:

```bash
go run . -id=0 -mode=strengthened -peers='0=http://localhost:4000,1=http://localhost:4001'
```

Terminal 2:

```bash
go run . -id=1 -mode=strengthened -peers='0=http://localhost:4000,1=http://localhost:4001'
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

curl "http://localhost:4000/range?start=0&end=1"
```

The cross-partition range returns merged results from both involved partitions.

## Docker

Build:

```bash
docker compose build
```

Run the default strengthened cluster:

```bash
docker compose up
```

Run original Skeen:

```bash
PROTOCOL_MODE=original docker compose up
```

Partition ports:

- partition `0`: `localhost:4000`
- partition `1`: `localhost:4001`

## Tests

```bash
go test ./...
```

The test suite covers:

- request validation;
- routing;
- local store behavior;
- rejection of internal `START` at non-destination partitions;
- single-partition and cross-partition execution through in-memory transport;
- original Skeen's atomic-global-order counterexample from the paper;
- strengthened Skeen delaying delivery until destination ACKs.

## Benchmarks

Run all benchmarks:

```bash
go test -bench=. -benchmem ./...
```

Or:

```bash
sh scripts/bench.sh
```

Benchmarks cover:

- single-partition `PUT`;
- single-partition `RANGE`;
- cross-partition `RANGE`;
- original Skeen vs strengthened mode.

Go's benchmark output reports latency as `ns/op`, memory allocations, and throughput-relevant operation counts. Use `-benchtime` to lengthen runs, for example:

```bash
go test -bench=. -benchmem -benchtime=5s ./...
```

## Limitations

- The implementation models one process per partition and does not replicate within a partition group.
- Original Skeen is non-fault-tolerant, matching the paper's discussion.
- There is no retry, membership change, persistence, or crash recovery.
- The HTTP transport is intended for local experiments, tests, and benchmarks, not production deployment.
