# Distributed Video Encoder — Architecture Document

## 1. Overview

A distributed video encoding system that offloads encoding tasks from a central **Controller** to one or more **Windows Server Agents**. The Controller orchestrates jobs, tracks state in PostgreSQL, exposes a REST/gRPC API, serves a management web UI, and fires webhooks on success or failure. Each Agent pulls work, generates batch/script files locally, and executes them — including GPU-accelerated encodes — even if the network link to the Controller is temporarily lost.

```
┌──────────────────────────────────────────────────────────────────────┐
│                          USERS / INTEGRATIONS                       │
│   Web UI (SPA)  ·  REST API consumers  ·  Discord/Teams/Slack hooks │
└──────────┬──────────────────────┬──────────────────────┬────────────┘
           │                      │                      │
           ▼                      ▼                      ▼
┌──────────────────────────────────────────────────────────────────────┐
│                       CONTROLLER  (Linux / Container)               │
│                                                                     │
│  ┌────────────┐  ┌────────────┐  ┌──────────┐  ┌───────────────┐   │
│  │  REST API   │  │  gRPC svc  │  │  Web UI  │  │  Webhook Svc  │   │
│  │  (Go HTTP)  │  │ (agent     │  │ (React + │  │  (Discord,    │   │
│  │            │  │  comms)    │  │  Vite)   │  │  Teams,Slack) │   │
│  └─────┬──────┘  └─────┬──────┘  └────┬─────┘  └──────┬────────┘   │
│        │               │              │               │             │
│        ▼               ▼              ▼               ▼             │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                    Core Engine                               │   │
│  │  Job Scheduler · Script Generator · Analysis Coordinator    │   │
│  │  Template Engine · Variable Store · Queue Manager           │   │
│  └──────────────────────┬──────────────────────────────────────┘   │
│                         │                                           │
│                         ▼                                           │
│  ┌──────────────────────────────────┐                               │
│  │  PostgreSQL (primary + optional  │                               │
│  │  Patroni/pgBouncer HA cluster)   │                               │
│  └──────────────────────────────────┘                               │
└──────────────────────────────────────────────────────────────────────┘
           │ gRPC / mTLS
           ▼
┌─────────────────────────────────────────────┐
│  AGENT  (Windows Server)   × N              │
│                                             │
│  ┌───────────┐  ┌──────────┐  ┌──────────┐ │
│  │ Task Exec │  │ Offline  │  │ GPU Mgr  │ │
│  │ (.bat +   │  │ Queue    │  │ (NVENC / │ │
│  │  scripts) │  │          │  │  QSV/AMF)│ │
│  └───────────┘  └──────────┘  └──────────┘ │
│  ┌──────────────────────────────────────┐   │
│  │  Local SQLite journal (offline ops)  │   │
│  └──────────────────────────────────────┘   │
└─────────────────────────────────────────────┘
       │  UNC \\share\path
       ▼
┌──────────────┐
│  NAS / SAN   │
│  (video src) │
└──────────────┘
```

### 1.1 Network Topology

```
                                    ┌─────────────────┐
                                    │   INTERNET /     │
                                    │   CORPORATE LAN  │
                                    └────────┬────────┘
                                             │
                         ┌───────────────────┼───────────────────┐
                         │                   │                   │
                         ▼                   ▼                   ▼
                  ┌────────────┐     ┌──────────────┐    ┌────────────┐
                  │  Browser   │     │  API Client  │    │ Discord /  │
                  │  (Web UI)  │     │  (curl, etc) │    │ Teams /    │
                  │            │     │              │    │ Slack      │
                  └─────┬──────┘     └──────┬───────┘    └─────▲──────┘
                        │                   │                  │
                        │ HTTPS :443        │ HTTPS :443       │ HTTPS :443
                        │                   │                  │ (webhooks out)
  ══════════════════════╪═══════════════════╪══════════════════╪═══════ DMZ / Reverse Proxy
                        │                   │                  │
                        ▼                   ▼                  │
               ┌─────────────────────────────────────────────────────────────┐
               │                CONTROLLER HOST  (Ubuntu / Docker)          │
               │                    10.0.1.10                               │
               │                                                            │
               │   :8080 ─► ┌──────────────┐    ┌──────────────┐ ──► :443  │
               │            │  HTTP Server  │    │ Webhook Svc  │───────────┤
               │            │  (REST + UI)  │    └──────────────┘           │
               │            └──────────────┘                                │
               │   :9443 ─► ┌──────────────┐    ┌──────────────┐           │
               │            │  gRPC Server  │    │  PostgreSQL  │◄── :5432 │
               │            │  (mTLS)       │    │  (container) │           │
               │            └──────────────┘    └──────────────┘           │
               └───────────────────┬────────────────────────────────────────┘
                                   │
                                   │ gRPC/mTLS :9443
                                   │
         ┌─────────────────────────┼─────────────────────────┐
         │                         │                         │
         ▼                         ▼                         ▼
┌──────────────────┐   ┌──────────────────┐   ┌──────────────────┐
│  AGENT-01        │   │  AGENT-02        │   │  AGENT-03        │
│  Windows Server  │   │  Windows Server  │   │  Windows Server  │
│  10.0.2.11       │   │  10.0.2.12       │   │  10.0.2.13       │
│  RTX 4090        │   │  RTX 3080        │   │  Intel Arc A770  │
│                  │   │                  │   │                  │
│  ┌────────────┐  │   │  ┌────────────┐  │   │  ┌────────────┐  │
│  │  Agent Svc │  │   │  │  Agent Svc │  │   │  │  Agent Svc │  │
│  └─────┬──────┘  │   │  └─────┬──────┘  │   │  └─────┬──────┘  │
│        │         │   │        │         │   │        │         │
└────────┼─────────┘   └────────┼─────────┘   └────────┼─────────┘
         │                      │                      │
         │ SMB :445             │ SMB :445             │ SMB :445
         │                      │                      │
         ▼                      ▼                      ▼
┌──────────────────────────────────────────────────────────────────┐
│                     NAS / SAN  (10.0.3.0/24)                    │
│                                                                  │
│   \\NAS01\media\         \\NAS01\encodes\       \\NAS01\temp\   │
│   (source m2ts)          (encoded output)       (work files)    │
│                                                                  │
│   Also mounted on controller via NFS:                           │
│   /mnt/nas/media   /mnt/nas/encodes   /mnt/nas/temp             │
└──────────────────────────────────────────────────────────────────┘
```

### 1.2 Data Flow Overview

```
 SOURCE FILES                    CONTROL PLANE                      OUTPUT
 ════════════                    ═════════════                      ══════

 \\NAS\media\                                                    \\NAS\encodes\
 movie.m2ts ──────┐                                        ┌──── movie.mkv
                  │         ┌────────────────────┐         │
                  │         │     CONTROLLER     │         │
                  │         │                    │         │
                  │    ┌────┤  ┌──────────────┐  │         │
                  │    │    │  │  Job Queue   │  │         │
                  │    │    │  │  (PostgreSQL) │  │         │
                  │    │    │  └──────┬───────┘  │         │
                  │    │    │         │          │         │
                  │    │    │  ┌──────▼───────┐  │         │
                  │    │    │  │  Scheduler   │  │         │
                  │    │    │  └──────┬───────┘  │         │
                  │    │    │         │          │         │
                  │    │    │  ┌──────▼───────┐  │         │
                  │    │    │  │  Script Gen  │  │         │
                  │    │    │  │  (.avs/.vpy  │  │         │
                  │    │    │  │   + .bat)    │  │         │
                  │    │    │  └──────┬───────┘  │         │
                  │    │    │         │          │         │
                  │    │    └─────────┼──────────┘         │
                  │    │              │ gRPC                │
                  │    │              │ TaskAssignment       │
                  │    │              ▼                     │
                  │    │    ┌─────────────────┐            │
                  │    │    │     AGENT       │            │
                  │    │    │                 │            │
                  ├────┼───►│  1. Write .bat  │            │
                  │    │    │     + scripts   │            │
 UNC read ────────┘    │    │                 │            │
                       │    │  2. Execute bat │            │
                       │    │     ┌───────┐   │            │
                       │    │     │ffmpeg │   │────────────┘
                       │    │     │x265   │   │  UNC write
                       │    │     │vspipe │   │
                       │    │     └───┬───┘   │
                       │    │         │       │
                       │    │  3. Report      │
                       │    │     result      │
                       │    └────────┬────────┘
                       │             │ gRPC
                       │             │ TaskResult
                       │             ▼
                       │    ┌─────────────────┐
                       │    │   CONTROLLER    │
                       │    │  Update job DB  │──── Webhook ───► Discord
                       │    └─────────────────┘                  Teams
                       │                                         Slack
                       │
           ┌───────────┘
           │  Analysis results
           │  (histogram, VMAF,
           │   scene boundaries)
           ▼
    ┌──────────────┐
    │  PostgreSQL  │
    │  JSONB store │
    └──────────────┘
```

---

## 2. Technology Choices

| Concern | Choice | Rationale |
|---|---|---|
| **Language** | **Go 1.25+** | Single static binary, trivial cross-compilation (`GOOS=windows`/`linux`), fast compile, low memory, strong concurrency. No Python, no Java. |
| **Agent ↔ Controller** | **gRPC + mTLS** | Bi-directional streaming, protobuf schemas, auto-generated client/server, TLS mutual auth. |
| **REST API** | **`net/http`** (stdlib) | No framework — stdlib router with minimal middleware. Keeps the dependency tree small and avoids churn. |
| **Web UI** | **React 19 + Vite 8** | Compiled to static assets, served by the Go binary itself (embed). No separate Node runtime in production. |
| **Database** | **PostgreSQL 16+** | JSONB for flexible metadata, strong indexing, mature HA tooling. |
| **DB HA** | **Patroni + pgBouncer** (optional) | Automatic leader election, connection pooling, zero-downtime failover. |
| **Migrations** | **golang-migrate** | Plain SQL migration files, embedded in the binary. |
| **Agent offline store** | **SQLite** (via modernc/sqlite) | Pure-Go SQLite driver, no CGO, local journal for offline resilience. |
| **Containerisation** | **Docker / Podman** | Multi-stage build → ~30 MB image. Compose file for Controller + Postgres + pgBouncer. |
| **CLI** | **cobra** | CLI framework for the controller binary (`controller agent`, `controller job`, etc.). |
| **Logging** | **`log/slog`** (stdlib) | Structured JSON logging, zero external dependencies. Go 1.21+. |
| **OIDC** | **`go-oidc/v3`** + **`golang.org/x/oauth2`** | OpenID Connect token verification and redirect flow for SSO. |
| **Password hashing** | **`golang.org/x/crypto/bcrypt`** | bcrypt for local user password storage. |
| **API Spec** | **OpenAPI 3.1** | Machine-readable API definition. Auto-generated from Go handler annotations. Powers Swagger UI, client SDK generation, and contract testing. |
| **API Errors** | **RFC 9457** (Problem Details) | Standard `application/problem+json` error format. Structured, machine-parseable errors with correlation IDs. |
| **CI** | **GitHub Actions** | `go build`, `go test`, `goreleaser` for release artefacts (Linux amd64, Windows amd64). Dependabot for weekly Go module, npm, and GitHub Actions dependency updates (`.github/dependabot.yml`). |

---

## 3. Component Deep-Dive

### 3.1 Controller

The Controller is the single source of truth. It runs on **Ubuntu 22.04+** bare-metal or in a **Docker container**.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           CONTROLLER PROCESS                                │
│                                                                             │
│  ┌─── HTTP Layer ───────────────────────────────────────────────────────┐   │
│  │                                                                      │   │
│  │   :8080                                                              │   │
│  │   ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐  │   │
│  │   │  Auth Middleware  │  │  CORS / CSRF     │  │  Rate Limiter    │  │   │
│  │   └────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘  │   │
│  │            └──────────────┬──────┘──────────────────────┘            │   │
│  │                           ▼                                          │   │
│  │   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │   │
│  │   │ /api/v1/ │  │ /api/v1/ │  │ /api/v1/ │  │ /api/v1/ │           │   │
│  │   │  jobs    │  │  agents  │  │ analysis │  │ webhooks │  ...      │   │
│  │   └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘           │   │
│  │        └──────────────┼─────────────┼─────────────┘                 │   │
│  │                       ▼             ▼                                │   │
│  │              ┌──────────────────────────┐   ┌────────────────────┐  │   │
│  │              │  Static File Server      │   │  WebSocket Hub     │  │   │
│  │              │  (embed.FS — React/Vite) │   │  (live job updates)│  │   │
│  │              └──────────────────────────┘   └────────────────────┘  │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─── gRPC Layer ───────────────────────────────────────────────────────┐   │
│  │                                                                      │   │
│  │   :9443 (mTLS)                                                       │   │
│  │   ┌──────────────────────────────────────────────────────────────┐   │   │
│  │   │                    AgentService                              │   │   │
│  │   │  Register() · Heartbeat() · PollTask() · ReportProgress()   │   │   │
│  │   │  ReportResult() · SyncOfflineResults()                      │   │   │
│  │   └──────────────────────────┬───────────────────────────────────┘   │   │
│  └──────────────────────────────┼───────────────────────────────────────┘   │
│                                 │                                           │
│  ┌─── Core Engine ──────────────┼───────────────────────────────────────┐   │
│  │                              ▼                                       │   │
│  │   ┌──────────────┐   ┌──────────────┐   ┌──────────────────────┐   │   │
│  │   │  Job         │   │  Queue       │   │  Agent Registry      │   │   │
│  │   │  Scheduler   │──►│  Manager     │   │  (online/offline     │   │   │
│  │   │  (priority   │   │  ┌────────┐  │   │   tracking, caps)    │   │   │
│  │   │   FIFO)      │   │  │encode  │  │   └──────────────────────┘   │   │
│  │   └──────┬───────┘   │  │analysis│  │                              │   │
│  │          │           │  │audio   │  │                              │   │
│  │          ▼           │  └────────┘  │                              │   │
│  │   ┌──────────────┐   └──────────────┘                              │   │
│  │   │  Script      │                                                 │   │
│  │   │  Generator   │   ┌──────────────┐   ┌──────────────────────┐   │   │
│  │   │  ┌────────┐  │   │  Analysis    │   │  Webhook Dispatcher  │   │   │
│  │   │  │Template│  │   │  Coordinator │   │  ┌────────────────┐  │   │   │
│  │   │  │Engine  │  │   │  (histogram, │   │  │ Discord Adapter│  │   │   │
│  │   │  └────────┘  │   │   VMAF,      │   │  │ Teams Adapter  │  │   │   │
│  │   │  ┌────────┐  │   │   scene det) │   │  │ Slack Adapter  │  │   │   │
│  │   │  │Variable│  │   └──────────────┘   │  └────────────────┘  │   │   │
│  │   │  │Store   │  │                      │  ┌────────────────┐  │   │   │
│  │   │  └────────┘  │                      │  │ Delivery Queue │  │   │   │
│  │   └──────────────┘                      │  │ + Retry Logic  │  │   │   │
│  │                                         │  └────────────────┘  │   │   │
│  │                                         └──────────────────────┘   │   │
│  └────────────────────────────────────────────────────────────────────┘   │
│                                 │                       ▲               │
│                                 ▼          NFS mount    │               │
│  ┌─── Analysis Runner ─────────────────────────────────────────────┐   │
│  │   (internal/controller/analysis)       /mnt/nas/...             │   │
│  │   ┌─────────────────┐  ┌───────────────────┐  ┌─────────────┐  │   │
│  │   │  HDR Detect     │  │ Scene + Stream    │  │ Audio Enc   │  │   │
│  │   │  (ffprobe JSON) │  │ (ffprobe/ffmpeg)  │  │ (ffmpeg -vn)│  │   │
│  │   └─────────────────┘  └───────────────────┘  └─────────────┘  │   │
│  │   Path Mappings: \\NAS\share → /mnt/nas/share  (DB + config)   │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌─── Data Layer ───────────────────────────────────────────────────────┐ │
│  │   ┌──────────────────────────────────────────────────────────────┐   │ │
│  │   │  PostgreSQL Connection Pool (pgx)                            │   │ │
│  │   │  ┌──────────────────┐  ┌──────────────────────────────────┐ │   │ │
│  │   │  │  sqlc Generated  │  │  golang-migrate                  │ │   │ │
│  │   │  │  Query Layer     │  │  (schema versioning)             │ │   │ │
│  │   │  └──────────────────┘  └──────────────────────────────────┘ │   │ │
│  │   └──────────────────────────────────────────────────────────────┘   │ │
│  └──────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
```

#### 3.1.1 REST API

Prefix: `/api/v1`

**Standard: OpenAPI 3.1**

The REST API follows the [OpenAPI 3.1](https://spec.openapis.org/oas/v3.1.0) specification. An auto-generated OpenAPI spec is served at `GET /api/v1/openapi.json` and a Swagger UI is available at `/api/docs` (admin only, disabled in production by default). The spec is generated from Go struct tags and handler annotations at build time.

**Response Envelope:**

All successful responses use a consistent JSON envelope:

```jsonc
// Single resource
{
  "data": { "id": "abc-123", "type": "job", ... },
  "meta": { "request_id": "req-456" }
}

// Collection (paginated)
{
  "data": [ { "id": "abc-123", "type": "job", ... }, ... ],
  "meta": {
    "request_id": "req-456",
    "total_count": 142,
    "page_size": 50,
    "next_cursor": "eyJpZCI6Ijk5In0="   // opaque, base64-encoded
  }
}
```

- `data` — the primary payload (object or array).
- `meta` — request metadata. Always includes `request_id` (correlation ID). Collections include pagination fields.
- No top-level `error` key on success.

**Error Responses — RFC 9457 (Problem Details):**

All errors use the [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) `application/problem+json` format:

```json
{
  "type": "https://distencoder.dev/errors/validation",
  "title": "Validation Error",
  "status": 422,
  "detail": "Field 'source_path' must be a valid UNC path.",
  "instance": "/api/v1/jobs",
  "request_id": "req-456",
  "errors": [
    { "field": "source_path", "message": "must start with \\\\" }
  ]
}
```

- `type` — a URI identifying the error class. Used by API consumers for programmatic handling.
- `status` — HTTP status code (mirrored in the response header).
- `detail` — human-readable explanation specific to this occurrence.
- `request_id` — same correlation ID as the success envelope, for log tracing.
- `errors` — optional array for field-level validation errors.

**Pagination:**

All list endpoints use **cursor-based pagination** with consistent query parameters:

| Parameter | Default | Description |
|---|---|---|
| `page_size` | `50` | Items per page (max 200) |
| `cursor` | _(none)_ | Opaque cursor from previous response's `meta.next_cursor` |
| `sort` | `created_at` | Sort field (endpoint-specific, documented in OpenAPI spec) |
| `order` | `desc` | Sort direction: `asc` or `desc` |

Cursor pagination avoids `OFFSET` drift on large tables and gives consistent results when new rows are inserted during paging.

**Filtering:**

List endpoints support field-level filters as query parameters:

```
GET /api/v1/jobs?status=running&agent_id=abc-123&sort=priority&order=desc
```

Filter names match the resource field names. Multiple values for the same field use comma separation: `?status=running,queued`.

**Standard HTTP Headers:**

| Header | Direction | Description |
|---|---|---|
| `X-Request-ID` | Response | Correlation ID (matches `meta.request_id`). Generated server-side if not provided by client. |
| `X-Request-ID` | Request | Optional client-provided correlation ID (forwarded to logs). |
| `Content-Type` | Both | `application/json` for requests/responses, `application/problem+json` for errors. |
| `ETag` | Response | Weak ETag on single-resource GETs. Enables `If-None-Match` conditional requests. |
| `X-Rate-Limit-Remaining` | Response | Remaining requests in the current window (when rate limiting is enabled). |
| `X-Total-Count` | Response | Total item count on collection endpoints (mirrors `meta.total_count`). |

**Versioning:**

The API is versioned via URL path (`/api/v1`). Breaking changes increment the version. Non-breaking additions (new fields, new endpoints) are added to the current version.

---

**Endpoints:**

**Authentication endpoints** (no session required):

| Method | Endpoint | Description |
|---|---|---|
| POST | `/auth/login` | Local login (username + password → session cookie) |
| POST | `/auth/logout` | Destroy session |
| GET | `/auth/oidc` | Start OIDC redirect flow |
| GET | `/auth/oidc/callback` | OIDC provider callback |

**Source & analysis endpoints** (session required, role-gated):

| Method | Endpoint | Role | Description |
|---|---|---|---|
| GET | `/sources` | viewer+ | List all sources with state and VMAF score |
| POST | `/sources` | operator+ | Register a new source by UNC path. Idempotent — returns existing source if already registered. **Auto-schedules `analysis` and `hdr_detect` jobs** for genuinely new sources. |
| GET | `/sources/{id}` | viewer+ | Source detail + VMAF results + HDR metadata |
| POST | `/sources/{id}/analyze` | operator+ | Manually trigger an `analysis` job (histogram, VMAF, scene detection) for a source |
| POST | `/sources/{id}/hdr-detect` | operator+ | Manually queue an `hdr_detect` job for a source |
| PATCH | `/sources/{id}/hdr` | operator+ | Set HDR metadata directly (`hdr_type`, `dv_profile`) — used by agents after `hdr_detect` completes |
| POST | `/sources/{id}/encode` | operator+ | Submit an encoding job for a source |
| DELETE | `/sources/{id}` | operator+ | Remove a source record (does not delete the file) |
| POST | `/analysis/scan` | operator+ | Trigger histogram / VMAF scan on a source file |
| GET | `/analysis/{id}` | viewer+ | Retrieve scan results (histogram data, VMAF scores) |

**Job & task endpoints** (session required, role-gated):

| Method | Endpoint | Role | Description |
|---|---|---|---|
| GET | `/jobs` | viewer+ | List / filter / paginate jobs with progress summary |
| POST | `/jobs` | operator+ | Create a new encoding job |
| GET | `/jobs/{id}` | viewer+ | Job detail + task list |
| POST | `/jobs/{id}/cancel` | operator+ | Cancel a running or queued job |
| POST | `/jobs/{id}/retry` | operator+ | Re-queue failed tasks in a job (manual only) |
| GET | `/tasks/{id}` | viewer+ | Task detail (exit code, timing, agent info) |

**Task log endpoints** (session required, role-gated):

| Method | Endpoint | Role | Description |
|---|---|---|---|
| GET | `/tasks/{id}/logs` | viewer+ | Paginated task logs (cursor pagination, filterable by `stream`, `level`). Returns `application/json`. |
| GET | `/tasks/{id}/logs/tail` | viewer+ | Live log tail via **Server-Sent Events** (SSE). Streams new log lines as they arrive. |
| GET | `/tasks/{id}/logs/download` | operator+ | Full log dump as `text/plain` download (`.log` file). |
| GET | `/jobs/{id}/logs` | viewer+ | Aggregated logs across all tasks in a job (interleaved by timestamp). |

**Server & agent endpoints** (session required):

| Method | Endpoint | Role | Description |
|---|---|---|---|
| GET | `/agents` | viewer+ | List registered agents + status |
| GET | `/agents/{id}` | viewer+ | Agent detail (capabilities, GPU info, load) |
| POST | `/agents/{id}/drain` | operator+ | Stop sending new work to an agent |
| PUT | `/servers/{name}` | operator+ | Enable/disable a server |

**Template & variable endpoints:**

| Method | Endpoint | Role | Description |
|---|---|---|---|
| GET | `/templates` | viewer+ | List all script templates (filterable by type) |
| GET | `/templates/{id}` | viewer+ | Template detail + content |
| POST | `/templates` | admin | Create a new script template |
| PUT | `/templates/{id}` | admin | Update template name, description, or content |
| DELETE | `/templates/{id}` | admin | Delete a script template |
| CRUD | `/variables` | admin | Global variables (paths, encoder flags, defaults) |

**Management endpoints:**

| Method | Endpoint | Role | Description |
|---|---|---|---|
| CRUD | `/webhooks` | admin | Manage webhook endpoints (Discord, Teams, Slack) |
| POST | `/webhooks/{id}/test` | admin | Send a test notification |
| POST | `/audio/convert` | operator+ | Queue an audio conversion job (FLAC, Opus, AAC-LC) |
| POST | `/scripts/preview` | operator+ | Render a template with provided context (or sample data) and return the generated script without executing. Used by the template editor live preview and encode config chunk preview. |
| POST | `/scripts/preview-chunk` | operator+ | Render a template for a specific chunk/scene of a source. Accepts `source_id`, `template_id`, `chunk_index`, and chunking config. Returns the rendered .avs/.vpy + .bat pair. |
| GET | `/sources/{id}/scenes` | viewer+ | Get scene detection results for a source (scene boundaries, frame ranges). Used by the encode config scene-based chunking UI. |
| POST | `/sources/{id}/scenes/chunks` | operator+ | Compute chunk boundaries from scene data with min_merge and max_chunk params. Returns the proposed chunk list without creating a job. |
| GET | `/users` | admin | List all users |
| POST | `/users` | admin | Create a local user |
| DELETE | `/users/{username}` | admin | Delete a user |
| PUT | `/users/{username}/role` | admin | Change a user's role |

**Agent endpoints** (API key auth, no session):

| Method | Endpoint | Description |
|---|---|---|
| POST | `/agent/register` | Agent self-registers on first start |
| GET | `/agent/poll` | Agent polls for assigned work |
| POST | `/agent/result` | Agent reports task result |
| POST | `/agent/heartbeat` | Agent liveness signal |
| POST | `/agent/source` | Agent notifies controller of new `.m2ts` file in UNC drop-folder |

**Unauthenticated:**

| Method | Endpoint | Description |
|---|---|---|
| GET | `/health` | Controller health check |

**Authentication:** See [Section 3.1.6 — Authentication & RBAC](#316-authentication--rbac) for full details. Summary:

- **Web UI** — local accounts (bcrypt + session cookies) or OIDC (Azure AD, Keycloak, Google Workspace).
- **REST API** — session cookie or `Authorization: Bearer <token>` for API clients.
- **Agent endpoints** — pre-shared API key (`Authorization` header), independent of user sessions.
- **Roles** — `admin`, `operator`, `viewer` with per-endpoint enforcement.

#### 3.1.2 gRPC Agent Service

```protobuf
service AgentService {
  // Agent calls on startup and periodically
  rpc Register(AgentInfo)        returns (RegisterResponse);
  rpc Heartbeat(HeartbeatReq)    returns (HeartbeatResp);

  // Controller pushes work; agent streams progress
  rpc PollTask(PollTaskReq)      returns (TaskAssignment);
  rpc ReportProgress(stream ProgressUpdate) returns (Ack);
  rpc ReportResult(TaskResult)   returns (Ack);

  // Centralized task logging — agent streams stdout/stderr/executor logs
  rpc StreamLogs(stream LogEntry) returns (Ack);

  // Offline sync — agent sends batched results after reconnect
  rpc SyncOfflineResults(stream TaskResult) returns (SyncResponse);
}

message LogEntry {
  string job_id    = 1;   // UUID of the job this log belongs to
  string stream    = 2;   // "stdout", "stderr", or "agent"
  string level     = 3;   // "debug", "info", "warn", "error"
  string message   = 4;
  google.protobuf.Timestamp timestamp = 5;
  google.protobuf.Struct metadata = 6;  // optional: {"frame":2400,"fps":24.5}
}
```

Transport: **mTLS** on port `9443`. Certificates auto-provisioned on first agent registration (controller acts as internal CA) or provided externally.

**Sequence — Normal Job Lifecycle:**

```
  AGENT                                    CONTROLLER                        POSTGRESQL
    │                                          │                                 │
    │──── Register(AgentInfo) ────────────────►│                                 │
    │                                          │── INSERT agents ───────────────►│
    │◄─── RegisterResponse (agent_id, ok) ─────│◄──────────────────────────────── │
    │                                          │                                 │
    │         ┌─── heartbeat loop ───┐         │                                 │
    │──── Heartbeat(status, metrics) ────────►│                                 │
    │◄─── HeartbeatResp (config updates) ──────│── UPDATE last_heartbeat ───────►│
    │         └──────────────────────┘         │                                 │
    │                                          │                                 │
    │──── PollTask(agent_id, caps) ──────────►│                                 │
    │                                          │── SELECT next queued job ──────►│
    │                                          │◄── job row ─────────────────────│
    │                                          │── UPDATE job status=assigned ──►│
    │◄─── TaskAssignment (scripts, vars) ──────│                                 │
    │                                          │                                 │
    │  ┌─ execute .bat ─┐                      │                                 │
    │  │  parse stdout   │                     │                                 │
    │  └─────────────────┘                     │                                 │
    │                                          │                                 │
    │════ StreamLogs (stream) ═══════════════►│                                 │
    │     {stdout, "frame 1200/48000..."}      │── INSERT task_logs ────────────►│
    │     {stderr, "warning: ..."}             │                                 │
    │     {agent, "gpu_util: 87%"}             │── SSE push to UI log viewer     │
    │     ...continuous while task runs...     │                                 │
    │                                          │                                 │
    │════ ReportProgress (stream) ═══════════►│                                 │
    │     {frame:1200, fps:24.5, eta:...}      │── UPDATE job progress ─────────►│
    │     {frame:2400, fps:25.1, eta:...}      │                                 │
    │     ...every 5s...                       │── WebSocket push to UI          │
    │                                          │                                 │
    │──── ReportResult(success, metrics) ─────►│                                 │
    │                                          │── UPDATE job status=completed ─►│
    │◄─── Ack ─────────────────────────────────│                                 │
    │                                          │── fire webhook ──────────► Discord
    │                                          │                            Teams
    │                                          │                            Slack
```

**Sequence — Offline Resilience:**

```
  AGENT                           CONTROLLER              SQLite (local)
    │                                  │                       │
    │══ ReportProgress (stream) ═════►│                       │
    │   {frame:3600, fps:24.5}         │                       │
    │                                  │                       │
    │        ╳╳╳ NETWORK LOST ╳╳╳     │                       │
    │                                  │                       │
    │  ┌─ encode continues ──┐         │                       │
    │  │  progress buffered   │        │                       │
    │  └──────────────────────┘        │                       │
    │                                  │                       │
    │── progress {frame:4800} ────────►╳ (fail)               │
    │                                  │                       │
    │── journal to local DB ──────────────────────────────────►│
    │   INSERT offline_results                                 │
    │                                  │                       │
    │── progress {frame:6000} ────────►╳ (fail)               │
    │── journal ──────────────────────────────────────────────►│
    │                                  │                       │
    │  ┌─ encode finishes ───┐         │                       │
    │  │  result: success     │        │                       │
    │  └──────────────────────┘        │                       │
    │                                  │                       │
    │── ReportResult ─────────────────►╳ (fail)               │
    │── journal result ───────────────────────────────────────►│
    │                                  │                       │
    │  ┌─ reconnect loop ──────────┐   │                       │
    │  │  5s, 10s, 20s, 40s...5min │   │                       │
    │  └───────────────────────────┘   │                       │
    │                                  │                       │
    │        ═══ NETWORK RESTORED ═══  │                       │
    │                                  │                       │
    │◄────────────────────────────────────── SELECT unsynced ──│
    │                                  │                       │
    │══ SyncOfflineResults (stream) ═►│                       │
    │   result_1 (progress:4800)       │                       │
    │   result_2 (progress:6000)       │                       │
    │   result_3 (completed, metrics)  │                       │
    │                                  │── UPDATE jobs ──► PostgreSQL
    │◄── SyncResponse (all acked) ─────│                       │
    │                                  │                       │
    │──────────────────────────────────────── UPDATE synced=1 ►│
    │                                  │                       │
    │  ┌─ resume normal polling ─┐     │                       │
    │  └─────────────────────────┘     │                       │
```

#### 3.1.3 Core Engine

| Sub-component | Responsibility |
|---|---|
| **Job Scheduler** | Priority queue (FIFO default, priority override). Assigns tasks to agents respecting the **1-task-per-agent** rule. Tracks job states (see diagram and algorithm below). |
| **Script Generator** | Renders `.avs` (AviSynth), `.vpy` (VapourSynth), and `.bat` files from Go templates. Injects global variables, UNC source paths, encoder CLI flags, output paths. |
| **Analysis Coordinator** | Routes `hdr_detect`, `analysis`, and `audio` jobs. When an `AnalysisRunner` is configured, executes them directly on the controller via ffprobe/ffmpeg (NFS path). Otherwise dispatches to a Windows agent. Stores results in `analysis_results` JSONB rows. See §3.1.3a. |
| **Template Engine** | Go `text/template` with custom functions (`uncPath`, `escapeBat`, `gpuFlag`, etc.). Templates stored in DB, editable via web UI. |
| **Variable Store** | Key-value global variables (e.g. `ENCODER_PATH`, `X265_PARAMS`, `OUTPUT_ROOT`). Injected at script-generation time. Exposed as `%VAR_NAME%` in `.bat` and as constants in `.avs`/`.vpy`. |
| **Queue Manager** | Separate logical queues: `encode`, `analysis`, `audio`, `hdr_detect`. Configurable concurrency per queue (default: 1 per agent). |

**Job State Machine:**

```
                          POST /api/v1/jobs
                                │
                                ▼
                        ┌───────────────┐
                ┌──────►│    QUEUED      │◄──────────────────────┐
                │       └───────┬───────┘                       │
                │               │ Scheduler assigns to           │
                │               │ idle agent with matching caps   │
                │               ▼                                │
                │       ┌───────────────┐                       │
                │       │   ASSIGNED    │                       │
                │       └───────┬───────┘                       │
                │               │ Agent receives via              │
                │               │ PollTask()                      │
                │               ▼                                │
                │       ┌───────────────┐                       │
                │       │   RUNNING     │                       │
                │       │               │                       │
                │       │  ┌─progress─┐ │                       │
                │       │  │frame:2400│ │                       │
                │       │  │fps: 24.5 │ │                       │
                │       │  └──────────┘ │                       │
                │       └───┬───────┬───┘                       │
                │           │       │                            │
                │       success   failure                        │
                │           │       │                            │
                │           ▼       ▼                            │
                │   ┌──────────┐ ┌──────────┐                   │
                │   │COMPLETED │ │  FAILED   │───── retry ──────┘
                │   └──────────┘ └──────────┘   POST /jobs/{id}/retry
                │
                │   ┌──────────┐
                └───│CANCELLED │◄──── POST /jobs/{id}/cancel
                    └──────────┘      (from QUEUED, ASSIGNED, or RUNNING)
```

**Dispatch Algorithm:**

The dispatcher is the central loop that feeds the farm. It runs every N seconds (configurable, default 10s):

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
               └───────────────────────┘
```

**Key rules:**
- A server with `state = busy` is never assigned another task.
- Tasks from higher-priority jobs are dispatched first. Within a job, tasks are dispatched in chunk order.
- If a job's `target_tags` are set, only servers with matching tags are eligible.
- If no idle server is available, tasks stay queued — the farm drains work as servers become free.
- When a server finishes a task, it immediately becomes eligible for **any** pending task in **any** job — not just the job it was working on. This maximizes farm utilization.

**Retry Policy — Manual Only:**

There are **no automatic retries**. An operator must manually re-queue failed tasks via the web UI (retry button on the job detail page) or CLI (`controller job retry <id>`). This is a deliberate design decision to prevent wasting farm resources on deterministic failures (e.g., broken scripts, missing source files, encoding errors that will repeat on every attempt).

When manually retried:
- Only **failed/timed-out** tasks are re-queued as `pending` — completed tasks are not re-run.
- A user can **cancel** a job — all its pending tasks are cancelled and running tasks are left to finish.

#### 3.1.3a Controller-Side Analysis & Path Mappings

Analysis jobs (`hdr_detect`, `analysis`, `audio`) can execute directly on the controller host instead of being dispatched to a Windows agent. This requires the NAS to be accessible via NFS/CIFS mounts on the controller and `ffmpeg`/`ffprobe` to be installed.

**Analysis Runner** (`internal/controller/analysis`):

| Method | Job Type | Description |
|---|---|---|
| `RunHDRDetect` | `hdr_detect` | Runs `ffprobe -of json` on the source, parses `color_transfer`, `codec_tag_string`, and `side_data_list` to determine HDR type (`hdr10`, `hdr10+`, `hlg`, `dolby_vision`). Optionally runs `dovi_tool info` to identify the exact Dolby Vision profile number. Updates the `sources` table via `UpdateSourceHDR`. |
| `RunAnalysis` | `analysis` | Runs `ffprobe -show_streams -show_format` (stream metadata) and `ffmpeg select/metadata` filter (scene detection) in sequence. Stores results in `analysis_results` as `stream_info` and `scene` rows. |
| `RunAudio` | `audio` | Extracts and encodes audio with `ffmpeg -vn`. Target format (`flac`, `opus`, `aac`) comes from `ExtraVars["AUDIO_FORMAT"]`, defaulting to FLAC. Output is written to `OutputRoot` (or next to the source if unset). |

When the `AnalysisRunner` is configured on the engine (`Engine.SetAnalysisRunner`), `expandJob` routes `hdr_detect`/`analysis`/`audio` jobs to the controller runner instead of dispatching a task to a Windows agent. The engine sets the job status to `running` synchronously, then executes the work in a goroutine — marking `completed` or `failed` when done.

**Concurrency:** A semaphore (`chan struct{}`) limits simultaneous controller-side analysis jobs (configured via `analysis.concurrency`, default 2).

**Fallback:** If no `AnalysisRunner` is set (e.g. controller has no NFS access or no ffmpeg), the engine falls back to the existing agent-dispatch path.

**Crash recovery note:** If the controller restarts while an analysis job is in `running` state, the job will be stuck. An operator can manually re-queue via `POST /jobs/{id}/retry`.

---

**Path Mappings** (`path_mappings` table, `internal/controller/api/pathmappings.go`):

Path mappings translate Windows UNC paths (used by agents) to Linux POSIX paths (accessible on the controller via NFS). This lets the analysis runner find source files even when they were registered via UNC.

```
\\NAS01\media\movies\foo.m2ts  →  /mnt/nas/media/movies/foo.m2ts
```

**Translation rules:**
- UNC path matching is **case-insensitive** (Windows convention).
- The matched Windows prefix is stripped and replaced with the Linux prefix.
- Backslashes are converted to forward slashes in the remainder.
- Only the first matching enabled mapping is applied.
- Paths that match no mapping are returned unchanged.

**DB table:**

```sql
path_mappings (
    id             UUID PRIMARY KEY,
    name           TEXT NOT NULL,          -- human-readable label
    windows_prefix TEXT NOT NULL,          -- e.g. \\NAS01\media
    linux_prefix   TEXT NOT NULL,          -- e.g. /mnt/nas/media
    enabled        BOOLEAN NOT NULL DEFAULT true,
    created_at     TIMESTAMPTZ,
    updated_at     TIMESTAMPTZ
)
```

**Bootstrap:** On startup, `bootstrapPathMappings` reads `analysis.path_mappings` from `controller.yaml` and inserts any missing entries by name (idempotent — existing names are skipped to preserve operator changes made via UI/API).

**REST API endpoints:**

| Method | Endpoint | Role | Description |
|---|---|---|---|
| GET | `/api/v1/path-mappings` | viewer+ | List all path mappings |
| POST | `/api/v1/path-mappings` | admin | Create a new mapping |
| GET | `/api/v1/path-mappings/{id}` | viewer+ | Get a single mapping |
| PUT | `/api/v1/path-mappings/{id}` | admin | Update name, prefixes, or enabled flag |
| DELETE | `/api/v1/path-mappings/{id}` | admin | Delete a mapping |

**Config (`controller.yaml`):**

```yaml
analysis:
  ffmpeg_bin:    ""              # leave empty to auto-detect from PATH
  ffprobe_bin:   ""
  dovi_tool_bin: ""              # optional; enables exact DV profile detection
  concurrency:   2

  path_mappings:                 # seeded to DB on first startup only
    - name:    "NAS media"
      windows: "\\\\NAS01\\media"
      linux:   "/mnt/nas/media"
```

---

#### 3.1.4 Webhook Service

Fires HTTP POST on job lifecycle events:

| Event | Payload |
|---|---|
| `job.completed` | Job ID, source file, output file, duration, size, VMAF score |
| `job.failed` | Job ID, source file, error message, agent name |
| `job.cancelled` | Job ID, cancelled by, tasks remaining |
| `task.failed` | Job ID, task chunk index, server name, stderr snippet |
| `agent.online` | Agent ID, hostname, GPU info |
| `agent.offline` | Agent ID, hostname, last seen |
| `agent.registered` | Agent name, host IP, tags (pending approval) |
| `source.detected` | Filename, UNC path, detected by agent |
| `source.scanned` | Filename, VMAF score, resolution |

**Adapters** format the payload for each target:

- **Discord** — Embed-rich JSON (`embeds[]` array, colour-coded by status).
- **Microsoft Teams** — Adaptive Card JSON.
- **Slack** — Block Kit JSON.

Each webhook endpoint is configurable per-event (e.g. send failures to `#alerts`, completions to `#encoding-log`).

**Webhook Delivery Pipeline:**

```
  Job completes/fails
         │
         ▼
  ┌──────────────────┐
  │  Event Emitter   │  Publishes: job.completed, job.failed,
  │                  │  job.cancelled, task.failed, agent.*,
  │                  │  source.detected, source.scanned
  └────────┬─────────┘
           │
           ▼
  ┌──────────────────┐     ┌─────────────────────────────────────────┐
  │  Event Router    │────►│  SELECT * FROM webhooks                 │
  │                  │     │  WHERE events @> '{job.completed}'      │
  │  Match event to  │     │  AND enabled = true                     │
  │  subscribed hooks│     └─────────────────────────────────────────┘
  └────────┬─────────┘
           │
           │  For each matched webhook:
           ▼
  ┌──────────────────┐
  │  Adapter Layer   │
  │                  │
  │  ┌─────────┐    │     ┌──────────────────────────────────────┐
  │  │ Discord │────┼────►│ { "embeds": [{ "title": "Encode     │
  │  └─────────┘    │     │   Complete", "color": 3066993, ...}] │
  │  ┌─────────┐    │     ├──────────────────────────────────────┤
  │  │  Teams  │────┼────►│ { "type": "AdaptiveCard",           │
  │  └─────────┘    │     │   "body": [{ "type": "TextBlock",   │
  │  ┌─────────┐    │     │   "text": "Encode Complete" }] }    │
  │  │  Slack  │────┼────►├──────────────────────────────────────┤
  │  └─────────┘    │     │ { "blocks": [{ "type": "section",   │
  │                  │     │   "text": { "type": "mrkdwn",       │
  └────────┬─────────┘     │   "text": "*Encode Complete*" }}] } │
           │               └──────────────────────────────────────┘
           ▼
  ┌──────────────────┐
  │  Delivery Queue  │  Async, buffered channel
  └────────┬─────────┘
           │
           ▼
  ┌──────────────────┐         ┌──────────────────────┐
  │  HTTP Sender     │         │  HMAC-SHA256 Signer  │
  │                  │◄────────│  X-Signature header  │
  │  POST to URL     │         └──────────────────────┘
  │  Timeout: 10s    │
  └────────┬─────────┘
           │
       ┌───┴───┐
       ▼       ▼
    2xx      4xx/5xx/timeout
     │         │
     ▼         ▼
  ┌────────┐ ┌──────────────────┐
  │ Log    │ │  Retry w/ backoff │  3 retries: 10s, 30s, 90s
  │ success│ │  then log failure │
  └────────┘ └──────────────────┘
       │         │
       ▼         ▼
  ┌──────────────────────────────┐
  │  INSERT webhook_deliveries   │
  │  (payload, response_code,    │
  │   success, delivered_at)     │
  └──────────────────────────────┘
```

#### 3.1.5 Web UI

Single-page app embedded in the Go binary (`embed.FS`). Built as a **TypeScript + React** SPA compiled by Vite. The production build output is embedded into the controller binary — no separate Node runtime in production. Auto-refreshes dashboard and job list every 5 seconds.

**Pages:**

| Page | Path | Auth | Description |
|---|---|---|---|
| **Login** | `/login` | No | Local username/password form + "Sign in with SSO" button (OIDC). |
| **Dashboard** | `/` | viewer+ | Farm overview: server count, active jobs, tasks pending/running, recent sources. |
| **Sources** | `/sources` | viewer+ | Detected `.m2ts` files with state, VMAF score badge, file size. |
| **Source Detail** | `/sources/{id}` | viewer+ | VMAF scan results (score, PSNR, SSIM, resolution, duration). "Configure Encode" button. |
| **Encode Config** | `/sources/{id}/encode` | operator+ | Select run script + frameserver template, set encoding params, chunking mode (fixed-size or scene-based), priority. Scene-based mode uses scene detection results to generate per-scene .avs/.vpy scripts with correct frame ranges. |
| **Farm Servers** | `/servers` | viewer+ | Server table: state, current task, last heartbeat. Enable/disable toggle. |
| **Job List** | `/jobs` | viewer+ | Jobs with progress bars, state, source file, priority. Filterable by state. |
| **Job Detail** | `/jobs/{id}` | viewer+ | Task breakdown table, per-task state, cancel/retry buttons (operator+). |
| **Task Detail** | `/jobs/{id}/tasks/{tid}` | viewer+ | Centralized log viewer (live tail + historical), exit code, server name, timing. |
| **Script Templates** | `/admin/templates` | admin | Manage run script (.bat) and frameserver (.avs/.vpy) templates. |
| **Template Editor** | `/admin/templates/{id}` | admin | Full-screen code editor with syntax highlighting, variable reference panel, live preview, and validation. |
| **Users** | `/admin/users` | admin | Manage local and OIDC users: create, delete, change roles. |
| **Webhooks** | `/admin/webhooks` | admin | Manage webhook endpoints: add, test, delete. |
| **Global Variables** | `/admin/variables` | admin | CRUD for variables, grouped by category. |
| **Audio Jobs** | `/audio` | operator+ | Queue and monitor audio conversions (FLAC, Opus, AAC-LC). |
| **Settings** | `/admin/settings` | admin | TLS certs, default encoder params, notification preferences. |

**Dashboard mockup:**

```
┌──────────────────────────────────────────────────────────────┐
│  Distributed Encoder — Farm Dashboard                [user ▼]│
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
│  Active Jobs                                                 │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ #7  movie_01.m2ts  ████████████░░░░░░  14/20  70%  P5 │  │
│  │ #8  clip_03.m2ts   ████░░░░░░░░░░░░░░   3/15  20%  P3 │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  Server Farm                                                 │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ ENCODE-01  ● busy   job #7 task 7-15   2m elapsed     │  │
│  │ ENCODE-02  ● busy   job #8 task 8-3    45s elapsed    │  │
│  │ ENCODE-03  ○ idle   —                                 │  │
│  │ ENCODE-04  ● busy   job #7 task 7-16   1m elapsed     │  │
│  │ ENCODE-05  ◌ off    last seen 12m ago                 │  │
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
│  │ Path:       \\NAS01\media\movie_01.m2ts               │  │
│  │ Size:       24.3 GB                                    │  │
│  │ Resolution: 1920x1080                                  │  │
│  │ Duration:   2h 14m 32s                                 │  │
│  │ Frames:     193,296                                    │  │
│  │ Detected:   2026-02-27 09:15 by ENCODE-03              │  │
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
┌──────────────────────────────────────────────────────────┐
│  Configure Encode: movie_01.m2ts                         │
│                                                          │
│  Run Script:        [ H.265 Slow CRF       ▼ ]          │
│                     (preview: script content below)      │
│                                                          │
│  Frameserver:       [ Deinterlace + Crop   ▼ ]          │
│                     [ ] None (encode directly)           │
│                                                          │
│  Encoding:                                               │
│    Preset:          [ slow               ]               │
│    CRF:             [ 18                 ]               │
│                                                          │
│  Chunking:          ○ Fixed-size                         │
│                     ● Scene-based (requires scene scan)  │
│                                                          │
│  ┌─ Scene-Based Chunking ─────────────────────────────┐  │
│  │                                                     │  │
│  │  47 scenes detected · total 193,296 frames          │  │
│  │  Min merge: [ 500   ] frames (merge smaller scenes) │  │
│  │  Max chunk:  [ 15000 ] frames (split larger scenes) │  │
│  │                                                     │  │
│  │  Scene Timeline (click to adjust boundaries):       │  │
│  │  ├─ 1 ──┤── 2 ──┤── 3 ──┤─ 4 ─┤───── 5 ─────┤──►  │  │
│  │  0    1847   4210   6832  8105       15320           │  │
│  │                                                     │  │
│  │  Resulting chunks: 38 tasks                         │  │
│  │  ┌──────────────────────────────────────────────┐   │  │
│  │  │ Chunk 1   scene 1      0 – 1846    1847 frm │   │  │
│  │  │ Chunk 2   scene 2-3    1847 – 6831  4985 frm│   │  │
│  │  │ Chunk 3   scene 4      6832 – 8104  1273 frm│   │  │
│  │  │ Chunk 4   scene 5a     8105 – 15319 7215 frm│   │  │
│  │  │ ...                                          │   │  │
│  │  └──────────────────────────────────────────────┘   │  │
│  │                                                     │  │
│  │  [ Preview Scripts for Chunk 1 ]                    │  │
│  └─────────────────────────────────────────────────────┘  │
│                                                          │
│  Target:            ○ All servers                        │
│                     ○ By tags  [ encoder ▼ ]             │
│  Priority:          [ 5                  ]               │
│  Timeout per task:  [ 2h                 ]               │
│                                                          │
│  [ Submit Encode Job (38 tasks) ]                        │
└──────────────────────────────────────────────────────────┘
```

**Scene-based chunking workflow:**

1. The source must have a completed **scene detection** scan (see §3.3.3). If not, the UI shows a "Run Scene Scan" button.
2. Scene boundaries are loaded from `analysis_results` (type `scene_detect`) and displayed as an interactive timeline.
3. **Min merge** — scenes shorter than this threshold are merged with their neighbour to avoid very small chunks that waste agent startup overhead.
4. **Max chunk** — scenes longer than this threshold are split into sub-chunks at the max size (falls back to fixed-size splitting within the scene).
5. The operator can **click on the timeline** to manually adjust boundaries (merge or split scenes).
6. Clicking **Preview Scripts for Chunk N** renders the selected frameserver template with that chunk's frame range and shows the output (calls `POST /api/v1/scripts/preview`).
7. On submit, the controller generates one `.avs`/`.vpy` + `.bat` pair per chunk, each with the correct `TrimStart`/`TrimEnd` for that scene range, and writes them all to the UNC share.

**Script Templates management:**

```
┌──────────────────────────────────────────────────────────────┐
│  Script Templates                                    [user ▼]│
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Run Scripts (.bat)                        [ + New Template ]│
│  ┌────────────────────────────────────────────────────────┐  │
│  │ H.265 Slow CRF          .bat  "Slow preset, CRF..."   │  │
│  │ H.265 Fast Preview       .bat  "Fast preview enc..."   │  │
│  │ AV1 SVT Two-Pass        .bat  "AV1 two-pass SVT..."   │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
│  Frameserver Scripts (.avs / .vpy)         [ + New Template ]│
│  ┌────────────────────────────────────────────────────────┐  │
│  │ Deinterlace + Crop       .avs  "QTGMC deinterlace..." │  │
│  │ Denoise + Resize 720p   .vpy  "BM3D denoise, res..." │  │
│  │ Passthrough              .avs  "No filtering, dir..." │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

**Template Editor** (`/admin/templates/{id}`):

```
┌──────────────────────────────────────────────────────────────────────────┐
│  Edit Template: Deinterlace + Crop                               [user ▼]│
│  Type: .avs (AviSynth)                    [ Save ] [ Preview ] [ Delete ]│
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌── Code Editor (syntax-highlighted) ──────────────┐  ┌─ Variables ──┐ │
│  │  1│ # {{.TemplateName}} — {{.JobID}}             │  │              │ │
│  │  2│ SetMemoryMax({{.Globals.AVS_MEMORY_MAX       │  │ Template     │ │
│  │  3│   | default "2048"}})                        │  │  .JobID      │ │
│  │  4│                                              │  │  .UNCPath    │ │
│  │  5│ {{range .Globals.AVS_PLUGINS | split ","}}   │  │  .OutputPath │ │
│  │  6│ LoadPlugin("{{.}}")                          │  │  .TemplateName│ │
│  │  7│ {{end}}                                      │  │  .ScriptType │ │
│  │  8│                                              │  │              │ │
│  │  9│ src = LWLibavVideoSource("{{.UNCPath}}")     │  │ Scene/Chunk  │ │
│  │ 10│                                              │  │  .TrimStart  │ │
│  │ 11│ {{if .TrimStart}}                            │  │  .TrimEnd    │ │
│  │ 12│ src = src.Trim({{.TrimStart}}, {{.TrimEnd}}) │  │  .SceneIndex │ │
│  │ 13│ {{end}}                                      │  │  .TotalScenes│ │
│  │ 14│                                              │  │  .ChunkIndex │ │
│  │ 15│ # Filtering                                  │  │              │ │
│  │ 16│ src = src.QTGMC(Preset="Slow")               │  │ Globals      │ │
│  │ 17│ src = src.Crop(0, 22, 0, -22)                │  │  .Globals.*  │ │
│  │ 18│                                              │  │  (from DB)   │ │
│  │ 19│ src                                          │  │              │ │
│  │   │                                              │  │ Functions    │ │
│  │   │                                              │  │  uncPath     │ │
│  │   │                                              │  │  escapeBat   │ │
│  │   │                                              │  │  gpuFlag     │ │
│  │   │                                              │  │  default     │ │
│  │   │                                              │  │  split / join│ │
│  └──────────────────────────────────────────────────┘  └──────────────┘ │
│                                                                          │
│  ┌── Preview (rendered with sample data) ───────────────────────────┐    │
│  │  # Deinterlace + Crop — job-abc-123                              │    │
│  │  SetMemoryMax(2048)                                              │    │
│  │  LoadPlugin("C:\AviSynth\plugins\QTGMC.dll")                    │    │
│  │  LoadPlugin("C:\AviSynth\plugins\LSMASHSource.dll")             │    │
│  │  src = LWLibavVideoSource("\\NAS01\media\movie_01.m2ts")        │    │
│  │  src = src.Trim(0, 4823)                                        │    │
│  │  src = src.QTGMC(Preset="Slow")                                 │    │
│  │  src = src.Crop(0, 22, 0, -22)                                  │    │
│  │  src                                                             │    │
│  │                           ✓ Template valid · 19 lines rendered   │    │
│  └──────────────────────────────────────────────────────────────────┘    │
│                                                                          │
│  Description: [ QTGMC deinterlace + letterbox crop for Blu-ray... ]     │
└──────────────────────────────────────────────────────────────────────────┘
```

The template editor provides:
- **Code editor** with syntax highlighting (CodeMirror) for `.avs`, `.vpy`, and `.bat` files.
- **Variable reference panel** — shows all available template variables (template context, scene/chunk variables, global variables from DB, and custom template functions). Click a variable to insert it at the cursor.
- **Live preview** — renders the template with sample data (a sample source path, default global variables, sample scene range). Updates on save. Uses `POST /api/v1/scripts/preview`.
- **Validation** — checks Go `text/template` syntax and highlights errors inline before saving.

**Task Detail — Centralized Log Viewer:**

```
┌──────────────────────────────────────────────────────────────┐
│  Task #7-15 — ENCODE-01                              [user ▼]│
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Status: ● RUNNING     Job: #7 movie_01.m2ts                │
│  Server: ENCODE-01     Started: 2026-02-28 10:15:03         │
│  Exit Code: —          Elapsed: 12m 34s                      │
│                                                              │
│  ┌── Log Viewer ──────────────────────────────────────────┐  │
│  │  Filter: [All ▼] [stdout] [stderr] [agent]  [Search…] │  │
│  │  Level:  [All ▼] [info] [warn] [error]                 │  │
│  │                                                         │  │
│  │  10:15:03 agent  ✓ Validation passed (4 checks)        │  │
│  │  10:15:03 agent  ► Task started: encode.bat             │  │
│  │  10:15:04 stdout   [encode] Starting encode for job #7  │  │
│  │  10:15:05 stdout   x265 [info]: HEVC encoder v3.5+...  │  │
│  │  10:15:06 stdout   frame 120/48000  0.25%  142fps       │  │
│  │  10:15:07 stdout   frame 264/48000  0.55%  138fps       │  │
│  │  10:15:08 agent    GPU util: 87%  VRAM: 4.2/24 GB      │  │
│  │  10:15:09 stderr   Warning: VBV underflow (-124 bits)   │  │
│  │  ...                                                    │  │
│  │  10:27:35 stdout   frame 47880/48000 99.75%  63fps      │  │
│  │  10:27:37 stdout   encoded 48000 frames in 752.14s      │  │
│  │                                          ○ Live tail ON │  │
│  └─────────────────────────────────────────────────────────┘  │
│                                                              │
│  [⬇ Download .log]  [↻ Refresh]                     242 lines│
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

The log viewer supports:
- **Live tail** — SSE connection (`GET /tasks/{id}/logs/tail`) auto-scrolls as new lines arrive.
- **Stream filtering** — toggle `stdout`, `stderr`, `agent` streams independently.
- **Level filtering** — filter by `info`, `warn`, `error` to surface issues quickly.
- **Text search** — free-text search across all visible log lines.
- **Download** — export full log as a `.log` text file (`GET /tasks/{id}/logs/download`).

#### 3.1.6 Authentication & RBAC

The web UI and REST API require authentication. Two providers are supported and can be used simultaneously:

**Internal (local) accounts:**

1. Operator creates users via the CLI (`controller user add`) or the admin panel in the web UI.
2. Passwords are hashed with **bcrypt** and stored in the `users` table.
3. On login, the controller validates credentials and issues a session token stored in a secure, HTTP-only cookie (`Secure`, `HttpOnly`, `SameSite=Lax`).
4. Sessions expire after a configurable TTL (default 24h). The user must log in again.

**OIDC (OpenID Connect):**

1. Controller is registered as a client with an OIDC provider (Azure AD, Keycloak, Google Workspace, etc.).
2. The login page shows a "Sign in with SSO" button alongside the local login form.
3. Clicking it redirects to the provider's authorization endpoint.
4. On callback, the controller exchanges the code for tokens, reads the `sub` and `email` claims, and either matches an existing user or auto-provisions a new one (if `oidc.auto_provision` is enabled in config).
5. A session is created identically to local login.
6. OIDC-only users have a `NULL` password hash and cannot log in with the local form.

**Roles:**

| Role | Permissions |
|---|---|
| **admin** | Full access: submit jobs, cancel jobs, manage servers, manage users, manage templates, manage webhooks. |
| **operator** | Submit jobs, cancel own jobs, view all jobs and servers. |
| **viewer** | Read-only: view dashboard, jobs, tasks, servers. Cannot submit or cancel. |

**Auth flow:**

```
┌──────────┐          ┌────────────────┐          ┌──────────────┐
│  Browser │──login──►│  Controller    │          │ OIDC Provider│
│          │          │  POST /login   │          │ (Azure AD,   │
│          │          │  or            │──redirect►│  Keycloak)   │
│          │          │  GET /auth/oidc│          │              │
│          │◄─cookie──│  set session   │◄─callback─│              │
└──────────┘          └────────────────┘          └──────────────┘
```

**Middleware:** Every web UI route and `/api/*` route (except `/api/agent/*` and `/api/health`) passes through the auth middleware. It validates the session cookie (or `Authorization: Bearer <token>` header for API clients). Agent endpoints use their own pre-shared API key auth, independent of user sessions.

#### 3.1.7 CLI Interface

The controller binary exposes a CLI (via `cobra`) for operators:

```
controller agent list                         # show all agents + state (incl. pending_approval)
controller agent approve <name>               # approve a dynamically registered agent
controller agent enable/disable <name>        # toggle an agent

controller source list                        # list all sources with state + VMAF score
controller source status <id>                 # show source detail: VMAF results, linked jobs
controller source rescan <id>                 # re-run VMAF scan for a source

controller template list                      # list all script templates
controller template add <name> --type run_script --file encode.bat
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

controller tls generate --cn <hostname>       # generate self-signed TLS cert + key
controller tls generate --cn controller.internal --out /etc/ssl/de/

controller run                                # start the controller (foreground)
controller run --daemon                       # start as background process
```

#### 3.1.8 Centralized Task Logging

All task execution logs (stdout, stderr, and agent-level executor logs) are **centralized on the controller** and stored in PostgreSQL. There is no need to SSH into agents to view logs — everything is visible through the web UI and REST API.

**Architecture:**

```
 AGENT (Windows Server)                         CONTROLLER (Linux)
 ┌────────────────────────────────┐             ┌──────────────────────────────────┐
 │  Task Executor                  │             │  Log Ingestion Service           │
 │  ┌──────────────────┐          │   gRPC      │  ┌────────────────────────────┐  │
 │  │ cmd.exe encode.bat│──stdout──┤  StreamLogs │  │ Receive log stream         │  │
 │  │                   │──stderr──┼────────────►│  │  ├── Batch INSERT (50ms    │  │
 │  └──────────────────┘          │             │  │  │    window or 100 lines)  │  │
 │                                 │             │  │  ├── SSE push to connected  │  │
 │  Agent Executor Logs            │             │  │  │    web UI clients        │  │
 │  ┌──────────────────┐          │             │  │  └── Buffer for offline     │  │
 │  │ slog → log stream │──agent──┤             │  │       agent reconnect       │  │
 │  └──────────────────┘          │             │  └──────────┬─────────────────┘  │
 │                                 │             │             │                     │
 │  Offline Buffer (SQLite)        │             │             ▼                     │
 │  ┌──────────────────┐          │             │  ┌────────────────────────────┐  │
 │  │ journal log lines │──sync──►┤  SyncLogs  │  │ PostgreSQL: task_logs      │  │
 │  │ when disconnected │         │────────────►│  │  (partitioned by month)    │  │
 │  └──────────────────┘          │             │  └──────────┬─────────────────┘  │
 └────────────────────────────────┘             │             │                     │
                                                │             ▼                     │
                                                │  ┌────────────────────────────┐  │
                                                │  │ REST API + SSE             │  │
                                                │  │  GET /tasks/{id}/logs      │  │
                                                │  │  GET /tasks/{id}/logs/tail │  │
                                                │  └────────────────────────────┘  │
                                                │             │                     │
                                                │             ▼                     │
                                                │  ┌────────────────────────────┐  │
                                                │  │ Web UI: Log Viewer         │  │
                                                │  │  Live tail (SSE)           │  │
                                                │  │  Historical search/filter  │  │
                                                │  │  Download as .log file     │  │
                                                │  └────────────────────────────┘  │
                                                └──────────────────────────────────┘
```

**Log Streams:**

| Stream | Source | Description |
|---|---|---|
| `stdout` | `cmd.exe` stdout pipe | Encoder output: frame progress, bitrate, timing. Parsed for progress updates. |
| `stderr` | `cmd.exe` stderr pipe | Encoder warnings, errors, diagnostic output. |
| `agent` | Agent executor (`slog`) | Agent-level events: task started, validation passed, GPU utilisation readings, UNC path resolution, task completed/failed. |

**Agent-Side Behavior:**

1. The agent captures stdout and stderr from the running `.bat` process in real time (line-buffered).
2. Each line is sent to the controller via `StreamLogs` gRPC stream as a `LogEntry`.
3. Agent executor-level events (validation, GPU checks, state transitions) are also streamed as `agent`-level log entries.
4. If the gRPC connection is lost, log lines are buffered in the local SQLite journal (same offline resilience as task results). On reconnect, buffered logs are synced via `SyncOfflineResults` alongside task results.

**Controller-Side Behavior:**

1. Incoming log entries are batched and written to `task_logs` in PostgreSQL (50ms window or 100 lines, whichever comes first).
2. Simultaneously, log entries are pushed to any connected SSE clients watching that task (live tail).
3. A background goroutine runs log retention cleanup on a configurable schedule (default: delete logs older than 30 days).

**Log Retention:**

| Setting | Default | Description |
|---|---|---|
| `logging.task_log_retention` | `30d` | How long task logs are kept in PostgreSQL. |
| `logging.task_log_cleanup_interval` | `6h` | How often the cleanup goroutine runs. |
| `logging.task_log_max_lines_per_job` | `500000` | Safety cap — oldest lines are pruned if exceeded. |

---

### 3.2 Agent

> **Naming convention:** A **server** is a registered Windows Server machine in the inventory. An **agent** is the Go service running on that server. In practice the terms are often used interchangeably, but the database distinguishes them: the `servers` table tracks machines; the agent binary is the software running on each.

The Agent is a single Go binary installed as a **Windows Service** on each Windows Server machine. It is fully autonomous once a task is assigned.

#### 3.2.1 Lifecycle

```
                    ┌──────────────┐
         install    │   STOPPED    │
        ─────────►  │  (service)   │
                    └──────┬───────┘
                           │ service start
                           ▼
                    ┌──────────────┐
                    │  REGISTERING │──── gRPC Register() ───► Controller
                    └──────┬───────┘
                           │ success
                           ▼
                    ┌──────────────┐
              ┌────►│    IDLE      │◄────────────────────────┐
              │     └──────┬───────┘                         │
              │            │ PollTask() returns assignment    │
              │            ▼                                  │
              │     ┌──────────────┐                         │
              │     │   RUNNING    │── streams progress ──►  │
              │     │  (1 task)    │                         │
              │     └──────┬───────┘                         │
              │            │ task completes/fails             │
              │            ▼                                  │
              │     ┌──────────────┐   ReportResult()        │
              └─────│  REPORTING   │─────────────────────────┘
                    └──────────────┘
```

#### 3.2.2 Registration & Approval

Servers register themselves dynamically. When an agent starts for the first time, it sends a registration request to the controller with its hostname, IP, and capabilities. The controller creates a new server record in the database with `status = pending_approval`.

**Registration flow:**

1. Agent starts and calls `Register()` (gRPC) with its hostname, IP, GPU info, and tags.
2. Controller creates the server in `pending_approval` state. The agent receives an acknowledgement.
3. An admin approves the agent via the web UI or CLI (`controller agent approve <name>`).
4. Once approved, the server moves to `idle` and the agent begins receiving work on its next poll.

Auto-approval can be enabled in `config.yaml` (`agent.auto_approve: true`) for trusted networks where any agent should be accepted immediately.

An optional `inventory.json` seed file can pre-register servers at controller startup (useful for initial deployment):

```jsonc
{
  "servers": [
    {
      "name": "ENCODE-01",
      "host": "10.0.2.11",
      "tags": ["encoder", "gpu"],
      "enabled": true
    }
  ]
}
```

#### 3.2.3 Offline Resilience

When the Controller is unreachable:

1. The agent **continues executing its current task** (critical for long GPU encodes).
2. Progress updates and the final result are written to a **local SQLite journal**.
3. A background goroutine retries Controller connectivity with exponential back-off.
4. On reconnect, `SyncOfflineResults()` streams all queued results to the Controller.
5. The agent then resumes normal polling.

The agent **never** stops a running encode due to Controller connectivity loss.

#### 3.2.4 Task Execution Pipeline

```
TaskAssignment received
  │
  ├─► 1. Read DE_PARAM_SOURCE_DIR from task parameters
  │
  ├─► 2. Validate task parameters (see 3.2.5 Pre-Execution Validation)
  │      ├── Required DE_PARAM_* vars present and non-empty
  │      ├── encode.bat exists and is readable at UNC path
  │      └── UNC paths accessible
  │
  ├─► 3. Set DE_PARAM_* environment variables for this chunk
  │      (DE_PARAM_FRAME_START, DE_PARAM_CRF, DE_PARAM_PRESET, etc.)
  │
  ├─► 4. Open gRPC StreamLogs stream to controller (or buffer to SQLite if offline)
  │
  ├─► 5. Execute encode.bat directly from UNC path via cmd.exe
  │      cmd.exe /C "\\NAS\...\encode.bat"
  │      ├── stdout/stderr captured line-by-line in real-time
  │      ├── each line sent to controller via StreamLogs (centralized logging)
  │      ├── progress parsed from stdout (frame count, fps, ETA) → ReportProgress
  │      └── GPU utilisation monitored (nvidia-smi / Intel iGPU metrics) → StreamLogs
  │
  ├─► 6. Validate output (file exists, size > 0, optional quick VMAF spot-check)
  │
  └─► 7. Report result to Controller (or queue locally if offline)
```

#### 3.2.5 Pre-Execution Validation

Before executing any task, the agent validates the parameters it received:

1. **Required parameters** — the agent checks that all `DE_PARAM_*` environment variables expected by the script are present and non-empty.
2. **Path validation** — parameters containing paths (UNC or local) are checked for accessibility: the agent verifies the file or directory exists and is reachable before invoking the script.
3. **Timeout sanity** — the agent rejects tasks with a timeout of `0` or negative duration.
4. **Script content** — the agent verifies the `.bat` script content is non-empty and was received intact.

If validation fails, the agent immediately reports the task as `failed` with a descriptive error (e.g., `"validation: required param DE_PARAM_INPUT_PATH is empty"`) without executing the script. The server returns to `idle` and is available for other work. This catches issues like missing UNC shares, unreachable source files, or misconfigured job parameters before wasting execution time.

#### 3.2.6 GPU Management

The agent auto-detects available GPUs at registration:

| Vendor | Detection | Encoders |
|---|---|---|
| NVIDIA | `nvidia-smi` | NVENC (H.264, H.265, AV1 on Ada+) |
| Intel | DXGI / `intel_gpu_top` | QSV (H.264, H.265, AV1 on Arc+) |
| AMD | `amdgpu` / DXGI | AMF (H.264, H.265, AV1 on RDNA3+) |

GPU info is reported to the Controller so the scheduler can target GPU-capable agents for hardware-accelerated jobs.

#### 3.2.7 One Task at a Time

The agent enforces a strict **single-task concurrency** model:

- A mutex gates task acceptance.
- `PollTask()` is only called when the agent is in `IDLE` state.
- If the Controller accidentally double-assigns, the agent rejects with `BUSY`.

This prevents resource contention on the encoding machine and ensures deterministic GPU utilisation.

---

### 3.3 Video Analysis

**Analysis Pipeline Overview:**

Analysis is **automatically triggered** when a source is registered via `POST /api/v1/sources`. Two jobs are auto-created: an `analysis` job (histogram, VMAF, scene detection) and an `hdr_detect` job. Both can also be re-triggered manually via the web UI or the `POST /sources/{id}/analyze` and `POST /sources/{id}/hdr-detect` endpoints.

```
  Source registered (POST /sources) — OR — operator triggers scan via Web UI / API
         │
         ▼
  ┌──────────────────────────────────────────────────────────────────────┐
  │                         CONTROLLER                                   │
  │                                                                      │
  │   POST /api/v1/analysis/scan                                         │
  │   { "source": "\\\\NAS\\media\\movie.m2ts",                         │
  │     "types": ["histogram", "vmaf", "scene_detect"] }                 │
  │                                                                      │
  │         │                                                            │
  │         ▼                                                            │
  │   ┌──────────────────┐                                               │
  │   │ Analysis         │  Creates separate jobs per analysis type      │
  │   │ Coordinator      │                                               │
  │   └────────┬─────────┘                                               │
  │            │                                                         │
  │      ┌─────┼──────┐                                                  │
  │      ▼     ▼      ▼                                                  │
  │   ┌─────┐┌─────┐┌───────┐                                           │
  │   │Histo││VMAF ││Scene  │   3 jobs queued in "analysis" queue        │
  │   │gram ││     ││Detect │                                            │
  │   └──┬──┘└──┬──┘└───┬───┘                                           │
  │      └──────┼───────┘                                                │
  │             │ Scheduled to available agents                          │
  └─────────────┼────────────────────────────────────────────────────────┘
                │ gRPC TaskAssignment
                ▼
  ┌──────────────────────────────────────────────────────────────────────┐
  │                           AGENT                                      │
  │                                                                      │
  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────┐  │
  │  │  Histogram Job   │  │  VMAF Job        │  │  Scene Detect Job   │  │
  │  │                  │  │                  │  │                     │  │
  │  │  ffprobe         │  │  ffmpeg          │  │  ffmpeg             │  │
  │  │  -show_frames    │  │  -lavfi libvmaf  │  │  -vf scdet=0.3     │  │
  │  │  \\NAS\movie.m2ts│  │  ref + distorted │  │  \\NAS\movie.m2ts  │  │
  │  │       │          │  │       │          │  │       │             │  │
  │  │       ▼          │  │       ▼          │  │       ▼             │  │
  │  │  Parse per-frame │  │  Parse VMAF JSON │  │  Parse scene cuts  │  │
  │  │  luma/chroma     │  │  per-frame       │  │  timestamps +      │  │
  │  │  statistics      │  │  scores          │  │  frame numbers     │  │
  │  └────────┬─────────┘  └────────┬─────────┘  └──────────┬────────┘  │
  │           └──────────────┬──────┘─────────────────────────┘          │
  │                          │                                           │
  └──────────────────────────┼───────────────────────────────────────────┘
                             │ gRPC ReportResult (JSONB payload)
                             ▼
  ┌──────────────────────────────────────────────────────────────────────┐
  │                        POSTGRESQL                                    │
  │                                                                      │
  │   analysis_results                                                   │
  │   ┌──────────────────────────────────────────────────────────────┐   │
  │   │ job_id │ type      │ frame_data (JSONB)    │ summary (JSONB)│   │
  │   ├────────┼───────────┼───────────────────────┼────────────────┤   │
  │   │ abc-1  │ histogram │ [{frame:0,luma:128,   │ {mean:126,     │   │
  │   │        │           │   chroma_u:114,...},   │  std:31,       │   │
  │   │        │           │  {frame:1,...}, ...]   │  min:12,       │   │
  │   │        │           │                       │  max:245}      │   │
  │   ├────────┼───────────┼───────────────────────┼────────────────┤   │
  │   │ abc-2  │ vmaf      │ [{frame:0,vmaf:97.2}, │ {mean:95.4,    │   │
  │   │        │           │  {frame:1,vmaf:96.8}, │  p5:88.2,      │   │
  │   │        │           │  ...]                 │  p50:96.1,     │   │
  │   │        │           │                       │  min:72.3}     │   │
  │   ├────────┼───────────┼───────────────────────┼────────────────┤   │
  │   │ abc-3  │ scene_det │ [{frame:0,score:0.02},│ {scene_count:  │   │
  │   │        │           │  {frame:1847,         │  47,            │   │
  │   │        │           │   score:0.91}, ...]   │  avg_length:   │   │
  │   │        │           │                       │  823 frames}   │   │
  │   └────────┴───────────┴───────────────────────┴────────────────┘   │
  └──────────────────────────────────────────────────────────────────────┘
                             │
                             ▼
  ┌──────────────────────────────────────────────────────────────────────┐
  │                         WEB UI                                       │
  │                                                                      │
  │   Analysis Viewer                                                    │
  │   ┌──────────────────────────────────────────────────────────────┐   │
  │   │                                                              │   │
  │   │   Luma Histogram          VMAF Score           Scene Cuts    │   │
  │   │   ▐▐▐▐▐▐▐▐▐▐             ──────────           │ │  │ │  │   │   │
  │   │    ▐▐▐▐▐▐▐▐▐▐▐           ╱        ╲           │ │  │ │  │   │   │
  │   │   ▐▐▐▐▐▐▐▐▐▐▐▐▐▐        │  96.2   │          │ │  │ │  │   │   │
  │   │  ▐▐▐▐▐▐▐▐▐▐▐▐▐▐▐▐▐      ╲  mean  ╱          │ │  │ │  │   │   │
  │   │   0  64  128  192  255     ──────────          0    frames  N │   │
  │   │                                                              │   │
  │   │   [Export CSV]  [Use for Scene Split]  [Create Encode Job]   │   │
  │   └──────────────────────────────────────────────────────────────┘   │
  └──────────────────────────────────────────────────────────────────────┘
```

#### 3.3.1 Histogram Scanning

Uses `ffmpeg` with the `histogram` filter to extract per-frame luma/chroma histograms:

```bash
ffmpeg -i "\\server\share\file.m2ts" -vf "histogram=display_mode=overlay" \
  -f rawvideo -y /dev/null 2>&1 | parse_histograms
```

Or more practically, the agent runs a custom Go routine that invokes `ffprobe` with `-show_frames` and computes histogram bins from reported frame stats.

Results stored as JSONB arrays in PostgreSQL:

```json
{
  "frame": 1042,
  "luma_mean": 128.4,
  "luma_std": 32.1,
  "chroma_u_mean": 114.2,
  "chroma_v_mean": 131.8,
  "histogram_bins": [0, 12, 45, ...],
  "scene_change_score": 0.87
}
```

#### 3.3.2 VMAF Analysis

Runs Netflix VMAF via `ffmpeg` with `libvmaf` filter:

```bash
ffmpeg -i distorted.mkv -i "\\server\share\reference.m2ts" \
  -lavfi libvmaf=model=version=vmaf_v0.6.1:log_fmt=json:log_path=vmaf.json \
  -f null -
```

Results parsed and stored per-frame. The web UI renders a VMAF timeline chart with min/max/mean/percentiles.

#### 3.3.3 Scene Detection

Uses `ffmpeg`'s `select` filter with `scene` detection or the `scdet` filter:

```bash
ffmpeg -i "\\server\share\file.m2ts" \
  -vf "scdet=threshold=0.3:sc_pass=1" \
  -f null - 2>&1 | parse_scene_changes
```

Scene boundaries are stored and used to:

1. **Display** scene cut points in the web UI timeline.
2. **Auto-generate** frame ranges for segment-based encoding in AviSynth/VapourSynth scripts.
3. **Split** long encodes into parallelisable chunks at scene boundaries.

---

### 3.4 Script Generation

**Script Generation Data Flow:**

```
  ┌─── INPUTS ────────────────────────────────────────────────────────────────┐
  │                                                                           │
  │  ┌──────────────────┐  ┌──────────────────┐  ┌────────────────────────┐  │
  │  │ Template (DB)    │  │ Global Variables │  │ Job Config             │  │
  │  │                  │  │ (DB)             │  │                        │  │
  │  │ encode.avs.tmpl  │  │ AVS_MEMORY_MAX   │  │ source: \\NAS\x.m2ts  │  │
  │  │ encode.vpy.tmpl  │  │ X265_PATH        │  │ output: \\NAS\x.mkv   │  │
  │  │ encode.bat.tmpl  │  │ AVS_PLUGINS      │  │ encoder_params: ...   │  │
  │  │                  │  │ OUTPUT_ROOT      │  │ script_type: avs      │  │
  │  │                  │  │ VSPIPE_PATH      │  │ trim: [0, 48000]     │  │
  │  └────────┬─────────┘  └────────┬─────────┘  └──────────┬─────────────┘  │
  └───────────┼──────────────────────┼───────────────────────┼────────────────┘
              │                      │                       │
              ▼                      ▼                       ▼
  ┌───────────────────────────────────────────────────────────────────────────┐
  │                       SCRIPT GENERATOR                                    │
  │                                                                           │
  │   1. Load template from DB                                                │
  │   2. Merge variable sources (global vars ← job overrides)                 │
  │   3. Build template context (per chunk):                                   │
  │      { .JobID, .UNCPath, .OutputPath, .GlobalVars,                        │
  │        .TrimStart, .TrimEnd, .SceneIndex, .TotalScenes,                   │
  │        .ChunkIndex, .TotalChunks, .FilterChain, .EncoderParams, ... }     │
  │   4. Apply custom template functions:                                     │
  │      ┌─────────────┬────────────────────────────────────────┐             │
  │      │ uncPath     │ Normalize UNC path separators          │             │
  │      │ escapeBat   │ Escape special chars for cmd.exe       │             │
  │      │ gpuFlag     │ Insert GPU-specific encoder flags      │             │
  │      │ default     │ Provide fallback values                │             │
  │      │ split       │ Split comma-separated strings          │             │
  │      │ join        │ Join arrays with delimiter             │             │
  │      │ trimAvs     │ Generate AviSynth Trim(start, end)    │             │
  │      │ trimVpy     │ Generate VapourSynth src[start:end]    │             │
  │      └─────────────┴────────────────────────────────────────┘             │
  │   5. Execute Go text/template                                             │
  │   6. Validate rendered output (syntax check)                              │
  │                                                                           │
  └──────────┬────────────────────────┬─────────────────────┬─────────────────┘
             │                        │                     │
             ▼                        ▼                     ▼
  ┌─── OUTPUTS ───────────────────────────────────────────────────────────────┐
  │                                                                           │
  │  ┌──────────────────┐  ┌──────────────────┐  ┌────────────────────────┐  │
  │  │ encode.avs       │  │ encode.bat       │  │ audio.bat (optional)   │  │
  │  │                  │  │                  │  │                        │  │
  │  │ SetMemoryMax(2048│  │ @echo off        │  │ @echo off              │  │
  │  │ )                │  │ set "JOB_ID=..." │  │ ffmpeg -i "\\NAS\..."  │  │
  │  │ LoadPlugin(...)  │  │ set "SOURCE=..." │  │  -vn -c:a flac        │  │
  │  │ src = LWLibav... │  │ set "X265=..."   │  │  output.flac          │  │
  │  │ src.Trim(0,48000)│  │ avs2pipemod ...  │  │                        │  │
  │  │ src              │  │  | x265 ...      │  │                        │  │
  │  └──────────────────┘  └──────────────────┘  └────────────────────────┘  │
  │                                                                           │
  │  Per chunk — written to UNC share on job submission                        │
  └───────────────────────────────────────────────────────────────────────────┘
             │
             │ Controller writes scripts to UNC share
             ▼
  ┌───────────────────────────────────────────────────────────┐
  │  UNC directory layout (per source, after submit):         │
  │  \\NAS\media\source_dir\                                  │
  │  ├── source.m2ts             (original source file)       │
  │  └── chunks\                                              │
  │      ├── chunk_000\                                       │
  │      │   ├── encode.bat      (run script, frames 0–1846) │
  │      │   └── frameserver.avs (Trim(0, 1846))             │
  │      ├── chunk_001\                                       │
  │      │   ├── encode.bat      (frames 1847–6831)          │
  │      │   └── frameserver.avs (Trim(1847, 6831))          │
  │      └── ...                                              │
  │                                                           │
  │  Agent executes directly from UNC path:                   │
  │  cmd.exe /C "\\NAS\...\chunks\chunk_000\encode.bat"       │
  │  (scripts are NOT copied locally)                         │
  └───────────────────────────────────────────────────────────┘
```

**Script deployment model:** When a user submits an encoding job, the controller:

1. **Reads the selected templates** from the database (run script + optional frameserver script).
2. **Determines chunks** — either fixed-size (frame count) or scene-based (from scene detection results, with min merge / max chunk applied).
3. **Renders one script set per chunk** using the template engine. Each chunk gets its own template context with the correct frame range:
   - `.TrimStart` / `.TrimEnd` — the chunk's frame boundaries
   - `.SceneIndex` — the scene number (scene-based mode only)
   - `.TotalScenes` — total number of scenes in the source
   - `.ChunkIndex` — sequential chunk number (0-based)
   - `.TotalChunks` — total chunks in the job
4. **Writes rendered scripts to the UNC share** in a per-chunk subdirectory:
   ```
   \\NAS\media\source_dir\
   ├── source.m2ts
   └── chunks\
       ├── chunk_000\
       │   ├── encode.bat          (run script for frames 0–1846)
       │   └── frameserver.avs     (frameserver with Trim(0, 1846))
       ├── chunk_001\
       │   ├── encode.bat          (run script for frames 1847–6831)
       │   └── frameserver.avs     (frameserver with Trim(1847, 6831))
       └── chunk_002\
           ├── ...
   ```
5. **Creates one task per chunk** with `DE_PARAM_SOURCE_DIR` pointing to that chunk's UNC subdirectory. Tasks do **not** carry script content in their payload — agents read and execute `encode.bat` directly from the UNC share.

This design means scripts live on the shared UNC path — all agents execute the same files, and operators can inspect or manually edit them on the share before retrying a failed job. For scene-based chunking, each script contains the exact `Trim()` (AviSynth) or slice (VapourSynth) for that scene's frame range.

#### 3.4.1 AviSynth Example (Generated)

```avs
# Auto-generated by DistEncoder — Job: {{.JobID}}
# Source: {{.UNCPath}}
# Template: {{.TemplateName}}
# Chunk: {{.ChunkIndex}}/{{.TotalChunks}} (scene {{.SceneIndex}})

SetMemoryMax({{.GlobalVars.AVS_MEMORY_MAX | default "2048"}})

{{if .GlobalVars.AVS_PLUGINS}}
{{range split .GlobalVars.AVS_PLUGINS ","}}
LoadPlugin("{{.}}")
{{end}}
{{end}}

src = LWLibavVideoSource("{{.UNCPath}}")

{{if .TrimStart}}
src = src.Trim({{.TrimStart}}, {{.TrimEnd}})
{{end}}

# Filtering
{{.FilterChain}}

src
```

#### 3.4.2 VapourSynth Example (Generated)

```python
# Auto-generated by DistEncoder — Job: {{.JobID}}
# Chunk: {{.ChunkIndex}}/{{.TotalChunks}} (scene {{.SceneIndex}})
import vapoursynth as vs
core = vs.core

core.max_cache_size = {{.GlobalVars.VPY_CACHE_SIZE | default "2048"}}

{{range .GlobalVars.VPY_PLUGINS | split ","}}
core.std.LoadPlugin("{{.}}")
{{end}}

src = core.lsmas.LWLibavSource("{{.UNCPath}}")

{{if .TrimStart}}
src = src[{{.TrimStart}}:{{.TrimEnd}}]
{{end}}

# Filtering
{{.FilterChain}}

src.set_output()
```

#### 3.4.3 Batch File Example (Generated)

```bat
@echo off
REM Auto-generated by DistEncoder — Job: {{.JobID}}
REM Agent: {{.AgentHostname}}
REM Source: {{.UNCPath}}

set "JOB_ID={{.JobID}}"
set "SOURCE={{.UNCPath}}"
set "OUTPUT={{.OutputPath}}"
set "WORK_DIR={{.WorkDir}}"

{{range $k, $v := .GlobalVars}}
set "{{$k}}={{$v}}"
{{end}}

cd /d "%WORK_DIR%"

echo [%date% %time%] Starting encode for %JOB_ID%

{{if eq .ScriptType "avs"}}
"%AVS_PIPE_PATH%" "%WORK_DIR%\encode.avs" -y4mp - | "%X265_PATH%" {{.EncoderParams}} --input - --y4m -o "%OUTPUT%" 2>&1
{{else if eq .ScriptType "vpy"}}
"%VSPIPE_PATH%" "%WORK_DIR%\encode.vpy" --y4m - | "%X265_PATH%" {{.EncoderParams}} --input - --y4m -o "%OUTPUT%" 2>&1
{{end}}

if %ERRORLEVEL% NEQ 0 (
    echo [FAILED] Encode failed with exit code %ERRORLEVEL%
    exit /b %ERRORLEVEL%
)

echo [SUCCESS] Encode completed: %OUTPUT%
exit /b 0
```

---

### 3.5 Audio Conversion

Audio jobs are separate tasks dispatched to agents (or run on the Controller itself).

| Target | FFmpeg command |
|---|---|
| **FLAC** | `ffmpeg -i "\\share\file.m2ts" -vn -c:a flac -compression_level 8 output.flac` |
| **Opus** | `ffmpeg -i "\\share\file.m2ts" -vn -c:a libopus -b:a 192k output.opus` |
| **AAC-LC** | `ffmpeg -i "\\share\file.m2ts" -vn -c:a aac -b:a 256k output.m4a` |

Configuration (bitrate, channels, sample rate, stream selection) is set per-job or via global defaults.

---

## 4. Database Schema (PostgreSQL)

### 4.0 Entity Relationship Diagram

```
┌─────────────────────┐          ┌─────────────────────────────────────┐
│     api_keys        │          │              agents                 │
├─────────────────────┤          ├─────────────────────────────────────┤
│ PK  id         UUID │          │ PK  id              UUID            │
│     name       TEXT │          │     hostname        TEXT (UNIQUE)   │
│     key_hash   TEXT │          │     ip_address      INET            │
│     permissions[]   │          │     os_version      TEXT            │
│     created_at      │          │     gpu_info        JSONB           │
│     expires_at      │          │     capabilities    JSONB           │
│     last_used_at    │          │     status          TEXT            │  -- pending_approval, online, offline, busy
└─────────────────────┘          │     last_heartbeat  TIMESTAMPTZ    │
                                 │     registered_at   TIMESTAMPTZ    │
                                 │     config          JSONB           │
                                 └──────────┬──────────────────────────┘
                                            │
                                            │ 1
                                            │
                                            │ ◄── agent_id (FK, nullable)
                                            │
┌─────────────────────┐           ┌─────────┴─────────────────────────┐
│    templates        │           │              jobs                  │
├─────────────────────┤           ├───────────────────────────────────┤
│ PK  id         UUID │ 1      * │ PK  id              UUID          │
│     name       TEXT ├──────────┤     source_path     TEXT          │
│     script_type TEXT│◄─ FK ────│     output_path     TEXT          │
│     category   TEXT │          │     job_type        TEXT          │
│     content    TEXT │          │     status          TEXT          │
│     description TEXT│          │ FK  agent_id        UUID ─────────┤──► agents
│     variables_used[]│          │ FK  template_id     UUID ─────────┤──► templates
│ FK  created_by TEXT │──► users │
│     created_at      │          │
│     updated_at      │          │
└─────────────────────┘          │     script_type     TEXT          │
                                 │     encoder_params  TEXT          │
                                 │     audio_codec     TEXT          │
                                 │     audio_params    JSONB         │
                                 │     variables       JSONB         │
                                 │     progress        JSONB         │
                                 │     result          JSONB         │
                                 │     error_message   TEXT          │
                                 │     priority        INT           │
                                 │     created_at      TIMESTAMPTZ  │
                                 │     started_at      TIMESTAMPTZ  │
                                 │     completed_at    TIMESTAMPTZ  │
                                 └──────────┬─────────────────────────┘
                                            │
                                            │ 1
                                            │
                                            │ *
                                 ┌──────────┴─────────────────────────┐
                                 │       analysis_results             │
                                 ├───────────────────────────────────┤
                                 │ PK  id              UUID          │
                                 │ FK  job_id          UUID ──► jobs │
                                 │     analysis_type   TEXT          │
                                 │     frame_data      JSONB         │
                                 │     summary         JSONB         │
                                 │     created_at      TIMESTAMPTZ  │
                                 └───────────────────────────────────┘

                                            │ *   (jobs also has)
                                            │
                                 ┌──────────┴─────────────────────────┐
                                 │          task_logs                  │
                                 ├───────────────────────────────────┤
                                 │ PK  id              BIGINT (seq)  │
                                 │ FK  job_id          UUID ──► jobs │
                                 │     agent_hostname  TEXT          │
                                 │     timestamp       TIMESTAMPTZ  │
                                 │     stream          TEXT          │  -- stdout, stderr, agent
                                 │     level           TEXT          │  -- info, warn, error, debug
                                 │     message         TEXT          │
                                 │     metadata        JSONB         │  -- frame, fps, gpu_util, etc.
                                 └───────────────────────────────────┘


┌─────────────────────┐          ┌───────────────────────────────────┐
│  global_variables   │          │         webhooks                  │
├─────────────────────┤          ├───────────────────────────────────┤
│ PK  key        TEXT │          │ PK  id              UUID          │
│     value      TEXT │          │     name            TEXT          │
│     category   TEXT │          │     url             TEXT          │
│     description TEXT│          │     platform        TEXT          │
│     updated_at      │          │     events          TEXT[]        │
└─────────────────────┘          │     secret          TEXT          │
                                 │     enabled         BOOLEAN       │
                                 │     created_at      TIMESTAMPTZ  │
                                 └──────────┬────────────────────────┘
                                            │
                                            │ 1
                                            │
                                            │ *
                                 ┌──────────┴────────────────────────┐
                                 │    webhook_deliveries             │
                                 ├───────────────────────────────────┤
                                 │ PK  id              UUID          │
                                 │ FK  webhook_id      UUID ──► wh  │
                                 │     event           TEXT          │
                                 │     payload         JSONB         │
                                 │     response_code   INT           │
                                 │     response_body   TEXT          │
                                 │     delivered_at    TIMESTAMPTZ  │
                                 │     success         BOOLEAN       │
                                 └───────────────────────────────────┘


┌─────────────────────┐          ┌───────────────────────────────────┐
│      users          │          │           sessions                │
├─────────────────────┤          ├───────────────────────────────────┤
│ PK  username   TEXT │ 1      * │ PK  token           TEXT          │
│     display_name    ├──────────┤ FK  username        TEXT ──► users│
│     password_hash   │          │     created_at      TIMESTAMPTZ  │
│     role       TEXT │          │     expires_at      TIMESTAMPTZ  │
│     auth_source TEXT│          └───────────────────────────────────┘
│     oidc_subject    │
│     created_at      │
│     last_login      │
└─────────────────────┘


 Relationships:
   agents        1 ──── *  jobs               (agent assigned to many jobs over time)
   templates     1 ──── *  jobs               (template used by many jobs)
   jobs          1 ──── *  analysis_results   (job can have multiple analysis types)
   jobs          1 ──── *  task_logs          (job has many log entries from agent execution)
   webhooks      1 ──── *  webhook_deliveries (webhook has delivery history)
   users         1 ──── *  sessions           (user has multiple sessions over time)
   global_variables      (standalone key-value store, no FK relationships)
   api_keys              (standalone auth table, no FK relationships)
```

### 4.1 Core Tables

```sql
-- Agents
CREATE TABLE agents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hostname        TEXT NOT NULL UNIQUE,
    ip_address      INET NOT NULL,
    os_version      TEXT,
    gpu_info        JSONB,          -- {"vendor":"NVIDIA","model":"RTX 4090","vram_mb":24564}
    capabilities    JSONB,          -- {"encoders":["nvenc_h265","x265"],"avs":true,"vpy":true}
    status          TEXT NOT NULL DEFAULT 'pending_approval',  -- pending_approval, online, offline, draining, busy
    last_heartbeat  TIMESTAMPTZ,
    registered_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    config          JSONB           -- agent-specific overrides
);

-- Sources (registered media files accessible via UNC)
CREATE TABLE sources (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    filename     TEXT        NOT NULL,
    unc_path     TEXT        NOT NULL UNIQUE,
    size_bytes   BIGINT      NOT NULL DEFAULT 0,
    detected_by  UUID        REFERENCES agents(id) ON DELETE SET NULL,
    state        TEXT        NOT NULL DEFAULT 'detected'
                             CHECK (state IN ('detected', 'scanning', 'ready', 'encoding', 'done')),
    vmaf_score   FLOAT,
    hdr_type     TEXT        NOT NULL DEFAULT '',    -- '', 'hdr10', 'hdr10+', 'dolby_vision', 'hlg'
    dv_profile   SMALLINT    NOT NULL DEFAULT 0,    -- 0 = no DV, otherwise DV profile number (5,7,8,9,...)
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sources_state ON sources (state);

-- Jobs
CREATE TABLE jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id       UUID NOT NULL REFERENCES sources(id) ON DELETE RESTRICT,  -- normalized FK to sources
    output_path     TEXT,
    job_type        TEXT NOT NULL,   -- encode, analysis, audio, hdr_detect
    status          TEXT NOT NULL DEFAULT 'queued',
    priority        INT NOT NULL DEFAULT 100,
    agent_id        UUID REFERENCES agents(id),
    template_id     UUID REFERENCES templates(id),
    script_type     TEXT,            -- avs, vpy
    encoder_params  TEXT,
    audio_codec     TEXT,            -- flac, opus, aac
    audio_params    JSONB,
    variables       JSONB,           -- job-specific variable overrides
    progress        JSONB,           -- {"frame":12000,"total_frames":48000,"fps":24.5,"eta":"00:12:33"}
    result          JSONB,           -- {"output_size":1234567,"vmaf_mean":96.2,"duration":"01:23:45"}
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ
);
CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_agent  ON jobs(agent_id);

-- Templates
CREATE TABLE templates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL UNIQUE,
    script_type     TEXT NOT NULL,   -- avs, vpy, bat
    category        TEXT NOT NULL,   -- frameserver (avs/vpy) or run_script (bat)
    content         TEXT NOT NULL,   -- Go template source
    description     TEXT,
    variables_used  TEXT[],          -- declared template vars, e.g. {".TrimStart",".GlobalVars.AVS_PLUGINS"}
    created_by      TEXT REFERENCES users(username),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Global Variables
CREATE TABLE global_variables (
    key             TEXT PRIMARY KEY,
    value           TEXT NOT NULL,
    category        TEXT,            -- paths, encoder, audio, general
    description     TEXT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Analysis Results
CREATE TABLE analysis_results (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id          UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    analysis_type   TEXT NOT NULL,   -- histogram, vmaf, scene_detect, hdr_detect
    frame_data      JSONB,           -- per-frame array
    summary         JSONB,           -- aggregated stats
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_analysis_job ON analysis_results(job_id);

-- Task Logs (centralized execution logs from agents)
CREATE TABLE task_logs (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    job_id          UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    agent_hostname  TEXT NOT NULL,        -- which server produced this log line
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT now(),
    stream          TEXT NOT NULL,        -- stdout, stderr, agent (agent = executor-level logs)
    level           TEXT NOT NULL DEFAULT 'info',  -- debug, info, warn, error
    message         TEXT NOT NULL,
    metadata        JSONB                 -- optional structured data: {"frame":2400,"fps":24.5,"gpu_util":87}
);
CREATE INDEX idx_task_logs_job       ON task_logs(job_id);
CREATE INDEX idx_task_logs_job_ts    ON task_logs(job_id, timestamp);
CREATE INDEX idx_task_logs_agent     ON task_logs(agent_hostname);
CREATE INDEX idx_task_logs_level     ON task_logs(job_id, level) WHERE level IN ('warn', 'error');

-- Log retention: a scheduled job (pg_cron or controller background goroutine) deletes
-- rows older than the configured retention period (default 30 days).
-- See config.yaml → logging.task_log_retention.

-- Webhook Endpoints
CREATE TABLE webhooks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    url             TEXT NOT NULL,
    platform        TEXT NOT NULL,   -- discord, teams, slack
    events          TEXT[] NOT NULL,  -- {job.completed, job.failed, agent.online, ...}
    secret          TEXT,            -- HMAC signing secret
    enabled         BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Webhook Delivery Log
CREATE TABLE webhook_deliveries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id      UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event           TEXT NOT NULL,
    payload         JSONB NOT NULL,
    response_code   INT,
    response_body   TEXT,
    delivered_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    success         BOOLEAN NOT NULL
);

-- API Keys
CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    key_hash        TEXT NOT NULL,   -- bcrypt hash
    permissions     TEXT[] NOT NULL, -- {read, write, admin}
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ,
    last_used_at    TIMESTAMPTZ
);

-- Users (authentication)
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

-- Sessions
CREATE TABLE sessions (
    token       TEXT PRIMARY KEY,           -- secure random token (stored in cookie)
    username    TEXT NOT NULL REFERENCES users(username) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL
);
```

### 4.2 High Availability

```
                 ┌─────────────────────┐
                 │     HAProxy /       │
                 │   pgBouncer VIP     │
                 └────────┬────────────┘
                          │
            ┌─────────────┼─────────────┐
            ▼             ▼             ▼
     ┌────────────┐ ┌────────────┐ ┌────────────┐
     │  PG Node 1 │ │  PG Node 2 │ │  PG Node 3 │
     │  (primary) │ │  (replica) │ │  (replica) │
     │  Patroni   │ │  Patroni   │ │  Patroni   │
     └────────────┘ └────────────┘ └────────────┘
            │             ▲             ▲
            └─── streaming replication ─┘
```

- **Patroni** manages leader election via etcd/Consul.
- **pgBouncer** pools connections and routes writes to primary, reads to replicas.
- For simpler setups, a single PostgreSQL instance is perfectly fine — HA is opt-in.

---

## 5. Deployment

### 5.0 Deployment Topology

**Single-node (dev/small):**

```
┌─────────────────────────────────────────────────────────┐
│                 CONTROLLER HOST                          │
│                 Ubuntu 22.04 / Docker                    │
│                                                          │
│   ┌──────────────────────────────────────────────────┐  │
│   │            docker compose                         │  │
│   │                                                   │  │
│   │   ┌────────────────┐    ┌────────────────────┐   │  │
│   │   │  controller    │    │  postgres:16       │   │  │
│   │   │  :8080  :9443  │───►│  :5432             │   │  │
│   │   │                │    │  pgdata volume     │   │  │
│   │   └────────────────┘    └────────────────────┘   │  │
│   │                                                   │  │
│   └──────────────────────────────────────────────────┘  │
│                                                          │
│   Optional: nginx reverse proxy (:443 → :8080)          │
└──────────────────────┬───────────────────────────────────┘
                       │ :9443 gRPC/mTLS
            ┌──────────┼──────────┐
            ▼          ▼          ▼
      ┌──────────┐┌──────────┐┌──────────┐
      │ Agent-01 ││ Agent-02 ││ Agent-03 │
      │ Win Svr  ││ Win Svr  ││ Win Svr  │
      └──────────┘└──────────┘└──────────┘
```

**High-availability (production):**

```
                          ┌───────────────────┐
                          │   Load Balancer   │
                          │   (HAProxy/nginx) │
                          │   :443  :9443     │
                          └─────────┬─────────┘
                                    │
                     ┌──────────────┼──────────────┐
                     ▼                             ▼
         ┌─────────────────────┐       ┌─────────────────────┐
         │  Controller Node 1  │       │  Controller Node 2  │
         │  (active)           │       │  (standby/active)   │
         │  Ubuntu / Container │       │  Ubuntu / Container │
         └──────────┬──────────┘       └──────────┬──────────┘
                    │                              │
                    └──────────────┬───────────────┘
                                   │
                    ┌──────────────┼──────────────┐
                    ▼              ▼              ▼
           ┌──────────────┐┌──────────────┐┌──────────────┐
           │  PG Node 1   ││  PG Node 2   ││  PG Node 3   │
           │  (primary)   ││  (replica)   ││  (replica)   │
           │  Patroni     ││  Patroni     ││  Patroni     │
           └──────┬───────┘└──────┬───────┘└──────┬───────┘
                  │               │               │
                  └─── etcd cluster (3 nodes) ────┘
                                   │
                    ┌──────────────┼──────────────┐
                    ▼              ▼              ▼
              ┌──────────┐  ┌──────────┐   ┌──────────┐
              │ pgBouncer│  │ pgBouncer│   │ pgBouncer│
              │ (pool)   │  │ (pool)   │   │ (pool)   │
              └──────────┘  └──────────┘   └──────────┘

  ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─

              ┌──────────┐  ┌──────────┐   ┌──────────┐   ┌──────────┐
              │ Agent-01 │  │ Agent-02 │   │ Agent-03 │   │ Agent-N  │
              │ Win Svr  │  │ Win Svr  │   │ Win Svr  │   │ Win Svr  │
              │ RTX 4090 │  │ RTX 3080 │   │ Arc A770 │   │ CPU only │
              └──────────┘  └──────────┘   └──────────┘   └──────────┘
                    │              │               │              │
                    └──────────────┴───────────────┴──────────────┘
                                         │
                                   ┌─────┴─────┐
                                   │  NAS/SAN  │
                                   │  (SMB3)   │
                                   └───────────┘
```

### 5.1 Controller (Docker Compose)

```yaml
# docker-compose.yml
services:
  controller:
    build: .
    image: distencoder/controller:latest
    ports:
      - "8080:8080"   # Web UI + REST API
      - "9443:9443"   # gRPC (agent comms)
    environment:
      - DATABASE_URL=postgres://distenc:${DB_PASS}@postgres:5432/distencoder?sslmode=require
      - GRPC_TLS_CERT=/certs/server.crt
      - GRPC_TLS_KEY=/certs/server.key
      - GRPC_TLS_CA=/certs/ca.crt
    volumes:
      - ./certs:/certs:ro
    depends_on:
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: distencoder
      POSTGRES_USER: distenc
      POSTGRES_PASSWORD: ${DB_PASS}
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U distenc"]
      interval: 5s
      timeout: 3s
      retries: 5

volumes:
  pgdata:
```

### 5.2 Controller Dockerfile

```dockerfile
# Stage 1: Web UI
FROM node:22-alpine AS web-builder
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Stage 2: Go binary
FROM golang:1.25-alpine AS go-builder
ARG VERSION=dev
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.Version=${VERSION}" \
    -o /controller ./cmd/controller

# Stage 3: FFmpeg (static build with VMAF, HDR10+, Dolby Vision)
FROM mwader/static-ffmpeg:latest AS ffmpeg-source

# Stage 4: dovi_tool (Dolby Vision RPU extraction)
FROM debian:bookworm-slim AS tools-fetcher
ARG DOVI_TOOL_VERSION=2.1.3
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
    && curl -fsSL "https://github.com/quietvoid/dovi_tool/releases/download/${DOVI_TOOL_VERSION}/dovi_tool-${DOVI_TOOL_VERSION}-x86_64-unknown-linux-musl.tar.gz" \
       | tar xz -C /usr/local/bin

# Stage 5: Runtime (~30 MB)
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*
COPY --from=go-builder    /controller               /app/controller
COPY --from=ffmpeg-source /ffmpeg                    /usr/local/bin/ffmpeg
COPY --from=ffmpeg-source /ffprobe                   /usr/local/bin/ffprobe
COPY --from=tools-fetcher /usr/local/bin/dovi_tool   /usr/local/bin/dovi_tool
WORKDIR /app
EXPOSE 8080 9443
ENTRYPOINT ["/app/controller", "run"]
```

### 5.3 Agent Installation (Windows Server)

```powershell
# Download the agent binary
Invoke-WebRequest -Uri "https://releases.example.com/distencoder-agent.exe" `
  -OutFile "C:\DistEncoder\distencoder-agent.exe"

# Install as Windows Service
sc.exe create DistEncoderAgent `
  binPath= "C:\DistEncoder\distencoder-agent.exe --config C:\DistEncoder\agent.yaml" `
  start= auto `
  DisplayName= "Distributed Encoder Agent"

# Start the service
sc.exe start DistEncoderAgent
```

**agent.yaml:**

```yaml
controller:
  address: "controller.internal:9443"
  tls:
    cert: "C:\\DistEncoder\\certs\\agent.crt"
    key: "C:\\DistEncoder\\certs\\agent.key"
    ca: "C:\\DistEncoder\\certs\\ca.crt"

agent:
  hostname: "ENCODE-SERVER-01"          # auto-detected if omitted
  work_dir: "C:\\DistEncoder\\work"
  log_dir: "C:\\DistEncoder\\logs"
  offline_db: "C:\\DistEncoder\\offline.db"

tools:
  ffmpeg: "C:\\Tools\\ffmpeg\\ffmpeg.exe"
  ffprobe: "C:\\Tools\\ffmpeg\\ffprobe.exe"
  x265: "C:\\Tools\\x265\\x265.exe"
  avs_pipe: "C:\\Program Files\\AviSynth+\\avs2pipemod.exe"
  vspipe: "C:\\Program Files\\VapourSynth\\vspipe.exe"

gpu:
  enabled: true
  # auto-detect vendor; override if needed
  # vendor: nvidia
```

### 5.4 Controller Configuration (`config.yaml`)

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
  task_log_retention: 30d    # how long task logs are kept in PostgreSQL
  task_log_cleanup_interval: 6h  # how often the cleanup goroutine runs
  task_log_max_lines_per_job: 500000  # safety cap per job

api:
  docs_enabled: false        # serve Swagger UI at /api/docs (admin only)
  rate_limit: 100            # requests per minute per client (0 = disabled)
  page_size_default: 50      # default page_size for list endpoints
  page_size_max: 200         # maximum allowed page_size
```

Secrets are loaded from a `.env` file at startup — see [Section 6 — Security](#6-security) for the `DE_`-prefixed variable reference.

---

## 6. Security

| Layer | Mechanism |
|---|---|
| Agent ↔ Controller | mTLS (mutual TLS) on gRPC. Only agents with valid certs can connect. |
| REST API | API key (header) for automation; JWT (cookie) for web UI sessions. |
| Web UI | HTTPS (TLS termination at reverse proxy or built-in). CSRF tokens. CSP headers. |
| Database | TLS connections. Scoped user permissions (no superuser for the app). |
| Webhooks | HMAC-SHA256 signing so receivers can verify authenticity. |
| Secrets | All secrets (DB password, API keys, TLS keys) from `.env` files loaded at startup (not in `config.yaml` or VCS). A `.env.example` documents required variables with placeholder values. |
| UNC paths | Agent validates that the UNC path is on an allow-listed share prefix before accessing. |

**Environment variable naming:** All environment variables use a `DE_` prefix for namespacing.

**Controller `.env`:**
```env
# PostgreSQL credentials
DE_DB_USER=distencoder
DE_DB_PASS=changeme

# Agent pre-shared API keys (comma-separated for multiple key groups)
DE_AGENT_API_KEYS=key-server-group-a,key-server-group-b

# OIDC client credentials (required if oidc.enabled=true)
DE_OIDC_CLIENT_ID=
DE_OIDC_CLIENT_SECRET=

# TLS key passphrase (if applicable)
DE_TLS_KEY_PASSPHRASE=
```

**Agent `.env`** (on each target server, e.g., `C:\DistEncoder\.env`):
```env
DE_API_KEY=key-server-group-a
```

A `.env.example` is committed to the repo documenting all variables with placeholder values. The actual `.env` files are in `.gitignore`.

---

## 7. Observability

| Signal | Tool |
|---|---|
| **Metrics** | Prometheus exposition endpoint (`/metrics`) on Controller. Agent exposes local metrics scraped by Controller or Prometheus. Key metrics: jobs_total, jobs_active, job_duration_seconds, agent_gpu_utilisation, encode_fps. |
| **Task Logs** | **Centralized in PostgreSQL** (`task_logs` table). Agents stream stdout, stderr, and executor-level logs to the controller via gRPC `StreamLogs`. Viewable in the web UI with live tail (SSE), filtering, and search. No need to access agent machines for log inspection. Retained for a configurable period (default 30 days). See [Section 3.1.8 — Centralized Task Logging](#318-centralized-task-logging). |
| **System Logs** | Structured JSON logs (`log/slog`, stdlib, Go 1.21+). Every log entry includes a **correlation ID** (`request_id` for REST, `assignment_id` for tasks) linking the log line to a specific operation. Agent system logs include `server_name` on every entry for filtering by host. Ship to Loki / ELK / CloudWatch for controller/agent process-level diagnostics. |
| **Tracing** | OpenTelemetry spans for job lifecycle. Optional Jaeger/Zipkin backend. |
| **Alerting** | Prometheus Alertmanager rules → Discord/Teams/Slack (reuses webhook adapters). |

**Logging Architecture — Two Layers:**

```
┌─────────────────────────────────────────────────────────────────┐
│                     Logging Architecture                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Layer 1: Task Logs (centralized in PostgreSQL)                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  Source: Agent stdout/stderr/executor during task runs     │  │
│  │  Transport: gRPC StreamLogs → PostgreSQL task_logs table   │  │
│  │  Access: Web UI log viewer, REST API, .log download        │  │
│  │  Retention: Configurable (default 30 days)                 │  │
│  │  Purpose: Operator-facing — "what happened during encode?" │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                  │
│  Layer 2: System Logs (log/slog → external aggregator)           │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  Source: Controller + Agent process-level slog output       │  │
│  │  Transport: stdout/stderr → Loki / ELK / CloudWatch        │  │
│  │  Access: External log aggregation UI (Grafana, Kibana)     │  │
│  │  Retention: Per aggregator configuration                    │  │
│  │  Purpose: Ops/SRE-facing — "is the service healthy?"       │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                  │
│  Both layers share the same correlation IDs (assignment_id,      │
│  request_id) so task logs and system logs can be cross-          │
│  referenced when debugging issues.                               │
└─────────────────────────────────────────────────────────────────┘
```

---

## 8. Project Layout

```
distributed-encoder/
├── cmd/
│   ├── controller/         # Controller entrypoint
│   │   └── main.go
│   └── agent/              # Agent entrypoint
│       └── main.go
├── internal/
│   ├── controller/
│   │   ├── api/            # REST handlers (net/http)
│   │   ├── grpc/           # gRPC server implementation
│   │   ├── scheduler/      # Job scheduling logic
│   │   ├── scriptgen/      # Template rendering (avs/vpy/bat)
│   │   ├── analysis/       # Histogram, VMAF, scene detect coordination
│   │   ├── webhook/        # Webhook dispatcher + adapters (discord/teams/slack)
│   │   ├── auth/           # Auth middleware, local login (bcrypt), OIDC, roles
│   │   ├── tasklog/        # Centralized task log ingestion, retention, SSE streaming
│   │   ├── openapi/        # OpenAPI 3.1 spec generation and Swagger UI handler
│   │   └── config/         # Controller configuration
│   ├── agent/
│   │   ├── executor/       # Task execution (bat runner, progress parser, log capture)
│   │   ├── logstream/      # gRPC StreamLogs client — ships stdout/stderr/agent logs to controller
│   │   ├── validator/      # Pre-execution task parameter validation
│   │   ├── scanner/        # UNC path scanner for .m2ts source detection
│   │   ├── gpu/            # GPU detection and monitoring
│   │   ├── offline/        # SQLite journal for offline resilience (logs + results)
│   │   ├── service/        # Windows Service integration
│   │   └── config/         # Agent configuration
│   ├── db/
│   │   ├── migrations/     # SQL migration files (001_init.up.sql, etc.)
│   │   ├── queries/        # sqlc query definitions
│   │   └── models/         # Generated models (sqlc)
│   └── shared/
│       ├── proto/          # Protobuf definitions
│       ├── models/         # Shared domain types
│       └── unc/            # UNC path utilities + validation
├── web/                    # React + Vite frontend source
│   ├── src/
│   └── package.json
├── proto/
│   └── agent.proto         # gRPC service definitions
├── deployments/
│   ├── docker-compose.yml
│   ├── Dockerfile
│   └── patroni/            # HA PostgreSQL configs
├── scripts/
│   └── install-agent.ps1   # Windows agent installer
├── configs/
│   ├── controller.yaml.example
│   ├── agent.yaml.example
│   └── inventory.json         # Optional server seed file
├── .env.example               # Documents all DE_* env vars (committed)
├── .env                       # Actual secrets (in .gitignore, never committed)
├── go.mod
├── go.sum
├── Makefile
├── ARCHITECTURE.md
└── README.md
```

---

## 9. Build & Cross-Compile

```makefile
# Makefile
VERSION := $(shell git describe --tags --always)
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION)"

.PHONY: all controller agent web clean

all: web controller agent

controller:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/controller-linux-amd64 ./cmd/controller

agent:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/agent-windows-amd64.exe ./cmd/agent

web:
	cd web && npm ci && npm run build
	# Output is embedded via go:embed in internal/controller/api/

clean:
	rm -rf dist/ web/build/

test:
	go test ./... -race -cover

lint:
	golangci-lint run ./...

proto:
	protoc --go_out=. --go-grpc_out=. proto/agent.proto

migrate-up:
	migrate -path internal/db/migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path internal/db/migrations -database "$(DATABASE_URL)" down 1
```

**Key**: `CGO_ENABLED=0` produces fully static binaries. The Windows agent binary is cross-compiled from Linux CI — no Windows build toolchain needed.

---

## 10. Job Workflow — End to End

```
User creates job via Web UI / API
         │
         ▼
┌─────────────────┐
│  Job persisted   │ status: queued
│  in PostgreSQL   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Scheduler picks │ Finds idle agent with matching
│  next job        │ capabilities (GPU, avs/vpy, etc.)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Script Generator│ Renders .avs/.vpy + .bat from
│  builds scripts  │ template + global vars + job vars
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Script Generator│ Writes .avs/.vpy + .bat to UNC share
│  writes to share │ \\NAS\...\chunks\chunk_NNN\
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Agent receives  │ via gRPC PollTask()
│  TaskAssignment  │ includes UNC script_dir + DE_PARAM_* vars
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Agent validates │ DE_PARAM_* completeness, UNC path
│  task            │ allow-list, script presence, timeout
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Agent executes  │ cmd.exe /c \\NAS\...\chunk_NNN\encode.bat
│  .bat file       │ Streams progress → Controller (or local SQLite)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Result reported │ success → job.completed
│  to Controller   │ failure → job.failed
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Webhook fires   │ Discord / Teams / Slack notification
│  to configured   │ with job details and outcome
│  endpoints       │
└─────────────────┘
```

---

## 11. Future Considerations

| Area | Description |
|---|---|
| **Chunk-based parallel encoding** | Split source at scene boundaries, encode segments in parallel across agents, concatenate. |
| **Linux agent support** | ✅ Implemented — agent binary cross-compiles to Linux (`GOOS=linux`). systemd service integration via `install`/`start`/`stop`/`uninstall` subcommands (writes `/etc/systemd/system/<name>.service`). Tasks delivered as `.sh` scripts executed via `/bin/sh`. NFS mount paths accepted in `allowed_shares`. See DEPLOYMENT.md §3.7 and `configs/agent-linux.yaml.example`. |
| **NFS support** | ✅ Implemented — allowed_shares and source path validation accept POSIX absolute paths (/mnt/nas/media) for NFS mounts alongside Windows UNC paths. Linux agents can mount NFS shares natively. |
| **S3 / object storage** | Support cloud storage as source/destination in addition to UNC/NFS shares. |
| **Multi-controller HA** | Active-passive or active-active controllers behind a load balancer for zero-downtime upgrades. |
| **Web-based VNC/RDP** | ✅ Implemented — NoVNC browser client served by the controller at `/novnc/{agent_id}`. Controller proxies WebSocket to the agent's VNC TCP port. Agent auto-installs TightVNC via `setup-vnc` subcommand with configurable installer URL. VNC port reported to controller at registration; "Remote Desktop" button appears in the Agents page when vnc_port > 0. |
