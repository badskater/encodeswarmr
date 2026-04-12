// Package app holds shared application state for the desktop manager.
package app

import (
	"image/color"
	"sync"

	"github.com/badskater/encodeswarmr/internal/desktop/client"
)

// Color palette used across all pages.
var (
	ColorPrimary   = color.NRGBA{R: 59, G: 130, B: 246, A: 255}  // blue-500
	ColorSecondary = color.NRGBA{R: 107, G: 114, B: 128, A: 255} // gray-500
	ColorSuccess   = color.NRGBA{R: 34, G: 197, B: 94, A: 255}   // green-500
	ColorWarning   = color.NRGBA{R: 234, G: 179, B: 8, A: 255}   // yellow-500
	ColorDanger    = color.NRGBA{R: 239, G: 68, B: 68, A: 255}   // red-500
	ColorMuted     = color.NRGBA{R: 75, G: 85, B: 99, A: 255}    // gray-600
	ColorSurface   = color.NRGBA{R: 30, G: 30, B: 46, A: 255}    // dark card surface
	ColorBorder    = color.NRGBA{R: 55, G: 65, B: 81, A: 255}    // gray-700
	ColorTextLight = color.NRGBA{R: 156, G: 163, B: 175, A: 255} // gray-400
)

// StatusColor returns a colour that represents the given status string.
func StatusColor(status string) color.NRGBA {
	switch status {
	case "idle":
		return ColorSuccess
	case "running", "assigned":
		return ColorPrimary
	case "offline", "failed", "cancelled":
		return ColorDanger
	case "draining", "waiting", "queued", "pending":
		return ColorWarning
	case "completed":
		return color.NRGBA{R: 16, G: 185, B: 129, A: 255} // emerald-500
	default:
		return ColorMuted
	}
}

// State holds the shared runtime state for the desktop application.
type State struct {
	mu          sync.RWMutex
	c           *client.Client
	ws          *client.WSClient
	user        *client.User
	profileName string
}

// Client returns the current API client, or nil if not authenticated.
func (s *State) Client() *client.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.c
}

// SetClient stores the authenticated API client.
func (s *State) SetClient(c *client.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.c = c
}

// WSClient returns the current WebSocket client.
func (s *State) WSClient() *client.WSClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ws
}

// SetWSClient stores the WebSocket client.
func (s *State) SetWSClient(ws *client.WSClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ws = ws
}

// User returns the currently authenticated user.
func (s *State) User() *client.User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.user
}

// SetUser stores the currently authenticated user.
func (s *State) SetUser(u *client.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.user = u
}

// ProfileName returns the active profile name.
func (s *State) ProfileName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.profileName
}

// SetProfileName stores the active profile name.
func (s *State) SetProfileName(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.profileName = name
}

// Reset clears all authenticated state (used on logout).
func (s *State) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ws != nil {
		s.ws.Close()
	}
	s.c = nil
	s.ws = nil
	s.user = nil
	s.profileName = ""
}
