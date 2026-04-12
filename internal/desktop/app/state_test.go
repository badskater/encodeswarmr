// Use an external test package to avoid pulling in the gioui rendering
// dependencies declared in app.go and theme.go, which require a display
// context and are not needed to test State or StatusColor.
package app_test

import (
	"image/color"
	"sync"
	"testing"

	"github.com/badskater/encodeswarmr/internal/desktop/app"
	"github.com/badskater/encodeswarmr/internal/desktop/client"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newState() *app.State { return &app.State{} }

// ---------------------------------------------------------------------------
// Initial zero values
// ---------------------------------------------------------------------------

func TestState_InitialValues(t *testing.T) {
	s := newState()

	if s.Client() != nil {
		t.Error("Client(): expected nil on fresh State")
	}
	if s.WSClient() != nil {
		t.Error("WSClient(): expected nil on fresh State")
	}
	if s.User() != nil {
		t.Error("User(): expected nil on fresh State")
	}
	if s.ProfileName() != "" {
		t.Errorf("ProfileName() = %q, want empty string on fresh State", s.ProfileName())
	}
}

// ---------------------------------------------------------------------------
// SetClient / Client round-trip
// ---------------------------------------------------------------------------

func TestState_SetClient(t *testing.T) {
	s := newState()
	c := client.New("http://controller:8080")

	s.SetClient(c)

	if s.Client() != c {
		t.Error("Client() did not return the value passed to SetClient()")
	}
}

// ---------------------------------------------------------------------------
// SetUser / User round-trip
// ---------------------------------------------------------------------------

func TestState_SetUser(t *testing.T) {
	s := newState()
	u := &client.User{ID: "u1", Username: "alice", Role: "admin"}

	s.SetUser(u)

	got := s.User()
	if got == nil {
		t.Fatal("User() returned nil after SetUser()")
	}
	if got.ID != "u1" {
		t.Errorf("User().ID = %q, want %q", got.ID, "u1")
	}
	if got.Username != "alice" {
		t.Errorf("User().Username = %q, want %q", got.Username, "alice")
	}
}

// ---------------------------------------------------------------------------
// SetProfileName / ProfileName round-trip
// ---------------------------------------------------------------------------

func TestState_SetProfileName(t *testing.T) {
	s := newState()

	s.SetProfileName("Home Lab")

	if s.ProfileName() != "Home Lab" {
		t.Errorf("ProfileName() = %q, want %q", s.ProfileName(), "Home Lab")
	}
}

// ---------------------------------------------------------------------------
// Reset clears all fields
// ---------------------------------------------------------------------------

func TestState_Reset(t *testing.T) {
	s := newState()

	s.SetClient(client.New("http://controller:8080"))
	s.SetUser(&client.User{ID: "u1", Username: "bob"})
	s.SetProfileName("Office")
	// WSClient requires a live WebSocket connection; we leave it nil and verify
	// Reset handles that gracefully (ws == nil branch in Reset).

	s.Reset()

	if s.Client() != nil {
		t.Error("Client(): expected nil after Reset()")
	}
	if s.WSClient() != nil {
		t.Error("WSClient(): expected nil after Reset()")
	}
	if s.User() != nil {
		t.Error("User(): expected nil after Reset()")
	}
	if s.ProfileName() != "" {
		t.Errorf("ProfileName() = %q, want empty string after Reset()", s.ProfileName())
	}
}

// ---------------------------------------------------------------------------
// Thread safety
// ---------------------------------------------------------------------------

func TestState_ConcurrentAccess(t *testing.T) {
	s := newState()
	c := client.New("http://controller:8080")
	u := &client.User{ID: "u1", Username: "concurrent"}

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 4)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			s.SetClient(c)
		}()
		go func() {
			defer wg.Done()
			_ = s.Client()
		}()
		go func() {
			defer wg.Done()
			s.SetUser(u)
		}()
		go func() {
			defer wg.Done()
			_ = s.User()
		}()
	}

	// Must not panic or deadlock; the race detector catches data races.
	wg.Wait()
}

// ---------------------------------------------------------------------------
// StatusColor — known statuses
// ---------------------------------------------------------------------------

func TestStatusColor_KnownStatuses(t *testing.T) {
	tests := []struct {
		status string
		want   color.NRGBA
	}{
		{"idle", app.ColorSuccess},
		{"running", app.ColorPrimary},
		{"assigned", app.ColorPrimary},
		{"offline", app.ColorDanger},
		{"failed", app.ColorDanger},
		{"cancelled", app.ColorDanger},
		{"draining", app.ColorWarning},
		{"waiting", app.ColorWarning},
		{"queued", app.ColorWarning},
		{"pending", app.ColorWarning},
		// "completed" maps to emerald-500, which is not one of the named palette
		// vars, so we verify the exact NRGBA value documented in state.go.
		{"completed", color.NRGBA{R: 16, G: 185, B: 129, A: 255}},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := app.StatusColor(tt.status)
			if got != tt.want {
				t.Errorf("StatusColor(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// StatusColor — unknown status returns ColorMuted
// ---------------------------------------------------------------------------

func TestStatusColor_UnknownStatusReturnsMuted(t *testing.T) {
	unknowns := []string{"", "unknown", "IDLE", "RUNNING", "42", "n/a"}

	for _, status := range unknowns {
		t.Run(status, func(t *testing.T) {
			got := app.StatusColor(status)
			if got != app.ColorMuted {
				t.Errorf("StatusColor(%q) = %v, want ColorMuted (%v)", status, got, app.ColorMuted)
			}
		})
	}
}
