# Implementation Status

## Project overview

The project implements a configurable N-partition atomic multicast key-value store in Go.

- `config.go` defines `ClusterConfig`: local partition id, partition count, peer addresses, and protocol mode.
- `routing.go` routes keys with `key % partitionCount` and computes destination sets for ranges.
- `skeen.go` contains the protocol model: requests, timestamps, protocol messages, per-request state, original Skeen delivery, and strengthened ACK-gated delivery.
- `transport.go` contains `InMemoryTransport` for tests/benchmarks and `HTTPTransport` for local partition processes.
- `partition.go` exposes `/put`, `/range`, and `/internal/protocol`.
- `deployments/docker-compose.yml` runs the default 2-partition cluster.
- `deployments/docker-compose.3.yml` runs a 3-partition cluster.

The current implementation models one process per partition. It does not implement replicated groups within a partition, crash recovery, retries, or membership changes.

## Implemented protocol pieces

### Original Skeen

Implemented in `Skeen.Submit`, `Skeen.ReceiveProtocol`, and the protocol helpers in `skeen.go`.

- `START`: `Submit` sends a `START` message to every destination in `Request.Dst`.
- Local timestamp: each destination increments its logical clock and assigns `(clock, partition id)`.
- `LOCAL_TS`: each destination sends its proposal to all destinations.
- Proposal collection: each request state tracks proposals by destination partition.
- Final timestamp: once all proposals are known, the final timestamp is the maximum proposal.
- `FINAL_TS`: final decisions are broadcast to all destinations.
- Clock advancement: learning a final timestamp advances the local logical clock to at least the final clock.
- Hold-back queue semantics: `tryDeliverLocked` delivers finalized requests in final timestamp order only when every pending local request is known to have a larger final timestamp or larger local timestamp.
- Integrity: request state has a `delivered` flag and a completion channel, so each partition executes a request at most once.

All proposal, final timestamp, ACK, and delivery checks are driven by `req.Dst`; no protocol rule assumes exactly two partitions.

### Strengthened ACK-gated variant

Mode is selected with `ProtocolMode`:

- `ModeOriginal`
- `ModeStrengthened`

In strengthened mode:

- after learning a final timestamp, a destination records/sends `ACK`;
- request state tracks ACKs by destination;
- `canDeliverLocked` requires ACKs from all destinations before delivery;
- deterministic tests show the original paper scenario is delayed until the common destination has advanced its clock and ACKed.

### Multi-partition execution

- `PUT` is multicast only to the owning partition.
- `RANGE` is multicast to every partition containing keys in the requested interval.
- `Submit` waits for every destination's `START` response.
- `RANGE` results from destination partitions are merged before returning to the client.
- Tests construct arbitrary destination subsets directly to exercise protocol behavior beyond router-generated ranges.

## Missing / incomplete protocol pieces

- No fault tolerance: original Skeen is non-fault-tolerant, and the strengthened variant here keeps that assumption.
- No replicated process group per partition; each partition is one process.
- No persistent log or recovery.
- No transport retry/backoff.
- No dynamic peer membership.
- No production-grade observability beyond basic logs.

## Risks or mismatches with the paper

- The paper discusses atomic multicast over process groups. This implementation maps each partition to one process, so it demonstrates ordering semantics but not replicated partition internals.
- The HTTP transport is meant for local experiments. In-flight request state is memory-only.
- The implementation includes an explicit `FINAL_TS` broadcast, although the paper's pseudocode can let each destination compute the final timestamp from all proposals. This keeps final-decision handling explicit and testable without changing the timestamp rule.
- ACK self-recording is local: a partition records its own ACK when sending ACKs to other destinations. This avoids unnecessary self-HTTP calls while preserving the ACK-set delivery condition.

## Tests currently covering this

Current tests include:

- `config_test.go`: cluster config and peer parsing.
- `request_test.go`: request validation and timestamp ordering.
- `routing_test.go`: default 2-partition routing and configurable 5-partition routing.
- `kvstore_test.go`: local store `Put`/`Range`.
- `skeen_test.go`:
  - internal `START` rejection at non-destination partitions;
  - single-partition `PUT` and cross-partition `RANGE`;
  - 5-partition all-range execution;
  - non-overlapping destination sets delivering independently;
  - overlapping destination sets preserving order at the common destination;
  - original Skeen reproducing the paper's atomic-global-order counterexample shape;
  - strengthened Skeen delaying delivery until destination ACKs;
  - the same unsafe/delayed scenario with 4 partitions.

Benchmarks in `skeen_benchmark_test.go` compare original and strengthened modes across N = 2, 3, and 5, with destination counts of 1, 2, 3 where valid, and all N.

## Recommended next steps, ordered by priority

1. Add HTTP integration tests using `httptest.Server` for 3+ real HTTP transports.
2. Add timeout/error-path tests for unavailable peers and partial peer maps.
3. Add structured trace hooks for protocol message flow.
4. Add a Docker-backed load runner if benchmark comparisons should include HTTP transport overhead.
5. Extend the model to replicated groups per partition if the next goal is fault tolerance rather than protocol-ordering experimentation.
