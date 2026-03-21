# ADR-002: Database — PostgreSQL

## Status: Accepted

## Context

The controller needs durable state for jobs, tasks, agents, users, webhooks,
templates, variables, audit logs, and scheduling. Options considered: SQLite
(single-file, no network access), MySQL (limited JSONB, older tooling), PostgreSQL
(mature, feature-rich, HA-capable).

The schema makes heavy use of JSONB for flexible metadata (`encode_config`,
`task variables`, `analysis results`), and requires reliable concurrent writers
(scheduler + gRPC task claims).

## Decision

Use PostgreSQL 16+ with `pgx/v5` connection pooling. Schema is versioned via
`golang-migrate` with plain `.sql` files embedded in the binary
(`internal/db/migrations/`). Optional Patroni + pgBouncer HA cluster for
production deployments (documented in `docs/ha-setup.md`).

## Consequences

**Positive:** JSONB storage avoids premature schema lock-in, `SELECT ... FOR UPDATE`
enables safe concurrent task claiming (`ClaimNextTask`), Patroni provides automatic
failover with no application changes.

**Negative:** PostgreSQL adds operational complexity compared to SQLite; requires a
separate container or managed service. Not suitable for truly embedded deployments.
