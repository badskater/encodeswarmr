# Capacity Planning Guide

This document helps operators estimate the hardware, storage, and network resources
needed to run the distributed-encoder system at a given encoding workload.

---

## 1. Agent Count

### Rule of thumb

Each agent handles **one task at a time** (strict single-task concurrency by design).
A task maps to one video chunk — typically 1 000–5 000 frames.

```
agents_needed = ceil(desired_throughput_chunks_per_hour / chunks_per_agent_per_hour)
```

**Estimating chunks_per_agent_per_hour:**

| Codec / preset       | Typical speed (CPU-only)    | Typical speed (GPU NVENC) |
|----------------------|-----------------------------|---------------------------|
| H.264 medium         | ~200–400 fps                | ~800–1 500 fps            |
| H.265 / HEVC medium  | ~80–180 fps                 | ~400–900 fps              |
| AV1 (SVT-AV1 fast)   | ~40–120 fps                 | N/A (SW only)             |

At 30 fps source, 2 000-frame chunks, and 300 fps encode speed:
- chunk duration: 66 s of encode time
- chunks per hour per agent: ~54

For 500 chunks/hour target: `ceil(500 / 54)` ≈ **10 agents**.

### Headroom

Add 20–30 % spare agents to absorb agent failures, Windows Updates, and burst
workloads without stalling queues.

---

## 2. CPU / GPU Requirements per Agent

### CPU-only agents (minimum viable)

| Component | Recommendation                              |
|-----------|---------------------------------------------|
| CPU       | 8+ physical cores (16+ for HEVC / AV1)      |
| RAM       | 16 GB minimum; 32 GB recommended            |
| Storage   | Fast local SSD for temp chunk output (NVMe preferred) |
| OS        | Windows Server 2019 / 2022                  |

### GPU-accelerated agents

| Component | Recommendation                              |
|-----------|---------------------------------------------|
| CPU       | 4–8 cores (GPU offloads the heavy work)     |
| GPU       | NVIDIA RTX/Quadro (NVENC); AMD RX/Pro (AMF); Intel Arc/iGPU (QSV) |
| GPU VRAM  | 4 GB minimum; 8 GB+ for 4K HDR              |
| RAM       | 16 GB                                       |

> One NVENC session per GPU per agent. NVIDIA consumer GPUs cap concurrent NVENC
> sessions at 3–5 (driver-enforced); Quadro/Data Center GPUs are uncapped.

### Concurrent GPU sessions

When multiple agents share a single physical machine (VMs / containers), divide
available NVENC slots by number of agents. Do not over-subscribe GPU VRAM.

---

## 3. Storage Sizing

### Source files (NAS / SAN)

| Content type          | Typical bitrate    | 1 TB holds       |
|-----------------------|--------------------|------------------|
| Blu-ray remux (H.264) | 25–40 Mbps         | ~55–88 hours     |
| UHD remux (H.265/HDR) | 50–80 Mbps         | ~27–44 hours     |
| Web-DL 1080p          | 8–15 Mbps          | ~148–278 hours   |

Estimate: `source_GB = hours_of_content × avg_bitrate_Mbps × 450`
(450 ≈ seconds-per-GB at 1 Mbps; adjust for actual bitrate.)

### Output files

Output is typically 40–70 % of source size depending on codec efficiency gains.
Reserve at least **1.5× source capacity** for output on a separate volume or path.

### Temporary chunk working area (per agent)

Each agent writes encoded chunk files to local disk before transfer.
Size per active task: `chunk_frames / fps × output_bitrate_Mbps × 0.125` MB

Example: 2 000 frames, 30 fps, 5 Mbps output → ~42 MB per chunk.
With a safety margin: reserve **10 GB SSD** per agent for temp work.

### Database (PostgreSQL)

| Row type         | Avg size | 1 M rows |
|------------------|----------|----------|
| jobs             | ~500 B   | ~500 MB  |
| tasks            | ~600 B   | ~600 MB  |
| task_logs        | ~200 B   | ~200 MB  |
| agent_metrics    | ~50 B    | ~50 MB   |
| audit_log        | ~300 B   | ~300 MB  |

**Growth estimates (busy deployment, 10 agents, 500 jobs/day):**

- jobs: ~15 K rows/month
- tasks (20 chunks/job avg): ~300 K rows/month
- task_logs (200 lines/task avg): ~60 M rows/month → ~12 GB/month

**Recommendation:** Configure log retention (`prune_logs_older_than`) to 30–90 days.
Use a Patroni HA cluster with at least 100 GB PostgreSQL volume for long-term installs.
Consider tablespace tiering: keep recent logs on SSD, archive older rows to slower storage.

---

## 4. Network Bandwidth

### NAS ↔ Agent (source file reads / output writes)

Each agent reads source chunks and writes encoded output over the network.

| Operation         | Typical rate      |
|-------------------|-------------------|
| Source read       | 100–500 MB/s (per agent, NVMe NAS) |
| Output write      | 5–80 MB/s (depends on codec bitrate) |

For 10 simultaneous agents at peak: plan **10 Gbps** NAS uplink to avoid I/O starvation.
Minimum viable: 1 Gbps per 2–3 agents for standard 1080p workloads.

### Agent ↔ Controller (gRPC + mTLS)

The controller sends task assignments, script files, and heartbeat ACKs.
Traffic is very low: < 1 Mbps per 50 agents under normal operation.

| Traffic type       | Bandwidth |
|--------------------|-----------|
| Heartbeats (30 s)  | ~2 KB/agent/min |
| Task assignment    | ~10–50 KB per task (scripts + metadata) |
| Log streaming      | ~50–200 KB/task (stdout/stderr) |

A 1 Gbps LAN is more than sufficient for hundreds of agents.

### Internet / external

- Webhook notifications (Discord, Teams, Slack): negligible, < 1 KB per event.
- Agent auto-update binary downloads: ~20–50 MB per upgrade event; stagger upgrades.

---

## 5. Quick-Reference Worksheet

```
desired_output_hours_per_day = ?
avg_source_duration_min      = ?
chunks_per_job               = ceil(avg_source_duration_min * 60 * fps / chunk_frames)
encode_speed_fps             = ? (from codec/preset table above)
chunk_encode_time_s          = chunk_frames / encode_speed_fps
chunks_per_agent_per_day     = 86400 / chunk_encode_time_s  (single task/agent)
jobs_per_day                 = desired_output_hours_per_day * 60 / avg_source_duration_min
total_chunks_per_day         = jobs_per_day * chunks_per_job
agents_needed                = ceil(total_chunks_per_day / chunks_per_agent_per_day * 1.25)
```

Multiply `agents_needed` by per-agent storage and network figures above to size
your NAS, database server, and switch uplinks.
