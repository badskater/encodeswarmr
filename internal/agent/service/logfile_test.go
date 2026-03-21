package service

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewRotatingFileWriter_CreatesFile verifies the writer creates the log file
// and can be closed cleanly.
func TestNewRotatingFileWriter_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	w, err := newRotatingFileWriter(dir, "agent.log", 1, 3)
	if err != nil {
		t.Fatalf("newRotatingFileWriter: %v", err)
	}
	defer w.Close()

	logPath := filepath.Join(dir, "agent.log")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatal("expected log file to exist after creation")
	}
}

// TestNewRotatingFileWriter_DefaultsApplied verifies that zero maxSizeMB and
// maxBackups get sensible defaults instead of 0.
func TestNewRotatingFileWriter_DefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	w, err := newRotatingFileWriter(dir, "agent.log", 0, 0)
	if err != nil {
		t.Fatalf("newRotatingFileWriter: %v", err)
	}
	defer w.Close()

	if w.maxBytes != 100*1024*1024 {
		t.Errorf("maxBytes = %d, want %d", w.maxBytes, 100*1024*1024)
	}
	if w.maxBackups != 5 {
		t.Errorf("maxBackups = %d, want 5", w.maxBackups)
	}
}

// TestRotatingFileWriter_Write writes small data and verifies it lands in the file.
func TestRotatingFileWriter_Write(t *testing.T) {
	dir := t.TempDir()
	w, err := newRotatingFileWriter(dir, "agent.log", 1, 3)
	if err != nil {
		t.Fatalf("newRotatingFileWriter: %v", err)
	}
	defer w.Close()

	msg := []byte("hello log\n")
	n, err := w.Write(msg)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(msg) {
		t.Errorf("wrote %d bytes, want %d", n, len(msg))
	}

	// Flush via close then re-read.
	w.Close()
	data, err := os.ReadFile(filepath.Join(dir, "agent.log"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Contains(data, msg) {
		t.Errorf("log file missing written message; got %q", data)
	}
}

// TestRotatingFileWriter_RotationTriggered verifies that writing beyond maxBytes
// causes the original file to be renamed to agent.log.1 and a new agent.log
// to be created.
func TestRotatingFileWriter_RotationTriggered(t *testing.T) {
	dir := t.TempDir()
	// Use maxSizeMB=0 which defaults to 100 MB. We set maxBytes directly after
	// construction so we can use a tiny threshold.
	w, err := newRotatingFileWriter(dir, "agent.log", 1, 3)
	if err != nil {
		t.Fatalf("newRotatingFileWriter: %v", err)
	}
	// Override the threshold to something tiny so we can trigger rotation cheaply.
	w.maxBytes = 10

	// Write a payload that exceeds the threshold.
	payload := []byte(strings.Repeat("x", 20))
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// The rotation renames agent.log -> agent.log.1 before the current write,
	// so .1 should exist now.
	backup := filepath.Join(dir, "agent.log.1")
	if _, err := os.Stat(backup); os.IsNotExist(err) {
		t.Fatal("expected agent.log.1 to exist after rotation")
	}

	// A fresh agent.log should also exist.
	if _, err := os.Stat(filepath.Join(dir, "agent.log")); os.IsNotExist(err) {
		t.Fatal("expected agent.log to exist after rotation")
	}
	w.Close()
}

// TestRotatingFileWriter_MultipleRotations verifies that repeated rotations shift
// backups through .1 → .2 → .3 correctly.
func TestRotatingFileWriter_MultipleRotations(t *testing.T) {
	dir := t.TempDir()
	w, err := newRotatingFileWriter(dir, "agent.log", 1, 5)
	if err != nil {
		t.Fatalf("newRotatingFileWriter: %v", err)
	}
	w.maxBytes = 5 // tiny threshold

	payload := []byte("abcdef") // 6 bytes — exceeds threshold every write

	for i := 0; i < 3; i++ {
		if _, err := w.Write(payload); err != nil {
			t.Fatalf("Write #%d: %v", i, err)
		}
	}
	w.Close()

	// After 3 writes with rotation on each, we expect .1, .2, .3 to exist.
	for i := 1; i <= 3; i++ {
		p := filepath.Join(dir, fmt.Sprintf("agent.log.%d", i))
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected backup %s to exist", p)
		}
	}
}

// TestRotatingFileWriter_ExcessBackupsRemoved verifies that the writer
// removes backups beyond maxBackups.
func TestRotatingFileWriter_ExcessBackupsRemoved(t *testing.T) {
	dir := t.TempDir()
	const maxBackups = 3
	w, err := newRotatingFileWriter(dir, "agent.log", 1, maxBackups)
	if err != nil {
		t.Fatalf("newRotatingFileWriter: %v", err)
	}
	w.maxBytes = 5

	payload := []byte("abcdef") // triggers rotation every write

	// Write enough times to overflow maxBackups.
	for i := 0; i < maxBackups+2; i++ {
		if _, err := w.Write(payload); err != nil {
			t.Fatalf("Write #%d: %v", i, err)
		}
	}
	w.Close()

	// Backups beyond maxBackups should be absent.
	for i := maxBackups + 1; i <= maxBackups+2; i++ {
		p := filepath.Join(dir, fmt.Sprintf("agent.log.%d", i))
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("backup %s should have been removed but still exists", p)
		}
	}
}

// TestRotatingFileWriter_Close_Idempotent verifies that the writer can be
// closed without panicking, and that the first close succeeds cleanly.
func TestRotatingFileWriter_Close_Idempotent(t *testing.T) {
	dir := t.TempDir()
	w, err := newRotatingFileWriter(dir, "agent.log", 1, 3)
	if err != nil {
		t.Fatalf("newRotatingFileWriter: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// A second close may return an "already closed" error on some platforms —
	// that is acceptable.  We just verify it does not panic.
	_ = w.Close()
}

// TestSetupLogWriter_ReturnsWriter verifies setupLogWriter returns a usable
// io.Writer and close function.
func TestSetupLogWriter_ReturnsWriter(t *testing.T) {
	dir := t.TempDir()
	w, closeFn, err := setupLogWriter(dir, 1, 3)
	if err != nil {
		t.Fatalf("setupLogWriter: %v", err)
	}
	defer closeFn()

	if w == nil {
		t.Fatal("expected non-nil writer")
	}

	// Writing through the returned MultiWriter should not error.
	if _, err := fmt.Fprintln(w, "test line"); err != nil {
		t.Errorf("Write through setupLogWriter writer: %v", err)
	}
}

// TestNewRotatingFileWriter_MkdirAll verifies the writer creates missing
// intermediate directories.
func TestNewRotatingFileWriter_MkdirAll(t *testing.T) {
	base := t.TempDir()
	nested := filepath.Join(base, "a", "b", "c")
	w, err := newRotatingFileWriter(nested, "agent.log", 1, 3)
	if err != nil {
		t.Fatalf("newRotatingFileWriter on nested path: %v", err)
	}
	defer w.Close()

	if _, err := os.Stat(nested); os.IsNotExist(err) {
		t.Fatal("nested directory was not created")
	}
}

// TestRotatingFileWriter_WriteConcurrent runs concurrent writes to check for
// data races (run with -race).
func TestRotatingFileWriter_WriteConcurrent(t *testing.T) {
	dir := t.TempDir()
	w, err := newRotatingFileWriter(dir, "agent.log", 1, 3)
	if err != nil {
		t.Fatalf("newRotatingFileWriter: %v", err)
	}
	defer w.Close()
	w.maxBytes = 50 // small to induce some rotations

	done := make(chan struct{})
	for i := 0; i < 4; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 20; j++ {
				_, _ = fmt.Fprintf(w, "goroutine %d iteration %d\n", id, j)
			}
		}(i)
	}
	for i := 0; i < 4; i++ {
		<-done
	}
}
