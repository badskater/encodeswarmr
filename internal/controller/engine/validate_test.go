package engine

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// writeFakeFFprobe writes a shell script (or .bat stub) that prints the
// provided JSON to stdout and exits 0.  Returns the path to the fake binary.
// On Windows this creates a .bat file; on Linux/macOS a shell script.
func writeFakeFFprobe(t *testing.T, jsonOutput string) string {
	t.Helper()
	dir := t.TempDir()
	// Write a small Go helper that outputs json and exits.
	// We use an executable that is guaranteed to work on the test host.
	// Strategy: write the json to a temp file, then use "cat" / "type"
	// depending on the platform.  However, since tests run on Linux in CI
	// and the test environment does not require a real ffprobe, we test
	// the parsing logic by exercising ValidateOutput against a fake binary
	// built with os/exec or by testing the inner parsing function directly.
	//
	// For portability, we use a small Go program compiled via go test's
	// TempDir.  Instead of that, we directly test the parsing logic by
	// calling validateOutputFromJSON (unexported) when available.
	//
	// Since ValidateOutput shells out to ffprobe, and we cannot guarantee
	// a real ffprobe is present, we test the function with a fake binary
	// that we create as a shell script on Unix or a .bat on Windows.
	jsonFile := filepath.Join(dir, "output.json")
	if err := os.WriteFile(jsonFile, []byte(jsonOutput), 0600); err != nil {
		t.Fatalf("write fake json: %v", err)
	}

	script := filepath.Join(dir, "ffprobe")
	content := "#!/bin/sh\ncat " + jsonFile + "\n"
	if err := os.WriteFile(script, []byte(content), 0700); err != nil {
		t.Fatalf("write fake ffprobe: %v", err)
	}
	return script
}

// validProbeJSON returns valid ffprobe JSON with a video stream and duration.
func validProbeJSON(codec, duration string) string {
	return `{
		"streams": [
			{"codec_type": "video", "codec_name": "` + codec + `"},
			{"codec_type": "audio", "codec_name": "flac"}
		],
		"format": {"duration": "` + duration + `"}
	}`
}

// ---------------------------------------------------------------------------
// TestValidateOutput_Disabled
// ---------------------------------------------------------------------------

func TestValidateOutput_Disabled(t *testing.T) {
	cfg := ValidationConfig{Enabled: false}
	result := ValidateOutput(context.Background(), cfg, "/nonexistent/file.mkv", "", 0, discardLogger())
	if !result.OK {
		t.Errorf("disabled validation: OK = false, want true")
	}
	if result.FailureReason != "" {
		t.Errorf("disabled validation: FailureReason = %q, want empty", result.FailureReason)
	}
}

// ---------------------------------------------------------------------------
// TestValidateOutput_FFprobeFails
// ---------------------------------------------------------------------------

func TestValidateOutput_FFprobeFails(t *testing.T) {
	cfg := ValidationConfig{
		Enabled:    true,
		FFprobeBin: "/nonexistent/ffprobe",
	}
	result := ValidateOutput(context.Background(), cfg, "/some/file.mkv", "", 0, discardLogger())
	if result.OK {
		t.Error("expected OK = false when ffprobe binary is missing")
	}
	if result.FailureReason == "" {
		t.Error("expected FailureReason to be set")
	}
}

// ---------------------------------------------------------------------------
// TestValidateOutput_NoVideoStream  (uses fake ffprobe)
// ---------------------------------------------------------------------------

func TestValidateOutput_NoVideoStream(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake ffprobe not supported on Windows")
	}

	probe := writeFakeFFprobe(t, `{
		"streams": [
			{"codec_type": "audio", "codec_name": "flac"}
		],
		"format": {"duration": "3600.0"}
	}`)

	cfg := ValidationConfig{
		Enabled:    true,
		FFprobeBin: probe,
	}
	result := ValidateOutput(context.Background(), cfg, "/any.mkv", "", 0, discardLogger())
	if result.OK {
		t.Error("expected OK = false when no video stream")
	}
	if result.FailureReason == "" {
		t.Error("expected FailureReason to be set")
	}
}

// ---------------------------------------------------------------------------
// TestValidateOutput_DurationTooShort (uses fake ffprobe)
// ---------------------------------------------------------------------------

func TestValidateOutput_DurationTooShort(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake ffprobe not supported on Windows")
	}

	// Source is 3600s, output is only 1800s → ratio 0.5 < 0.9 minimum.
	probe := writeFakeFFprobe(t, validProbeJSON("hevc", "1800.0"))

	cfg := ValidationConfig{
		Enabled:          true,
		FFprobeBin:       probe,
		MinDurationRatio: 0.9,
	}
	result := ValidateOutput(context.Background(), cfg, "/any.mkv", "", 3600.0, discardLogger())
	if result.OK {
		t.Error("expected OK = false when output duration is too short")
	}
	if result.FailureReason == "" {
		t.Error("expected FailureReason to be set")
	}
}

// ---------------------------------------------------------------------------
// TestValidateOutput_AllChecksPass (uses fake ffprobe)
// ---------------------------------------------------------------------------

func TestValidateOutput_AllChecksPass(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake ffprobe not supported on Windows")
	}

	// Output is 3500s out of 3600s source → ratio 0.972 > 0.9.
	probe := writeFakeFFprobe(t, validProbeJSON("hevc", "3500.0"))

	cfg := ValidationConfig{
		Enabled:          true,
		FFprobeBin:       probe,
		MinDurationRatio: 0.9,
	}
	result := ValidateOutput(context.Background(), cfg, "/any.mkv", "hevc", 3600.0, discardLogger())
	if !result.OK {
		t.Errorf("expected OK = true; FailureReason = %q", result.FailureReason)
	}
	if result.Codec != "hevc" {
		t.Errorf("Codec = %q, want hevc", result.Codec)
	}
}

// ---------------------------------------------------------------------------
// TestValidateOutput_CodecMismatch (uses fake ffprobe)
// ---------------------------------------------------------------------------

func TestValidateOutput_CodecMismatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake ffprobe not supported on Windows")
	}

	probe := writeFakeFFprobe(t, validProbeJSON("h264", "3500.0"))

	cfg := ValidationConfig{
		Enabled:          true,
		FFprobeBin:       probe,
		MinDurationRatio: 0.9,
	}
	// Expect hevc but ffprobe reports h264.
	result := ValidateOutput(context.Background(), cfg, "/any.mkv", "hevc", 3600.0, discardLogger())
	if result.OK {
		t.Error("expected OK = false when codec mismatches")
	}
}

// ---------------------------------------------------------------------------
// TestDefaultValidationConfig
// ---------------------------------------------------------------------------

func TestDefaultValidationConfig(t *testing.T) {
	cfg := DefaultValidationConfig()
	if !cfg.Enabled {
		t.Error("default: Enabled = false, want true")
	}
	if cfg.FFprobeBin != "ffprobe" {
		t.Errorf("default: FFprobeBin = %q, want ffprobe", cfg.FFprobeBin)
	}
	if cfg.MinDurationRatio != 0.9 {
		t.Errorf("default: MinDurationRatio = %f, want 0.9", cfg.MinDurationRatio)
	}
}
