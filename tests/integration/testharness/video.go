//go:build integration

package testharness

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const (
	// testVideoCachePath is the well-known path for caching the generated
	// test video across test runs.
	testVideoCachePath = "/tmp/encodeswarmr-test-video/test_input.mkv"
)

// DownloadTestVideo generates a short synthetic test video using ffmpeg and
// returns the absolute path to the file.
//
// The file is cached at testVideoCachePath between test runs. If the cached
// copy already exists and is non-empty it is used directly.
//
// The returned path points to a copy inside t.TempDir() so that each test
// gets its own writable copy and cleanup is automatic.
//
// This replaces the previous approach of downloading from external mirrors,
// which was unreliable in CI due to third-party server outages.
func DownloadTestVideo(t *testing.T) string {
	t.Helper()

	// Ensure cache directory exists.
	cacheDir := filepath.Dir(testVideoCachePath)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("video: create cache dir %s: %v", cacheDir, err)
	}

	// Check whether a cached copy is already available and non-empty.
	if info, err := os.Stat(testVideoCachePath); err == nil && info.Size() > 0 {
		t.Logf("video: using cached test video at %s (%d bytes)", testVideoCachePath, info.Size())
	} else {
		// Generate a 5-second 720p test video with ffmpeg using the
		// testsrc2 pattern generator — no network access needed.
		t.Log("video: generating synthetic test video with ffmpeg")
		cmd := exec.Command("ffmpeg", "-y",
			"-f", "lavfi", "-i", "testsrc2=duration=5:size=1280x720:rate=30",
			"-f", "lavfi", "-i", "sine=frequency=440:duration=5",
			"-c:v", "libx264", "-preset", "ultrafast", "-crf", "28",
			"-c:a", "aac", "-b:a", "64k",
			"-pix_fmt", "yuv420p",
			testVideoCachePath,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("video: ffmpeg generate failed: %v\n%s", err, out)
		}
		info, err := os.Stat(testVideoCachePath)
		if err != nil {
			t.Fatalf("video: stat generated file: %v", err)
		}
		t.Logf("video: generated %d bytes at %s", info.Size(), testVideoCachePath)
	}

	// Copy the cached file into the test's temp directory so each test has
	// its own isolated copy and t.Cleanup handles removal automatically.
	destDir := t.TempDir()
	dest := filepath.Join(destDir, "source.mkv")

	if err := copyFile(testVideoCachePath, dest); err != nil {
		t.Fatalf("video: copy cached video to temp dir: %v", err)
	}

	return dest
}

// copyFile copies src to dst, creating dst if it does not exist.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create dst %s: %w", dst, err)
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copy %s → %s: %w", src, dst, err)
	}

	return out.Close()
}
