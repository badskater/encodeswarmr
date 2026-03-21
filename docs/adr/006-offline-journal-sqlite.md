# ADR-006: Agent Offline Resilience — SQLite Journal

## Status: Accepted

## Context

Encoding jobs can run for hours. If the network link between an agent and the
controller drops mid-encode, the agent must continue working and not lose the result.
On reconnect, completed task data (frames encoded, avg_fps, output size, exit code)
must be replayed to the controller without duplication.

Options: hold results in memory until reconnect (lost on agent crash), write to a
local file (fragile format), use a local relational store (structured, transactional).

## Decision

Agents maintain a local SQLite journal at a configurable path using the pure-Go
`modernc.org/sqlite` driver (no CGO). The schema is in
`internal/db/sqlite/journal.go`. On reconnect the agent calls `SyncOfflineResults`
gRPC to replay buffered results. The controller deduplicates by task ID before
applying updates.

## Consequences

**Positive:** Resilient to network outages of arbitrary duration, no data loss on
agent restart (journal survives process crash), pure-Go driver keeps cross-compilation
simple.

**Negative:** SQLite introduces a second database technology into the project. The
journal schema is independent of the PostgreSQL schema and must be kept in sync
with `CompleteTask` fields. Agent must handle SQLite write errors gracefully.
