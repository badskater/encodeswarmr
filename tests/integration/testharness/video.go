//go:build integration

package testharness

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

const (
	// testVideoCachePath is the well-known path for caching the test video
	// across test runs to avoid redundant downloads.
	testVideoCachePath = "/tmp/encodeswarmr-test-video/big_buck_bunny_1080_10s.mkv"
)

// testVideoURLs lists Big Buck Bunny mirrors in preference order.
// If the primary source is down (HTTP 522, timeout, etc.) the next is tried.
var testVideoURLs = []string{
	"https://test-videos.co.uk/vids/bigbuckbunny/mkv/1080/Big_Buck_Bunny_1080_10s_5MB.mkv",
	"https://sample-videos.com/video321/mkv/720/big_buck_bunny_720_1mb.mkv",
	"https://filesamples.com/samples/video/mkv/sample_960x400_ocean_with_audio.mkv",
}

// DownloadTestVideo downloads the Big Buck Bunny test clip to a temp directory
// and returns the absolute path to the file.
//
// The file is cached at testVideoCachePath between test runs. If the cached
// copy already exists and is non-empty it is used directly. This avoids
// re-downloading the file on every test run, which is important in CI where
// bandwidth is shared.
//
// The returned path points to a copy inside t.TempDir() so that each test
// gets its own writable copy and cleanup is automatic.
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
		// Cache miss — try each mirror until one succeeds.
		var dlErr error
		for _, u := range testVideoURLs {
			t.Logf("video: downloading test video from %s", u)
			if dlErr = downloadFile(u, testVideoCachePath); dlErr == nil {
				break
			}
			t.Logf("video: mirror failed: %v, trying next", dlErr)
		}
		if dlErr != nil {
			t.Fatalf("video: all mirrors failed, last error: %v", dlErr)
		}
		info, err := os.Stat(testVideoCachePath)
		if err != nil {
			t.Fatalf("video: stat cached file after download: %v", err)
		}
		t.Logf("video: downloaded %d bytes to %s", info.Size(), testVideoCachePath)
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

// downloadFile fetches url and writes the response body to dest.
// It overwrites dest if it already exists. On error the partially-written
// file is removed to avoid leaving a corrupt cache entry.
func downloadFile(url, dest string) error {
	resp, err := http.Get(url) //nolint:gosec,noctx
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: unexpected status %d", url, resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(dest) //nolint:errcheck // best-effort cleanup on error
		return fmt.Errorf("write %s: %w", dest, err)
	}

	return f.Close()
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
