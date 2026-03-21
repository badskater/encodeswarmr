# ADR-004: Split Platform — Windows Agents, Linux Controller

## Status: Accepted

## Context

Video encoding tools (x265, x264, SVT-AV1, AviSynth+, VapourSynth, NVENC) have
the best support and performance on Windows, particularly for GPU-accelerated encodes
(NVENC via CUDA, QSV via Intel Media SDK, AMF via AMD). However, container
orchestration, PostgreSQL, and long-running service management are better suited
to Linux.

## Decision

Agents run as Windows Services on Windows Server hosts using the
`golang.org/x/sys/windows/svc` package. The controller runs on Ubuntu 22.04+ or
in Docker (Linux amd64). Cross-compilation from the Go module produces both targets.
UNC paths (`\\NAS01\media`) are used for source files on the agent side; the
controller accesses the same NAS via NFS mounts (`/mnt/nas/media`) using path
mapping translations stored in the `path_mappings` table.

## Consequences

**Positive:** Access to the full Windows GPU encode ecosystem, battle-tested GPU
driver support, Windows Service lifecycle management.

**Negative:** Requires maintaining two deployment targets; NAS path mapping logic
adds complexity (`internal/controller/analysis` and the `pathmappings` API).
