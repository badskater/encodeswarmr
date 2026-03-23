package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"

	"github.com/badskater/encodeswarmr/internal/db"
)

// expandPendingJobs queries jobs needing expansion and expands each one.
func (e *Engine) expandPendingJobs(ctx context.Context) error {
	jobs, err := e.store.GetJobsNeedingExpansion(ctx)
	if err != nil {
		return fmt.Errorf("engine: get jobs needing expansion: %w", err)
	}
	for _, job := range jobs {
		if err := e.expandJob(ctx, job); err != nil {
			e.logger.Warn("engine: expand job failed",
				"job_id", job.ID,
				"error", err,
			)
		}
	}
	return nil
}

// expandJob dispatches to the appropriate expansion strategy based on job type.
// When an AnalysisRunner is configured, analysis/hdr_detect/audio jobs run
// directly on the controller host instead of being dispatched to an agent.
// When EncodeConfig.FlowID is set, flow-based expansion is used instead of
// the default template path.
func (e *Engine) expandJob(ctx context.Context, job *db.Job) error {
	// If a flow pipeline is attached, delegate to the flow engine.
	if job.EncodeConfig.FlowID != "" {
		return e.expandFlowJob(ctx, job)
	}

	// Route analysis-type jobs to the controller runner when available.
	if e.analysis != nil && isControllerAnalysisJob(job.JobType) {
		return e.expandControllerAnalysisJob(ctx, job)
	}

	switch job.JobType {
	case "encode":
		return e.expandEncodeJob(ctx, job)
	case "analysis", "audio":
		return e.expandSingleTaskJob(ctx, job)
	case "hdr_detect":
		return e.expandHDRDetectJob(ctx, job)
	case "merge":
		return e.expandMergeJob(ctx, job)
	default:
		e.logger.Error("engine: unknown job type, skipping", "job_id", job.ID, "job_type", job.JobType)
		return nil
	}
}

// expandFlowJob uses the FlowEngine to translate a flow graph into TaskSteps
// and then creates one task per step.  This is the entry point for jobs that
// carry a flow_id in their EncodeConfig.
func (e *Engine) expandFlowJob(ctx context.Context, job *db.Job) error {
	flowID := job.EncodeConfig.FlowID

	flow, err := e.store.GetFlowByID(ctx, flowID)
	if err != nil {
		_ = e.store.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("engine: expand flow job: get flow %s: %w", flowID, err)
	}

	fe := NewFlowEngine(e.store, e.logger)
	steps, err := fe.ExecuteFlow(ctx, flow, job.SourceID)
	if err != nil {
		_ = e.store.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("engine: expand flow job: execute flow %s: %w", flowID, err)
	}

	if len(steps) == 0 {
		e.logger.Warn("engine: flow produced no steps, marking job failed",
			"job_id", job.ID, "flow_id", flowID)
		_ = e.store.UpdateJobStatus(ctx, job.ID, "failed")
		return nil
	}

	// failJob cleans up any tasks already created before returning the error.
	failJob := func(cause error) error {
		if delErr := e.store.DeleteTasksByJobID(ctx, job.ID); delErr != nil {
			e.logger.Error("engine: cleanup orphan tasks (flow)",
				"job_id", job.ID, "error", delErr)
		}
		_ = e.store.UpdateJobStatus(ctx, job.ID, "failed")
		return cause
	}

	// Load source once — most steps need it.
	source, srcErr := e.store.GetSourceByID(ctx, job.SourceID)
	if srcErr != nil {
		return failJob(fmt.Errorf("engine: expand flow job get source: %w", srcErr))
	}

	// Load source analysis summary for condition evaluation.
	var analysisSummary map[string]any
	if ars, err := e.store.ListAnalysisResults(ctx, job.SourceID); err == nil {
		for _, ar := range ars {
			if len(ar.Summary) > 0 {
				_ = json.Unmarshal(ar.Summary, &analysisSummary)
				break
			}
		}
	}

	taskIndex := 0
	for _, step := range steps {
		vars := make(map[string]string)
		for k, v := range step.Config {
			if s, ok := v.(string); ok {
				vars[k] = s
			}
		}
		vars["flow_node_id"] = step.NodeID
		vars["flow_node_type"] = step.NodeType

		switch step.NodeType {
		case "notify_webhook", "webhook":
			// Fire the webhook notification immediately during expansion.
			if e.webhooks != nil {
				eventType := "flow.notify"
				if et, ok := step.Config["event_type"].(string); ok && et != "" {
					eventType = et
				}
				payload := map[string]any{
					"job_id":      job.ID,
					"source_id":   job.SourceID,
					"flow_id":     flowID,
					"node_id":     step.NodeID,
					"source_path": source.UNCPath,
				}
				for k, v := range step.Config {
					payload[k] = v
				}
				e.webhooks.EmitRaw(ctx, eventType, payload)
			} else {
				e.logger.Info("engine: flow webhook step (no emitter configured)",
					"job_id", job.ID, "node_id", step.NodeID,
					slog.Any("config", step.Config),
				)
			}
			// Webhook nodes do not create DB tasks.
			continue

		case "template_run":
			// Look up the named template and inject its content as script_content.
			if templateID, ok := step.Config["template_id"].(string); ok && templateID != "" {
				if tmpl, err := e.store.GetTemplateByID(ctx, templateID); err == nil {
					vars["script_content"] = tmpl.Content
					vars["template_name"] = tmpl.Name
				} else {
					e.logger.Warn("engine: flow template_run: template not found",
						"job_id", job.ID, "template_id", templateID, "error", err)
				}
			}

		case "audio_flac", "audio_opus", "audio_aac":
			// Map audio node types to ffmpeg audio arguments.
			audioArgs := map[string]string{
				"audio_flac": "-vn -c:a flac",
				"audio_opus": "-vn -c:a libopus -b:a 128k",
				"audio_aac":  "-vn -c:a aac -b:a 192k -profile:a aac_low",
			}
			vars["audio_ffmpeg_args"] = audioArgs[step.NodeType]
			vars["ffmpeg_audio_codec"] = step.NodeType[len("audio_"):]

		case "condition":
			// Evaluate real condition using source analysis data.
			if analysisSummary != nil {
				condKey, _ := step.Config["field"].(string)
				if condKey != "" {
					if val, exists := analysisSummary[condKey]; exists {
						vars["condition_field"] = condKey
						vars["condition_value"] = fmt.Sprintf("%v", val)
					}
				}
			}
			// Condition nodes do not create DB tasks; the FlowEngine already
			// evaluated the branch and pruned the graph accordingly.
			continue
		}

		if _, err := e.store.CreateTask(ctx, db.CreateTaskParams{
			JobID:      job.ID,
			ChunkIndex: taskIndex,
			TaskType:   db.TaskTypeEncode,
			SourcePath: source.UNCPath,
			Variables:  vars,
		}); err != nil {
			return failJob(fmt.Errorf("engine: expand flow job create task step %d: %w", taskIndex, err))
		}
		taskIndex++
	}

	if err := e.store.UpdateJobTaskCounts(ctx, job.ID); err != nil {
		return fmt.Errorf("engine: expand flow job update task counts: %w", err)
	}
	if err := e.store.UpdateJobStatus(ctx, job.ID, "queued"); err != nil {
		return fmt.Errorf("engine: expand flow job update status: %w", err)
	}
	return nil
}

// expandControllerAnalysisJob runs an analysis/hdr_detect/audio job directly
// on the controller.  It sets the job status to "running" synchronously, then
// launches a goroutine that calls the AnalysisRunner and marks the job as
// "completed" or "failed" when done.
func (e *Engine) expandControllerAnalysisJob(ctx context.Context, job *db.Job) error {
	source, err := e.store.GetSourceByID(ctx, job.SourceID)
	if err != nil {
		return fmt.Errorf("engine: controller analysis get source %s: %w", job.SourceID, err)
	}

	// Transition to "running" immediately so the job is not re-picked-up.
	if err := e.store.UpdateJobStatus(ctx, job.ID, "running"); err != nil {
		return fmt.Errorf("engine: controller analysis set running: %w", err)
	}

	go func() {
		var runErr error
		switch job.JobType {
		case "hdr_detect":
			runErr = e.analysis.RunHDRDetect(ctx, job, source)
		case "analysis":
			runErr = e.analysis.RunAnalysis(ctx, job, source)
		case "audio":
			runErr = e.analysis.RunAudio(ctx, job, source)
		}

		if runErr != nil {
			e.logger.Error("engine: controller analysis job failed",
				"job_id", job.ID, "job_type", job.JobType, "error", runErr)
			_ = e.store.UpdateJobStatus(ctx, job.ID, "failed")
			return
		}

		if err := e.store.UpdateJobStatus(ctx, job.ID, "completed"); err != nil {
			e.logger.Error("engine: set job completed", "job_id", job.ID, "error", err)
			return
		}
		e.logger.Info("engine: controller analysis job completed",
			"job_id", job.ID, "job_type", job.JobType)

		// Unblock any dependent jobs that were waiting for this one.
		if unblockErr := e.store.UnblockDependentJobs(ctx, job.ID); unblockErr != nil {
			e.logger.Warn("engine: unblock dependent jobs failed",
				"job_id", job.ID, "error", unblockErr)
		}
	}()

	return nil
}

// isControllerAnalysisJob reports whether job type is handled by the
// controller-side AnalysisRunner.
func isControllerAnalysisJob(jobType string) bool {
	switch jobType {
	case "analysis", "hdr_detect", "audio":
		return true
	}
	return false
}

// computeAdaptiveChunks distributes totalFrames among the given agents
// proportionally to their historical avg_fps.  Each chunk is clamped to
// [minChunk, maxChunk].  Falls back to equal-sized chunks when no agent speed
// data is available.
func computeAdaptiveChunks(agentFPS []float64, totalFrames int) []db.ChunkBoundary {
	const minChunk = 500
	n := len(agentFPS)
	if n == 0 {
		return nil
	}
	maxChunk := totalFrames / 2
	if maxChunk < minChunk {
		maxChunk = minChunk
	}

	// Total FPS sum for weight calculation.
	total := 0.0
	for _, fps := range agentFPS {
		if fps <= 0 {
			fps = 1 // treat unknown/zero as minimum weight
		}
		total += fps
	}

	boundaries := make([]db.ChunkBoundary, 0, n)
	start := 0
	remaining := totalFrames
	for i, fps := range agentFPS {
		if i == n-1 {
			// Give remaining frames to last agent to avoid rounding gaps.
			boundaries = append(boundaries, db.ChunkBoundary{StartFrame: start, EndFrame: start + remaining - 1})
			break
		}
		w := fps
		if w <= 0 {
			w = 1
		}
		size := int(math.Round(float64(totalFrames) * w / total))
		if size < minChunk {
			size = minChunk
		}
		if size > maxChunk {
			size = maxChunk
		}
		if size > remaining-minChunk {
			size = remaining - minChunk
		}
		boundaries = append(boundaries, db.ChunkBoundary{StartFrame: start, EndFrame: start + size - 1})
		start += size
		remaining -= size
	}
	return boundaries
}

// expandEncodeJob creates tasks for a multi-chunk encode job and appends a
// final concat task that merges the chunk outputs once all chunks complete.
func (e *Engine) expandEncodeJob(ctx context.Context, job *db.Job) error {
	// If adaptive chunking is requested, recompute chunk boundaries based on
	// each available agent's historical encoding speed.
	if cc := job.EncodeConfig.ChunkingConfig; cc != nil && cc.AdaptiveChunking {
		agents, err := e.store.ListAgents(ctx)
		if err != nil {
			e.logger.Warn("engine: adaptive chunking: list agents failed, using fixed boundaries",
				"job_id", job.ID, "error", err)
		} else {
			var fpsList []float64
			for _, a := range agents {
				if a.Status != "idle" && a.Status != "busy" {
					continue
				}
				fps, err := e.store.GetAgentAvgFPS(ctx, a.ID)
				if err != nil {
					fps = 0
				}
				fpsList = append(fpsList, fps)
			}
			if len(fpsList) > 0 {
				// Determine total frames from existing boundaries.
				totalFrames := 0
				for _, b := range job.EncodeConfig.ChunkBoundaries {
					if b.EndFrame+1 > totalFrames {
						totalFrames = b.EndFrame + 1
					}
				}
				if totalFrames == 0 && cc.ChunkSizeFrames > 0 && len(job.EncodeConfig.ChunkBoundaries) > 0 {
					totalFrames = job.EncodeConfig.ChunkBoundaries[len(job.EncodeConfig.ChunkBoundaries)-1].EndFrame + 1
				}
				if totalFrames > 0 {
					newBounds := computeAdaptiveChunks(fpsList, totalFrames)
					if len(newBounds) > 0 {
						job.EncodeConfig.ChunkBoundaries = newBounds
						e.logger.Info("engine: adaptive chunking applied",
							"job_id", job.ID,
							"chunks", len(newBounds),
							"agents", len(fpsList),
						)
					}
				}
			}
		}
	}

	if len(job.EncodeConfig.ChunkBoundaries) == 0 {
		e.logger.Error("engine: encode job has no chunk boundaries, skipping",
			"job_id", job.ID,
		)
		return nil
	}

	source, err := e.store.GetSourceByID(ctx, job.SourceID)
	if err != nil {
		return fmt.Errorf("engine: get source %s: %w", job.SourceID, err)
	}

	ext := job.EncodeConfig.OutputExtension
	if ext == "" {
		ext = "mkv"
	}

	// failJob deletes any tasks already created for this job, then marks it failed.
	// This prevents orphan tasks when expansion only partially succeeds.
	failJob := func(cause error) error {
		if delErr := e.store.DeleteTasksByJobID(ctx, job.ID); delErr != nil {
			e.logger.Error("engine: cleanup orphan tasks failed",
				"job_id", job.ID, "error", delErr)
		}
		_ = e.store.UpdateJobStatus(ctx, job.ID, "failed")
		return cause
	}

	chunkPaths := make([]string, len(job.EncodeConfig.ChunkBoundaries))

	// Create a task for each chunk and render its scripts.
	for i := range job.EncodeConfig.ChunkBoundaries {
		// Build output path using string concatenation to preserve UNC prefix.
		outputPath := job.EncodeConfig.OutputRoot + `\` + job.ID + fmt.Sprintf(`\chunk_%04d.%s`, i, ext)
		chunkPaths[i] = outputPath

		task, err := e.store.CreateTask(ctx, db.CreateTaskParams{
			JobID:      job.ID,
			ChunkIndex: i,
			TaskType:   db.TaskTypeEncode,
			SourcePath: source.UNCPath,
			OutputPath: outputPath,
			Variables:  map[string]string{},
		})
		if err != nil {
			return failJob(fmt.Errorf("engine: create task chunk %d: %w", i, err))
		}

		scriptDir, err := e.gen.Render(ctx, job, task, source)
		if err != nil {
			return failJob(fmt.Errorf("engine: render scripts chunk %d: %w", i, err))
		}

		if err := e.store.SetTaskScriptDir(ctx, task.ID, scriptDir); err != nil {
			return failJob(fmt.Errorf("engine: set script dir chunk %d: %w", i, err))
		}
	}

	// Append a concat task that merges all chunk outputs into the final output
	// path.  The scheduler will not dispatch it until every non-concat sibling
	// task in the same job has reached a terminal state.
	finalOutput := job.EncodeConfig.OutputRoot + `\` + job.ID + fmt.Sprintf(`\output.%s`, ext)
	concatChunkIndex := len(job.EncodeConfig.ChunkBoundaries) // place after all chunk tasks

	concatTask, err := e.store.CreateTask(ctx, db.CreateTaskParams{
		JobID:      job.ID,
		ChunkIndex: concatChunkIndex,
		TaskType:   db.TaskTypeConcat,
		SourcePath: source.UNCPath,
		OutputPath: finalOutput,
		Variables:  map[string]string{},
	})
	if err != nil {
		_ = e.store.UpdateJobStatus(ctx, job.ID, "failed")
		return fmt.Errorf("engine: create concat task: %w", err)
	}

	// Only generate agent-side concat scripts if no controller-side runner.
	if e.concat == nil {
		concatScriptDir, err := e.gen.RenderConcat(ctx, job, concatTask, chunkPaths, finalOutput)
		if err != nil {
			_ = e.store.UpdateJobStatus(ctx, job.ID, "failed")
			return fmt.Errorf("engine: render concat scripts: %w", err)
		}

		if err := e.store.SetTaskScriptDir(ctx, concatTask.ID, concatScriptDir); err != nil {
			_ = e.store.UpdateJobStatus(ctx, job.ID, "failed")
			return fmt.Errorf("engine: set script dir for concat task: %w", err)
		}
	}

	// All tasks created successfully — update counters and keep status as queued.
	if err := e.store.UpdateJobTaskCounts(ctx, job.ID); err != nil {
		return fmt.Errorf("engine: update task counts for job %s: %w", job.ID, err)
	}
	if err := e.store.UpdateJobStatus(ctx, job.ID, "queued"); err != nil {
		return fmt.Errorf("engine: update job status for job %s: %w", job.ID, err)
	}
	return nil
}

// expandMergeJob creates a single merge task that combines a video file from
// a completed encode job with an audio file from a completed audio job.
// The video and audio output paths are passed via encode_config.extra_vars
// (keys: "video_path" and "audio_path"). The merged output goes to
// encode_config.output_root + "/" + job.ID + "/merged.mkv".
func (e *Engine) expandMergeJob(ctx context.Context, job *db.Job) error {
	source, err := e.store.GetSourceByID(ctx, job.SourceID)
	if err != nil {
		return fmt.Errorf("engine: merge job get source %s: %w", job.SourceID, err)
	}

	ext := job.EncodeConfig.OutputExtension
	if ext == "" {
		ext = "mkv"
	}
	outputPath := job.EncodeConfig.OutputRoot + `\` + job.ID + fmt.Sprintf(`\merged.%s`, ext)

	vars := make(map[string]string)
	for k, v := range job.EncodeConfig.ExtraVars {
		vars[k] = v
	}
	vars["merge_output"] = outputPath

	failJob := func(cause error) error {
		if delErr := e.store.DeleteTasksByJobID(ctx, job.ID); delErr != nil {
			e.logger.Error("engine: cleanup orphan tasks (merge)",
				"job_id", job.ID, "error", delErr)
		}
		_ = e.store.UpdateJobStatus(ctx, job.ID, "failed")
		return cause
	}

	task, err := e.store.CreateTask(ctx, db.CreateTaskParams{
		JobID:      job.ID,
		ChunkIndex: 0,
		SourcePath: source.UNCPath,
		OutputPath: outputPath,
		Variables:  vars,
	})
	if err != nil {
		return failJob(fmt.Errorf("engine: create merge task: %w", err))
	}

	scriptDir, err := e.gen.RenderSingle(ctx, job, task, source)
	if err != nil {
		return failJob(fmt.Errorf("engine: render merge task scripts: %w", err))
	}

	if err := e.store.SetTaskScriptDir(ctx, task.ID, scriptDir); err != nil {
		return failJob(fmt.Errorf("engine: set script dir for merge task: %w", err))
	}

	if err := e.store.UpdateJobTaskCounts(ctx, job.ID); err != nil {
		return fmt.Errorf("engine: update task counts for merge job %s: %w", job.ID, err)
	}
	if err := e.store.UpdateJobStatus(ctx, job.ID, "queued"); err != nil {
		return fmt.Errorf("engine: update job status for merge job %s: %w", job.ID, err)
	}
	return nil
}

// expandSingleTaskJob creates a single task for analysis or audio jobs.
func (e *Engine) expandSingleTaskJob(ctx context.Context, job *db.Job) error {
	source, err := e.store.GetSourceByID(ctx, job.SourceID)
	if err != nil {
		return fmt.Errorf("engine: get source %s: %w", job.SourceID, err)
	}

	// failJob deletes any tasks already created for this job, then marks it
	// failed, preventing orphan tasks when expansion only partially succeeds.
	failJob := func(cause error) error {
		if delErr := e.store.DeleteTasksByJobID(ctx, job.ID); delErr != nil {
			e.logger.Error("engine: cleanup orphan tasks failed",
				"job_id", job.ID, "error", delErr)
		}
		_ = e.store.UpdateJobStatus(ctx, job.ID, "failed")
		return cause
	}

	task, err := e.store.CreateTask(ctx, db.CreateTaskParams{
		JobID:      job.ID,
		ChunkIndex: 0,
		SourcePath: source.UNCPath,
		Variables:  map[string]string{},
	})
	if err != nil {
		return failJob(fmt.Errorf("engine: create single task: %w", err))
	}

	scriptDir, err := e.gen.RenderSingle(ctx, job, task, source)
	if err != nil {
		return failJob(fmt.Errorf("engine: render single task scripts: %w", err))
	}

	if err := e.store.SetTaskScriptDir(ctx, task.ID, scriptDir); err != nil {
		return failJob(fmt.Errorf("engine: set script dir for single task: %w", err))
	}

	if err := e.store.UpdateJobTaskCounts(ctx, job.ID); err != nil {
		return fmt.Errorf("engine: update task counts for job %s: %w", job.ID, err)
	}
	if err := e.store.UpdateJobStatus(ctx, job.ID, "queued"); err != nil {
		return fmt.Errorf("engine: update job status for job %s: %w", job.ID, err)
	}
	return nil
}
