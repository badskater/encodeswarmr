package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// rotatingFileWriter is a size-based rotating log file writer.
// When the current file exceeds maxBytes, it is rotated to a numbered
// backup (agent.log.1, agent.log.2, …) and a new file is created.
type rotatingFileWriter struct {
	mu         sync.Mutex
	dir        string
	filename   string
	maxBytes   int64
	maxBackups int
	file       *os.File
	size       int64
}

// newRotatingFileWriter opens (or creates) a log file at dir/filename
// with size-based rotation.
func newRotatingFileWriter(dir, filename string, maxSizeMB, maxBackups int) (*rotatingFileWriter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir %s: %w", dir, err)
	}

	w := &rotatingFileWriter{
		dir:        dir,
		filename:   filename,
		maxBytes:   int64(maxSizeMB) * 1024 * 1024,
		maxBackups: maxBackups,
	}
	if w.maxBytes <= 0 {
		w.maxBytes = 100 * 1024 * 1024 // 100 MB default
	}
	if w.maxBackups <= 0 {
		w.maxBackups = 5
	}

	if err := w.openFile(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotatingFileWriter) openFile() error {
	path := filepath.Join(w.dir, w.filename)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return fmt.Errorf("stat log file: %w", err)
	}
	w.file = f
	w.size = info.Size()
	return nil
}

func (w *rotatingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			// If rotation fails, still try to write to the current file.
			_ = err
		}
	}

	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

func (w *rotatingFileWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		return err
	}

	basePath := filepath.Join(w.dir, w.filename)

	// Shift existing backups: .4 → .5, .3 → .4, etc.
	for i := w.maxBackups - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", basePath, i)
		dst := fmt.Sprintf("%s.%d", basePath, i+1)
		_ = os.Remove(dst)
		_ = os.Rename(src, dst)
	}

	// Current → .1
	_ = os.Rename(basePath, basePath+".1")

	// Remove excess backups.
	w.removeExcessBackups()

	return w.openFile()
}

func (w *rotatingFileWriter) removeExcessBackups() {
	basePath := filepath.Join(w.dir, w.filename)
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}

	var backups []string
	prefix := w.filename + "."
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) {
			backups = append(backups, e.Name())
		}
	}
	sort.Strings(backups)

	for len(backups) > w.maxBackups {
		oldest := backups[len(backups)-1]
		_ = os.Remove(filepath.Join(basePath[:len(basePath)-len(w.filename)], oldest))
		backups = backups[:len(backups)-1]
	}
}

// setupLogWriter creates a log writer that writes to a file in logDir
// and also to stderr. Returns the writer and a close function.
func setupLogWriter(logDir string, maxSizeMB, maxBackups int) (io.Writer, func(), error) {
	rfw, err := newRotatingFileWriter(logDir, "agent.log", maxSizeMB, maxBackups)
	if err != nil {
		return os.Stderr, func() {}, err
	}
	w := io.MultiWriter(os.Stderr, rfw)
	return w, func() { rfw.Close() }, nil
}
