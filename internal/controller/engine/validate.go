package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

// ValidationConfig holds the configuration for post-encode output validation.
type ValidationConfig struct {
	// Enabled enables or disables post-encode validation. Defaults to true.
	Enabled bool
	// FFprobeBin is the path to the ffprobe binary. Defaults to "ffprobe" (PATH).
	FFprobeBin string
	// MinDurationRatio is the minimum acceptable ratio of output duration to
	// source duration. Defaults to 0.9 (output must be at least 90% of source).
	MinDurationRatio float64
}

// DefaultValidationConfig returns a ValidationConfig with default settings.
func DefaultValidationConfig() ValidationConfig {
	return ValidationConfig{
		Enabled:          true,
		FFprobeBin:       "ffprobe",
		MinDurationRatio: 0.9,
	}
}

// ValidationResult holds the result of a post-encode output validation check.
type ValidationResult struct {
	// OK is true when all validation checks passed.
	OK bool `json:"ok"`
	// FailureReason is a human-readable explanation of why validation failed.
	// Empty when OK is true.
	FailureReason string `json:"failure_reason,omitempty"`
	// Codec is the detected video codec from the output file.
	Codec string `json:"codec,omitempty"`
	// DurationSec is the detected duration of the output file in seconds.
	DurationSec float64 `json:"duration_sec,omitempty"`
	// ValidatedAt is the time the validation was performed.
	ValidatedAt time.Time `json:"validated_at"`
}

// ffprobeOutput mirrors the parts of ffprobe -print_format json output we need.
type ffprobeOutput struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// ValidateOutput runs ffprobe on the output file at outputPath and validates
// that it meets the configured requirements. When sourceDurationSec is > 0
// and cfg.MinDurationRatio > 0, the output duration is compared against the
// source duration. expectedCodec may be empty to skip codec matching.
func ValidateOutput(ctx context.Context, cfg ValidationConfig, outputPath, expectedCodec string, sourceDurationSec float64, logger *slog.Logger) ValidationResult {
	result := ValidationResult{
		ValidatedAt: time.Now(),
	}

	if !cfg.Enabled {
		result.OK = true
		return result
	}

	bin := cfg.FFprobeBin
	if bin == "" {
		bin = "ffprobe"
	}

	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		outputPath,
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		result.FailureReason = fmt.Sprintf("ffprobe failed: %v", err)
		logger.Warn("output validation: ffprobe error",
			slog.String("output_path", outputPath),
			slog.String("error", err.Error()),
			slog.String("stderr", stderr.String()),
		)
		return result
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(stdout.Bytes(), &probe); err != nil {
		result.FailureReason = fmt.Sprintf("ffprobe output parse failed: %v", err)
		return result
	}

	// Check that at least one video stream is present.
	var videoCodec string
	for _, s := range probe.Streams {
		if s.CodecType == "video" {
			videoCodec = s.CodecName
			break
		}
	}
	if videoCodec == "" {
		result.FailureReason = "no video stream found in output"
		return result
	}
	result.Codec = videoCodec

	// Check codec matches expected when specified.
	if expectedCodec != "" && videoCodec != expectedCodec {
		result.FailureReason = fmt.Sprintf("codec mismatch: expected %q, got %q", expectedCodec, videoCodec)
		return result
	}

	// Parse duration from format section.
	if probe.Format.Duration != "" {
		var dur float64
		if _, err := fmt.Sscanf(probe.Format.Duration, "%f", &dur); err == nil {
			result.DurationSec = dur
		}
	}

	// Validate duration > 0.
	if result.DurationSec <= 0 {
		result.FailureReason = "output duration is zero or missing"
		return result
	}

	// Validate output duration is at least MinDurationRatio of source duration.
	if sourceDurationSec > 0 && cfg.MinDurationRatio > 0 {
		ratio := result.DurationSec / sourceDurationSec
		if ratio < cfg.MinDurationRatio {
			result.FailureReason = fmt.Sprintf(
				"output duration %.2fs is less than %.0f%% of source duration %.2fs (ratio %.3f)",
				result.DurationSec,
				cfg.MinDurationRatio*100,
				sourceDurationSec,
				ratio,
			)
			return result
		}
	}

	result.OK = true
	logger.Info("output validation passed",
		slog.String("output_path", outputPath),
		slog.String("codec", videoCodec),
		slog.Float64("duration_sec", result.DurationSec),
	)
	return result
}
