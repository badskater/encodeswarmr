package engine

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/badskater/encodeswarmr/internal/db"
)

// VMAFTargetRunner executes an iterative VMAF-targeted CRF encode on the
// controller host.  It performs a binary search over CRF values, encoding
// and measuring VMAF after each iteration until the target score is reached
// or the maximum number of iterations is exhausted.
//
// It is wired into the engine via SetVMAFTargetRunner.
type VMAFTargetRunner interface {
	RunVMAFTarget(ctx context.Context, job *db.Job, task *db.Task, cfg VMAFTargetConfig) error
}

// VMAFTargetConfig carries the parameters extracted from a flow node's config
// map for an encode_vmaf_target node.
type VMAFTargetConfig struct {
	// TargetVMAF is the desired VMAF score (default 95, range 80-100).
	TargetVMAF float64
	// Codec is "x265" or "x264" (default "x265").
	Codec string
	// LibCodec is the ffmpeg library name derived from Codec ("libx265"/"libx264").
	LibCodec string
	// Preset is the encoder speed preset (default "slow").
	Preset string
	// CRFMin is the best-quality (lowest CRF) bound of the search range.
	CRFMin float64
	// CRFMax is the smallest-file (highest CRF) bound of the search range.
	CRFMax float64
	// MaxIterations caps the binary search loop (default 5).
	MaxIterations int
	// VMafModel is the VMAF model file name (default "vmaf_v0.6.1").
	VMafModel string
	// OutputPath is where the final encode is written.
	OutputPath string
	// WorkDir is used for intermediate files.
	WorkDir string
}

// VMAFTargetConfigFromVars extracts a VMAFTargetConfig from a task's Variables
// map (as populated by the flow expander).
func VMAFTargetConfigFromVars(vars map[string]string, jobID string, outputRoot string) VMAFTargetConfig {
	cfg := VMAFTargetConfig{
		TargetVMAF:    95,
		Codec:         "x265",
		LibCodec:      "libx265",
		Preset:        "slow",
		CRFMin:        15,
		CRFMax:        28,
		MaxIterations: 5,
		VMafModel:     "vmaf_v0.6.1",
		WorkDir:       filepath.Join(outputRoot, jobID, "vmaf_target"),
	}

	if v := vars["target_vmaf"]; v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.TargetVMAF = f
		}
	}
	if v := vars["codec"]; v != "" {
		cfg.Codec = v
		if v == "x264" {
			cfg.LibCodec = "libx264"
		}
	}
	if v := vars["preset"]; v != "" {
		cfg.Preset = v
	}
	if v := vars["crf_min"]; v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.CRFMin = f
		}
	}
	if v := vars["crf_max"]; v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.CRFMax = f
		}
	}
	if v := vars["max_iterations"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxIterations = n
		}
	}
	if v := vars["vmaf_model"]; v != "" {
		cfg.VMafModel = v
	}
	cfg.OutputPath = filepath.Join(cfg.WorkDir, "output.mkv")
	return cfg
}

// ---------------------------------------------------------------------------
// Default controller-side VMAF target runner
// ---------------------------------------------------------------------------

// controllerVMAFRunner is the built-in VMAFTargetRunner that runs ffmpeg
// directly on the controller host.
type controllerVMAFRunner struct {
	ffmpegBin string
	logger    *slog.Logger
}

// NewControllerVMAFRunner creates a VMAFTargetRunner that uses the given
// ffmpeg binary (defaults to "ffmpeg" when empty).
func NewControllerVMAFRunner(ffmpegBin string, logger *slog.Logger) VMAFTargetRunner {
	if ffmpegBin == "" {
		ffmpegBin = "ffmpeg"
	}
	return &controllerVMAFRunner{ffmpegBin: ffmpegBin, logger: logger}
}

// RunVMAFTarget executes the iterative VMAF binary-search encode loop.
func (r *controllerVMAFRunner) RunVMAFTarget(ctx context.Context, job *db.Job, task *db.Task, cfg VMAFTargetConfig) error {
	if err := os.MkdirAll(cfg.WorkDir, 0o755); err != nil {
		return fmt.Errorf("vmaf_target: mkdir work dir: %w", err)
	}

	lo := cfg.CRFMin
	hi := cfg.CRFMax
	bestOutput := ""
	bestCRF := math.Round((lo + hi) / 2)

	r.logger.Info("vmaf_target: starting binary search",
		slog.String("job_id", job.ID),
		slog.String("task_id", task.ID),
		slog.Float64("target_vmaf", cfg.TargetVMAF),
		slog.Float64("crf_min", lo),
		slog.Float64("crf_max", hi),
		slog.Int("max_iterations", cfg.MaxIterations),
	)

	for iter := 0; iter < cfg.MaxIterations; iter++ {
		crf := math.Round((lo + hi) / 2)
		crfStr := strconv.FormatFloat(crf, 'f', 0, 64)
		iterOutput := filepath.Join(cfg.WorkDir, fmt.Sprintf("iter_%02d_crf%s.mkv", iter, crfStr))

		r.logger.Info("vmaf_target: iteration encode",
			slog.String("job_id", job.ID),
			slog.Int("iteration", iter+1),
			slog.Float64("crf", crf),
		)

		if err := r.encode(ctx, task.SourcePath, iterOutput, cfg.LibCodec, crfStr, cfg.Preset); err != nil {
			return fmt.Errorf("vmaf_target: encode iteration %d (crf %.0f): %w", iter, crf, err)
		}

		vmaf, err := r.measureVMAF(ctx, task.SourcePath, iterOutput, cfg.VMafModel)
		if err != nil {
			return fmt.Errorf("vmaf_target: measure vmaf iteration %d (crf %.0f): %w", iter, crf, err)
		}

		r.logger.Info("vmaf_target: iteration result",
			slog.String("job_id", job.ID),
			slog.Int("iteration", iter+1),
			slog.Float64("crf", crf),
			slog.Float64("vmaf", vmaf),
			slog.Float64("target", cfg.TargetVMAF),
		)

		bestOutput = iterOutput
		bestCRF = crf

		if math.Abs(vmaf-cfg.TargetVMAF) < 0.5 {
			// Close enough — stop early.
			r.logger.Info("vmaf_target: target reached early",
				slog.String("job_id", job.ID),
				slog.Float64("crf", crf),
				slog.Float64("vmaf", vmaf),
			)
			break
		}

		if vmaf >= cfg.TargetVMAF {
			// Quality exceeds target — try a higher CRF (worse quality, smaller file).
			lo = crf + 1
		} else {
			// Quality below target — try a lower CRF (better quality, larger file).
			hi = crf - 1
		}

		if lo > hi {
			// Search space exhausted — best so far is good enough.
			break
		}
	}

	if bestOutput == "" {
		return fmt.Errorf("vmaf_target: no output produced after %d iterations", cfg.MaxIterations)
	}

	// Move the best output to the final output path.
	if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0o755); err != nil {
		return fmt.Errorf("vmaf_target: mkdir output dir: %w", err)
	}
	if err := os.Rename(bestOutput, cfg.OutputPath); err != nil {
		return fmt.Errorf("vmaf_target: move final output (crf %.0f): %w", bestCRF, err)
	}

	r.logger.Info("vmaf_target: completed",
		slog.String("job_id", job.ID),
		slog.String("task_id", task.ID),
		slog.Float64("final_crf", bestCRF),
		slog.String("output", cfg.OutputPath),
	)

	return nil
}

// encode runs a single CRF-based ffmpeg encode pass.
func (r *controllerVMAFRunner) encode(ctx context.Context, input, output, libCodec, crf, preset string) error {
	args := []string{
		"-y", "-i", input,
		"-c:v", libCodec,
		"-crf", crf,
		"-preset", preset,
		"-an",
		output,
	}
	cmd := exec.CommandContext(ctx, r.ffmpegBin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg encode crf %s: %w (stderr: %s)", crf, err, stderr.String())
	}
	return nil
}

// vmafScoreRe matches the VMAF mean score line produced by ffmpeg's libvmaf.
// Example: "[libvmaf] VMAF score: 96.123456"
var vmafScoreRe = regexp.MustCompile(`(?i)VMAF score[:\s]+([0-9]+(?:\.[0-9]+)?)`)

// measureVMAF runs ffmpeg with the libvmaf filter and parses the mean score.
func (r *controllerVMAFRunner) measureVMAF(ctx context.Context, reference, distorted, model string) (float64, error) {
	// Build the libvmaf filter string. The model name must not include the
	// path separator — it is looked up from ffmpeg's built-in model store.
	vmafFilter := fmt.Sprintf(
		"[0:v][1:v]libvmaf=model_path=%s:log_fmt=xml:log_path=/dev/null",
		model,
	)
	args := []string{
		"-i", distorted,
		"-i", reference,
		"-lavfi", vmafFilter,
		"-f", "null", "/dev/null",
	}
	cmd := exec.CommandContext(ctx, r.ffmpegBin, args...)
	var combined bytes.Buffer
	cmd.Stderr = &combined
	cmd.Stdout = &combined

	// ffmpeg exits non-zero when /dev/null is used as output on some builds.
	// We check output regardless of the exit code.
	_ = cmd.Run()

	sc := bufio.NewScanner(&combined)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if m := vmafScoreRe.FindStringSubmatch(line); len(m) == 2 {
			score, err := strconv.ParseFloat(m[1], 64)
			if err == nil {
				return score, nil
			}
		}
	}

	return 0, fmt.Errorf("vmaf_target: could not parse VMAF score from ffmpeg output")
}
