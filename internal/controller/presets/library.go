package presets

import "fmt"

// builtins holds the registered built-in presets, keyed by name.
var builtins = map[string]*Preset{}

func init() {
	register(
		&Preset{
			Name:        "4K HDR10 x265 Quality",
			Description: "High-quality 4K HDR10 encode for archival or distribution. Slow preset for maximum compression efficiency.",
			Category:    "4K",
			Codec:       "x265",
			Container:   "mkv",
			CRF:         18,
			Params:      "--preset slow --profile main10 --hdr10 --hdr10-opt --repeat-headers",
			HDRSupport:  true,
			TwoPass:     false,
			Tags:        []string{"hdr10", "quality", "slow"},
		},
		&Preset{
			Name:        "4K HDR10 x265 Balanced",
			Description: "Balanced 4K HDR10 encode. Medium preset trades some efficiency for significantly faster encodes.",
			Category:    "4K",
			Codec:       "x265",
			Container:   "mkv",
			CRF:         20,
			Params:      "--preset medium --profile main10 --hdr10 --hdr10-opt --repeat-headers",
			HDRSupport:  true,
			TwoPass:     false,
			Tags:        []string{"hdr10", "balanced"},
		},
		&Preset{
			Name:        "1080p x265 Quality",
			Description: "High-quality 1080p SDR encode. Slow preset produces small file sizes with excellent quality.",
			Category:    "1080p",
			Codec:       "x265",
			Container:   "mkv",
			CRF:         18,
			Params:      "--preset slow --profile main",
			HDRSupport:  false,
			TwoPass:     false,
			Tags:        []string{"quality", "slow"},
		},
		&Preset{
			Name:        "1080p x265 Fast",
			Description: "Fast 1080p encode suitable for batch jobs where speed matters more than file size.",
			Category:    "1080p",
			Codec:       "x265",
			Container:   "mkv",
			CRF:         22,
			Params:      "--preset fast --profile main",
			HDRSupport:  false,
			TwoPass:     false,
			Tags:        []string{"fast"},
		},
		&Preset{
			Name:        "1080p x264 Compatible",
			Description: "Broad-compatibility 1080p encode using x264. Plays on virtually any device without transcoding.",
			Category:    "1080p",
			Codec:       "x264",
			Container:   "mkv",
			CRF:         18,
			Params:      "--preset slow --profile high --level 4.1",
			HDRSupport:  false,
			TwoPass:     false,
			Tags:        []string{"compatible", "quality"},
		},
		&Preset{
			Name:        "Web Optimized H.264",
			Description: "MP4-container H.264 encode tuned for web delivery. Uses film tuning and faststart for progressive streaming.",
			Category:    "web",
			Codec:       "x264",
			Container:   "mp4",
			CRF:         23,
			Params:      "--preset medium --tune film --profile high --level 4.1 --movflags +faststart",
			HDRSupport:  false,
			TwoPass:     false,
			Tags:        []string{"web", "streaming", "compatible"},
		},
		&Preset{
			Name:        "Web Optimized AV1",
			Description: "Modern AV1 encode for web streaming. Excellent quality-per-bit at the cost of slower encoding.",
			Category:    "web",
			Codec:       "svt-av1",
			Container:   "mp4",
			CRF:         30,
			Params:      "--preset 6 --irefresh-type 2",
			HDRSupport:  false,
			TwoPass:     false,
			Tags:        []string{"web", "streaming", "av1"},
		},
		&Preset{
			Name:        "Archive Lossless",
			Description: "Lossless x264 encode (QP=0) for master archival. Files are large but bit-for-bit identical to source.",
			Category:    "archive",
			Codec:       "x264",
			Container:   "mkv",
			CRF:         0,
			Params:      "--qp 0 --preset ultrafast",
			HDRSupport:  false,
			TwoPass:     false,
			Tags:        []string{"lossless", "archive"},
		},
		&Preset{
			Name:        "Dolby Vision x265",
			Description: "Dolby Vision Profile 8.1 encode using x265. Requires DV RPU metadata already demuxed from source.",
			Category:    "4K",
			Codec:       "x265",
			Container:   "mkv",
			CRF:         18,
			Params:      "--preset slow --profile main10 --dolby-vision-profile 8.1 --hdr10 --hdr10-opt --repeat-headers",
			HDRSupport:  true,
			TwoPass:     false,
			Tags:        []string{"dolby-vision", "hdr10", "quality", "slow"},
		},
		&Preset{
			Name:        "HDR10+ x265",
			Description: "HDR10+ encode with dynamic metadata passthrough using x265. Requires HDR10+ JSON tone-mapping metadata.",
			Category:    "4K",
			Codec:       "x265",
			Container:   "mkv",
			CRF:         18,
			Params:      "--preset slow --profile main10 --hdr10 --hdr10plus-opt --repeat-headers",
			HDRSupport:  true,
			TwoPass:     false,
			Tags:        []string{"hdr10+", "quality", "slow"},
		},
	)
}

// register adds presets to the built-in library. Panics on duplicate name.
func register(ps ...*Preset) {
	for _, p := range ps {
		if _, exists := builtins[p.Name]; exists {
			panic(fmt.Sprintf("presets: duplicate preset name %q", p.Name))
		}
		builtins[p.Name] = p
	}
}

// All returns all built-in presets as an ordered slice, sorted by category
// then name. The slice is a copy; callers may not modify the returned presets.
func All() []*Preset {
	// Stable category order for UI display.
	categoryOrder := []string{"4K", "1080p", "web", "archive"}
	seen := map[string]bool{}
	result := make([]*Preset, 0, len(builtins))

	for _, cat := range categoryOrder {
		for _, p := range builtins {
			if p.Category == cat {
				result = append(result, p)
				seen[p.Name] = true
			}
		}
	}
	// Append any presets with unknown categories at the end.
	for _, p := range builtins {
		if !seen[p.Name] {
			result = append(result, p)
		}
	}
	return result
}

// Get returns the named preset, or nil if not found.
func Get(name string) *Preset {
	return builtins[name]
}
