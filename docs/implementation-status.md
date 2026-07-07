# Implementation Status

## Project overview

The project now implements a testable two-partition atomic multicast key-value store in Go.

- `skeen.go` contains the core protocol model: requests, timestamps, protocol messages, per-request state, original Skeen delivery, and strengthened ACK-gated delivery.
- `transport.go` contains `InMemoryTransport` for tests/benchmarks and `HTTPTransport` for local partition processes.
- `partition.go` exposes the public `/put` and `/range` endpoints plus the internal `/internal/protocol` endpoint.
- `routing.go` maps even keys to partition `0` and odd keys to partition `1`.
- `kvstore.go` provides a per-partition in-memory store.
- `docker-compose.yml` runs partition `0` on port `4000` and partition `1` on port `4001`.

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

- `request_test.go`: request validation and timestamp ordering.
- `routing_test.go`: key/range destination routing.
- `kvstore_test.go`: local store `Put`/`Range`.
- `skeen_test.go`:
  - internal `START` rejection at non-destination partitions;
  - single-partition `PUT` and cross-partition `RANGE` through in-memory transport;
  - original Skeen reproducing the paper's atomic-global-order counterexample shape;
  - strengthened Skeen delaying delivery until destination ACKs.

Benchmarks in `skeen_benchmark_test.go` cover:

- original vs strengthened single-partition `PUT`;
- original vs strengthened single-partition `RANGE`;
- original vs strengthened cross-partition `RANGE`.

## Recommended next steps, ordered by priority

1. Add HTTP integration tests using `httptest.Server` for two real HTTP transports.
2. Add timeout/error-path tests for unavailable peers.
3. Add structured logs or trace hooks for protocol message flow.
4. Add a small load runner against Docker Compose if benchmark comparisons should include HTTP transport overhead.
5. Extend the model to replicated groups per partition if the next goal is fault tolerance rather than protocol-ordering experimentation.
