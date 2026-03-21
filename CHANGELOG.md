# Changelog

All notable changes to this project are documented in this file.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

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
