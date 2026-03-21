# Architecture Decision Records

This directory contains ADRs for significant design choices in the distributed-encoder project.
Each record captures the context, the decision made, and its consequences.

## Index

| ADR | Title | Status |
|-----|-------|--------|
| [001](001-language-go.md) | Language: Go | Accepted |
| [002](002-database-postgresql.md) | Database: PostgreSQL | Accepted |
| [003](003-agent-communication-grpc.md) | Agent Communication: gRPC + mTLS | Accepted |
| [004](004-windows-agent-linux-controller.md) | Split Platform: Windows Agents, Linux Controller | Accepted |
| [005](005-script-generation.md) | Server-Side Script Generation | Accepted |
| [006](006-offline-journal-sqlite.md) | Agent Offline Resilience: SQLite Journal | Accepted |
| [007](007-chunk-encoding-architecture.md) | Chunk Encoding Architecture | Accepted |
| [008](008-controller-side-concat.md) | Controller-Side Concat Task | Accepted |

## Format

Each ADR follows this template:

```
# ADR-NNN: Title
## Status: Accepted | Superseded | Deprecated
## Context
## Decision
## Consequences
```
