package engine

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/badskater/distributed-encoder/internal/db"
)

// ---------------------------------------------------------------------------
// Stub store for ScriptGenerator tests
// ---------------------------------------------------------------------------

// scriptGenStub implements the subset of db.Store used by ScriptGenerator.
type scriptGenStub struct {
	variables []*db.Variable
	template  *db.Template
	templateErr error
}

func (s *scriptGenStub) ListVariables(_ context.Context, _ string) ([]*db.Variable, error) {
	return s.variables, nil
}
func (s *scriptGenStub) GetTemplateByID(_ context.Context, _ string) (*db.Template, error) {
	return s.template, s.templateErr
}

// Remaining Store methods are no-ops.
func (s *scriptGenStub) CreateUser(context.Context, db.CreateUserParams) (*db.User, error)          { return nil, nil }
func (s *scriptGenStub) GetUserByUsername(context.Context, string) (*db.User, error)                { return nil, nil }
func (s *scriptGenStub) GetUserByOIDCSub(context.Context, string) (*db.User, error)                 { return nil, nil }
func (s *scriptGenStub) GetUserByID(context.Context, string) (*db.User, error)                      { return nil, nil }
func (s *scriptGenStub) ListUsers(context.Context) ([]*db.User, error)                              { return nil, nil }
func (s *scriptGenStub) UpdateUserRole(context.Context, string, string) error                       { return nil }
func (s *scriptGenStub) DeleteUser(context.Context, string) error                                   { return nil }
func (s *scriptGenStub) CountAdminUsers(context.Context) (int64, error)                             { return 1, nil }
func (s *scriptGenStub) UpsertAgent(context.Context, db.UpsertAgentParams) (*db.Agent, error)       { return nil, nil }
func (s *scriptGenStub) GetAgentByID(context.Context, string) (*db.Agent, error)                   { return nil, nil }
func (s *scriptGenStub) GetAgentByName(context.Context, string) (*db.Agent, error)                 { return nil, nil }
func (s *scriptGenStub) ListAgents(context.Context) ([]*db.Agent, error)                           { return nil, nil }
func (s *scriptGenStub) UpdateAgentStatus(context.Context, string, string) error                   { return nil }
func (s *scriptGenStub) UpdateAgentHeartbeat(context.Context, db.UpdateAgentHeartbeatParams) error { return nil }
func (s *scriptGenStub) UpdateAgentVNCPort(context.Context, string, int) error                     { return nil }
func (s *scriptGenStub) SetAgentAPIKey(context.Context, string, string) error                      { return nil }
func (s *scriptGenStub) MarkStaleAgents(context.Context, time.Duration) (int64, error)             { return 0, nil }
func (s *scriptGenStub) CreateSource(context.Context, db.CreateSourceParams) (*db.Source, error)   { return nil, nil }
func (s *scriptGenStub) GetSourceByID(context.Context, string) (*db.Source, error)                 { return nil, nil }
func (s *scriptGenStub) GetSourceByUNCPath(context.Context, string) (*db.Source, error)            { return nil, nil }
func (s *scriptGenStub) ListSources(context.Context, db.ListSourcesFilter) ([]*db.Source, int64, error) { return nil, 0, nil }
func (s *scriptGenStub) UpdateSourceState(context.Context, string, string) error                   { return nil }
func (s *scriptGenStub) UpdateSourceVMAF(context.Context, string, float64) error                   { return nil }
func (s *scriptGenStub) UpdateSourceHDR(context.Context, db.UpdateSourceHDRParams) error           { return nil }
func (s *scriptGenStub) DeleteSource(context.Context, string) error                                { return nil }
func (s *scriptGenStub) CreateJob(context.Context, db.CreateJobParams) (*db.Job, error)            { return nil, nil }
func (s *scriptGenStub) GetJobByID(context.Context, string) (*db.Job, error)                      { return nil, nil }
func (s *scriptGenStub) ListJobs(context.Context, db.ListJobsFilter) ([]*db.Job, int64, error)    { return nil, 0, nil }
func (s *scriptGenStub) UpdateJobStatus(context.Context, string, string) error                    { return nil }
func (s *scriptGenStub) UpdateJobTaskCounts(context.Context, string) error                        { return nil }
func (s *scriptGenStub) GetJobsNeedingExpansion(context.Context) ([]*db.Job, error)               { return nil, nil }
func (s *scriptGenStub) CreateTask(context.Context, db.CreateTaskParams) (*db.Task, error)        { return nil, nil }
func (s *scriptGenStub) GetTaskByID(context.Context, string) (*db.Task, error)                   { return nil, nil }
func (s *scriptGenStub) ListTasksByJob(context.Context, string) ([]*db.Task, error)               { return nil, nil }
func (s *scriptGenStub) ClaimNextTask(context.Context, string, []string) (*db.Task, error)        { return nil, nil }
func (s *scriptGenStub) UpdateTaskStatus(context.Context, string, string) error                   { return nil }
func (s *scriptGenStub) SetTaskScriptDir(context.Context, string, string) error                   { return nil }
func (s *scriptGenStub) CompleteTask(context.Context, db.CompleteTaskParams) error                { return nil }
func (s *scriptGenStub) FailTask(context.Context, string, int, string) error                      { return nil }
func (s *scriptGenStub) CancelPendingTasksForJob(context.Context, string) error                   { return nil }
func (s *scriptGenStub) InsertTaskLog(context.Context, db.InsertTaskLogParams) error              { return nil }
func (s *scriptGenStub) ListTaskLogs(context.Context, db.ListTaskLogsParams) ([]*db.TaskLog, error) { return nil, nil }
func (s *scriptGenStub) TailTaskLogs(context.Context, string, int64) ([]*db.TaskLog, error)      { return nil, nil }
func (s *scriptGenStub) CreateTemplate(context.Context, db.CreateTemplateParams) (*db.Template, error) { return nil, nil }
func (s *scriptGenStub) ListTemplates(context.Context, string) ([]*db.Template, error)            { return nil, nil }
func (s *scriptGenStub) UpdateTemplate(context.Context, db.UpdateTemplateParams) error            { return nil }
func (s *scriptGenStub) DeleteTemplate(context.Context, string) error                             { return nil }
func (s *scriptGenStub) UpsertVariable(context.Context, db.UpsertVariableParams) (*db.Variable, error) { return nil, nil }
func (s *scriptGenStub) GetVariableByName(context.Context, string) (*db.Variable, error)         { return nil, nil }
func (s *scriptGenStub) DeleteVariable(context.Context, string) error                             { return nil }
func (s *scriptGenStub) CreateWebhook(context.Context, db.CreateWebhookParams) (*db.Webhook, error) { return nil, nil }
func (s *scriptGenStub) GetWebhookByID(context.Context, string) (*db.Webhook, error)             { return nil, nil }
func (s *scriptGenStub) ListWebhooksByEvent(context.Context, string) ([]*db.Webhook, error)      { return nil, nil }
func (s *scriptGenStub) ListWebhooks(context.Context) ([]*db.Webhook, error)                     { return nil, nil }
func (s *scriptGenStub) UpdateWebhook(context.Context, db.UpdateWebhookParams) error             { return nil }
func (s *scriptGenStub) DeleteWebhook(context.Context, string) error                              { return nil }
func (s *scriptGenStub) InsertWebhookDelivery(context.Context, db.InsertWebhookDeliveryParams) error { return nil }
func (s *scriptGenStub) ListWebhookDeliveries(context.Context, string, int, int) ([]*db.WebhookDelivery, error) { return nil, nil }
func (s *scriptGenStub) UpsertAnalysisResult(context.Context, db.UpsertAnalysisResultParams) (*db.AnalysisResult, error) { return nil, nil }
func (s *scriptGenStub) GetAnalysisResult(context.Context, string, string) (*db.AnalysisResult, error) { return nil, nil }
func (s *scriptGenStub) ListAnalysisResults(context.Context, string) ([]*db.AnalysisResult, error) { return nil, nil }
func (s *scriptGenStub) CreateSession(context.Context, db.CreateSessionParams) (*db.Session, error) { return nil, nil }
func (s *scriptGenStub) GetSessionByToken(context.Context, string) (*db.Session, error)          { return nil, nil }
func (s *scriptGenStub) DeleteSession(context.Context, string) error                              { return nil }
func (s *scriptGenStub) PruneExpiredSessions(context.Context) error                               { return nil }
func (s *scriptGenStub) CreateEnrollmentToken(context.Context, db.CreateEnrollmentTokenParams) (*db.EnrollmentToken, error) { return nil, nil }
func (s *scriptGenStub) GetEnrollmentToken(context.Context, string) (*db.EnrollmentToken, error) { return nil, nil }
func (s *scriptGenStub) ConsumeEnrollmentToken(context.Context, db.ConsumeEnrollmentTokenParams) error { return nil }
func (s *scriptGenStub) ListEnrollmentTokens(context.Context) ([]*db.EnrollmentToken, error)     { return nil, nil }
func (s *scriptGenStub) DeleteEnrollmentToken(context.Context, string) error                     { return nil }
func (s *scriptGenStub) PruneExpiredEnrollmentTokens(context.Context) error                      { return nil }
func (s *scriptGenStub) RetryFailedTasksForJob(context.Context, string) error                    { return nil }
func (s *scriptGenStub) ListJobLogs(context.Context, db.ListJobLogsParams) ([]*db.TaskLog, error) { return nil, nil }
func (s *scriptGenStub) PruneOldTaskLogs(context.Context, time.Time) error                       { return nil }
func (s *scriptGenStub) Ping(context.Context) error                                               { return nil }
func (s *scriptGenStub) CreatePathMapping(context.Context, db.CreatePathMappingParams) (*db.PathMapping, error) {
	return nil, nil
}
func (s *scriptGenStub) GetPathMappingByID(context.Context, string) (*db.PathMapping, error) {
	return nil, nil
}
func (s *scriptGenStub) ListPathMappings(context.Context) ([]*db.PathMapping, error) { return nil, nil }
func (s *scriptGenStub) UpdatePathMapping(context.Context, db.UpdatePathMappingParams) (*db.PathMapping, error) {
	return nil, nil
}
func (s *scriptGenStub) DeletePathMapping(context.Context, string) error                         { return nil }
func (s *scriptGenStub) DeleteTasksByJobID(_ context.Context, _ string) error                    { return nil }
func (s *scriptGenStub) CreateAuditEntry(_ context.Context, _ db.CreateAuditEntryParams) error   { return nil }
func (s *scriptGenStub) InsertAgentMetric(_ context.Context, _ db.InsertAgentMetricParams) error { return nil }
func (s *scriptGenStub) ListAgentMetrics(_ context.Context, _ string, _ time.Time) ([]*db.AgentMetric, error) { return nil, nil }
func (s *scriptGenStub) ListAuditLog(_ context.Context, _, _ int) ([]*db.AuditEntry, int, error) { return nil, 0, nil }

// ---------------------------------------------------------------------------
// RenderSingle tests
// ---------------------------------------------------------------------------

func TestRenderSingle_NoTemplate(t *testing.T) {
	// When no run script template is configured, RenderSingle should still
	// succeed and return an empty directory path.
	stub := &scriptGenStub{}
	gen := newScriptGenerator(stub, t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))

	job := &db.Job{ID: "job-1", JobType: "analysis", EncodeConfig: db.EncodeConfig{}}
	task := &db.Task{
		ID:         "task-1",
		SourcePath: `\\nas\src.mkv`,
		OutputPath: `\\nas\out.mkv`,
	}
	src := &db.Source{ID: "src-1"}

	dir, err := gen.RenderSingle(context.Background(), job, task, src)
	if err != nil {
		t.Fatalf("RenderSingle() error = %v", err)
	}
	if dir == "" {
		t.Error("expected a non-empty directory path")
	}
	if _, statErr := os.Stat(dir); statErr != nil {
		t.Errorf("expected output dir to exist: %v", statErr)
	}
}

func TestRenderSingle_WithTemplate(t *testing.T) {
	// RenderSingle should render the run script and populate standard variables.
	const tplContent = `SOURCE={{.SOURCE_PATH}}
OUTPUT={{.OUTPUT_PATH}}
JOB={{.JOB_ID}}
TYPE={{.JOB_TYPE}}`

	stub := &scriptGenStub{
		template: &db.Template{
			ID:        "tpl-1",
			Name:      "audio_run",
			Type:      "bat",
			Extension: "bat",
			Content:   tplContent,
		},
	}
	gen := newScriptGenerator(stub, t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))

	job := &db.Job{
		ID:      "job-audio",
		JobType: "audio",
		EncodeConfig: db.EncodeConfig{
			RunScriptTemplateID: "tpl-1",
		},
	}
	task := &db.Task{
		ID:         "task-audio",
		SourcePath: `\\nas\movie.mkv`,
		OutputPath: `\\nas\movie.flac`,
	}
	src := &db.Source{ID: "src-audio"}

	dir, err := gen.RenderSingle(context.Background(), job, task, src)
	if err != nil {
		t.Fatalf("RenderSingle() error = %v", err)
	}

	scriptPath := filepath.Join(dir, "run.bat")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("reading rendered script: %v", err)
	}

	got := string(content)
	checks := map[string]string{
		"SOURCE_PATH": `\\nas\movie.mkv`,
		"OUTPUT_PATH": `\\nas\movie.flac`,
		"JOB_ID":      "job-audio",
		"JOB_TYPE":    "audio",
	}
	for key, want := range checks {
		if !strings.Contains(got, want) {
			t.Errorf("rendered script missing %s=%q\nscript:\n%s", key, want, got)
		}
	}
}

func TestRenderSingle_GlobalVariables(t *testing.T) {
	// Global variables from the DB must be injected into the template.
	stub := &scriptGenStub{
		variables: []*db.Variable{
			{Name: "ENCODER_BIN", Value: `C:\tools\ffmpeg.exe`},
		},
		template: &db.Template{
			ID: "tpl-2", Name: "t", Type: "bat", Extension: "bat",
			Content: `{{.ENCODER_BIN}}`,
		},
	}
	gen := newScriptGenerator(stub, t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))

	job := &db.Job{
		ID: "job-2", JobType: "audio",
		EncodeConfig: db.EncodeConfig{RunScriptTemplateID: "tpl-2"},
	}
	task := &db.Task{ID: "t2", SourcePath: `\\s\a`, OutputPath: `\\s\b`}
	src := &db.Source{ID: "src-2"}

	dir, err := gen.RenderSingle(context.Background(), job, task, src)
	if err != nil {
		t.Fatalf("RenderSingle() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "run.bat"))
	if err != nil {
		t.Fatalf("reading script: %v", err)
	}
	if want := `C:\tools\ffmpeg.exe`; !strings.Contains(string(content), want) {
		t.Errorf("expected global variable %q in script, got:\n%s", want, content)
	}
}

func TestRenderSingle_ExtraVarsOverrideGlobals(t *testing.T) {
	// ExtraVars on the job should override global variables of the same name.
	stub := &scriptGenStub{
		variables: []*db.Variable{
			{Name: "QUALITY", Value: "high"},
		},
		template: &db.Template{
			ID: "tpl-3", Name: "t", Type: "bat", Extension: "bat",
			Content: `{{.QUALITY}}`,
		},
	}
	gen := newScriptGenerator(stub, t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))

	job := &db.Job{
		ID: "job-3", JobType: "audio",
		EncodeConfig: db.EncodeConfig{
			RunScriptTemplateID: "tpl-3",
			ExtraVars:           map[string]string{"QUALITY": "ultra"},
		},
	}
	task := &db.Task{ID: "t3", SourcePath: `\\s\a`, OutputPath: `\\s\b`}
	src := &db.Source{ID: "src-3"}

	dir, err := gen.RenderSingle(context.Background(), job, task, src)
	if err != nil {
		t.Fatalf("RenderSingle() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "run.bat"))
	if err != nil {
		t.Fatalf("reading script: %v", err)
	}
	if want := "ultra"; !strings.Contains(string(content), want) {
		t.Errorf("expected ExtraVar override %q in script, got:\n%s", want, content)
	}
}

func TestRenderSingle_DirCleanedUpOnTemplateError(t *testing.T) {
	// If template rendering fails, the directory must not be left behind.
	stub := &scriptGenStub{
		template: &db.Template{
			ID: "bad", Name: "bad", Type: "bat", Extension: "bat",
			Content: `{{ .MISSING_FUNC call }}`, // bad template
		},
	}
	baseDir := t.TempDir()
	gen := newScriptGenerator(stub, baseDir, slog.New(slog.NewTextHandler(io.Discard, nil)))

	job := &db.Job{
		ID: "job-err", JobType: "audio",
		EncodeConfig: db.EncodeConfig{RunScriptTemplateID: "bad"},
	}
	task := &db.Task{ID: "t-err", SourcePath: `\\s\a`, OutputPath: `\\s\b`}
	src := &db.Source{ID: "src-err"}

	_, err := gen.RenderSingle(context.Background(), job, task, src)
	if err == nil {
		t.Fatal("expected error for bad template, got nil")
	}

	// The script directory should have been cleaned up.
	expectedDir := filepath.Join(baseDir, "job-err", "0000")
	if _, statErr := os.Stat(expectedDir); !os.IsNotExist(statErr) {
		t.Errorf("expected script dir %q to be removed after error, but it still exists", expectedDir)
	}
}

func TestEscapeBat(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "ampersand", input: "a&b", want: "a^&b"},
		{name: "pipe", input: "a|b", want: "a^|b"},
		{name: "less than", input: "a<b", want: "a^<b"},
		{name: "greater than", input: "a>b", want: "a^>b"},
		{name: "caret", input: "a^b", want: "a^^b"},
		{name: "no special chars", input: "hello world", want: "hello world"},
		{name: "multiple special chars", input: "echo foo & bar | baz > out < in ^ end", want: "echo foo ^& bar ^| baz ^> out ^< in ^^ end"},
		{name: "empty string", input: "", want: ""},
		{name: "all specials adjacent", input: "&|<>^", want: "^&^|^<^>^^"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeBat(tt.input)
			if got != tt.want {
				t.Errorf("escapeBat(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTrimAvs(t *testing.T) {
	fn := templateFuncs["trimAvs"].(func(int, int) string)

	tests := []struct {
		name       string
		start, end int
		want       string
	}{
		{name: "normal range", start: 0, end: 1000, want: "Trim(0, 1000)"},
		{name: "same frame", start: 500, end: 500, want: "Trim(500, 500)"},
		{name: "large values", start: 100000, end: 200000, want: "Trim(100000, 200000)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fn(tt.start, tt.end)
			if got != tt.want {
				t.Errorf("trimAvs(%d, %d) = %q, want %q", tt.start, tt.end, got, tt.want)
			}
		})
	}
}

func TestTrimVpy(t *testing.T) {
	fn := templateFuncs["trimVpy"].(func(int, int) string)

	tests := []struct {
		name       string
		start, end int
		want       string
	}{
		{name: "normal range", start: 0, end: 1000, want: "[0:1000]"},
		{name: "same frame", start: 500, end: 500, want: "[500:500]"},
		{name: "large values", start: 100000, end: 200000, want: "[100000:200000]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fn(tt.start, tt.end)
			if got != tt.want {
				t.Errorf("trimVpy(%d, %d) = %q, want %q", tt.start, tt.end, got, tt.want)
			}
		})
	}
}

func TestGpuFlag(t *testing.T) {
	fn := templateFuncs["gpuFlag"].(func(string) string)

	tests := []struct {
		name   string
		vendor string
		want   string
	}{
		{name: "nvidia", vendor: "nvidia", want: "--hwaccel nvenc --hwaccel_output_format cuda"},
		{name: "nvidia uppercase", vendor: "NVIDIA", want: "--hwaccel nvenc --hwaccel_output_format cuda"},
		{name: "amd", vendor: "amd", want: "--hwaccel amf"},
		{name: "amd mixed case", vendor: "Amd", want: "--hwaccel amf"},
		{name: "intel", vendor: "intel", want: "--hwaccel qsv"},
		{name: "intel uppercase", vendor: "INTEL", want: "--hwaccel qsv"},
		{name: "unknown vendor", vendor: "qualcomm", want: ""},
		{name: "empty string", vendor: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fn(tt.vendor)
			if got != tt.want {
				t.Errorf("gpuFlag(%q) = %q, want %q", tt.vendor, got, tt.want)
			}
		})
	}
}

func TestDefaultFunc(t *testing.T) {
	fn := templateFuncs["default"].(func(string, string) string)

	tests := []struct {
		name string
		dflt string
		val  string
		want string
	}{
		{name: "empty val returns default", dflt: "fallback", val: "", want: "fallback"},
		{name: "non-empty val returns val", dflt: "fallback", val: "actual", want: "actual"},
		{name: "both empty", dflt: "", val: "", want: ""},
		{name: "default empty but val set", dflt: "", val: "actual", want: "actual"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fn(tt.dflt, tt.val)
			if got != tt.want {
				t.Errorf("default(%q, %q) = %q, want %q", tt.dflt, tt.val, got, tt.want)
			}
		})
	}
}

func TestRenderToFile(t *testing.T) {
	t.Run("simple variable substitution", func(t *testing.T) {
		dir := t.TempDir()
		outPath := filepath.Join(dir, "output.bat")

		data := map[string]string{
			"SOURCE_PATH": `\\NAS01\test.mkv`,
		}

		err := renderToFile("test", "{{.SOURCE_PATH}}", data, outPath)
		if err != nil {
			t.Fatalf("renderToFile() error = %v", err)
		}

		got, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("reading output file: %v", err)
		}

		want := `\\NAS01\test.mkv`
		if string(got) != want {
			t.Errorf("file contents = %q, want %q", string(got), want)
		}
	})

	t.Run("escapeBat template function", func(t *testing.T) {
		dir := t.TempDir()
		outPath := filepath.Join(dir, "output.bat")

		data := map[string]string{
			"V": "a&b",
		}

		err := renderToFile("test", "{{ escapeBat .V }}", data, outPath)
		if err != nil {
			t.Fatalf("renderToFile() error = %v", err)
		}

		got, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("reading output file: %v", err)
		}

		want := "a^&b"
		if string(got) != want {
			t.Errorf("file contents = %q, want %q", string(got), want)
		}
	})

	t.Run("bad template syntax returns error", func(t *testing.T) {
		dir := t.TempDir()
		outPath := filepath.Join(dir, "output.bat")

		data := map[string]string{}

		err := renderToFile("bad", "{{ .Foo", data, outPath)
		if err == nil {
			t.Fatal("renderToFile() expected error for bad template syntax, got nil")
		}
		if !strings.Contains(err.Error(), "parse template") {
			t.Errorf("error = %q, want it to contain %q", err.Error(), "parse template")
		}
	})

	t.Run("multiple template functions combined", func(t *testing.T) {
		dir := t.TempDir()
		outPath := filepath.Join(dir, "output.bat")

		data := map[string]string{
			"CMD": "echo hello & goodbye",
		}

		err := renderToFile("test", `{{ escapeBat .CMD }}`, data, outPath)
		if err != nil {
			t.Fatalf("renderToFile() error = %v", err)
		}

		got, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("reading output file: %v", err)
		}

		want := "echo hello ^& goodbye"
		if string(got) != want {
			t.Errorf("file contents = %q, want %q", string(got), want)
		}
	})
}

func TestDvFlag(t *testing.T) {
	fn := templateFuncs["dvFlag"].(func(string) string)

	tests := []struct {
		name    string
		profile string
		want    string
	}{
		{name: "profile 5", profile: "5", want: "--dolby-vision-profile 5"},
		{name: "profile 7", profile: "7", want: "--dolby-vision-profile 7"},
		{name: "profile 8", profile: "8", want: "--dolby-vision-profile 8.1"},
		{name: "profile 9", profile: "9", want: "--dolby-vision-profile 9"},
		{name: "no DV (0)", profile: "0", want: ""},
		{name: "empty string", profile: "", want: ""},
		{name: "unknown profile", profile: "3", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fn(tt.profile)
			if got != tt.want {
				t.Errorf("dvFlag(%q) = %q, want %q", tt.profile, got, tt.want)
			}
		})
	}
}

func TestHdrFlag(t *testing.T) {
	fn := templateFuncs["hdrFlag"].(func(string) string)

	tests := []struct {
		name    string
		hdrType string
		want    string
	}{
		{name: "hdr10", hdrType: "hdr10", want: "--hdr10 --hdr10-opt"},
		{name: "hdr10 uppercase", hdrType: "HDR10", want: "--hdr10 --hdr10-opt"},
		{name: "hdr10+", hdrType: "hdr10+", want: "--hdr10 --hdr10-opt --dhdr10-opt"},
		{name: "dolby_vision", hdrType: "dolby_vision", want: "--hdr10 --hdr10-opt"},
		{name: "hlg", hdrType: "hlg", want: "--transfer-characteristics arib-std-b67 --colorprim bt2020 --colormatrix bt2020nc"},
		{name: "SDR empty", hdrType: "", want: ""},
		{name: "unknown type", hdrType: "sdr", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fn(tt.hdrType)
			if got != tt.want {
				t.Errorf("hdrFlag(%q) = %q, want %q", tt.hdrType, got, tt.want)
			}
		})
	}
}

func TestDvBitstreamFilter(t *testing.T) {
	fn := templateFuncs["dvBitstreamFilter"].(func(string) string)

	tests := []struct {
		name    string
		profile string
		want    string
	}{
		{name: "profile 5 needs BSF", profile: "5", want: "hevc_mp4toannexb"},
		{name: "profile 8 needs BSF", profile: "8", want: "hevc_mp4toannexb"},
		{name: "profile 9 needs BSF", profile: "9", want: "hevc_mp4toannexb"},
		{name: "profile 7 no BSF", profile: "7", want: ""},
		{name: "no DV", profile: "0", want: ""},
		{name: "empty", profile: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fn(tt.profile)
			if got != tt.want {
				t.Errorf("dvBitstreamFilter(%q) = %q, want %q", tt.profile, got, tt.want)
			}
		})
	}
}

func TestRenderSingle_HDRVariablesInjected(t *testing.T) {
	// HDR_TYPE and DV_PROFILE from the source must be available in templates.
	const tplContent = `HDR={{.HDR_TYPE}} DV={{.DV_PROFILE}}`

	stub := &scriptGenStub{
		template: &db.Template{
			ID: "tpl-hdr", Name: "hdr_run", Type: "sh", Extension: "sh",
			Content: tplContent,
		},
	}
	gen := newScriptGenerator(stub, t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))

	job := &db.Job{
		ID: "job-hdr", JobType: "encode",
		EncodeConfig: db.EncodeConfig{RunScriptTemplateID: "tpl-hdr"},
	}
	task := &db.Task{ID: "t-hdr", SourcePath: `\\s\movie.mkv`, OutputPath: `\\s\out.mkv`}
	src := &db.Source{ID: "src-hdr", HDRType: "dolby_vision", DVProfile: 8}

	dir, err := gen.RenderSingle(context.Background(), job, task, src)
	if err != nil {
		t.Fatalf("RenderSingle() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "run.sh"))
	if err != nil {
		t.Fatalf("reading script: %v", err)
	}

	got := string(content)
	if !strings.Contains(got, "dolby_vision") {
		t.Errorf("expected HDR_TYPE=dolby_vision in script, got:\n%s", got)
	}
	if !strings.Contains(got, "8") {
		t.Errorf("expected DV_PROFILE=8 in script, got:\n%s", got)
	}
}
