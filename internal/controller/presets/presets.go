// Package presets provides the built-in encoding preset library.
//
// Presets are immutable, in-memory definitions that describe a full
// encoder configuration (codec, CRF, command-line params, container).
// The job creation UI and cost estimation API reference presets by name.
package presets

// Preset describes a complete encoding configuration.
type Preset struct {
	// Name is the unique human-readable identifier, e.g. "4K HDR10 x265 Quality".
	Name string `json:"name"`
	// Description is a short explanation of when to use this preset.
	Description string `json:"description"`
	// Category groups related presets: "4K", "1080p", "web", "archive".
	Category string `json:"category"`
	// Codec identifies the encoder binary: "x265", "x264", "svt-av1", "av1".
	Codec string `json:"codec"`
	// Container is the output file format: "mkv" or "mp4".
	Container string `json:"container"`
	// CRF is the constant-rate-factor quality value (0 = lossless).
	CRF int `json:"crf"`
	// Params is the full set of encoder command-line parameters that will be
	// injected into the generated batch/script file.
	Params string `json:"params"`
	// HDRSupport indicates the preset is configured for HDR passthrough or
	// encoding (HDR10, HDR10+, Dolby Vision).
	HDRSupport bool `json:"hdr_support"`
	// TwoPass indicates the preset uses two-pass encoding for better bitrate
	// control (not applicable for CRF-based encodes).
	TwoPass bool `json:"two_pass"`
	// Tags are optional labels for filtering in the UI, e.g. ["slow", "quality"].
	Tags []string `json:"tags"`
}

// DefaultFPSByCodec holds rough default encoding FPS assumptions used when
// no historical data is available for the cost estimator.
var DefaultFPSByCodec = map[string]float64{
	"x265":    15.0,
	"x264":    30.0,
	"svt-av1": 10.0,
	"av1":     8.0,
}
