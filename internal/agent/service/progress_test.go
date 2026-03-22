package service

import (
	"testing"

	pb "github.com/badskater/encodeswarmr/internal/proto/encoderv1"
)

// --- parseProgress: x265 format ---

func TestParseProgress_X265_Basic(t *testing.T) {
	line := "[12.30%] 1200/9750 frames, 24.50 fps, eta 0:01:23"
	pm := parseProgress(line)
	if pm == nil {
		t.Fatal("parseProgress returned nil for x265 line")
	}
	if pm.Frame != 1200 {
		t.Errorf("Frame = %d, want 1200", pm.Frame)
	}
	if pm.TotalFrames != 9750 {
		t.Errorf("TotalFrames = %d, want 9750", pm.TotalFrames)
	}
	if pm.FPS < 24.4 || pm.FPS > 24.6 {
		t.Errorf("FPS = %f, want ~24.50", pm.FPS)
	}
	if pm.Percent < 12.2 || pm.Percent > 12.4 {
		t.Errorf("Percent = %f, want ~12.30", pm.Percent)
	}
	// ETA: 0h 1m 23s = 83 seconds
	if pm.ETASec != 83 {
		t.Errorf("ETASec = %d, want 83", pm.ETASec)
	}
}

func TestParseProgress_X265_NoETA(t *testing.T) {
	line := "[50.00%] 4875/9750 frames, 30.00 fps"
	pm := parseProgress(line)
	if pm == nil {
		t.Fatal("parseProgress returned nil")
	}
	if pm.ETASec != 0 {
		t.Errorf("ETASec = %d, want 0 when no eta present", pm.ETASec)
	}
	if pm.Percent < 49.9 || pm.Percent > 50.1 {
		t.Errorf("Percent = %f, want 50.00", pm.Percent)
	}
}

func TestParseProgress_X265_ETAWithHours(t *testing.T) {
	line := "[5.00%] 500/10000 frames, 10.00 fps, eta 1:02:03"
	pm := parseProgress(line)
	if pm == nil {
		t.Fatal("parseProgress returned nil")
	}
	// 1h 2m 3s = 3723 seconds
	if pm.ETASec != 3723 {
		t.Errorf("ETASec = %d, want 3723", pm.ETASec)
	}
}

// --- parseProgress: x264 format ---

func TestParseProgress_X264_Basic(t *testing.T) {
	line := "1200/9750 frames, 48.22 fps"
	pm := parseProgress(line)
	if pm == nil {
		t.Fatal("parseProgress returned nil for x264 line")
	}
	if pm.Frame != 1200 {
		t.Errorf("Frame = %d, want 1200", pm.Frame)
	}
	if pm.TotalFrames != 9750 {
		t.Errorf("TotalFrames = %d, want 9750", pm.TotalFrames)
	}
	if pm.FPS < 48.1 || pm.FPS > 48.3 {
		t.Errorf("FPS = %f, want ~48.22", pm.FPS)
	}
	wantPct := float32(1200) / float32(9750) * 100
	if pm.Percent < wantPct-0.1 || pm.Percent > wantPct+0.1 {
		t.Errorf("Percent = %f, want ~%f", pm.Percent, wantPct)
	}
}

func TestParseProgress_X264_Leading_Whitespace(t *testing.T) {
	line := "   500/1000 frames, 25.00 fps"
	pm := parseProgress(line)
	if pm == nil {
		t.Fatal("parseProgress returned nil (x264 with leading whitespace)")
	}
	if pm.Frame != 500 {
		t.Errorf("Frame = %d, want 500", pm.Frame)
	}
}

// --- parseProgress: SVT-AV1 format ---

func TestParseProgress_SVT_Basic(t *testing.T) {
	line := "Encoding frame 1200/9750"
	pm := parseProgress(line)
	if pm == nil {
		t.Fatal("parseProgress returned nil for SVT-AV1 line")
	}
	if pm.Frame != 1200 {
		t.Errorf("Frame = %d, want 1200", pm.Frame)
	}
	if pm.TotalFrames != 9750 {
		t.Errorf("TotalFrames = %d, want 9750", pm.TotalFrames)
	}
	wantPct := float32(1200) / float32(9750) * 100
	if pm.Percent < wantPct-0.1 || pm.Percent > wantPct+0.1 {
		t.Errorf("Percent = %f, want ~%f", pm.Percent, wantPct)
	}
}

func TestParseProgress_SVT_FirstFrame(t *testing.T) {
	line := "Encoding frame 1/5000"
	pm := parseProgress(line)
	if pm == nil {
		t.Fatal("parseProgress returned nil")
	}
	if pm.Frame != 1 || pm.TotalFrames != 5000 {
		t.Errorf("Frame=%d TotalFrames=%d, want 1/5000", pm.Frame, pm.TotalFrames)
	}
}

// --- parseProgress: FFmpeg format ---

func TestParseProgress_FFmpeg_Basic(t *testing.T) {
	line := "frame=  1200 fps= 24 size=   12345kB time=00:00:50.00 bitrate= 2000kbps speed=0.95x"
	pm := parseProgress(line)
	if pm == nil {
		t.Fatal("parseProgress returned nil for FFmpeg line")
	}
	if pm.Frame != 1200 {
		t.Errorf("Frame = %d, want 1200", pm.Frame)
	}
	if pm.FPS < 23.9 || pm.FPS > 24.1 {
		t.Errorf("FPS = %f, want ~24", pm.FPS)
	}
	// FFmpeg lines don't carry total frames.
	if pm.TotalFrames != 0 {
		t.Errorf("TotalFrames = %d, want 0 for FFmpeg lines", pm.TotalFrames)
	}
}

func TestParseProgress_FFmpeg_DecimalFPS(t *testing.T) {
	line := "frame=500 fps=29.97 size=1024kB"
	pm := parseProgress(line)
	if pm == nil {
		t.Fatal("parseProgress returned nil")
	}
	if pm.FPS < 29.9 || pm.FPS > 30.1 {
		t.Errorf("FPS = %f, want ~29.97", pm.FPS)
	}
}

// --- parseProgress: no match ---

func TestParseProgress_NoMatch(t *testing.T) {
	cases := []string{
		"",
		"  ",
		"some random log line",
		"INFO: encoding started",
		"[warn] low disk space",
	}
	for _, c := range cases {
		if pm := parseProgress(c); pm != nil {
			t.Errorf("parseProgress(%q) = %+v, want nil", c, pm)
		}
	}
}

// TestParseProgress_SVT_ZeroTotal verifies that a zero total frame count does
// not cause a division-by-zero in the SVT-AV1 parser.
func TestParseProgress_SVT_ZeroTotal(t *testing.T) {
	line := "Encoding frame 0/0"
	pm := parseProgress(line)
	if pm == nil {
		t.Fatal("parseProgress returned nil for SVT zero/zero line")
	}
	if pm.Percent != 0 {
		t.Errorf("Percent = %f, want 0 for zero total frames", pm.Percent)
	}
}

// TestParseProgress_X264_ZeroTotal verifies that a zero total frame count
// does not panic in the x264 parser.
func TestParseProgress_X264_ZeroTotal(t *testing.T) {
	line := "0/0 frames, 0.00 fps"
	pm := parseProgress(line)
	if pm == nil {
		t.Fatal("parseProgress returned nil for x264 zero/zero line")
	}
	if pm.Percent != 0 {
		t.Errorf("Percent = %f, want 0", pm.Percent)
	}
}

// TestParseProgress_FFmpeg_ZeroFPS verifies that a zero fps value is handled.
func TestParseProgress_FFmpeg_ZeroFPS(t *testing.T) {
	line := "frame=0 fps=0"
	pm := parseProgress(line)
	if pm == nil {
		t.Fatal("parseProgress returned nil for ffmpeg zero fps line")
	}
	if pm.FPS != 0 {
		t.Errorf("FPS = %f, want 0", pm.FPS)
	}
}

// TestParseProgress_FFmpeg_SpacedFormat verifies the spaced-out FFmpeg format
// used in real ffmpeg output.
func TestParseProgress_FFmpeg_SpacedFormat(t *testing.T) {
	line := "frame=   500 fps=  25.0 size=   2048kB time=00:00:20.00 bitrate=819.2kbits/s speed=1.05x"
	pm := parseProgress(line)
	if pm == nil {
		t.Fatal("parseProgress returned nil for spaced FFmpeg line")
	}
	if pm.Frame != 500 {
		t.Errorf("Frame = %d, want 500", pm.Frame)
	}
}

// TestParseProgress_X265_LargeValues verifies x265 format with large frame counts.
func TestParseProgress_X265_LargeValues(t *testing.T) {
	line := "[99.99%] 99990/100000 frames, 120.00 fps, eta 0:00:01"
	pm := parseProgress(line)
	if pm == nil {
		t.Fatal("parseProgress returned nil for large-value x265 line")
	}
	if pm.Frame != 99990 {
		t.Errorf("Frame = %d, want 99990", pm.Frame)
	}
	if pm.TotalFrames != 100000 {
		t.Errorf("TotalFrames = %d, want 100000", pm.TotalFrames)
	}
	if pm.ETASec != 1 {
		t.Errorf("ETASec = %d, want 1", pm.ETASec)
	}
}

// --- stateLabel (health.go) ---

func TestStateLabel_AllStates(t *testing.T) {
	cases := []struct {
		input    pb.AgentState
		expected string
	}{
		{pb.AgentState_AGENT_STATE_UNSPECIFIED, "UNKNOWN"},
		{pb.AgentState_AGENT_STATE_IDLE, "IDLE"},
		{pb.AgentState_AGENT_STATE_BUSY, "BUSY"},
		{pb.AgentState_AGENT_STATE_DRAINING, "DRAINING"},
		{pb.AgentState_AGENT_STATE_OFFLINE, "OFFLINE"},
		{pb.AgentState(99), "UNKNOWN"},
	}

	for _, tc := range cases {
		got := stateLabel(tc.input)
		if got != tc.expected {
			t.Errorf("stateLabel(%v) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

// --- stateGauge (health.go) ---

func TestStateGauge_AllStates(t *testing.T) {
	cases := []struct {
		input    pb.AgentState
		expected int
	}{
		{pb.AgentState_AGENT_STATE_IDLE, 1},
		{pb.AgentState_AGENT_STATE_BUSY, 2},
		{pb.AgentState_AGENT_STATE_UNSPECIFIED, 0},
		{pb.AgentState_AGENT_STATE_DRAINING, 0},
		{pb.AgentState_AGENT_STATE_OFFLINE, 0},
	}
	for _, tc := range cases {
		got := stateGauge(tc.input)
		if got != tc.expected {
			t.Errorf("stateGauge(%v) = %d, want %d", tc.input, got, tc.expected)
		}
	}
}

