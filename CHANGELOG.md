# Changelog

All notable changes to this project are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

### Added

#### Controller
- Per-user and per-IP cap on WebSocket log-stream connections. New config section `logstream:` with `max_per_user` (default 10) and `max_per_ip` (default 20). Prometheus metrics `encodeswarmr_logstream_rejected_total{reason}` and `encodeswarmr_logstream_active_connections`.
- Flow-engine hardening: cycle detection, panic recovery, context cancellation checks, and error-edge routing in the DAG walker. `ValidateFlow` is now called from `POST /api/v1/flows` and `PUT /api/v1/flows/{id}` — invalid graphs rejected with HTTP 400 at save time.
- Cloud-storage adapters (S3, Azure, GCS) now use SDK-native retry with configurable exponential backoff. New config section `cloud_storage.retry:` with `max_elapsed`, `initial_interval`, `max_interval`, `multiplier`. All three adapters log per-attempt warnings with structured fields (`attempt`, `operation`, `bucket`, `key`, `next_delay`, `error`).
- `controller agent drain <host>` CLI — graceful drain (finish current task, stop polling).
- `controller task reset <task-id>` CLI — reset a stuck task to `pending` with safety checks (refuses running-but-live, refuses terminal states even with `--force`). Writes audit-log entry.

#### Desktop manager
- Test coverage raised from effectively zero to ~41% for the `client` package and ~95% for `client/ws.go` (via injectable `wsDialer`).

#### Documentation
- New `DEPLOYMENT.md` at the repo root — canonical deployment entry point.
- New wiki pages: `Runbook` (incident triage), `Testing-Guide`, `gRPC-Reference` (auto-gen from proto), `Operational-Semantics`, `Plugin-Loading`.
- CLAUDE.md codifies that tests are mandatory for every change.

### Changed

- `controller agent disable <host>` CLI now correctly sets agent status to `disabled` (abort current task, do not pick up new work). Prior behavior (drain-and-stop) is available as `controller agent drain`. A deprecation warning is printed on stderr for one release to ease the transition.
- `tests/integration/stress_test.go` moved behind `//go:build integration && stress` — excluded from default CI integration runs. New `make stress` target for opt-in local runs.
- TypeScript API client (`client.gen.ts`): corrected generated error-cast type and documented non-OK response shape; the CI workflow now post-patches the regenerated output.

### Removed

- Dead generated API clients: `api/generated/ts/`, `api/generated/go/`, `api/openapi-ts.config.ts`, `api/oapi-codegen.yaml`, and the `.github/workflows/generate-sdk.yml` workflow. The OpenAPI spec at `api/openapi.yaml` remains; integrators can generate their own clients. Wiki page `API-Client-SDKs` removed.
- Dead config keys `agent.stale_threshold` and `engine.stale_threshold` — neither was read at runtime. `agent.heartbeat_timeout` is the single source of truth for stale detection.
- `github.com/oapi-codegen/runtime` Go dependency (no longer needed after the generated Go client was removed).

### Fixed

- `HeartbeatResp.drain` and `HeartbeatResp.disabled` are now reachable from separate CLI subcommands (previously `agent disable` emitted `drain`, and there was no way to trigger `disabled`).
- WebSocket log-stream hub no longer has an unbounded connection ceiling — a DoS vector is closed.

---

## [1.0.0] — 2026-03-05

### Added

#### Controller
- Job orchestration with priority FIFO queue and PostgreSQL state tracking
- Script generation: AviSynth (`.avs`), VapourSynth (`.vpy`), batch (`.bat`/`.sh`) from Go `text/template`
- Auto-analysis: registering a source automatically queues `analysis` and `hdr_detect` jobs
- HDR metadata detection: HDR10, HDR10+, Dolby Vision (with profile), HLG
- Live log streaming over gRPC; viewable in the web UI via Server-Sent Events
- Progress reporting: parses x265, x264, SVT-AV1, and FFmpeg progress lines
- Webhooks: Discord, Teams, and Slack notifications on job events with HMAC-SHA256 signing and retry delivery
- OIDC SSO: optional OpenID Connect login alongside local username/password
- Agent self-upgrade: agents poll for a newer binary, verify SHA-256, and self-restart
- OpenAPI 3.1 spec at `/api/v1/openapi.json`; Swagger UI at `/api/docs`
- WebSocket live-event hub at `/api/v1/ws` for real-time web UI updates
- Per-IP rate limiting (200 req/s, burst 400) and ETag caching on all GET API responses
- CORS and `X-Request-ID` correlation headers on all responses
- Setup wizard: first-run flow creates the initial admin account before any agent connects
- Agent enrollment tokens for zero-touch registration
- Web-based VNC/noVNC remote desktop proxy for agent hosts
- Global template variables stored in the database, injected into generated scripts
- Role-based access control: viewer / operator / admin
- `GET /health` and `GET /metrics` (Prometheus) endpoints

#### Agent
- Runs as a Windows Service (no NSSM) or Linux systemd unit
- mTLS client authentication over gRPC
- Offline resilience: task results and log lines buffered to SQLite and replayed on reconnect
- GPU detection and monitoring: NVIDIA NVENC, Intel QSV, AMD AMF
- Pre-task validation: UNC/NFS path allow-list, required parameter checks, script presence
- VNC port reporting; integrates with controller's noVNC proxy

#### Packaging & deployment
- Docker Compose deployment with PostgreSQL and optional pgBouncer
- Optional Patroni + pgBouncer for HA PostgreSQL failover
- Debian/Ubuntu `.deb` packages for controller and agent (`make deb`, `make deb-agent`)
- RHEL/Rocky Linux/AlmaLinux `.rpm` packages for controller and agent (`make rpm`, `make rpm-agent`)
- Inno Setup GUI installer for Windows agents (`make installer`)
- Bootstrap scripts: `scripts/install-controller.sh` (Ubuntu), `scripts/install-controller-rpm.sh` (RHEL/Rocky), `scripts/install-agent-linux.sh` (Linux), `scripts/install-agent.ps1` (Windows)
- GitHub Actions CI (build, test, race detector) on every push and PR
- GitHub Actions release pipeline: `.deb`, `.rpm`, Windows `.exe`, Windows installer, container image on every semver tag

[Unreleased]: https://github.com/badskater/encodeswarmr/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/badskater/encodeswarmr/releases/tag/v1.0.0
