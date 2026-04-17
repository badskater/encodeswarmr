package widget

import (
	"testing"
)

// ---------------------------------------------------------------------------
// NewSearchBar
// ---------------------------------------------------------------------------

func TestNewSearchBar_Defaults(t *testing.T) {
	t.Parallel()
	s := NewSearchBar()
	if s == nil {
		t.Fatal("NewSearchBar() returned nil")
	}
	if !s.Editor.SingleLine {
		t.Error("Editor.SingleLine = false, want true (single-line input)")
	}
	if s.OnSearch != nil {
		t.Error("OnSearch should be nil on a fresh SearchBar")
	}
	if s.prevContent != "" {
		t.Errorf("prevContent = %q, want empty string", s.prevContent)
	}
}

// ---------------------------------------------------------------------------
// OnSearch callback wiring
// ---------------------------------------------------------------------------

func TestNewSearchBar_OnSearchCallbackCanBeSet(t *testing.T) {
	t.Parallel()
	s := NewSearchBar()
	var received []string
	s.OnSearch = func(q string) {
		received = append(received, q)
	}
	// Call it directly to verify the callback is wired correctly.
	s.OnSearch("hello")
	s.OnSearch("world")
	if len(received) != 2 {
		t.Fatalf("OnSearch called %d times, want 2", len(received))
	}
	if received[0] != "hello" {
		t.Errorf("received[0] = %q, want hello", received[0])
	}
	if received[1] != "world" {
		t.Errorf("received[1] = %q, want world", received[1])
	}
}

// ---------------------------------------------------------------------------
// colorSearchIcon constant
// ---------------------------------------------------------------------------

func TestColorSearchIcon_IsGray400(t *testing.T) {
	t.Parallel()
	if colorSearchIcon.R != 156 || colorSearchIcon.G != 163 || colorSearchIcon.B != 175 || colorSearchIcon.A != 255 {
		t.Errorf("colorSearchIcon = %v, want gray-400 (156, 163, 175, 255)", colorSearchIcon)
	}
}

// ---------------------------------------------------------------------------
// form color constants
// ---------------------------------------------------------------------------

func TestFormColorConstants(t *testing.T) {
	t.Parallel()
	// colorFormLabel: gray-700
	if colorFormLabel.R != 55 || colorFormLabel.G != 65 || colorFormLabel.B != 81 {
		t.Errorf("colorFormLabel = %v, want gray-700 (55,65,81,255)", colorFormLabel)
	}
	// colorInputBg: white
	if colorInputBg.R != 255 || colorInputBg.G != 255 || colorInputBg.B != 255 {
		t.Errorf("colorInputBg = %v, want white (255,255,255,255)", colorInputBg)
	}
	// colorInputBorder: gray-300
	if colorInputBorder.R != 209 || colorInputBorder.G != 213 || colorInputBorder.B != 219 {
		t.Errorf("colorInputBorder = %v, want gray-300 (209,213,219,255)", colorInputBorder)
	}
	// colorInputText: gray-900
	if colorInputText.R != 17 || colorInputText.G != 24 || colorInputText.B != 39 {
		t.Errorf("colorInputText = %v, want gray-900 (17,24,39,255)", colorInputText)
	}
	// colorInputHint: gray-400
	if colorInputHint.R != 156 || colorInputHint.G != 163 || colorInputHint.B != 175 {
		t.Errorf("colorInputHint = %v, want gray-400 (156,163,175,255)", colorInputHint)
	}
}
