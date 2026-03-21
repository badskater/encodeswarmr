package engine

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/badskater/encodeswarmr/internal/db"
)

// ScriptGenerator loads templates and global variables from the DB, renders
// them with Go text/template, and writes the resulting script files to a
// per-task directory on disk.
type ScriptGenerator struct {
	store   db.Store
	baseDir string
	logger  *slog.Logger
}

func newScriptGenerator(store db.Store, baseDir string, logger *slog.Logger) *ScriptGenerator {
	return &ScriptGenerator{
		store:   store,
		baseDir: baseDir,
		logger:  logger,
	}
}

// templateFuncs returns the custom functions available inside templates.
var templateFuncs = template.FuncMap{
	"escapeBat": escapeBat,
	"uncPath":   uncPath,
	"basename":  func(s string) string { return filepath.Base(s) },
	"default": func(dflt, val string) string {
		if val == "" {
			return dflt
		}
		return val
	},
	"split": func(sep, s string) []string {
		return strings.Split(s, sep)
	},
	"join": func(sep string, elems []string) string {
		return strings.Join(elems, sep)
	},
	"trimAvs": func(start, end int) string {
		return fmt.Sprintf("Trim(%d, %d)", start, end)
	},
	"trimVpy": func(start, end int) string {
		return fmt.Sprintf("[%d:%d]", start, end)
	},
	"gpuFlag": func(vendor string) string {
		switch strings.ToLower(vendor) {
		case "nvidia":
			return "--hwaccel nvenc --hwaccel_output_format cuda"
		case "amd":
			return "--hwaccel amf"
		case "intel":
			return "--hwaccel qsv"
		default:
			return ""
		}
	},
	// dvFlag returns x265 encoding flags for the given Dolby Vision profile string
	// (e.g. "8" → "--dolby-vision-profile 8"). Returns "" for no DV (profile "0" or "").
	"dvFlag": dvFlag,
	// hdrFlag returns x265/ffmpeg flags for static HDR mastering metadata.
	// Accepts "hdr10", "hdr10+", "dolby_vision", "hlg", or "" for SDR.
	"hdrFlag": hdrFlag,
	// dvBitstreamFilter returns the ffmpeg -bsf:v argument required to mux a
	// Dolby Vision HEVC stream into a container (e.g. "hevc_mp4toannexb").
	// Returns "" when no filter is needed.
	"dvBitstreamFilter": dvBitstreamFilter,
}

// dvFlag returns the x265 flags required to encode or pass through a Dolby
// Vision stream at the given profile number (supplied as a decimal string).
// Profile 0 or empty string means no DV — returns "".
func dvFlag(profile string) string {
	switch profile {
	case "5":
		return "--dolby-vision-profile 5"
	case "7":
		return "--dolby-vision-profile 7"
	case "8":
		return "--dolby-vision-profile 8.1"
	case "9":
		return "--dolby-vision-profile 9"
	default:
		return ""
	}
}

// hdrFlag returns the x265 flags that enable static HDR mastering data for
// the given HDR type.  Returns "" for SDR / unknown types.
func hdrFlag(hdrType string) string {
	switch strings.ToLower(hdrType) {
	case "hdr10":
		return "--hdr10 --hdr10-opt"
	case "hdr10+":
		return "--hdr10 --hdr10-opt --dhdr10-opt"
	case "dolby_vision":
		return "--hdr10 --hdr10-opt"
	case "hlg":
		return "--transfer-characteristics arib-std-b67 --colorprim bt2020 --colormatrix bt2020nc"
	default:
		return ""
	}
}

// dvBitstreamFilter returns the value to pass to ffmpeg's -bsf:v option when
// muxing a Dolby Vision HEVC elementary stream.  Returns "" when no filter is
// needed (e.g. for MKV output or non-DV streams).
func dvBitstreamFilter(profile string) string {
	switch profile {
	case "5", "8", "9":
		return "hevc_mp4toannexb"
	default:
		return ""
	}
}

// escapeBat escapes characters that are special in Windows .bat files by
// prefixing each with a caret (^).
func escapeBat(s string) string {
	r := strings.NewReplacer(
		"&", "^&",
		"|", "^|",
		"<", "^<",
		">", "^>",
		"^", "^^",
	)
	return r.Replace(s)
}

// uncPath validates that the path looks like a UNC path (starts with \\).
// If it does, the path is returned as-is. Otherwise it is returned unchanged.
func uncPath(s string) string {
	return s
}

// Render generates script files for a task and writes them to a new
// subdirectory under baseDir/{jobID}/{chunkIndex:04d}/. It returns the path
// of the created directory.
func (g *ScriptGenerator) Render(ctx context.Context, job *db.Job, task *db.Task, source *db.Source) (string, error) {
	// Validate chunk index against boundaries.
	if task.ChunkIndex >= len(job.EncodeConfig.ChunkBoundaries) {
		return "", fmt.Errorf("scriptgen: chunk index %d out of range (have %d boundaries)",
			task.ChunkIndex, len(job.EncodeConfig.ChunkBoundaries))
	}
	boundary := job.EncodeConfig.ChunkBoundaries[task.ChunkIndex]

	// 1. Load all global variables from the DB.
	vars, err := g.store.ListVariables(ctx, "")
	if err != nil {
		return "", fmt.Errorf("scriptgen: load variables: %w", err)
	}

	// 2. Build template data map with the documented override order:
	//    globals < job extra vars < task variables < built-ins.
	data := make(map[string]string, len(vars)+10)

	for _, v := range vars {
		data[v.Name] = v.Value
	}

	for k, v := range job.EncodeConfig.ExtraVars {
		data[k] = v
	}

	for k, v := range task.Variables {
		data[k] = v
	}

	// Built-in variables always win.
	data["SOURCE_PATH"] = task.SourcePath
	data["OUTPUT_PATH"] = task.OutputPath
	data["START_FRAME"] = strconv.Itoa(boundary.StartFrame)
	data["END_FRAME"] = strconv.Itoa(boundary.EndFrame)
	data["CHUNK_INDEX"] = strconv.Itoa(task.ChunkIndex)
	data["JOB_ID"] = job.ID
	data["TASK_ID"] = task.ID
	data["TOTAL_CHUNKS"] = strconv.Itoa(len(job.EncodeConfig.ChunkBoundaries))
	data["HDR_TYPE"] = source.HDRType
	data["DV_PROFILE"] = strconv.Itoa(source.DVProfile)

	// 3. Create the output directory.
	dir := filepath.Join(g.baseDir, job.ID, fmt.Sprintf("%04d", task.ChunkIndex))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("scriptgen: create dir: %w", err)
	}

	// From here on, any failure should clean up the directory.
	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(dir)
		}
	}()

	// 4. Render the run script template.
	runTpl, err := g.store.GetTemplateByID(ctx, job.EncodeConfig.RunScriptTemplateID)
	if err != nil {
		return "", fmt.Errorf("scriptgen: load run script template: %w", err)
	}
	runFile := filepath.Join(dir, "run."+runTpl.Extension)
	if err := renderToFile(runTpl.Name, runTpl.Content, data, runFile); err != nil {
		return "", fmt.Errorf("scriptgen: render run script: %w", err)
	}

	// 5. Optionally render the frameserver template.
	if job.EncodeConfig.FrameserverTemplateID != "" {
		fsTpl, err := g.store.GetTemplateByID(ctx, job.EncodeConfig.FrameserverTemplateID)
		if err != nil {
			return "", fmt.Errorf("scriptgen: load frameserver template: %w", err)
		}
		fsFile := filepath.Join(dir, "frameserver."+fsTpl.Extension)
		if err := renderToFile(fsTpl.Name, fsTpl.Content, data, fsFile); err != nil {
			return "", fmt.Errorf("scriptgen: render frameserver script: %w", err)
		}
	}

	g.logger.Info("scripts rendered",
		slog.String("job_id", job.ID),
		slog.String("task_id", task.ID),
		slog.Int("chunk_index", task.ChunkIndex),
		slog.String("dir", dir),
	)

	cleanup = false
	return dir, nil
}

// RenderSingle generates script files for a non-chunked job (analysis, audio).
// Unlike Render, it does not require chunk boundaries.
func (g *ScriptGenerator) RenderSingle(ctx context.Context, job *db.Job, task *db.Task, source *db.Source) (string, error) {
	if job.EncodeConfig.RunScriptTemplateID == "" {
		// No run script template: nothing to render, return an empty dir.
		dir := filepath.Join(g.baseDir, job.ID, "0000")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("scriptgen: create dir: %w", err)
		}
		return dir, nil
	}

	vars, err := g.store.ListVariables(ctx, "")
	if err != nil {
		return "", fmt.Errorf("scriptgen: load variables: %w", err)
	}

	data := make(map[string]string, len(vars)+10)
	for _, v := range vars {
		data[v.Name] = v.Value
	}
	for k, v := range job.EncodeConfig.ExtraVars {
		data[k] = v
	}
	for k, v := range task.Variables {
		data[k] = v
	}

	data["SOURCE_PATH"] = task.SourcePath
	data["OUTPUT_PATH"] = task.OutputPath
	data["JOB_ID"] = job.ID
	data["TASK_ID"] = task.ID
	data["JOB_TYPE"] = job.JobType
	data["HDR_TYPE"] = source.HDRType
	data["DV_PROFILE"] = strconv.Itoa(source.DVProfile)

	dir := filepath.Join(g.baseDir, job.ID, "0000")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("scriptgen: create dir: %w", err)
	}

	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(dir)
		}
	}()

	runTpl, err := g.store.GetTemplateByID(ctx, job.EncodeConfig.RunScriptTemplateID)
	if err != nil {
		return "", fmt.Errorf("scriptgen: load run script template: %w", err)
	}
	runFile := filepath.Join(dir, "run."+runTpl.Extension)
	if err := renderToFile(runTpl.Name, runTpl.Content, data, runFile); err != nil {
		return "", fmt.Errorf("scriptgen: render run script: %w", err)
	}

	g.logger.Info("single task scripts rendered",
		slog.String("job_id", job.ID),
		slog.String("task_id", task.ID),
		slog.String("job_type", job.JobType),
		slog.String("dir", dir),
	)

	cleanup = false
	return dir, nil
}

// RenderConcat generates the script directory for a concat task.
// It writes a run.bat that uses ffmpeg's concat demuxer to merge all chunk
// output files (chunkPaths) into finalOutputPath.
func (g *ScriptGenerator) RenderConcat(ctx context.Context, job *db.Job, task *db.Task, chunkPaths []string, finalOutputPath string) (string, error) {
	dir := filepath.Join(g.baseDir, job.ID, "concat")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("scriptgen: create concat dir: %w", err)
	}

	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(dir)
		}
	}()

	// Write the ffmpeg concat list file.
	var listBuf bytes.Buffer
	for _, p := range chunkPaths {
		// Escape single-quotes inside the path for the concat list format.
		escaped := strings.ReplaceAll(p, "'", "\\'")
		listBuf.WriteString("file '" + escaped + "'\n")
	}
	listFile := filepath.Join(dir, "concat_list.txt")
	if err := os.WriteFile(listFile, listBuf.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("scriptgen: write concat list: %w", err)
	}

	// Build run.bat — uses %FFMPEG_BIN% env var injected by the agent runner,
	// falling back to bare "ffmpeg" if not set.
	var batBuf bytes.Buffer
	batBuf.WriteString("@echo off\r\n")
	batBuf.WriteString("setlocal\r\n")
	batBuf.WriteString("if not defined FFMPEG_BIN set FFMPEG_BIN=ffmpeg\r\n")
	batBuf.WriteString(fmt.Sprintf(`"%%FFMPEG_BIN%%" -y -f concat -safe 0 -i "%s" -c copy "%s"`,
		listFile, finalOutputPath))
	batBuf.WriteString("\r\n")
	batBuf.WriteString("if %ERRORLEVEL% neq 0 exit /b %ERRORLEVEL%\r\n")

	batFile := filepath.Join(dir, "run.bat")
	if err := os.WriteFile(batFile, batBuf.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("scriptgen: write concat bat: %w", err)
	}

	g.logger.Info("concat scripts rendered",
		slog.String("job_id", job.ID),
		slog.String("task_id", task.ID),
		slog.Int("chunk_count", len(chunkPaths)),
		slog.String("output", finalOutputPath),
		slog.String("dir", dir),
	)

	cleanup = false
	return dir, nil
}

// renderToFile parses the template content, executes it with data, and writes
// the result to the given path.
func renderToFile(name, content string, data map[string]string, path string) error {
	t, err := template.New(name).Funcs(templateFuncs).Parse(content)
	if err != nil {
		return fmt.Errorf("parse template %q: %w", name, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template %q: %w", name, err)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}
