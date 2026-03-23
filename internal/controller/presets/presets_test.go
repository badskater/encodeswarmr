package presets

import (
	"sort"
	"testing"
)

// ---------------------------------------------------------------------------
// All()
// ---------------------------------------------------------------------------

func TestAll_ReturnsTenPresets(t *testing.T) {
	all := All()
	if len(all) != 10 {
		t.Errorf("All() returned %d presets, want 10", len(all))
	}
}

func TestAll_ReturnsNonNilSlice(t *testing.T) {
	all := All()
	if all == nil {
		t.Fatal("All() returned nil slice")
	}
}

func TestAll_CategoryOrder(t *testing.T) {
	all := All()

	// Collect category sequence as seen in All() output.
	// 4K items must all come before 1080p items, 1080p before web, web before archive.
	categoryOrder := []string{"4K", "1080p", "web", "archive"}
	lastIdx := map[string]int{}
	for i, p := range all {
		lastIdx[p.Category] = i
	}
	firstIdx := map[string]int{}
	for i, p := range all {
		if _, seen := firstIdx[p.Category]; !seen {
			firstIdx[p.Category] = i
		}
	}

	for i := 0; i < len(categoryOrder)-1; i++ {
		catA := categoryOrder[i]
		catB := categoryOrder[i+1]
		if lastIdxA, ok := lastIdx[catA]; ok {
			if firstIdxB, ok := firstIdx[catB]; ok {
				if lastIdxA > firstIdxB {
					t.Errorf("category %q appears after category %q (want %q first)", catA, catB, catA)
				}
			}
		}
	}
}

func TestAll_AllPresetsHaveRequiredFields(t *testing.T) {
	for _, p := range All() {
		if p.Name == "" {
			t.Errorf("preset has empty Name: %+v", p)
		}
		if p.Category == "" {
			t.Errorf("preset %q has empty Category", p.Name)
		}
		if p.Codec == "" {
			t.Errorf("preset %q has empty Codec", p.Name)
		}
		if p.Container == "" {
			t.Errorf("preset %q has empty Container", p.Name)
		}
	}
}

func TestAll_NoDuplicateNames(t *testing.T) {
	seen := map[string]int{}
	for _, p := range All() {
		seen[p.Name]++
	}
	for name, count := range seen {
		if count > 1 {
			t.Errorf("duplicate preset name %q (count=%d)", name, count)
		}
	}
}

// ---------------------------------------------------------------------------
// Get()
// ---------------------------------------------------------------------------

func TestGet_ExistingPreset(t *testing.T) {
	// "4K HDR10 x265 Quality" is registered in library.go
	p := Get("4K HDR10 x265 Quality")
	if p == nil {
		t.Fatal("Get(\"4K HDR10 x265 Quality\") returned nil")
	}
	if p.Codec != "x265" {
		t.Errorf("Codec = %q, want x265", p.Codec)
	}
	if !p.HDRSupport {
		t.Error("HDRSupport = false, want true")
	}
	if p.Category != "4K" {
		t.Errorf("Category = %q, want 4K", p.Category)
	}
}

func TestGet_MissingPreset(t *testing.T) {
	p := Get("does not exist")
	if p != nil {
		t.Errorf("Get(missing) = %v, want nil", p)
	}
}

func TestGet_AllPresetsRetrievable(t *testing.T) {
	for _, p := range All() {
		got := Get(p.Name)
		if got == nil {
			t.Errorf("Get(%q) returned nil but preset is in All()", p.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// DefaultFPSByCodec
// ---------------------------------------------------------------------------

func TestDefaultFPSByCodec_ContainsExpectedCodecs(t *testing.T) {
	expected := []string{"x265", "x264", "svt-av1", "av1"}
	for _, codec := range expected {
		fps, ok := DefaultFPSByCodec[codec]
		if !ok {
			t.Errorf("DefaultFPSByCodec missing codec %q", codec)
			continue
		}
		if fps <= 0 {
			t.Errorf("DefaultFPSByCodec[%q] = %v, want > 0", codec, fps)
		}
	}
}

func TestDefaultFPSByCodec_SlowerCodecsHaveLowerFPS(t *testing.T) {
	// x264 (faster encoder) should have higher default FPS than x265 (slower).
	x265fps, ok265 := DefaultFPSByCodec["x265"]
	x264fps, ok264 := DefaultFPSByCodec["x264"]
	if !ok265 || !ok264 {
		t.Skip("x265 or x264 not in DefaultFPSByCodec")
	}
	if x264fps <= x265fps {
		t.Errorf("expected x264 FPS (%v) > x265 FPS (%v)", x264fps, x265fps)
	}
}

// ---------------------------------------------------------------------------
// Named preset properties
// ---------------------------------------------------------------------------

func TestPreset_SpecificValues(t *testing.T) {
	tests := []struct {
		name      string
		wantCRF   int
		wantCodec string
		wantHDR   bool
	}{
		{"4K HDR10 x265 Quality", 18, "x265", true},
		{"4K HDR10 x265 Balanced", 20, "x265", true},
		{"1080p x265 Quality", 18, "x265", false},
		{"1080p x265 Fast", 22, "x265", false},
		{"1080p x264 Compatible", 18, "x264", false},
		{"Web Optimized H.264", 23, "x264", false},
		{"Web Optimized AV1", 30, "svt-av1", false},
		{"Archive Lossless", 0, "x264", false},
		{"Dolby Vision x265", 18, "x265", true},
		{"HDR10+ x265", 18, "x265", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Get(tt.name)
			if p == nil {
				t.Fatalf("Get(%q) returned nil", tt.name)
			}
			if p.CRF != tt.wantCRF {
				t.Errorf("CRF = %d, want %d", p.CRF, tt.wantCRF)
			}
			if p.Codec != tt.wantCodec {
				t.Errorf("Codec = %q, want %q", p.Codec, tt.wantCodec)
			}
			if p.HDRSupport != tt.wantHDR {
				t.Errorf("HDRSupport = %v, want %v", p.HDRSupport, tt.wantHDR)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// All() — sorted within categories
// ---------------------------------------------------------------------------

func TestAll_PresetsWithinCategoryAreSorted(t *testing.T) {
	// Within each category bucket, All() iterates a map so order is non-deterministic.
	// What we CAN test is that grouping by category is correct: all 4K presets,
	// then all 1080p, etc. We verify that the names within each expected category
	// are all present.
	wantInCategory := map[string][]string{
		"4K":     {"4K HDR10 x265 Quality", "4K HDR10 x265 Balanced", "Dolby Vision x265", "HDR10+ x265"},
		"1080p":  {"1080p x265 Quality", "1080p x265 Fast", "1080p x264 Compatible"},
		"web":    {"Web Optimized H.264", "Web Optimized AV1"},
		"archive": {"Archive Lossless"},
	}

	byCategory := map[string][]string{}
	for _, p := range All() {
		byCategory[p.Category] = append(byCategory[p.Category], p.Name)
	}

	for cat, wantNames := range wantInCategory {
		gotNames := byCategory[cat]
		sort.Strings(gotNames)
		sort.Strings(wantNames)
		if len(gotNames) != len(wantNames) {
			t.Errorf("category %q: got %d presets, want %d", cat, len(gotNames), len(wantNames))
			continue
		}
		for i, wn := range wantNames {
			if gotNames[i] != wn {
				t.Errorf("category %q: preset[%d] = %q, want %q", cat, i, gotNames[i], wn)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// AllAudio()
// ---------------------------------------------------------------------------

func TestAllAudio_ReturnsTwelvePresets(t *testing.T) {
	all := AllAudio()
	if len(all) != 12 {
		t.Errorf("AllAudio() returned %d presets, want 12", len(all))
	}
}

func TestAllAudio_ReturnsNonNilSlice(t *testing.T) {
	if AllAudio() == nil {
		t.Fatal("AllAudio() returned nil slice")
	}
}

func TestAllAudio_AllPresetsHaveRequiredFields(t *testing.T) {
	for _, p := range AllAudio() {
		if p.Name == "" {
			t.Errorf("audio preset has empty Name: %+v", p)
		}
		if p.Category == "" {
			t.Errorf("audio preset %q has empty Category", p.Name)
		}
		if p.Codec == "" {
			t.Errorf("audio preset %q has empty Codec", p.Name)
		}
		if p.Params == "" {
			t.Errorf("audio preset %q has empty Params", p.Name)
		}
	}
}

func TestAllAudio_NoDuplicateNames(t *testing.T) {
	seen := map[string]int{}
	for _, p := range AllAudio() {
		seen[p.Name]++
	}
	for name, count := range seen {
		if count > 1 {
			t.Errorf("duplicate audio preset name %q (count=%d)", name, count)
		}
	}
}

func TestAllAudio_CategoryOrder(t *testing.T) {
	// lossless must all come before surround, surround before stereo, stereo before legacy.
	categoryOrder := []string{"lossless", "surround", "stereo", "legacy"}
	firstIdx := map[string]int{}
	lastIdx := map[string]int{}
	for i, p := range AllAudio() {
		if _, seen := firstIdx[p.Category]; !seen {
			firstIdx[p.Category] = i
		}
		lastIdx[p.Category] = i
	}
	for i := 0; i < len(categoryOrder)-1; i++ {
		catA := categoryOrder[i]
		catB := categoryOrder[i+1]
		if la, ok := lastIdx[catA]; ok {
			if fb, ok2 := firstIdx[catB]; ok2 {
				if la > fb {
					t.Errorf("category %q appears after first item of category %q", catA, catB)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// GetAudio()
// ---------------------------------------------------------------------------

func TestGetAudio_KnownPreset(t *testing.T) {
	p := GetAudio("FLAC Lossless")
	if p == nil {
		t.Fatal("GetAudio(\"FLAC Lossless\") returned nil")
	}
	if p.Codec != "flac" {
		t.Errorf("Codec = %q, want flac", p.Codec)
	}
	if p.Category != "lossless" {
		t.Errorf("Category = %q, want lossless", p.Category)
	}
}

func TestGetAudio_UnknownPreset_ReturnsNil(t *testing.T) {
	if p := GetAudio("no such preset"); p != nil {
		t.Errorf("GetAudio(unknown) = %v, want nil", p)
	}
}

func TestGetAudio_AllPresetsRetrievable(t *testing.T) {
	for _, p := range AllAudio() {
		got := GetAudio(p.Name)
		if got == nil {
			t.Errorf("GetAudio(%q) returned nil but preset is in AllAudio()", p.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// Audio codec names — each must be a valid ffmpeg codec string
// ---------------------------------------------------------------------------

func TestAllAudio_ValidFfmpegCodecNames(t *testing.T) {
	// Known valid ffmpeg audio codec names used in this project.
	validCodecs := map[string]bool{
		"flac":        true,
		"pcm_s24le":   true,
		"copy":        true,
		"libopus":     true,
		"aac":         true,
		"libfdk_aac":  true,
		"libmp3lame":  true,
		"libvorbis":   true,
		"ac3":         true,
		"eac3":        true,
		"dca":         true,
	}

	for _, p := range AllAudio() {
		if !validCodecs[p.Codec] {
			t.Errorf("audio preset %q has unrecognised ffmpeg codec %q", p.Name, p.Codec)
		}
	}
}

// ---------------------------------------------------------------------------
// Specific audio preset values
// ---------------------------------------------------------------------------

func TestAudioPreset_SpecificValues(t *testing.T) {
	tests := []struct {
		name      string
		wantCodec string
		wantCat   string
	}{
		{"FLAC Lossless", "flac", "lossless"},
		{"PCM 24-bit", "pcm_s24le", "lossless"},
		{"TrueHD (Dolby Atmos passthrough)", "copy", "lossless"},
		{"Opus 128k", "libopus", "stereo"},
		{"Opus 320k", "libopus", "stereo"},
		{"AAC-LC 256k", "aac", "stereo"},
		{"AAC-HE v2 64k", "libfdk_aac", "stereo"},
		{"MP3 320k", "libmp3lame", "legacy"},
		{"Vorbis 192k", "libvorbis", "legacy"},
		{"AC3 640k (Dolby Digital)", "ac3", "surround"},
		{"EAC3 1536k (Dolby Digital Plus)", "eac3", "surround"},
		{"DTS 1536k", "dca", "surround"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := GetAudio(tt.name)
			if p == nil {
				t.Fatalf("GetAudio(%q) returned nil", tt.name)
			}
			if p.Codec != tt.wantCodec {
				t.Errorf("Codec = %q, want %q", p.Codec, tt.wantCodec)
			}
			if p.Category != tt.wantCat {
				t.Errorf("Category = %q, want %q", p.Category, tt.wantCat)
			}
		})
	}
}
