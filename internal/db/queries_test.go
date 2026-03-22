// Package db tests use pgxmock to verify query logic without a live database.
// Every function in queries.go has at least a success-path and an error-path test.
package db

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	pgxmock "github.com/pashagolub/pgxmock/v4"
)

// newMock creates a fresh pgxmock pool and wraps it in a pgStore.
func newMock(t *testing.T) (*pgStore, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	return &pgStore{pool: mock}, mock
}

// anyArg matches any single argument in pgxmock expectations.
var anyArg = pgxmock.AnyArg()

// now is a reusable timestamp for row data.
var now = time.Now()

// ---------------------------------------------------------------------------
// Ping
// ---------------------------------------------------------------------------

func TestPing_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectPing()
	if err := s.Ping(context.Background()); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestPing_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectPing().WillReturnError(errors.New("conn refused"))
	if err := s.Ping(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// joinConditions (pure logic — no DB needed)
// ---------------------------------------------------------------------------

func TestJoinConditions(t *testing.T) {
	if got := joinConditions(nil); got != "" {
		t.Errorf("empty: want '', got %q", got)
	}
	if got := joinConditions([]string{"a=1"}); got != "a=1" {
		t.Errorf("single: got %q", got)
	}
	if got := joinConditions([]string{"a=1", "b=2"}); got != "a=1 AND b=2" {
		t.Errorf("two: got %q", got)
	}
}

// ---------------------------------------------------------------------------
// helpers — column lists for rows
// ---------------------------------------------------------------------------

func userCols() []string {
	return []string{"id", "username", "email", "role", "password_hash", "oidc_sub", "created_at", "updated_at"}
}

func userRow(id, username, email, role string) *pgxmock.Rows {
	return pgxmock.NewRows(userCols()).
		AddRow(id, username, email, role, nil, nil, now, now)
}

func agentCols() []string {
	return []string{
		"id", "name", "hostname", "ip_address", "status", "tags",
		"gpu_vendor", "gpu_model", "gpu_enabled",
		"agent_version", "os_version", "cpu_count", "ram_mib",
		"nvenc", "qsv", "amf", "vnc_port", "api_key_hash", "last_heartbeat",
		"upgrade_requested", "created_at", "updated_at",
	}
}

func agentRow(id, name string) *pgxmock.Rows {
	return pgxmock.NewRows(agentCols()).
		AddRow(id, name, "host1", "192.168.1.1", "idle", []string{"gpu"},
			"nvidia", "RTX 4090", true,
			"1.0.0", "Windows Server 2022", int32(16), int64(32768),
			true, false, false, 5900, nil, nil,
			false, now, now)
}

func sourceCols() []string {
	return []string{
		"id", "filename", "unc_path", "size_bytes", "detected_by",
		"state", "vmaf_score", "cloud_uri", "hdr_type", "dv_profile", "created_at", "updated_at",
	}
}

func sourceRow(id, filename string) *pgxmock.Rows {
	return pgxmock.NewRows(sourceCols()).
		AddRow(id, filename, `\\nas\share\`+filename, int64(1024*1024*1024),
			nil, "new", nil, nil, "", 0, now, now)
}

func jobCols() []string {
	return []string{
		"id", "source_id", "status", "job_type", "priority", "target_tags",
		"tasks_total", "tasks_pending", "tasks_running", "tasks_completed", "tasks_failed",
		"encode_config", "audio_config", "max_retries", "depends_on", "chain_group",
		"completed_at", "failed_at", "created_at", "updated_at",
		"source_path",
	}
}

func jobRow(id, sourceID string) *pgxmock.Rows {
	cfg := []byte(`{}`)
	return pgxmock.NewRows(jobCols()).
		AddRow(id, sourceID, "queued", "encode", 0, []string{},
			0, 0, 0, 0, 0,
			cfg, nil, 0, nil, nil,
			nil, nil, now, now,
			`\\nas\share\video.mkv`)
}

func taskCols() []string {
	return []string{
		"id", "job_id", "chunk_index", "task_type", "status", "agent_id",
		"script_dir", "source_path", "output_path", "variables",
		"exit_code", "frames_encoded", "avg_fps", "output_size", "duration_sec",
		"vmaf_score", "psnr", "ssim", "error_msg",
		"retry_count", "retry_after",
		"started_at", "completed_at", "created_at", "updated_at",
	}
}

func taskRow(id, jobID string) *pgxmock.Rows {
	vars := []byte(`{"key":"val"}`)
	return pgxmock.NewRows(taskCols()).
		AddRow(id, jobID, 0, "", "pending", nil,
			"", `\\nas\src.mkv`, `\\out\chunk0.mkv`, vars,
			nil, nil, nil, nil, nil,
			nil, nil, nil, nil,
			0, nil,
			nil, nil, now, now)
}

func templateCols() []string {
	return []string{"id", "name", "description", "type", "extension", "content", "created_at", "updated_at"}
}

func templateRow(id string) *pgxmock.Rows {
	return pgxmock.NewRows(templateCols()).
		AddRow(id, "tmpl1", "desc", "encode", ".bat", "::content", now, now)
}

func variableCols() []string {
	return []string{"id", "name", "value", "description", "category", "created_at", "updated_at"}
}

func variableRow(id string) *pgxmock.Rows {
	return pgxmock.NewRows(variableCols()).
		AddRow(id, "myvar", "myval", "desc", "general", now, now)
}

func webhookCols() []string {
	return []string{"id", "name", "provider", "url", "secret_hash", "events", "enabled", "created_at", "updated_at"}
}

func webhookRow(id string) *pgxmock.Rows {
	return pgxmock.NewRows(webhookCols()).
		AddRow(id, "hook1", "discord", "https://example.com/hook", nil, []string{"job.completed"}, true, now, now)
}

func analysisResultCols() []string {
	return []string{"id", "source_id", "type", "frame_data", "summary", "created_at"}
}

func analysisResultRow(id, sourceID string) *pgxmock.Rows {
	return pgxmock.NewRows(analysisResultCols()).
		AddRow(id, sourceID, "histogram", []byte("data"), []byte(`{}`), now)
}

func pathMappingCols() []string {
	return []string{"id", "name", "windows_prefix", "linux_prefix", "enabled", "created_at", "updated_at"}
}

func pathMappingRow(id string) *pgxmock.Rows {
	return pgxmock.NewRows(pathMappingCols()).
		AddRow(id, "nas-map", `\\NAS01\media`, "/mnt/nas/media", true, now, now)
}

func sessionCols() []string {
	return []string{"token", "user_id", "created_at", "expires_at"}
}

func sessionRow(token, userID string) *pgxmock.Rows {
	return pgxmock.NewRows(sessionCols()).
		AddRow(token, userID, now, now.Add(24*time.Hour))
}

func enrollmentTokenCols() []string {
	return []string{"id", "token", "created_by", "used_by", "used_at", "expires_at", "created_at"}
}

func enrollmentTokenRow(id, token string) *pgxmock.Rows {
	return pgxmock.NewRows(enrollmentTokenCols()).
		AddRow(id, token, "admin", nil, nil, now.Add(24*time.Hour), now)
}

func taskLogCols() []string {
	return []string{"id", "task_id", "job_id", "stream", "level", "message", "metadata", "logged_at"}
}

func taskLogRow(id int64, taskID, jobID string) *pgxmock.Rows {
	return pgxmock.NewRows(taskLogCols()).
		AddRow(id, taskID, jobID, "stdout", "info", "encoding frame 1", []byte(`{}`), now)
}

func scheduleCols() []string {
	return []string{"id", "name", "cron_expr", "job_template", "enabled", "last_run_at", "next_run_at", "created_at"}
}

func scheduleRow(id string) *pgxmock.Rows {
	tmpl, _ := json.Marshal(map[string]any{"job_type": "encode"})
	nextRun := now.Add(time.Hour)
	return pgxmock.NewRows(scheduleCols()).
		AddRow(id, "nightly", "0 2 * * *", tmpl, true, nil, &nextRun, now)
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

func TestCreateUser_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO users`)).
		WithArgs(anyArg, anyArg, anyArg, anyArg, anyArg).
		WillReturnRows(userRow("uid1", "alice", "alice@example.com", "admin"))
	u, err := s.CreateUser(context.Background(), CreateUserParams{
		Username: "alice", Email: "alice@example.com", Role: "admin",
	})
	if err != nil || u.ID != "uid1" {
		t.Fatalf("unexpected: err=%v u=%v", err, u)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestCreateUser_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO users`)).
		WithArgs(anyArg, anyArg, anyArg, anyArg, anyArg).
		WillReturnError(errors.New("duplicate key"))
	_, err := s.CreateUser(context.Background(), CreateUserParams{Username: "alice"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetUserByUsername_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM users WHERE username`)).
		WithArgs("alice").
		WillReturnRows(userRow("uid1", "alice", "alice@example.com", "viewer"))
	u, err := s.GetUserByUsername(context.Background(), "alice")
	if err != nil || u.Username != "alice" {
		t.Fatalf("err=%v u=%v", err, u)
	}
}

func TestGetUserByUsername_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM users WHERE username`)).
		WithArgs("nobody").
		WillReturnRows(pgxmock.NewRows(userCols()))
	_, err := s.GetUserByUsername(context.Background(), "nobody")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetUserByOIDCSub_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM users WHERE oidc_sub`)).
		WithArgs("sub123").
		WillReturnRows(userRow("uid2", "bob", "bob@example.com", "viewer"))
	u, err := s.GetUserByOIDCSub(context.Background(), "sub123")
	if err != nil || u.ID != "uid2" {
		t.Fatalf("err=%v u=%v", err, u)
	}
}

func TestGetUserByOIDCSub_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM users WHERE oidc_sub`)).
		WithArgs("missing").
		WillReturnRows(pgxmock.NewRows(userCols()))
	_, err := s.GetUserByOIDCSub(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetUserByID_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM users WHERE id`)).
		WithArgs("uid1").
		WillReturnRows(userRow("uid1", "alice", "alice@example.com", "admin"))
	u, err := s.GetUserByID(context.Background(), "uid1")
	if err != nil || u.ID != "uid1" {
		t.Fatalf("err=%v u=%v", err, u)
	}
}

func TestGetUserByID_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM users WHERE id`)).
		WithArgs("nope").
		WillReturnRows(pgxmock.NewRows(userCols()))
	_, err := s.GetUserByID(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListUsers_Success(t *testing.T) {
	s, mock := newMock(t)
	rows := pgxmock.NewRows(userCols()).
		AddRow("uid1", "alice", "alice@example.com", "admin", nil, nil, now, now).
		AddRow("uid2", "bob", "bob@example.com", "viewer", nil, nil, now, now)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM users ORDER BY username`)).
		WillReturnRows(rows)
	users, err := s.ListUsers(context.Background())
	if err != nil || len(users) != 2 {
		t.Fatalf("err=%v len=%d", err, len(users))
	}
}

func TestListUsers_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM users ORDER BY username`)).
		WillReturnError(errors.New("db error"))
	_, err := s.ListUsers(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateUserRole_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE users SET role`)).
		WithArgs("uid1", "admin").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := s.UpdateUserRole(context.Background(), "uid1", "admin"); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateUserRole_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE users SET role`)).
		WithArgs("nope", "admin").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	err := s.UpdateUserRole(context.Background(), "nope", "admin")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateUserRole_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE users SET role`)).
		WithArgs(anyArg, anyArg).
		WillReturnError(errors.New("db error"))
	if err := s.UpdateUserRole(context.Background(), "uid1", "admin"); err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteUser_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM users WHERE id`)).
		WithArgs("uid1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	if err := s.DeleteUser(context.Background(), "uid1"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM users WHERE id`)).
		WithArgs("nope").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))
	if !errors.Is(s.DeleteUser(context.Background(), "nope"), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

func TestCountAdminUsers_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM users WHERE role = 'admin'`)).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(3)))
	n, err := s.CountAdminUsers(context.Background())
	if err != nil || n != 3 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}

func TestCountAdminUsers_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(*) FROM users WHERE role = 'admin'`)).
		WillReturnError(errors.New("db error"))
	_, err := s.CountAdminUsers(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Agents
// ---------------------------------------------------------------------------

func TestUpsertAgent_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO agents`)).
		WithArgs(anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg).
		WillReturnRows(agentRow("aid1", "agent-01"))
	a, err := s.UpsertAgent(context.Background(), UpsertAgentParams{
		Name: "agent-01", Hostname: "host1", IPAddress: "10.0.0.1", Tags: []string{"gpu"},
	})
	if err != nil || a.ID != "aid1" {
		t.Fatalf("err=%v a=%v", err, a)
	}
}

func TestUpsertAgent_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO agents`)).
		WillReturnError(errors.New("db error"))
	_, err := s.UpsertAgent(context.Background(), UpsertAgentParams{Name: "agent-01"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetAgentByID_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM agents WHERE id`)).
		WithArgs("aid1").
		WillReturnRows(agentRow("aid1", "agent-01"))
	a, err := s.GetAgentByID(context.Background(), "aid1")
	if err != nil || a.ID != "aid1" {
		t.Fatalf("err=%v a=%v", err, a)
	}
}

func TestGetAgentByID_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM agents WHERE id`)).
		WithArgs("nope").
		WillReturnRows(pgxmock.NewRows(agentCols()))
	_, err := s.GetAgentByID(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetAgentByName_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM agents WHERE name`)).
		WithArgs("agent-01").
		WillReturnRows(agentRow("aid1", "agent-01"))
	a, err := s.GetAgentByName(context.Background(), "agent-01")
	if err != nil || a.Name != "agent-01" {
		t.Fatalf("err=%v a=%v", err, a)
	}
}

func TestGetAgentByName_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM agents WHERE name`)).
		WithArgs("nobody").
		WillReturnRows(pgxmock.NewRows(agentCols()))
	_, err := s.GetAgentByName(context.Background(), "nobody")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListAgents_Success(t *testing.T) {
	s, mock := newMock(t)
	rows := pgxmock.NewRows(agentCols()).
		AddRow("aid1", "agent-01", "host1", "192.168.1.1", "idle", []string{},
			"", "", false, "1.0", "Win", int32(4), int64(8192),
			false, false, false, 0, nil, nil, false, now, now)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM agents ORDER BY name`)).
		WillReturnRows(rows)
	agents, err := s.ListAgents(context.Background())
	if err != nil || len(agents) != 1 {
		t.Fatalf("err=%v len=%d", err, len(agents))
	}
}

func TestListAgents_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM agents ORDER BY name`)).
		WillReturnError(errors.New("db error"))
	_, err := s.ListAgents(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateAgentStatus_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE agents SET status`)).
		WithArgs("aid1", "busy").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := s.UpdateAgentStatus(context.Background(), "aid1", "busy"); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateAgentStatus_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE agents SET status`)).
		WithArgs("nope", "busy").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	if !errors.Is(s.UpdateAgentStatus(context.Background(), "nope", "busy"), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

func TestUpdateAgentHeartbeat_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE agents SET status`)).
		WithArgs("aid1", "idle").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	err := s.UpdateAgentHeartbeat(context.Background(), UpdateAgentHeartbeatParams{ID: "aid1", Status: "idle"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateAgentHeartbeat_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE agents SET status`)).
		WithArgs(anyArg, anyArg).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	err := s.UpdateAgentHeartbeat(context.Background(), UpdateAgentHeartbeatParams{ID: "nope", Status: "idle"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSetAgentAPIKey_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE agents SET api_key_hash`)).
		WithArgs("aid1", "hashvalue").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := s.SetAgentAPIKey(context.Background(), "aid1", "hashvalue"); err != nil {
		t.Fatal(err)
	}
}

func TestSetAgentAPIKey_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE agents SET api_key_hash`)).
		WillReturnError(errors.New("db error"))
	if err := s.SetAgentAPIKey(context.Background(), "aid1", "hash"); err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateAgentVNCPort_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE agents SET vnc_port`)).
		WithArgs("aid1", 5900).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := s.UpdateAgentVNCPort(context.Background(), "aid1", 5900); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateAgentVNCPort_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE agents SET vnc_port`)).
		WithArgs(anyArg, anyArg).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	if !errors.Is(s.UpdateAgentVNCPort(context.Background(), "nope", 5900), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

func TestMarkStaleAgents_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE agents SET status = 'offline'`)).
		WithArgs(anyArg).
		WillReturnResult(pgxmock.NewResult("UPDATE", 2))
	n, err := s.MarkStaleAgents(context.Background(), 5*time.Minute)
	if err != nil || n != 2 {
		t.Fatalf("err=%v n=%d", err, n)
	}
}

func TestMarkStaleAgents_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE agents SET status = 'offline'`)).
		WillReturnError(errors.New("db error"))
	_, err := s.MarkStaleAgents(context.Background(), 5*time.Minute)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Sources
// ---------------------------------------------------------------------------

func TestCreateSource_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO sources`)).
		WithArgs(anyArg, anyArg, anyArg, anyArg, anyArg).
		WillReturnRows(sourceRow("src1", "movie.mkv"))
	src, err := s.CreateSource(context.Background(), CreateSourceParams{
		Filename: "movie.mkv", UNCPath: `\\nas\movie.mkv`, SizeBytes: 1024,
	})
	if err != nil || src.ID != "src1" {
		t.Fatalf("err=%v src=%v", err, src)
	}
}

func TestCreateSource_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO sources`)).
		WillReturnError(errors.New("db error"))
	_, err := s.CreateSource(context.Background(), CreateSourceParams{Filename: "movie.mkv"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetSourceByID_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM sources WHERE id`)).
		WithArgs("src1").
		WillReturnRows(sourceRow("src1", "movie.mkv"))
	src, err := s.GetSourceByID(context.Background(), "src1")
	if err != nil || src.ID != "src1" {
		t.Fatalf("err=%v src=%v", err, src)
	}
}

func TestGetSourceByID_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM sources WHERE id`)).
		WithArgs("nope").
		WillReturnRows(pgxmock.NewRows(sourceCols()))
	_, err := s.GetSourceByID(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetSourceByUNCPath_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM sources WHERE unc_path`)).
		WithArgs(`\\nas\movie.mkv`).
		WillReturnRows(sourceRow("src1", "movie.mkv"))
	src, err := s.GetSourceByUNCPath(context.Background(), `\\nas\movie.mkv`)
	if err != nil || src.ID != "src1" {
		t.Fatalf("err=%v src=%v", err, src)
	}
}

func TestGetSourceByUNCPath_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM sources WHERE unc_path`)).
		WithArgs(anyArg).
		WillReturnRows(pgxmock.NewRows(sourceCols()))
	_, err := s.GetSourceByUNCPath(context.Background(), `\\nas\missing.mkv`)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListSources_NoFilter(t *testing.T) {
	s, mock := newMock(t)
	// count query
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM sources`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(2)))
	// list query — last arg is pageSize limit
	rows := pgxmock.NewRows(sourceCols()).
		AddRow("src1", "a.mkv", `\\nas\a.mkv`, int64(1024), nil, "new", nil, nil, "", 0, now, now).
		AddRow("src2", "b.mkv", `\\nas\b.mkv`, int64(2048), nil, "new", nil, nil, "", 0, now, now)
	mock.ExpectQuery(`SELECT id`).WithArgs(anyArg).WillReturnRows(rows)

	srcs, total, err := s.ListSources(context.Background(), ListSourcesFilter{})
	if err != nil || total != 2 || len(srcs) != 2 {
		t.Fatalf("err=%v total=%d len=%d", err, total, len(srcs))
	}
}

func TestListSources_WithState(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM sources`).
		WithArgs("new").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(1)))
	// args: state, pageSize
	mock.ExpectQuery(`SELECT id`).
		WithArgs("new", anyArg).
		WillReturnRows(sourceRow("src1", "a.mkv"))
	srcs, total, err := s.ListSources(context.Background(), ListSourcesFilter{State: "new"})
	if err != nil || total != 1 || len(srcs) != 1 {
		t.Fatalf("err=%v total=%d len=%d", err, total, len(srcs))
	}
}

func TestListSources_CountError(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM sources`).
		WillReturnError(errors.New("db error"))
	_, _, err := s.ListSources(context.Background(), ListSourcesFilter{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateSourceState_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE sources SET state`)).
		WithArgs("src1", "processing").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := s.UpdateSourceState(context.Background(), "src1", "processing"); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateSourceState_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE sources SET state`)).
		WithArgs(anyArg, anyArg).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	if !errors.Is(s.UpdateSourceState(context.Background(), "nope", "processing"), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

func TestUpdateSourceVMAF_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE sources SET vmaf_score`)).
		WithArgs("src1", float64(95.5)).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := s.UpdateSourceVMAF(context.Background(), "src1", 95.5); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateSourceVMAF_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE sources SET vmaf_score`)).
		WillReturnError(errors.New("db error"))
	if err := s.UpdateSourceVMAF(context.Background(), "src1", 95.5); err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateSourceHDR_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE sources SET hdr_type`)).
		WithArgs("src1", "hdr10", 0).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	err := s.UpdateSourceHDR(context.Background(), UpdateSourceHDRParams{ID: "src1", HDRType: "hdr10"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateSourceHDR_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE sources SET hdr_type`)).
		WithArgs(anyArg, anyArg, anyArg).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	if !errors.Is(s.UpdateSourceHDR(context.Background(), UpdateSourceHDRParams{ID: "nope"}), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

func TestDeleteSource_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM sources WHERE id`)).
		WithArgs("src1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	if err := s.DeleteSource(context.Background(), "src1"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteSource_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM sources WHERE id`)).
		WithArgs("nope").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))
	if !errors.Is(s.DeleteSource(context.Background(), "nope"), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

// ---------------------------------------------------------------------------
// Jobs
// ---------------------------------------------------------------------------

func TestCreateJob_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`WITH ins AS`).
		WithArgs(anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg).
		WillReturnRows(jobRow("jid1", "src1"))
	job, err := s.CreateJob(context.Background(), CreateJobParams{
		SourceID: "src1", JobType: "encode", Priority: 0, TargetTags: []string{},
	})
	if err != nil || job.ID != "jid1" {
		t.Fatalf("err=%v job=%v", err, job)
	}
}

func TestCreateJob_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`WITH ins AS`).
		WillReturnError(errors.New("db error"))
	_, err := s.CreateJob(context.Background(), CreateJobParams{SourceID: "src1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetJobByID_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM jobs j LEFT JOIN sources`)).
		WithArgs("jid1").
		WillReturnRows(jobRow("jid1", "src1"))
	job, err := s.GetJobByID(context.Background(), "jid1")
	if err != nil || job.ID != "jid1" {
		t.Fatalf("err=%v job=%v", err, job)
	}
}

func TestGetJobByID_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM jobs j LEFT JOIN sources`)).
		WithArgs("nope").
		WillReturnRows(pgxmock.NewRows(jobCols()))
	_, err := s.GetJobByID(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListJobs_NoFilter(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM jobs`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT j\.id`).
		WithArgs(anyArg).
		WillReturnRows(jobRow("jid1", "src1"))
	jobs, total, err := s.ListJobs(context.Background(), ListJobsFilter{})
	if err != nil || total != 1 || len(jobs) != 1 {
		t.Fatalf("err=%v total=%d len=%d", err, total, len(jobs))
	}
}

func TestListJobs_WithStatus(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM jobs`).
		WithArgs("queued").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(1)))
	// args: status, pageSize
	mock.ExpectQuery(`SELECT j\.id`).
		WithArgs("queued", anyArg).
		WillReturnRows(jobRow("jid1", "src1"))
	jobs, total, err := s.ListJobs(context.Background(), ListJobsFilter{Status: "queued"})
	if err != nil || total != 1 || len(jobs) != 1 {
		t.Fatalf("err=%v total=%d len=%d", err, total, len(jobs))
	}
}

func TestListJobs_CountError(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM jobs`).
		WillReturnError(errors.New("db error"))
	_, _, err := s.ListJobs(context.Background(), ListJobsFilter{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetJobsNeedingExpansion_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`WHERE j\.status = 'queued' AND j\.tasks_total = 0`).
		WillReturnRows(jobRow("jid1", "src1"))
	jobs, err := s.GetJobsNeedingExpansion(context.Background())
	if err != nil || len(jobs) != 1 {
		t.Fatalf("err=%v len=%d", err, len(jobs))
	}
}

func TestGetJobsNeedingExpansion_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`WHERE j\.status = 'queued' AND j\.tasks_total = 0`).
		WillReturnError(errors.New("db error"))
	_, err := s.GetJobsNeedingExpansion(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateJobStatus_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`UPDATE jobs SET status`).
		WithArgs("jid1", "running").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := s.UpdateJobStatus(context.Background(), "jid1", "running"); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateJobStatus_Completed(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`UPDATE jobs SET status`).
		WithArgs("jid1", "completed").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := s.UpdateJobStatus(context.Background(), "jid1", "completed"); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateJobStatus_Failed(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`UPDATE jobs SET status`).
		WithArgs("jid1", "failed").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := s.UpdateJobStatus(context.Background(), "jid1", "failed"); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateJobStatus_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`UPDATE jobs SET status`).
		WithArgs(anyArg, anyArg).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	if !errors.Is(s.UpdateJobStatus(context.Background(), "nope", "running"), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

func TestUpdateJobTaskCounts_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`UPDATE jobs SET`).
		WithArgs("jid1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := s.UpdateJobTaskCounts(context.Background(), "jid1"); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateJobTaskCounts_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`UPDATE jobs SET`).
		WillReturnError(errors.New("db error"))
	if err := s.UpdateJobTaskCounts(context.Background(), "jid1"); err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Tasks
// ---------------------------------------------------------------------------

func TestCreateTask_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO tasks`)).
		WithArgs(anyArg, anyArg, anyArg, anyArg, anyArg, anyArg).
		WillReturnRows(taskRow("tid1", "jid1"))
	task, err := s.CreateTask(context.Background(), CreateTaskParams{
		JobID: "jid1", ChunkIndex: 0, SourcePath: `\\nas\src.mkv`, OutputPath: `\\out\0.mkv`,
		Variables: map[string]string{"k": "v"},
	})
	if err != nil || task.ID != "tid1" {
		t.Fatalf("err=%v task=%v", err, task)
	}
}

func TestCreateTask_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO tasks`)).
		WillReturnError(errors.New("db error"))
	_, err := s.CreateTask(context.Background(), CreateTaskParams{JobID: "jid1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetTaskByID_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM tasks WHERE id`)).
		WithArgs("tid1").
		WillReturnRows(taskRow("tid1", "jid1"))
	task, err := s.GetTaskByID(context.Background(), "tid1")
	if err != nil || task.ID != "tid1" {
		t.Fatalf("err=%v task=%v", err, task)
	}
}

func TestGetTaskByID_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM tasks WHERE id`)).
		WithArgs("nope").
		WillReturnRows(pgxmock.NewRows(taskCols()))
	_, err := s.GetTaskByID(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListTasksByJob_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM tasks WHERE job_id`)).
		WithArgs("jid1").
		WillReturnRows(taskRow("tid1", "jid1"))
	tasks, err := s.ListTasksByJob(context.Background(), "jid1")
	if err != nil || len(tasks) != 1 {
		t.Fatalf("err=%v len=%d", err, len(tasks))
	}
}

func TestListTasksByJob_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM tasks WHERE job_id`)).
		WillReturnError(errors.New("db error"))
	_, err := s.ListTasksByJob(context.Background(), "jid1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClaimNextTask_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`WITH next_task AS`).
		WithArgs("aid1", anyArg).
		WillReturnRows(taskRow("tid1", "jid1"))
	task, err := s.ClaimNextTask(context.Background(), "aid1", []string{"gpu"})
	if err != nil || task == nil || task.ID != "tid1" {
		t.Fatalf("err=%v task=%v", err, task)
	}
}

func TestClaimNextTask_NoWork(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`WITH next_task AS`).
		WithArgs("aid1", anyArg).
		WillReturnRows(pgxmock.NewRows(taskCols()))
	task, err := s.ClaimNextTask(context.Background(), "aid1", []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task != nil {
		t.Fatal("expected nil task when queue empty")
	}
}

func TestClaimNextTask_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`WITH next_task AS`).
		WillReturnError(errors.New("db error"))
	_, err := s.ClaimNextTask(context.Background(), "aid1", []string{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateTaskStatus_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE tasks SET status`)).
		WithArgs("tid1", "running").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := s.UpdateTaskStatus(context.Background(), "tid1", "running"); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateTaskStatus_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE tasks SET status`)).
		WithArgs(anyArg, anyArg).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	if !errors.Is(s.UpdateTaskStatus(context.Background(), "nope", "running"), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

func TestSetTaskScriptDir_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE tasks SET script_dir`)).
		WithArgs("tid1", `/scripts/task1`).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := s.SetTaskScriptDir(context.Background(), "tid1", `/scripts/task1`); err != nil {
		t.Fatal(err)
	}
}

func TestSetTaskScriptDir_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE tasks SET script_dir`)).
		WithArgs(anyArg, anyArg).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	if !errors.Is(s.SetTaskScriptDir(context.Background(), "nope", "/scripts"), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

func TestCompleteTask_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`UPDATE tasks SET`).
		WithArgs("tid1", anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	err := s.CompleteTask(context.Background(), CompleteTaskParams{
		ID: "tid1", ExitCode: 0, FramesEncoded: 1000, AvgFPS: 30.5,
		OutputSize: 500000, DurationSec: 33,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompleteTask_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`UPDATE tasks SET`).
		WillReturnError(errors.New("db error"))
	if err := s.CompleteTask(context.Background(), CompleteTaskParams{ID: "tid1"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestFailTask_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`UPDATE tasks SET status = 'failed'`).
		WithArgs("tid1", 1, "encoding failed").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := s.FailTask(context.Background(), "tid1", 1, "encoding failed"); err != nil {
		t.Fatal(err)
	}
}

func TestFailTask_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`UPDATE tasks SET status = 'failed'`).
		WillReturnError(errors.New("db error"))
	if err := s.FailTask(context.Background(), "tid1", 1, "err"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCancelPendingTasksForJob_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`UPDATE tasks SET status = 'cancelled'`).
		WithArgs("jid1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 3))
	if err := s.CancelPendingTasksForJob(context.Background(), "jid1"); err != nil {
		t.Fatal(err)
	}
}

func TestCancelPendingTasksForJob_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`UPDATE tasks SET status = 'cancelled'`).
		WillReturnError(errors.New("db error"))
	if err := s.CancelPendingTasksForJob(context.Background(), "jid1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteTasksByJobID_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM tasks WHERE job_id`)).
		WithArgs("jid1").
		WillReturnResult(pgxmock.NewResult("DELETE", 5))
	if err := s.DeleteTasksByJobID(context.Background(), "jid1"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteTasksByJobID_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM tasks WHERE job_id`)).
		WillReturnError(errors.New("db error"))
	if err := s.DeleteTasksByJobID(context.Background(), "jid1"); err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Task Logs
// ---------------------------------------------------------------------------

func TestInsertTaskLog_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO task_logs`)).
		WithArgs(anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	err := s.InsertTaskLog(context.Background(), InsertTaskLogParams{
		TaskID: "tid1", JobID: "jid1", Stream: "stdout", Level: "info",
		Message: "frame 1", Metadata: map[string]any{"fps": 30},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInsertTaskLog_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO task_logs`)).
		WillReturnError(errors.New("db error"))
	err := s.InsertTaskLog(context.Background(), InsertTaskLogParams{
		TaskID: "tid1", JobID: "jid1", Stream: "stdout", Level: "info", Message: "msg",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListTaskLogs_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`SELECT id, task_id, job_id, stream, level, message, metadata, logged_at`).
		WithArgs("tid1", anyArg).
		WillReturnRows(taskLogRow(1, "tid1", "jid1"))
	logs, err := s.ListTaskLogs(context.Background(), ListTaskLogsParams{TaskID: "tid1"})
	if err != nil || len(logs) != 1 {
		t.Fatalf("err=%v len=%d", err, len(logs))
	}
}

func TestListTaskLogs_WithStream(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`SELECT id, task_id, job_id, stream`).
		WithArgs("tid1", "stdout", anyArg).
		WillReturnRows(taskLogRow(1, "tid1", "jid1"))
	logs, err := s.ListTaskLogs(context.Background(), ListTaskLogsParams{TaskID: "tid1", Stream: "stdout"})
	if err != nil || len(logs) != 1 {
		t.Fatalf("err=%v len=%d", err, len(logs))
	}
}

func TestListTaskLogs_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`SELECT id, task_id, job_id, stream`).
		WillReturnError(errors.New("db error"))
	_, err := s.ListTaskLogs(context.Background(), ListTaskLogsParams{TaskID: "tid1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTailTaskLogs_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM task_logs WHERE task_id`).
		WithArgs("tid1", int64(0)).
		WillReturnRows(taskLogRow(1, "tid1", "jid1"))
	logs, err := s.TailTaskLogs(context.Background(), "tid1", 0)
	if err != nil || len(logs) != 1 {
		t.Fatalf("err=%v len=%d", err, len(logs))
	}
}

func TestTailTaskLogs_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM task_logs WHERE task_id`).
		WillReturnError(errors.New("db error"))
	_, err := s.TailTaskLogs(context.Background(), "tid1", 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

func TestCreateTemplate_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO templates`)).
		WithArgs(anyArg, anyArg, anyArg, anyArg, anyArg).
		WillReturnRows(templateRow("tmpl1"))
	tmpl, err := s.CreateTemplate(context.Background(), CreateTemplateParams{
		Name: "tmpl1", Type: "encode", Content: "::body",
	})
	if err != nil || tmpl.ID != "tmpl1" {
		t.Fatalf("err=%v tmpl=%v", err, tmpl)
	}
}

func TestCreateTemplate_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO templates`)).
		WillReturnError(errors.New("db error"))
	_, err := s.CreateTemplate(context.Background(), CreateTemplateParams{Name: "t"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetTemplateByID_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM templates WHERE id`)).
		WithArgs("tmpl1").
		WillReturnRows(templateRow("tmpl1"))
	tmpl, err := s.GetTemplateByID(context.Background(), "tmpl1")
	if err != nil || tmpl.ID != "tmpl1" {
		t.Fatalf("err=%v tmpl=%v", err, tmpl)
	}
}

func TestGetTemplateByID_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM templates WHERE id`)).
		WithArgs("nope").
		WillReturnRows(pgxmock.NewRows(templateCols()))
	_, err := s.GetTemplateByID(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListTemplates_All(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM templates ORDER BY name`).
		WillReturnRows(templateRow("tmpl1"))
	tmpls, err := s.ListTemplates(context.Background(), "")
	if err != nil || len(tmpls) != 1 {
		t.Fatalf("err=%v len=%d", err, len(tmpls))
	}
}

func TestListTemplates_ByType(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM templates WHERE type`).
		WithArgs("encode").
		WillReturnRows(templateRow("tmpl1"))
	tmpls, err := s.ListTemplates(context.Background(), "encode")
	if err != nil || len(tmpls) != 1 {
		t.Fatalf("err=%v len=%d", err, len(tmpls))
	}
}

func TestListTemplates_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM templates`).
		WillReturnError(errors.New("db error"))
	_, err := s.ListTemplates(context.Background(), "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateTemplate_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE templates SET name`)).
		WithArgs("tmpl1", "new-name", "desc", "new content").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	err := s.UpdateTemplate(context.Background(), UpdateTemplateParams{
		ID: "tmpl1", Name: "new-name", Description: "desc", Content: "new content",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateTemplate_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE templates SET name`)).
		WithArgs(anyArg, anyArg, anyArg, anyArg).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	if !errors.Is(s.UpdateTemplate(context.Background(), UpdateTemplateParams{ID: "nope"}), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

func TestDeleteTemplate_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM templates WHERE id`)).
		WithArgs("tmpl1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	if err := s.DeleteTemplate(context.Background(), "tmpl1"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteTemplate_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM templates WHERE id`)).
		WithArgs("nope").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))
	if !errors.Is(s.DeleteTemplate(context.Background(), "nope"), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

// ---------------------------------------------------------------------------
// Variables
// ---------------------------------------------------------------------------

func TestUpsertVariable_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO variables`)).
		WithArgs(anyArg, anyArg, anyArg, anyArg).
		WillReturnRows(variableRow("vid1"))
	v, err := s.UpsertVariable(context.Background(), UpsertVariableParams{
		Name: "myvar", Value: "myval", Category: "general",
	})
	if err != nil || v.ID != "vid1" {
		t.Fatalf("err=%v v=%v", err, v)
	}
}

func TestUpsertVariable_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO variables`)).
		WillReturnError(errors.New("db error"))
	_, err := s.UpsertVariable(context.Background(), UpsertVariableParams{Name: "myvar"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetVariableByName_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM variables WHERE name`)).
		WithArgs("myvar").
		WillReturnRows(variableRow("vid1"))
	v, err := s.GetVariableByName(context.Background(), "myvar")
	if err != nil || v.Name != "myvar" {
		t.Fatalf("err=%v v=%v", err, v)
	}
}

func TestGetVariableByName_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM variables WHERE name`)).
		WithArgs("nope").
		WillReturnRows(pgxmock.NewRows(variableCols()))
	_, err := s.GetVariableByName(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListVariables_All(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM variables ORDER BY`).
		WillReturnRows(variableRow("vid1"))
	vars, err := s.ListVariables(context.Background(), "")
	if err != nil || len(vars) != 1 {
		t.Fatalf("err=%v len=%d", err, len(vars))
	}
}

func TestListVariables_ByCategory(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM variables WHERE category`).
		WithArgs("general").
		WillReturnRows(variableRow("vid1"))
	vars, err := s.ListVariables(context.Background(), "general")
	if err != nil || len(vars) != 1 {
		t.Fatalf("err=%v len=%d", err, len(vars))
	}
}

func TestListVariables_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM variables`).
		WillReturnError(errors.New("db error"))
	_, err := s.ListVariables(context.Background(), "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteVariable_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM variables WHERE id`)).
		WithArgs("vid1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	if err := s.DeleteVariable(context.Background(), "vid1"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteVariable_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM variables WHERE id`)).
		WithArgs("nope").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))
	if !errors.Is(s.DeleteVariable(context.Background(), "nope"), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

// ---------------------------------------------------------------------------
// Webhooks
// ---------------------------------------------------------------------------

func TestCreateWebhook_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO webhooks`)).
		WithArgs(anyArg, anyArg, anyArg, anyArg, anyArg).
		WillReturnRows(webhookRow("wid1"))
	wh, err := s.CreateWebhook(context.Background(), CreateWebhookParams{
		Name: "hook1", Provider: "discord", URL: "https://discord.com/hook", Events: []string{"job.completed"},
	})
	if err != nil || wh.ID != "wid1" {
		t.Fatalf("err=%v wh=%v", err, wh)
	}
}

func TestCreateWebhook_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO webhooks`)).
		WillReturnError(errors.New("db error"))
	_, err := s.CreateWebhook(context.Background(), CreateWebhookParams{Name: "hook1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetWebhookByID_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM webhooks WHERE id`)).
		WithArgs("wid1").
		WillReturnRows(webhookRow("wid1"))
	wh, err := s.GetWebhookByID(context.Background(), "wid1")
	if err != nil || wh.ID != "wid1" {
		t.Fatalf("err=%v wh=%v", err, wh)
	}
}

func TestGetWebhookByID_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM webhooks WHERE id`)).
		WithArgs("nope").
		WillReturnRows(pgxmock.NewRows(webhookCols()))
	_, err := s.GetWebhookByID(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListWebhooks_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM webhooks ORDER BY name`).
		WillReturnRows(webhookRow("wid1"))
	whs, err := s.ListWebhooks(context.Background())
	if err != nil || len(whs) != 1 {
		t.Fatalf("err=%v len=%d", err, len(whs))
	}
}

func TestListWebhooks_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM webhooks ORDER BY name`).
		WillReturnError(errors.New("db error"))
	_, err := s.ListWebhooks(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListWebhooksByEvent_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM webhooks WHERE events`).
		WithArgs(anyArg).
		WillReturnRows(webhookRow("wid1"))
	whs, err := s.ListWebhooksByEvent(context.Background(), "job.completed")
	if err != nil || len(whs) != 1 {
		t.Fatalf("err=%v len=%d", err, len(whs))
	}
}

func TestListWebhooksByEvent_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM webhooks WHERE events`).
		WillReturnError(errors.New("db error"))
	_, err := s.ListWebhooksByEvent(context.Background(), "job.completed")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateWebhook_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE webhooks SET name`)).
		WithArgs("wid1", "new-name", "https://example.com", anyArg, true).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	err := s.UpdateWebhook(context.Background(), UpdateWebhookParams{
		ID: "wid1", Name: "new-name", URL: "https://example.com",
		Events: []string{"job.completed"}, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateWebhook_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE webhooks SET name`)).
		WithArgs(anyArg, anyArg, anyArg, anyArg, anyArg).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	if !errors.Is(s.UpdateWebhook(context.Background(), UpdateWebhookParams{ID: "nope"}), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

func TestDeleteWebhook_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM webhooks WHERE id`)).
		WithArgs("wid1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	if err := s.DeleteWebhook(context.Background(), "wid1"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteWebhook_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM webhooks WHERE id`)).
		WithArgs("nope").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))
	if !errors.Is(s.DeleteWebhook(context.Background(), "nope"), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

func TestInsertWebhookDelivery_Success(t *testing.T) {
	s, mock := newMock(t)
	code := 200
	var errMsg *string // typed nil
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO webhook_deliveries`)).
		WithArgs("wid1", "job.completed", anyArg, &code, true, 1, errMsg).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	err := s.InsertWebhookDelivery(context.Background(), InsertWebhookDeliveryParams{
		WebhookID: "wid1", Event: "job.completed", Payload: []byte(`{}`),
		ResponseCode: &code, Success: true, Attempt: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInsertWebhookDelivery_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO webhook_deliveries`)).
		WillReturnError(errors.New("db error"))
	if err := s.InsertWebhookDelivery(context.Background(), InsertWebhookDeliveryParams{
		WebhookID: "wid1", Event: "job.completed",
	}); err == nil {
		t.Fatal("expected error")
	}
}

func TestListWebhookDeliveries_Success(t *testing.T) {
	s, mock := newMock(t)
	code := 200
	rows := pgxmock.NewRows([]string{"id", "webhook_id", "event", "response_code", "success", "attempt", "error_msg", "delivered_at"}).
		AddRow(int64(1), "wid1", "job.completed", &code, true, 1, nil, now)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM webhook_deliveries WHERE webhook_id`)).
		WithArgs("wid1", 50, 0).
		WillReturnRows(rows)
	dels, err := s.ListWebhookDeliveries(context.Background(), "wid1", 50, 0)
	if err != nil || len(dels) != 1 {
		t.Fatalf("err=%v len=%d", err, len(dels))
	}
}

func TestListWebhookDeliveries_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM webhook_deliveries WHERE webhook_id`)).
		WillReturnError(errors.New("db error"))
	_, err := s.ListWebhookDeliveries(context.Background(), "wid1", 10, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Analysis Results
// ---------------------------------------------------------------------------

func TestUpsertAnalysisResult_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO analysis_results`)).
		WithArgs("src1", "histogram", anyArg, anyArg).
		WillReturnRows(analysisResultRow("arid1", "src1"))
	r, err := s.UpsertAnalysisResult(context.Background(), UpsertAnalysisResultParams{
		SourceID: "src1", Type: "histogram",
		FrameData: []byte("data"), Summary: []byte(`{}`),
	})
	if err != nil || r.ID != "arid1" {
		t.Fatalf("err=%v r=%v", err, r)
	}
}

func TestUpsertAnalysisResult_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO analysis_results`)).
		WillReturnError(errors.New("db error"))
	_, err := s.UpsertAnalysisResult(context.Background(), UpsertAnalysisResultParams{SourceID: "src1", Type: "histogram"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetAnalysisResult_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM analysis_results WHERE source_id`)).
		WithArgs("src1", "histogram").
		WillReturnRows(analysisResultRow("arid1", "src1"))
	r, err := s.GetAnalysisResult(context.Background(), "src1", "histogram")
	if err != nil || r.ID != "arid1" {
		t.Fatalf("err=%v r=%v", err, r)
	}
}

func TestGetAnalysisResult_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM analysis_results WHERE source_id`)).
		WithArgs("nope", "histogram").
		WillReturnRows(pgxmock.NewRows(analysisResultCols()))
	_, err := s.GetAnalysisResult(context.Background(), "nope", "histogram")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListAnalysisResults_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM analysis_results WHERE source_id`).
		WithArgs("src1").
		WillReturnRows(analysisResultRow("arid1", "src1"))
	rs, err := s.ListAnalysisResults(context.Background(), "src1")
	if err != nil || len(rs) != 1 {
		t.Fatalf("err=%v len=%d", err, len(rs))
	}
}

func TestListAnalysisResults_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM analysis_results WHERE source_id`).
		WillReturnError(errors.New("db error"))
	_, err := s.ListAnalysisResults(context.Background(), "src1")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Path Mappings
// ---------------------------------------------------------------------------

func TestCreatePathMapping_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO path_mappings`)).
		WithArgs("nas-map", `\\NAS01\media`, "/mnt/nas/media").
		WillReturnRows(pathMappingRow("pmid1"))
	pm, err := s.CreatePathMapping(context.Background(), CreatePathMappingParams{
		Name: "nas-map", WindowsPrefix: `\\NAS01\media`, LinuxPrefix: "/mnt/nas/media",
	})
	if err != nil || pm.ID != "pmid1" {
		t.Fatalf("err=%v pm=%v", err, pm)
	}
}

func TestCreatePathMapping_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO path_mappings`)).
		WillReturnError(errors.New("db error"))
	_, err := s.CreatePathMapping(context.Background(), CreatePathMappingParams{Name: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetPathMappingByID_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM path_mappings WHERE id`)).
		WithArgs("pmid1").
		WillReturnRows(pathMappingRow("pmid1"))
	pm, err := s.GetPathMappingByID(context.Background(), "pmid1")
	if err != nil || pm.ID != "pmid1" {
		t.Fatalf("err=%v pm=%v", err, pm)
	}
}

func TestGetPathMappingByID_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM path_mappings WHERE id`)).
		WithArgs("nope").
		WillReturnRows(pgxmock.NewRows(pathMappingCols()))
	_, err := s.GetPathMappingByID(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListPathMappings_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM path_mappings ORDER BY name`).
		WillReturnRows(pathMappingRow("pmid1"))
	pms, err := s.ListPathMappings(context.Background())
	if err != nil || len(pms) != 1 {
		t.Fatalf("err=%v len=%d", err, len(pms))
	}
}

func TestListPathMappings_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM path_mappings ORDER BY name`).
		WillReturnError(errors.New("db error"))
	_, err := s.ListPathMappings(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdatePathMapping_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`UPDATE path_mappings`).
		WithArgs("pmid1", "nas-map", anyArg, anyArg, true).
		WillReturnRows(pathMappingRow("pmid1"))
	pm, err := s.UpdatePathMapping(context.Background(), UpdatePathMappingParams{
		ID: "pmid1", Name: "nas-map", Enabled: true,
	})
	if err != nil || pm.ID != "pmid1" {
		t.Fatalf("err=%v pm=%v", err, pm)
	}
}

func TestUpdatePathMapping_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`UPDATE path_mappings`).
		WithArgs(anyArg, anyArg, anyArg, anyArg, anyArg).
		WillReturnRows(pgxmock.NewRows(pathMappingCols()))
	_, err := s.UpdatePathMapping(context.Background(), UpdatePathMappingParams{ID: "nope"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeletePathMapping_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM path_mappings WHERE id`)).
		WithArgs("pmid1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	if err := s.DeletePathMapping(context.Background(), "pmid1"); err != nil {
		t.Fatal(err)
	}
}

func TestDeletePathMapping_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM path_mappings WHERE id`)).
		WithArgs("nope").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))
	if !errors.Is(s.DeletePathMapping(context.Background(), "nope"), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

func TestCreateSession_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO sessions`)).
		WithArgs("tok1", "uid1", anyArg).
		WillReturnRows(sessionRow("tok1", "uid1"))
	sess, err := s.CreateSession(context.Background(), CreateSessionParams{
		Token: "tok1", UserID: "uid1", ExpiresAt: now.Add(24 * time.Hour),
	})
	if err != nil || sess.Token != "tok1" {
		t.Fatalf("err=%v sess=%v", err, sess)
	}
}

func TestCreateSession_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO sessions`)).
		WillReturnError(errors.New("db error"))
	_, err := s.CreateSession(context.Background(), CreateSessionParams{Token: "tok1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetSessionByToken_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM sessions WHERE token`)).
		WithArgs("tok1").
		WillReturnRows(sessionRow("tok1", "uid1"))
	sess, err := s.GetSessionByToken(context.Background(), "tok1")
	if err != nil || sess.Token != "tok1" {
		t.Fatalf("err=%v sess=%v", err, sess)
	}
}

func TestGetSessionByToken_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM sessions WHERE token`)).
		WithArgs("nope").
		WillReturnRows(pgxmock.NewRows(sessionCols()))
	_, err := s.GetSessionByToken(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteSession_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM sessions WHERE token`)).
		WithArgs("tok1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	if err := s.DeleteSession(context.Background(), "tok1"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteSession_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM sessions WHERE token`)).
		WillReturnError(errors.New("db error"))
	if err := s.DeleteSession(context.Background(), "tok1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestPruneExpiredSessions_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`DELETE FROM sessions WHERE expires_at`).
		WillReturnResult(pgxmock.NewResult("DELETE", 5))
	if err := s.PruneExpiredSessions(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestPruneExpiredSessions_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`DELETE FROM sessions WHERE expires_at`).
		WillReturnError(errors.New("db error"))
	if err := s.PruneExpiredSessions(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Enrollment Tokens
// ---------------------------------------------------------------------------

func TestCreateEnrollmentToken_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO enrollment_tokens`)).
		WithArgs("etok1", "admin", anyArg).
		WillReturnRows(enrollmentTokenRow("etid1", "etok1"))
	et, err := s.CreateEnrollmentToken(context.Background(), CreateEnrollmentTokenParams{
		Token: "etok1", CreatedBy: "admin", ExpiresAt: now.Add(24 * time.Hour),
	})
	if err != nil || et.ID != "etid1" {
		t.Fatalf("err=%v et=%v", err, et)
	}
}

func TestCreateEnrollmentToken_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO enrollment_tokens`)).
		WillReturnError(errors.New("db error"))
	_, err := s.CreateEnrollmentToken(context.Background(), CreateEnrollmentTokenParams{Token: "t"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetEnrollmentToken_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM enrollment_tokens`).
		WithArgs("etok1").
		WillReturnRows(enrollmentTokenRow("etid1", "etok1"))
	et, err := s.GetEnrollmentToken(context.Background(), "etok1")
	if err != nil || et.Token != "etok1" {
		t.Fatalf("err=%v et=%v", err, et)
	}
}

func TestGetEnrollmentToken_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM enrollment_tokens`).
		WithArgs("nope").
		WillReturnRows(pgxmock.NewRows(enrollmentTokenCols()))
	_, err := s.GetEnrollmentToken(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestConsumeEnrollmentToken_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`UPDATE enrollment_tokens`).
		WithArgs("etok1", "aid1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	err := s.ConsumeEnrollmentToken(context.Background(), ConsumeEnrollmentTokenParams{
		Token: "etok1", AgentID: "aid1",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestConsumeEnrollmentToken_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`UPDATE enrollment_tokens`).
		WithArgs(anyArg, anyArg).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	err := s.ConsumeEnrollmentToken(context.Background(), ConsumeEnrollmentTokenParams{Token: "nope", AgentID: "aid1"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListEnrollmentTokens_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM enrollment_tokens ORDER BY created_at`).
		WillReturnRows(enrollmentTokenRow("etid1", "etok1"))
	ets, err := s.ListEnrollmentTokens(context.Background())
	if err != nil || len(ets) != 1 {
		t.Fatalf("err=%v len=%d", err, len(ets))
	}
}

func TestListEnrollmentTokens_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM enrollment_tokens ORDER BY created_at`).
		WillReturnError(errors.New("db error"))
	_, err := s.ListEnrollmentTokens(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDeleteEnrollmentToken_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM enrollment_tokens WHERE id`)).
		WithArgs("etid1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	if err := s.DeleteEnrollmentToken(context.Background(), "etid1"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteEnrollmentToken_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM enrollment_tokens WHERE id`)).
		WithArgs("nope").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))
	if !errors.Is(s.DeleteEnrollmentToken(context.Background(), "nope"), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

func TestPruneExpiredEnrollmentTokens_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`DELETE FROM enrollment_tokens WHERE expires_at`).
		WillReturnResult(pgxmock.NewResult("DELETE", 2))
	if err := s.PruneExpiredEnrollmentTokens(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestPruneExpiredEnrollmentTokens_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`DELETE FROM enrollment_tokens WHERE expires_at`).
		WillReturnError(errors.New("db error"))
	if err := s.PruneExpiredEnrollmentTokens(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Extended queries
// ---------------------------------------------------------------------------

func TestRetryFailedTasksForJob_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`UPDATE tasks SET`).
		WithArgs("jid1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 3))
	if err := s.RetryFailedTasksForJob(context.Background(), "jid1"); err != nil {
		t.Fatal(err)
	}
}

func TestRetryFailedTasksForJob_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(`UPDATE tasks SET`).
		WillReturnError(errors.New("db error"))
	if err := s.RetryFailedTasksForJob(context.Background(), "jid1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestListJobLogs_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM task_logs WHERE job_id`).
		WithArgs("jid1", anyArg).
		WillReturnRows(taskLogRow(1, "tid1", "jid1"))
	logs, err := s.ListJobLogs(context.Background(), ListJobLogsParams{JobID: "jid1"})
	if err != nil || len(logs) != 1 {
		t.Fatalf("err=%v len=%d", err, len(logs))
	}
}

func TestListJobLogs_WithStream(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM task_logs WHERE job_id`).
		WithArgs("jid1", "stdout", anyArg).
		WillReturnRows(taskLogRow(1, "tid1", "jid1"))
	logs, err := s.ListJobLogs(context.Background(), ListJobLogsParams{JobID: "jid1", Stream: "stdout"})
	if err != nil || len(logs) != 1 {
		t.Fatalf("err=%v len=%d", err, len(logs))
	}
}

func TestListJobLogs_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM task_logs WHERE job_id`).
		WillReturnError(errors.New("db error"))
	_, err := s.ListJobLogs(context.Background(), ListJobLogsParams{JobID: "jid1"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPruneOldTaskLogs_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM task_logs WHERE logged_at`)).
		WithArgs(anyArg).
		WillReturnResult(pgxmock.NewResult("DELETE", 10))
	if err := s.PruneOldTaskLogs(context.Background(), now.Add(-24*time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func TestPruneOldTaskLogs_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM task_logs WHERE logged_at`)).
		WillReturnError(errors.New("db error"))
	if err := s.PruneOldTaskLogs(context.Background(), now); err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Audit Log
// ---------------------------------------------------------------------------

func TestCreateAuditEntry_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO audit_log`)).
		WithArgs(anyArg, anyArg, anyArg, anyArg, anyArg, anyArg, anyArg).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	err := s.CreateAuditEntry(context.Background(), CreateAuditEntryParams{
		Username: "alice", Action: "delete", Resource: "source",
		ResourceID: "src1", IPAddress: "10.0.0.1",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateAuditEntry_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO audit_log`)).
		WillReturnError(errors.New("db error"))
	if err := s.CreateAuditEntry(context.Background(), CreateAuditEntryParams{Username: "alice"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestListAuditLog_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_log`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int(1)))
	auditRows := pgxmock.NewRows([]string{"id", "user_id", "username", "action", "resource", "resource_id", "detail", "ip_address", "logged_at"}).
		AddRow(int64(1), nil, "alice", "delete", "source", "src1", []byte(`{}`), "10.0.0.1", now)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM audit_log ORDER BY logged_at`)).
		WithArgs(10, 0).
		WillReturnRows(auditRows)
	entries, total, err := s.ListAuditLog(context.Background(), 10, 0)
	if err != nil || total != 1 || len(entries) != 1 {
		t.Fatalf("err=%v total=%d len=%d", err, total, len(entries))
	}
}

func TestListAuditLog_CountError(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_log`).
		WillReturnError(errors.New("db error"))
	_, _, err := s.ListAuditLog(context.Background(), 10, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListAuditLog_QueryError(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM audit_log`).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int(5)))
	mock.ExpectQuery(regexp.QuoteMeta(`FROM audit_log ORDER BY logged_at`)).
		WillReturnError(errors.New("db error"))
	_, _, err := s.ListAuditLog(context.Background(), 10, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Agent Metrics
// ---------------------------------------------------------------------------

func TestInsertAgentMetric_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO agent_metrics`)).
		WithArgs("aid1", float32(25.5), float32(80.0), float32(60.0)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	err := s.InsertAgentMetric(context.Background(), InsertAgentMetricParams{
		AgentID: "aid1", CPUPct: 25.5, GPUPct: 80.0, MemPct: 60.0,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInsertAgentMetric_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO agent_metrics`)).
		WillReturnError(errors.New("db error"))
	if err := s.InsertAgentMetric(context.Background(), InsertAgentMetricParams{AgentID: "aid1"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestListAgentMetrics_Success(t *testing.T) {
	s, mock := newMock(t)
	metricRows := pgxmock.NewRows([]string{"id", "agent_id", "cpu_pct", "gpu_pct", "mem_pct", "recorded_at"}).
		AddRow(int64(1), "aid1", float32(25.5), float32(80.0), float32(60.0), now)
	mock.ExpectQuery(`FROM agent_metrics`).
		WithArgs("aid1", anyArg).
		WillReturnRows(metricRows)
	metrics, err := s.ListAgentMetrics(context.Background(), "aid1", now.Add(-time.Hour))
	if err != nil || len(metrics) != 1 {
		t.Fatalf("err=%v len=%d", err, len(metrics))
	}
}

func TestListAgentMetrics_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM agent_metrics`).
		WillReturnError(errors.New("db error"))
	_, err := s.ListAgentMetrics(context.Background(), "aid1", now.Add(-time.Hour))
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Schedules
// ---------------------------------------------------------------------------

func TestCreateSchedule_Success(t *testing.T) {
	s, mock := newMock(t)
	tmpl, _ := json.Marshal(map[string]any{"job_type": "encode"})
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO schedules`)).
		WithArgs(anyArg, anyArg, anyArg, anyArg, anyArg).
		WillReturnRows(scheduleRow("scid1"))
	sc, err := s.CreateSchedule(context.Background(), CreateScheduleParams{
		Name: "nightly", CronExpr: "0 2 * * *",
		JobTemplate: tmpl, Enabled: true,
	})
	if err != nil || sc.ID != "scid1" {
		t.Fatalf("err=%v sc=%v", err, sc)
	}
}

func TestCreateSchedule_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO schedules`)).
		WillReturnError(errors.New("db error"))
	_, err := s.CreateSchedule(context.Background(), CreateScheduleParams{Name: "nightly"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetScheduleByID_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM schedules WHERE id`)).
		WithArgs("scid1").
		WillReturnRows(scheduleRow("scid1"))
	sc, err := s.GetScheduleByID(context.Background(), "scid1")
	if err != nil || sc.ID != "scid1" {
		t.Fatalf("err=%v sc=%v", err, sc)
	}
}

func TestGetScheduleByID_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM schedules WHERE id`)).
		WithArgs("nope").
		WillReturnRows(pgxmock.NewRows(scheduleCols()))
	_, err := s.GetScheduleByID(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListSchedules_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM schedules ORDER BY name`).
		WillReturnRows(scheduleRow("scid1"))
	scs, err := s.ListSchedules(context.Background())
	if err != nil || len(scs) != 1 {
		t.Fatalf("err=%v len=%d", err, len(scs))
	}
}

func TestListSchedules_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM schedules ORDER BY name`).
		WillReturnError(errors.New("db error"))
	_, err := s.ListSchedules(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateSchedule_Success(t *testing.T) {
	s, mock := newMock(t)
	tmpl, _ := json.Marshal(map[string]any{"job_type": "encode"})
	mock.ExpectQuery(`UPDATE schedules`).
		WithArgs("scid1", "nightly", "0 2 * * *", anyArg, true, anyArg).
		WillReturnRows(scheduleRow("scid1"))
	sc, err := s.UpdateSchedule(context.Background(), UpdateScheduleParams{
		ID: "scid1", Name: "nightly", CronExpr: "0 2 * * *",
		JobTemplate: tmpl, Enabled: true,
	})
	if err != nil || sc.ID != "scid1" {
		t.Fatalf("err=%v sc=%v", err, sc)
	}
}

func TestUpdateSchedule_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`UPDATE schedules`).
		WithArgs(anyArg, anyArg, anyArg, anyArg, anyArg, anyArg).
		WillReturnRows(pgxmock.NewRows(scheduleCols()))
	_, err := s.UpdateSchedule(context.Background(), UpdateScheduleParams{ID: "nope"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteSchedule_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM schedules WHERE id`)).
		WithArgs("scid1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	if err := s.DeleteSchedule(context.Background(), "scid1"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteSchedule_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM schedules WHERE id`)).
		WithArgs("nope").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))
	if !errors.Is(s.DeleteSchedule(context.Background(), "nope"), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

func TestListDueSchedules_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`WHERE enabled = true AND next_run_at`).
		WillReturnRows(scheduleRow("scid1"))
	scs, err := s.ListDueSchedules(context.Background())
	if err != nil || len(scs) != 1 {
		t.Fatalf("err=%v len=%d", err, len(scs))
	}
}

func TestListDueSchedules_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`WHERE enabled = true AND next_run_at`).
		WillReturnError(errors.New("db error"))
	_, err := s.ListDueSchedules(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMarkScheduleRun_Success(t *testing.T) {
	s, mock := newMock(t)
	nextRun := now.Add(time.Hour)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE schedules SET last_run_at`)).
		WithArgs("scid1", anyArg, &nextRun).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	err := s.MarkScheduleRun(context.Background(), MarkScheduleRunParams{
		ID: "scid1", LastRunAt: now, NextRunAt: &nextRun,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMarkScheduleRun_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE schedules SET last_run_at`)).
		WithArgs(anyArg, anyArg, anyArg).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	if !errors.Is(s.MarkScheduleRun(context.Background(), MarkScheduleRunParams{ID: "nope"}), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}

func TestMarkScheduleRun_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE schedules SET last_run_at`)).
		WillReturnError(errors.New("db error"))
	if err := s.MarkScheduleRun(context.Background(), MarkScheduleRunParams{ID: "scid1"}); err == nil {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Flows
// ---------------------------------------------------------------------------

func flowCols() []string {
	return []string{"id", "name", "description", "graph", "created_at", "updated_at"}
}

func flowRow(id, name string) *pgxmock.Rows {
	graph, _ := json.Marshal(map[string]any{"nodes": []any{}, "edges": []any{}})
	return pgxmock.NewRows(flowCols()).
		AddRow(id, name, "test description", graph, now, now)
}

func TestCreateFlow_Success(t *testing.T) {
	s, mock := newMock(t)
	graph := json.RawMessage(`{"nodes":[],"edges":[]}`)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO flows`)).
		WithArgs(anyArg, anyArg, anyArg).
		WillReturnRows(flowRow("fid1", "my-pipeline"))
	f, err := s.CreateFlow(context.Background(), CreateFlowParams{
		Name:        "my-pipeline",
		Description: "test description",
		Graph:       graph,
	})
	if err != nil || f.ID != "fid1" {
		t.Fatalf("err=%v f=%v", err, f)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestCreateFlow_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO flows`)).
		WillReturnError(errors.New("db error"))
	_, err := s.CreateFlow(context.Background(), CreateFlowParams{Name: "my-pipeline"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetFlowByID_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM flows WHERE id`)).
		WithArgs("fid1").
		WillReturnRows(flowRow("fid1", "my-pipeline"))
	f, err := s.GetFlowByID(context.Background(), "fid1")
	if err != nil || f.ID != "fid1" {
		t.Fatalf("err=%v f=%v", err, f)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestGetFlowByID_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(regexp.QuoteMeta(`FROM flows WHERE id`)).
		WithArgs("nope").
		WillReturnRows(pgxmock.NewRows(flowCols()))
	_, err := s.GetFlowByID(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListFlows_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM flows ORDER BY updated_at`).
		WillReturnRows(flowRow("fid1", "my-pipeline"))
	flows, err := s.ListFlows(context.Background())
	if err != nil || len(flows) != 1 {
		t.Fatalf("err=%v len=%d", err, len(flows))
	}
}

func TestListFlows_Empty(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM flows ORDER BY updated_at`).
		WillReturnRows(pgxmock.NewRows(flowCols()))
	flows, err := s.ListFlows(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(flows) != 0 {
		t.Errorf("expected empty list, got %d", len(flows))
	}
}

func TestListFlows_Error(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`FROM flows ORDER BY updated_at`).
		WillReturnError(errors.New("db error"))
	_, err := s.ListFlows(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdateFlow_Success(t *testing.T) {
	s, mock := newMock(t)
	graph := json.RawMessage(`{"nodes":[],"edges":[]}`)
	mock.ExpectQuery(`UPDATE flows`).
		WithArgs("fid1", "updated-pipeline", anyArg, anyArg).
		WillReturnRows(flowRow("fid1", "updated-pipeline"))
	f, err := s.UpdateFlow(context.Background(), UpdateFlowParams{
		ID:          "fid1",
		Name:        "updated-pipeline",
		Description: "updated description",
		Graph:       graph,
	})
	if err != nil || f.ID != "fid1" {
		t.Fatalf("err=%v f=%v", err, f)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Error(err)
	}
}

func TestUpdateFlow_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectQuery(`UPDATE flows`).
		WithArgs(anyArg, anyArg, anyArg, anyArg).
		WillReturnRows(pgxmock.NewRows(flowCols()))
	_, err := s.UpdateFlow(context.Background(), UpdateFlowParams{ID: "nope", Name: "x"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteFlow_Success(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM flows WHERE id`)).
		WithArgs("fid1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	if err := s.DeleteFlow(context.Background(), "fid1"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteFlow_NotFound(t *testing.T) {
	s, mock := newMock(t)
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM flows WHERE id`)).
		WithArgs("nope").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))
	if !errors.Is(s.DeleteFlow(context.Background(), "nope"), ErrNotFound) {
		t.Fatal("expected ErrNotFound")
	}
}
