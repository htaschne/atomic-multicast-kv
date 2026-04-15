# Atomic Multicast KV Store (Skeen Prototype)

This project is a **toy implementation of a partitioned key-value store** designed to experiment with **atomic multicast**, specifically the **Skeen algorithm** and its stronger variant described in:

> *Strengthening Atomic Multicast for Partitioned State Machine Replication* (LADC 2022)

The goal is to **understand and reproduce ordering guarantees** required for linearizable execution across partitions.

---

## Overview

The system is composed of:

- **Partitions (nodes)**
  Each partition runs as an independent HTTP service with:
  - its own KV storage
  - its own logical clock (planned)
  - its own ordering state (planned)

- **KV Store**
  - `put(k, v)` → writes to a single partition
  - `range(a, b)` → reads across multiple partitions

- **(Planned) Skeen Protocol**
  - timestamp agreement across partitions
  - hold-back queues
  - ordered delivery

---
## Setup & Run

1. Setup (only once)
```bash
go mod tidy
```

2. Run (we're focusing on two partitions at the moment)

```bash
go run . -id=0   # runs on :4000
go run . -id=1   # runs on :4001
```

---
## How to test

Since we don't have a client yet we're using cURL in localhost as such:

### Put
```bash
curl -X POST http://localhost:4000/put \
  -H "Content-Type: application/json" \
  -d '{"key": 0, "value": 42}'
```

### Get
```bash
curl "http://localhost:4000/range?a=0&b=10"
```

---
## Next Steps
1.	Introduce a Request struct (id, dst, etc.)
2.	Implement Skeen:
  - START
  - LOCAL_TS
  - FINAL_TS
3.	Add hold-back queue and delivery logic
4.	Compare:
  - baseline Skeen
  - strengthened variant

---

### References
- Leandro Pacheco, Fernando Dotti, Fernando Pedone.
  *Strengthening Atomic Multicast for Partitioned State Machine Replication*

- D. Skeen.
  *Nonblocking Commit Protocols*
