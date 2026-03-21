# ADR-008: Controller-Side Concat Task

## Status: Accepted

## Context

After all chunk encode tasks for a job complete, the output segments must be merged
into a single output file using `ffmpeg -f concat`. The merge step needs access to
all chunk output paths and must run exactly once. Options: (1) run concat on any
available agent, (2) run concat on the controller itself using its NFS mount.

Running on an agent requires sending the list of chunk output paths and picking an
agent that has NAS read/write access — equivalent to any other agent but adds task
dispatch overhead. Running on the controller avoids round-trips and the NFS mount
is already available for analysis tasks.

## Decision

Concat is modelled as a special `task_type = 'concat'` task (see migration
`014_task_type.up.sql`). After all encode tasks complete, the engine creates one
concat task per job. The concat task is claimed by the controller's own analysis
runner (`ClaimConcatTask` in `internal/db/db.go`) rather than dispatched to agents.
The concat command is built from the job's `encode_config.chunk_boundaries` and
executed via `exec.Command` using the configured `ffmpeg_bin`.

## Consequences

**Positive:** No agent required for the final merge, avoids NAS path re-translation,
concat failures are immediately visible in controller logs with full context.

**Negative:** Controller must have ffmpeg installed and NFS mount access. Concat
task failures block job completion; the job must be retried manually via
`POST /api/v1/jobs/{id}/retry`.
