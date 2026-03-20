# Agent Specification

Detailed design for the agent component of the Distributed Encoder system. For the full system architecture, see [ARCHITECTURE.md](ARCHITECTURE.md).

---

## 1. Overview

The Agent is a **single static Go binary** (~15-20 MB) that runs as a **Windows Service** (on Windows) or a **systemd service** (on Linux). It connects to the Controller via gRPC/mTLS, pulls tasks, executes scripts from UNC/NFS shares, and reports results. All task execution logs (stdout, stderr, agent-level events) are **streamed to the Controller** for centralized viewing in the web UI. It operates autonomously during network outages, buffering results and logs locally until reconnection.

On **Windows** agents, tasks are delivered as `.bat` scripts executed via `cmd.exe`. On **Linux** agents, tasks are delivered as `.sh` scripts executed via `/bin/sh`. Source files are accessed via SMB/UNC shares on Windows or NFS mounts on Linux.

---

## 2. Installation & Configuration

### 2.1 Prerequisites

#### Windows

| Requirement | Notes |
|---|---|
| **OS** | Windows Server 2019 / 2022 / 2025 |
| **FFmpeg** | Required. Must include `libvmaf` for VMAF analysis tasks. |
| **AviSynth+** | Optional. Required if encoding with `.avs` scripts. |
| **VapourSynth** | Optional. Required if encoding with `.vpy` scripts. |
| **Video encoder** | x264, x265, SVT-AV1, or GPU encoder (NVENC/QSV/AMF via ffmpeg or standalone). |
| **GPU drivers** | Current NVIDIA/Intel/AMD drivers if GPU encoding is used. |
| **Network** | Access to Controller on gRPC port (default 9443). Access to UNC file shares. |

#### Linux

| Requirement | Notes |
|---|---|
| **OS** | Ubuntu 22.04 LTS / Debian 12 / RHEL 9 or compatible |
| **FFmpeg** | Required. Must include `libvmaf`. Install via package manager or static build. |
| **VapourSynth** | Optional. Required if encoding with `.vpy` scripts. AviSynth+ is not available on Linux. |
| **Video encoder** | x264, x265, SVT-AV1, or GPU encoder (NVENC/VA-API via ffmpeg). |
| **GPU drivers** | NVIDIA: `nvidia-driver`; Intel: `intel-gpu-tools` / VA-API. |
| **NFS client** | `nfs-common` (Debian/Ubuntu) or `nfs-utils` (RHEL). Shares must be mounted before service start. |
| **Network** | Access to Controller on gRPC port (default 9443). NFS mounts accessible at configured paths. |

### 2.2 Service Installation

#### Windows

```powershell
# Install
.\distencoder-agent.exe install --config "C:\DistEncoder\agent.yaml"

# Manage
.\distencoder-agent.exe start
.\distencoder-agent.exe stop
.\distencoder-agent.exe uninstall

# Run interactively (for debugging)
.\distencoder-agent.exe run --config "C:\DistEncoder\agent.yaml" --debug
```

The binary uses [golang.org/x/sys/windows/svc](https://pkg.go.dev/golang.org/x/sys/windows/svc) for native Windows Service integration — no wrapper (NSSM) needed.

#### Linux

```bash
# Install (writes /etc/systemd/system/distributed-encoder-agent.service, enables unit)
sudo distencoder-agent install --config /etc/distributed-encoder/agent.yaml

# Manage
sudo distencoder-agent start
sudo distencoder-agent stop
sudo distencoder-agent uninstall   # removes unit file

# Run interactively (for debugging)
distencoder-agent run --config /etc/distributed-encoder/agent.yaml --debug
```

The binary integrates with **systemd** natively — the `install` subcommand writes the unit file, runs `systemctl daemon-reload`, and enables the service. Logs are written to the systemd journal (`journalctl -u distributed-encoder-agent -f`). See DEPLOYMENT.md §3.7 for the full Linux deployment procedure.

### 2.3 Configuration Reference

Example files are provided for both platforms:
- **Windows**: `configs/agent.yaml.example`
- **Linux**: `configs/agent-linux.yaml.example`

```yaml
# agent.yaml — full reference (Windows paths shown; see agent-linux.yaml.example for Linux)

controller:
  address: "controller.example.com:9443"
  tls:
    cert: "C:\\DistEncoder\\certs\\agent.crt"      # Linux: /etc/distributed-encoder/certs/agent.crt
    key: "C:\\DistEncoder\\certs\\agent.key"        # Linux: /etc/distributed-encoder/certs/agent.key
    ca: "C:\\DistEncoder\\certs\\ca.crt"            # Linux: /etc/distributed-encoder/certs/ca.crt
  # Reconnect settings (used during offline operation)
  reconnect:
    initial_delay: 5s
    max_delay: 5m
    multiplier: 2.0

agent:
  # Overrides auto-detected hostname
  hostname: "ENCODE-01"
  # Local working directory for scripts and temp files
  work_dir: "C:\\DistEncoder\\work"                 # Linux: /var/lib/distributed-encoder-agent/work
  # Log output directory
  log_dir: "C:\\DistEncoder\\logs"                  # Linux: /var/log/distributed-encoder-agent
  # SQLite database for offline result journaling
  offline_db: "C:\\DistEncoder\\offline.db"         # Linux: /var/lib/distributed-encoder-agent/offline.db
  # Heartbeat interval
  heartbeat_interval: 30s
  # Poll interval when idle (looking for new tasks)
  poll_interval: 10s
  # Clean up work dir after successful jobs
  cleanup_on_success: true
  # Keep work dir for N failed jobs (for debugging)
  keep_failed_jobs: 10

tools:
  ffmpeg: "C:\\Tools\\ffmpeg\\ffmpeg.exe"           # Linux: /usr/bin/ffmpeg
  ffprobe: "C:\\Tools\\ffmpeg\\ffprobe.exe"         # Linux: /usr/bin/ffprobe
  x265: "C:\\Tools\\x265\\x265.exe"                 # Linux: /usr/bin/x265
  x264: "C:\\Tools\\x264\\x264.exe"                 # Linux: /usr/bin/x264
  svt_av1: ""
  avs_pipe: "C:\\Program Files\\AviSynth+\\avs2pipemod.exe"  # Linux: "" (not available on Linux)
  vspipe: "C:\\Program Files\\VapourSynth\\vspipe.exe"       # Linux: /usr/bin/vspipe (if installed)

gpu:
  enabled: true
  # Auto-detects vendor. Override: nvidia | intel | amd
  vendor: ""
  # Limit VRAM usage (0 = no limit)
  max_vram_mb: 0
  # Monitor interval for GPU utilisation metrics
  monitor_interval: 5s

# Path allow-list (security: agent refuses paths outside these prefixes)
# Windows: UNC paths           Linux: NFS mount paths
allowed_shares:
  - "\\\\NAS01\\media"         # Windows UNC
  - "\\\\NAS01\\encodes"       # Windows UNC
  # - "/mnt/nas/media"         # Linux NFS mount
  # - "/mnt/nas/encodes"       # Linux NFS mount

logging:
  level: info          # debug, info, warn, error
  format: json         # json, text
  max_size_mb: 100     # per local system log file
  max_backups: 5       # rotated files to keep
  compress: true       # gzip old log files
  # Task log streaming (centralized logs sent to controller)
  stream_buffer_size: 1000   # in-memory buffer before writing to SQLite offline journal
  stream_flush_interval: 1s  # how often to flush buffered log lines to gRPC stream
```

---

## 3. State Machine

```
                         service start
                              │
                              ▼
                    ┌───────────────────┐
                    │    INITIALISING    │
                    │  - Load config     │
                    │  - Detect GPU      │
                    │  - Open offline DB │
                    │  - Validate tools  │
                    └────────┬──────────┘
                             │
                   ┌─────────┴──────────┐
                   │  Controller         │
                   │  reachable?         │
                   ├── YES ──┐   ┌── NO ┤
                   │         ▼   ▼      │
                   │  ┌────────────┐    │
                   │  │ REGISTERING│    │
                   │  └─────┬──────┘    │
                   │        │           │
                   │        ▼           ▼
                   │  ┌──────────────┐ ┌──────────────┐
                   │  │ PENDING      │ │  OFFLINE     │
                   │  │ APPROVAL     │ │  (waiting to │
                   │  │ (waiting for │ │   reconnect) │
                   │  │  admin)      │ │              │
                   │  └─────┬────────┘ └──────┬───────┘
                   │        │ approved        │ reconnect
                   │        ▼                 │ success
                   │  ┌────────────┐          │
                   │  │   IDLE     │◄─────────┘
                   │  │ (polling)  │
                   │  └─────┬──────┘
                   │        │
                   │        │ task received
                   │        ▼
                   │  ┌────────────┐
                   │  │ VALIDATING │ Pre-execution checks:
                   │  │            │ DE_PARAM_* vars, UNC paths,
                   │  │            │ script content, timeouts
                   │  └─────┬──────┘
                   │        │
                   │   ┌────┴────┐
                   │   │         │
                   │  pass     fail ──► report error, back to IDLE
                   │   │
                   │   ▼
                   │  ┌────────────┐
                   │  │  RUNNING   │ Execute .bat, stream logs
                   │  │  (locked)  │ + progress (1 task max)
                   │  └─────┬──────┘
                   │        │
                   │   ┌────┴────┐
                   │   ▼         ▼
                   │ success   failure
                   │   │         │
                   │   ▼         ▼
                   │  ┌────────────┐
                   │  │ REPORTING  │ Send result to Controller
                   │  │            │ (or journal to SQLite)
                   │  └─────┬──────┘
                   │        │
                   │        ▼
                   │     back to IDLE
                   └────────────────────
```

**Registration & Approval**: When the agent registers for the first time, the Controller creates a server record with status `pending_approval`. The agent remains in this state until an admin approves it via the web UI or CLI (`controller agent approve <name>`). Auto-approval can be enabled in the controller's `config.yaml` (`agent.auto_approve: true`) for trusted networks.

---

## 4. Offline Operation

### 4.1 Principles

1. **A running task is never interrupted** due to Controller loss.
2. **Results and logs are never lost** — they are journaled locally in SQLite.
3. **The agent does not accept new tasks** while offline (it has no way to receive them).
4. **Reconnection is automatic** with exponential back-off.
5. **Buffered logs are synced on reconnect** alongside task results, so the centralized log viewer has the full picture.

### 4.2 SQLite Journal Schema

```sql
-- Task results and progress (existing)
CREATE TABLE offline_results (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id      TEXT NOT NULL,
    result_type TEXT NOT NULL,      -- 'completed', 'failed', 'progress'
    payload     TEXT NOT NULL,      -- JSON-serialised TaskResult / ProgressUpdate
    created_at  TEXT NOT NULL,      -- ISO 8601
    synced      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_unsynced ON offline_results(synced) WHERE synced = 0;

-- Buffered log lines (centralized logging fallback when disconnected)
CREATE TABLE offline_logs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id      TEXT NOT NULL,
    stream      TEXT NOT NULL,      -- 'stdout', 'stderr', 'agent'
    level       TEXT NOT NULL DEFAULT 'info',
    message     TEXT NOT NULL,
    metadata    TEXT,               -- JSON: {"frame":2400,"fps":24.5}
    created_at  TEXT NOT NULL,      -- ISO 8601
    synced      INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_logs_unsynced ON offline_logs(synced) WHERE synced = 0;
```

### 4.3 Sync Flow

```
Agent reconnects to Controller
         │
         ▼
Query: SELECT * FROM offline_results WHERE synced = 0 ORDER BY id ASC
         │
         ▼
Stream results via gRPC SyncOfflineResults()
         │
         ▼
For each ACK from Controller:
    UPDATE offline_results SET synced = 1 WHERE id = ?
         │
         ▼
Query: SELECT * FROM offline_logs WHERE synced = 0 ORDER BY id ASC
         │
         ▼
Stream buffered log lines via gRPC StreamLogs()
         │
         ▼
For each ACK:
    UPDATE offline_logs SET synced = 1 WHERE id = ?
         │
         ▼
Resume normal IDLE → POLL cycle
```

Logs and results are synced in order (results first, then logs) so the Controller has task outcomes before the detailed log data.

---

## 5. Task Execution Detail

### 5.1 Pre-Execution Validation

Before executing any task, the agent runs validation checks (see ARCHITECTURE.md §3.2.5):

1. **Required parameters** — all `DE_PARAM_*` environment variables expected by the script are present and non-empty.
2. **Path validation** — UNC paths in parameters are checked for accessibility (file/directory exists and is reachable).
3. **Timeout sanity** — rejects tasks with timeout of `0` or negative duration.
4. **Script content** — verifies the `.bat` script content at the UNC path is non-empty and readable.

If validation fails, the agent immediately reports the task as `failed` with a descriptive error (e.g., `"validation: required param DE_PARAM_INPUT_PATH is empty"`) via both `ReportResult` and `StreamLogs` (error-level log entry). The server returns to `idle` and is available for other work.

### 5.2 Script Location & Execution

Scripts are generated by the Controller from templates and written to the UNC share in per-chunk subdirectories (see ARCHITECTURE.md §3.4). The `TaskAssignment` contains the UNC path to the chunk's script directory, not the script content itself:

```protobuf
message TaskAssignment {
  string job_id = 1;
  string source_path = 2;          // UNC path to source file
  string output_path = 3;
  string script_dir = 4;           // UNC path to chunk directory (e.g. \\NAS\...\chunks\chunk_000\)
  string script_type = 5;          // "avs" or "vpy"
  map<string, string> variables = 6;  // DE_PARAM_* variables for this chunk
  JobType job_type = 7;
  AudioParams audio_params = 8;
}
```

The controller generates one script set per chunk and writes them to the UNC share in per-chunk subdirectories:

```
\\NAS01\media\movie_01\
├── movie_01.m2ts                  (source file)
└── chunks\
    ├── chunk_000\
    │   ├── encode.bat             (run script, frames 0–1846)
    │   └── frameserver.avs        (or .vpy — Trim(0, 1846))
    ├── chunk_001\
    │   ├── encode.bat             (frames 1847–6831)
    │   └── frameserver.avs        (Trim(1847, 6831))
    └── ...
```

Each chunk's frameserver script contains the correct `Trim()` (AviSynth) or slice (VapourSynth) for that chunk's frame range. For scene-based chunking, boundaries align with detected scene cuts.

The agent executes `.bat` files directly from the UNC path via `cmd.exe /C "\\NAS01\...\chunks\chunk_000\encode.bat"`.

### 5.3 Process Execution

```go
// Simplified execution flow
cmd := exec.Command("cmd.exe", "/c", batFilePath)
cmd.Dir = workDir
cmd.Env = append(os.Environ(), deParamVars...)

stdout, _ := cmd.StdoutPipe()
stderr, _ := cmd.StderrPipe()

cmd.Start()

// Open centralized log stream to controller
logStream := grpcClient.StreamLogs(ctx)

// Capture stdout — each line goes to both progress parser and centralized log stream
go func() {
    scanner := bufio.NewScanner(stdout)
    for scanner.Scan() {
        line := scanner.Text()
        logStream.Send(&LogEntry{JobID: jobID, Stream: "stdout", Level: "info", Message: line})
        parseProgress(line, progressChan)
    }
}()

// Capture stderr — streamed to controller as error-level log entries
go func() {
    scanner := bufio.NewScanner(stderr)
    for scanner.Scan() {
        logStream.Send(&LogEntry{JobID: jobID, Stream: "stderr", Level: "warn", Message: scanner.Text()})
    }
}()

// Stream progress metrics separately (frame count, fps, ETA)
go streamProgress(progressChan, grpcClient)

// Agent-level events also go to the log stream
logStream.Send(&LogEntry{JobID: jobID, Stream: "agent", Level: "info", Message: "gpu_util: 87%"})

err := cmd.Wait()
logStream.CloseSend()
```

If the gRPC connection is lost during execution, log lines are buffered to the local SQLite `offline_logs` table and synced on reconnect (see §4.2).

### 5.4 Progress Parsing

The agent parses encoder stdout for progress indicators:

| Encoder | Pattern | Extracted |
|---|---|---|
| x265 | `[12.3%] 1200/9750 frames, 24.50 fps, ...` | frame, total, fps, percentage |
| x264 | `1200/9750 frames, 48.22 fps, ...` | frame, total, fps |
| SVT-AV1 | `Encoding frame 1200/9750` | frame, total |
| FFmpeg | `frame= 1200 fps= 24 ...` | frame, fps, time |

The parsed progress is sent as `ProgressUpdate` messages to the Controller every 5 seconds (configurable).

---

## 6. GPU Detection & Monitoring

### 6.1 Detection at Startup

```go
func DetectGPUs() []GPUInfo {
    var gpus []GPUInfo

    // NVIDIA: parse nvidia-smi --query-gpu
    if out, err := exec.Command("nvidia-smi",
        "--query-gpu=name,memory.total,driver_version,gpu_uuid",
        "--format=csv,noheader").Output(); err == nil {
        gpus = append(gpus, parseNvidiaSmi(out)...)
    }

    // Intel: enumerate via DXGI / WMI
    gpus = append(gpus, detectIntelGPU()...)

    // AMD: enumerate via DXGI / WMI
    gpus = append(gpus, detectAMDGPU()...)

    return gpus
}
```

### 6.2 Runtime Monitoring

While a GPU task is running, a background goroutine polls utilisation:

| Vendor | Method | Metrics |
|---|---|---|
| NVIDIA | `nvidia-smi --query-gpu=utilization.gpu,utilization.encoder,memory.used --format=csv` | GPU %, encoder %, VRAM used |
| Intel | `intel_gpu_top -J` (JSON mode) or WMI counters | Render %, video % |
| AMD | `amdgpu` or WMI counters | GPU %, VRAM used |

Metrics are included in `ProgressUpdate` messages and displayed in the web UI.

---

## 7. Security

### 7.1 mTLS Certificate Bootstrap

On first run, the agent can auto-enrol:

1. Agent generates a CSR (Certificate Signing Request).
2. Agent sends CSR to Controller via a one-time registration token (provided by admin in the web UI).
3. Controller signs the CSR with its internal CA.
4. Agent stores the signed certificate for all future connections.

Alternatively, certificates can be pre-provisioned and placed in the agent's cert directory.

### 7.2 UNC Path Validation

Before accessing any file, the agent checks against the `allowed_shares` list:

```go
func (a *Agent) ValidateUNCPath(path string) error {
    for _, prefix := range a.config.AllowedShares {
        if strings.HasPrefix(strings.ToLower(path), strings.ToLower(prefix)) {
            return nil
        }
    }
    return fmt.Errorf("path %q not in allowed shares", path)
}
```

This prevents a compromised Controller from directing the agent to access arbitrary network paths.

### 7.3 Bat Script Sandboxing

- Scripts are written to a dedicated work directory with restricted ACLs.
- The agent runs `cmd.exe` with the same service account (configure a low-privilege service account).
- Environment variables are explicitly set; the agent does not pass through all system env vars.

---

## 8. Logging & Diagnostics

The agent produces **two layers of logs** (see ARCHITECTURE.md §7 for the full observability model):

### 8.1 Task Logs (Centralized — streamed to Controller)

All task execution output is streamed to the Controller via gRPC `StreamLogs` and stored centrally in PostgreSQL (`task_logs` table). These logs are viewable in the web UI's log viewer with live tail, filtering, and search. **No need to access the agent machine to view encode output.**

| Stream | Content |
|---|---|
| `stdout` | Encoder output (frame progress, bitrate, timing). Parsed for progress metrics. |
| `stderr` | Encoder warnings, errors, diagnostic output. |
| `agent` | Agent-level events: task started, validation passed/failed, GPU utilisation readings, UNC path resolution, task completed/failed. |

Each log entry includes:
- `job_id` — links to the specific task
- `timestamp` — when the line was produced
- `stream` — which pipe it came from
- `level` — `debug`, `info`, `warn`, `error`
- `metadata` — optional structured data (frame count, fps, GPU util %)

If the gRPC connection is lost, log lines are buffered in the local SQLite journal (`offline_logs` table) and synced on reconnect.

### 8.2 System Logs (Local — agent process diagnostics)

Agent process-level logs use `log/slog` (stdlib, Go 1.21+) and are written locally:

```json
{
  "time": "2026-02-28T14:23:01.234Z",
  "level": "INFO",
  "source": {"function": "executor.Run"},
  "msg": "encode started",
  "server_name": "ENCODE-01",
  "assignment_id": "abc-123",
  "source_path": "\\\\NAS01\\media\\movie.m2ts",
  "encoder": "x265",
  "gpu": false
}
```

System logs cover:
- Service startup/shutdown
- Controller connection state changes
- gRPC stream lifecycle
- Offline journal operations
- Configuration reloads

These are written to `C:\DistEncoder\logs\` (rotated by size, compressed). They can optionally be shipped to an external aggregator (Loki, ELK) via Promtail or Filebeat for ops/SRE diagnostics.

### 8.3 Health Endpoint

When running interactively or with the `--http-debug` flag, the agent exposes a local HTTP endpoint:

```
GET http://localhost:9080/health
{
  "status": "running",
  "controller_connected": true,
  "current_job": "abc-123",
  "gpu": { "vendor": "nvidia", "model": "RTX 4090", "utilisation": 87 },
  "uptime": "4h32m",
  "log_stream": "connected",
  "offline_buffered_logs": 0
}

GET http://localhost:9080/metrics
# Prometheus-format metrics
```

---

## 9. Upgrade Strategy

The agent binary is a single file. Upgrades are performed by:

1. Controller pushes a new version notification via gRPC heartbeat response.
2. Agent finishes any running task (never interrupts).
3. Agent downloads the new binary to a staging path.
4. Agent verifies the binary checksum (SHA-256, provided by Controller).
5. Agent signals the Windows Service to restart with the new binary.

This can be automated or require manual approval in the web UI per-agent.
