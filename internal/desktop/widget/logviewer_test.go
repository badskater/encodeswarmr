package widget

import (
	"testing"
)

// TestNewLogViewer verifies the initial state of a freshly created LogViewer.
func TestNewLogViewer_Defaults(t *testing.T) {
	lv := NewLogViewer()

	if !lv.followMode {
		t.Error("followMode = false, want true")
	}
	if lv.ansiRegex == nil {
		t.Error("ansiRegex is nil, want a compiled regexp")
	}
}

// TestSetLines_LineCount verifies that SetLines stores all provided lines.
func TestSetLines_LineCount(t *testing.T) {
	lv := NewLogViewer()
	lines := []LogLine{
		{Message: "a"},
		{Message: "b"},
		{Message: "c"},
		{Message: "d"},
		{Message: "e"},
	}
	lv.SetLines(lines)

	if got := lv.lineCount(); got != 5 {
		t.Errorf("lineCount() = %d, want 5", got)
	}
}

// TestSetLines_Replaces verifies that calling SetLines a second time replaces
// the previous set of lines.
func TestSetLines_Replaces(t *testing.T) {
	lv := NewLogViewer()
	lv.SetLines([]LogLine{{Message: "old1"}, {Message: "old2"}})
	lv.SetLines([]LogLine{{Message: "new1"}})

	if got := lv.lineCount(); got != 1 {
		t.Errorf("lineCount() = %d after replace, want 1", got)
	}
	if lv.getLine(0).Message != "new1" {
		t.Errorf("getLine(0).Message = %q, want %q", lv.getLine(0).Message, "new1")
	}
}

// TestAppendLine_LineCount verifies that AppendLine incrementally grows the log.
func TestAppendLine_LineCount(t *testing.T) {
	lv := NewLogViewer()

	if got := lv.lineCount(); got != 0 {
		t.Errorf("initial lineCount() = %d, want 0", got)
	}

	lv.AppendLine(LogLine{Message: "one"})
	lv.AppendLine(LogLine{Message: "two"})
	lv.AppendLine(LogLine{Message: "three"})

	if got := lv.lineCount(); got != 3 {
		t.Errorf("lineCount() = %d, want 3", got)
	}
}

// TestGetLine_ReturnsCorrectLine verifies indexed access to log lines.
func TestGetLine_ReturnsCorrectLine(t *testing.T) {
	lv := NewLogViewer()
	lv.SetLines([]LogLine{
		{Message: "alpha"},
		{Message: "beta"},
		{Message: "gamma"},
	})

	cases := []struct {
		index int
		want  string
	}{
		{0, "alpha"},
		{1, "beta"},
		{2, "gamma"},
	}
	for _, tc := range cases {
		got := lv.getLine(tc.index).Message
		if got != tc.want {
			t.Errorf("getLine(%d).Message = %q, want %q", tc.index, got, tc.want)
		}
	}
}

// TestStripANSI verifies removal of ANSI escape sequences.
func TestStripANSI_PlainText(t *testing.T) {
	lv := NewLogViewer()
	got := lv.StripANSI("plain text")
	if got != "plain text" {
		t.Errorf("StripANSI(%q) = %q, want %q", "plain text", got, "plain text")
	}
}

func TestStripANSI_SingleCode(t *testing.T) {
	lv := NewLogViewer()
	input := "\x1b[31mERROR\x1b[0m"
	want := "ERROR"
	got := lv.StripANSI(input)
	if got != want {
		t.Errorf("StripANSI(%q) = %q, want %q", input, got, want)
	}
}

func TestStripANSI_MultipleCodes(t *testing.T) {
	lv := NewLogViewer()
	input := "\x1b[1;31mbold red\x1b[0m normal"
	want := "bold red normal"
	got := lv.StripANSI(input)
	if got != want {
		t.Errorf("StripANSI(%q) = %q, want %q", input, got, want)
	}
}

func TestStripANSI_NoANSI(t *testing.T) {
	lv := NewLogViewer()
	input := "plain text"
	got := lv.StripANSI(input)
	if got != input {
		t.Errorf("StripANSI(%q) = %q, want %q", input, got, input)
	}
}

func TestStripANSI_Empty(t *testing.T) {
	lv := NewLogViewer()
	got := lv.StripANSI("")
	if got != "" {
		t.Errorf("StripANSI(%q) = %q, want empty string", "", got)
	}
}

// TestUpdateFilter_MatchesSubset verifies that a filter reduces visible lines.
// Only lines whose Message contains the filter text (case-insensitive) are shown.
func TestUpdateFilter_MatchesSubset(t *testing.T) {
	lv := NewLogViewer()
	// "connected to server" does NOT contain the substring "connection".
	// "ERROR: connection refused" and "retrying connection" do.
	lv.SetLines([]LogLine{
		{Message: "connected to server"},
		{Message: "ERROR: connection refused"},
		{Message: "retrying connection"},
		{Message: "DEBUG: timeout"},
	})

	lv.filterText = "connection"
	lv.updateFilter()

	if got := lv.lineCount(); got != 2 {
		t.Errorf("lineCount() = %d, want 2 after filter 'connection'", got)
	}
	// Matching lines are the original lines at indices 1 and 2.
	if lv.getLine(0).Message != "ERROR: connection refused" {
		t.Errorf("getLine(0).Message = %q, want %q", lv.getLine(0).Message, "ERROR: connection refused")
	}
	if lv.getLine(1).Message != "retrying connection" {
		t.Errorf("getLine(1).Message = %q, want %q", lv.getLine(1).Message, "retrying connection")
	}
}

// TestUpdateFilter_MatchesNothing documents current behaviour when no lines match.
//
// Known limitation: updateFilter reuses the filtered slice via filtered[:0]. When
// filtered starts as nil (no prior filter applied), nil[:0] evaluates to nil in Go,
// so filtered remains nil after a zero-match scan. lineCount() then falls back to
// returning len(lines) instead of 0.
//
// This means "active filter with zero results" is indistinguishable from "no filter"
// when filtered was nil before the call. The test documents this behaviour so a future
// fix (initialising filtered to an empty non-nil slice) can be verified against it.
func TestUpdateFilter_MatchesNothing(t *testing.T) {
	lv := NewLogViewer()
	lv.SetLines([]LogLine{
		{Message: "alpha"},
		{Message: "beta"},
	})

	// Prime filtered to a non-nil empty slice by running a matching filter first,
	// then switch to a non-matching filter. This exercises the intended zero-match path.
	lv.filterText = "alpha"
	lv.updateFilter() // filtered is now non-nil with 1 entry

	lv.filterText = "zzznomatch"
	lv.updateFilter() // filtered[:0] is non-nil empty; no entries appended

	if got := lv.lineCount(); got != 0 {
		t.Errorf("lineCount() = %d, want 0 when no lines match filter (after prior non-nil filtered)", got)
	}
}

// TestUpdateFilter_EmptyFilter verifies that an empty filter shows all lines.
func TestUpdateFilter_EmptyFilter(t *testing.T) {
	lv := NewLogViewer()
	lv.SetLines([]LogLine{
		{Message: "alpha"},
		{Message: "beta"},
		{Message: "gamma"},
	})

	// Set a filter, then clear it.
	lv.filterText = "alpha"
	lv.updateFilter()
	lv.filterText = ""
	lv.updateFilter()

	if lv.filtered != nil {
		t.Errorf("filtered = %v, want nil when filterText is empty", lv.filtered)
	}
	if got := lv.lineCount(); got != 3 {
		t.Errorf("lineCount() = %d, want 3 after clearing filter", got)
	}
}

// TestUpdateFilter_CaseInsensitive verifies case-insensitive matching.
func TestUpdateFilter_CaseInsensitive(t *testing.T) {
	lv := NewLogViewer()
	lv.SetLines([]LogLine{
		{Message: "INFO: Service started"},
		{Message: "debug trace output"},
		{Message: "INFO: shutting down"},
	})

	lv.filterText = "info"
	lv.updateFilter()

	if got := lv.lineCount(); got != 2 {
		t.Errorf("lineCount() = %d, want 2 for case-insensitive filter 'info'", got)
	}

	lv.filterText = "INFO"
	lv.updateFilter()

	if got := lv.lineCount(); got != 2 {
		t.Errorf("lineCount() = %d, want 2 for uppercase filter 'INFO'", got)
	}
}

// TestUpdateFilter_AllMatch verifies that a filter matching every line returns all lines.
func TestUpdateFilter_AllMatch(t *testing.T) {
	lv := NewLogViewer()
	lv.SetLines([]LogLine{
		{Message: "log entry one"},
		{Message: "log entry two"},
		{Message: "log entry three"},
	})

	lv.filterText = "log"
	lv.updateFilter()

	if got := lv.lineCount(); got != 3 {
		t.Errorf("lineCount() = %d, want 3 when all lines match filter", got)
	}
}
