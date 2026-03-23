package analysis

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ThumbnailPositions defines the relative positions (as fractions of duration)
// at which individual thumbnails are captured.
var ThumbnailPositions = []float64{0.10, 0.25, 0.50, 0.75, 0.90}

// GenerateThumbnails produces a set of preview images for sourcePath and writes
// them into outputDir.  It returns the relative paths (relative to outputDir)
// of the generated files.
//
// Two passes are run:
//  1. A 5-frame tile strip saved as "strip.jpg" using ffmpeg's tile filter.
//  2. Individual frames at ThumbnailPositions saved as "thumb_N.jpg".
//
// The caller is responsible for creating outputDir before calling this function.
func (r *Runner) GenerateThumbnails(ctx context.Context, sourcePath, outputDir string, count int) ([]string, error) {
	if count <= 0 {
		count = len(ThumbnailPositions)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("thumbnails: mkdir %s: %w", outputDir, err)
	}

	// Probe the duration so we can compute absolute timestamps.
	duration, err := r.probeDuration(ctx, sourcePath)
	if err != nil {
		return nil, fmt.Errorf("thumbnails: probe duration: %w", err)
	}
	if duration <= 0 {
		return nil, fmt.Errorf("thumbnails: source has zero duration")
	}

	var paths []string

	// --- 1. tile strip (5 representative frames in a single image) ---
	stripPath := filepath.Join(outputDir, "strip.jpg")
	if err := r.generateStrip(ctx, sourcePath, stripPath); err != nil {
		r.logger.Warn("thumbnails: strip generation failed", "error", err, "path", sourcePath)
		// Non-fatal — continue with individual thumbnails.
	} else {
		paths = append(paths, "strip.jpg")
	}

	// --- 2. individual frames at percentage positions ---
	positions := ThumbnailPositions
	if count > 0 && count != len(ThumbnailPositions) {
		// Build evenly-spaced positions if a custom count was requested.
		positions = make([]float64, count)
		for i := range positions {
			positions[i] = float64(i+1) / float64(count+1)
		}
	}

	for i, frac := range positions {
		ts := frac * duration
		outFile := fmt.Sprintf("thumb_%d.jpg", i)
		outPath := filepath.Join(outputDir, outFile)
		if err := r.captureFrame(ctx, sourcePath, ts, outPath); err != nil {
			r.logger.Warn("thumbnails: frame capture failed",
				"error", err, "position", frac, "ts", ts)
			continue
		}
		paths = append(paths, outFile)
	}

	return paths, nil
}

// generateStrip runs ffmpeg's thumbnail+tile filter to produce a 5-column
// contact sheet saved as a JPEG.
func (r *Runner) generateStrip(ctx context.Context, sourcePath, outputPath string) error {
	cmd := exec.CommandContext(ctx, r.ffmpegBin,
		"-y",
		"-i", sourcePath,
		"-vf", "thumbnail,scale=320:-1,tile=5x1",
		"-frames:v", "1",
		outputPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg strip: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

// captureFrame seeks to ts seconds and captures a single JPEG frame.
func (r *Runner) captureFrame(ctx context.Context, sourcePath string, ts float64, outputPath string) error {
	tsStr := strconv.FormatFloat(ts, 'f', 3, 64)
	cmd := exec.CommandContext(ctx, r.ffmpegBin,
		"-y",
		"-ss", tsStr,
		"-i", sourcePath,
		"-vf", "scale=320:-1",
		"-frames:v", "1",
		"-q:v", "3",
		outputPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg frame: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

// probeDuration returns the duration of sourcePath in seconds using ffprobe.
func (r *Runner) probeDuration(ctx context.Context, sourcePath string) (float64, error) {
	cmd := exec.CommandContext(ctx, r.ffprobeBin,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		sourcePath,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("ffprobe: %w (stderr: %s)", err, stderr.String())
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(stdout.String()), 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", stdout.String(), err)
	}
	return d, nil
}
