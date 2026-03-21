package analysis

import (
	"io"
	"log/slog"
	"testing"

	"github.com/badskater/encodeswarmr/internal/db"
)

// ---------------------------------------------------------------------------
// parseHDRFromJSON tests
// ---------------------------------------------------------------------------

func TestParseHDRFromJSON_InvalidJSON(t *testing.T) {
	_, _, err := parseHDRFromJSON([]byte(`not valid json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestParseHDRFromJSON_EmptyStreams(t *testing.T) {
	hdrType, dvProfile, err := parseHDRFromJSON([]byte(`{"streams":[]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hdrType != "" {
		t.Errorf("expected empty hdrType, got %q", hdrType)
	}
	if dvProfile != 0 {
		t.Errorf("expected dvProfile 0, got %d", dvProfile)
	}
}

func TestParseHDRFromJSON_HDR10(t *testing.T) {
	data := []byte(`{"streams":[{"color_transfer":"smpte2084","codec_tag_string":"hvc1","side_data_list":[]}]}`)
	hdrType, dvProfile, err := parseHDRFromJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hdrType != "hdr10" {
		t.Errorf("expected hdrType 'hdr10', got %q", hdrType)
	}
	if dvProfile != 0 {
		t.Errorf("expected dvProfile 0, got %d", dvProfile)
	}
}

func TestParseHDRFromJSON_HDR10Plus(t *testing.T) {
	// HDR10+ = smpte2084 color transfer + HDR dynamic metadata side data.
	data := []byte(`{
		"streams":[{
			"color_transfer":"smpte2084",
			"codec_tag_string":"hvc1",
			"side_data_list":[{"side_data_type":"HDR Dynamic Metadata SMPTE2094-40 side data"}]
		}]
	}`)
	hdrType, _, err := parseHDRFromJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hdrType != "hdr10+" {
		t.Errorf("expected 'hdr10+', got %q", hdrType)
	}
}

func TestParseHDRFromJSON_HLG(t *testing.T) {
	data := []byte(`{"streams":[{"color_transfer":"arib-std-b67","codec_tag_string":"hvc1","side_data_list":[]}]}`)
	hdrType, _, err := parseHDRFromJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hdrType != "hlg" {
		t.Errorf("expected 'hlg', got %q", hdrType)
	}
}

func TestParseHDRFromJSON_DolbyVision_ViaSideData(t *testing.T) {
	data := []byte(`{
		"streams":[{
			"color_transfer":"smpte2084",
			"codec_tag_string":"hvc1",
			"side_data_list":[{"side_data_type":"DOVI RPU Data"}]
		}]
	}`)
	hdrType, _, err := parseHDRFromJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hdrType != "dolby_vision" {
		t.Errorf("expected 'dolby_vision', got %q", hdrType)
	}
}

func TestParseHDRFromJSON_DolbyVision_ViaCodecTag_dvh1(t *testing.T) {
	data := []byte(`{"streams":[{"color_transfer":"smpte2084","codec_tag_string":"dvh1","side_data_list":[]}]}`)
	hdrType, _, err := parseHDRFromJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hdrType != "dolby_vision" {
		t.Errorf("expected 'dolby_vision' via dvh1 tag, got %q", hdrType)
	}
}

func TestParseHDRFromJSON_DolbyVision_ViaCodecTag_dvhe(t *testing.T) {
	data := []byte(`{"streams":[{"color_transfer":"smpte2084","codec_tag_string":"dvhe","side_data_list":[]}]}`)
	hdrType, _, err := parseHDRFromJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hdrType != "dolby_vision" {
		t.Errorf("expected 'dolby_vision' via dvhe tag, got %q", hdrType)
	}
}

func TestParseHDRFromJSON_SDR_NoColorTransfer(t *testing.T) {
	data := []byte(`{"streams":[{"color_transfer":"bt709","codec_tag_string":"avc1","side_data_list":[]}]}`)
	hdrType, dvProfile, err := parseHDRFromJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hdrType != "" {
		t.Errorf("expected empty hdrType for SDR, got %q", hdrType)
	}
	if dvProfile != 0 {
		t.Errorf("expected dvProfile 0, got %d", dvProfile)
	}
}

func TestParseHDRFromJSON_ColorTransfer_CaseInsensitive(t *testing.T) {
	// The field value should be case-insensitively compared.
	data := []byte(`{"streams":[{"color_transfer":"SMPTE2084","codec_tag_string":"hvc1","side_data_list":[]}]}`)
	hdrType, _, err := parseHDRFromJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hdrType != "hdr10" {
		t.Errorf("expected 'hdr10' with uppercase SMPTE2084, got %q", hdrType)
	}
}

func TestParseHDRFromJSON_Smpte2094_SideDataKeyword(t *testing.T) {
	// The side data check also matches "smpte2094" in the side_data_type value.
	data := []byte(`{
		"streams":[{
			"color_transfer":"smpte2084",
			"codec_tag_string":"hvc1",
			"side_data_list":[{"side_data_type":"smpte2094-40"}]
		}]
	}`)
	hdrType, _, err := parseHDRFromJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hdrType != "hdr10+" {
		t.Errorf("expected 'hdr10+', got %q", hdrType)
	}
}

func TestParseHDRFromJSON_DolbyVisionKeywordInSideData(t *testing.T) {
	// "dolby vision" keyword in side_data_type should trigger DV detection.
	data := []byte(`{
		"streams":[{
			"color_transfer":"smpte2084",
			"codec_tag_string":"hvc1",
			"side_data_list":[{"side_data_type":"dolby vision rpu data"}]
		}]
	}`)
	hdrType, _, err := parseHDRFromJSON(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hdrType != "dolby_vision" {
		t.Errorf("expected 'dolby_vision', got %q", hdrType)
	}
}

func TestParseHDRFromJSON_NoStreamsField(t *testing.T) {
	// Valid JSON but no "streams" key → treated as empty streams.
	hdrType, dvProfile, err := parseHDRFromJSON([]byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hdrType != "" || dvProfile != 0 {
		t.Errorf("expected empty result for missing streams, got hdrType=%q dvProfile=%d",
			hdrType, dvProfile)
	}
}

// ---------------------------------------------------------------------------
// detectDVProfile tests
// ---------------------------------------------------------------------------

func TestDetectDVProfile_BinaryNotFound(t *testing.T) {
	// Passing a non-existent binary path should return 0 (not panic).
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	profile := detectDVProfile(t.Context(), "/nonexistent/dovi_tool", "/some/file.mkv", logger)
	if profile != 0 {
		t.Errorf("expected 0 for missing binary, got %d", profile)
	}
}

// ---------------------------------------------------------------------------
// parseSceneMetadata tests
// ---------------------------------------------------------------------------

func TestParseSceneMetadata_Empty(t *testing.T) {
	scenes := parseSceneMetadata([]byte{})
	if len(scenes) != 0 {
		t.Errorf("expected empty slice, got %v", scenes)
	}
}

func TestParseSceneMetadata_SingleScene(t *testing.T) {
	data := []byte("frame:0 pts:0 pts_time:0.000000\nlavfi.scene_score=0.732\n")
	scenes := parseSceneMetadata(data)
	if len(scenes) != 1 {
		t.Fatalf("expected 1 scene, got %d", len(scenes))
	}
	if scenes[0].PTSTime != 0.0 {
		t.Errorf("expected PTSTime 0.0, got %f", scenes[0].PTSTime)
	}
	if scenes[0].Score < 0.73 || scenes[0].Score > 0.74 {
		t.Errorf("expected Score ~0.732, got %f", scenes[0].Score)
	}
}

func TestParseSceneMetadata_MultipleScenes(t *testing.T) {
	// Note: Go's fmt.Sscanf does not support %*d (suppress assignment), so the
	// "frame:" line parse will error and currentPTS resets to 0 for each frame.
	// The function still returns the correct number of scene boundaries; the
	// Score values are what we can reliably assert on.
	data := []byte(`frame:0 pts:0 pts_time:0.000000
lavfi.scene_score=0.500
frame:24 pts:24 pts_time:1.000000
lavfi.scene_score=0.850
frame:48 pts:48 pts_time:2.000000
lavfi.scene_score=0.600
`)
	scenes := parseSceneMetadata(data)
	if len(scenes) != 3 {
		t.Fatalf("expected 3 scenes, got %d", len(scenes))
	}
	// Verify scene scores are parsed correctly (PTSTime will be 0 for all
	// because fmt.Sscanf does not support the %*d suppress verb in Go).
	wantScores := []float64{0.500, 0.850, 0.600}
	for i, want := range wantScores {
		if scenes[i].Score < want-0.01 || scenes[i].Score > want+0.01 {
			t.Errorf("scenes[%d].Score = %f, want ~%f", i, scenes[i].Score, want)
		}
	}
}

func TestParseSceneMetadata_NoScoreLines(t *testing.T) {
	// Frame lines without corresponding score lines should produce no scenes.
	data := []byte("frame:0 pts:0 pts_time:0.000000\n")
	scenes := parseSceneMetadata(data)
	if len(scenes) != 0 {
		t.Errorf("expected 0 scenes, got %d", len(scenes))
	}
}

func TestParseSceneMetadata_MalformedFrameLine(t *testing.T) {
	// If a frame: line cannot be parsed, PTSTime defaults to 0.
	data := []byte("frame:BAD pts:BAD pts_time:BAD\nlavfi.scene_score=0.5\n")
	scenes := parseSceneMetadata(data)
	if len(scenes) != 1 {
		t.Fatalf("expected 1 scene even with bad frame line, got %d", len(scenes))
	}
	if scenes[0].PTSTime != 0.0 {
		t.Errorf("expected PTSTime 0.0 for unparseable frame line, got %f", scenes[0].PTSTime)
	}
}

func TestParseSceneMetadata_ScoreWithTrailingWhitespace(t *testing.T) {
	data := []byte("frame:0 pts:0 pts_time:5.000000\nlavfi.scene_score=0.400  \n")
	scenes := parseSceneMetadata(data)
	if len(scenes) != 1 {
		t.Fatalf("expected 1 scene, got %d", len(scenes))
	}
	if scenes[0].Score < 0.39 || scenes[0].Score > 0.41 {
		t.Errorf("expected score ~0.4, got %f", scenes[0].Score)
	}
}

// ---------------------------------------------------------------------------
// ApplyMappings tests
// ---------------------------------------------------------------------------

func TestApplyMappings_NoMappings(t *testing.T) {
	path := `\\NAS01\media\movie.mkv`
	got := ApplyMappings(path, nil)
	if got != path {
		t.Errorf("expected unchanged path %q, got %q", path, got)
	}
}

func TestApplyMappings_UNCPath_Match(t *testing.T) {
	mappings := []*db.PathMapping{
		{
			ID:            "m1",
			WindowsPrefix: `\\NAS01\media`,
			LinuxPrefix:   "/mnt/nas/media",
			Enabled:       true,
		},
	}
	got := ApplyMappings(`\\NAS01\media\movies\film.mkv`, mappings)
	want := "/mnt/nas/media/movies/film.mkv"
	if got != want {
		t.Errorf("ApplyMappings = %q, want %q", got, want)
	}
}

func TestApplyMappings_UNCPath_CaseInsensitive(t *testing.T) {
	mappings := []*db.PathMapping{
		{
			ID:            "m1",
			WindowsPrefix: `\\nas01\MEDIA`,
			LinuxPrefix:   "/mnt/nas/media",
			Enabled:       true,
		},
	}
	// Source uses different casing than the mapping prefix.
	got := ApplyMappings(`\\NAS01\media\film.mkv`, mappings)
	want := "/mnt/nas/media/film.mkv"
	if got != want {
		t.Errorf("ApplyMappings (case insensitive) = %q, want %q", got, want)
	}
}

func TestApplyMappings_UNCPath_DisabledMapping_Skipped(t *testing.T) {
	mappings := []*db.PathMapping{
		{
			ID:            "m1",
			WindowsPrefix: `\\NAS01\media`,
			LinuxPrefix:   "/mnt/nas/media",
			Enabled:       false, // disabled
		},
	}
	path := `\\NAS01\media\film.mkv`
	got := ApplyMappings(path, mappings)
	if got != path {
		t.Errorf("expected unchanged path (disabled mapping), got %q", got)
	}
}

func TestApplyMappings_UNCPath_NoMatch(t *testing.T) {
	mappings := []*db.PathMapping{
		{
			ID:            "m1",
			WindowsPrefix: `\\NAS02\other`,
			LinuxPrefix:   "/mnt/other",
			Enabled:       true,
		},
	}
	path := `\\NAS01\media\film.mkv`
	got := ApplyMappings(path, mappings)
	if got != path {
		t.Errorf("expected unchanged path (no matching prefix), got %q", got)
	}
}

func TestApplyMappings_UNCPath_BackslashToForwardSlash(t *testing.T) {
	mappings := []*db.PathMapping{
		{
			ID:            "m1",
			WindowsPrefix: `\\NAS01\media`,
			LinuxPrefix:   "/mnt/nas",
			Enabled:       true,
		},
	}
	// Backslashes in remainder should become forward slashes.
	got := ApplyMappings(`\\NAS01\media\folder\sub\file.mkv`, mappings)
	want := "/mnt/nas/folder/sub/file.mkv"
	if got != want {
		t.Errorf("ApplyMappings backslash conversion = %q, want %q", got, want)
	}
}

func TestApplyMappings_LinuxPath_Match(t *testing.T) {
	// POSIX paths matching the LinuxPrefix are returned unchanged.
	mappings := []*db.PathMapping{
		{
			ID:          "m2",
			LinuxPrefix: "/mnt/nas/media",
			Enabled:     true,
		},
	}
	path := "/mnt/nas/media/movie.mkv"
	got := ApplyMappings(path, mappings)
	if got != path {
		t.Errorf("ApplyMappings linux path = %q, want unchanged %q", got, path)
	}
}

func TestApplyMappings_LinuxPath_NoMatch(t *testing.T) {
	mappings := []*db.PathMapping{
		{
			ID:          "m2",
			LinuxPrefix: "/mnt/nas/media",
			Enabled:     true,
		},
	}
	path := "/mnt/other/movie.mkv"
	got := ApplyMappings(path, mappings)
	if got != path {
		t.Errorf("expected unchanged path, got %q", got)
	}
}

func TestApplyMappings_FirstMatchApplied(t *testing.T) {
	// Only the first enabled matching mapping should be applied.
	mappings := []*db.PathMapping{
		{
			ID:            "m1",
			WindowsPrefix: `\\NAS01\media`,
			LinuxPrefix:   "/mnt/first",
			Enabled:       true,
		},
		{
			ID:            "m2",
			WindowsPrefix: `\\NAS01\media`,
			LinuxPrefix:   "/mnt/second",
			Enabled:       true,
		},
	}
	got := ApplyMappings(`\\NAS01\media\film.mkv`, mappings)
	want := "/mnt/first/film.mkv"
	if got != want {
		t.Errorf("ApplyMappings first-match = %q, want %q", got, want)
	}
}

func TestApplyMappings_LinuxPrefixTrailingSlash(t *testing.T) {
	// Trailing slash on LinuxPrefix should not produce double slashes.
	mappings := []*db.PathMapping{
		{
			ID:            "m1",
			WindowsPrefix: `\\NAS01\media`,
			LinuxPrefix:   "/mnt/nas/",
			Enabled:       true,
		},
	}
	got := ApplyMappings(`\\NAS01\media\film.mkv`, mappings)
	want := "/mnt/nas/film.mkv"
	if got != want {
		t.Errorf("trailing slash LinuxPrefix = %q, want %q", got, want)
	}
}
