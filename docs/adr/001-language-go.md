# ADR-001: Language — Go

## Status: Accepted

## Context

The project requires two separate runtime environments: a Linux/container controller
and Windows Server encoding agents. Both must be maintainable from a single codebase.
The team ruled out Python (no static binaries, slow startup, GIL) and Java (JVM
overhead, complex deployment). Rust was considered but deemed too steep a learning
curve for the team.

## Decision

Use Go 1.25+ for all components (controller and agent). Cross-compile to produce
`agent-windows-amd64.exe` and `controller-linux-amd64` from the same source tree
using `GOOS`/`GOARCH`.

Key packages: `net/http` (REST), `google.golang.org/grpc` (agent comms), `pgx/v5`
(PostgreSQL), `log/slog` (structured logging), `embed` (static assets baked into
binary).

## Consequences

**Positive:** Single ~30 MB static binary per target, no runtime dependencies on
agents, fast compile times in CI, strong standard library, trivial cross-compilation.

**Negative:** Go generics are less expressive than Rust traits; some boilerplate
for error handling. Team must avoid CGO to keep cross-compilation simple (enforced
by using `modernc.org/sqlite` for the agent journal).
