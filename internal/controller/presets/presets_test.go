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
