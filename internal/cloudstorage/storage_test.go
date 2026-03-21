package cloudstorage

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// parseS3URI
// ---------------------------------------------------------------------------

func TestParseS3URI_Valid(t *testing.T) {
	tests := []struct {
		name       string
		uri        string
		wantBucket string
		wantKey    string
	}{
		{
			name:       "simple key",
			uri:        "s3://my-bucket/path/to/object.mkv",
			wantBucket: "my-bucket",
			wantKey:    "path/to/object.mkv",
		},
		{
			name:       "top-level key",
			uri:        "s3://encoding-bucket/video.mp4",
			wantBucket: "encoding-bucket",
			wantKey:    "video.mp4",
		},
		{
			name:       "deep nested key",
			uri:        "s3://bucket/a/b/c/d/e.mkv",
			wantBucket: "bucket",
			wantKey:    "a/b/c/d/e.mkv",
		},
		{
			name:       "key with dots",
			uri:        "s3://media-bucket/2024/movie.1080p.h265.mkv",
			wantBucket: "media-bucket",
			wantKey:    "2024/movie.1080p.h265.mkv",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bucket, key, err := parseS3URI(tc.uri)
			if err != nil {
				t.Fatalf("parseS3URI(%q): unexpected error: %v", tc.uri, err)
			}
			if bucket != tc.wantBucket {
				t.Errorf("bucket = %q, want %q", bucket, tc.wantBucket)
			}
			if key != tc.wantKey {
				t.Errorf("key = %q, want %q", key, tc.wantKey)
			}
		})
	}
}

func TestParseS3URI_Invalid(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{"no bucket", "s3:///key"},
		{"no key", "s3://bucket/"},
		{"empty bucket and key", "s3:///"},
		{"missing both", "s3://"},
		{"bucket only no slash", "s3://bucket"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseS3URI(tc.uri)
			if err == nil {
				t.Errorf("parseS3URI(%q): expected error, got nil", tc.uri)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseGCSURI
// ---------------------------------------------------------------------------

func TestParseGCSURI_Valid(t *testing.T) {
	tests := []struct {
		name         string
		uri          string
		wantBucket   string
		wantObject   string
	}{
		{
			name:       "simple object",
			uri:        "gs://my-gcs-bucket/videos/source.mkv",
			wantBucket: "my-gcs-bucket",
			wantObject: "videos/source.mkv",
		},
		{
			name:       "top-level object",
			uri:        "gs://raw-video/file.mp4",
			wantBucket: "raw-video",
			wantObject: "file.mp4",
		},
		{
			name:       "nested path",
			uri:        "gs://archive/2024/q1/encode/output.mkv",
			wantBucket: "archive",
			wantObject: "2024/q1/encode/output.mkv",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bucket, object, err := parseGCSURI(tc.uri)
			if err != nil {
				t.Fatalf("parseGCSURI(%q): unexpected error: %v", tc.uri, err)
			}
			if bucket != tc.wantBucket {
				t.Errorf("bucket = %q, want %q", bucket, tc.wantBucket)
			}
			if object != tc.wantObject {
				t.Errorf("object = %q, want %q", object, tc.wantObject)
			}
		})
	}
}

func TestParseGCSURI_Invalid(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{"no bucket", "gs:///object.mkv"},
		{"no object", "gs://bucket/"},
		{"empty", "gs://"},
		{"bucket only", "gs://bucket"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseGCSURI(tc.uri)
			if err == nil {
				t.Errorf("parseGCSURI(%q): expected error, got nil", tc.uri)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseAzureURI
// ---------------------------------------------------------------------------

func TestParseAzureURI_Valid(t *testing.T) {
	tests := []struct {
		name          string
		uri           string
		wantContainer string
		wantBlob      string
	}{
		{
			name:          "simple blob",
			uri:           "az://my-container/path/video.mkv",
			wantContainer: "my-container",
			wantBlob:      "path/video.mkv",
		},
		{
			name:          "top-level blob",
			uri:           "az://encodes/output.mp4",
			wantContainer: "encodes",
			wantBlob:      "output.mp4",
		},
		{
			name:          "nested blob",
			uri:           "az://raw/2024/movie/hdr/source.mkv",
			wantContainer: "raw",
			wantBlob:      "2024/movie/hdr/source.mkv",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			container, blob, err := parseAzureURI(tc.uri)
			if err != nil {
				t.Fatalf("parseAzureURI(%q): unexpected error: %v", tc.uri, err)
			}
			if container != tc.wantContainer {
				t.Errorf("container = %q, want %q", container, tc.wantContainer)
			}
			if blob != tc.wantBlob {
				t.Errorf("blob = %q, want %q", blob, tc.wantBlob)
			}
		})
	}
}

func TestParseAzureURI_Invalid(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{"no container", "az:///blob.mkv"},
		{"no blob", "az://container/"},
		{"empty", "az://"},
		{"container only", "az://container"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseAzureURI(tc.uri)
			if err == nil {
				t.Errorf("parseAzureURI(%q): expected error, got nil", tc.uri)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// writeToFile
// ---------------------------------------------------------------------------

func TestWriteToFile_Success(t *testing.T) {
	dir := t.TempDir()
	destPath := filepath.Join(dir, "output.mkv")
	content := []byte("fake video content bytes")

	if err := writeToFile(destPath, bytes.NewReader(content)); err != nil {
		t.Fatalf("writeToFile: %v", err)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("file content = %q, want %q", got, content)
	}
}

func TestWriteToFile_CreatesParentDirs(t *testing.T) {
	parent := t.TempDir()
	destPath := filepath.Join(parent, "deep", "nested", "dir", "file.bin")

	if err := writeToFile(destPath, bytes.NewReader([]byte("hi"))); err != nil {
		t.Fatalf("writeToFile: %v", err)
	}
	if _, err := os.Stat(destPath); err != nil {
		t.Errorf("file not found after writeToFile with nested dirs: %v", err)
	}
}

func TestWriteToFile_EmptyContent(t *testing.T) {
	dir := t.TempDir()
	destPath := filepath.Join(dir, "empty.bin")

	if err := writeToFile(destPath, bytes.NewReader(nil)); err != nil {
		t.Fatalf("writeToFile with empty content: %v", err)
	}

	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("expected empty file, got size %d", info.Size())
	}
}

func TestWriteToFile_LargeContent(t *testing.T) {
	dir := t.TempDir()
	destPath := filepath.Join(dir, "large.bin")
	// Write 1 MiB of data.
	content := bytes.Repeat([]byte{0xFF, 0x00, 0xAB}, 1024*1024/3+1)
	if err := writeToFile(destPath, bytes.NewReader(content)); err != nil {
		t.Fatalf("writeToFile large: %v", err)
	}
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read large file: %v", err)
	}
	if len(got) != len(content) {
		t.Errorf("expected %d bytes, got %d", len(content), len(got))
	}
}

func TestWriteToFile_InvalidPath(t *testing.T) {
	// Attempting to create a file in a path where the parent is a file (not a
	// dir) should fail.
	dir := t.TempDir()
	// Create a regular file at the location we want to use as a directory.
	blockerPath := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blockerPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("create blocker file: %v", err)
	}
	// Try to write into blocker/subdir/file.bin — should fail because
	// blocker is a file, not a directory.
	destPath := filepath.Join(blockerPath, "subdir", "file.bin")
	if err := writeToFile(destPath, bytes.NewReader([]byte("data"))); err == nil {
		t.Error("expected error when parent path is a file")
	}
}

// ---------------------------------------------------------------------------
// NewStore — scheme dispatch
// ---------------------------------------------------------------------------

// TestNewStore_UnsupportedScheme verifies that an unknown URI scheme returns
// an error without calling any cloud SDK.
func TestNewStore_UnsupportedScheme(t *testing.T) {
	tests := []struct {
		name string
		uri  string
	}{
		{"ftp", "ftp://bucket/key"},
		{"file", "file:///local/path"},
		{"http", "http://bucket/key"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewStore(tc.uri)
			if err == nil {
				t.Errorf("NewStore(%q): expected error for unsupported scheme", tc.uri)
			}
			if err != nil && !strings.Contains(err.Error(), "unsupported scheme") {
				t.Errorf("NewStore(%q): error = %v, want 'unsupported scheme'", tc.uri, err)
			}
		})
	}
}

// TestNewStore_InvalidURI verifies that a completely malformed URI returns an error.
func TestNewStore_InvalidURI(t *testing.T) {
	// url.Parse is very lenient; we pass a URI that triggers the unsupported
	// scheme path rather than a parse error, so we test the error message.
	_, err := NewStore("not-a-uri-at-all")
	if err == nil {
		t.Error("expected error for non-URI string")
	}
}

// TestNewStore_S3_Dispatches verifies the s3:// scheme dispatches to the S3
// constructor.  The S3 constructor loads AWS config from env/files; in a
// clean CI environment with no AWS config it may fail, so we only check that
// a non-nil error is returned only when the SDK itself fails — not a scheme
// error.
func TestNewStore_S3_Dispatches(t *testing.T) {
	// We cannot guarantee AWS creds in the test environment, so we do not
	// assert success.  We just verify the error is NOT "unsupported scheme".
	_, err := NewStore("s3://test-bucket/key.mkv")
	if err != nil {
		if strings.Contains(err.Error(), "unsupported scheme") {
			t.Error("S3 URI should not trigger unsupported scheme error")
		}
		// Other errors (no AWS config etc.) are acceptable in this environment.
	}
}

// TestNewStore_GCS_MissingCreds verifies the gs:// scheme dispatches to the
// GCS constructor.  Without credentials the GCS client should fail, but it
// should NOT fail with "unsupported scheme".
func TestNewStore_GCS_MissingCreds(t *testing.T) {
	// Clear any accidental credential env vars.
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")

	_, err := NewStore("gs://test-bucket/object.mkv")
	if err != nil {
		if strings.Contains(err.Error(), "unsupported scheme") {
			t.Error("GCS URI should not trigger unsupported scheme error")
		}
		// SDK errors (no credentials) are acceptable here.
	}
}

// TestNewStore_Azure_MissingEnv verifies that the az:// scheme dispatches to
// the Azure constructor and that missing env vars produce an explicit error.
func TestNewStore_Azure_MissingEnv(t *testing.T) {
	t.Setenv("AZURE_STORAGE_ACCOUNT", "")
	t.Setenv("AZURE_STORAGE_KEY", "")

	_, err := NewStore("az://my-container/blob.mkv")
	if err == nil {
		t.Error("expected error when Azure env vars are not set")
	}
	// Should mention the required env vars.
	if !strings.Contains(err.Error(), "AZURE_STORAGE_ACCOUNT") {
		t.Errorf("error should mention AZURE_STORAGE_ACCOUNT, got: %v", err)
	}
}

// TestNewStore_Azure_Dispatches_NotSchemeError verifies that with some env vars
// the az:// scheme does not return an "unsupported scheme" error.
func TestNewStore_Azure_Dispatches_NotSchemeError(t *testing.T) {
	t.Setenv("AZURE_STORAGE_ACCOUNT", "")
	t.Setenv("AZURE_STORAGE_KEY", "")

	_, err := NewStore("az://container/blob.bin")
	if err != nil && strings.Contains(err.Error(), "unsupported scheme") {
		t.Error("az:// should not trigger unsupported scheme error")
	}
}

// ---------------------------------------------------------------------------
// URI parser error message quality
// ---------------------------------------------------------------------------

func TestParseS3URI_ErrorContainsURI(t *testing.T) {
	_, _, err := parseS3URI("s3://")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "s3://") {
		t.Errorf("error should include the URI, got: %v", err)
	}
}

func TestParseGCSURI_ErrorContainsURI(t *testing.T) {
	_, _, err := parseGCSURI("gs://bucket/")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "gs://") {
		t.Errorf("error should include the URI, got: %v", err)
	}
}

func TestParseAzureURI_ErrorContainsURI(t *testing.T) {
	_, _, err := parseAzureURI("az://container/")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "az://") {
		t.Errorf("error should include the URI, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Scheme case-insensitivity (NewStore uses strings.ToLower)
// ---------------------------------------------------------------------------

func TestNewStore_SchemeIsCaseInsensitive(t *testing.T) {
	// S3:// (uppercase) should not produce "unsupported scheme".
	_, err := NewStore("S3://bucket/key")
	if err != nil && strings.Contains(err.Error(), "unsupported scheme") {
		t.Error("S3:// (uppercase) should be treated as s3://")
	}
}
