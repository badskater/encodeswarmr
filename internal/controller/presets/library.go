package presets

import (
	"fmt"
	"sort"
)

// builtins holds the registered built-in presets, keyed by name.
var builtins = map[string]*Preset{}

// audioBuiltins holds the registered built-in audio presets, keyed by name.
var audioBuiltins = map[string]*AudioPreset{}

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

// registerAudio adds audio presets to the built-in library. Panics on duplicate name.
func registerAudio(ps ...*AudioPreset) {
	for _, p := range ps {
		if _, exists := audioBuiltins[p.Name]; exists {
			panic(fmt.Sprintf("presets: duplicate audio preset name %q", p.Name))
		}
		audioBuiltins[p.Name] = p
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

// AllAudio returns all built-in audio presets as an ordered slice, sorted by
// category then name.
func AllAudio() []*AudioPreset {
	categoryOrder := []string{"lossless", "surround", "stereo", "legacy"}
	seen := map[string]bool{}
	result := make([]*AudioPreset, 0, len(audioBuiltins))

	for _, cat := range categoryOrder {
		// Collect and sort by name within category for stable ordering.
		var group []*AudioPreset
		for _, p := range audioBuiltins {
			if p.Category == cat {
				group = append(group, p)
				seen[p.Name] = true
			}
		}
		sort.Slice(group, func(i, j int) bool { return group[i].Name < group[j].Name })
		result = append(result, group...)
	}
	// Append any presets with unknown categories.
	var rest []*AudioPreset
	for _, p := range audioBuiltins {
		if !seen[p.Name] {
			rest = append(rest, p)
		}
	}
	sort.Slice(rest, func(i, j int) bool { return rest[i].Name < rest[j].Name })
	return append(result, rest...)
}

// GetAudio returns the named audio preset, or nil if not found.
func GetAudio(name string) *AudioPreset {
	return audioBuiltins[name]
}

func init() {
	registerAudio(
		&AudioPreset{
			Name:        "FLAC Lossless",
			Description: "Lossless FLAC audio. Ideal for archival; large files.",
			Category:    "lossless",
			Codec:       "flac",
			Params:      "-c:a flac -sample_fmt s16",
			Tags:        []string{"lossless", "archival"},
		},
		&AudioPreset{
			Name:        "PCM 24-bit",
			Description: "Uncompressed 24-bit PCM for studio-quality archival.",
			Category:    "lossless",
			Codec:       "pcm_s24le",
			Params:      "-c:a pcm_s24le",
			Tags:        []string{"lossless", "pcm"},
		},
		&AudioPreset{
			Name:        "TrueHD (Dolby Atmos passthrough)",
			Description: "Pass-through for TrueHD / Dolby Atmos tracks. Source must already be TrueHD.",
			Category:    "lossless",
			Codec:       "copy",
			Params:      "-c:a copy",
			Tags:        []string{"lossless", "atmos", "passthrough"},
		},
		&AudioPreset{
			Name:        "Opus 128k",
			Description: "Opus at 128 kbps stereo — excellent quality for streaming.",
			Category:    "stereo",
			Codec:       "libopus",
			Bitrate:     "128k",
			Params:      "-c:a libopus -b:a 128k",
			Tags:        []string{"streaming", "modern"},
		},
		&AudioPreset{
			Name:        "Opus 320k",
			Description: "Opus at 320 kbps stereo — transparent quality.",
			Category:    "stereo",
			Codec:       "libopus",
			Bitrate:     "320k",
			Params:      "-c:a libopus -b:a 320k",
			Tags:        []string{"quality", "modern"},
		},
		&AudioPreset{
			Name:        "AAC-LC 256k",
			Description: "AAC-LC at 256 kbps. Broad device compatibility.",
			Category:    "stereo",
			Codec:       "aac",
			Bitrate:     "256k",
			Params:      "-c:a aac -b:a 256k",
			Tags:        []string{"compatible", "streaming"},
		},
		&AudioPreset{
			Name:        "AAC-HE v2 64k",
			Description: "HE-AAC v2 at 64 kbps for low-bitrate stereo streaming.",
			Category:    "stereo",
			Codec:       "libfdk_aac",
			Bitrate:     "64k",
			Params:      "-c:a libfdk_aac -profile:a aac_he_v2 -b:a 64k",
			Tags:        []string{"low-bitrate", "streaming"},
		},
		&AudioPreset{
			Name:        "MP3 320k",
			Description: "MP3 at 320 kbps for maximum compatibility with legacy devices.",
			Category:    "legacy",
			Codec:       "libmp3lame",
			Bitrate:     "320k",
			Params:      "-c:a libmp3lame -b:a 320k",
			Tags:        []string{"legacy", "compatible"},
		},
		&AudioPreset{
			Name:        "Vorbis 192k",
			Description: "Vorbis at 192 kbps for open-source web streaming.",
			Category:    "legacy",
			Codec:       "libvorbis",
			Bitrate:     "192k",
			Params:      "-c:a libvorbis -b:a 192k",
			Tags:        []string{"open-source", "streaming"},
		},
		&AudioPreset{
			Name:        "AC3 640k (Dolby Digital)",
			Description: "Dolby Digital 5.1 at 640 kbps. Compatible with Blu-ray and most AV receivers.",
			Category:    "surround",
			Codec:       "ac3",
			Bitrate:     "640k",
			Params:      "-c:a ac3 -b:a 640k",
			Tags:        []string{"surround", "dolby", "compatible"},
		},
		&AudioPreset{
			Name:        "EAC3 1536k (Dolby Digital Plus)",
			Description: "Dolby Digital Plus at 1536 kbps for high-quality surround.",
			Category:    "surround",
			Codec:       "eac3",
			Bitrate:     "1536k",
			Params:      "-c:a eac3 -b:a 1536k",
			Tags:        []string{"surround", "dolby"},
		},
		&AudioPreset{
			Name:        "DTS 1536k",
			Description: "DTS core at 1536 kbps for AV receivers with DTS support.",
			Category:    "surround",
			Codec:       "dca",
			Bitrate:     "1536k",
			Params:      "-c:a dca -b:a 1536k -strict -2",
			Tags:        []string{"surround", "dts"},
		},
	)
}
