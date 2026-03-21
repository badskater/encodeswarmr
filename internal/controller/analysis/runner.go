// Package analysis implements controller-side media analysis: HDR/DV
// detection, scene scanning, stream metadata, and audio encoding.
//
// Jobs of type "hdr_detect", "analysis", and "audio" are executed directly
// on the controller host via ffprobe/ffmpeg instead of being dispatched to a
// Windows agent.  Source files on the NAS are accessed through Linux NFS
// mount paths resolved via the path_mappings table.
package analysis

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/badskater/encodeswarmr/internal/db"
)

// Config holds the analysis runner settings.
type Config struct {
	FFmpegBin   string
	FFprobeBin  string
	DoviToolBin string
	Concurrency int
}

// Runner executes analysis jobs directly on the controller host.
type Runner struct {
	store      db.Store
	ffmpegBin  string
	ffprobeBin string
	doviTool   string
	logger     *slog.Logger
	sem        chan struct{} // nil = unlimited
}

// New creates a Runner.  Binary paths default to the PATH-resolved name when
// empty.
func New(store db.Store, cfg Config, logger *slog.Logger) *Runner {
	ffmpeg := cfg.FFmpegBin
	if ffmpeg == "" {
		ffmpeg = "ffmpeg"
	}
	ffprobe := cfg.FFprobeBin
	if ffprobe == "" {
		ffprobe = "ffprobe"
	}
	r := &Runner{
		store:      store,
		ffmpegBin:  ffmpeg,
		ffprobeBin: ffprobe,
		doviTool:   cfg.DoviToolBin,
		logger:     logger,
	}
	if cfg.Concurrency > 0 {
		r.sem = make(chan struct{}, cfg.Concurrency)
	}
	return r
}

// acquire blocks until a concurrency slot is available.
func (r *Runner) acquire() {
	if r.sem != nil {
		r.sem <- struct{}{}
	}
}

// release frees a concurrency slot.
func (r *Runner) release() {
	if r.sem != nil {
		<-r.sem
	}
}

// ---------------------------------------------------------------------------
// Path translation
// ---------------------------------------------------------------------------

// TranslatePath converts a source path (UNC or POSIX) to the controller's
// local filesystem path using the path_mappings table.  Returns the original
// path unchanged when no mapping matches.
func (r *Runner) TranslatePath(ctx context.Context, sourcePath string) (string, error) {
	mappings, err := r.store.ListPathMappings(ctx)
	if err != nil {
		return "", fmt.Errorf("analysis: list path mappings: %w", err)
	}
	return ApplyMappings(sourcePath, mappings), nil
}

// ApplyMappings applies a slice of path mappings to sourcePath and returns the
// translated path.  Only the first matching enabled mapping is applied.
// UNC prefix matching is case-insensitive; POSIX prefix matching is exact.
func ApplyMappings(sourcePath string, mappings []*db.PathMapping) string {
	isUNC := strings.HasPrefix(sourcePath, `\\`)
	for _, m := range mappings {
		if !m.Enabled {
			continue
		}
		if isUNC {
			if !strings.HasPrefix(strings.ToLower(sourcePath), strings.ToLower(m.WindowsPrefix)) {
				continue
			}
			remainder := sourcePath[len(m.WindowsPrefix):]
			remainder = strings.ReplaceAll(remainder, `\`, "/")
			linuxBase := strings.TrimRight(m.LinuxPrefix, "/")
			return linuxBase + "/" + strings.TrimLeft(remainder, "/")
		}
		// POSIX paths: apply linux_prefix → linux_prefix identity (no-op).
		// This branch lets operators register Linux NFS paths directly.
		if strings.HasPrefix(sourcePath, m.LinuxPrefix) {
			return sourcePath
		}
	}
	return sourcePath
}

// ---------------------------------------------------------------------------
// HDR detection
// ---------------------------------------------------------------------------

// RunHDRDetect runs ffprobe on the source file, determines the HDR type and
// Dolby Vision profile, and writes the result back to the source record.
func (r *Runner) RunHDRDetect(ctx context.Context, job *db.Job, source *db.Source) error {
	r.acquire()
	defer r.release()

	localPath, err := r.TranslatePath(ctx, source.UNCPath)
	if err != nil {
		return err
	}

	r.logger.Info("controller hdr_detect started",
		"job_id", job.ID,
		"source_id", source.ID,
		"local_path", localPath,
	)

	cmd := exec.CommandContext(ctx, r.ffprobeBin,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=color_transfer,codec_tag_string",
		"-show_entries", "stream_side_data=side_data_type",
		"-of", "json",
		localPath,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("analysis: hdr_detect ffprobe: %w (stderr: %s)", err, stderr.String())
	}

	hdrType, dvProfile, err := parseHDRFromJSON(stdout.Bytes())
	if err != nil {
		return fmt.Errorf("analysis: hdr_detect parse: %w", err)
	}

	// Attempt dovi_tool for the exact DV profile number.
	if hdrType == "dolby_vision" && r.doviTool != "" {
		if p := detectDVProfile(ctx, r.doviTool, localPath, r.logger); p > 0 {
			dvProfile = p
		}
	}

	if err := r.store.UpdateSourceHDR(ctx, db.UpdateSourceHDRParams{
		ID:        source.ID,
		HDRType:   hdrType,
		DVProfile: dvProfile,
	}); err != nil {
		return fmt.Errorf("analysis: hdr_detect update source: %w", err)
	}

	r.logger.Info("controller hdr_detect completed",
		"job_id", job.ID,
		"source_id", source.ID,
		"hdr_type", hdrType,
		"dv_profile", dvProfile,
	)
	return nil
}

// ffprobeStreamOutput is the JSON shape returned by ffprobe with -of json.
type ffprobeStreamOutput struct {
	Streams []struct {
		ColorTransfer  string `json:"color_transfer"`
		CodecTagString string `json:"codec_tag_string"`
		SideDataList   []struct {
			SideDataType string `json:"side_data_type"`
		} `json:"side_data_list"`
	} `json:"streams"`
}

func parseHDRFromJSON(data []byte) (hdrType string, dvProfile int, err error) {
	var out ffprobeStreamOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return "", 0, fmt.Errorf("parse ffprobe output: %w", err)
	}
	if len(out.Streams) == 0 {
		return "", 0, nil
	}
	s := out.Streams[0]

	switch strings.ToLower(s.ColorTransfer) {
	case "smpte2084":
		hdrType = "hdr10"
	case "arib-std-b67":
		hdrType = "hlg"
	}

	var hasHDR10P, hasDV bool
	for _, sd := range s.SideDataList {
		t := strings.ToLower(sd.SideDataType)
		if strings.Contains(t, "hdr dynamic metadata") || strings.Contains(t, "smpte2094") {
			hasHDR10P = true
		}
		if strings.Contains(t, "dovi") || strings.Contains(t, "dolby vision") {
			hasDV = true
		}
	}

	if hasHDR10P && hdrType == "hdr10" {
		hdrType = "hdr10+"
	}
	ct := strings.ToLower(s.CodecTagString)
	if hasDV || ct == "dvh1" || ct == "dvhe" {
		hdrType = "dolby_vision"
	}

	return hdrType, 0, nil
}

func detectDVProfile(ctx context.Context, doviTool, path string, logger *slog.Logger) int {
	cmd := exec.CommandContext(ctx, doviTool, "info", "-i", path)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		logger.Debug("dovi_tool failed", "error", err)
		return 0
	}

	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := scanner.Text()
		// dovi_tool outputs JSON; look for "profile": <N>
		if idx := strings.Index(line, `"profile"`); idx >= 0 {
			rest := line[idx+len(`"profile"`):]
			rest = strings.TrimLeft(rest, ": ")
			rest = strings.Trim(rest, `",}`)
			if p, err := strconv.Atoi(strings.TrimSpace(rest)); err == nil && p > 0 {
				return p
			}
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// Scene scanning
// ---------------------------------------------------------------------------

// SceneBoundary is a single detected scene change.
type SceneBoundary struct {
	PTSTime float64 `json:"pts_time"`
	Score   float64 `json:"score"`
}

// RunSceneScan detects scene boundaries using ffmpeg's scene filter and stores
// the result in analysis_results with type "scene".
func (r *Runner) RunSceneScan(ctx context.Context, job *db.Job, source *db.Source) error {
	r.acquire()
	defer r.release()

	localPath, err := r.TranslatePath(ctx, source.UNCPath)
	if err != nil {
		return err
	}

	r.logger.Info("controller scene scan started", "job_id", job.ID, "path", localPath)

	scenes, err := r.detectScenes(ctx, localPath)
	if err != nil {
		return fmt.Errorf("analysis: scene scan: %w", err)
	}

	frameData, err := json.Marshal(scenes)
	if err != nil {
		return fmt.Errorf("analysis: marshal scene data: %w", err)
	}
	summary, _ := json.Marshal(map[string]any{"scene_count": len(scenes)})

	if _, err := r.store.UpsertAnalysisResult(ctx, db.UpsertAnalysisResultParams{
		SourceID:  source.ID,
		Type:      "scene",
		FrameData: frameData,
		Summary:   summary,
	}); err != nil {
		return fmt.Errorf("analysis: store scene result: %w", err)
	}

	r.logger.Info("controller scene scan completed",
		"job_id", job.ID, "source_id", source.ID, "scenes", len(scenes))
	return nil
}

// detectScenes runs ffmpeg to detect scene boundaries.
// It uses metadata=print:file=- to write frame metadata to stdout.
func (r *Runner) detectScenes(ctx context.Context, path string) ([]SceneBoundary, error) {
	// Write metadata to a temp file to avoid mixing with other output.
	tmpFile, err := os.CreateTemp("", "de-scene-*.txt")
	if err != nil {
		return nil, fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	cmd := exec.CommandContext(ctx, r.ffmpegBin,
		"-i", path,
		"-vf", "select=gt(scene,0.4),metadata=print:file="+tmpPath,
		"-an",
		"-f", "null", "-",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	// ffmpeg exits non-zero with -f null even on success; ignore exit error if
	// the metadata file was written.
	_ = cmd.Run()

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w (ffmpeg stderr: %s)", err, stderr.String())
	}

	return parseSceneMetadata(data), nil
}

// parseSceneMetadata parses the lavf metadata output lines into SceneBoundary
// values.  Each detected frame block looks like:
//
//	frame:N   pts:N   pts_time:T
//	lavfi.scene_score=S
func parseSceneMetadata(data []byte) []SceneBoundary {
	var scenes []SceneBoundary
	var currentPTS float64

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "frame:") {
			// "frame:N pts:N pts_time:T"
			if _, err := fmt.Sscanf(line, "frame:%*d pts:%*d pts_time:%f", &currentPTS); err != nil {
				currentPTS = 0
			}
		} else if strings.HasPrefix(line, "lavfi.scene_score=") {
			scoreStr := strings.TrimPrefix(line, "lavfi.scene_score=")
			score, _ := strconv.ParseFloat(strings.TrimSpace(scoreStr), 64)
			scenes = append(scenes, SceneBoundary{PTSTime: currentPTS, Score: score})
		}
	}
	return scenes
}

// ---------------------------------------------------------------------------
// Full analysis (stream info + scene scan)
// ---------------------------------------------------------------------------

// RunAnalysis runs stream metadata extraction and scene detection for the
// source.  Results are stored in analysis_results.
func (r *Runner) RunAnalysis(ctx context.Context, job *db.Job, source *db.Source) error {
	r.acquire()
	defer r.release()

	localPath, err := r.TranslatePath(ctx, source.UNCPath)
	if err != nil {
		return err
	}

	r.logger.Info("controller analysis started",
		"job_id", job.ID, "source_id", source.ID, "path", localPath)

	// Stream metadata via ffprobe.
	streamInfo, err := r.probeStreamInfo(ctx, localPath)
	if err != nil {
		r.logger.Warn("stream info failed", "job_id", job.ID, "error", err)
	} else {
		if infoData, jerr := json.Marshal(streamInfo); jerr == nil {
			_, _ = r.store.UpsertAnalysisResult(ctx, db.UpsertAnalysisResultParams{
				SourceID:  source.ID,
				Type:      "stream_info",
				FrameData: infoData,
			})
		}
	}

	// Scene detection.
	scenes, err := r.detectScenes(ctx, localPath)
	if err != nil {
		r.logger.Warn("scene detection failed", "job_id", job.ID, "error", err)
	} else {
		sceneData, _ := json.Marshal(scenes)
		summary, _ := json.Marshal(map[string]any{"scene_count": len(scenes)})
		_, _ = r.store.UpsertAnalysisResult(ctx, db.UpsertAnalysisResultParams{
			SourceID:  source.ID,
			Type:      "scene",
			FrameData: sceneData,
			Summary:   summary,
		})
	}

	r.logger.Info("controller analysis completed",
		"job_id", job.ID, "source_id", source.ID,
		"scenes", len(scenes),
	)
	return nil
}

// probeStreamInfo runs ffprobe and returns raw JSON stream/format info.
func (r *Runner) probeStreamInfo(ctx context.Context, path string) (json.RawMessage, error) {
	cmd := exec.CommandContext(ctx, r.ffprobeBin,
		"-v", "error",
		"-show_streams",
		"-show_format",
		"-of", "json",
		path,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe: %w (stderr: %s)", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// ---------------------------------------------------------------------------
// Chunk concat
// ---------------------------------------------------------------------------

// RunConcat merges completed chunk files into a single output file using
// ffmpeg's concat demuxer.  Chunk paths are translated from UNC to Linux paths
// via the path_mappings table before use.
func (r *Runner) RunConcat(ctx context.Context, job *db.Job, chunkPaths []string, outputPath string) error {
	r.acquire()
	defer r.release()

	// Translate UNC paths to Linux paths.
	translatedPaths := make([]string, len(chunkPaths))
	for i, p := range chunkPaths {
		tp, err := r.TranslatePath(ctx, p)
		if err != nil {
			return fmt.Errorf("analysis: concat translate path: %w", err)
		}
		translatedPaths[i] = tp
	}
	translatedOutput, err := r.TranslatePath(ctx, outputPath)
	if err != nil {
		return fmt.Errorf("analysis: concat translate output: %w", err)
	}

	// Create output directory.
	if err := os.MkdirAll(filepath.Dir(translatedOutput), 0o755); err != nil {
		return fmt.Errorf("analysis: concat mkdir: %w", err)
	}

	// Write temp concat list file.
	listFile := filepath.Join(filepath.Dir(translatedOutput), "concat_list.txt")
	var buf bytes.Buffer
	for _, p := range translatedPaths {
		escaped := strings.ReplaceAll(p, "'", "\\'")
		buf.WriteString("file '" + escaped + "'\n")
	}
	if err := os.WriteFile(listFile, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("analysis: concat write list: %w", err)
	}
	defer os.Remove(listFile)

	// Run ffmpeg concat demuxer.
	cmd := exec.CommandContext(ctx, r.ffmpegBin, "-y", "-f", "concat", "-safe", "0", "-i", listFile, "-c", "copy", translatedOutput)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	r.logger.Info("running concat",
		slog.String("job_id", job.ID),
		slog.Int("chunks", len(chunkPaths)),
		slog.String("output", translatedOutput),
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("analysis: concat ffmpeg: %w: %s", err, stderr.String())
	}

	// Log output file size.
	if info, err := os.Stat(translatedOutput); err == nil {
		r.logger.Info("concat complete",
			slog.String("job_id", job.ID),
			slog.Int64("size_bytes", info.Size()),
		)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Audio encoding
// ---------------------------------------------------------------------------

// AudioFormat is the target codec for controller-side audio encoding.
type AudioFormat string

const (
	AudioFLAC AudioFormat = "flac"
	AudioOpus AudioFormat = "opus"
	AudioAAC  AudioFormat = "aac"
)

// RunAudio extracts and encodes the audio track(s) from the source using
// ffmpeg on the controller.  The output file is written next to the source
// file unless the job's EncodeConfig.OutputRoot specifies a different root.
// The output path uses the same path-mapping logic so it resolves to a
// locally writable NFS path.
func (r *Runner) RunAudio(ctx context.Context, job *db.Job, source *db.Source) error {
	r.acquire()
	defer r.release()

	localSrc, err := r.TranslatePath(ctx, source.UNCPath)
	if err != nil {
		return err
	}

	// Determine output path.
	outputRoot := job.EncodeConfig.OutputRoot
	if outputRoot == "" {
		outputRoot = filepath.Dir(localSrc)
	} else {
		// Translate output root from UNC to Linux if needed.
		outputRoot, err = r.TranslatePath(ctx, outputRoot)
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return fmt.Errorf("analysis: audio mkdir: %w", err)
	}

	// Determine format from ExtraVars or default to FLAC.
	format := AudioFLAC
	if fmtStr, ok := job.EncodeConfig.ExtraVars["AUDIO_FORMAT"]; ok {
		format = AudioFormat(strings.ToLower(fmtStr))
	}

	ext := string(format)
	base := strings.TrimSuffix(filepath.Base(localSrc), filepath.Ext(localSrc))
	outputPath := filepath.Join(outputRoot, base+"."+ext)

	r.logger.Info("controller audio encoding started",
		"job_id", job.ID, "source_id", source.ID,
		"format", format, "output", outputPath)

	args := []string{"-y", "-i", localSrc, "-vn"}
	switch format {
	case AudioFLAC:
		args = append(args, "-c:a", "flac")
	case AudioOpus:
		args = append(args, "-c:a", "libopus", "-b:a", "192k")
	case AudioAAC:
		args = append(args, "-c:a", "aac", "-b:a", "256k")
	default:
		args = append(args, "-c:a", "flac")
	}
	args = append(args, outputPath)

	cmd := exec.CommandContext(ctx, r.ffmpegBin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("analysis: audio ffmpeg: %w (stderr: %s)", err, stderr.String())
	}

	r.logger.Info("controller audio encoding completed",
		"job_id", job.ID, "source_id", source.ID, "output", outputPath)
	return nil
}
