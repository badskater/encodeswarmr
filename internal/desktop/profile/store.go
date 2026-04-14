// Package profile persists connection profiles to the user's local config directory.
package profile

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Profile stores the connection details for a saved controller endpoint.
type Profile struct {
	// Name is the human-readable label shown in the profile list.
	Name string `json:"name"`
	// URL is the controller base URL (e.g. http://controller:8080).
	URL string `json:"url"`
	// AuthMode is either "session" (username/password) or "apikey".
	AuthMode string `json:"auth_mode"`
	// Username is the session-auth username (AuthMode == "session").
	Username string `json:"username,omitempty"`
	// APIKey is the API key value (AuthMode == "apikey").
	// Note: stored in plaintext in the config file — treat accordingly.
	APIKey string `json:"api_key,omitempty"`
}

// Store manages a slice of profiles persisted to disk.
type Store struct {
	path     string
	profiles []Profile
}

// NewStore opens (or creates) the profile store in the OS config directory.
func NewStore() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	appDir := filepath.Join(dir, "encodeswarmr-desktop")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(appDir, "profiles.json")

	s := &Store{path: path}
	if err := s.load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return s, nil
}

// Profiles returns the current list of profiles (read-only copy).
func (s *Store) Profiles() []Profile {
	out := make([]Profile, len(s.profiles))
	copy(out, s.profiles)
	return out
}

// Add appends or replaces a profile (matched by URL) and persists the store.
func (s *Store) Add(p Profile) error {
	if p.URL == "" {
		return errors.New("profile URL must not be empty")
	}
	// Replace existing entry with the same URL.
	for i, existing := range s.profiles {
		if existing.URL == p.URL {
			s.profiles[i] = p
			return s.save()
		}
	}
	s.profiles = append(s.profiles, p)
	return s.save()
}

// Remove deletes the profile at the given index.
func (s *Store) Remove(idx int) error {
	if idx < 0 || idx >= len(s.profiles) {
		return errors.New("profile index out of range")
	}
	s.profiles = append(s.profiles[:idx], s.profiles[idx+1:]...)
	return s.save()
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.profiles)
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.profiles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}
