# ADR-003: Agent Communication — gRPC + mTLS

## Status: Accepted

## Context

Agents run on Windows Server machines that are behind a firewall. The controller
needs to push task assignments and receive progress/result updates in near real-time.
Options considered: plain REST polling (high latency, chattiness), WebSocket
(browser-centric, no streaming semantics), gRPC (purpose-built for service-to-service,
streaming, typed contracts).

Security is critical: only enrolled agents should communicate with the controller.
Credentials must not rely on a shared secret that could be leaked via logs.

## Decision

Use gRPC with mutual TLS (mTLS) on port 9443. Protobuf definitions live in `proto/`.
The controller acts as the gRPC server; agents are clients. mTLS client certificates
are issued at enroll time via `POST /api/v1/agent/enroll` using a one-time enrollment
token. The `AgentService` RPCs cover `Register`, `Heartbeat`, `PollTask`,
`ReportProgress`, `ReportResult`, and `SyncOfflineResults`.

## Consequences

**Positive:** Strongly typed contracts, bidirectional streaming for log tailing,
built-in mutual auth, auto-generated Go client/server stubs, connection reuse.

**Negative:** gRPC requires HTTP/2; reverse proxies must be configured for h2
passthrough. Proto schema changes require careful backward-compatibility management.
