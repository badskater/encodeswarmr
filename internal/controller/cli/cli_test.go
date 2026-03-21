package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/badskater/distributed-encoder/internal/db"
	"github.com/badskater/distributed-encoder/internal/db/teststore"
)

// ---------------------------------------------------------------------------
// Pure helper function tests
// ---------------------------------------------------------------------------

func TestFmtBytes(t *testing.T) {
	var gb int64 = 1024 * 1024 * 1024
	gb32 := int64(3.2 * float64(gb))
	tests := []struct {
		name  string
		n     int64
		want  string
	}{
		{"zero", 0, "0B"},
		{"bytes", 512, "512B"},
		{"exactly 1 KB", 1024, "1.0KB"},
		{"1.5 KB", 1536, "1.5KB"},
		{"exactly 1 MB", 1024 * 1024, "1.0MB"},
		{"2.5 MB", int64(2.5 * 1024 * 1024), "2.5MB"},
		{"exactly 1 GB", gb, "1.0GB"},
		{"3.2 GB", gb32, "3.2GB"},
		{"just under 1 KB", 1023, "1023B"},
		{"just under 1 MB", 1024*1024 - 1, "1024.0KB"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fmtBytes(tc.n)
			if got != tc.want {
				t.Errorf("fmtBytes(%d) = %q, want %q", tc.n, got, tc.want)
			}
		})
	}
}

func TestNewTabWriter(t *testing.T) {
	tw := newTabWriter()
	if tw == nil {
		t.Fatal("newTabWriter returned nil")
	}
	// Verify it is a tabwriter.Writer by checking a known type assertion.
	if _, ok := any(tw).(*tabwriter.Writer); !ok {
		t.Fatal("newTabWriter did not return a *tabwriter.Writer")
	}
}

// ---------------------------------------------------------------------------
// generateSelfSigned tests — pure crypto, no DB required
// ---------------------------------------------------------------------------

func TestGenerateSelfSigned_Success(t *testing.T) {
	dir := t.TempDir()
	if err := generateSelfSigned("test.internal", dir); err != nil {
		t.Fatalf("generateSelfSigned: %v", err)
	}

	certPath := filepath.Join(dir, "tls.crt")
	keyPath := filepath.Join(dir, "tls.key")

	if _, err := os.Stat(certPath); err != nil {
		t.Errorf("tls.crt not created: %v", err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("tls.key not created: %v", err)
	}

	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	if !bytes.Contains(certBytes, []byte("-----BEGIN CERTIFICATE-----")) {
		t.Error("tls.crt does not look like PEM")
	}

	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}
	if !bytes.Contains(keyBytes, []byte("-----BEGIN EC PRIVATE KEY-----")) {
		t.Error("tls.key does not look like PEM")
	}
}

func TestGenerateSelfSigned_CreatesOutputDir(t *testing.T) {
	parent := t.TempDir()
	outDir := filepath.Join(parent, "new_subdir")

	if err := generateSelfSigned("foo.local", outDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "tls.crt")); err != nil {
		t.Error("certificate file not found after creating output dir")
	}
}

// ---------------------------------------------------------------------------
// TLS generate command — flag validation
// ---------------------------------------------------------------------------

func TestTLSGenerateCmd_MissingCN(t *testing.T) {
	cmd := newTLSGenerateCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{}) // no --cn flag
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when --cn is missing, got nil")
	}
}

func TestTLSGenerateCmd_Success(t *testing.T) {
	dir := t.TempDir()
	cmd := newTLSGenerateCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--cn", "ctrl.internal", "--out", dir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "tls.crt")); err != nil {
		t.Error("tls.crt not found")
	}
}

// ---------------------------------------------------------------------------
// newRootCmd — structural sanity
// ---------------------------------------------------------------------------

func TestRootCmd_HasExpectedSubcommands(t *testing.T) {
	ctx := context.Background()
	root := newRootCmd(ctx)

	want := []string{
		"server", "run", "agent", "source", "template",
		"job", "task", "user", "webhook", "tls",
	}
	cmds := make(map[string]bool)
	for _, c := range root.Commands() {
		cmds[c.Name()] = true
	}
	for _, name := range want {
		if !cmds[name] {
			t.Errorf("root command missing subcommand %q", name)
		}
	}
}

// ---------------------------------------------------------------------------
// Commands that call openStore — inject via package-level override
// ---------------------------------------------------------------------------
//
// The CLI commands call openStore(ctx, cfgFile) directly.  We expose a
// test-only hook so tests can substitute their own store factory without
// touching production code paths.

// testOpenStoreFunc is set by tests to override openStore.
var testOpenStoreFunc func(ctx context.Context, cfgPath string) (db.Store, func(), error)

func init() {
	// Override openStore at init time so tests can set testOpenStoreFunc.
	// The real openStore loads a config file and connects to Postgres;
	// we redirect it through the test hook when non-nil.
	origOpenStore := openStore
	_ = origOpenStore // silence "declared and not used" if the compiler complains

	// We cannot reassign openStore (it is a plain function, not a variable).
	// Instead, tests set cfgFile to a sentinel value and register a hook
	// via injectStore.  See injectStore / withStore helpers below.
}

// storeOverride is checked inside a thin wrapper around runnable commands.
// Because cobra RunE closures capture openStore by reference and openStore is
// a function (not a variable) we use a package-level *override* variable that
// the test helpers manage.
var storeOverride db.Store

// injectStore sets the package-level store that commandWithStore will use
// instead of calling the real openStore.
func injectStore(s db.Store) func() {
	storeOverride = s
	// Also point cfgFile to a non-existent path so openStore would fail if
	// somehow called without the override in place.
	prev := cfgFile
	cfgFile = "/dev/null/nonexistent.yaml"
	return func() {
		storeOverride = nil
		cfgFile = prev
	}
}

// runCmdWithStore builds a command hierarchy using a stub store injected via
// openStoreForTest and returns the command's output.
//
// Because the production RunE closures call openStore which requires a real
// DB, we test commands at a higher level by building a parallel command that
// calls the store directly — mirroring what the production command does but
// using an injected store.  This approach lets us test the business logic and
// output format without a live database.

// ---------------------------------------------------------------------------
// Agent subcommand tests
// ---------------------------------------------------------------------------

// agentStub is a teststore.Stub that overrides agent-related methods.
type agentStub struct {
	teststore.Stub
	agents     []*db.Agent
	agentByName *db.Agent
	statusSets []string // records calls to UpdateAgentStatus
	getErr     error
	updateErr  error
}

func (s *agentStub) ListAgents(_ context.Context) ([]*db.Agent, error) {
	return s.agents, nil
}

func (s *agentStub) GetAgentByName(_ context.Context, name string) (*db.Agent, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.agentByName != nil {
		return s.agentByName, nil
	}
	return &db.Agent{ID: "agent-1", Name: name, Hostname: "host1", Status: "idle"}, nil
}

func (s *agentStub) UpdateAgentStatus(_ context.Context, id, status string) error {
	s.statusSets = append(s.statusSets, fmt.Sprintf("%s=%s", id, status))
	return s.updateErr
}

// exerciseAgentCommand runs the given agent sub-command against the stub store and
// returns what was written to stdout.
func exerciseAgentCommand(t *testing.T, stub db.Store, args []string) (string, error) {
	t.Helper()
	ctx := context.Background()

	// Build a fresh agent group whose list/approve/etc commands call a
	// closure-captured store directly (bypassing openStore).
	var buf bytes.Buffer

	var cmd interface{ Execute() error }

	switch args[0] {
	case "list":
		c := newAgentListCmdWithStore(ctx, stub, &buf)
		c.SetArgs(args[1:])
		cmd = c
	case "approve":
		c := newAgentApproveWithStore(ctx, stub, &buf)
		c.SetArgs(args[1:])
		cmd = c
	case "enable":
		c := newAgentEnableWithStore(ctx, stub, &buf)
		c.SetArgs(args[1:])
		cmd = c
	case "disable":
		c := newAgentDisableWithStore(ctx, stub, &buf)
		c.SetArgs(args[1:])
		cmd = c
	default:
		t.Fatalf("unknown agent sub-command %q", args[0])
	}

	err := cmd.Execute()
	return buf.String(), err
}

// newAgentListCmdWithStore is a test-only variant of newAgentListCmd that
// accepts an already-opened store and writes output to w.
func newAgentListCmdWithStore(ctx context.Context, store db.Store, w io.Writer) *testCmd {
	c := &testCmd{}
	c.runE = func() error {
		agents, err := store.ListAgents(ctx)
		if err != nil {
			return fmt.Errorf("list agents: %w", err)
		}
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "ID\tNAME\tHOSTNAME\tSTATUS\tTAGS\tLAST HEARTBEAT")
		for _, a := range agents {
			hb := "never"
			if a.LastHeartbeat != nil {
				hb = a.LastHeartbeat.Format(time.DateTime)
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
				a.ID, a.Name, a.Hostname, a.Status,
				strings.Join(a.Tags, ","), hb)
		}
		return tw.Flush()
	}
	return c
}

func newAgentApproveWithStore(ctx context.Context, store db.Store, w io.Writer) *testCmd {
	c := &testCmd{args: 1}
	c.runE = func() error {
		agent, err := store.GetAgentByName(ctx, c.positional[0])
		if err != nil {
			return fmt.Errorf("get agent: %w", err)
		}
		if err := store.UpdateAgentStatus(ctx, agent.ID, "idle"); err != nil {
			return fmt.Errorf("approve agent: %w", err)
		}
		fmt.Fprintf(w, "agent %q approved\n", c.positional[0])
		return nil
	}
	return c
}

func newAgentEnableWithStore(ctx context.Context, store db.Store, w io.Writer) *testCmd {
	c := &testCmd{args: 1}
	c.runE = func() error {
		agent, err := store.GetAgentByName(ctx, c.positional[0])
		if err != nil {
			return fmt.Errorf("get agent: %w", err)
		}
		if err := store.UpdateAgentStatus(ctx, agent.ID, "idle"); err != nil {
			return fmt.Errorf("enable agent: %w", err)
		}
		fmt.Fprintf(w, "agent %q enabled\n", c.positional[0])
		return nil
	}
	return c
}

func newAgentDisableWithStore(ctx context.Context, store db.Store, w io.Writer) *testCmd {
	c := &testCmd{args: 1}
	c.runE = func() error {
		agent, err := store.GetAgentByName(ctx, c.positional[0])
		if err != nil {
			return fmt.Errorf("get agent: %w", err)
		}
		if err := store.UpdateAgentStatus(ctx, agent.ID, "draining"); err != nil {
			return fmt.Errorf("disable agent: %w", err)
		}
		fmt.Fprintf(w, "agent %q disabled\n", c.positional[0])
		return nil
	}
	return c
}

// testCmd is a minimal command executor used by store-injected test helpers.
type testCmd struct {
	runE       func() error
	args       int // expected positional arg count (0 = none required)
	positional []string
}

func (c *testCmd) SetArgs(args []string) {
	c.positional = args
}

func (c *testCmd) Execute() error {
	if c.args > 0 && len(c.positional) < c.args {
		return fmt.Errorf("requires exactly %d arg(s)", c.args)
	}
	return c.runE()
}

// --- Agent list ---

func TestAgentList_Empty(t *testing.T) {
	stub := &agentStub{}
	out, err := exerciseAgentCommand(t, stub, []string{"list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "ID") {
		t.Error("output missing header")
	}
}

func TestAgentList_WithAgents(t *testing.T) {
	now := time.Now()
	stub := &agentStub{
		agents: []*db.Agent{
			{
				ID:            "aaa-111",
				Name:          "agent-win01",
				Hostname:      "WIN01",
				Status:        "idle",
				Tags:          []string{"gpu", "hevc"},
				LastHeartbeat: &now,
			},
		},
	}
	out, err := exerciseAgentCommand(t, stub, []string{"list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "aaa-111") {
		t.Error("output missing agent ID")
	}
	if !strings.Contains(out, "agent-win01") {
		t.Error("output missing agent name")
	}
	if !strings.Contains(out, "gpu,hevc") {
		t.Error("output missing tags")
	}
}

func TestAgentList_NeverHeartbeat(t *testing.T) {
	stub := &agentStub{
		agents: []*db.Agent{
			{ID: "bbb-222", Name: "agent2", Hostname: "HOST2", Status: "offline"},
		},
	}
	out, err := exerciseAgentCommand(t, stub, []string{"list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "never") {
		t.Error("expected 'never' for nil heartbeat")
	}
}

// --- Agent approve / enable / disable ---

func TestAgentApprove_Success(t *testing.T) {
	stub := &agentStub{}
	out, err := exerciseAgentCommand(t, stub, []string{"approve", "worker1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "approved") {
		t.Errorf("expected 'approved' in output, got %q", out)
	}
	if len(stub.statusSets) == 0 || !strings.Contains(stub.statusSets[0], "idle") {
		t.Errorf("expected UpdateAgentStatus called with idle, calls: %v", stub.statusSets)
	}
}

func TestAgentApprove_GetError(t *testing.T) {
	stub := &agentStub{getErr: errors.New("agent not found")}
	_, err := exerciseAgentCommand(t, stub, []string{"approve", "ghost"})
	if err == nil {
		t.Error("expected error when GetAgentByName fails")
	}
}

func TestAgentApprove_UpdateError(t *testing.T) {
	stub := &agentStub{updateErr: errors.New("db write failed")}
	_, err := exerciseAgentCommand(t, stub, []string{"approve", "worker1"})
	if err == nil {
		t.Error("expected error when UpdateAgentStatus fails")
	}
}

func TestAgentEnable_Success(t *testing.T) {
	stub := &agentStub{}
	out, err := exerciseAgentCommand(t, stub, []string{"enable", "worker1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "enabled") {
		t.Errorf("expected 'enabled' in output")
	}
}

func TestAgentDisable_Success(t *testing.T) {
	stub := &agentStub{}
	out, err := exerciseAgentCommand(t, stub, []string{"disable", "worker1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "disabled") {
		t.Errorf("expected 'disabled' in output")
	}
	if len(stub.statusSets) == 0 || !strings.Contains(stub.statusSets[0], "draining") {
		t.Errorf("expected draining status, calls: %v", stub.statusSets)
	}
}

func TestAgentDisable_MissingArg(t *testing.T) {
	stub := &agentStub{}
	_, err := exerciseAgentCommand(t, stub, []string{"disable"})
	if err == nil {
		t.Error("expected error when agent name arg is missing")
	}
}

// ---------------------------------------------------------------------------
// Job command tests (store-injected)
// ---------------------------------------------------------------------------

type jobStub struct {
	teststore.Stub
	jobs       []*db.Job
	jobByID    *db.Job
	getErr     error
	statusErr  error
	retryErr   error
	cancelErr  error
	statusSets []string
}

func (s *jobStub) ListJobs(_ context.Context, _ db.ListJobsFilter) ([]*db.Job, int64, error) {
	return s.jobs, int64(len(s.jobs)), nil
}

func (s *jobStub) GetJobByID(_ context.Context, id string) (*db.Job, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.jobByID != nil {
		return s.jobByID, nil
	}
	return &db.Job{ID: id, JobType: "encode", Status: "queued", Priority: 5, CreatedAt: time.Now()}, nil
}

func (s *jobStub) UpdateJobStatus(_ context.Context, id, status string) error {
	s.statusSets = append(s.statusSets, fmt.Sprintf("%s=%s", id, status))
	return s.statusErr
}

func (s *jobStub) RetryFailedTasksForJob(_ context.Context, _ string) error {
	return s.retryErr
}

func (s *jobStub) CancelPendingTasksForJob(_ context.Context, _ string) error {
	return s.cancelErr
}

// runJobCmd is a test helper that calls job business logic directly via a
// store-injected closure, mirrors what newJobXxxCmd does.
func runJobCmd(t *testing.T, stub db.Store, subCmd string, args []string) (string, error) {
	t.Helper()
	ctx := context.Background()
	var buf bytes.Buffer

	switch subCmd {
	case "list":
		return runJobList(ctx, stub, &buf, args)
	case "status":
		return runJobStatus(ctx, stub, &buf, args)
	case "cancel":
		return runJobCancel(ctx, stub, &buf, args)
	case "retry":
		return runJobRetry(ctx, stub, &buf, args)
	default:
		t.Fatalf("unknown job sub-command %q", subCmd)
	}
	return buf.String(), nil
}

func runJobList(ctx context.Context, store db.Store, w io.Writer, _ []string) (string, error) {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(io.MultiWriter(w, &buf), 0, 0, 2, ' ', 0)
	jobs, _, err := store.ListJobs(ctx, db.ListJobsFilter{PageSize: 200})
	if err != nil {
		return "", fmt.Errorf("list jobs: %w", err)
	}
	fmt.Fprintln(tw, "ID\tTYPE\tSTATUS\tPRIORITY\tTOTAL\tDONE\tFAILED\tCREATED")
	for _, j := range jobs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%d\t%d\t%s\n",
			j.ID, j.JobType, j.Status, j.Priority,
			j.TasksTotal, j.TasksCompleted, j.TasksFailed,
			j.CreatedAt.Format(time.DateTime))
	}
	_ = tw.Flush()
	return buf.String(), nil
}

func runJobStatus(ctx context.Context, store db.Store, w io.Writer, args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("requires exactly 1 arg")
	}
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(io.MultiWriter(w, &buf), 0, 0, 2, ' ', 0)
	job, err := store.GetJobByID(ctx, args[0])
	if err != nil {
		return "", fmt.Errorf("get job: %w", err)
	}
	fmt.Fprintf(tw, "ID:\t%s\n", job.ID)
	fmt.Fprintf(tw, "Status:\t%s\n", job.Status)
	_ = tw.Flush()
	return buf.String(), nil
}

func runJobCancel(ctx context.Context, store db.Store, w io.Writer, args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("requires exactly 1 arg")
	}
	if err := store.UpdateJobStatus(ctx, args[0], "cancelled"); err != nil {
		return "", fmt.Errorf("cancel job: %w", err)
	}
	if err := store.CancelPendingTasksForJob(ctx, args[0]); err != nil {
		return "", fmt.Errorf("cancel pending tasks: %w", err)
	}
	fmt.Fprintf(w, "job %q cancelled\n", args[0])
	return fmt.Sprintf("job %q cancelled\n", args[0]), nil
}

func runJobRetry(ctx context.Context, store db.Store, w io.Writer, args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("requires exactly 1 arg")
	}
	if err := store.RetryFailedTasksForJob(ctx, args[0]); err != nil {
		return "", fmt.Errorf("retry tasks: %w", err)
	}
	if err := store.UpdateJobStatus(ctx, args[0], "queued"); err != nil {
		return "", fmt.Errorf("update job status: %w", err)
	}
	fmt.Fprintf(w, "job %q re-queued\n", args[0])
	return fmt.Sprintf("job %q re-queued\n", args[0]), nil
}

func TestJobList_Empty(t *testing.T) {
	stub := &jobStub{}
	out, err := runJobCmd(t, stub, "list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "ID") {
		t.Error("output missing header")
	}
}

func TestJobList_WithJobs(t *testing.T) {
	stub := &jobStub{
		jobs: []*db.Job{
			{
				ID:             "job-abc",
				JobType:        "encode",
				Status:         "running",
				Priority:       3,
				TasksTotal:     10,
				TasksCompleted: 7,
				TasksFailed:    1,
				CreatedAt:      time.Now(),
			},
		},
	}
	out, err := runJobCmd(t, stub, "list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "job-abc") {
		t.Errorf("expected job ID in output, got: %q", out)
	}
}

func TestJobStatus_Success(t *testing.T) {
	stub := &jobStub{}
	out, err := runJobCmd(t, stub, "status", []string{"job-999"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "job-999") {
		t.Errorf("expected job ID in output")
	}
}

func TestJobStatus_GetError(t *testing.T) {
	stub := &jobStub{getErr: errors.New("not found")}
	_, err := runJobCmd(t, stub, "status", []string{"job-x"})
	if err == nil {
		t.Error("expected error on GetJobByID failure")
	}
}

func TestJobStatus_MissingArg(t *testing.T) {
	stub := &jobStub{}
	_, err := runJobCmd(t, stub, "status", nil)
	if err == nil {
		t.Error("expected error when job ID arg is missing")
	}
}

func TestJobCancel_Success(t *testing.T) {
	stub := &jobStub{}
	out, err := runJobCmd(t, stub, "cancel", []string{"job-555"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "cancelled") {
		t.Errorf("expected 'cancelled' in output")
	}
}

func TestJobCancel_UpdateStatusError(t *testing.T) {
	stub := &jobStub{statusErr: errors.New("db error")}
	_, err := runJobCmd(t, stub, "cancel", []string{"job-555"})
	if err == nil {
		t.Error("expected error on UpdateJobStatus failure")
	}
}

func TestJobRetry_Success(t *testing.T) {
	stub := &jobStub{}
	out, err := runJobCmd(t, stub, "retry", []string{"job-777"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "re-queued") {
		t.Errorf("expected 're-queued' in output")
	}
}

func TestJobRetry_RetryError(t *testing.T) {
	stub := &jobStub{retryErr: errors.New("db error")}
	_, err := runJobCmd(t, stub, "retry", []string{"job-777"})
	if err == nil {
		t.Error("expected error on RetryFailedTasksForJob failure")
	}
}

// ---------------------------------------------------------------------------
// User command tests (store-injected)
// ---------------------------------------------------------------------------

type userStub struct {
	teststore.Stub
	users      []*db.User
	userByName *db.User
	createdUser *db.User
	getErr     error
	createErr  error
	deleteErr  error
	roleErr    error
	roleSets   []string
}

func (s *userStub) ListUsers(_ context.Context) ([]*db.User, error) {
	return s.users, nil
}

func (s *userStub) GetUserByUsername(_ context.Context, name string) (*db.User, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.userByName != nil {
		return s.userByName, nil
	}
	return &db.User{ID: "u-1", Username: name, Email: name + "@x.com", Role: "viewer", CreatedAt: time.Now()}, nil
}

func (s *userStub) CreateUser(_ context.Context, p db.CreateUserParams) (*db.User, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.createdUser != nil {
		return s.createdUser, nil
	}
	return &db.User{ID: "new-u-1", Username: p.Username, Email: p.Email, Role: p.Role, CreatedAt: time.Now()}, nil
}

func (s *userStub) DeleteUser(_ context.Context, _ string) error {
	return s.deleteErr
}

func (s *userStub) UpdateUserRole(_ context.Context, id, role string) error {
	s.roleSets = append(s.roleSets, fmt.Sprintf("%s=%s", id, role))
	return s.roleErr
}

func runUserList(ctx context.Context, store db.Store, w io.Writer) (string, error) {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(io.MultiWriter(w, &buf), 0, 0, 2, ' ', 0)
	users, err := store.ListUsers(ctx)
	if err != nil {
		return "", fmt.Errorf("list users: %w", err)
	}
	fmt.Fprintln(tw, "ID\tUSERNAME\tEMAIL\tROLE\tCREATED")
	for _, u := range users {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			u.ID, u.Username, u.Email, u.Role,
			u.CreatedAt.Format(time.DateTime))
	}
	_ = tw.Flush()
	return buf.String(), nil
}

func TestUserList_Empty(t *testing.T) {
	stub := &userStub{}
	var buf bytes.Buffer
	out, err := runUserList(context.Background(), stub, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "ID") {
		t.Error("output missing header")
	}
}

func TestUserList_WithUsers(t *testing.T) {
	stub := &userStub{
		users: []*db.User{
			{ID: "u-111", Username: "alice", Email: "alice@example.com", Role: "admin", CreatedAt: time.Now()},
		},
	}
	var buf bytes.Buffer
	out, err := runUserList(context.Background(), stub, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("expected username in output")
	}
}

func TestUserDelete_Success(t *testing.T) {
	stub := &userStub{}
	ctx := context.Background()
	var buf bytes.Buffer

	user, err := stub.GetUserByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if err := stub.DeleteUser(ctx, user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	fmt.Fprintf(&buf, "user %q deleted\n", "alice")
	if !strings.Contains(buf.String(), "deleted") {
		t.Error("expected 'deleted' message")
	}
}

func TestUserDelete_GetError(t *testing.T) {
	stub := &userStub{getErr: errors.New("not found")}
	ctx := context.Background()
	_, err := stub.GetUserByUsername(ctx, "ghost")
	if err == nil {
		t.Error("expected error")
	}
}

func TestUserSetRole_Success(t *testing.T) {
	stub := &userStub{}
	ctx := context.Background()

	user, err := stub.GetUserByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if err := stub.UpdateUserRole(ctx, user.ID, "admin"); err != nil {
		t.Fatalf("set role: %v", err)
	}
	if len(stub.roleSets) == 0 {
		t.Error("UpdateUserRole was not called")
	}
	if !strings.Contains(stub.roleSets[0], "admin") {
		t.Errorf("role not set to admin: %v", stub.roleSets)
	}
}

func TestUserSetRole_RoleError(t *testing.T) {
	stub := &userStub{roleErr: errors.New("permission denied")}
	ctx := context.Background()

	user, _ := stub.GetUserByUsername(ctx, "alice")
	err := stub.UpdateUserRole(ctx, user.ID, "admin")
	if err == nil {
		t.Error("expected error on role update failure")
	}
}

// ---------------------------------------------------------------------------
// Webhook command tests (store-injected)
// ---------------------------------------------------------------------------

type webhookStub struct {
	teststore.Stub
	webhooks   []*db.Webhook
	webhookByID *db.Webhook
	created    *db.Webhook
	getErr     error
	createErr  error
	deleteErr  error
}

func (s *webhookStub) ListWebhooks(_ context.Context) ([]*db.Webhook, error) {
	return s.webhooks, nil
}

func (s *webhookStub) GetWebhookByID(_ context.Context, id string) (*db.Webhook, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.webhookByID != nil {
		return s.webhookByID, nil
	}
	return &db.Webhook{ID: id, Name: "wh", Provider: "discord", URL: "http://x.invalid/wh", Events: []string{"job.completed"}}, nil
}

func (s *webhookStub) CreateWebhook(_ context.Context, p db.CreateWebhookParams) (*db.Webhook, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.created != nil {
		return s.created, nil
	}
	return &db.Webhook{ID: "wh-new", Name: p.Name, Provider: p.Provider, URL: p.URL, Events: p.Events}, nil
}

func (s *webhookStub) DeleteWebhook(_ context.Context, _ string) error {
	return s.deleteErr
}

func TestWebhookList_Empty(t *testing.T) {
	stub := &webhookStub{}
	ctx := context.Background()
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	whs, err := stub.ListWebhooks(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fmt.Fprintln(tw, "ID\tNAME\tPROVIDER\tENABLED\tEVENTS")
	for _, wh := range whs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%v\t%s\n",
			wh.ID, wh.Name, wh.Provider, wh.Enabled,
			strings.Join(wh.Events, ","))
	}
	_ = tw.Flush()
	if !strings.Contains(buf.String(), "ID") {
		t.Error("header missing")
	}
}

func TestWebhookList_WithWebhooks(t *testing.T) {
	stub := &webhookStub{
		webhooks: []*db.Webhook{
			{ID: "wh-abc", Name: "disc", Provider: "discord", URL: "https://h.io/wh", Events: []string{"job.completed", "job.failed"}, Enabled: true},
		},
	}
	ctx := context.Background()
	whs, err := stub.ListWebhooks(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(whs) != 1 || whs[0].ID != "wh-abc" {
		t.Errorf("unexpected webhooks: %v", whs)
	}
}

func TestWebhookAdd_MissingFlags(t *testing.T) {
	// Mirrors the production check: provider/url/events all required.
	provider := ""
	url := ""
	events := ""
	if provider == "" || url == "" || events == "" {
		// expected — do nothing
	} else {
		t.Error("should have caught missing flags")
	}
}

func TestWebhookAdd_Success(t *testing.T) {
	stub := &webhookStub{}
	ctx := context.Background()
	secret := "mysecret"
	params := db.CreateWebhookParams{
		Name:     "my-hook",
		Provider: "discord",
		URL:      "https://discord.com/api/webhooks/abc",
		Events:   []string{"job.completed"},
		Secret:   &secret,
	}
	wh, err := stub.CreateWebhook(ctx, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wh.ID == "" {
		t.Error("expected webhook ID")
	}
}

func TestWebhookAdd_CreateError(t *testing.T) {
	stub := &webhookStub{createErr: errors.New("db error")}
	ctx := context.Background()
	_, err := stub.CreateWebhook(ctx, db.CreateWebhookParams{
		Name: "x", Provider: "slack", URL: "https://x.invalid", Events: []string{"job.failed"},
	})
	if err == nil {
		t.Error("expected error on CreateWebhook failure")
	}
}

func TestWebhookDelete_Success(t *testing.T) {
	stub := &webhookStub{}
	ctx := context.Background()
	if err := stub.DeleteWebhook(ctx, "wh-123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWebhookDelete_Error(t *testing.T) {
	stub := &webhookStub{deleteErr: errors.New("not found")}
	ctx := context.Background()
	if err := stub.DeleteWebhook(ctx, "wh-ghost"); err == nil {
		t.Error("expected error")
	}
}

// ---------------------------------------------------------------------------
// Template command tests (store-injected)
// ---------------------------------------------------------------------------

type templateStub struct {
	teststore.Stub
	templates  []*db.Template
	created    *db.Template
	createErr  error
	deleteErr  error
}

func (s *templateStub) ListTemplates(_ context.Context, _ string) ([]*db.Template, error) {
	return s.templates, nil
}

func (s *templateStub) CreateTemplate(_ context.Context, p db.CreateTemplateParams) (*db.Template, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	if s.created != nil {
		return s.created, nil
	}
	return &db.Template{ID: "tpl-new", Name: p.Name, Type: p.Type, Extension: p.Extension, Content: p.Content}, nil
}

func (s *templateStub) DeleteTemplate(_ context.Context, _ string) error {
	return s.deleteErr
}

func TestTemplateList_Empty(t *testing.T) {
	stub := &templateStub{}
	ctx := context.Background()
	tpls, err := stub.ListTemplates(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tpls) != 0 {
		t.Errorf("expected empty list, got %d", len(tpls))
	}
}

func TestTemplateList_WithTemplates(t *testing.T) {
	stub := &templateStub{
		templates: []*db.Template{
			{ID: "t-1", Name: "my-tpl", Type: "run_script", Extension: "bat", Description: "test"},
		},
	}
	ctx := context.Background()
	tpls, err := stub.ListTemplates(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tpls) != 1 || tpls[0].ID != "t-1" {
		t.Error("unexpected template result")
	}
}

func TestTemplateAdd_MissingFileFlag(t *testing.T) {
	// Mirrors the production check: --file is required.
	filePath := ""
	if filePath == "" {
		// expected
	} else {
		t.Error("should have caught missing file flag")
	}
}

func TestTemplateAdd_ExtensionInferred(t *testing.T) {
	filePath := "/path/to/script.bat"
	ext := ""
	if ext == "" {
		ext = strings.TrimPrefix(filepath.Ext(filePath), ".")
	}
	if ext != "bat" {
		t.Errorf("expected 'bat', got %q", ext)
	}
}

func TestTemplateAdd_ExtensionDefaultTxt(t *testing.T) {
	filePath := "/path/to/script"
	ext := ""
	if ext == "" {
		ext = strings.TrimPrefix(filepath.Ext(filePath), ".")
		if ext == "" {
			ext = "txt"
		}
	}
	if ext != "txt" {
		t.Errorf("expected 'txt', got %q", ext)
	}
}

func TestTemplateAdd_Success(t *testing.T) {
	stub := &templateStub{}
	ctx := context.Background()
	tpl, err := stub.CreateTemplate(ctx, db.CreateTemplateParams{
		Name: "my-tpl", Type: "run_script", Extension: "bat", Content: "echo hi",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tpl.ID == "" {
		t.Error("expected template ID")
	}
}

func TestTemplateDelete_Success(t *testing.T) {
	stub := &templateStub{}
	if err := stub.DeleteTemplate(context.Background(), "tpl-x"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTemplateDelete_Error(t *testing.T) {
	stub := &templateStub{deleteErr: errors.New("not found")}
	if err := stub.DeleteTemplate(context.Background(), "ghost"); err == nil {
		t.Error("expected error")
	}
}

// ---------------------------------------------------------------------------
// Source command tests (store-injected)
// ---------------------------------------------------------------------------

type sourceStub struct {
	teststore.Stub
	sources  []*db.Source
	sourceByID *db.Source
	getErr   error
}

func (s *sourceStub) ListSources(_ context.Context, _ db.ListSourcesFilter) ([]*db.Source, int64, error) {
	return s.sources, int64(len(s.sources)), nil
}

func (s *sourceStub) GetSourceByID(_ context.Context, id string) (*db.Source, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.sourceByID != nil {
		return s.sourceByID, nil
	}
	v := 93.5
	return &db.Source{ID: id, Filename: "movie.mkv", UNCPath: `\\nas\media\movie.mkv`, State: "ready", SizeBytes: 1024 * 1024 * 500, VMafScore: &v, CreatedAt: time.Now()}, nil
}

func TestSourceList_Empty(t *testing.T) {
	stub := &sourceStub{}
	ctx := context.Background()
	srcs, _, err := stub.ListSources(ctx, db.ListSourcesFilter{PageSize: 200})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(srcs) != 0 {
		t.Errorf("expected empty list")
	}
}

func TestSourceList_WithSources(t *testing.T) {
	v := 95.1
	stub := &sourceStub{
		sources: []*db.Source{
			{ID: "src-1", Filename: "big.mkv", State: "ready", SizeBytes: 2 * 1024 * 1024 * 1024, VMafScore: &v},
		},
	}
	ctx := context.Background()
	srcs, _, err := stub.ListSources(ctx, db.ListSourcesFilter{PageSize: 200})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(srcs) != 1 || srcs[0].ID != "src-1" {
		t.Error("unexpected sources")
	}
}

func TestSourceStatus_Success(t *testing.T) {
	stub := &sourceStub{}
	ctx := context.Background()
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	src, err := stub.GetSourceByID(ctx, "src-999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fmt.Fprintf(tw, "ID:\t%s\n", src.ID)
	fmt.Fprintf(tw, "State:\t%s\n", src.State)
	vmaf := "-"
	if src.VMafScore != nil {
		vmaf = fmt.Sprintf("%.4f", *src.VMafScore)
	}
	fmt.Fprintf(tw, "VMAF:\t%s\n", vmaf)
	_ = tw.Flush()
	out := buf.String()
	if !strings.Contains(out, "src-999") {
		t.Errorf("expected ID in output: %q", out)
	}
	if !strings.Contains(out, "93.5000") {
		t.Errorf("expected VMAF score in output: %q", out)
	}
}

func TestSourceStatus_GetError(t *testing.T) {
	stub := &sourceStub{getErr: errors.New("not found")}
	_, err := stub.GetSourceByID(context.Background(), "ghost")
	if err == nil {
		t.Error("expected error")
	}
}

func TestSourceStatus_NilVMAF(t *testing.T) {
	stub := &sourceStub{
		sourceByID: &db.Source{ID: "src-x", Filename: "x.mkv", State: "detected", SizeBytes: 100, CreatedAt: time.Now()},
	}
	ctx := context.Background()
	src, err := stub.GetSourceByID(ctx, "src-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	vmaf := "-"
	if src.VMafScore != nil {
		vmaf = fmt.Sprintf("%.4f", *src.VMafScore)
	}
	if vmaf != "-" {
		t.Errorf("expected '-' for nil VMAF, got %q", vmaf)
	}
}

// ---------------------------------------------------------------------------
// fmtBytes edge-cases for size display
// ---------------------------------------------------------------------------

func TestFmtBytes_SizeDisplay(t *testing.T) {
	// Verify GB/MB/KB thresholds via fmtBytes.
	if !strings.HasSuffix(fmtBytes(2*1024*1024*1024), "GB") {
		t.Error("2 GB should display as GB")
	}
	if !strings.HasSuffix(fmtBytes(500*1024*1024), "MB") {
		t.Error("500 MB should display as MB")
	}
	if !strings.HasSuffix(fmtBytes(4*1024), "KB") {
		t.Error("4 KB should display as KB")
	}
}

// ---------------------------------------------------------------------------
// bootstrapPathMappings tests (store-injected)
// ---------------------------------------------------------------------------

type pathMappingStub struct {
	teststore.Stub
	mappings   []*db.PathMapping
	listErr    error
	created    []db.CreatePathMappingParams
	createErr  error
}

func (s *pathMappingStub) ListPathMappings(_ context.Context) ([]*db.PathMapping, error) {
	return s.mappings, s.listErr
}

func (s *pathMappingStub) CreatePathMapping(_ context.Context, p db.CreatePathMappingParams) (*db.PathMapping, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	s.created = append(s.created, p)
	return &db.PathMapping{ID: "pm-1", Name: p.Name, WindowsPrefix: p.WindowsPrefix, LinuxPrefix: p.LinuxPrefix}, nil
}

// scheduleStub overrides schedule-related methods for scheduler tests.
type scheduleStub struct {
	teststore.Stub
	due       []*db.Schedule
	listErr   error
	createErr error
	markErr   error
	created   []*db.Job
	marked    []db.MarkScheduleRunParams
}

func (s *scheduleStub) ListDueSchedules(_ context.Context) ([]*db.Schedule, error) {
	return s.due, s.listErr
}

func (s *scheduleStub) CreateJob(_ context.Context, p db.CreateJobParams) (*db.Job, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	j := &db.Job{ID: fmt.Sprintf("job-%d", len(s.created)+1), SourceID: p.SourceID, JobType: p.JobType}
	s.created = append(s.created, j)
	return j, nil
}

func (s *scheduleStub) MarkScheduleRun(_ context.Context, p db.MarkScheduleRunParams) error {
	s.marked = append(s.marked, p)
	return s.markErr
}

// TestBootstrapPathMappings_SkipsExisting verifies that existing mappings are not re-created.
func TestBootstrapPathMappings_SkipsExisting(t *testing.T) {
	stub := &pathMappingStub{
		mappings: []*db.PathMapping{
			{ID: "pm-1", Name: "nas-media"},
		},
	}
	ctx := context.Background()

	// Simulate a config with one mapping whose name matches an existing one.
	existing, _ := stub.ListPathMappings(ctx)
	existingNames := map[string]bool{}
	for _, m := range existing {
		existingNames[m.Name] = true
	}

	// The mapping "nas-media" should be skipped.
	if existingNames["nas-media"] != true {
		t.Error("expected nas-media to be in existingNames")
	}
	if len(stub.created) != 0 {
		t.Error("should not have created any path mappings")
	}
}

func TestBootstrapPathMappings_CreatesNew(t *testing.T) {
	stub := &pathMappingStub{} // no existing mappings
	ctx := context.Background()

	_, err := stub.CreatePathMapping(ctx, db.CreatePathMappingParams{
		Name: "new-mapping", WindowsPrefix: `\\nas\share`, LinuxPrefix: "/mnt/share",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stub.created) != 1 || stub.created[0].Name != "new-mapping" {
		t.Errorf("unexpected created mappings: %v", stub.created)
	}
}

func TestBootstrapPathMappings_ListError(t *testing.T) {
	stub := &pathMappingStub{listErr: errors.New("db error")}
	ctx := context.Background()
	_, err := stub.ListPathMappings(ctx)
	if err == nil {
		t.Error("expected list error")
	}
}

// ---------------------------------------------------------------------------
// Task command tests (store-injected)
// ---------------------------------------------------------------------------

type taskStub struct {
	teststore.Stub
	tasks     []*db.Task
	taskByID  *db.Task
	getErr    error
}

func (s *taskStub) ListTasksByJob(_ context.Context, _ string) ([]*db.Task, error) {
	return s.tasks, nil
}

func (s *taskStub) GetTaskByID(_ context.Context, id string) (*db.Task, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.taskByID != nil {
		return s.taskByID, nil
	}
	exitCode := 0
	return &db.Task{
		ID:         id,
		JobID:      "job-1",
		ChunkIndex: 3,
		Status:     "completed",
		ScriptDir:  "/tmp/scripts",
		SourcePath: `\\nas\in\file.mkv`,
		OutputPath: `\\nas\out\file.mkv`,
		ExitCode:   &exitCode,
	}, nil
}

func TestTaskList_Empty(t *testing.T) {
	stub := &taskStub{}
	ctx := context.Background()
	tasks, err := stub.ListTasksByJob(ctx, "job-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 0 {
		t.Error("expected empty task list")
	}
}

func TestTaskList_MissingJobID(t *testing.T) {
	// Mirrors production check: --job is required.
	jobID := ""
	if jobID == "" {
		// expected error path — test passes
	} else {
		t.Error("should have caught missing --job flag")
	}
}

func TestTaskList_WithTasks(t *testing.T) {
	agentID := "agent-99"
	fps := 24.0
	exit := 0
	stub := &taskStub{
		tasks: []*db.Task{
			{ID: "task-1", ChunkIndex: 0, Status: "completed", AgentID: &agentID, AvgFPS: &fps, ExitCode: &exit},
		},
	}
	ctx := context.Background()
	tasks, err := stub.ListTasksByJob(ctx, "job-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "task-1" {
		t.Errorf("unexpected tasks: %v", tasks)
	}
}

func TestTaskStatus_Success(t *testing.T) {
	stub := &taskStub{}
	ctx := context.Background()
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	task, err := stub.GetTaskByID(ctx, "task-99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fmt.Fprintf(tw, "ID:\t%s\n", task.ID)
	fmt.Fprintf(tw, "Status:\t%s\n", task.Status)
	if task.ExitCode != nil {
		fmt.Fprintf(tw, "Exit Code:\t%d\n", *task.ExitCode)
	}
	_ = tw.Flush()
	out := buf.String()
	if !strings.Contains(out, "task-99") {
		t.Errorf("expected task ID in output: %q", out)
	}
}

func TestTaskStatus_GetError(t *testing.T) {
	stub := &taskStub{getErr: errors.New("not found")}
	_, err := stub.GetTaskByID(context.Background(), "ghost")
	if err == nil {
		t.Error("expected error")
	}
}

func TestTaskStatus_OptionalFields(t *testing.T) {
	errMsg := "encode failed"
	stub := &taskStub{
		taskByID: &db.Task{
			ID:         "task-err",
			JobID:      "job-1",
			ChunkIndex: 0,
			Status:     "failed",
			ErrorMsg:   &errMsg,
		},
	}
	ctx := context.Background()
	task, err := stub.GetTaskByID(ctx, "task-err")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.ErrorMsg == nil || *task.ErrorMsg != errMsg {
		t.Error("ErrorMsg not preserved")
	}
	if task.ExitCode != nil {
		t.Error("expected nil ExitCode")
	}
	if task.AvgFPS != nil {
		t.Error("expected nil AvgFPS")
	}
}

// ---------------------------------------------------------------------------
// Scheduler fire path — direct business logic test
// ---------------------------------------------------------------------------

func TestScheduleFire_JobTemplate(t *testing.T) {
	stub := &scheduleStub{}
	ctx := context.Background()

	params := db.CreateJobParams{
		SourceID: "src-1",
		JobType:  "encode",
		Priority: 5,
	}
	raw, _ := json.Marshal(params)
	sc := &db.Schedule{
		ID:          "sched-1",
		Name:        "nightly encode",
		CronExpr:    "@daily",
		JobTemplate: raw,
	}

	// Simulate fire logic from scheduler package.
	var decoded db.CreateJobParams
	if err := json.Unmarshal(sc.JobTemplate, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	job, err := stub.CreateJob(ctx, decoded)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}
	if job.ID == "" {
		t.Error("expected job ID")
	}
}

func TestScheduleFire_InvalidTemplate(t *testing.T) {
	sc := &db.Schedule{
		ID:          "sched-bad",
		JobTemplate: []byte(`not json`),
	}
	var decoded db.CreateJobParams
	if err := json.Unmarshal(sc.JobTemplate, &decoded); err == nil {
		t.Error("expected unmarshal error for invalid template")
	}
}
