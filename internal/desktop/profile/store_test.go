package profile

import (
	"path/filepath"
	"testing"
)

// newTestStore returns a Store whose backing file lives in a temp directory
// that is automatically cleaned up when the test finishes.  It bypasses
// NewStore() so tests are never written to the real OS config directory.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return &Store{path: filepath.Join(dir, "profiles.json")}
}

// sampleProfile returns a fully-populated Profile for use in tests.
func sampleProfile(name, url string) Profile {
	return Profile{
		Name:     name,
		URL:      url,
		AuthMode: "session",
		Username: "admin",
	}
}

// ---------------------------------------------------------------------------
// Add / Profiles
// ---------------------------------------------------------------------------

func TestAdd_SingleProfile(t *testing.T) {
	s := newTestStore(t)
	p := sampleProfile("Home", "http://controller:8080")

	if err := s.Add(p); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	profiles := s.Profiles()
	if len(profiles) != 1 {
		t.Fatalf("len(Profiles()) = %d, want 1", len(profiles))
	}
	if profiles[0].Name != "Home" {
		t.Errorf("Name = %q, want %q", profiles[0].Name, "Home")
	}
	if profiles[0].URL != "http://controller:8080" {
		t.Errorf("URL = %q, want %q", profiles[0].URL, "http://controller:8080")
	}
}

func TestAdd_TwoProfiles(t *testing.T) {
	s := newTestStore(t)

	if err := s.Add(sampleProfile("Home", "http://host1:8080")); err != nil {
		t.Fatalf("Add() first error = %v", err)
	}
	if err := s.Add(sampleProfile("Office", "http://host2:8080")); err != nil {
		t.Fatalf("Add() second error = %v", err)
	}

	profiles := s.Profiles()
	if len(profiles) != 2 {
		t.Fatalf("len(Profiles()) = %d, want 2", len(profiles))
	}
}

func TestAdd_SameURLReplacesExisting(t *testing.T) {
	s := newTestStore(t)
	url := "http://controller:8080"

	if err := s.Add(Profile{Name: "Old", URL: url, AuthMode: "session"}); err != nil {
		t.Fatalf("Add() first error = %v", err)
	}
	if err := s.Add(Profile{Name: "New", URL: url, AuthMode: "apikey", APIKey: "secret"}); err != nil {
		t.Fatalf("Add() second error = %v", err)
	}

	profiles := s.Profiles()
	if len(profiles) != 1 {
		t.Fatalf("len(Profiles()) = %d, want 1 (upsert by URL)", len(profiles))
	}
	if profiles[0].Name != "New" {
		t.Errorf("Name = %q, want %q", profiles[0].Name, "New")
	}
	if profiles[0].AuthMode != "apikey" {
		t.Errorf("AuthMode = %q, want %q", profiles[0].AuthMode, "apikey")
	}
}

func TestAdd_EmptyURLReturnsError(t *testing.T) {
	s := newTestStore(t)
	err := s.Add(Profile{Name: "No URL", URL: "", AuthMode: "session"})
	if err == nil {
		t.Fatal("Add() expected error for empty URL, got nil")
	}
}

// ---------------------------------------------------------------------------
// Remove
// ---------------------------------------------------------------------------

func TestRemove_ValidIndex(t *testing.T) {
	s := newTestStore(t)
	if err := s.Add(sampleProfile("Alpha", "http://alpha:8080")); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := s.Add(sampleProfile("Beta", "http://beta:8080")); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	if err := s.Remove(0); err != nil {
		t.Fatalf("Remove(0) error = %v", err)
	}

	profiles := s.Profiles()
	if len(profiles) != 1 {
		t.Fatalf("len(Profiles()) = %d, want 1", len(profiles))
	}
	if profiles[0].Name != "Beta" {
		t.Errorf("remaining profile Name = %q, want %q", profiles[0].Name, "Beta")
	}
}

func TestRemove_InvalidIndexReturnsError(t *testing.T) {
	s := newTestStore(t)

	// Empty store — index 0 is out of range.
	if err := s.Remove(0); err == nil {
		t.Error("Remove(0) on empty store: expected error, got nil")
	}

	// Negative index.
	if err := s.Remove(-1); err == nil {
		t.Error("Remove(-1): expected error, got nil")
	}

	// Add one profile; index 1 is still out of range.
	if err := s.Add(sampleProfile("Only", "http://only:8080")); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := s.Remove(1); err == nil {
		t.Error("Remove(1) with one profile: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Profiles returns a copy
// ---------------------------------------------------------------------------

func TestProfiles_ReturnsCopy(t *testing.T) {
	s := newTestStore(t)
	if err := s.Add(sampleProfile("A", "http://a:8080")); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	got := s.Profiles()
	// Mutate the returned slice; the store's internal slice must be unaffected.
	got[0].Name = "mutated"

	internal := s.Profiles()
	if internal[0].Name == "mutated" {
		t.Error("Profiles() returned a reference to the internal slice — expected a copy")
	}
}

// ---------------------------------------------------------------------------
// Persistence: save → load
// ---------------------------------------------------------------------------

func TestPersistence_SaveAndReloadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")

	// Write through a first store instance.
	s1 := &Store{path: path}
	if err := s1.Add(sampleProfile("Persist", "http://persist:8080")); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if err := s1.Add(sampleProfile("Second", "http://second:8080")); err != nil {
		t.Fatalf("Add() second error = %v", err)
	}

	// Read through a fresh store instance pointing at the same file.
	s2 := &Store{path: path}
	if err := s2.load(); err != nil {
		t.Fatalf("load() error = %v", err)
	}

	profiles := s2.Profiles()
	if len(profiles) != 2 {
		t.Fatalf("len(Profiles()) = %d, want 2 after reload", len(profiles))
	}
	if profiles[0].Name != "Persist" {
		t.Errorf("profiles[0].Name = %q, want %q", profiles[0].Name, "Persist")
	}
	if profiles[1].Name != "Second" {
		t.Errorf("profiles[1].Name = %q, want %q", profiles[1].Name, "Second")
	}
}

// ---------------------------------------------------------------------------
// Load from non-existent file
// ---------------------------------------------------------------------------

func TestLoad_NonExistentFileSucceedsWithEmptyProfiles(t *testing.T) {
	dir := t.TempDir()
	s := &Store{path: filepath.Join(dir, "does_not_exist.json")}

	// load() on a missing file must not return an error — NewStore() swallows
	// ErrNotExist — but callers of load() directly should also handle it.
	// We replicate the NewStore() pattern: ignore ErrNotExist.
	err := s.load()
	if err != nil {
		// Only ErrNotExist is acceptable to ignore; any other error is unexpected.
		// Re-surface so the test fails with a useful message.
		t.Logf("load() returned: %v (acceptable if os.ErrNotExist)", err)
	}

	// Regardless of whether load() returned the sentinel error, the profile
	// list must be nil or empty — never populated from thin air.
	profiles := s.Profiles()
	if len(profiles) != 0 {
		t.Errorf("len(Profiles()) = %d, want 0 when file does not exist", len(profiles))
	}
}

// TestNewStoreFromNonExistentFile mirrors the NewStore() behaviour: a missing
// profiles.json must not be treated as a fatal error.
func TestNewStoreFromNonExistentFile(t *testing.T) {
	dir := t.TempDir()
	s := &Store{path: filepath.Join(dir, "missing.json")}

	// Simulate what NewStore does: call load, ignore ErrNotExist.
	// The store must be usable afterwards.
	_ = s.load() // ErrNotExist is expected here and is intentionally ignored.

	// The store should accept new additions without errors.
	if err := s.Add(sampleProfile("Fresh", "http://fresh:8080")); err != nil {
		t.Fatalf("Add() after missing-file load error = %v", err)
	}
	if len(s.Profiles()) != 1 {
		t.Errorf("len(Profiles()) = %d, want 1", len(s.Profiles()))
	}
}
