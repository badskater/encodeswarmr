# Distributed Encoder

A distributed video encoding system that offloads encoding workloads from a central **Controller** to one or more **Windows Server Agents**. The Controller orchestrates jobs, tracks state in PostgreSQL, exposes a REST and gRPC API, serves a management web UI, and fires webhooks on completion. Each Agent pulls work, executes batch and script files from UNC shares, and reports results — including GPU-accelerated encodes — surviving network outages via a local SQLite journal.

---

## Overview

```
  Browser / REST API / Webhooks
              |
              v
  CONTROLLER  (Linux / Docker)
  +-- HTTP :8080  (REST API + Web UI)
  +-- gRPC :9443  (agent comms, mTLS)
  +-- PostgreSQL  (jobs, agents, results)
              | gRPC/mTLS
              v
  AGENT x N   (Windows Server)
  +-- Windows Service (no NSSM needed)
  +-- GPU encoding  (NVENC / QSV / AMF)
  +-- Offline resilience  (SQLite journal)
  +-- UNC file access  (NAS / SAN)
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for the full system design and [AGENTS.md](AGENTS.md) for agent internals.

---

## Features

- **Job orchestration** — priority FIFO queue, script generation (AviSynth `.avs`, VapourSynth `.vpy`, `.bat`), PostgreSQL state tracking
- **Auto-analysis** — registering a source automatically queues `analysis` (VMAF, histogram, scene detection) and `hdr_detect` jobs; no manual operator action required
- **HDR/DV metadata** — `hdr_detect` jobs detect HDR10, HDR10+, Dolby Vision (with profile), and HLG; results stored on the source record and exposed in the web UI
- **GPU encoding** — NVIDIA NVENC, Intel QSV, AMD AMF auto-detected and monitored per agent
- **Offline resilience** — agents buffer task results and log lines to SQLite and replay on reconnect
- **Live log streaming** — stdout/stderr streamed to the Controller over gRPC; viewable in the web UI in real time
- **Progress reporting** — parses x265, x264, SVT-AV1, and FFmpeg progress lines; reported back via gRPC
- **Webhooks** — Discord, Teams, and Slack notifications on job success or failure
- **OIDC SSO** — optional OpenID Connect login alongside local username/password
- **Auto-upgrade** — agents poll the Controller for a newer binary, verify SHA-256, and self-restart
- **OpenAPI 3.1** — machine-readable spec served at `/api/v1/openapi.json`; Swagger UI at `/api/docs`
- **HA database** — optional Patroni + pgBouncer for zero-downtime PostgreSQL failover

---

## Repository Layout

```
cmd/
  controller/        Controller entry point
  agent/             Agent entry point
configs/
  agent.yaml.example
  controller.yaml.example
deployments/
  Dockerfile.controller
  docker-compose.yml
  patroni.yml
internal/
  agent/             Agent service, runner, GPU, progress, offline store, upgrade
  controller/        API handlers, scheduler, script generator, webhooks
  db/                PostgreSQL migrations and sqlc query layer
  proto/             gRPC service definitions
  shared/            Shared types and utilities
web/                 React + Vite static build (embedded in controller binary)
proto/               Protobuf source files
scripts/
  gen-certs.sh       mTLS certificate generation helper
Makefile
```

---

## Quick Start

### Prerequisites

| Tool | Version |
|---|---|
| Go | 1.25+ |
| Node / npm | For web UI build |
| Docker + Compose v2 | Controller deployment |
| `protoc` + Go plugins | Only if regenerating proto |
| `golangci-lint` | Optional, for linting |
| `golang-migrate` CLI | Database migrations |

### Build

```bash
# Build everything (web UI + controller Linux binary + agent Windows binary)
make all

# Individual targets
make web          # npm ci && npm run build in web/
make controller   # CGO_ENABLED=0 GOOS=linux  -> bin/controller
make agent        # CGO_ENABLED=0 GOOS=windows -> bin/agent.exe

# Regenerate gRPC/protobuf code
make proto

# Run tests
make test

# Lint
make lint
```

### Controller — Docker Compose

```bash
cd deployments

# Copy and edit the environment file — never commit secrets
cp ../.env.example .env
$EDITOR .env

# Generate mTLS certificates (see DEPLOYMENT.md for details)
# Place server.crt, server.key, ca.crt in deployments/certs/

# Start Controller + PostgreSQL + pgBouncer
docker compose up -d

# Apply database migrations
DATABASE_URL="postgres://distenc:<pass>@localhost:5432/distencoder?sslmode=disable" \
  make migrate-up
```

The web UI is available at `http://localhost:8080`.

### Controller — Native install (Debian / Ubuntu 22.04 / 24.04)

Download `distributed-encoder-controller_*_linux_amd64.deb` from the [latest GitHub Release](https://github.com/badskater/distributed-encoder/releases/latest):

```bash
sudo dpkg -i distributed-encoder-controller_*_linux_amd64.deb
```

Then complete the setup:

1. **Edit the config** — `/etc/distributed-encoder/controller.yaml`
   - `database.url` — PostgreSQL connection string
   - `grpc.tls.cert` / `key` / `ca` — mTLS certificate paths (place files in `/etc/distributed-encoder/certs/`)
   - `auth.session_secret` — at least 32 random characters: `openssl rand -hex 32`

2. **Run database migrations** (requires [`golang-migrate`](https://github.com/golang-migrate/migrate)):
   ```bash
   migrate -path /usr/share/distributed-encoder/migrations \
           -database "postgres://distencoder:<pass>@localhost:5432/distencoder" up
   ```

3. **Start the service:**
   ```bash
   sudo systemctl start distributed-encoder-controller
   sudo systemctl status distributed-encoder-controller
   ```

The web UI is available at `http://<host>:8080`.
Logs: `journalctl -u distributed-encoder-controller -f`

To build the package locally (requires [`nFPM`](https://nfpm.goreleaser.com/)):

```bash
go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
make deb VERSION=1.2.0
# Output: dist/distributed-encoder-controller_1.2.0_linux_amd64.deb
```

### Controller — Native install (RHEL / Rocky Linux / AlmaLinux 9)

Download `distributed-encoder-controller_*_linux_amd64.rpm` from the [latest GitHub Release](https://github.com/badskater/distributed-encoder/releases/latest):

```bash
sudo dnf install -y ./distributed-encoder-controller_*_linux_amd64.rpm
```

Then complete the setup:

1. **Install PostgreSQL** (if not already running):
   ```bash
   sudo dnf install -y https://download.postgresql.org/pub/repos/yum/reporpms/EL-9-x86_64/pgdg-redhat-repo-latest.noarch.rpm
   sudo dnf -qy module disable postgresql
   sudo dnf install -y postgresql16-server
   sudo /usr/pgsql-16/bin/postgresql-16-setup initdb
   sudo systemctl enable --now postgresql-16
   ```

2. **Edit the config** — `/etc/distributed-encoder/controller.yaml`
   - `database.url` — PostgreSQL connection string
   - `grpc.tls.cert` / `key` / `ca` — mTLS certificate paths (place files in `/etc/distributed-encoder/certs/`)
   - `auth.session_secret` — at least 32 random characters: `openssl rand -hex 32`

3. **Run database migrations** (requires [`golang-migrate`](https://github.com/golang-migrate/migrate)):
   ```bash
   migrate -path /usr/share/distributed-encoder/migrations \
           -database "postgres://distencoder:<pass>@localhost:5432/distencoder" up
   ```

4. **Start the service:**
   ```bash
   sudo systemctl start distributed-encoder-controller
   sudo systemctl status distributed-encoder-controller
   ```

The web UI is available at `http://<host>:8080`.
Logs: `journalctl -u distributed-encoder-controller -f`

> **SELinux note:** If the service fails with an AVC denial, run:
> `sudo restorecon -rv /usr/bin/distributed-encoder-controller`

To build the package locally:

```bash
make rpm VERSION=1.2.0
# Output: dist/distributed-encoder-controller_1.2.0.x86_64.rpm
```

---

### Agent — Linux

```bash
# Debian / Ubuntu
sudo CONTROLLER_ADDRESS=encoder.example.com:9443 \
     AGENT_HOSTNAME=encode-01 \
     AGENT_VERSION=1.0.0 \
     CERT_DIR=/tmp/certs \
     ./scripts/install-agent-linux.sh

# Or install the package directly and configure manually
sudo apt install ./distributed-encoder-agent_*_linux_amd64.deb     # Debian/Ubuntu
sudo dnf install ./distributed-encoder-agent_*.x86_64.rpm          # RHEL/Rocky
```

The script detects the distro, downloads the appropriate package, copies certificates, writes `/etc/distributed-encoder/agent.yaml`, and starts the systemd service.

---

### Agent — Windows Server

#### GUI Installer (recommended)

Download `distencoder-agent-setup.exe` from the [latest GitHub Release](https://github.com/badskater/distributed-encoder/releases/latest) and run it as Administrator. The wizard collects:

- **Controller address** — host:port of the gRPC endpoint (e.g. `encoder.example.com:9443`)
- **Agent hostname** — pre-filled with the computer name; used in logs and the web UI
- **Release version** — downloaded from GitHub Releases during install
- **Certificate folder** — directory containing `ca.crt`, `<hostname>.crt`, `<hostname>.key`

The installer creates `C:\DistEncoder`, downloads the agent binary, copies certificates, writes `C:\ProgramData\distributed-encoder\agent.yaml`, and registers and starts the `distributed-encoder-agent` Windows Service.

To build the installer locally (requires [Inno Setup 6](https://jrsoftware.org/isdl.php)):

```bash
make installer VERSION=1.2.0
# Output: dist\distencoder-agent-setup.exe
```

#### Manual installation

```powershell
# Copy the agent binary and config to the agent host
# Edit agent.yaml with the correct controller address and cert paths

# Install as a Windows Service (requires elevated prompt)
.\distencoder-agent.exe install --config "C:\DistEncoder\agent.yaml"
.\distencoder-agent.exe start

# Run interactively for debugging
.\distencoder-agent.exe run --config "C:\DistEncoder\agent.yaml" --debug --http-debug

# Manage the service
.\distencoder-agent.exe stop
.\distencoder-agent.exe uninstall
```

The agent exposes a local debug HTTP server on `localhost:9080` when `--http-debug` is set:
- `GET /health` — JSON status (state, controller connection, current job, uptime)
- `GET /metrics` — Prometheus text metrics

After installation, approve the agent in the web UI or via the CLI:

```bash
controller agent approve <agent-hostname>
```

---

## First Run

After `docker compose up -d` and `make migrate-up`, open `http://localhost:8080`. You will be prompted to create the first admin account.

**Minimum setup before your first encode job:**

1. **Approve an agent** — Agents start in `PENDING_APPROVAL`. Go to **Farm Servers** and click **Approve**, or run:
   ```bash
   controller agent approve <agent-hostname>
   ```

2. **Add a source** — Go to **Sources → Add Source** and enter the UNC path to a media file (e.g., `\\NAS01\media\movie.m2ts`). Saving it automatically queues an `analysis` job and an `hdr_detect` job — no manual scan step required. These run on the next available idle agent.

3. **Create a template** — Go to **Templates → New Template**. Templates are Go `text/template` scripts that define encode parameters. The editor validates syntax before saving.

4. **Set global variables** — Go to **Variables** to define shared constants injected into all templates (e.g., `OUTPUT_DIR`, `PRESET`).

5. **Submit a job** — Go to **Sources**, select a file, click **Encode**, choose a template, and submit. The job appears on the **Jobs** page.

6. **Monitor progress** — Click a running job to see the live log stream and progress percentage from the agent.

---

## Configuration

### Agent (`agent.yaml`)

```yaml
controller:
  address: "controller.example.com:9443"
  tls:
    cert: "C:\\DistEncoder\\certs\\agent.crt"
    key:  "C:\\DistEncoder\\certs\\agent.key"
    ca:   "C:\\DistEncoder\\certs\\ca.crt"
  reconnect:
    initial_delay: 5s
    max_delay: 5m
    multiplier: 2.0

agent:
  hostname: "ENCODE-01"
  work_dir:  "C:\\DistEncoder\\work"
  log_dir:   "C:\\DistEncoder\\logs"
  offline_db: "C:\\DistEncoder\\offline.db"
  heartbeat_interval: 30s
  poll_interval: 10s
  cleanup_on_success: true
  keep_failed_jobs: 10

tools:
  ffmpeg:  "C:\\Tools\\ffmpeg\\ffmpeg.exe"
  ffprobe: "C:\\Tools\\ffmpeg\\ffprobe.exe"
  x265:    "C:\\Tools\\x265\\x265.exe"
  x264:    "C:\\Tools\\x264\\x264.exe"
  svt_av1: ""
  avs_pipe: "C:\\Program Files\\AviSynth+\\avs2pipemod.exe"
  vspipe:   "C:\\Program Files\\VapourSynth\\vspipe.exe"

gpu:
  enabled: true
  vendor: ""          # auto-detected; override: nvidia | intel | amd
  max_vram_mb: 0      # 0 = no limit
  monitor_interval: 5s

# UNC path allow-list — agent rejects paths outside these prefixes
allowed_shares:
  - "\\\\NAS01\\media"
  - "\\\\NAS01\\encodes"

logging:
  level: info
  format: json
```

See `configs/agent.yaml.example` for the full reference.

### Controller (`controller.yaml` / environment)

All controller settings can be supplied via `controller.yaml` or environment variables prefixed with `DE_`. Key variables:

| Variable | Description |
|---|---|
| `DE_DB_PASS` | PostgreSQL password |
| `DE_DB_HOST` | PostgreSQL host |
| `DE_DB_NAME` | Database name |
| `DE_GRPC_TLS_CERT/KEY/CA` | mTLS certificate paths |
| `DE_HTTP_PORT` | HTTP listen port (default `8080`) |
| `DE_GRPC_PORT` | gRPC listen port (default `9443`) |
| `DE_SESSION_SECRET` | 64-char hex session signing key |
| `DE_OIDC_CLIENT_ID/SECRET` | OIDC provider credentials (optional) |
| `DE_WEBHOOK_HMAC_SECRET` | Webhook HMAC signing secret |

See `configs/controller.yaml.example` and [DEPLOYMENT.md](DEPLOYMENT.md) for the full reference.

**Configuration precedence:** Environment variables (`DE_*`) always win over `controller.yaml`. The recommended pattern is to keep secrets exclusively in `.env` (or Docker secrets) and use `controller.yaml` only for non-secret settings like timeouts and pool sizes.

---

## API

The REST API is served at `/api/v1`. An OpenAPI 3.1 spec is available at `GET /api/v1/openapi.json`. Swagger UI at `/api/docs` (admin only).

All responses use a consistent JSON envelope:

```json
{ "data": { ... }, "meta": { "request_id": "req-456" } }
```

Errors use RFC 9457 Problem Details (`application/problem+json`).

Key resource groups:

| Prefix | Description |
|---|---|
| `/auth` | Login, logout, OIDC redirect |
| `/api/v1/sources` | Register sources (auto-schedules analysis), HDR detect, HDR metadata |
| `/api/v1/jobs` | Create, list, cancel, requeue encoding jobs |
| `/api/v1/agents` | List agents, approve, revoke, view capabilities |
| `/api/v1/analysis` | Histogram, VMAF, scene detection results |
| `/api/v1/webhooks` | Manage webhook endpoints |
| `/api/v1/variables` | Global script variables |
| `/api/v1/agent/upgrade/check` | Agent self-upgrade endpoint |

---

## Example Workflow

A complete encode from source file to finished output using the REST API:

```bash
# Authenticate and save session cookie
curl -s -c cookies.txt -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"yourpassword"}'

# Submit a job
curl -s -b cookies.txt -X POST http://localhost:8080/api/v1/jobs \
  -H "Content-Type: application/json" \
  -d '{"source_id":"<source-uuid>","template_id":"<template-uuid>","priority":50}'
# Returns: {"data":{"id":"<job-uuid>","status":"queued",...}}

# Poll for status
curl -s -b cookies.txt http://localhost:8080/api/v1/jobs/<job-uuid>

# Stream live logs (Server-Sent Events — Ctrl+C to stop)
curl -s -b cookies.txt http://localhost:8080/api/v1/jobs/<job-uuid>/log/stream

# List all agents and their current state
curl -s -b cookies.txt http://localhost:8080/api/v1/agents

# Approve a pending agent
curl -s -b cookies.txt -X POST http://localhost:8080/api/v1/agents/<agent-id>/approve
```

All of the above is also available via the web UI at `http://localhost:8080`.

---

## Agent State Machine

```
INITIALISING -> REGISTERING -> PENDING_APPROVAL -> IDLE -> VALIDATING -> RUNNING -> REPORTING -> IDLE
                                                    ^                                               |
                                               OFFLINE <------------------------------------------------
                                            (reconnects with backoff)
```

- **PENDING_APPROVAL** — new agents wait for admin approval before receiving tasks
- **VALIDATING** — pre-execution checks: `DE_PARAM_*` completeness, UNC path allow-list, script presence, timeout validity
- **OFFLINE** — agent continues any running task; results and logs are journaled to SQLite and replayed on reconnect

---

## Technology Stack

| Concern | Choice |
|---|---|
| Language | Go 1.25+ (CGO-free static binaries) |
| Agent to Controller | gRPC + mTLS (protobuf) |
| REST API | stdlib `net/http` |
| Web UI | React + Vite (embedded in binary via `embed.FS`) |
| Database | PostgreSQL 16+ with `pgx` |
| DB HA | Patroni + pgBouncer (optional) |
| Migrations | `golang-migrate` (plain SQL, embedded) |
| Query layer | `sqlc` generated |
| Agent offline store | SQLite via `modernc/sqlite` (pure Go, no CGO) |
| CLI | Cobra |
| Logging | `log/slog` (structured JSON) |
| Auth | Session cookies + optional OIDC (`go-oidc/v3`) |
| Password hashing | `bcrypt` |
| Containerisation | Docker multi-stage build (~30 MB image) |
| CI | GitHub Actions (`go build`, `go test`, `goreleaser`) |

---

## Development

```bash
# Run all tests with race detector
make test

# Lint
make lint

# Apply a database migration
DATABASE_URL="postgres://..." make migrate-up

# Roll back one migration
DATABASE_URL="postgres://..." make migrate-down

# Regenerate protobuf
make proto
```

Tests live alongside the code they cover in `internal/**/...`. Integration tests requiring a live database are skipped unless `TEST_DATABASE_URL` is set.

---

## Roadmap

### Phase 1 — UI Completeness ✅

- [x] Audio conversion page (API exists, needs frontend)
- [x] Path mappings admin page (UNC-to-Linux translations)
- [x] Agent enrollment tokens admin page
- [x] VNC "Open Remote Desktop" button on Agents page

### Phase 2 — Hardening ✅

- [x] SHA-256 verification for agent self-upgrade binaries
- [x] Agent upgrade rollback on failed restart
- [x] OIDC end-to-end integration tests
- [x] Job expansion rollback on partial task creation failure
- [x] Offline journal cleanup (prune old synced entries)

### Phase 3 — Chunked Encoding UI ✅

- [x] Scene-based chunking configuration in job creation
- [x] Chunk boundary visualization (preview API already exists)
- [x] Merge/concat task generation for chunked encodes

### Phase 4 — Observability & Polish ✅

- [x] Agent resource utilization graphs (CPU/GPU/memory over time)
- [x] Structured audit logging for sensitive actions
- [x] Bulk operations (approve multiple agents, cancel multiple jobs)
- [x] Dark mode toggle in web UI
- [x] Capacity planning documentation

### Phase 5 — Advanced Features

- [ ] Multi-controller HA (active-passive failover)
- [ ] Cloud storage source support (S3, GCS, Azure Blob)
- [ ] Job scheduling / cron (recurring encodes)
- [x] OpenAPI-generated client SDKs

---

## Documentation

| Document | Purpose |
|---|---|
| [ARCHITECTURE.md](ARCHITECTURE.md) | Full system design, data flows, component deep-dives |
| [AGENTS.md](AGENTS.md) | Agent specification, state machine, configuration reference |
| [DEPLOYMENT.md](DEPLOYMENT.md) | Step-by-step deployment, TLS setup, HA configuration, troubleshooting |
| [CLAUDE.md](CLAUDE.md) | Contributor and AI agent working instructions - Used to help build commits and documentations |

### API Client SDKs

The machine-readable REST API specification lives at [`api/openapi.yaml`](api/openapi.yaml) (OpenAPI 3.1). A JSON copy is also served at runtime by `GET /api/v1/openapi.json`.

Pre-generated client stubs are committed to `api/generated/` and kept up to date by the [`generate-sdk`](.github/workflows/generate-sdk.yml) CI workflow, which runs automatically when `api/openapi.yaml` changes on `main`.

**Regenerate clients locally:**

```bash
# TypeScript client (@hey-api/openapi-ts)
npm install --save-dev @hey-api/openapi-ts @hey-api/client-fetch
npx @hey-api/openapi-ts --config api/openapi-ts.config.ts

# Go client (oapi-codegen)
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest \
  -config api/oapi-codegen.yaml \
  api/openapi.yaml
```

Generated output directories:

| Path | Contents |
|---|---|
| `api/generated/ts/` | TypeScript fetch client |
| `api/generated/go/client.go` | Go typed HTTP client (package `apiclient`) |

---

## License

See [LICENSE](LICENSE).
