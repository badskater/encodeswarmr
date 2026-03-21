package plugins

// RegisterBuiltins registers the standard encoding plugins that ship with the
// controller.  It is called once during server initialisation.
func RegisterBuiltins(r *Registry) error {
	builtins := []Plugin{
		{
			Name:        "x265",
			Version:     "3.6",
			Description: "x265 HEVC encoder — high-efficiency video codec encoder (libx265 / Handbrake-style CLI)",
			Enabled:     true,
			EncoderCmd:  "x265",
			DefaultArgs: []string{
				"--preset", "slow",
				"--crf", "18",
				"--output-depth", "10",
			},
			SupportedCodecs: []string{"hevc", "h265"},
		},
		{
			Name:        "x264",
			Version:     "164",
			Description: "x264 AVC encoder — battle-tested H.264 encoder with broad compatibility",
			Enabled:     true,
			EncoderCmd:  "x264",
			DefaultArgs: []string{
				"--preset", "slow",
				"--crf", "18",
				"--profile", "high",
				"--level", "4.1",
			},
			SupportedCodecs: []string{"avc", "h264"},
		},
		{
			Name:        "svt-av1",
			Version:     "2.1.0",
			Description: "SVT-AV1 encoder — scalable video technology AV1 encoder from Intel/Netflix",
			Enabled:     true,
			EncoderCmd:  "SvtAv1EncApp",
			DefaultArgs: []string{
				"--preset", "4",
				"--crf", "30",
				"--film-grain", "0",
			},
			SupportedCodecs: []string{"av1"},
		},
		{
			Name:        "ffmpeg-copy",
			Version:     "6.1",
			Description: "ffmpeg stream copy — remux without re-encoding; useful for container conversion",
			Enabled:     true,
			EncoderCmd:  "ffmpeg",
			DefaultArgs: []string{
				"-c:v", "copy",
				"-c:a", "copy",
			},
			SupportedCodecs: []string{"copy"},
		},
	}

	for _, p := range builtins {
		if err := r.Register(p); err != nil {
			return err
		}
	}
	return nil
}
