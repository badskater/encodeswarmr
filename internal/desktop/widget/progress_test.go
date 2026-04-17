package widget

import (
	"image/color"
	"testing"
)

// ---------------------------------------------------------------------------
// colorProgressTrack constant
// ---------------------------------------------------------------------------

func TestColorProgressTrack_IsGray200(t *testing.T) {
	t.Parallel()
	want := color.NRGBA{R: 229, G: 231, B: 235, A: 255}
	if colorProgressTrack != want {
		t.Errorf("colorProgressTrack = %v, want %v (gray-200)", colorProgressTrack, want)
	}
}

// ---------------------------------------------------------------------------
// logLevelColor — pure function, no Gio context needed
// ---------------------------------------------------------------------------

func TestLogLevelColor_Error(t *testing.T) {
	t.Parallel()
	want := color.NRGBA{R: 239, G: 68, B: 68, A: 255} // red-500
	if got := logLevelColor("error"); got != want {
		t.Errorf("logLevelColor(error) = %v, want %v", got, want)
	}
}

func TestLogLevelColor_Fatal(t *testing.T) {
	t.Parallel()
	want := color.NRGBA{R: 239, G: 68, B: 68, A: 255} // red-500 (same as error)
	if got := logLevelColor("fatal"); got != want {
		t.Errorf("logLevelColor(fatal) = %v, want %v", got, want)
	}
}

func TestLogLevelColor_Warn(t *testing.T) {
	t.Parallel()
	want := color.NRGBA{R: 234, G: 179, B: 8, A: 255} // yellow-500
	if got := logLevelColor("warn"); got != want {
		t.Errorf("logLevelColor(warn) = %v, want %v", got, want)
	}
}

func TestLogLevelColor_Warning(t *testing.T) {
	t.Parallel()
	want := color.NRGBA{R: 234, G: 179, B: 8, A: 255} // yellow-500
	if got := logLevelColor("warning"); got != want {
		t.Errorf("logLevelColor(warning) = %v, want %v", got, want)
	}
}

func TestLogLevelColor_Debug(t *testing.T) {
	t.Parallel()
	want := color.NRGBA{R: 156, G: 163, B: 175, A: 255} // gray-400
	if got := logLevelColor("debug"); got != want {
		t.Errorf("logLevelColor(debug) = %v, want %v", got, want)
	}
}

func TestLogLevelColor_Trace(t *testing.T) {
	t.Parallel()
	want := color.NRGBA{R: 156, G: 163, B: 175, A: 255} // gray-400
	if got := logLevelColor("trace"); got != want {
		t.Errorf("logLevelColor(trace) = %v, want %v", got, want)
	}
}

func TestLogLevelColor_Info_DefaultsToGray300(t *testing.T) {
	t.Parallel()
	want := color.NRGBA{R: 209, G: 213, B: 219, A: 255} // gray-300 (default)
	if got := logLevelColor("info"); got != want {
		t.Errorf("logLevelColor(info) = %v, want %v", got, want)
	}
}

func TestLogLevelColor_Empty_DefaultsToGray300(t *testing.T) {
	t.Parallel()
	want := color.NRGBA{R: 209, G: 213, B: 219, A: 255}
	if got := logLevelColor(""); got != want {
		t.Errorf("logLevelColor('') = %v, want %v (default gray-300)", got, want)
	}
}

func TestLogLevelColor_Unknown_DefaultsToGray300(t *testing.T) {
	t.Parallel()
	want := color.NRGBA{R: 209, G: 213, B: 219, A: 255}
	if got := logLevelColor("notice"); got != want {
		t.Errorf("logLevelColor(notice) = %v, want %v (default gray-300)", got, want)
	}
}

// Table-driven: all known levels return a non-default colour.
func TestLogLevelColor_AllKnownLevels(t *testing.T) {
	t.Parallel()
	defaultColor := color.NRGBA{R: 209, G: 213, B: 219, A: 255}
	knownLevels := []string{"error", "fatal", "warn", "warning", "debug", "trace"}
	for _, level := range knownLevels {
		level := level
		t.Run(level, func(t *testing.T) {
			t.Parallel()
			got := logLevelColor(level)
			if got == defaultColor {
				t.Errorf("logLevelColor(%q) returned default gray-300; expected a distinct colour", level)
			}
		})
	}
}

// Case-insensitivity: the function lowercases the input before switching.
func TestLogLevelColor_CaseInsensitive(t *testing.T) {
	t.Parallel()
	cases := []struct {
		level string
		same  string
	}{
		{"ERROR", "error"},
		{"WARN", "warn"},
		{"DEBUG", "debug"},
	}
	for _, tc := range cases {
		upper := logLevelColor(tc.level)
		lower := logLevelColor(tc.same)
		if upper != lower {
			t.Errorf("logLevelColor(%q) = %v, logLevelColor(%q) = %v; want equal", tc.level, upper, tc.same, lower)
		}
	}
}

// ---------------------------------------------------------------------------
// Table color constants — verify they are the documented design-system values
// ---------------------------------------------------------------------------

func TestTableColorConstants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		got   color.NRGBA
		want  color.NRGBA
		label string
	}{
		{"colorTableCard", colorTableCard, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, "white"},
		{"colorTableHeader", colorTableHeader, color.NRGBA{R: 243, G: 244, B: 246, A: 255}, "gray-100"},
		{"colorHeaderText", colorHeaderText, color.NRGBA{R: 55, G: 65, B: 81, A: 255}, "gray-700"},
		{"colorRowEven", colorRowEven, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, "white"},
		{"colorRowOdd", colorRowOdd, color.NRGBA{R: 249, G: 250, B: 251, A: 255}, "gray-50"},
		{"colorMuted", colorMuted, color.NRGBA{R: 107, G: 114, B: 128, A: 255}, "gray-500"},
		{"colorBtnBackground", colorBtnBackground, color.NRGBA{R: 229, G: 231, B: 235, A: 255}, "gray-200"},
		{"colorBtnText", colorBtnText, color.NRGBA{R: 55, G: 65, B: 81, A: 255}, "gray-700"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s = %v, want %v (%s)", tt.name, tt.got, tt.want, tt.label)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// max helper
// ---------------------------------------------------------------------------

func TestMax_ReturnsLarger(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a, b int
		want int
	}{
		{1, 2, 2},
		{5, 3, 5},
		{0, 0, 0},
		{-1, -3, -1},
		{7, 7, 7},
	}
	for _, tc := range cases {
		got := max(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("max(%d, %d) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
