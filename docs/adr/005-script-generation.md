# ADR-005: Server-Side Script Generation

## Status: Accepted

## Context

Each encoding task requires an AviSynth (.avs) or VapourSynth (.vpy) frameserver
script, a batch file (.bat) that invokes the encoder, and optional audio conversion
scripts. The scripts must be parameterised with source path, output path, chunk
boundaries, and global variables. Two strategies were considered: (1) agent-side
generation using templates downloaded at task-claim time, or (2) controller-side
generation with scripts written to a shared NAS path.

## Decision

Generate all scripts on the controller using the template engine
(`internal/controller/engine`). Templates are stored in the `templates` table and
rendered with Go's `text/template`. Generated files are written to a NAS path
(`agent.script_base_dir` config key, e.g. `\\NAS01\temp\scripts\{task_id}\`).
Agents receive only the script directory path via the gRPC `TaskAssignment` message
and execute whatever they find there.

## Consequences

**Positive:** Centralised template versioning, consistent script output, no template
distribution overhead, easy audit of generated scripts.

**Negative:** Controller requires write access to the NAS. If the NAS share is
unavailable, script generation blocks task dispatch. Script directory must be
cleaned up after task completion to avoid disk exhaustion.
