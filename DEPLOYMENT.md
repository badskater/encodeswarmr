# Deployment Guide

Operational guide for deploying and running the Distributed Video Encoder system. For architecture details, see [ARCHITECTURE.md](ARCHITECTURE.md). For agent internals, see [AGENTS.md](AGENTS.md).

---

## 1. Prerequisites

### 1.1 Controller Host

| Requirement | Minimum | Recommended |
|---|---|---|
| **OS** | Ubuntu 22.04 LTS / RHEL 9 / Rocky Linux 9 (or any Docker host) | Ubuntu 24.04 LTS or Rocky Linux 9 |
| **CPU** | 2 cores | 4+ cores |
| **RAM** | 2 GB | 4+ GB |
| **Disk** | 20 GB | 50+ GB (log retention, analysis data) |
| **Docker** | 24.0+ with Compose v2 | Latest stable |
| **Network** | Ports 8080 (HTTP) and 9443 (gRPC) open to agents | Reverse proxy (nginx/Caddy) in front of 8080 |

### 1.2 Agent Host

| Requirement | Minimum | Recommended |
|---|---|---|
| **OS** | Windows Server 2019 | Windows Server 2022 |
| **CPU** | 4 cores | 8+ cores (encoding is CPU-intensive) |
| **RAM** | 8 GB | 16+ GB |
| **Disk** | 50 GB free (work directory) | SSD with 100+ GB |
| **GPU** | Optional | NVIDIA RTX series (NVENC) or Intel Arc (QSV) |
| **Network** | Access to Controller on port 9443; access to UNC file shares | 10 GbE for large m2ts files |

### 1.3 Shared Storage

- NAS or SAN accessible via UNC paths (`\\server\share`) from all agent machines.
- Agents need read access to source files and write access to output directories.
- Recommended: SMB3 with encryption enabled for data in transit.

### 1.4 Agent Tool Requirements

Each Windows Server agent requires the following tools installed and their paths configured in `agent.yaml`:

| Tool | Minimum Version | Download |
|---|---|---|
| **FFmpeg** | 6.1+ (static build) | https://www.gyan.dev/ffmpeg/builds/ — use `ffmpeg-release-full-shared.7z` |
| **x265** | 3.5+ | https://github.com/x265/x265 — pre-built Windows binaries in release assets |
| **x264** | r3100+ | https://artifacts.videolan.org/x264/release-win64/ |
| **SVT-AV1** | 1.8+ (optional) | https://gitlab.com/AOMediaCodec/SVT-AV1/-/releases |
| **AviSynth+** | 3.7.3+ | https://github.com/AviSynth/AviSynth/releases (includes `avs2pipemod.exe`) |
| **VapourSynth** | R65+ | https://github.com/vapoursynth/vapoursynth/releases |

**Recommended layout on each agent host:**

```
C:\Tools\
  ffmpeg\
    ffmpeg.exe
    ffprobe.exe
  x265\
    x265.exe
  x264\
    x264.exe
  svt-av1\
    SvtAv1EncApp.exe        # optional
C:\Program Files\
  AviSynth+\
    avs2pipemod.exe
  VapourSynth\
    vspipe.exe
```

**Verification — run in an elevated PowerShell prompt before starting the agent service:**

```powershell
& "C:\Tools\ffmpeg\ffmpeg.exe"  -version | Select-Object -First 1
& "C:\Tools\x265\x265.exe"      --version 2>&1 | Select-Object -First 1
& "C:\Tools\x264\x264.exe"      --version 2>&1 | Select-Object -First 1
& avs2pipemod                   --version
& vspipe                        --version
```

Each command should print a version string without error. If a tool is missing or has the wrong path, the agent logs `tool not found` and rejects tasks requiring that tool.

> **PATH note:** AviSynth+ and VapourSynth installers add themselves to the system PATH automatically. FFmpeg, x265, x264, and SVT-AV1 do not — configure the full path in `agent.yaml` under the `tools:` section, or add them to the system PATH manually.

### 1.5 PostgreSQL

- Version 16+ required.
- Can run as a container alongside the Controller (default) or as an external managed instance.
- For HA: Patroni + pgBouncer (see section 6).

---

## 2. Controller Deployment

### 2.1 Environment File

Create a `.env` file in the deployment directory. **Never commit this file to version control.** All environment variables use the `DE_` prefix (see ARCHITECTURE.md §6 for the full reference).

```bash
# .env — Controller
DE_DB_PASS=<strong-random-password>
DE_DB_HOST=postgres
DE_DB_PORT=5432
DE_DB_NAME=distencoder
DE_DB_USER=distenc

# gRPC TLS (paths inside the container)
DE_GRPC_TLS_CERT=/certs/server.crt
DE_GRPC_TLS_KEY=/certs/server.key
DE_GRPC_TLS_CA=/certs/ca.crt

# Web UI + REST API
DE_HTTP_PORT=8080
DE_GRPC_PORT=9443

# Webhook signing (optional, auto-generated if empty)
DE_WEBHOOK_HMAC_SECRET=<random-secret>

# Authentication
DE_SESSION_SECRET=<random-64-char-hex>
DE_OIDC_CLIENT_ID=                      # leave empty to disable OIDC
DE_OIDC_CLIENT_SECRET=
```

A `.env.example` is committed to the repo documenting all variables with placeholder values.

### 2.2 TLS Certificate Setup

Generate a self-signed CA and server/agent certificates for mTLS:

```bash
# Create CA
openssl genrsa -out ca.key 4096
openssl req -new -x509 -days 3650 -key ca.key -out ca.crt \
  -subj "/CN=DistEncoder CA"

# Create server cert (Controller)
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr \
  -subj "/CN=controller.internal"
openssl x509 -req -days 365 -in server.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out server.crt \
  -extfile <(printf "subjectAltName=DNS:controller.internal,DNS:localhost,IP:10.0.0.10")

# Create agent cert (repeat per agent, or use a wildcard)
openssl genrsa -out agent.key 2048
openssl req -new -key agent.key -out agent.csr \
  -subj "/CN=agent"
openssl x509 -req -days 365 -in agent.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out agent.crt
```

Place `ca.crt`, `server.crt`, and `server.key` in a `certs/` directory accessible to the Controller container.

### 2.3 Docker Compose Launch

The quickest path to a running controller is the provided bootstrap scripts, which install Docker CE, generate mTLS certificates, write the `.env`, and start the stack in a single command.

**Debian / Ubuntu (22.04 / 24.04):**

```bash
sudo ./scripts/install-controller.sh
```

**RHEL / Rocky Linux / AlmaLinux 9:**

```bash
sudo ./scripts/install-controller-rpm.sh
```

Both scripts accept the same environment variables (`DOMAIN`, `AGENT_NAMES`, `CONTROLLER_VERSION`, `POSTGRES_PASSWORD`, `SESSION_SECRET`) and are idempotent.

To start manually without the bootstrap script:

```bash
# From the deployments/ directory
cd deployments/

# Start everything
docker compose up -d

# Verify
docker compose ps
docker compose logs -f controller
```

Expected output (structured JSON via `log/slog`):
```
controller  | {"level":"info","msg":"starting controller","http_port":8080,"grpc_port":9443,"request_id":"init"}
controller  | {"level":"info","msg":"database connected","host":"postgres"}
controller  | {"level":"info","msg":"migrations applied","version":2}
controller  | {"level":"info","msg":"task log cleanup scheduled","retention":"30d","interval":"6h"}
controller  | {"level":"info","msg":"gRPC server listening","port":9443,"tls":true}
controller  | {"level":"info","msg":"HTTP server listening","port":8080}
```

### 2.4 Bare-Metal — Debian / Ubuntu (No Docker)

Install via the provided `.deb` package (recommended) or manually.

**`.deb` package install:**

```bash
# Build the package (from source)
make deb VERSION=1.2.0
# Produces: dist/distributed-encoder-controller_1.2.0_amd64.deb

# Install
sudo apt install -y ./dist/distributed-encoder-controller_1.2.0_amd64.deb

# Then follow the post-install instructions printed to the terminal.
```

**Manual install:**

```bash
# Install PostgreSQL
sudo apt install -y postgresql-16

# Create database and user
sudo -u postgres psql -c "CREATE USER distenc WITH PASSWORD '<password>';"
sudo -u postgres psql -c "CREATE DATABASE distencoder OWNER distenc;"

# Download controller binary
curl -Lo /usr/local/bin/distencoder \
  https://releases.example.com/distencoder-controller-linux-amd64
chmod +x /usr/local/bin/distencoder

# Create systemd service
cat > /etc/systemd/system/distencoder.service << 'EOF'
[Unit]
Description=Distributed Encoder Controller
After=network.target postgresql.service

[Service]
Type=simple
User=distencoder
EnvironmentFile=/etc/distencoder/.env
ExecStart=/usr/local/bin/distencoder run --config /etc/distencoder/controller.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable --now distencoder
```

### 2.5 Bare-Metal — RHEL / Rocky Linux / AlmaLinux (No Docker)

Install via the provided `.rpm` package (recommended) or manually.

**`.rpm` package install:**

```bash
# Build the package (from source)
make rpm VERSION=1.2.0
# Produces: dist/distributed-encoder-controller-1.2.0.x86_64.rpm

# Install
sudo dnf install -y ./dist/distributed-encoder-controller-1.2.0.x86_64.rpm

# Then follow the post-install instructions printed to the terminal.
```

**Manual install:**

```bash
# Install PostgreSQL
sudo dnf install -y https://download.postgresql.org/pub/repos/yum/reporpms/EL-9-x86_64/pgdg-redhat-repo-latest.noarch.rpm
sudo dnf install -y postgresql16-server postgresql16
sudo /usr/pgsql-16/bin/postgresql-16-setup initdb
sudo systemctl enable --now postgresql-16

# Create database and user
sudo -u postgres psql -c "CREATE USER distenc WITH PASSWORD '<password>';"
sudo -u postgres psql -c "CREATE DATABASE distencoder OWNER distenc;"

# Download controller binary
curl -Lo /usr/local/bin/distencoder \
  https://releases.example.com/distencoder-controller-linux-amd64
chmod +x /usr/local/bin/distencoder

# Create systemd service
cat > /usr/lib/systemd/system/distencoder.service << 'EOF'
[Unit]
Description=Distributed Encoder Controller
After=network.target postgresql-16.service

[Service]
Type=simple
User=distencoder
EnvironmentFile=/etc/distencoder/.env
ExecStart=/usr/local/bin/distencoder run --config /etc/distencoder/controller.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable --now distencoder
```

> **SELinux note:** On RHEL/Rocky with SELinux enforcing, the binary must have the correct context. If the service fails to start, run:
> ```bash
> sudo restorecon -v /usr/local/bin/distencoder
> # or for a package install:
> sudo restorecon -rv /usr/bin/distributed-encoder-controller
> ```

### 2.5 Initial Configuration via Web UI

1. Open `http://<controller-ip>:8080` in a browser.
2. Complete the first-run setup wizard:
   - Create an **admin** account (local username/password, hashed with bcrypt).
   - Set the default UNC share paths for source files and output.
   - Configure global variables (encoder paths, default parameters).
3. **(Optional) Configure OIDC/SSO** — under **Settings**, enter the OIDC provider URL, client ID, and client secret (Azure AD, Keycloak, or Google Workspace). OIDC users are auto-provisioned on first login if `auto_provision: true` is set in `config.yaml`.
4. Add webhook endpoints (Discord/Teams/Slack) under **Settings > Webhooks**.
5. Create at least one encoding template under **Templates**:
   - **Run scripts** (`.bat`) — define the encoder invocation (x265, SVT-AV1, ffmpeg, etc.).
   - **Frameserver scripts** (`.avs` / `.vpy`) — define video filtering (deinterlace, denoise, crop, etc.).
   - Use the **Template Editor** (`/admin/templates/{id}`) for syntax-highlighted editing with a variable reference panel and live preview. Templates use Go `text/template` syntax — see ARCHITECTURE.md §3.4 for available variables and custom functions.
   - Click **Preview** to render the template with sample data before saving. A bad template can cause all jobs to fail.
6. **Add sources** — Go to **Sources → Add Source** and enter a UNC path (e.g., `\\NAS01\media\movie.m2ts`). When a new source is saved, `analysis` (VMAF, scene detection, histogram) and `hdr_detect` jobs are **automatically queued**. Agents must be approved and idle for these to run. Scene boundaries from the analysis job unlock **scene-based chunking** in the encode config.
7. **Review the API docs** (admin only) at `/api/docs` to explore the REST API via the built-in Swagger UI. Disabled in production by default — enable in `config.yaml` with `api.docs_enabled: true`.

### 2.6 Agent Approval

When agents first connect, they appear in the web UI with a **pending_approval** status. An admin must approve each agent before it receives work:

1. Open **Farm Servers** in the web UI.
2. New agents appear in orange with status `pending_approval`.
3. Click **Approve** to move the agent to `idle` (it will start receiving tasks on next poll).

Alternatively, approve via CLI:

```bash
controller agent approve ENCODE-01
controller agent approve --all    # approve all pending agents (trusted networks only)
```

For trusted networks, set `agent.auto_approve: true` in `config.yaml` to skip manual approval.

---

## 3. Agent Deployment

### 3.1 Directory Structure

Create the following on each Windows Server agent:

```
C:\DistEncoder\
├── distencoder-agent.exe
├── agent.yaml
├── .env                (agent secrets — never committed)
├── certs\
│   ├── ca.crt          (same CA as controller)
│   ├── agent.crt
│   └── agent.key
├── work\               (auto-created, stores job scripts and temp files)
├── logs\               (auto-created, local system log output — task logs go to controller)
└── offline.db          (auto-created, SQLite journal for offline results + buffered logs)
```

### 3.2 Agent Configuration

Create `agent.yaml` (see [AGENTS.md](AGENTS.md) for the full reference):

```yaml
controller:
  address: "<controller-hostname-or-ip>:9443"
  tls:
    cert: "C:\\DistEncoder\\certs\\agent.crt"
    key: "C:\\DistEncoder\\certs\\agent.key"
    ca: "C:\\DistEncoder\\certs\\ca.crt"

agent:
  work_dir: "C:\\DistEncoder\\work"
  log_dir: "C:\\DistEncoder\\logs"
  offline_db: "C:\\DistEncoder\\offline.db"
  heartbeat_interval: 30s
  poll_interval: 10s
  cleanup_on_success: true

tools:
  ffmpeg: "C:\\Tools\\ffmpeg\\ffmpeg.exe"
  ffprobe: "C:\\Tools\\ffmpeg\\ffprobe.exe"
  x265: "C:\\Tools\\x265\\x265.exe"
  avs_pipe: "C:\\Program Files\\AviSynth+\\avs2pipemod.exe"
  vspipe: "C:\\Program Files\\VapourSynth\\vspipe.exe"

gpu:
  enabled: true

allowed_shares:
  - "\\\\NAS01\\media"
  - "\\\\NAS01\\encodes"

logging:
  level: info
  format: json
```

### 3.3 Service Installation

Run PowerShell as Administrator:

```powershell
# Install the Windows Service
C:\DistEncoder\distencoder-agent.exe install --config "C:\DistEncoder\agent.yaml"

# Start the service
C:\DistEncoder\distencoder-agent.exe start

# Verify
Get-Service DistEncoderAgent
```

The agent should appear as **pending_approval** in the Controller web UI within 30 seconds. An admin must approve it before it receives tasks (see §2.6 Agent Approval).

### 3.4 Tool Verification

Before assigning jobs, verify all required tools are accessible:

```powershell
# FFmpeg
& "C:\Tools\ffmpeg\ffmpeg.exe" -version

# x265
& "C:\Tools\x265\x265.exe" --version

# AviSynth+ (if used)
& "C:\Program Files\AviSynth+\avs2pipemod.exe" -h

# VapourSynth (if used)
& "C:\Program Files\VapourSynth\vspipe.exe" --version

# GPU (NVIDIA)
nvidia-smi
```

### 3.5 Encoding Workflow — Templates & Scene-Based Chunking

Once agents are approved and tools verified, the typical encoding workflow is:

1. **Register source** — an operator adds the source via **Sources → Add Source** (UNC path), or an agent auto-discovers it in a configured drop-folder.
2. **Analysis auto-scheduled** — when a source is registered, `analysis` (VMAF, histogram, scene detection) and `hdr_detect` jobs are automatically queued. Wait for these to complete before using scene-based chunking. To manually re-trigger analysis, use **Sources → Re-run Analysis** or the API endpoints `POST /api/v1/sources/{id}/analyze` and `POST /api/v1/sources/{id}/hdr-detect`.
3. **Configure encode** — navigate to **Sources > Configure Encode**:
   - Select a **run script** template (`.bat`) and a **frameserver** template (`.avs` or `.vpy`).
   - Choose **chunking mode**:
     - **Fixed-size** — splits the source into equal frame-count chunks.
     - **Scene-based** — uses scene detection results to generate per-scene chunks with correct `Trim()` (AVS) or slice (VPY) frame ranges. Requires a completed scene detection scan.
   - For scene-based mode, adjust **min merge** (merge scenes smaller than this threshold) and **max chunk** (split scenes larger than this) to control chunk granularity.
   - Click **Preview Scripts for Chunk N** to render the template for a specific chunk before submitting.
4. **Submit** — the controller generates one `.avs`/`.vpy` + `.bat` per chunk and writes them to the UNC share:
   ```
   \\NAS\media\source_dir\
   ├── source.m2ts
   └── chunks\
       ├── chunk_000\
       │   ├── encode.bat
       │   └── frameserver.avs
       ├── chunk_001\
       │   ├── encode.bat
       │   └── frameserver.avs
       └── ...
   ```
5. **Agents execute** — each agent picks up a chunk task, executes `encode.bat` directly from the UNC path, and streams logs + progress to the controller.

**Template management** is under **Admin > Templates**. The template editor provides syntax highlighting, a variable reference panel, and live preview. See ARCHITECTURE.md §3.4 for template variables (`.TrimStart`, `.TrimEnd`, `.ChunkIndex`, `.SceneIndex`, etc.) and custom functions (`uncPath`, `escapeBat`, `gpuFlag`, `trimAvs`, `trimVpy`).

### 3.6 Service Account

For production, run the agent under a dedicated low-privilege service account:

1. Create a local or domain user (e.g. `svc_distencoder`).
2. Grant it:
   - Read access to source UNC shares.
   - Read/write access to output UNC shares.
   - Full control of `C:\DistEncoder\`.
   - "Log on as a service" right.
3. Update the service to run as this account:
   ```powershell
   sc.exe config DistEncoderAgent obj= "DOMAIN\svc_distencoder" password= "<password>"
   ```

---

### 3.7 Linux Agent Deployment

Linux agents are supported for NFS-based encoding workflows. The agent binary is fully cross-compiled (`GOOS=linux GOARCH=amd64`) and integrates with **systemd** natively — no wrapper needed.

#### 3.7.1 Prerequisites

| Requirement | Notes |
|---|---|
| **OS** | Ubuntu 22.04 LTS / Debian 12 / RHEL 9 (or compatible) |
| **FFmpeg** | 6.1+ — must include `libvmaf` for VMAF analysis tasks |
| **x265 / x264** | Required for the respective encode templates |
| **VapourSynth** | Optional — install only if `.vpy` script templates are used |
| **GPU drivers** | NVIDIA: current `nvidia-driver` package; Intel: `intel-gpu-tools` |
| **NFS client** | `nfs-common` (Debian/Ubuntu) or `nfs-utils` (RHEL) for mounting NAS shares |
| **Network** | Access to Controller on gRPC port 9443; NFS mounts accessible at configured paths |

**Option A — package install (recommended):**

Debian / Ubuntu:

```bash
# Build or download the .deb package, then:
sudo apt install -y ./dist/distributed-encoder-agent_<version>_amd64.deb
# Post-install instructions are printed to the terminal.
```

RHEL / Rocky Linux / AlmaLinux:

```bash
# Build or download the .rpm package, then:
sudo dnf install -y ./dist/distributed-encoder-agent-<version>.x86_64.rpm
# Post-install instructions are printed to the terminal.
```

The package creates the service user, directories, and systemd unit automatically. Skip to §3.7.3 to configure and start.

**Option B — manual install (tool prerequisites):**

Debian / Ubuntu:

```bash
# Core tools
sudo apt-get install -y ffmpeg x265 x264 nfs-common

# NVIDIA GPU (if applicable)
sudo apt-get install -y nvidia-driver nvidia-cuda-toolkit

# VapourSynth (optional)
sudo apt-get install -y vapoursynth
```

RHEL / Rocky Linux / AlmaLinux:

```bash
# Enable EPEL and RPM Fusion for ffmpeg, x265, x264
sudo dnf install -y epel-release
sudo dnf install -y --nogpgcheck \
  https://download1.rpmfusion.org/free/el/rpmfusion-free-release-$(rpm -E %rhel).noarch.rpm \
  https://download1.rpmfusion.org/nonfree/el/rpmfusion-nonfree-release-$(rpm -E %rhel).noarch.rpm
sudo dnf install -y ffmpeg x265 x264 nfs-utils

# NVIDIA GPU (if applicable) — requires EPEL + RPM Fusion nonfree
sudo dnf install -y akmod-nvidia xorg-x11-drv-nvidia-cuda

# VapourSynth (optional — build from source or use a COPR repository)
```

#### 3.7.2 Directory Structure

```
/etc/distributed-encoder/
├── agent.yaml          (config — readable by service user only)
└── certs/
    ├── ca.crt          (same CA as controller)
    ├── agent.crt
    └── agent.key

/var/lib/distributed-encoder-agent/
├── work/               (auto-created, job scripts and temp files)
└── offline.db          (auto-created, SQLite journal)

/var/log/distributed-encoder-agent/
└── agent.log           (auto-created, rotated)

/usr/local/bin/
└── distencoder-agent   (the agent binary)
```

Set up directories and permissions:

```bash
sudo useradd -r -s /sbin/nologin distencoder-agent

sudo mkdir -p /etc/distributed-encoder/certs
sudo mkdir -p /var/lib/distributed-encoder-agent
sudo mkdir -p /var/log/distributed-encoder-agent

sudo chown -R distencoder-agent: /var/lib/distributed-encoder-agent
sudo chown -R distencoder-agent: /var/log/distributed-encoder-agent
sudo chown root:distencoder-agent /etc/distributed-encoder/agent.yaml
sudo chmod 640 /etc/distributed-encoder/agent.yaml
sudo chmod 600 /etc/distributed-encoder/certs/agent.key
```

#### 3.7.3 Agent Configuration

Copy the example config and edit for your environment:

```bash
sudo cp configs/agent-linux.yaml.example /etc/distributed-encoder/agent.yaml
sudo nano /etc/distributed-encoder/agent.yaml
```

Minimum required changes:

```yaml
controller:
  address: "<controller-hostname-or-ip>:9443"
  tls:
    cert: "/etc/distributed-encoder/certs/agent.crt"
    key:  "/etc/distributed-encoder/certs/agent.key"
    ca:   "/etc/distributed-encoder/certs/ca.crt"

tools:
  ffmpeg:  "/usr/bin/ffmpeg"
  ffprobe: "/usr/bin/ffprobe"
  x265:    "/usr/bin/x265"
  x264:    "/usr/bin/x264"

allowed_shares:
  - "/mnt/nas/media"
  - "/mnt/nas/encodes"
```

See `configs/agent-linux.yaml.example` for the full reference.

#### 3.7.4 NFS Mounts

Add NFS mounts to `/etc/fstab` so they are available before the agent starts:

```
nas01.example.com:/media    /mnt/nas/media    nfs    defaults,_netdev,ro    0 0
nas01.example.com:/encodes  /mnt/nas/encodes  nfs    defaults,_netdev,rw    0 0
```

Mount them now:

```bash
sudo mkdir -p /mnt/nas/media /mnt/nas/encodes
sudo mount -a
```

The `_netdev` option ensures the mounts are brought up after the network is available, before the agent service starts.

#### 3.7.5 systemd Service Installation

> **Package installs** (`.deb` / `.rpm`) register and enable the systemd unit automatically during package installation — skip this section if you used Option A.

For a manual binary install, copy the binary and register the service as root:

```bash
sudo cp distencoder-agent /usr/local/bin/distencoder-agent
sudo chmod +x /usr/local/bin/distencoder-agent

# Write the systemd unit file and enable the service
sudo /usr/local/bin/distencoder-agent install \
    --config /etc/distributed-encoder/agent.yaml

# Start the service
sudo /usr/local/bin/distencoder-agent start

# Verify it is running
systemctl status distributed-encoder-agent
```

The `install` subcommand writes `/etc/systemd/system/distributed-encoder-agent.service`, runs `systemctl daemon-reload`, and enables the unit so it starts automatically on boot.

> **RPM / SELinux note:** On RHEL/Rocky with SELinux enforcing, run `sudo restorecon -v /usr/local/bin/distencoder-agent` if the service fails to start with an `AVC denied` error.

To run interactively for testing:

```bash
sudo -u distencoder-agent /usr/local/bin/distencoder-agent run \
    --config /etc/distributed-encoder/agent.yaml --debug
```

Service management:

```bash
sudo /usr/local/bin/distencoder-agent stop
sudo /usr/local/bin/distencoder-agent start
sudo /usr/local/bin/distencoder-agent uninstall   # removes unit file
```

Or use `systemctl` directly:

```bash
sudo systemctl stop distributed-encoder-agent
sudo systemctl start distributed-encoder-agent
sudo systemctl disable distributed-encoder-agent
journalctl -u distributed-encoder-agent -f       # follow logs
```

#### 3.7.6 Tool Verification

Before assigning jobs, verify all required tools are accessible as the service user:

```bash
sudo -u distencoder-agent ffmpeg -version | head -1
sudo -u distencoder-agent x265 --version 2>&1 | head -1
sudo -u distencoder-agent x264 --version 2>&1 | head -1

# GPU (NVIDIA)
sudo -u distencoder-agent nvidia-smi
```

The agent logs `tool not found` at startup and rejects tasks requiring missing tools.

---

## 4. Database Migrations

### 4.1 Running Migrations

Migrations run automatically on Controller startup by default. To run them manually:

```bash
# Inside the container
docker compose exec controller distencoder migrate up

# Or with the standalone migrate tool
export DATABASE_URL="postgres://distenc:<password>@<host>:5432/distencoder?sslmode=require"
migrate -path internal/db/migrations -database "$DATABASE_URL" up
```

### 4.2 Rolling Back

```bash
# Roll back the last migration
migrate -path internal/db/migrations -database "$DATABASE_URL" down 1

# Check current version
migrate -path internal/db/migrations -database "$DATABASE_URL" version
```

### 4.3 Migration History Notes

| Migration | Description |
|---|---|
| `001` | Core tables: `users`, `agents`, `sources` |
| `002` | `jobs` and `tasks` tables; initial `job_type` constraint (`encode`, `analysis`, `audio`) |
| `003–009` | Task logs, templates, variables, webhooks, analysis results, sessions, encode config, enrollment tokens |
| `010` | `sources.hdr_type` and `sources.dv_profile` columns; extend `analysis_results` type constraint to include `hdr_detect` |
| `011` | **Bugfix**: extend `jobs.job_type` CHECK constraint to include `hdr_detect` (was omitted when the job type was introduced in `010`) |

> **Note:** If you are upgrading from a version prior to `011`, run migration `011` before deploying — any `hdr_detect` job insert will fail at the database level on PostgreSQL without it.

### 4.4 Creating New Migrations

```bash
migrate create -ext sql -dir internal/db/migrations -seq <description>
# Creates: NNN_<description>.up.sql and NNN_<description>.down.sql
```

---

## 5. Networking & Firewall

### 5.1 Required Ports

| From | To | Port | Protocol | Purpose |
|---|---|---|---|---|
| Agents | Controller | 9443/tcp | gRPC (TLS) | Agent registration, heartbeat, task polling, result reporting |
| Users/API | Controller | 8080/tcp | HTTPS | Web UI and REST API |
| Controller | PostgreSQL | 5432/tcp | TLS | Database connections |
| Agents | NAS/SAN | 445/tcp | SMB | UNC file share access |
| Controller | Discord/Teams/Slack | 443/tcp | HTTPS | Webhook delivery (outbound only) |

### 5.2 Windows Firewall (Agent)

The agent makes outbound connections only — no inbound firewall rules are required on the agent unless the optional health/metrics HTTP endpoint is enabled.

```powershell
# Optional: allow inbound for agent health endpoint
New-NetFirewallRule -DisplayName "DistEncoder Agent Health" `
  -Direction Inbound -Protocol TCP -LocalPort 9080 -Action Allow
```

### 5.3 Reverse Proxy (Recommended for Production)

Place nginx or Caddy in front of the Controller for TLS termination, rate limiting, and access logging:

```nginx
# /etc/nginx/sites-available/distencoder
server {
    listen 443 ssl;
    server_name encoder.example.com;

    ssl_certificate     /etc/letsencrypt/live/encoder.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/encoder.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

The gRPC port (9443) should **not** go through the HTTP reverse proxy — agents connect directly.

---

## 6. High Availability (PostgreSQL)

For environments that need zero-downtime database failover.

### 6.1 Patroni + pgBouncer Setup

```yaml
# deployments/patroni.yml
services:
  etcd:
    image: quay.io/coreos/etcd:v3.5
    environment:
      ETCD_LISTEN_CLIENT_URLS: http://0.0.0.0:2379
      ETCD_ADVERTISE_CLIENT_URLS: http://etcd:2379
    volumes:
      - etcd-data:/etcd-data

  pg-node-1:
    image: patroni-distencoder:latest
    environment:
      PATRONI_NAME: pg1
      PATRONI_ETCD_HOSTS: etcd:2379
      PATRONI_POSTGRESQL_DATA_DIR: /data/pg
      PATRONI_REPLICATION_USERNAME: replicator
      PATRONI_REPLICATION_PASSWORD: ${REPL_PASS}
      PATRONI_SUPERUSER_USERNAME: postgres
      PATRONI_SUPERUSER_PASSWORD: ${PG_SUPER_PASS}
    volumes:
      - pg1-data:/data/pg

  pg-node-2:
    image: patroni-distencoder:latest
    environment:
      PATRONI_NAME: pg2
      PATRONI_ETCD_HOSTS: etcd:2379
      PATRONI_POSTGRESQL_DATA_DIR: /data/pg
      PATRONI_REPLICATION_USERNAME: replicator
      PATRONI_REPLICATION_PASSWORD: ${REPL_PASS}
      PATRONI_SUPERUSER_USERNAME: postgres
      PATRONI_SUPERUSER_PASSWORD: ${PG_SUPER_PASS}
    volumes:
      - pg2-data:/data/pg

  pgbouncer:
    image: bitnami/pgbouncer:latest
    ports:
      - "6432:6432"
    environment:
      PGBOUNCER_DATABASE: distencoder
      POSTGRESQL_HOST: pg-node-1
      POSTGRESQL_PORT: 5432
      POSTGRESQL_USERNAME: distenc
      POSTGRESQL_PASSWORD: ${DB_PASS}

volumes:
  etcd-data:
  pg1-data:
  pg2-data:
```

### 6.2 Controller Connection String for HA

Point the Controller at pgBouncer instead of a single PostgreSQL instance:

```bash
DATABASE_URL=postgres://distenc:<password>@pgbouncer:6432/distencoder?sslmode=require
```

---

## 7. Upgrades

### 7.1 Controller Upgrade

```bash
# Pull new image
docker compose pull controller

# Rolling restart (zero downtime if behind a load balancer)
docker compose up -d controller

# Verify migrations applied
docker compose logs controller | grep "migrations applied"
```

For bare-metal:

```bash
# Download new binary
curl -Lo /usr/local/bin/distencoder.new \
  https://releases.example.com/distencoder-controller-linux-amd64-vX.Y.Z
chmod +x /usr/local/bin/distencoder.new

# Swap and restart
sudo systemctl stop distencoder
mv /usr/local/bin/distencoder.new /usr/local/bin/distencoder
sudo systemctl start distencoder
```

### 7.2 Agent Upgrade

1. In the web UI, **drain** the agent (prevents new task assignment).
2. Wait for any running task to complete.
3. Stop the service: `C:\DistEncoder\distencoder-agent.exe stop`
4. Replace the binary with the new version.
5. Start the service: `C:\DistEncoder\distencoder-agent.exe start`
6. Remove drain in the web UI.

If the agent supports auto-update (configured in `agent.yaml`), the Controller pushes a new-version notification and the agent self-updates after finishing its current task.

---

## 8. Backups

### 8.1 PostgreSQL

```bash
# Daily dump (run via cron on the controller host or a backup server)
pg_dump -h <pg-host> -U distenc -Fc distencoder > /backups/distencoder-$(date +%Y%m%d).dump

# Restore
pg_restore -h <pg-host> -U distenc -d distencoder --clean /backups/distencoder-YYYYMMDD.dump
```

### 8.2 What to Back Up

| Item | Location | Frequency |
|---|---|---|
| PostgreSQL database | Container volume or external DB | Daily (automated) |
| TLS certificates | `certs/` directory | On change |
| Controller config | `controller.yaml`, `.env` | On change |
| Agent configs | `C:\DistEncoder\agent.yaml`, `.env` per agent | On change |
| Encoding templates | Stored in DB (included in pg_dump) | — |
| Global variables | Stored in DB (included in pg_dump) | — |
| User accounts/sessions | Stored in DB (included in pg_dump) | — |
| Task logs | Stored in DB `task_logs` table (included in pg_dump) | Pruned by retention policy |

### 8.3 What NOT to Back Up

- Agent `work/` directory — ephemeral, rebuilt per job.
- Agent `offline.db` — synced to Controller on reconnect, then cleared.
- Docker volumes for `pgdata` if you're already doing `pg_dump`.

---

## 9. Monitoring & Health Checks

### 9.1 Controller Health

```bash
# HTTP health endpoint (uses RFC 9457 format on errors)
curl http://<controller>:8080/api/v1/health
# Expected: {"data":{"status":"ok","db":"connected","agents_online":3},"meta":{"request_id":"req-abc"}}

# Prometheus metrics
curl http://<controller>:8080/metrics

# OpenAPI spec (machine-readable)
curl http://<controller>:8080/api/v1/openapi.json

# Swagger UI (admin only, must be enabled in config.yaml)
# Open in browser: http://<controller>:8080/api/docs
```

### 9.2 Centralized Task Logs

All task execution logs (stdout, stderr, agent-level events) are centralized on the controller — there is **no need to SSH into agents** to view encode output.

**Web UI**: Navigate to **Jobs > Job Detail > Task**, and the built-in log viewer shows live and historical logs with stream/level filtering.

**REST API**:
```bash
# Paginated task logs (cursor-based)
curl http://<controller>:8080/api/v1/tasks/{id}/logs?stream=stderr&level=error&page_size=100

# Live tail via SSE (connect and stream new lines as they arrive)
curl -N http://<controller>:8080/api/v1/tasks/{id}/logs/tail

# Download full log as .log file
curl -O http://<controller>:8080/api/v1/tasks/{id}/logs/download
```

**Log retention** is configured in `config.yaml`:

```yaml
logging:
  task_log_retention: 30d           # how long task logs are kept (default: 30 days)
  task_log_cleanup_interval: 6h     # cleanup goroutine schedule
  task_log_max_lines_per_job: 500000  # safety cap per job
```

Old logs are automatically purged by a background goroutine. The `task_logs` table is indexed by `(job_id, timestamp)` for fast queries.

### 9.3 Scene Detection & Template Preview APIs

Useful API endpoints for automation and debugging template/chunking issues:

```bash
# Get scene detection results for a source
curl http://<controller>:8080/api/v1/sources/{id}/scenes

# Compute chunk boundaries from scene data (dry-run, does not create a job)
curl -X POST http://<controller>:8080/api/v1/sources/{id}/scenes/chunks \
  -H "Content-Type: application/json" \
  -d '{"min_merge": 500, "max_chunk": 15000}'

# Preview a rendered template with sample data (used by template editor)
curl -X POST http://<controller>:8080/api/v1/scripts/preview \
  -H "Content-Type: application/json" \
  -d '{"template_id": "<uuid>", "source_id": "<uuid>"}'

# Preview a template for a specific chunk/scene
curl -X POST http://<controller>:8080/api/v1/scripts/preview-chunk \
  -H "Content-Type: application/json" \
  -d '{"template_id": "<uuid>", "source_id": "<uuid>", "chunk_index": 0}'
```

These are useful for verifying that templates render correctly before submitting an encode job. The web UI's template editor and encode config page use these endpoints internally.

### 9.4 Agent Health

```powershell
# Service status
Get-Service DistEncoderAgent

# Local health endpoint (if --http-debug enabled)
Invoke-RestMethod http://localhost:9080/health
```

Note: Agent system-level logs (`log/slog` output) are still written locally to `C:\DistEncoder\logs\` and can be shipped to an external aggregator (Loki, ELK) for process-level diagnostics. Task execution logs go directly to the controller.

### 9.5 Key Alerts to Configure

| Alert | Condition | Severity |
|---|---|---|
| Agent offline | No heartbeat for > 2 minutes | Warning |
| Job stuck | Job in `running` state for > expected duration * 2 | Warning |
| Job failed | Any job enters `failed` state | Error |
| DB connection lost | Controller cannot reach PostgreSQL | Critical |
| Disk space low | Agent work directory < 10 GB free | Warning |
| GPU error | nvidia-smi / encoder returns error during task | Error |

---

## 10. Troubleshooting

### 10.1 Agent Cannot Connect to Controller

**Symptoms**: Agent logs show `gRPC connection failed` or `TLS handshake error`.

**Checks**:
1. Verify the Controller is running: `docker compose ps` or `systemctl status distencoder`.
2. Test network connectivity from agent: `Test-NetConnection <controller-ip> -Port 9443`.
3. Verify certificates:
   - Agent's `ca.crt` must match the CA that signed the Controller's `server.crt`.
   - Check certificate expiry: `openssl x509 -in agent.crt -noout -dates`.
   - Ensure the server cert SAN includes the hostname/IP the agent is connecting to.
4. Check the agent's `controller.address` in `agent.yaml` matches the server cert CN/SAN.

**Fix**: Regenerate certificates if expired. Ensure firewall rules allow port 9443 outbound from agent.

### 10.2 Agent Shows Pending or Online but Gets No Tasks

**Symptoms**: Agent appears in the web UI but never receives work.

**Checks**:
1. Verify the agent status is not **pending_approval** — an admin must approve new agents before they receive work (see §2.6).
2. Verify the agent is not in **drained** state in the web UI.
3. Check that queued jobs exist and match the agent's capabilities (e.g. a GPU-only job won't be sent to a CPU-only agent).
4. Verify the agent's `allowed_shares` include the UNC path of the source file.
5. Check Controller logs for scheduling errors.

**Fix**: Approve the agent if pending. Un-drain if drained. Ensure agent capabilities match job requirements.

### 10.3 Encode Fails Immediately

**Symptoms**: Job status goes to `failed` within seconds of starting.

**Checks**:
1. **Check the centralized log viewer first** — in the web UI, go to **Jobs > Job Detail > Task** and open the log viewer. The `agent`-stream logs show pre-execution validation results, and any `error`-level entries explain exactly what failed (e.g. `"validation: required param DE_PARAM_INPUT_PATH is empty"`).
2. Look at the job error message via API: `GET /api/v1/tasks/{id}/logs?level=error`.
3. If the agent validated successfully but the script failed, check `stdout`/`stderr` streams in the log viewer for encoder output.
4. Verify the UNC source path is accessible from the agent: `dir "\\NAS01\media\file.m2ts"`.
5. Verify the encoder binary exists at the path specified in `agent.yaml` `tools:`.

**Fix**: Correct template errors, tool paths, or UNC share permissions as indicated by the log viewer.

### 10.4 All Tasks in a Job Fail with Script Errors

**Symptoms**: Every task in a job fails with template rendering or script syntax errors.

**Checks**:
1. Open the **Template Editor** for the template used by the job. Check for Go `text/template` syntax errors (the editor validates on save).
2. Click **Preview** in the template editor to render with sample data — if the preview fails, the template is broken.
3. For scene-based jobs, verify scene detection completed successfully for the source: `GET /api/v1/sources/{id}/scenes`.
4. Check that chunk scripts exist on the UNC share: `dir "\\NAS01\media\source_dir\chunks\"`.
5. If scripts exist but contain bad content, the template rendered incorrectly. Use `POST /api/v1/scripts/preview-chunk` to debug specific chunks.

**Fix**: Fix the template in the editor, use Preview to verify, then retry the job. If chunk scripts on the share are stale, the controller regenerates them on retry.

### 10.5 Encode Fails Mid-Way

**Symptoms**: Job fails after running for some time. Partial output file may exist.

**Checks**:
1. **Check the centralized log viewer** — open the task's log viewer and filter by `stderr` stream or `error` level to find the failure point. The `agent` stream shows GPU utilisation readings leading up to the failure.
2. Download the full log via **Download .log** button (or `GET /api/v1/tasks/{id}/logs/download`) for offline analysis.
3. Look for disk space issues: `Get-PSDrive C` or check the output share.
4. For GPU encodes, check `nvidia-smi` for GPU errors or driver crashes.
5. Check if the source file is intact: `ffprobe "\\NAS01\media\file.m2ts"`.

**Fix**: Address disk space, GPU driver, or source file issues. Retry the job via the web UI.

### 10.6 Offline Agent Does Not Sync After Reconnect

**Symptoms**: Agent reconnects but offline results and logs are not appearing in the Controller.

**Checks**:
1. Check agent local system logs in `C:\DistEncoder\logs\` for `SyncOfflineResults` errors.
2. Verify the `offline.db` file exists and is not corrupted:
   ```powershell
   # Check unsynced results
   sqlite3 C:\DistEncoder\offline.db "SELECT count(*) FROM offline_results WHERE synced = 0;"
   # Check buffered log lines
   sqlite3 C:\DistEncoder\offline.db "SELECT count(*) FROM offline_logs WHERE synced = 0;"
   ```
3. Check Controller logs for sync stream errors.

**Fix**: If the offline DB is corrupted, the results and buffered logs for that period are lost — delete `offline.db` and restart the agent. The jobs will remain in `running` state on the Controller and can be manually marked as failed and retried.

### 10.7 Webhook Delivery Failures

**Symptoms**: Notifications not arriving in Discord/Teams/Slack.

**Checks**:
1. Check webhook delivery log in the web UI (**Webhook Config > Delivery Log**) or via API (`GET /api/v1/webhooks/{id}/deliveries`).
2. Look for HTTP response codes: 401 (bad token), 403 (permissions), 429 (rate limited).
3. Test the webhook URL manually:
   ```bash
   curl -X POST <webhook-url> -H "Content-Type: application/json" \
     -d '{"content":"test"}'
   ```
4. For Discord: ensure the webhook URL has not been rotated or deleted.
5. For Teams: ensure the Incoming Webhook connector is still active on the channel.
6. For Slack: ensure the app/bot has permission to post to the target channel.

**Fix**: Update the webhook URL in the web UI. Use the **Test Fire** button to verify delivery.

### 10.8 Database Connection Issues

**Symptoms**: Controller logs show `database connection refused` or `too many connections`.

**Checks**:
1. Verify PostgreSQL is running: `docker compose ps postgres` or `systemctl status postgresql`.
2. Check connection count: `SELECT count(*) FROM pg_stat_activity WHERE datname = 'distencoder';`
3. Check PostgreSQL logs for connection limit errors.
4. If using pgBouncer, verify pool settings.

**Fix**: Increase `max_connections` in `postgresql.conf` or add pgBouncer for connection pooling. Restart PostgreSQL after config changes.

---

## 11. Prevention Tips

- **Certificate expiry**: Set a calendar reminder 30 days before TLS certificate expiry. Consider automating renewal with a cron job.
- **Disk space**: Monitor agent work directories and output shares. Set up alerts at 80% usage.
- **Database bloat**: Schedule periodic `VACUUM ANALYZE` on the PostgreSQL database. The `task_logs` table can grow large — ensure `logging.task_log_retention` is set appropriately (default 30 days). The cleanup goroutine runs automatically.
- **Task log monitoring**: If the `task_logs` table grows unexpectedly, check for jobs producing excessive output (e.g., verbose encoder logging). Reduce retention or increase `task_log_max_lines_per_job` as needed.
- **Agent drift**: Keep all agents on the same binary version. Use the web UI to check agent versions and plan rolling upgrades.
- **Agent approval**: When adding new agents to the farm, remember they start in `pending_approval` state. Check the Farm Servers page after deployment.
- **Backup testing**: Periodically restore a database backup to a test instance to verify recoverability.
- **Template changes**: Use the **Preview** feature in the template editor before saving edits. A bad template can cause all jobs to fail. The editor validates Go `text/template` syntax and highlights errors before saving.
- **Scene detection**: Run scene detection before using scene-based chunking. If scene data is stale (source file changed), re-run the scan. Without scene data, the encode config page only offers fixed-size chunking.
- **Chunk UNC paths**: After submitting an encode job, the controller writes per-chunk scripts to `\\NAS\...\chunks\chunk_NNN\` on the UNC share. Ensure the agent service account has read access to the `chunks\` subdirectory structure. If chunk scripts are missing or inaccessible, the agent's pre-execution validation will fail the task immediately — check the centralized log viewer for details.
- **Share permissions**: After Windows updates or domain changes, verify that the agent service account still has access to UNC shares.
- **API versioning**: The REST API is versioned at `/api/v1`. When upgrading the controller, check the changelog for breaking API changes. The OpenAPI spec at `/api/v1/openapi.json` is the source of truth for the current API surface.
- **Session/OIDC expiry**: Session TTL defaults to 24h. OIDC tokens are refreshed automatically. If users report frequent logouts, check `auth.session_ttl` in `config.yaml`.

---

## 12. CLI Command Reference

The controller binary exposes a CLI in addition to the HTTP server. All commands accept `--config <path>` to specify a non-default config file.

```bash
controller <command> [flags]
```

| Command | Description |
|---|---|
| `controller run` | Start the HTTP + gRPC server (primary production command) |
| `controller agent approve <hostname>` | Approve a pending agent |
| `controller agent disable <hostname>` | Disable an approved agent |
| `controller agent list` | List all registered agents and their state |
| `controller source list` | List all registered media sources |
| `controller source scan <id>` | Trigger a rescan of a source directory |
| `controller job list` | List recent jobs (`--limit`, `--status` flags available) |
| `controller job cancel <id>` | Cancel a queued or running job |
| `controller job requeue <id>` | Requeue a failed or cancelled job |
| `controller template list` | List all encode templates |
| `controller user list` | List local users |
| `controller user create` | Create a local user (`--name`, `--password`, `--admin` flags) |
| `controller user reset-password <username>` | Reset a user's password |
| `controller webhook list` | List configured webhooks |
| `controller tls gen-certs` | Generate a self-signed CA + server cert (development only) |

**Examples:**

```bash
# Approve a newly connected agent
controller agent approve ENCODE-01

# List all jobs in running state
controller job list --status running

# Requeue a failed job
controller job requeue abc-123

# Create an admin user (useful for first-run recovery if locked out)
controller user create --name admin --password "s3cr3t" --admin
```

---

## 13. Webhook Event Reference

Webhooks fire for the following events. Configure which events a webhook receives in **Settings → Webhooks** or via the API (`POST /api/v1/webhooks`).

| Event | Fired When |
|---|---|
| `job.queued` | A new job is submitted and enters the queue |
| `job.started` | A job's first chunk begins executing on an agent |
| `job.completed` | All chunks succeed; job reaches `completed` state |
| `job.failed` | A chunk fails and retries are exhausted; job moves to `failed` |
| `job.cancelled` | A job is manually cancelled via the UI or API |
| `agent.connected` | An agent connects or reconnects after being offline |
| `agent.approved` | An admin approves a `pending_approval` agent |
| `agent.offline` | An agent misses its heartbeat deadline |
| `test` | Sent by the **Test Fire** button in the UI |

**Payload envelope** (all events):

```json
{
  "event": "job.completed",
  "timestamp": "2026-03-03T12:00:00Z",
  "data": {
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "job_name": "Movie.2024.mkv — x265 encode",
    "status": "completed",
    "agent": "ENCODE-01",
    "duration_seconds": 1847
  }
}
```

**Provider format:** The controller wraps the envelope for each provider automatically — Discord uses `{"content":"..."}`, Teams uses Adaptive Cards, Slack uses `{"text":"..."}`. You configure only the webhook URL and event list.

**HMAC signature:** When `DE_WEBHOOK_HMAC_SECRET` is set, each delivery includes `X-Hub-Signature-256: sha256=<hex>` computed over the raw JSON body. Verify in your receiver:

```python
import hmac, hashlib
expected = hmac.new(secret.encode(), body, hashlib.sha256).hexdigest()
assert hmac.compare_digest(expected, received_sig.removeprefix("sha256="))
```

---

## 14. Production Readiness Checklist

Run through this before going live with real workloads.

### Security
- [ ] `DE_SESSION_SECRET` is a random 32+ character string (not the example value)
- [ ] `.env` file has permissions `600` — `chmod 600 .env`
- [ ] `.env` is listed in `.gitignore` and has never been committed
- [ ] mTLS certificates use at least 2048-bit RSA or P-256 EC keys
- [ ] Calendar reminder set 30 days before TLS certificate expiry
- [ ] `DE_AGENT_AUTO_APPROVE=false` (default) — agents require manual approval
- [ ] Port 8080 is behind a reverse proxy with TLS; not exposed directly to the internet
- [ ] Port 9443 is accessible only from agent subnets

### Database
- [ ] `POSTGRES_PASSWORD` / `DE_DB_PASS` are strong, random passwords
- [ ] PostgreSQL data volume is on persistent storage (not ephemeral container storage)
- [ ] At least one backup taken and a restore test completed
- [ ] `VACUUM ANALYZE` scheduled (weekly) — cron or `pg_cron`
- [ ] `DE_TASK_LOG_RETENTION` set to an appropriate value (default `720h` = 30 days)

### Agents
- [ ] All required tools installed and verified on each agent host (see §1.4)
- [ ] Agent service account has read access to UNC source shares
- [ ] Agent service account has write access to UNC output/chunk directories
- [ ] `allowed_shares` in `agent.yaml` is restricted to minimum required paths
- [ ] Agent `work_dir` has 100 GB+ free disk space

### Observability
- [ ] Controller logs forwarded to a log aggregator or persistent file
- [ ] Agent logs archived (default: `C:\DistEncoder\logs\`)
- [ ] Alert configured for disk usage > 80% on agent work directories
- [ ] `job.failed` webhook configured to notify the ops team

### Operations
- [ ] All agents on the same binary version as the controller
- [ ] `scripts/gen-certs.sh` output stored securely (private keys are sensitive)
- [ ] OIDC redirect URL matches identity provider registration (if using SSO)

---

## 15. Web UI Guide

The web UI is served at `http://<controller-host>:8080` after login.

| Page | Purpose |
|---|---|
| **Dashboard** | Live overview: active agents, queued/running/failed job counts, recent activity feed |
| **Jobs** | Submit new jobs, view status, cancel, requeue, access live log stream per job |
| **Sources** | Register and scan UNC media directories; browse indexed files; trigger encodes |
| **Templates** | Create and edit Go `text/template` encode scripts; use **Preview** to validate before saving |
| **Variables** | Define global key-value pairs injected into all templates as `{{.Vars.NAME}}` |
| **Farm Servers** | View agent status, approve/revoke agents, inspect capabilities and active task |
| **Webhooks** | Configure Discord/Teams/Slack endpoints; filter by event type; view delivery history |
| **Users** | Manage local user accounts and admin roles (admin only) |
| **Logs** | Centralized log viewer across all agents and controller; filterable by job/agent |
| **API Docs** | Swagger UI at `/api/docs` — interactive REST API reference (admin only) |

**Template context variables:**

| Variable | Type | Description |
|---|---|---|
| `{{.SourcePath}}` | string | Full UNC path to the source file |
| `{{.OutputDir}}` | string | UNC path for output files |
| `{{.ChunkDir}}` | string | UNC path for this chunk's working directory |
| `{{.ChunkIndex}}` | int | Zero-based chunk index |
| `{{.TotalChunks}}` | int | Total number of chunks for this job |
| `{{.StartFrame}}` | int | First frame of this chunk (for trim operations) |
| `{{.EndFrame}}` | int | Last frame of this chunk |
| `{{.Vars.NAME}}` | string | Global variable `NAME` from the Variables page |
| `{{.Params.NAME}}` | string | Per-job parameter passed at submission time |

**Tips:**
- Use the **Preview** button in the template editor before saving — it validates Go `text/template` syntax and shows rendered output for a sample payload.
- A broken template causes all new jobs to fail at the script-generation step. Use the **Preview** feature to catch errors before they reach production.
- Variables are injected at render time. Changes to a variable value take effect on the next job submission — existing queued jobs use the values that were current at submission time.

---

## 12. Backup and Restore

The controller's only stateful component is the **PostgreSQL database**. Agent work directories (script files) are ephemeral and re-created on job retry. NAS/SAN source and output files are outside the controller's responsibility.

### 12.1 Backup Strategy

#### Recommended: `pg_dump` daily snapshot

Run as the `postgres` OS user (or any user with CONNECT privilege):

```bash
pg_dump \
  --format=custom \
  --compress=9 \
  --file=/backups/distencoder_$(date +%Y%m%d_%H%M%S).pgdump \
  "postgres://distencoder:<password>@localhost:5432/distencoder"
```

Store backups outside the database host (remote share, object storage, off-site).

**Suggested schedule (cron):**
```cron
# Daily backup at 02:00, keep 14 days
0 2 * * * postgres pg_dump --format=custom --compress=9 \
  --file=/backups/distencoder_$(date +\%Y\%m\%d).pgdump \
  "postgres://distencoder:<password>@localhost:5432/distencoder" \
  && find /backups -name 'distencoder_*.pgdump' -mtime +14 -delete
```

#### Alternative: WAL archiving (point-in-time recovery)

For production deployments or Patroni HA clusters, enable WAL archiving in `postgresql.conf`:

```ini
wal_level = replica
archive_mode = on
archive_command = 'test ! -f /wal_archive/%f && cp %p /wal_archive/%f'
```

Combine with a base backup:
```bash
pg_basebackup -D /backups/base_$(date +%Y%m%d) -Ft -z -P \
  -h localhost -U replication_user
```

### 12.2 Restore

#### From a `pg_dump` archive

```bash
# 1. Stop the controller to prevent new writes.
systemctl stop distencoder-controller   # or docker compose stop controller

# 2. Drop and recreate the database (or restore into a fresh DB).
psql -U postgres -c "DROP DATABASE IF EXISTS distencoder;"
psql -U postgres -c "CREATE DATABASE distencoder OWNER distencoder;"

# 3. Restore.
pg_restore \
  --dbname="postgres://distencoder:<password>@localhost:5432/distencoder" \
  --verbose \
  /backups/distencoder_20250101_020000.pgdump

# 4. Restart the controller (it will run migrations automatically on start).
systemctl start distencoder-controller
```

#### Verify restore

```bash
psql "postgres://distencoder:<password>@localhost:5432/distencoder" \
  -c "SELECT status, COUNT(*) FROM jobs GROUP BY status;"
```

### 12.3 What Is Not Backed Up

| Data | Location | Notes |
|---|---|---|
| Source video files | NAS/SAN UNC paths | Outside controller scope; backup at storage layer |
| Encoded output files | NAS/SAN UNC paths | Outside controller scope |
| Agent work directories | `C:\encoder-work\` on agents | Ephemeral; re-created on retry |
| Controller binary / config | `/opt/distencoder/` | Re-deployable from release artifacts |
| TLS certificates | `/etc/distencoder/certs/` | Back up separately or use cert-manager |

### 12.4 Backup Checklist

- [ ] `pg_dump` scheduled and retention policy confirmed
- [ ] Backup files written to a host different from the database server
- [ ] Restore procedure tested at least once (restore to a staging environment)
- [ ] Alert on backup job failure (check exit code in cron/systemd)
- [ ] WAL archiving considered for RPO < 1 day requirements
