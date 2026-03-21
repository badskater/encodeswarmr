# ADR-007: Chunk Encoding Architecture

## Status: Accepted

## Context

Long video files (feature films, 4K HDR content) can take 10–20 hours to encode on
a single machine. To leverage multiple agents simultaneously, files must be split into
independently encodable segments. Scene detection (via ffmpeg/ffprobe) provides
natural split points that minimise quality impact at boundaries.

## Decision

A job is expanded into N encode tasks, each covering a `ChunkBoundary` (start_frame,
end_frame) stored in `encode_config.chunk_boundaries` (JSONB, see migration
`008_encode_config.up.sql`). The scheduler writes tasks to the `tasks` table
(`task_type = ''` for encode tasks) and the engine dispatches them to available
agents. Chunk size and overlap frames are configurable via `ChunkingConfig` in the
job creation payload.

Scene boundaries are detected by the analysis pipeline
(`internal/controller/analysis`) and surfaced via
`GET /api/v1/sources/{id}/scenes`. The UI uses these to propose split points before
job submission.

## Consequences

**Positive:** Near-linear speed scaling with agent count, scene-aligned splits
eliminate visible quality glitches at chunk joins.

**Negative:** Chunking requires a concat step after all encode tasks complete (see
ADR-008). Scene detection adds latency before encoding can start. Small chunk counts
on short content provide little benefit.
