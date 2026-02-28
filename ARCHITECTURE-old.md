# Distributed Encoder — Architecture Document

## 1. Overview

Distributed Encoder is a Go application that operates like a **render farm** with a **VMAF-first workflow**: `.m2ts` video files are placed in a shared UNC drop-folder, the system automatically runs a VMAF quality scan, and the results are presented in a web UI where users configure and launch encoding jobs. The controller splits each job into **tasks** and distributes them across a fleet of Windows Server systems. Each server processes one task at a time — when it finishes, it immediately picks up the next available task from the queue.

**Workflow at a glance:**

1. A `.m2ts` source file is placed in a watched UNC drop-folder.
2. An agent detects the file and notifies the controller, which creates a **source** record.
3. The controller dispatches an automatic **VMAF scan** task — an agent runs `ffmpeg -lavfi libvmaf` against the source and reports quality metrics back.
4. The user opens the web UI, reviews the VMAF results, selects a **run script template** (encoding commands) and a **frameserver script template** (AviSynth/VapourSynth filtering), and configures encoding parameters (preset, CRF, chunking, etc.).
5. On submission, the selected templates are copied to the UNC path alongside the source file, and the controller creates the encoding **job**, splits it into tasks, and distributes them across the farm.

Target Windows Servers are deployed as Proxmox VMs. See [DEPLOYMENT.md](DEPLOYMENT.md) for provisioning and installation steps.

The system has three parts:

- **Controller** — a single Go binary that hosts the REST API, web interface (with built-in authentication and optional OIDC), dispatch logic, webhook notifications, and connects to a PostgreSQL database. It manages the source file pipeline (detection → VMAF scan → user configuration → encoding job), splits jobs into tasks, and assigns tasks to idle servers.
- **Agent** — a lightweight Go binary installed as a Windows Service on each target server. It self-registers with the controller on first poll, validates task parameters before execution, executes `.cmd` scripts locally (with `ffmpeg`/`libvmaf` for VMAF scans and video encoding), and reports results back. Scripts write output to a shared UNC path. Agents scan UNC drop-folders for new `.m2ts` files and notify the controller of new sources.
- **Web UI** — a TypeScript + React browser interface served by the controller. Operators review VMAF scan results for detected source files, select run script and frameserver script templates, configure encoding parameters, submit encoding jobs, monitor task progress across the farm, and administer users.

```
 ┌─────────────┐
 │ UNC Drop    │  .m2ts files placed here
 │ Folder(s)   │
 └──────┬──────┘
        │  agent detects new .m2ts
        │
┌───────▼──────────────────────────────────────────────────────────┐
│                       CONTROLLER HOST                            │
│                                                                  │
│  ┌──────────┐   ┌───────────────┐   ┌──────────────┐            │
│  │  Web UI  │──>│   REST API    │<──│  CLI         │            │
│  │ (React)  │   │   (Go HTTP)   │   │  (cobra)     │            │
│  └──────────┘   └───────┬───────┘   └──────────────┘            │
│   VMAF results          │                                        │
│   Template picker       │                                        │
│   Encode config  ┌──────▼───────┐   ┌──────────────┐            │
│                  │  Dispatcher  │──>│  Task Queue   │            │
│                  │  (Scheduler) │   │ (PostgreSQL)  │            │
│                  └──────────────┘   └──────────────┘            │
└─────────────────────────────┬────────────────────────────────────┘
                              │  Agents poll over HTTPS
                              │  (VMAF scans + encode tasks)
          ┌───────────────────┼───────────────────────┐
          │                   │                       │
   ┌──────▼──────┐    ┌──────▼──────┐        ┌──────▼──────┐
   │ WinServer1  │    │ WinServer2  │        │ WinServerN  │
   │ ┌─────────┐ │    │ ┌─────────┐ │        │ ┌─────────┐ │
   │ │  Agent  │ │    │ │  Agent  │ │  ...   │ │  Agent  │ │
   │ │(service)│ │    │ │(service)│ │        │ │(service)│ │
   │ └─────────┘ │    │ └─────────┘ │        │ └─────────┘ │
   │  (busy)     │    │  (idle)     │        │  (idle)     │
   └─────────────┘    └─────────────┘        └─────────────┘
```

---

## 2. Core Concepts

| Concept              | Definition                                                                 |
|----------------------|---------------------------------------------------------------------------|
| **Source**           | A `.m2ts` video file detected in a UNC drop-folder. A source is the starting point of the pipeline — it gets a VMAF scan before the user configures an encoding job for it. |
| **VMAF Scan**        | An automatic quality analysis task dispatched when a new source is detected. An agent runs `ffmpeg -lavfi libvmaf` against the source file and reports metrics (VMAF score, PSNR, SSIM) back to the controller. |
| **Script Template**  | A reusable script managed via the web UI. Two types: **run scripts** (`.cmd` files containing encoding commands) and **frameserver scripts** (AviSynth `.avs` or VapourSynth `.vpy` files for frame processing/filtering). Templates are stored in the database and copied to the UNC path alongside the source when an encoding job is submitted. |
| **Job**              | An encoding job tied to a source file. A job references a run script template, an optional frameserver script template, defines encoding parameters, and specifies how to split work into tasks (e.g., frame ranges). |
| **Task**             | One chunk of a job. Each task runs the same `.cmd` script with different chunk-specific parameters. A single-chunk job has exactly one task. |
| **Server**           | A Windows Server target (Proxmox VM) registered in the server inventory.  |
| **Assignment**       | The binding of one task to one server. A server holds at most one active assignment. When it completes, the server is immediately eligible for the next task. |
| **Agent**            | A lightweight Go service running on each target server. Polls the controller for work (VMAF scans and encoding tasks). Also scans UNC drop-folders for new `.m2ts` files. |
| **Dispatch**         | The act of assigning a task to a server. The agent picks it up on its next poll. |
| **Result**           | Stdout, stderr, and exit code captured by the agent and reported back to the controller. |

---

## 3. Component Design

### 3.1 Server Inventory (Dynamic Registration)

Servers register themselves dynamically. When an agent starts for the first time, it sends a registration request to the controller with its `server_name`, `host`, and `tags`. The controller creates a new server record in the database with `state = pending_approval`.

**Registration flow:**

1. Agent starts and calls `POST /api/agent/register` with its name, host IP, and tags.
2. Controller creates the server in `pending_approval` state. The agent receives a `202 Accepted`.
3. An admin approves the server via the web UI or CLI (`controller server approve <name>`).
4. Once approved, the server moves to `idle` and the agent begins receiving work on its next poll.

Auto-approval can be enabled in `config.yaml` (`agent.auto_approve: true`) for trusted networks where any agent should be accepted immediately.

An optional `inventory.json` seed file can pre-register servers at controller startup (useful for initial deployment):

```jsonc
{
  "servers": [
    {
      "name": "APP-SERVER-01",
      "host": "10.0.1.10",
      "tags": ["encoder"],
      "enabled": true
    }
  ]
}
```

Credentials are stored in a `.env` file at the project root, excluded from version control via `.gitignore`. A `.env.example` file documents required variables without real values (see Section 7).

### 3.2 Source Pipeline & Job Definitions

The system follows a **detect → scan → configure → encode** pipeline for every `.m2ts` source file.

**Pipeline stages:**

```
 .m2ts detected         VMAF scan           User configures       Encoding job
 in UNC folder    ───>  dispatched    ───>  via web UI      ───>  distributed
 (source created)       (auto task)         (templates +          across farm
                                             parameters)
```

**Stage 1 — Source detection:** An agent's UNC scanner detects a new `.m2ts` file and notifies the controller via `POST /api/agent/source`. The controller creates a `source` record with state `detected`.

**Stage 2 — VMAF scan:** The controller automatically creates a single-task VMAF scan job for the source. An agent picks it up, runs `ffmpeg -lavfi libvmaf` to analyze video quality, and reports metrics (VMAF score, PSNR, SSIM, frame count, resolution, duration) back. The source moves to state `scanned` and the metrics are stored in the `vmaf_results` table.

**Stage 3 — User configuration:** The user opens the source in the web UI, reviews the VMAF results, and configures the encoding job:
- Selects a **run script template** — the `.cmd` file containing encoding commands (e.g., H.265 transcode, AV1 encode).
- Selects an optional **frameserver script template** — an AviSynth (`.avs`) or VapourSynth (`.vpy`) script for frame processing (deinterlacing, denoising, cropping, etc.).
- Sets encoding parameters: preset, CRF/bitrate, codec, chunking (frame range + chunk size), priority, timeout.

**Stage 4 — Job submission (template deployment + task creation):**

When the user submits the encode configuration, the controller performs the following steps before any agent picks up work:

1. **Reads the selected templates from the database** — the run script template content and (optionally) the frameserver script template content.
2. **Writes the templates as files to the source's UNC directory.** The run script is written as `encode.bat` and the frameserver script is written as `frameserver.avs` (or `.vpy`) alongside the original `.m2ts` file. These are real files on the UNC share — every agent in the farm can access them.
3. **Creates the encoding job** tied to the source record.
4. **Generates tasks from the chunking config.** Each task's parameters include `DE_PARAM_SOURCE_DIR` (the UNC directory containing the `.m2ts`, `encode.bat`, and frameserver script). The task does **not** carry the script content in its payload — the agent reads and executes the `.bat` file directly from the UNC path.
5. Tasks are queued for dispatch across the farm.

```
 User clicks "Submit Encode Job"
         │
         ▼
 ┌───────────────────────────────────────────────────┐
 │  Controller                                        │
 │                                                    │
 │  1. Read run script template from DB               │
 │  2. Read frameserver template from DB (if any)     │
 │  3. Write encode.bat → \\nas\media\source_dir\     │
 │  4. Write frameserver.avs → same UNC dir (if any)  │
 │  5. Create job + generate tasks                    │
 │  6. Each task points to DE_PARAM_SOURCE_DIR        │
 └──────────────────────┬────────────────────────────┘
                        │
                        ▼  agent polls, picks up task
 ┌───────────────────────────────────────────────────┐
 │  Agent                                             │
 │                                                    │
 │  1. Read DE_PARAM_SOURCE_DIR from task params      │
 │  2. Validate encode.bat exists at UNC path         │
 │  3. Execute: cmd.exe /C \\nas\...\encode.bat       │
 │     (with DE_PARAM_* env vars for this chunk)      │
 │  4. Report result back to controller               │
 └───────────────────────────────────────────────────┘
```

This design means the `.bat` and frameserver scripts live on the shared UNC path — all agents execute the same files, and operators can inspect or manually edit them on the share if needed before retrying a failed job.

**Script templates** are managed entirely through the web UI (stored in the `script_templates` database table, not on the filesystem). Two types:

| Type                   | Extension    | Purpose                                                        | Example                                        |
|------------------------|--------------|----------------------------------------------------------------|------------------------------------------------|
| **Run script**         | `.bat`       | The batch script each task executes. Contains encoding commands (ffmpeg, x265, etc.). Written to `encode.bat` in the source's UNC directory on job submit. | H.265 slow CRF encode, AV1 SVT two-pass       |
| **Frameserver script** | `.avs`/`.vpy`| AviSynth or VapourSynth script for frame processing/filtering before encoding. Written alongside `encode.bat` on job submit. | Deinterlace + denoise, crop + resize            |

**UNC directory layout** (per source file, after job submission):

```
\\nas\media\source_dir\
├── source.m2ts                     # original source file (placed by user)
├── vmaf_scan.json                  # VMAF scan results (auto-generated by agent)
├── encode.bat                      # run script (written by controller from template on job submit)
├── frameserver.avs                 # frameserver script (written by controller from template, optional)
└── chunks/                         # encoding output directory (created by encode.bat)
    ├── chunk_0000.mkv
    ├── chunk_0500.mkv
    └── ...
```

**Example run script template** (H.265 encode with optional frameserver):
```bat
@echo off
REM encode.bat — written to UNC path by controller from template
REM Each task receives chunk-specific env vars (DE_PARAM_FRAME_START, etc.)

REM If a frameserver script exists, pipe through it; otherwise encode directly
if exist "%DE_PARAM_SOURCE_DIR%\frameserver.avs" (
    ffmpeg -i "%DE_PARAM_SOURCE_DIR%\frameserver.avs" ^
      -ss %DE_PARAM_FRAME_START% -frames:v %DE_PARAM_CHUNK_SIZE% ^
      -c:v libx265 -preset %DE_PARAM_PRESET% -crf %DE_PARAM_CRF% ^
      "%DE_PARAM_OUTPUT_DIR%\chunk_%DE_PARAM_FRAME_START%.mkv"
) else (
    ffmpeg -i "%DE_PARAM_INPUT_PATH%" ^
      -ss %DE_PARAM_FRAME_START% -frames:v %DE_PARAM_CHUNK_SIZE% ^
      -c:v libx265 -preset %DE_PARAM_PRESET% -crf %DE_PARAM_CRF% ^
      "%DE_PARAM_OUTPUT_DIR%\chunk_%DE_PARAM_FRAME_START%.mkv"
)
echo Encode complete for chunk starting at frame %DE_PARAM_FRAME_START%
```

**Example frameserver script template** (AviSynth deinterlace + crop):
```avs
# frameserver.avs — written to UNC path by controller from template
# Referenced by encode.bat if present

source = LWLibavVideoSource("%DE_PARAM_INPUT_PATH%")
deinterlaced = QTGMC(source, Preset="Slow")
cropped = Crop(deinterlaced, 0, 20, 0, -20)
return cropped
```

**Chunking modes:**

| Mode        | Behavior                                                                      | Example                                     |
|-------------|-------------------------------------------------------------------------------|---------------------------------------------|
| **range**   | Split a numeric range into fixed-size chunks. Each task gets `start` + `end`. | Frames 0–10000 with size 500 → 20 tasks     |
| **single**  | No splitting. The job is one task (useful for short sources).                 | Short clip → 1 task                          |

When a job is submitted, the controller generates tasks from the chunking config:

```
Job "encode source.m2ts" (id=7)
├── Task 7-1:  DE_PARAM_FRAME_START=0     DE_PARAM_FRAME_END=499
├── Task 7-2:  DE_PARAM_FRAME_START=500   DE_PARAM_FRAME_END=999
├── Task 7-3:  DE_PARAM_FRAME_START=1000  DE_PARAM_FRAME_END=1499
│   ...
└── Task 7-20: DE_PARAM_FRAME_START=9500  DE_PARAM_FRAME_END=9999
```

All tasks share the job's base `params` (input_path, output_dir, preset, crf, source_dir). Each task also receives its chunk-specific parameters. Parameters are injected as environment variables prefixed with `DE_PARAM_` — e.g., `DE_PARAM_FRAME_START=500`, `DE_PARAM_CRF=22`.

### 3.3 Dispatcher (Scheduler)

The dispatcher is the central loop that feeds the farm — it matches pending tasks to idle servers, prioritizing higher-priority jobs first, then FIFO within the same priority.

```
               ┌───────────────────────┐
               │  Dispatch Loop        │
               │  (runs every N sec)   │
               └──────────┬────────────┘
                          │
               ┌──────────▼────────────┐
               │ 1. Expand new jobs:   │
               │    generate tasks     │
               │    from chunking      │
               │    config             │
               └──────────┬────────────┘
                          │
               ┌──────────▼────────────┐
               │ 2. Query pending      │
               │    tasks (ordered by  │
               │    job priority, then │
               │    chunk_index)       │
               └──────────┬────────────┘
                          │
               ┌──────────▼────────────┐
               │ 3. Query idle servers │
               │    (matching tags)    │
               └──────────┬────────────┘
                          │
               ┌──────────▼────────────┐
               │ 4. Assign one task    │
               │    per idle server    │
               │    (server → busy)    │
               └──────────┬────────────┘
                          │
               ┌──────────▼────────────┐
               │ 5. Update job state:  │
               │    check if all tasks │
               │    are finished       │
               └──────────────────────┘
```

**Key rules:**
- A server with `state = busy` is never assigned another task.
- Tasks from higher-priority jobs are dispatched first. Within a job, tasks are dispatched in chunk order.
- If a job's `target_tags` are set, only servers with matching tags are eligible.
- If no idle server is available, tasks stay queued — the farm drains work as servers become free.
- When a server finishes a task, it immediately becomes eligible for any pending task in any job — not just the job it was working on. This maximizes farm utilization.

### 3.4 State Store (PostgreSQL)

PostgreSQL is the persistence layer — chosen for concurrent access performance, robust JSON support, and reliability under high-throughput farm workloads with many agents polling simultaneously.

```sql
-- ── Farm servers ──

CREATE TABLE servers (
    name            TEXT PRIMARY KEY,
    host            TEXT NOT NULL,
    tags            JSONB,                  -- e.g. ["encoder", "prod"]
    state           TEXT DEFAULT 'idle' CHECK(state IN ('idle', 'busy', 'offline', 'pending_approval')),
    enabled         BOOLEAN DEFAULT true,
    last_heartbeat  TIMESTAMPTZ,
    registered_at   TIMESTAMPTZ DEFAULT now()
);

-- ── Sources and VMAF ──

CREATE TABLE sources (
    id              SERIAL PRIMARY KEY,
    filename        TEXT NOT NULL,              -- e.g. "source.m2ts"
    unc_path        TEXT NOT NULL UNIQUE,       -- full UNC path to the .m2ts file
    unc_dir         TEXT NOT NULL,              -- parent UNC directory
    state           TEXT DEFAULT 'detected' CHECK(state IN ('detected', 'scanning', 'scanned', 'encoding', 'completed', 'failed')),
    detected_by     TEXT REFERENCES servers(name), -- agent that found the file
    file_size_bytes BIGINT,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE vmaf_results (
    id              SERIAL PRIMARY KEY,
    source_id       INTEGER NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    vmaf_score      REAL,                      -- overall VMAF score (0–100)
    psnr            REAL,                      -- peak signal-to-noise ratio
    ssim            REAL,                      -- structural similarity index
    frame_count     INTEGER,                   -- total frames in source
    resolution      TEXT,                      -- e.g. "1920x1080"
    duration_secs   REAL,                      -- source duration in seconds
    raw_json        JSONB,                     -- full ffmpeg VMAF JSON output
    scanned_at      TIMESTAMPTZ DEFAULT now()
);

-- ── Script templates ──

CREATE TABLE script_templates (
    id              SERIAL PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,       -- display name (e.g. "H.265 Slow CRF")
    type            TEXT NOT NULL CHECK(type IN ('run_script', 'frameserver')),
    extension       TEXT NOT NULL,              -- ".cmd", ".avs", or ".vpy"
    description     TEXT,
    content         TEXT NOT NULL,              -- the full script body
    created_by      TEXT REFERENCES users(username),
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

-- ── Jobs and tasks ──

CREATE TABLE jobs (
    id                  SERIAL PRIMARY KEY,
    source_id           INTEGER REFERENCES sources(id),  -- the .m2ts source this job encodes
    name                TEXT NOT NULL,
    description         TEXT,
    run_script_id       INTEGER NOT NULL REFERENCES script_templates(id),  -- selected run script template
    frameserver_id      INTEGER REFERENCES script_templates(id),           -- optional frameserver template
    params              JSONB,                     -- encoding params (preset, crf, codec, etc.)
    chunking            JSONB NOT NULL,            -- mode, param, start, end, size
    target_tags         JSONB,                     -- e.g. ["encoder"], null = any server
    timeout             TEXT DEFAULT '30m',        -- per-task timeout
    priority            INTEGER DEFAULT 0,         -- higher = dispatched first
    state               TEXT DEFAULT 'pending' CHECK(state IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    submitted_by        TEXT REFERENCES users(username),
    task_count          INTEGER DEFAULT 0,         -- total tasks generated
    created_at          TIMESTAMPTZ DEFAULT now(),
    finished_at         TIMESTAMPTZ
);

CREATE TABLE tasks (
    id          SERIAL PRIMARY KEY,
    job_id      INTEGER NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,           -- 0-based index within the job
    params      JSONB,                      -- chunk-specific params merged with job params
    state       TEXT DEFAULT 'pending' CHECK(state IN ('pending', 'running', 'completed', 'failed', 'timed_out')),
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE assignments (
    id          SERIAL PRIMARY KEY,
    task_id     INTEGER NOT NULL REFERENCES tasks(id),
    server_name TEXT NOT NULL REFERENCES servers(name),
    state       TEXT DEFAULT 'running' CHECK(state IN ('running', 'completed', 'failed', 'timed_out')),
    exit_code   INTEGER,
    stdout      TEXT,
    stderr      TEXT,
    started_at  TIMESTAMPTZ DEFAULT now(),
    finished_at TIMESTAMPTZ
);

-- Enforce one active assignment per server
CREATE UNIQUE INDEX idx_one_active_per_server
    ON assignments(server_name) WHERE state = 'running';

-- ── Webhooks ──

CREATE TABLE webhooks (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    provider    TEXT NOT NULL CHECK(provider IN ('discord', 'slack')),
    url         TEXT NOT NULL,              -- webhook URL (stored encrypted or via .env reference)
    events      JSONB NOT NULL,             -- e.g. ["job.completed", "job.failed", "task.failed"]
    enabled     BOOLEAN DEFAULT true,
    created_at  TIMESTAMPTZ DEFAULT now()
);

-- ── Authentication ──

CREATE TABLE users (
    username      TEXT PRIMARY KEY,
    display_name  TEXT,
    password_hash TEXT,                     -- bcrypt, NULL for OIDC-only users
    role          TEXT DEFAULT 'operator' CHECK(role IN ('admin', 'operator', 'viewer')),
    auth_source   TEXT DEFAULT 'local' CHECK(auth_source IN ('local', 'oidc')),
    oidc_subject  TEXT,                     -- OIDC sub claim, unique per provider
    created_at    TIMESTAMPTZ DEFAULT now(),
    last_login    TIMESTAMPTZ
);

CREATE TABLE sessions (
    token       TEXT PRIMARY KEY,           -- secure random token (stored in cookie)
    username    TEXT NOT NULL REFERENCES users(username) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL
);
```

**Job state machine:**

- A job starts as `pending`. When the controller generates its tasks it moves to `running`.
- When all tasks are `completed`, the job is marked `completed`.
- If any task is `failed`/`timed_out`, the job is marked `failed`. **There are no automatic retries.** An operator must manually re-queue failed tasks via the web UI (retry button on the job detail page) or CLI (`controller job retry <id>`). This prevents wasting farm resources on deterministic failures (e.g., broken scripts, missing source files). When manually retried, only the failed/timed-out tasks are re-queued as `pending` — completed tasks are not re-run.
- A user can cancel a job — all its pending tasks are cancelled and running tasks are left to finish.

### 3.5 Agent

The agent is a separate Go binary (`task-agent`) installed as a **Windows Service** on every target server. It requires no inbound firewall rules — it initiates all connections outbound to the controller.

**Lifecycle:**

```
┌──────────────────────────────────────────────────────────┐
│  Agent (Windows Service on target server)                │
│                                                          │
│  ┌────────────┐     ┌─────────────┐     ┌────────────┐  │
│  │ Poll Loop  │────>│ Download    │────>│ Execute    │  │
│  │ GET /agent │     │ .cmd script │     │ cmd.exe /C │  │
│  │ /poll      │     │ + params    │     │ script.cmd │  │
│  └────────────┘     └─────────────┘     └─────┬──────┘  │
│                                               │         │
│                                         ┌─────▼──────┐  │
│                                         │ Report     │  │
│                                         │ POST /agent│  │
│                                         │ /result    │  │
│                                         └────────────┘  │
└──────────────────────────────────────────────────────────┘
```

**Poll model:**

1. Agent sends `GET /api/agent/poll` to the controller every N seconds, identifying itself by server name + a pre-shared API key.
2. If the controller has an assignment for this server, it responds with the task payload (task type, parameters, timeout). For encoding tasks, the parameters include `DE_PARAM_SOURCE_DIR` pointing to the UNC directory where `encode.bat` has already been written.
3. If no work is available, the controller responds with `204 No Content`.

**Execution model (encoding tasks):**

1. Agent reads `DE_PARAM_SOURCE_DIR` from the task parameters to locate the `encode.bat` file on the UNC share.
2. Agent validates that `encode.bat` exists and is readable at the UNC path.
3. Parameters are set as environment variables (`DE_PARAM_<KEY>=<VALUE>`).
4. Agent runs `cmd.exe /C "<DE_PARAM_SOURCE_DIR>\encode.bat"`, capturing stdout and stderr via pipes. The `.bat` file is **not** copied locally — it is executed directly from the UNC path.
5. A context deadline enforces the task timeout — if exceeded, the agent kills the process tree.
6. On completion (or timeout), agent sends `POST /api/agent/result` with stdout, stderr, and exit code.

**Task parameter validation (agent-side):**

Before executing a task, the agent validates the parameters it received:

1. **Required parameters** — the agent checks that all `DE_PARAM_*` environment variables expected by the script are present and non-empty.
2. **Path validation** — parameters containing paths (UNC or local) are checked for accessibility (`Test-Path` equivalent): the agent verifies the file or directory exists and is reachable before invoking the script.
3. **Timeout sanity** — the agent rejects tasks with a timeout of `0` or negative duration.
4. **Script content** — the agent verifies the `.cmd` script content is non-empty and was received intact.

If validation fails, the agent immediately reports the task as `failed` with a descriptive error in `stderr` (e.g., `"validation: required param DE_PARAM_INPUT_PATH is empty"`) without executing the script. The server returns to `idle` and is available for other work. This catches issues like missing UNC shares, unreachable source files, or misconfigured job parameters before wasting execution time.

Validation rules are defined in the agent binary. The agent validates what it can verify locally (file access, non-empty values, ffmpeg availability) and leaves domain-specific validation to the script itself.

**Agent API endpoints** (served by the controller):

| Method | Endpoint              | Description                                      |
|--------|-----------------------|--------------------------------------------------|
| POST   | `/api/agent/register` | Agent self-registers on first startup            |
| GET    | `/api/agent/poll`     | Agent checks for assigned work (VMAF scans + encode tasks) |
| POST   | `/api/agent/result`   | Agent reports task completion                    |
| POST   | `/api/agent/heartbeat`| Agent sends periodic liveness signal             |
| POST   | `/api/agent/source`   | Agent notifies controller of a new `.m2ts` file found in UNC drop-folder |

**Agent configuration** (`agent-config.yaml` on each target server):

```yaml
server_name: APP-SERVER-01        # must match inventory
controller_url: https://controller.internal:8080
tls_ca: "C:\\distributed-encoder\\ca.crt"   # internal CA root or self-signed controller cert
poll_interval: 5s
heartbeat_interval: 30s
work_dir: "C:\\distributed-encoder\\work"   # temp dir for script execution
log_path: "C:\\distributed-encoder\\agent.log"
# API key is loaded from .env (DE_API_KEY), not stored here.
```

**Agent pseudocode:**

```go
func (a *Agent) Run(ctx context.Context) {
    // Self-register on first startup
    a.register() // POST /api/agent/register {name, host, tags}

    // Start UNC path scanner in background (if enabled)
    if a.ScannerEnabled {
        go a.scanner.Run(ctx) // watches UNC paths for new .m2ts files
    }

    for {
        select {
        case <-ctx.Done():
            return
        case <-time.After(a.PollInterval):
            task, err := a.poll()
            if err != nil || task == nil {
                continue // 204 = no work, or pending_approval
            }

            // Validate task parameters before execution
            if err := a.validate(task); err != nil {
                a.reportResult(task.AssignmentID, "", err.Error(), 1) // fail early
                continue
            }

            var stdout, stderr string
            var exitCode int

            switch task.Type {
            case "vmaf_scan":
                // Built-in VMAF scan routine — no user script involved
                stdout, stderr, exitCode = a.runVMAFScan(ctx, task)

            case "encode":
                // Execute encode.bat directly from the UNC path
                sourceDir := task.Params["SOURCE_DIR"]
                batPath := filepath.Join(sourceDir, "encode.bat")

                if _, err := os.Stat(batPath); err != nil {
                    a.reportResult(task.AssignmentID, "", "encode.bat not found at "+batPath, 1)
                    continue
                }

                env := buildEnv(task.Params) // DE_PARAM_KEY=VALUE
                execCtx, cancel := context.WithTimeout(ctx, task.Timeout)
                stdout, stderr, exitCode = a.exec("cmd.exe", "/C", batPath, env)
                cancel()
                // encode.bat stays on the UNC share — not deleted by the agent
            }

            a.reportResult(task.AssignmentID, stdout, stderr, exitCode)
        }
    }
}
```

**VMAF scan (automatic):** When the controller assigns a VMAF scan task, the agent executes a built-in scan routine (not a user-provided script):

1. Runs `ffmpeg -i <source.m2ts>` to extract metadata: resolution, duration, frame count.
2. Runs `ffmpeg -lavfi libvmaf=model=version=vmaf_v0.6.1:log_fmt=json:log_path=<output>` to generate per-frame VMAF scores, PSNR, and SSIM against a reference (the source itself at native quality serves as baseline for future encoded comparisons).
3. Reports the JSON results back to the controller via `POST /api/agent/result`. The controller parses the output, stores metrics in the `vmaf_results` table, and updates the source state to `scanned`.

Each agent VM must have `ffmpeg` compiled with `libvmaf` installed and available on `PATH`. **VMAF models are bundled with ffmpeg** — the gyan.dev full build includes all standard models (e.g., `vmaf_v0.6.1`), so no separate download or per-job model management is needed. The agent's startup validation verifies that `ffmpeg` is on `PATH` and that `libvmaf` is available (by running `ffmpeg -filters` and checking for `libvmaf`). See [DEPLOYMENT.md](DEPLOYMENT.md) for installation steps.

**Encoding tasks:** For encoding jobs, the task payload contains the source's UNC directory path (`DE_PARAM_SOURCE_DIR`) and chunk-specific parameters — it does **not** contain the script content. The controller has already written `encode.bat` (and optionally a frameserver script) to the UNC directory during job submission (see Section 3.2, Stage 4). The agent:

1. Reads `DE_PARAM_SOURCE_DIR` from the task parameters.
2. Validates that `encode.bat` exists at `<DE_PARAM_SOURCE_DIR>\encode.bat`.
3. Sets all `DE_PARAM_*` environment variables for this chunk (frame start, frame end, preset, CRF, etc.).
4. Executes `cmd.exe /C "<DE_PARAM_SOURCE_DIR>\encode.bat"`, capturing stdout and stderr.
5. Reports the result back to the controller.

Because the `.bat` file lives on the shared UNC path, every agent executes the exact same script. If an operator needs to fix a script error before retrying a failed job, they can edit `encode.bat` directly on the UNC share — no redeployment needed.

### 3.6 Authentication

The web UI and REST API require authentication. Two providers are supported and can be used simultaneously:

**Internal (local) accounts:**

1. Operator creates users via the CLI (`controller user add`) or the admin panel in the web UI.
2. Passwords are hashed with **bcrypt** and stored in the `users` table.
3. On login, the controller validates credentials and issues a session token stored in a secure, HTTP-only cookie.
4. Sessions expire after a configurable TTL (default 24h). The user must log in again.

**OIDC (OpenID Connect):**

1. Controller is registered as a client with an OIDC provider (Azure AD, Keycloak, Google Workspace, etc.).
2. The login page shows a "Sign in with SSO" button alongside the local login form.
3. Clicking it redirects to the provider's authorization endpoint.
4. On callback, the controller exchanges the code for tokens, reads the `sub` and `email` claims, and either matches an existing user or auto-provisions a new one (if `oidc.auto_provision` is enabled in config).
5. A session is created identically to local login.
6. OIDC-only users have a `NULL` password hash and cannot log in with the local form.

**Roles:**

| Role         | Permissions                                                                |
|--------------|----------------------------------------------------------------------------|
| **admin**    | Full access: submit jobs, cancel jobs, manage servers, manage users.       |
| **operator** | Submit jobs, cancel own jobs, view all jobs and servers.                    |
| **viewer**   | Read-only: view dashboard, jobs, tasks, servers. Cannot submit or cancel.  |

**Auth flow diagram:**

```
┌──────────┐          ┌────────────────┐          ┌──────────────┐
│  Browser │──login──>│  Controller    │          │ OIDC Provider│
│          │          │  POST /login   │          │ (Azure AD,   │
│          │          │  or            │──redirect>│  Keycloak)   │
│          │          │  GET /auth/oidc│          │              │
│          │<─cookie──│  set session   │<─callback─│              │
└──────────┘          └────────────────┘          └──────────────┘
```

**Middleware:** Every web UI route and `/api/*` route (except `/api/agent/*` and `/api/health`) passes through the auth middleware. It validates the session cookie (or `Authorization: Bearer <token>` header for API clients). Agent endpoints use their own pre-shared API key auth, independent of user sessions.

### 3.7 CLI Interface

The controller binary exposes a CLI for operators:

```
controller server list                        # show all servers + state (incl. pending_approval)
controller server approve <name>              # approve a dynamically registered agent
controller server enable/disable <name>       # toggle a server

controller source list                        # list all sources with state + VMAF score
controller source status <id>                 # show source detail: VMAF results, linked jobs
controller source rescan <id>                 # re-run VMAF scan for a source

controller template list                      # list all script templates
controller template add <name> --type run_script --file encode.cmd
controller template add <name> --type frameserver --file filter.avs
controller template delete <id>               # remove a script template

controller job list                           # show all jobs with progress
controller job status <id>                    # show job detail: tasks, progress, failures
controller job cancel <id>                    # cancel a job (stops pending tasks)
controller job retry <id>                     # re-queue failed tasks in a job

controller task list --job <id>               # show tasks for a specific job
controller task status <id>                   # show task detail: stdout/stderr, exit code

controller user add <username> --role admin   # create a local user (prompts for password)
controller user list                          # list all users
controller user delete <username>             # remove a user
controller user set-role <username> operator  # change a user's role

controller webhook add <name> --provider discord --url <url> --events job.completed,job.failed
controller webhook list                       # list all webhooks
controller webhook delete <id>                # remove a webhook
controller webhook test <id>                  # send a test message

controller tls generate --cn <hostname>        # generate self-signed TLS cert + key
controller tls generate --cn controller.internal --out /etc/ssl/de/

controller run                                # start the controller (foreground)
controller run --daemon                       # start as background process
```

### 3.8 Web Interface

The controller serves a web UI for operators to manage the system from a browser. Built as a **TypeScript + React** single-page application (SPA) compiled by [Vite](https://vite.dev/). The production build output is embedded into the Go controller binary via `embed.FS`, preserving the single-binary deployment model.

**Pages:**

| Page                     | Path                          | Auth required | Description                                                  |
|--------------------------|-------------------------------|---------------|--------------------------------------------------------------|
| **Login**                | `/login`                      | No            | Local username/password form + "Sign in with SSO" button (OIDC). |
| **Dashboard**            | `/`                           | viewer+       | Farm overview: server count, active jobs, tasks pending/running, recent sources detected. |
| **Sources**              | `/sources`                    | viewer+       | All detected `.m2ts` files with state (detected/scanning/scanned/encoding/completed/failed), VMAF score badge, file size. Click to open source detail. |
| **Source Detail**        | `/sources/{id}`               | viewer+       | VMAF scan results (score, PSNR, SSIM, resolution, duration, frame count). "Configure Encode" button (operator+) opens the encode configuration form. |
| **Encode Configuration** | `/sources/{id}/encode`        | operator+     | Select a run script template, optional frameserver script template, set encoding parameters (preset, CRF, codec), chunking config, priority, timeout. Submit launches the encoding job. |
| **Farm Servers**         | `/servers`                    | viewer+       | Table of all servers with state, current task, last heartbeat, uptime. Enable/disable toggle (admin/operator). |
| **Job List**             | `/jobs`                       | viewer+       | All jobs with progress bars (e.g., "45/100 tasks complete"), state, linked source file, priority. Filterable by state. |
| **Job Detail**           | `/jobs/{id}`                  | viewer+       | Task breakdown table, per-task state, progress bar, cancel/retry buttons (operator+). Click a task to see stdout/stderr. |
| **Task Detail**          | `/jobs/{id}/tasks/{tid}`      | viewer+       | Stdout/stderr output, exit code, server name, timing.         |
| **Script Templates**     | `/admin/templates`            | admin         | Manage run script and frameserver script templates: create, edit, delete. Syntax-highlighted editor for `.cmd`, `.avs`, `.vpy` content. |
| **Users**                | `/admin/users`                | admin         | Manage local and OIDC users: create, delete, change roles.    |
| **Webhooks**             | `/admin/webhooks`             | admin         | Manage Discord/Slack webhook endpoints: add, test, delete.    |

**Dashboard mockup (render-farm style):**

```
┌──────────────────────────────────────────────────────────────┐
│  Distributed Encoder — Farm Dashboard                [user ▼]   │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Servers         Jobs             Tasks          Sources     │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐   ┌────────┐  │
│  │ 12 total │    │  3 active│    │ 47 running│   │ 2 new  │  │
│  │  8 busy  │    │  1 queued│    │112 pending│   │ 1 scan │  │
│  │  3 idle  │    │ 24 done  │    │891 done   │   │ 5 ready│  │
│  │  1 off   │    └──────────┘    └──────────┘   └────────┘  │
│  └──────────┘                                                │
│                                                              │
│  Recent Sources                                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ movie_01.m2ts  1920x1080  VMAF: 94.2  ● scanned      │  │
│  │ movie_02.m2ts  3840x2160  VMAF: —     ◐ scanning      │  │
│  │ clip_03.m2ts   1920x1080  VMAF: 91.8  ● scanned      │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  Active Jobs                                                 │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ #7  movie_01.m2ts  ████████████░░░░░░  14/20  70%  P5 │  │
│  │ #8  clip_03.m2ts   ████░░░░░░░░░░░░░░   3/15  20%  P3 │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  Server Farm                                                 │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ APP-SRV-01  ● busy   job #7 task 7-15   2m elapsed   │  │
│  │ APP-SRV-02  ● busy   job #8 task 8-3    45s elapsed  │  │
│  │ APP-SRV-03  ○ idle   —                               │  │
│  │ APP-SRV-04  ● busy   job #7 task 7-16   1m elapsed   │  │
│  │ APP-SRV-05  ◌ off    last seen 12m ago               │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

**Source Detail + VMAF results:**

```
┌──────────────────────────────────────────────────────────────┐
│  Source: movie_01.m2ts                               [user ▼]│
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  File Info                                                   │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ Path:       \\nas\media\movie_01.m2ts                  │  │
│  │ Size:       24.3 GB                                    │  │
│  │ Resolution: 1920x1080                                  │  │
│  │ Duration:   2h 14m 32s                                 │  │
│  │ Frames:     193,296                                    │  │
│  │ Detected:   2026-02-27 09:15 by APP-SRV-03            │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  VMAF Scan Results                                           │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ VMAF Score:  94.2                                      │  │
│  │ PSNR:        41.3 dB                                   │  │
│  │ SSIM:        0.987                                     │  │
│  │ Scanned:     2026-02-27 09:22                          │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  [ Configure Encode ]                                        │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**Encode Configuration form:**

```
┌──────────────────────────────────────────────────────┐
│  Configure Encode: movie_01.m2ts                     │
│                                                      │
│  Run Script:        [ H.265 Slow CRF       ▼ ]      │
│                     (preview: script content below)  │
│                                                      │
│  Frameserver:       [ Deinterlace + Crop   ▼ ]      │
│                     [ ] None (encode directly)       │
│                                                      │
│  Encoding:                                           │
│    Preset:          [ slow               ]           │
│    CRF:             [ 18                 ]           │
│                                                      │
│  Chunking:                                           │
│    Start frame:     [ 0                  ]           │
│    End frame:       [ 193296             ] (auto)    │
│    Chunk size:      [ 5000               ]           │
│                                                      │
│  Target:            ○ All servers                    │
│                     ○ By tags  [ encoder ▼ ]         │
│  Priority:          [ 5                  ]           │
│  Timeout per task:  [ 2h                 ]           │
│                                                      │
│  [ Submit Encode Job ]                               │
└──────────────────────────────────────────────────────┘
```

**Script Templates management:**

```
┌──────────────────────────────────────────────────────────────┐
│  Script Templates                                    [user ▼]│
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Run Scripts (.cmd)                      [ + New Template ]  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ H.265 Slow CRF          .cmd   "Slow preset, CRF..."  │  │
│  │ H.265 Fast Preview       .cmd   "Fast preview enc..."  │  │
│  │ AV1 SVT Two-Pass        .cmd   "AV1 two-pass SVT..."  │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  Frameserver Scripts (.avs / .vpy)       [ + New Template ]  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ Deinterlace + Crop       .avs   "QTGMC deinterlace..." │  │
│  │ Denoise + Resize 720p   .vpy   "BM3D denoise, res..." │  │
│  │ Passthrough              .avs   "No filtering, dir..." │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**Technology choices:**

- **TypeScript** — all frontend code is written in TypeScript for type safety.
- **React** — component-based UI with React Router for client-side navigation.
- **Vite** — fast dev server with HMR during development; optimized production bundler.
- **Embedded SPA** — `vite build` produces static assets in `web/ui/dist/`. The Go controller embeds this directory via `embed.FS` and serves it at `/`. API calls go to `/api/*` on the same origin (no CORS needed).
- **Auto-refresh** — the dashboard and job list poll the REST API every 5 seconds for updates, giving a live farm-monitor feel.
- **CSS** — minimal stylesheet co-located with components, no CSS framework.

### 3.9 REST API

The HTTP API powers both the web UI and the agent protocol. It also serves as an integration point for external tools.

**Authentication endpoints** (no session required):

| Method | Endpoint                  | Description                          |
|--------|---------------------------|--------------------------------------|
| POST   | `/api/auth/login`         | Local login (username + password → session cookie) |
| POST   | `/api/auth/logout`        | Destroy session                      |
| GET    | `/auth/oidc`              | Start OIDC redirect flow             |
| GET    | `/auth/oidc/callback`     | OIDC provider callback               |

**Source & VMAF endpoints** (session required, role-gated):

| Method | Endpoint                       | Role       | Description                          |
|--------|--------------------------------|------------|--------------------------------------|
| GET    | `/api/sources`                 | viewer+    | List all sources with state and VMAF score |
| GET    | `/api/sources/{id}`            | viewer+    | Get source detail + VMAF results     |
| POST   | `/api/sources/{id}/encode`     | operator+  | Submit an encoding job for a source (run script, frameserver, params, chunking) |
| DELETE | `/api/sources/{id}`            | operator+  | Remove a source record (does not delete the file) |

**Script template endpoints** (session required):

| Method | Endpoint                       | Role       | Description                          |
|--------|--------------------------------|------------|--------------------------------------|
| GET    | `/api/templates`               | viewer+    | List all script templates (filterable by type) |
| GET    | `/api/templates/{id}`          | viewer+    | Get template detail + content        |
| POST   | `/api/templates`               | admin      | Create a new script template         |
| PUT    | `/api/templates/{id}`          | admin      | Update template name, description, or content |
| DELETE | `/api/templates/{id}`          | admin      | Delete a script template             |

**Job & task endpoints** (session required, role-gated):

| Method | Endpoint                  | Role       | Description                          |
|--------|---------------------------|------------|--------------------------------------|
| GET    | `/api/jobs`               | viewer+    | List all jobs with progress summary  |
| GET    | `/api/jobs/{id}`          | viewer+    | Get job detail + task list           |
| DELETE | `/api/jobs/{id}`          | operator+  | Cancel a job                         |
| POST   | `/api/jobs/{id}/retry`    | operator+  | Re-queue failed tasks in a job       |
| GET    | `/api/tasks/{id}`         | viewer+    | Get task detail (stdout/stderr)      |

**Server endpoints** (session required):

| Method | Endpoint                  | Role       | Description                          |
|--------|---------------------------|------------|--------------------------------------|
| GET    | `/api/servers`            | viewer+    | List servers and their states        |
| PUT    | `/api/servers/{name}`     | operator+  | Enable/disable a server              |

**User management endpoints** (admin only):

| Method | Endpoint                  | Description                          |
|--------|---------------------------|--------------------------------------|
| GET    | `/api/users`              | List all users                       |
| POST   | `/api/users`              | Create a local user                  |
| DELETE | `/api/users/{username}`   | Delete a user                        |
| PUT    | `/api/users/{username}/role` | Change a user's role              |

**Agent endpoints** (API key auth, no session):

| Method | Endpoint                  | Description                          |
|--------|---------------------------|--------------------------------------|
| POST   | `/api/agent/register`     | Agent self-registers on first start  |
| GET    | `/api/agent/poll`         | Agent polls for assigned work (VMAF scans + encode tasks) |
| POST   | `/api/agent/result`       | Agent reports task result            |
| POST   | `/api/agent/heartbeat`    | Agent liveness signal                |
| POST   | `/api/agent/source`       | Agent notifies controller of new `.m2ts` file in UNC drop-folder |

**Webhook management endpoints** (admin only):

| Method | Endpoint                    | Description                          |
|--------|-----------------------------|--------------------------------------|
| GET    | `/api/webhooks`             | List all webhooks                    |
| POST   | `/api/webhooks`             | Create a webhook                     |
| DELETE | `/api/webhooks/{id}`        | Delete a webhook                     |
| POST   | `/api/webhooks/{id}/test`   | Send a test notification             |

**Unauthenticated:**

| Method | Endpoint                  | Description                          |
|--------|---------------------------|--------------------------------------|
| GET    | `/api/health`             | Controller health check              |

### 3.10 Webhooks (Discord / Slack)

The controller fires webhook notifications to Discord and/or Slack when significant events occur. Webhooks are configured via the web UI (`/admin/webhooks`) or CLI.

**Supported events:**

| Event               | Trigger                                                    | Payload includes                       |
|---------------------|------------------------------------------------------------|----------------------------------------|
| `job.completed`     | All tasks in a job finished successfully                   | Job name, task count, duration, submitter |
| `job.failed`        | A job has failed tasks and no retries pending              | Job name, failed/total tasks, first error |
| `job.cancelled`     | A user cancelled a job                                     | Job name, cancelled by, tasks remaining |
| `task.failed`       | A single task failed or timed out                          | Job name, task chunk index, server, stderr snippet |
| `server.offline`    | A server stopped heartbeating                              | Server name, last heartbeat time       |
| `server.registered` | A new agent registered (pending approval)                  | Server name, host IP, tags             |
| `source.detected`   | A new `.m2ts` file was detected in a UNC drop-folder       | Filename, UNC path, detected by agent  |
| `source.scanned`    | VMAF scan completed for a source file                      | Filename, VMAF score, resolution       |

**Provider format:**

The controller formats messages natively for each provider:

- **Discord** — POST to the Discord webhook URL with an embed containing the event title, description, color (green/red/yellow), and fields.
- **Slack** — POST to the Slack webhook URL with a Block Kit message containing sections, fields, and color-coded attachments.

**Example Discord embed for `job.completed`:**
```json
{
  "embeds": [{
    "title": "Job Completed: encode-video (#7)",
    "description": "All 20 tasks finished successfully.",
    "color": 3066993,
    "fields": [
      {"name": "Duration", "value": "1h 23m", "inline": true},
      {"name": "Submitted by", "value": "admin", "inline": true}
    ],
    "timestamp": "2026-02-26T14:30:00Z"
  }]
}
```

**Delivery:** Webhooks are fired asynchronously in a background goroutine. Failed deliveries are retried up to 3 times with exponential backoff (1s, 5s, 25s). Delivery failures are logged but do not block task processing.

### 3.11 UNC Path Scanner (Source Detection)

Agents monitor UNC paths for new `.m2ts` files and notify the controller, which triggers the VMAF-first pipeline. This is the entry point for all source files — users simply drop a `.m2ts` file into a watched UNC directory and the system takes over.

**How it works:**

```
┌────────────────────────────────────────────────────────────────┐
│  Agent (UNC Scanner)                                           │
│                                                                │
│  ┌──────────────┐    ┌────────────────┐    ┌───────────────┐  │
│  │ Watch Loop   │───>│ Detect new     │───>│ Notify ctrl   │  │
│  │ (every Ns)   │    │ .m2ts files in │    │ POST /api/    │  │
│  │              │    │ watched dirs   │    │ agent/source  │  │
│  └──────────────┘    └────────────────┘    └───────────────┘  │
│                                                                │
└────────────────────────────────────────────────────────────────┘

                          Controller receives notification
                                      │
                          ┌───────────▼────────────┐
                          │ Create source record   │
                          │ (state = detected)     │
                          └───────────┬────────────┘
                                      │
                          ┌───────────▼────────────┐
                          │ Dispatch VMAF scan     │
                          │ task (auto, single)    │
                          └───────────┬────────────┘
                                      │
                          ┌───────────▼────────────┐
                          │ Agent runs VMAF scan   │
                          │ → results stored in DB │
                          │ → source → scanned     │
                          └───────────┬────────────┘
                                      │
                          ┌───────────▼────────────┐
                          │ User configures encode │
                          │ via web UI             │
                          └────────────────────────┘
```

**Scan logic:**

1. The agent periodically scans configured UNC directories for files matching `*.m2ts`.
2. When a new `.m2ts` file is found, the agent sends `POST /api/agent/source` with the filename, full UNC path, and file size.
3. The controller creates a `source` record (state `detected`) and immediately dispatches a VMAF scan task. The scan task is a single-task internal job — any idle agent can pick it up.
4. The agent tracks which files it has already reported (in-memory set + optional marker file) to avoid duplicate notifications. Files are never moved or renamed by the scanner.
5. Once the VMAF scan completes, the source moves to state `scanned` and appears in the web UI with quality metrics. The user can then configure and submit an encoding job.

**Agent configuration** (`agent-config.yaml`):

```yaml
scanner:
  enabled: false                          # set true to enable UNC path scanning
  watch_paths:                            # list of UNC paths to monitor for .m2ts files
    - "\\\\nas\\media\\incoming"
    - "\\\\nas\\media\\priority"
  scan_interval: 30s                      # how often to scan for new .m2ts files
```

**Security:** The agent reports sources using its own API key, so the controller can attribute detections to the scanner agent. Admins can restrict which agents are allowed to scan via a config flag (`agent.allow_scan: true`).

**Agent API endpoint:**

| Method | Endpoint              | Description                                           |
|--------|-----------------------|-------------------------------------------------------|
| POST   | `/api/agent/source`   | Agent notifies controller of a new `.m2ts` file found in UNC drop-folder |

The payload includes `filename`, `unc_path`, `unc_dir`, and `file_size_bytes`. The controller responds with `201 Created` (new source) or `200 OK` (already known). The `detected_by` field on the source record is set to the reporting agent's server name.

---

## 4. Source, Job & Task Lifecycle

**Source lifecycle:**

```
                     ┌──────────┐
 agent detects ────> │ DETECTED │
 .m2ts file          └────┬─────┘
                          │  controller dispatches VMAF scan task
                     ┌────▼─────┐
                     │ SCANNING │  (VMAF analysis in progress)
                     └────┬─────┘
                          │  scan completes → metrics stored
                     ┌────▼─────┐
                     │ SCANNED  │  (ready for user to configure encode)
                     └────┬─────┘
                          │  user submits encoding job via web UI
                     ┌────▼─────┐
                     │ ENCODING │  (encoding job in progress)
                     └────┬─────┘
                          │
               ┌──────────┼──────────┐
               │                     │
         ┌─────▼─────┐        ┌─────▼────┐
         │ COMPLETED │        │ FAILED   │
         │(all tasks │        │(encode   │
         │ done)     │        │ failed)  │
         └───────────┘        └──────────┘
```

**Job lifecycle:**

```
                     ┌──────────┐
      submit ──────> │ PENDING  │
                     └────┬─────┘
                          │  controller generates tasks from chunking config
                     ┌────▼─────┐
                     │ RUNNING  │  (tasks being dispatched across the farm)
                     └────┬─────┘
                          │
               ┌──────────┼──────────┐
               │          │          │
         ┌─────▼────┐ ┌──▼─────┐ ┌──▼───────┐
         │COMPLETED  │ │ FAILED │ │CANCELLED │
         │(all tasks │ │(some   │ │(user     │
         │ done)     │ │ failed)│ │ stopped) │
         └───────────┘ └────────┘ └──────────┘
```

**Task lifecycle** (each task within a job):

```
                     ┌──────────┐
   job generates ──> │ PENDING  │
                     └────┬─────┘
                          │  dispatcher assigns to an idle server
                     ┌────▼─────┐
                     │ RUNNING  │──── server marked busy
                     └────┬─────┘
                          │  agent executes .cmd
               ┌──────────┼──────────┐
               │          │          │
         ┌─────▼──┐ ┌────▼───┐ ┌───▼──────┐
         │COMPLETED│ │ FAILED │ │TIMED_OUT │
         └────┬────┘ └────┬───┘ └────┬─────┘
              │           │          │
              │           └──────────┘
              │                │
              │         (manual retry only)
              │          operator re-queues
              │          via UI or CLI
              │                │
              │           ┌────▼─────┐
              │           │ PENDING  │  (re-queued)
              │           └──────────┘
              │
              └───── server marked idle
                   (immediately eligible for next task)
```

When a server finishes a task, it returns to the `idle` pool and the dispatcher assigns it the next pending task — from the same job or a different one, whichever has the highest priority. This is the core render-farm behavior: servers continuously drain the queue.

---

## 5. Project Structure

```
distributed-encoder/
├── cmd/
│   ├── controller/
│   │   └── main.go              # controller CLI entrypoint
│   └── agent/
│       └── main.go              # agent CLI entrypoint (installed on targets)
├── internal/
│   ├── config/
│   │   └── config.go            # load inventory.json + app config + .env
│   ├── auth/
│   │   ├── auth.go              # auth middleware, session validation
│   │   ├── local.go             # local login (bcrypt verify, session create)
│   │   ├── oidc.go              # OIDC redirect flow + callback handler
│   │   └── roles.go             # role constants + permission checks
│   ├── dispatcher/
│   │   ├── dispatcher.go        # core dispatch loop + matching logic
│   │   └── chunker.go           # job → task expansion (range, list, single)
│   ├── agent/
│   │   ├── agent.go             # poll loop, heartbeat, execution
│   │   ├── executor.go          # cmd.exe /C wrapper, stdout/stderr capture
│   │   ├── validator.go         # agent-side task parameter validation
│   │   ├── scanner.go           # UNC path scanner for .m2ts source detection
│   │   └── vmaf.go              # built-in VMAF scan task handler
│   ├── model/
│   │   ├── server.go            # Server struct
│   │   ├── source.go            # Source + VMAFResult structs
│   │   ├── template.go          # ScriptTemplate struct
│   │   ├── job.go               # Job struct
│   │   ├── task.go              # Task struct
│   │   ├── assignment.go        # Assignment struct
│   │   └── user.go              # User + Session structs
│   ├── store/
│   │   ├── store.go             # DB interface
│   │   └── postgres.go           # PostgreSQL implementation
│   ├── webhook/
│   │   ├── webhook.go           # event dispatcher, retry logic
│   │   ├── discord.go           # Discord embed formatter
│   │   └── slack.go             # Slack Block Kit formatter
│   └── api/
│       ├── api.go               # REST API router + middleware setup
│       ├── auth_api.go          # login, logout, OIDC endpoints
│       ├── source_api.go        # source list, detail, encode submission
│       ├── template_api.go      # script template CRUD
│       ├── job_api.go           # job list, detail, cancel, retry
│       ├── task_api.go          # task detail endpoint
│       ├── server_api.go        # server list + approve + enable/disable
│       ├── user_api.go          # user management (admin)
│       ├── webhook_api.go       # webhook CRUD + test endpoint
│       ├── agent_api.go         # agent register/poll/result/heartbeat/source endpoints
│       └── web.go               # serves embedded SPA assets, SPA fallback route
├── web/
│   └── ui/                          # TypeScript + React SPA
│       ├── src/
│       │   ├── main.tsx             # React entry point
│       │   ├── App.tsx              # root component, React Router setup
│       │   ├── api/
│       │   │   └── client.ts        # typed fetch wrapper for REST API
│       │   ├── pages/
│       │   │   ├── Login.tsx           # login form + OIDC button
│       │   │   ├── Dashboard.tsx       # farm overview (servers, jobs, sources)
│       │   │   ├── Sources.tsx         # source list with state + VMAF badges
│       │   │   ├── SourceDetail.tsx    # VMAF results + "Configure Encode" button
│       │   │   ├── EncodeConfig.tsx    # template picker, params, chunking, submit
│       │   │   ├── Servers.tsx         # server farm table
│       │   │   ├── Jobs.tsx            # job list with progress bars
│       │   │   ├── JobDetail.tsx       # single job: task table, progress, cancel/retry
│       │   │   ├── TaskDetail.tsx      # single task: stdout/stderr output
│       │   │   ├── ScriptTemplates.tsx # manage run + frameserver script templates
│       │   │   ├── Users.tsx           # user management (admin)
│       │   │   └── Webhooks.tsx        # webhook management (admin)
│       │   ├── components/
│       │   │   └── Layout.tsx       # nav, header, footer, user menu
│       │   └── index.css            # global stylesheet
│       ├── index.html               # Vite HTML entry point
│       ├── tsconfig.json
│       ├── vite.config.ts
│       └── package.json
├── inventory.json               # server inventory (optional seed file)
├── config.yaml                  # controller configuration
├── agent-config.yaml            # example agent configuration
├── .env.example                 # documents all required env vars (committed)
├── .env                         # actual secrets (in .gitignore, never committed)
├── .gitignore
├── go.mod
├── go.sum
├── AGENTS.md                    # contributor and AI agent guidelines
├── ARCHITECTURE.md              # this document
└── DEPLOYMENT.md                # Proxmox VM provisioning and installation steps
```

The project produces **two binaries**:
- `controller` — runs on the management host.
- `agent` — distributed to and installed on each Windows Server target.

---

## 6. Configuration (`config.yaml`)

```yaml
dispatch:
  interval: 10s              # how often the dispatch loop checks for work

database:
  host: localhost
  port: 5432
  name: distributed_encoder
  sslmode: prefer            # disable | prefer | require

server:
  listen: ":8080"            # serves both REST API and web UI
  tls_cert: ""               # path to TLS cert (self-signed or internal CA)
  tls_key: ""                # path to TLS key
  # Not internet-facing — use self-signed or internal CA certs.
  # Generate self-signed: controller tls generate --cn controller.internal --out /etc/ssl/de/

auth:
  session_ttl: 24h           # how long a login session lasts
  oidc:
    enabled: false           # set true to enable OIDC login
    provider_url: ""         # e.g. https://login.microsoftonline.com/{tenant}/v2.0
    redirect_url: ""         # e.g. https://controller.internal:8080/auth/oidc/callback
    auto_provision: true     # auto-create users on first OIDC login
    default_role: operator   # role assigned to auto-provisioned OIDC users

agent:
  heartbeat_timeout: 90s     # mark server offline if no heartbeat within this window
  auto_approve: false        # set true to skip manual approval for new agents
  allow_scan: true           # allow agents to report new .m2ts sources from UNC drop-folders

webhooks:
  retry_attempts: 3          # max retries on delivery failure
  retry_backoff: exponential # exponential (1s, 5s, 25s) or fixed

logging:
  level: info                # debug | info | warn | error
  format: json               # json | text
  output: stdout             # stdout | file path
```

Secrets are loaded from a `.env` file at startup, not from `config.yaml`.

**Controller `.env`**:
```env
# PostgreSQL credentials
DE_DB_USER=distributedencoder
DE_DB_PASS=changeme

# Agent pre-shared API keys (comma-separated for multiple keys)
DE_AGENT_API_KEYS=key-server-group-a,key-server-group-b

# OIDC client credentials (required if oidc.enabled=true)
DE_OIDC_CLIENT_ID=
DE_OIDC_CLIENT_SECRET=

# TLS key passphrase (if applicable)
DE_TLS_KEY_PASSPHRASE=
```

**Agent `.env`** (on each target server, e.g., `C:\distributed-encoder\.env`):
```env
DE_API_KEY=key-server-group-a
```

A `.env.example` is committed to the repo documenting all variables with placeholder values. The actual `.env` files are in `.gitignore`.

---

## 7. Security Considerations

| Area                   | Approach                                                              |
|------------------------|-----------------------------------------------------------------------|
| **TLS**                | The system is not internet-facing — all traffic stays within the Proxmox management VLAN. TLS uses either a **self-signed certificate** or certificates from an **internal CA**. The controller generates/loads TLS certs from paths in `config.yaml` (`tls_cert`, `tls_key`). Agents are configured to trust the internal CA root or the controller's self-signed cert via `tls_ca` in `agent-config.yaml`. For self-signed setups, the controller CLI can generate a cert pair: `controller tls generate --cn controller.internal --out /etc/ssl/de/`. |
| **Agent → Controller** | Agents connect outbound to the controller over HTTPS. No inbound ports required on target servers. Agents verify the controller's TLS certificate against the configured CA bundle (self-signed or internal CA). |
| **Authentication**     | Agents authenticate with a pre-shared API key sent in the `Authorization` header. |
| **Secrets management** | All secrets (DB credentials, API keys, OIDC secrets, TLS passphrases) are stored in a `.env` file, never in code or committed config. A `.env.example` documents required variables. On target servers, the agent API key is stored in a local `.env` file read at service startup. Webhook URLs are stored in the database — treat them as secrets (they contain auth tokens). |
| **Database**           | PostgreSQL access restricted to the controller host. DB user has least-privilege (no superuser). Connections use `sslmode=prefer` or `require` in production. |
| **Web UI access**      | Built-in auth with bcrypt-hashed passwords and session cookies, plus optional OIDC integration. All session cookies are `Secure`, `HttpOnly`, `SameSite=Lax`. |
| **Script integrity**   | Optional: controller validates `.cmd` scripts against a SHA-256 allowlist before dispatch. |
| **Network**            | Controller should be in a management VLAN on the Proxmox network. Agent outbound traffic limited to the controller's IP + port. |
| **Least privilege**    | The agent Windows Service account should have only the permissions required by the scripts. |
| **Audit trail**        | All assignments are logged to the DB with full stdout/stderr capture. Web UI shows read-only results. |

---

## 8. Error Handling & Resilience

| Scenario                         | Behavior                                                        |
|----------------------------------|-----------------------------------------------------------------|
| **Agent stops heartbeating**     | Controller marks server `offline` after `heartbeat_timeout`. Running assignment stays `running` until agent reconnects or is manually failed. |
| **Script timeout**               | Agent kills the process tree locally, reports `timed_out` to controller. Server returns to `idle`. |
| **Script non-zero exit**         | Agent reports `failed` with stderr. Server returns to `idle`. |
| **Controller crash/restart**     | On startup, scan for `running` assignments. Agents will continue polling — when an agent reports a result for a running assignment, it is recorded normally. |
| **Agent crash mid-task**         | The `.cmd` process is orphaned. On agent restart, it detects no active work and resumes polling. The controller marks the assignment `failed` after heartbeat timeout. |
| **Network interruption**         | Agent retries polling with exponential backoff. If it was mid-task, execution continues locally. Result is reported once connectivity is restored. |

---

## 9. Observability

- **Structured logging** (JSON) to stdout — pipe to any log aggregator.
  - Every log entry includes a **correlation ID** (`assignment_id`) linking the log line to a specific task execution. This applies to both controller dispatch logs and agent execution logs.
  - Agent logs include `server_name` on every entry for filtering by host.
  - Failures are logged with full context: server name, task name, assignment ID, exit code, and a stderr snippet.
- **Metrics** exposed via `/api/metrics` (Prometheus format):
  - `dispatcher_tasks_pending` — gauge
  - `dispatcher_tasks_running` — gauge
  - `dispatcher_tasks_completed_total` — counter (label: `status`)
  - `dispatcher_servers_idle` — gauge
  - `dispatcher_servers_busy` — gauge
  - `dispatcher_dispatch_duration_seconds` — histogram
- **Health endpoint** at `/api/health` for uptime monitoring.

---

## 10. Open Decisions

Design choices that are not yet finalized. These should be resolved before or during implementation.

| Decision                              | Options                                                                 | Notes                                   |
|---------------------------------------|-------------------------------------------------------------------------|-----------------------------------------|
| ~~**TLS certificate provisioning**~~  | **Resolved:** self-signed or internal CA. The system is not internet-facing — all communication stays within the Proxmox management VLAN. See Section 7 for details. | |
| ~~**Task parameter validation**~~     | **Resolved:** agent-side. The agent validates task parameters before execution and rejects invalid tasks early. See Section 3.5 for details. | |
| ~~**Failed task retry policy**~~      | **Resolved:** manual retry only. Operators must explicitly re-queue failed tasks via the web UI or CLI (`controller job retry <id>`). No automatic retries. See Section 4 for details. | |
| ~~**VMAF model version management**~~ | **Resolved:** bundled with ffmpeg. The VMAF models ship with the ffmpeg full build installed on each agent (gyan.dev full build includes all models). No separate download or per-job management needed — the automatic VMAF scan uses `vmaf_v0.6.1` by default and ffmpeg resolves it from its bundled data. See Section 3.5 and [DEPLOYMENT.md](DEPLOYMENT.md) Section 4.3. | |
| ~~**PostgreSQL high availability**~~  | **Resolved:** single instance for now. HA is deferred to a later upgrade. See Section 11 (Future Extensions). | |

---

## 11. Future Extensions

These are **not** in scope for the initial build but are natural next steps:

- **Task dependencies** — DAG-based execution (task B waits for task A).
- **Multi-task concurrency per server** — configurable slot count instead of hard 1.
- **Live log streaming** — agent streams stdout/stderr to controller via WebSocket instead of reporting only at completion.
- **Agent auto-update** — controller serves the latest agent binary; agents self-update on poll.
- **Kerberos/AD integration** — auto-discover servers from AD OUs, use domain auth for the web UI.
- **VMAF trend analysis** — aggregate per-frame VMAF scores across multiple encodes of the same source and display quality-vs-bitrate curves in the web UI.
- **Post-encode VMAF comparison** — automatically run VMAF on encoded output vs. original source to verify quality targets were met.
- **PostgreSQL high availability** — streaming replication or managed PostgreSQL (e.g., CloudNativePG on Proxmox) for larger farms requiring database redundancy.
- **ADRs** — adopt Architecture Decision Records for significant design changes.
- **Incident runbook** — lightweight triage guide for common failure modes.

---

## 12. Key Dependencies

**Controller:**

| Dependency                          | Purpose                                    |
|-------------------------------------|--------------------------------------------|
| `github.com/jackc/pgx/v5`         | PostgreSQL driver (pure Go, high performance) |
| `github.com/spf13/cobra`           | CLI framework                              |
| `gopkg.in/yaml.v3`                 | Config file parsing                        |
| `github.com/joho/godotenv`         | Load `.env` files                          |
| `github.com/coreos/go-oidc/v3`    | OIDC token verification and discovery      |
| `golang.org/x/oauth2`             | OAuth2 client for OIDC redirect flow       |
| `golang.org/x/crypto/bcrypt`      | Password hashing for local accounts        |
| `crypto/rand` (stdlib)            | Secure session token generation            |
| `net/http` (stdlib)               | REST API + static file server + webhook delivery |
| `embed` (stdlib)                  | Embed built SPA assets (`web/ui/dist/`) in binary |
| `log/slog` (stdlib, Go 1.21+)    | Structured logging                         |

**Web UI (TypeScript — built at compile time, embedded into controller binary):**

| Dependency                          | Purpose                                    |
|-------------------------------------|--------------------------------------------|
| React                               | Component-based UI                         |
| React Router                        | Client-side SPA routing                    |
| TypeScript                          | Static type checking for all frontend code |
| Vite                                | Dev server (HMR) + production bundler      |

**Agent host requirements** (installed on each Windows Server VM):

| Dependency                          | Purpose                                    |
|-------------------------------------|--------------------------------------------|
| `ffmpeg` with `libvmaf`           | Video encoding + VMAF quality analysis     |
| UNC share mount (SMB)             | Write job output to shared network path    |

**Agent:**

| Dependency                          | Purpose                                    |
|-------------------------------------|--------------------------------------------|
| `net/http` (stdlib)                | HTTP client for polling the controller     |
| `os/exec` (stdlib)                 | Run `cmd.exe /C script.cmd`                |
| `gopkg.in/yaml.v3`                 | Agent config file parsing                  |
| `log/slog` (stdlib, Go 1.21+)     | Structured logging                         |
| `golang.org/x/sys/windows/svc`    | Windows Service integration                |

The agent has minimal dependencies by design — it should be a small, self-contained binary easy to distribute to target servers.

---

## 13. Summary

Distributed Encoder operates like a render farm with a **VMAF-first workflow**, built around two Go binaries — a **controller** and an **agent**. Source `.m2ts` video files are placed in UNC drop-folders; agents detect them and notify the controller, which automatically dispatches a **VMAF quality scan**. Once scanned, users review the quality metrics in the web UI, select **run script** and **frameserver script** templates (managed in the database via the web UI), configure encoding parameters, and submit an encoding job. The controller splits each job into **tasks** based on chunking configuration and distributes them across the farm. Each Windows Server VM runs one task at a time, writes output to a shared UNC path, and immediately picks up the next task when finished.

Agents self-register with the controller on first startup (with optional admin approval), eliminating manual inventory management. Before executing tasks, agents validate parameters locally — checking for required values, accessible paths, and `ffmpeg`/`libvmaf` availability. Failed tasks require **manual retry** by an operator — there are no automatic retries, preventing wasted farm resources on deterministic failures.

All communication between agents and controller uses HTTPS with **self-signed or internal CA** certificates (the system is not internet-facing). The controller stores all state in PostgreSQL — including source records, VMAF results, script templates, jobs, and tasks — and uses Discord/Slack webhooks to notify teams of job completions, failures, and new source detections. Built-in authentication (local accounts with bcrypt + optional OIDC) provides role-based access control. The web UI is a **TypeScript + React** SPA (built with Vite, embedded into the Go binary) that gives operators a real-time farm dashboard with source VMAF results, script template management, job progress bars, per-server status, and full task output. The one-task-per-server invariant is enforced at the database level (unique partial index on active assignments) and at the application level (dispatch loop).
