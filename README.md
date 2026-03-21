# Distributed Encoder

A distributed video encoding system that offloads encoding workloads from a central **Controller** to one or more **Agents**. The Controller orchestrates jobs, tracks state in PostgreSQL, exposes a REST and gRPC API, serves a management web UI, and fires webhooks on completion. Each Agent pulls work, executes batch and script files from UNC shares, and reports results — including GPU-accelerated encodes — surviving network outages via a local SQLite journal.

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
  AGENT x N   (Windows Server / Linux)
  +-- Windows Service / systemd
  +-- GPU encoding  (NVENC / QSV / AMF)
  +-- Offline resilience  (SQLite journal)
  +-- UNC / NFS file access  (NAS / SAN)
```

## Quick Start

```bash
# Build everything
make all

# Start Controller + PostgreSQL
cd deployments && docker compose up -d

# Apply migrations
DATABASE_URL="postgres://distenc:<pass>@localhost:5432/distencoder?sslmode=disable" make migrate-up

# Install agent (Windows — GUI installer)
# Download distencoder-agent-setup.exe from the latest GitHub Release

# Install agent (Linux)
sudo CONTROLLER_ADDRESS=encoder.example.com:9443 ./scripts/install-agent-linux.sh

# Approve agent
controller agent approve <hostname>
```

Web UI at `http://localhost:8080`. First visit prompts admin account creation.

## Features

- **Job orchestration** — priority queue, server-side script generation, chunked encoding with controller-side concat
- **Auto-analysis** — source registration auto-queues analysis + HDR detection
- **HDR/DV passthrough** — HDR10, HDR10+, Dolby Vision, HLG detection and metadata forwarding
- **GPU encoding** — NVIDIA NVENC, Intel QSV, AMD AMF auto-detected per agent
- **Offline resilience** — SQLite journal buffers results/logs during network outages
- **Live streaming** — stdout/stderr + progress parsing (x265, x264, SVT-AV1, FFmpeg) over gRPC
- **Webhooks** — Discord, Teams, Slack notifications
- **Auth** — local accounts, OIDC SSO with role mapping, API keys
- **HA** — active-passive failover via PostgreSQL advisory locks
- **Cloud storage** — S3, GCS, Azure Blob as source/destination
- **Scheduling** — cron-based recurring encodes
- **Auto-upgrade** — SHA-256 verified binary upgrades with rollback
- **Auto-retry** — exponential backoff on task failure
- **Observability** — Prometheus metrics, Grafana dashboards, structured audit logging
- **OpenAPI 3.1** — machine-readable spec + generated TypeScript/Go clients

## Documentation

All documentation lives in the **[Wiki](https://github.com/badskater/distributed-encoder/wiki)**.

| Resource | Description |
|---|---|
| **[Wiki Home](https://github.com/badskater/distributed-encoder/wiki)** | Navigation hub for all documentation |
| [Getting Started](https://github.com/badskater/distributed-encoder/wiki/Getting-Started) | Prerequisites, build, install, first run |
| [Deployment](https://github.com/badskater/distributed-encoder/wiki/Deployment) | Step-by-step deployment, TLS, HA, troubleshooting |
| [Agents](https://github.com/badskater/distributed-encoder/wiki/Agents) | Agent specification, state machine, configuration |
| [Configuration](https://github.com/badskater/distributed-encoder/wiki/Configuration) | Agent and Controller YAML reference |
| [API Reference](https://github.com/badskater/distributed-encoder/wiki/API-Reference) | REST endpoints and example workflow |
| [Architecture](https://github.com/badskater/distributed-encoder/wiki/Architecture) | System design and component deep-dives |
| [Roadmap](https://github.com/badskater/distributed-encoder/wiki/Roadmap) | All 6 completed phases |
| [CLAUDE.md](CLAUDE.md) | Contributor and AI agent working instructions - Used to help build commits and documentations |

Also in repo: [ARCHITECTURE.md](ARCHITECTURE.md) (detailed system design)

## License

[AGPL-3.0](LICENSE)
