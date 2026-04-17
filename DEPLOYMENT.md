# DEPLOYMENT.md — EncodeSwarmr

> **This is the canonical deployment entry point.**
> For deep dives, see the [Wiki](https://github.com/badskater/encodeswarmr/wiki).

---

## 0. Overview

EncodeSwarmr consists of a **Controller** (Linux / Docker) and one or more **Agents** (Windows Server or Linux). The Controller exposes a REST + Web UI on HTTP and a gRPC endpoint for agent communication, with state stored in PostgreSQL.

```
  Browser / REST API / Webhooks
              |
              v
  CONTROLLER  (Linux / Docker)
  +-- HTTP  :8080  (REST API + Web UI)
  +-- gRPC  :9443  (agent comms, mTLS)
  +-- PostgreSQL   (jobs, agents, results)
              | gRPC / mTLS
              v
  AGENT x N   (Windows Server / Linux)
  +-- Windows Service / systemd
  +-- GPU encoding  (NVENC / QSV / AMF)
  +-- Offline resilience  (SQLite journal)
  +-- UNC / NFS file access  (NAS / SAN)
```

**Port reference**

| Port | Protocol | Purpose |
|------|----------|---------|
| 8080 | HTTP | REST API + Web UI |
| 9443 | gRPC/TLS | Agent communication (mTLS) |
| 5432 | TCP | PostgreSQL (direct — skip in production, use pgBouncer) |
| 6432 | TCP | pgBouncer connection pool |

---

## 1. Prerequisites

### 1.1 Controller host (choose one)
- **Docker** (recommended): any Linux host with Docker Engine + Compose V2
- **Bare-metal**: Ubuntu 22.04 / 24.04 or RHEL 9 / Rocky Linux 9 / AlmaLinux 9
  - ffmpeg + ffprobe (required for controller-side analysis, HDR detect, VMAF, scene scan)
  - Optional: `dovi_tool` for Dolby Vision RPU extraction (bundled in the Docker image)
- **Kubernetes**: Helm 3.x + Kubernetes 1.25+

### 1.2 Agent host
- **Windows**: Windows Server 2019 or 2022 (64-bit)
- **Linux**: Debian/Ubuntu (apt) or RHEL/Rocky/AlmaLinux (dnf) or binary fallback

### 1.3 Database
- PostgreSQL 15+ (PostgreSQL 16 is used in the bundled Docker Compose stack)
- Migrations run automatically when the controller starts (see `internal/db/migrations`)

### 1.4 Encoding tools (per agent)
| Tool | Windows default path | Linux default path |
|------|----------------------|-------------------|
| ffmpeg | `C:\Tools\ffmpeg\ffmpeg.exe` | `/usr/bin/ffmpeg` |
| ffprobe | `C:\Tools\ffmpeg\ffprobe.exe` | `/usr/bin/ffprobe` |
| x265 | `C:\Tools\x265\x265.exe` | `/usr/bin/x265` |
| x264 | `C:\Tools\x264\x264.exe` | `/usr/bin/x264` |
| AviSynth+ (optional) | `C:\Program Files\AviSynth+\avs2pipemod.exe` | not supported |
| VapourSynth (optional) | `C:\Program Files\VapourSynth\vspipe.exe` | set path in config |
| dovi_tool (optional) | `C:\Tools\dovi_tool\dovi_tool.exe` | set path in config |

GPU drivers for hardware encoding (NVENC / QSV / AMF) must be installed separately on each agent host.

### 1.5 Build-from-source requirements
- Go 1.25+, Node 22+, npm
- nFPM (`go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest`) for `.deb` / `.rpm` packages
- Inno Setup 6 (`choco install innosetup`) for the Windows `.exe` installer
- `openssl` for certificate generation

---

## 2. Deployment paths

### 2.1 Docker Compose (recommended quickstart)

The fastest path to a running system. Uses the pre-built image from GHCR.

```bash
# Clone the repo or copy deployments/ and configs/ to your server
git clone https://github.com/badskater/encodeswarmr.git
cd encodeswarmr

# 1. Copy and edit environment file
cp .env.example .env
$EDITOR .env                 # Set POSTGRES_PASSWORD and AUTH_SESSION_SECRET at minimum

# 2. Copy and edit controller config
cp configs/controller.yaml.example configs/controller.yaml
$EDITOR configs/controller.yaml   # Set database.url, grpc.tls paths, auth.session_secret

# 3. Generate mTLS certificates (see §3)
./scripts/gen-certs.sh \
  --out certs \
  --controller-cn encoder.example.com \
  --controller-ip 10.0.0.10 \
  --agents "ENCODE-01,ENCODE-02"

# 4. Start the stack
cd deployments
docker compose up -d

# 5. Verify
curl http://localhost:8080/health
```

**Migrations run automatically** at container startup — no separate migration step is needed.

The compose file (`deployments/docker-compose.yml`) starts:
- `controller` — image `ghcr.io/badskater/encodeswarmr-controller:latest`, ports 8080 + 9443
- `postgres` — PostgreSQL 16, port 5432
- `pgbouncer` — connection pool, port 6432

For local development (PostgreSQL + pgAdmin only, controller runs natively):

```bash
docker compose -f deployments/docker-compose.dev.yml up -d
# pgAdmin: http://localhost:5050  (admin@local.dev / admin)
```

See wiki: [Deployment](https://github.com/badskater/encodeswarmr/wiki/Deployment)

---

### 2.2 Bare-metal via install scripts

**Ubuntu 22.04 / 24.04:**

```bash
sudo DOMAIN=encoder.example.com \
     AGENT_NAMES="ENCODE-01,ENCODE-02" \
     CONTROLLER_VERSION=v1.0.0 \
     ./scripts/install-controller.sh
```

**RHEL 9 / Rocky Linux 9 / AlmaLinux 9:**

```bash
sudo DOMAIN=encoder.example.com \
     AGENT_NAMES="ENCODE-01,ENCODE-02" \
     CONTROLLER_VERSION=v1.0.0 \
     ./scripts/install-controller-rpm.sh
```

Both scripts accept env vars or interactive prompts for:
- `CONTROLLER_VERSION` — release tag (e.g. `v1.0.0`); use `dev` for a local build
- `DOMAIN` — hostname or IP used in the TLS SAN and the access URL
- `AGENT_NAMES` — comma-separated agent hostnames for per-agent cert generation
- `POSTGRES_PASSWORD` — auto-generated if not set
- `SESSION_SECRET` — auto-generated if not set

Each script:
1. Installs Docker CE + Compose V2 if not present
2. Creates `/opt/encodeswarmr/` with `certs/`, `data/`, `logs/`, `scripts/`
3. Runs `gen-certs.sh` to generate CA + controller + per-agent mTLS certs
4. Writes `/opt/encodeswarmr/.env` (permissions 600)
5. Copies `deployments/docker-compose.yml` and runs `docker compose up -d`
6. Waits for PostgreSQL to become healthy

Migrations run automatically at container startup.

See wiki: [Deployment-Guide](https://github.com/badskater/encodeswarmr/wiki/Deployment-Guide)

---

### 2.3 Ansible / Terraform / Helm

**Ansible** — on-premise deployments (standard, HA, or Docker Compose mode):

```bash
cd deploy/ansible
cp inventory/hosts.yml.example inventory/hosts.yml
# Edit inventory/hosts.yml with your hosts and credentials
ansible-vault create inventory/group_vars/vault.yml
ansible-playbook -i inventory/hosts.yml playbooks/site.yml --ask-vault-pass
```

See [deploy/ansible/README.md](deploy/ansible/README.md) and wiki: [Ansible-Deployment](https://github.com/badskater/encodeswarmr/wiki/Ansible-Deployment)

**Terraform** — cloud infrastructure (choose provider):

```bash
cd deploy/terraform/aws     # or azure / gcp
cp terraform.tfvars.example terraform.tfvars
# Edit terraform.tfvars
terraform init && terraform apply
```

See wiki: [Terraform-AWS](https://github.com/badskater/encodeswarmr/wiki/Terraform-AWS) | [Terraform-Azure](https://github.com/badskater/encodeswarmr/wiki/Terraform-Azure) | [Terraform-GCP](https://github.com/badskater/encodeswarmr/wiki/Terraform-GCP)

**Helm** — Kubernetes:

```bash
helm install encodeswarmr ./deploy/helm/encodeswarmr \
  --set postgres.dsn="postgresql://user:pass@postgres:5432/encodeswarmr" \
  --set auth.sessionSecret="$(openssl rand -base64 32)"
```

See [deploy/helm/encodeswarmr/README.md](deploy/helm/encodeswarmr/README.md) and wiki: [Kubernetes-Helm](https://github.com/badskater/encodeswarmr/wiki/Kubernetes-Helm)

---

### 2.4 High Availability (Patroni + leader election)

The controller uses **PostgreSQL advisory locks** for active-passive HA at the application layer — only the lock-holding controller processes jobs. The Patroni config (`deployments/patroni.yml`) handles HA at the **PostgreSQL** layer.

```bash
# Ansible HA deployment (two controllers + HAProxy + Patroni)
ansible-playbook -i inventory/hosts.yml playbooks/ha.yml \
  -e use_patroni=true --ask-vault-pass
```

See wiki: [HA-Setup](https://github.com/badskater/encodeswarmr/wiki/HA-Setup)

---

## 3. Certificates (mTLS)

All agent-to-controller communication uses mutual TLS. Generate certs with:

```bash
./scripts/gen-certs.sh \
  --out /etc/encodeswarmr/certs \
  --controller-cn encoder.example.com \
  --controller-ip 10.0.0.10 \
  --agents "ENCODE-01,ENCODE-02,ENCODE-03"
```

**Options:**

| Flag | Default | Description |
|------|---------|-------------|
| `--out DIR` | `./certs` | Output directory |
| `--controller-cn CN` | `controller.internal` | Controller CN and SAN DNS name |
| `--controller-ip IP` | _(none)_ | Additional SAN IP address |
| `--days-ca N` | `3650` | CA certificate validity (days) |
| `--days-leaf N` | `365` | Server / agent cert validity (days) |
| `--agents LIST` | `agent` | Comma-separated agent names |

**Outputs** (in `--out DIR`):

| File | Goes to |
|------|---------|
| `ca.crt` | Controller + every agent |
| `ca.key` | Keep offline — only needed to sign new certs |
| `server.crt` / `server.key` | Controller only |
| `<agent-name>.crt` / `.key` | Corresponding agent only |

**Controller** (Docker Compose): mount the `certs/` directory into the container:
```yaml
# deployments/docker-compose.yml already includes:
- ../certs:/etc/encodeswarmr/certs:ro
```

**Windows agent**: copy `ca.crt`, `<hostname>.crt`, `<hostname>.key` to `C:\DistEncoder\certs\`

**Linux agent**: copy to `/etc/encodeswarmr/certs/`

Check expiry:
```bash
openssl x509 -in certs/server.crt -noout -dates
```

---

## 4. Configuration

### 4.1 `.env` file

Copy `.env.example` to `.env` and set at minimum:

```bash
# Required
POSTGRES_PASSWORD=<strong-password>
AUTH_SESSION_SECRET=<min-32-chars>   # generate: openssl rand -hex 32

# Optional — leave blank to disable
SMTP_PASSWORD=
TELEGRAM_BOT_TOKEN=
PUSHOVER_APP_TOKEN=
NTFY_TOKEN=
```

The `.env` file is loaded by Docker Compose and by the install scripts. Never commit it.

### 4.2 `configs/controller.yaml`

Copy `configs/controller.yaml.example` to `configs/controller.yaml`. Required fields:

| Field | Description |
|-------|-------------|
| `database.url` | PostgreSQL DSN |
| `grpc.tls.cert` / `.key` / `.ca` | mTLS cert paths |
| `auth.session_secret` | 32+ character secret (or use `AUTH_SESSION_SECRET` env var) |
| `analysis.ffmpeg_bin` / `.ffprobe_bin` | Paths to ffmpeg/ffprobe on the controller host |
| `analysis.thumbnail_dir` | Writable directory for source preview thumbnails |

The Docker image copies migrations to `/app/migrations`; the controller runs them at startup. For native installs the `.deb`/`.rpm` places migrations at `/usr/share/encodeswarmr/migrations/` — update `database.migrations_path` accordingly.

See wiki: [Configuration](https://github.com/badskater/encodeswarmr/wiki/Configuration) for the full field reference.

### 4.3 `configs/agent.yaml` (Windows) / `configs/agent-linux.yaml` (Linux)

Both install scripts write `agent.yaml` automatically. To configure manually, copy the appropriate example:

- Windows: `configs/agent.yaml.example` → `C:\ProgramData\encodeswarmr\agent.yaml`
- Linux: `configs/agent-linux.yaml.example` → `/etc/encodeswarmr/agent.yaml`

Required fields:

| Field | Description |
|-------|-------------|
| `controller.address` | `host:9443` |
| `controller.tls.cert` / `.key` / `.ca` | mTLS cert paths |
| `agent.hostname` | Name shown in the Web UI |
| `agent.work_dir` | Writable scratch directory for encode jobs |
| `tools.ffmpeg` / `.ffprobe` / `.x265` / `.x264` | Paths to encoding tools |

### 4.4 Session secret generation

```bash
openssl rand -hex 32
```

---

## 5. Install agents

### 5.1 Windows — GUI installer

Build the installer (requires Inno Setup 6 on Windows):

```bash
make installer VERSION=1.0.0
# Output: dist/encodeswarmr-agent-setup.exe
```

Or download the pre-built `encodeswarmr-agent-setup.exe` from the GitHub Release page. The wizard prompts for controller address, agent hostname, version to download, and cert directory. See [`installer/agent-setup.iss`](installer/agent-setup.iss).

**Windows — PowerShell script (non-interactive / scripted):**

```powershell
# Run as Administrator
.\scripts\install-agent.ps1 `
    -ControllerAddress "encoder.example.com:9443" `
    -AgentHostname "ENCODE-01" `
    -AgentBinary "C:\Downloads\encodeswarmr-agent.exe" `
    -CertDir "C:\Downloads\certs"

# Or download binary from GitHub Releases
.\scripts\install-agent.ps1 `
    -ControllerAddress "encoder.example.com:9443" `
    -Version "1.0.0"
```

The script creates `C:\DistEncoder\`, copies certs, writes `C:\ProgramData\encodeswarmr\agent.yaml`, installs, and starts the `encodeswarmr-agent` Windows Service.

### 5.2 Linux agent

```bash
sudo CONTROLLER_ADDRESS=encoder.example.com:9443 \
     AGENT_HOSTNAME=encode-01 \
     AGENT_VERSION=1.0.0 \
     CERT_DIR=/tmp/certs \
     ./scripts/install-agent-linux.sh
```

**Environment variables accepted by `install-agent-linux.sh`:**

| Variable | Description |
|----------|-------------|
| `CONTROLLER_ADDRESS` | Controller gRPC host:port (required) |
| `AGENT_HOSTNAME` | Agent name — defaults to `hostname -s` |
| `AGENT_VERSION` | Release version without `v` prefix (e.g. `1.0.0`) |
| `CERT_DIR` | Directory with `ca.crt`, `<hostname>.crt`, `<hostname>.key` — default `/tmp/certs` |
| `AGENT_BINARY` | Path to a pre-downloaded binary (skips download) |

The script auto-detects the package manager (apt / dnf / binary fallback), installs via `.deb`, `.rpm`, or raw binary, copies certs to `/etc/encodeswarmr/certs/`, writes `/etc/encodeswarmr/agent.yaml`, and starts the `encodeswarmr-agent` systemd service.

### 5.3 First-time agent approval

New agents register with status `pending` and will not receive jobs until approved.

**Option A — Web UI:** open `http://<controller>:8080` → Farm Servers → Approve

**Option B — Controller CLI:**

```bash
# Docker Compose
docker compose exec controller /app/controller agent approve ENCODE-01

# Native install
encodeswarmr-controller --config /etc/encodeswarmr/controller.yaml agent approve ENCODE-01
```

`agent approve` is implemented in `internal/controller/cli/cli.go` (`newAgentApproveCmd`).

---

## 6. Verification

**Health check** (endpoint registered at `GET /health` in `internal/controller/api/server.go`):

```bash
curl http://<controller-host>:8080/health
# Expected: HTTP 200
```

**Prometheus metrics** (endpoint registered at `GET /metrics`):

```bash
curl http://<controller-host>:8080/metrics
```

**Controller logs** (Docker Compose):

```bash
docker compose logs -f controller
```

**Agent status:**

```bash
# Linux
systemctl status encodeswarmr-agent
journalctl -u encodeswarmr-agent -f

# Windows (PowerShell)
Get-Service encodeswarmr-agent
Get-EventLog -LogName Application -Source 'encodeswarmr-agent' -Newest 20
```

Service names confirmed in [`scripts/install-agent-linux.sh`](scripts/install-agent-linux.sh) (`SERVICE_NAME="encodeswarmr-agent"`) and [`scripts/install-agent.ps1`](scripts/install-agent.ps1) (`$serviceName = 'encodeswarmr-agent'`).

---

## 7. Upgrade

### Tagged release (Docker Compose)

```bash
# Edit deployments/docker-compose.yml — set image tag to the new version, e.g.:
#   image: ghcr.io/badskater/encodeswarmr-controller:v1.1.0

cd deployments
docker compose pull
docker compose up -d
```

Migrations run automatically on startup.

### `.deb` packages (Ubuntu/Debian)

```bash
apt update && apt upgrade encodeswarmr-controller encodeswarmr-agent
```

Build a new `.deb` from source:

```bash
make deb VERSION=1.1.0          # controller
make deb-agent VERSION=1.1.0    # Linux agent
```

### `.rpm` packages (RHEL/Rocky/AlmaLinux)

```bash
dnf upgrade encodeswarmr-controller encodeswarmr-agent
```

Build a new `.rpm` from source:

```bash
make rpm VERSION=1.1.0          # controller
make rpm-agent VERSION=1.1.0    # Linux agent
```

### Agents — auto-upgrade

The controller distributes agent binaries via `GET /api/v1/agent/upgrade/check`. Place binaries in the directory configured at `upgrade.binary_dir` in `controller.yaml`. See wiki: [Agent-Update-Channels](https://github.com/badskater/encodeswarmr/wiki/Agent-Update-Channels)

### Ansible rolling upgrade

```bash
ansible-playbook -i inventory/hosts.yml playbooks/upgrade.yml \
  -e encodeswarmr_version=1.1.0 --ask-vault-pass
```

---

## 8. Rollback

### Docker — pin previous image tag

```bash
# In deployments/docker-compose.yml, set the previous tag, then:
docker compose pull
docker compose up -d
```

### `.deb` — downgrade to previous version

```bash
apt install encodeswarmr-controller=<prev-version>
```

### `.rpm` — downgrade to previous version

```bash
dnf downgrade encodeswarmr-controller
```

### Database rollback

Each `migrate-down` invocation reverts exactly one migration step:

```bash
DATABASE_URL="postgres://encodeswarmr:<pass>@localhost:5432/encodeswarmr?sslmode=disable" \
  make migrate-down
```

Run repeatedly to step back multiple versions. Check current version:

```bash
DATABASE_URL="postgres://encodeswarmr:<pass>@localhost:5432/encodeswarmr?sslmode=disable" \
  make migrate-status
```

### Agent binary rollback

Replace the binary on the update channel with the previous version. Agents will self-downgrade on next check. For manual rollback, re-run the installer or PowerShell script with the previous version number.

---

## 9. Backup & restore

**PostgreSQL** (Docker Compose — run from the host):

```bash
# Backup
docker compose exec postgres pg_dump -U encodeswarmr encodeswarmr > backup.sql

# Restore (stop the controller first to avoid write conflicts)
docker compose stop controller
docker compose exec -T postgres psql -U encodeswarmr encodeswarmr < backup.sql
docker compose start controller
```

**Native install:**

```bash
pg_dump -U encodeswarmr encodeswarmr > backup.sql
psql -U encodeswarmr encodeswarmr < backup.sql
```

**What to back up:**

| Item | Location | Notes |
|------|----------|-------|
| Database | PostgreSQL `encodeswarmr` database | Full state |
| `.env` | `/opt/encodeswarmr/.env` | Contains secrets |
| Certificates | `/opt/encodeswarmr/certs/` or `./certs/` | Including `ca.key` if kept |
| Controller config | `/etc/encodeswarmr/controller.yaml` | |
| Thumbnails (Docker) | `thumbnails_data` named volume | Source preview images |
| Agent binaries (Docker) | `agent_bins` named volume | Only if using auto-upgrade |

---

## 10. Troubleshooting quick list

| Symptom | Fix |
|---------|-----|
| `GET /health` returns non-200 | Check `docker compose logs controller` — usually a DB connection or cert file path issue |
| Agent stuck in `pending` | Approve via Web UI (Farm Servers) or `controller agent approve <name>` |
| Agent shows `stale` | Check systemd/service status; ensure heartbeat interval < `agent.heartbeat_timeout` (default 90s) |
| gRPC TLS handshake fails | Verify `ca.crt` on both sides is the same CA; run `openssl verify -CAfile ca.crt server.crt` |
| Controller cert CN mismatch | Re-generate certs with `--controller-cn` matching the hostname agents use to connect |
| Migrations fail at startup | Check `database.url` / `DATABASE_URL` is reachable; confirm PostgreSQL is healthy |
| `encodeswarmr-agent` service won't start (Windows) | Check `Get-EventLog -LogName Application -Source 'encodeswarmr-agent' -Newest 10`; verify cert files in `C:\DistEncoder\certs\` |
| `encodeswarmr-agent` service won't start (Linux) | `journalctl -u encodeswarmr-agent -n 50`; verify cert files in `/etc/encodeswarmr/certs/` |
| Encoding tools MISSING in installer | Install ffmpeg/x265/x264 to default paths then update `tools.*` in `agent.yaml` |
| pgBouncer `max_client_conn` exceeded | Increase `MAX_CLIENT_CONN` in docker-compose or reduce `database.max_conns` in controller.yaml |

Full runbook: wiki [Runbook](https://github.com/badskater/encodeswarmr/wiki/Runbook)

---

## 11. Links

| Resource | URL |
|----------|-----|
| Wiki home | https://github.com/badskater/encodeswarmr/wiki |
| Deployment | https://github.com/badskater/encodeswarmr/wiki/Deployment |
| Deployment guide (bare-metal) | https://github.com/badskater/encodeswarmr/wiki/Deployment-Guide |
| Ansible deployment | https://github.com/badskater/encodeswarmr/wiki/Ansible-Deployment |
| Terraform — AWS | https://github.com/badskater/encodeswarmr/wiki/Terraform-AWS |
| Terraform — Azure | https://github.com/badskater/encodeswarmr/wiki/Terraform-Azure |
| Terraform — GCP | https://github.com/badskater/encodeswarmr/wiki/Terraform-GCP |
| Kubernetes / Helm | https://github.com/badskater/encodeswarmr/wiki/Kubernetes-Helm |
| HA setup | https://github.com/badskater/encodeswarmr/wiki/HA-Setup |
| Configuration reference | https://github.com/badskater/encodeswarmr/wiki/Configuration |
| Agent update channels | https://github.com/badskater/encodeswarmr/wiki/Agent-Update-Channels |
| Testing guide | https://github.com/badskater/encodeswarmr/wiki/Testing-Guide |
| Runbook | https://github.com/badskater/encodeswarmr/wiki/Runbook |
| ARCHITECTURE.md §5 | [ARCHITECTURE.md](ARCHITECTURE.md) |
