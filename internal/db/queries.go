package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ---------------------------------------------------------------------------
// Sentinel errors
// ---------------------------------------------------------------------------

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("db: not found")

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

func (s *pgStore) CreateUser(ctx context.Context, p CreateUserParams) (*User, error) {
	const q = `
		INSERT INTO users (username, email, role, password_hash, oidc_sub)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, username, email, role, password_hash, oidc_sub, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.Username, p.Email, p.Role, p.PasswordHash, p.OIDCSub)
	return scanUser(row)
}

func (s *pgStore) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	const q = `SELECT id, username, email, role, password_hash, oidc_sub, created_at, updated_at
	           FROM users WHERE username = $1`
	return scanUser(s.pool.QueryRow(ctx, q, username))
}

func (s *pgStore) GetUserByOIDCSub(ctx context.Context, sub string) (*User, error) {
	const q = `SELECT id, username, email, role, password_hash, oidc_sub, created_at, updated_at
	           FROM users WHERE oidc_sub = $1`
	return scanUser(s.pool.QueryRow(ctx, q, sub))
}

func (s *pgStore) GetUserByID(ctx context.Context, id string) (*User, error) {
	const q = `SELECT id, username, email, role, password_hash, oidc_sub, created_at, updated_at
	           FROM users WHERE id = $1`
	return scanUser(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) ListUsers(ctx context.Context) ([]*User, error) {
	const q = `SELECT id, username, email, role, password_hash, oidc_sub, created_at, updated_at
	           FROM users ORDER BY username`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db: list users: %w", err)
	}
	defer rows.Close()
	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *pgStore) UpdateUserRole(ctx context.Context, id, role string) error {
	const q = `UPDATE users SET role = $2, updated_at = now() WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id, role)
	if err != nil {
		return fmt.Errorf("db: update user role: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) DeleteUser(ctx context.Context, id string) error {
	const q = `DELETE FROM users WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("db: delete user: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.Role,
		&u.PasswordHash, &u.OIDCSub,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan user: %w", err)
	}
	return &u, nil
}

// ---------------------------------------------------------------------------
// Agents
// ---------------------------------------------------------------------------

func (s *pgStore) UpsertAgent(ctx context.Context, p UpsertAgentParams) (*Agent, error) {
	const q = `
		INSERT INTO agents
		    (name, hostname, ip_address, tags, gpu_vendor, gpu_model, gpu_enabled,
		     agent_version, os_version, cpu_count, ram_mib, nvenc, qsv, amf)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		ON CONFLICT (name) DO UPDATE SET
		    hostname      = EXCLUDED.hostname,
		    ip_address    = EXCLUDED.ip_address,
		    tags          = EXCLUDED.tags,
		    gpu_vendor    = EXCLUDED.gpu_vendor,
		    gpu_model     = EXCLUDED.gpu_model,
		    gpu_enabled   = EXCLUDED.gpu_enabled,
		    agent_version = EXCLUDED.agent_version,
		    os_version    = EXCLUDED.os_version,
		    cpu_count     = EXCLUDED.cpu_count,
		    ram_mib       = EXCLUDED.ram_mib,
		    nvenc         = EXCLUDED.nvenc,
		    qsv           = EXCLUDED.qsv,
		    amf           = EXCLUDED.amf,
		    updated_at    = now()
		RETURNING id, name, hostname, ip_address, status, tags,
		          gpu_vendor, gpu_model, gpu_enabled,
		          agent_version, os_version, cpu_count, ram_mib,
		          nvenc, qsv, amf, api_key_hash, last_heartbeat,
		          created_at, updated_at`
	row := s.pool.QueryRow(ctx, q,
		p.Name, p.Hostname, p.IPAddress, p.Tags,
		p.GPUVendor, p.GPUModel, p.GPUEnabled,
		p.AgentVersion, p.OSVersion, p.CPUCount, p.RAMMIB,
		p.NVENC, p.QSV, p.AMF,
	)
	return scanAgent(row)
}

func (s *pgStore) GetAgentByID(ctx context.Context, id string) (*Agent, error) {
	const q = `SELECT id, name, hostname, ip_address, status, tags,
	                  gpu_vendor, gpu_model, gpu_enabled,
	                  agent_version, os_version, cpu_count, ram_mib,
	                  nvenc, qsv, amf, api_key_hash, last_heartbeat,
	                  created_at, updated_at
	           FROM agents WHERE id = $1`
	return scanAgent(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) GetAgentByName(ctx context.Context, name string) (*Agent, error) {
	const q = `SELECT id, name, hostname, ip_address, status, tags,
	                  gpu_vendor, gpu_model, gpu_enabled,
	                  agent_version, os_version, cpu_count, ram_mib,
	                  nvenc, qsv, amf, api_key_hash, last_heartbeat,
	                  created_at, updated_at
	           FROM agents WHERE name = $1`
	return scanAgent(s.pool.QueryRow(ctx, q, name))
}

func (s *pgStore) ListAgents(ctx context.Context) ([]*Agent, error) {
	const q = `SELECT id, name, hostname, ip_address, status, tags,
	                  gpu_vendor, gpu_model, gpu_enabled,
	                  agent_version, os_version, cpu_count, ram_mib,
	                  nvenc, qsv, amf, api_key_hash, last_heartbeat,
	                  created_at, updated_at
	           FROM agents ORDER BY name`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db: list agents: %w", err)
	}
	defer rows.Close()
	var out []*Agent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *pgStore) UpdateAgentStatus(ctx context.Context, id, status string) error {
	const q = `UPDATE agents SET status = $2, updated_at = now() WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id, status)
	if err != nil {
		return fmt.Errorf("db: update agent status: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) UpdateAgentHeartbeat(ctx context.Context, p UpdateAgentHeartbeatParams) error {
	const q = `UPDATE agents SET status = $2, last_heartbeat = now(), updated_at = now()
	           WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, p.ID, p.Status)
	if err != nil {
		return fmt.Errorf("db: update agent heartbeat: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) SetAgentAPIKey(ctx context.Context, id, hash string) error {
	const q = `UPDATE agents SET api_key_hash = $2, updated_at = now() WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, id, hash)
	return err
}

func scanAgent(row pgx.Row) (*Agent, error) {
	var a Agent
	err := row.Scan(
		&a.ID, &a.Name, &a.Hostname, &a.IPAddress, &a.Status, &a.Tags,
		&a.GPUVendor, &a.GPUModel, &a.GPUEnabled,
		&a.AgentVersion, &a.OSVersion, &a.CPUCount, &a.RAMMIB,
		&a.NVENC, &a.QSV, &a.AMF, &a.APIKeyHash, &a.LastHeartbeat,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan agent: %w", err)
	}
	return &a, nil
}

// ---------------------------------------------------------------------------
// Sources
// ---------------------------------------------------------------------------

func (s *pgStore) CreateSource(ctx context.Context, p CreateSourceParams) (*Source, error) {
	const q = `
		INSERT INTO sources (filename, unc_path, size_bytes, detected_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, filename, unc_path, size_bytes, detected_by, state, vmaf_score,
		          created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.Filename, p.UNCPath, p.SizeBytes, p.DetectedBy)
	return scanSource(row)
}

func (s *pgStore) GetSourceByID(ctx context.Context, id string) (*Source, error) {
	const q = `SELECT id, filename, unc_path, size_bytes, detected_by, state, vmaf_score,
	                  created_at, updated_at FROM sources WHERE id = $1`
	return scanSource(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) GetSourceByUNCPath(ctx context.Context, uncPath string) (*Source, error) {
	const q = `SELECT id, filename, unc_path, size_bytes, detected_by, state, vmaf_score,
	                  created_at, updated_at FROM sources WHERE unc_path = $1`
	return scanSource(s.pool.QueryRow(ctx, q, uncPath))
}

func (s *pgStore) ListSources(ctx context.Context, f ListSourcesFilter) ([]*Source, int64, error) {
	pageSize := f.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	countQ := `SELECT COUNT(*) FROM sources`
	var total int64
	countArgs := []any{}
	if f.State != "" {
		countQ += ` WHERE state = $1`
		countArgs = append(countArgs, f.State)
	}
	if err := s.pool.QueryRow(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("db: count sources: %w", err)
	}

	q := `SELECT id, filename, unc_path, size_bytes, detected_by, state, vmaf_score,
	             created_at, updated_at FROM sources`
	args := []any{}
	argN := 1

	if f.State != "" {
		q += fmt.Sprintf(` WHERE state = $%d`, argN)
		args = append(args, f.State)
		argN++
	}
	if f.Cursor != "" {
		if f.State != "" {
			q += fmt.Sprintf(` AND id > $%d`, argN)
		} else {
			q += fmt.Sprintf(` WHERE id > $%d`, argN)
		}
		args = append(args, f.Cursor)
		argN++
	}
	q += fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d`, argN)
	args = append(args, pageSize)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("db: list sources: %w", err)
	}
	defer rows.Close()

	var out []*Source
	for rows.Next() {
		src, err := scanSource(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, src)
	}
	return out, total, rows.Err()
}

func (s *pgStore) UpdateSourceState(ctx context.Context, id, state string) error {
	const q = `UPDATE sources SET state = $2, updated_at = now() WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id, state)
	if err != nil {
		return fmt.Errorf("db: update source state: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) UpdateSourceVMAF(ctx context.Context, id string, score float64) error {
	const q = `UPDATE sources SET vmaf_score = $2, updated_at = now() WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, id, score)
	return err
}

func (s *pgStore) DeleteSource(ctx context.Context, id string) error {
	const q = `DELETE FROM sources WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("db: delete source: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanSource(row pgx.Row) (*Source, error) {
	var src Source
	err := row.Scan(
		&src.ID, &src.Filename, &src.UNCPath, &src.SizeBytes,
		&src.DetectedBy, &src.State, &src.VMafScore,
		&src.CreatedAt, &src.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan source: %w", err)
	}
	return &src, nil
}

// ---------------------------------------------------------------------------
// Jobs
// ---------------------------------------------------------------------------

func (s *pgStore) CreateJob(ctx context.Context, p CreateJobParams) (*Job, error) {
	const q = `
		INSERT INTO jobs (source_id, job_type, priority, target_tags)
		VALUES ($1, $2, $3, $4)
		RETURNING id, source_id, status, job_type, priority, target_tags,
		          tasks_total, tasks_pending, tasks_running, tasks_completed, tasks_failed,
		          completed_at, failed_at, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.SourceID, p.JobType, p.Priority, p.TargetTags)
	return scanJob(row)
}

func (s *pgStore) GetJobByID(ctx context.Context, id string) (*Job, error) {
	const q = `SELECT id, source_id, status, job_type, priority, target_tags,
	                  tasks_total, tasks_pending, tasks_running, tasks_completed, tasks_failed,
	                  completed_at, failed_at, created_at, updated_at
	           FROM jobs WHERE id = $1`
	return scanJob(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) ListJobs(ctx context.Context, f ListJobsFilter) ([]*Job, int64, error) {
	pageSize := f.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	countQ := `SELECT COUNT(*) FROM jobs`
	var total int64
	countArgs := []any{}
	if f.Status != "" {
		countQ += ` WHERE status = $1`
		countArgs = append(countArgs, f.Status)
	}
	if err := s.pool.QueryRow(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("db: count jobs: %w", err)
	}

	q := `SELECT id, source_id, status, job_type, priority, target_tags,
	             tasks_total, tasks_pending, tasks_running, tasks_completed, tasks_failed,
	             completed_at, failed_at, created_at, updated_at FROM jobs`
	args := []any{}
	argN := 1

	if f.Status != "" {
		q += fmt.Sprintf(` WHERE status = $%d`, argN)
		args = append(args, f.Status)
		argN++
	}
	if f.Cursor != "" {
		if f.Status != "" {
			q += fmt.Sprintf(` AND id > $%d`, argN)
		} else {
			q += fmt.Sprintf(` WHERE id > $%d`, argN)
		}
		args = append(args, f.Cursor)
		argN++
	}
	q += fmt.Sprintf(` ORDER BY priority DESC, created_at ASC LIMIT $%d`, argN)
	args = append(args, pageSize)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("db: list jobs: %w", err)
	}
	defer rows.Close()
	var out []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, j)
	}
	return out, total, rows.Err()
}

func (s *pgStore) UpdateJobStatus(ctx context.Context, id, status string) error {
	var q string
	switch status {
	case "completed":
		q = `UPDATE jobs SET status = $2, completed_at = now(), updated_at = now() WHERE id = $1`
	case "failed":
		q = `UPDATE jobs SET status = $2, failed_at = now(), updated_at = now() WHERE id = $1`
	default:
		q = `UPDATE jobs SET status = $2, updated_at = now() WHERE id = $1`
	}
	ct, err := s.pool.Exec(ctx, q, id, status)
	if err != nil {
		return fmt.Errorf("db: update job status: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateJobTaskCounts recalculates the denormalised task counter columns
// from the tasks table.  Called after any task status change.
func (s *pgStore) UpdateJobTaskCounts(ctx context.Context, id string) error {
	const q = `
		UPDATE jobs SET
		    tasks_total     = (SELECT COUNT(*)                         FROM tasks WHERE job_id = $1),
		    tasks_pending   = (SELECT COUNT(*) FROM tasks WHERE job_id = $1 AND status = 'pending'),
		    tasks_running   = (SELECT COUNT(*) FROM tasks WHERE job_id = $1 AND status = 'running'),
		    tasks_completed = (SELECT COUNT(*) FROM tasks WHERE job_id = $1 AND status = 'completed'),
		    tasks_failed    = (SELECT COUNT(*) FROM tasks WHERE job_id = $1 AND status = 'failed'),
		    updated_at      = now()
		WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, id)
	return err
}

func scanJob(row pgx.Row) (*Job, error) {
	var j Job
	err := row.Scan(
		&j.ID, &j.SourceID, &j.Status, &j.JobType, &j.Priority, &j.TargetTags,
		&j.TasksTotal, &j.TasksPending, &j.TasksRunning, &j.TasksCompleted, &j.TasksFailed,
		&j.CompletedAt, &j.FailedAt, &j.CreatedAt, &j.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan job: %w", err)
	}
	return &j, nil
}

// ---------------------------------------------------------------------------
// Tasks
// ---------------------------------------------------------------------------

func (s *pgStore) CreateTask(ctx context.Context, p CreateTaskParams) (*Task, error) {
	varsJSON, err := json.Marshal(p.Variables)
	if err != nil {
		return nil, fmt.Errorf("db: marshal task variables: %w", err)
	}
	const q = `
		INSERT INTO tasks (job_id, chunk_index, source_path, output_path, variables)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, job_id, chunk_index, status, agent_id,
		          script_dir, source_path, output_path, variables,
		          exit_code, frames_encoded, avg_fps, output_size, duration_sec,
		          vmaf_score, psnr, ssim, error_msg,
		          started_at, completed_at, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q,
		p.JobID, p.ChunkIndex, p.SourcePath, p.OutputPath, varsJSON,
	)
	return scanTask(row)
}

func (s *pgStore) GetTaskByID(ctx context.Context, id string) (*Task, error) {
	const q = `SELECT id, job_id, chunk_index, status, agent_id,
	                  script_dir, source_path, output_path, variables,
	                  exit_code, frames_encoded, avg_fps, output_size, duration_sec,
	                  vmaf_score, psnr, ssim, error_msg,
	                  started_at, completed_at, created_at, updated_at
	           FROM tasks WHERE id = $1`
	return scanTask(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) ListTasksByJob(ctx context.Context, jobID string) ([]*Task, error) {
	const q = `SELECT id, job_id, chunk_index, status, agent_id,
	                  script_dir, source_path, output_path, variables,
	                  exit_code, frames_encoded, avg_fps, output_size, duration_sec,
	                  vmaf_score, psnr, ssim, error_msg,
	                  started_at, completed_at, created_at, updated_at
	           FROM tasks WHERE job_id = $1 ORDER BY chunk_index`
	rows, err := s.pool.Query(ctx, q, jobID)
	if err != nil {
		return nil, fmt.Errorf("db: list tasks: %w", err)
	}
	defer rows.Close()
	var out []*Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ClaimNextTask atomically selects and assigns the next pending task to the
// given agent, respecting job priority and tag matching.
func (s *pgStore) ClaimNextTask(ctx context.Context, agentID string, tags []string) (*Task, error) {
	// Use a CTE with FOR UPDATE SKIP LOCKED to prevent double-assignment
	// under concurrent agent polls.
	const q = `
		WITH next_task AS (
		    SELECT t.id
		    FROM   tasks t
		    JOIN   jobs  j ON j.id = t.job_id
		    WHERE  t.status = 'pending'
		      AND  j.status IN ('queued', 'running')
		      AND  (j.target_tags = '{}' OR j.target_tags && $2::text[])
		    ORDER BY j.priority DESC, j.created_at ASC, t.chunk_index ASC
		    LIMIT  1
		    FOR UPDATE OF t SKIP LOCKED
		)
		UPDATE tasks SET
		    status     = 'assigned',
		    agent_id   = $1,
		    started_at = now(),
		    updated_at = now()
		FROM next_task
		WHERE tasks.id = next_task.id
		RETURNING tasks.id, tasks.job_id, tasks.chunk_index, tasks.status, tasks.agent_id,
		          tasks.script_dir, tasks.source_path, tasks.output_path, tasks.variables,
		          tasks.exit_code, tasks.frames_encoded, tasks.avg_fps, tasks.output_size,
		          tasks.duration_sec, tasks.vmaf_score, tasks.psnr, tasks.ssim, tasks.error_msg,
		          tasks.started_at, tasks.completed_at, tasks.created_at, tasks.updated_at`
	row := s.pool.QueryRow(ctx, q, agentID, tags)
	t, err := scanTask(row)
	if errors.Is(err, ErrNotFound) {
		return nil, nil // no work available
	}
	return t, err
}

func (s *pgStore) UpdateTaskStatus(ctx context.Context, id, status string) error {
	const q = `UPDATE tasks SET status = $2, updated_at = now() WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id, status)
	if err != nil {
		return fmt.Errorf("db: update task status: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) CompleteTask(ctx context.Context, p CompleteTaskParams) error {
	const q = `
		UPDATE tasks SET
		    status         = 'completed',
		    exit_code      = $2,
		    frames_encoded = $3,
		    avg_fps        = $4,
		    output_size    = $5,
		    duration_sec   = $6,
		    vmaf_score     = $7,
		    psnr           = $8,
		    ssim           = $9,
		    completed_at   = now(),
		    updated_at     = now()
		WHERE id = $1`
	_, err := s.pool.Exec(ctx, q,
		p.ID, p.ExitCode,
		p.FramesEncoded, p.AvgFPS, p.OutputSize, p.DurationSec,
		p.VMafScore, p.PSNR, p.SSIM,
	)
	return err
}

func (s *pgStore) FailTask(ctx context.Context, id string, exitCode int, errMsg string) error {
	const q = `UPDATE tasks SET status = 'failed', exit_code = $2, error_msg = $3,
	                            completed_at = now(), updated_at = now()
	           WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, id, exitCode, errMsg)
	return err
}

func (s *pgStore) CancelPendingTasksForJob(ctx context.Context, jobID string) error {
	const q = `UPDATE tasks SET status = 'cancelled', updated_at = now()
	           WHERE job_id = $1 AND status IN ('pending', 'assigned')`
	_, err := s.pool.Exec(ctx, q, jobID)
	return err
}

func scanTask(row pgx.Row) (*Task, error) {
	var t Task
	var rawVars []byte
	err := row.Scan(
		&t.ID, &t.JobID, &t.ChunkIndex, &t.Status, &t.AgentID,
		&t.ScriptDir, &t.SourcePath, &t.OutputPath, &rawVars,
		&t.ExitCode, &t.FramesEncoded, &t.AvgFPS, &t.OutputSize, &t.DurationSec,
		&t.VMafScore, &t.PSNR, &t.SSIM, &t.ErrorMsg,
		&t.StartedAt, &t.CompletedAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan task: %w", err)
	}
	if len(rawVars) > 0 {
		if err := json.Unmarshal(rawVars, &t.Variables); err != nil {
			return nil, fmt.Errorf("db: unmarshal task variables: %w", err)
		}
	}
	return &t, nil
}

// ---------------------------------------------------------------------------
// Task Logs
// ---------------------------------------------------------------------------

func (s *pgStore) InsertTaskLog(ctx context.Context, p InsertTaskLogParams) error {
	var metaJSON []byte
	if p.Metadata != nil {
		var err error
		metaJSON, err = json.Marshal(p.Metadata)
		if err != nil {
			return fmt.Errorf("db: marshal log metadata: %w", err)
		}
	}
	loggedAt := time.Now()
	if p.LoggedAt != nil {
		loggedAt = *p.LoggedAt
	}
	const q = `INSERT INTO task_logs (task_id, job_id, stream, level, message, metadata, logged_at)
	           VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := s.pool.Exec(ctx, q,
		p.TaskID, p.JobID, p.Stream, p.Level, p.Message, metaJSON, loggedAt,
	)
	return err
}

func (s *pgStore) ListTaskLogs(ctx context.Context, p ListTaskLogsParams) ([]*TaskLog, error) {
	pageSize := p.PageSize
	if pageSize <= 0 || pageSize > 1000 {
		pageSize = 200
	}
	q := `SELECT id, task_id, job_id, stream, level, message, metadata, logged_at
	      FROM task_logs WHERE task_id = $1`
	args := []any{p.TaskID}
	argN := 2
	if p.Stream != "" {
		q += fmt.Sprintf(` AND stream = $%d`, argN)
		args = append(args, p.Stream)
		argN++
	}
	if p.Cursor > 0 {
		q += fmt.Sprintf(` AND id > $%d`, argN)
		args = append(args, p.Cursor)
		argN++
	}
	q += fmt.Sprintf(` ORDER BY logged_at ASC, id ASC LIMIT $%d`, argN)
	args = append(args, pageSize)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("db: list task logs: %w", err)
	}
	defer rows.Close()
	return scanTaskLogs(rows)
}

func (s *pgStore) TailTaskLogs(ctx context.Context, taskID string, afterID int64) ([]*TaskLog, error) {
	const q = `SELECT id, task_id, job_id, stream, level, message, metadata, logged_at
	           FROM task_logs WHERE task_id = $1 AND id > $2
	           ORDER BY logged_at ASC, id ASC LIMIT 500`
	rows, err := s.pool.Query(ctx, q, taskID, afterID)
	if err != nil {
		return nil, fmt.Errorf("db: tail task logs: %w", err)
	}
	defer rows.Close()
	return scanTaskLogs(rows)
}

func scanTaskLogs(rows pgx.Rows) ([]*TaskLog, error) {
	var out []*TaskLog
	for rows.Next() {
		var l TaskLog
		var rawMeta []byte
		if err := rows.Scan(
			&l.ID, &l.TaskID, &l.JobID, &l.Stream, &l.Level,
			&l.Message, &rawMeta, &l.LoggedAt,
		); err != nil {
			return nil, fmt.Errorf("db: scan task log: %w", err)
		}
		if len(rawMeta) > 0 {
			if err := json.Unmarshal(rawMeta, &l.Metadata); err != nil {
				return nil, fmt.Errorf("db: unmarshal log metadata: %w", err)
			}
		}
		out = append(out, &l)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

func (s *pgStore) CreateTemplate(ctx context.Context, p CreateTemplateParams) (*Template, error) {
	const q = `
		INSERT INTO templates (name, description, type, extension, content)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, description, type, extension, content, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.Name, p.Description, p.Type, p.Extension, p.Content)
	return scanTemplate(row)
}

func (s *pgStore) GetTemplateByID(ctx context.Context, id string) (*Template, error) {
	const q = `SELECT id, name, description, type, extension, content, created_at, updated_at
	           FROM templates WHERE id = $1`
	return scanTemplate(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) ListTemplates(ctx context.Context, templateType string) ([]*Template, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if templateType != "" {
		rows, err = s.pool.Query(ctx,
			`SELECT id, name, description, type, extension, content, created_at, updated_at
			 FROM templates WHERE type = $1 ORDER BY name`, templateType)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT id, name, description, type, extension, content, created_at, updated_at
			 FROM templates ORDER BY name`)
	}
	if err != nil {
		return nil, fmt.Errorf("db: list templates: %w", err)
	}
	defer rows.Close()
	var out []*Template
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *pgStore) UpdateTemplate(ctx context.Context, p UpdateTemplateParams) error {
	const q = `UPDATE templates SET name = $2, description = $3, content = $4, updated_at = now()
	           WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, p.ID, p.Name, p.Description, p.Content)
	if err != nil {
		return fmt.Errorf("db: update template: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) DeleteTemplate(ctx context.Context, id string) error {
	const q = `DELETE FROM templates WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("db: delete template: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanTemplate(row pgx.Row) (*Template, error) {
	var t Template
	err := row.Scan(&t.ID, &t.Name, &t.Description, &t.Type, &t.Extension,
		&t.Content, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan template: %w", err)
	}
	return &t, nil
}

// ---------------------------------------------------------------------------
// Variables
// ---------------------------------------------------------------------------

func (s *pgStore) UpsertVariable(ctx context.Context, p UpsertVariableParams) (*Variable, error) {
	const q = `
		INSERT INTO variables (name, value, description, category)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (name) DO UPDATE SET
		    value       = EXCLUDED.value,
		    description = EXCLUDED.description,
		    category    = EXCLUDED.category,
		    updated_at  = now()
		RETURNING id, name, value, description, category, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.Name, p.Value, p.Description, p.Category)
	return scanVariable(row)
}

func (s *pgStore) GetVariableByName(ctx context.Context, name string) (*Variable, error) {
	const q = `SELECT id, name, value, description, category, created_at, updated_at
	           FROM variables WHERE name = $1`
	return scanVariable(s.pool.QueryRow(ctx, q, name))
}

func (s *pgStore) ListVariables(ctx context.Context, category string) ([]*Variable, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if category != "" {
		rows, err = s.pool.Query(ctx,
			`SELECT id, name, value, description, category, created_at, updated_at
			 FROM variables WHERE category = $1 ORDER BY name`, category)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT id, name, value, description, category, created_at, updated_at
			 FROM variables ORDER BY category, name`)
	}
	if err != nil {
		return nil, fmt.Errorf("db: list variables: %w", err)
	}
	defer rows.Close()
	var out []*Variable
	for rows.Next() {
		v, err := scanVariable(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *pgStore) DeleteVariable(ctx context.Context, id string) error {
	const q = `DELETE FROM variables WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("db: delete variable: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanVariable(row pgx.Row) (*Variable, error) {
	var v Variable
	err := row.Scan(&v.ID, &v.Name, &v.Value, &v.Description, &v.Category,
		&v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan variable: %w", err)
	}
	return &v, nil
}

// ---------------------------------------------------------------------------
// Webhooks
// ---------------------------------------------------------------------------

func (s *pgStore) CreateWebhook(ctx context.Context, p CreateWebhookParams) (*Webhook, error) {
	const q = `
		INSERT INTO webhooks (name, provider, url, secret_hash, events)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, provider, url, secret_hash, events, enabled, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.Name, p.Provider, p.URL, p.SecretHash, p.Events)
	return scanWebhook(row)
}

func (s *pgStore) GetWebhookByID(ctx context.Context, id string) (*Webhook, error) {
	const q = `SELECT id, name, provider, url, secret_hash, events, enabled, created_at, updated_at
	           FROM webhooks WHERE id = $1`
	return scanWebhook(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) ListWebhooksByEvent(ctx context.Context, event string) ([]*Webhook, error) {
	const q = `SELECT id, name, provider, url, secret_hash, events, enabled, created_at, updated_at
	           FROM webhooks WHERE events @> $1::text[] AND enabled = true`
	rows, err := s.pool.Query(ctx, q, []string{event})
	if err != nil {
		return nil, fmt.Errorf("db: list webhooks by event: %w", err)
	}
	defer rows.Close()
	return scanWebhooks(rows)
}

func (s *pgStore) ListWebhooks(ctx context.Context) ([]*Webhook, error) {
	const q = `SELECT id, name, provider, url, secret_hash, events, enabled, created_at, updated_at
	           FROM webhooks ORDER BY name`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db: list webhooks: %w", err)
	}
	defer rows.Close()
	return scanWebhooks(rows)
}

func (s *pgStore) UpdateWebhook(ctx context.Context, p UpdateWebhookParams) error {
	const q = `UPDATE webhooks SET name = $2, url = $3, events = $4, enabled = $5, updated_at = now()
	           WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, p.ID, p.Name, p.URL, p.Events, p.Enabled)
	if err != nil {
		return fmt.Errorf("db: update webhook: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) DeleteWebhook(ctx context.Context, id string) error {
	const q = `DELETE FROM webhooks WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("db: delete webhook: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) InsertWebhookDelivery(ctx context.Context, p InsertWebhookDeliveryParams) error {
	const q = `INSERT INTO webhook_deliveries
	               (webhook_id, event, payload, response_code, success, attempt, error_msg)
	           VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := s.pool.Exec(ctx, q,
		p.WebhookID, p.Event, p.Payload,
		p.ResponseCode, p.Success, p.Attempt, p.ErrorMsg,
	)
	return err
}

func scanWebhook(row pgx.Row) (*Webhook, error) {
	var w Webhook
	err := row.Scan(&w.ID, &w.Name, &w.Provider, &w.URL, &w.SecretHash,
		&w.Events, &w.Enabled, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan webhook: %w", err)
	}
	return &w, nil
}

func scanWebhooks(rows pgx.Rows) ([]*Webhook, error) {
	var out []*Webhook
	for rows.Next() {
		w, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Analysis Results
// ---------------------------------------------------------------------------

func (s *pgStore) UpsertAnalysisResult(ctx context.Context, p UpsertAnalysisResultParams) (*AnalysisResult, error) {
	const q = `
		INSERT INTO analysis_results (source_id, type, frame_data, summary)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (source_id, type) DO UPDATE SET
		    frame_data = EXCLUDED.frame_data,
		    summary    = EXCLUDED.summary,
		    created_at = now()
		RETURNING id, source_id, type, frame_data, summary, created_at`
	row := s.pool.QueryRow(ctx, q, p.SourceID, p.Type, p.FrameData, p.Summary)
	return scanAnalysisResult(row)
}

func (s *pgStore) GetAnalysisResult(ctx context.Context, sourceID, analysisType string) (*AnalysisResult, error) {
	const q = `SELECT id, source_id, type, frame_data, summary, created_at
	           FROM analysis_results WHERE source_id = $1 AND type = $2`
	return scanAnalysisResult(s.pool.QueryRow(ctx, q, sourceID, analysisType))
}

func (s *pgStore) ListAnalysisResults(ctx context.Context, sourceID string) ([]*AnalysisResult, error) {
	const q = `SELECT id, source_id, type, frame_data, summary, created_at
	           FROM analysis_results WHERE source_id = $1 ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, q, sourceID)
	if err != nil {
		return nil, fmt.Errorf("db: list analysis results: %w", err)
	}
	defer rows.Close()
	var out []*AnalysisResult
	for rows.Next() {
		r, err := scanAnalysisResult(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanAnalysisResult(row pgx.Row) (*AnalysisResult, error) {
	var r AnalysisResult
	err := row.Scan(&r.ID, &r.SourceID, &r.Type, &r.FrameData, &r.Summary, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan analysis result: %w", err)
	}
	return &r, nil
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

func (s *pgStore) CreateSession(ctx context.Context, p CreateSessionParams) (*Session, error) {
	const q = `INSERT INTO sessions (token, user_id, expires_at)
	           VALUES ($1, $2, $3)
	           RETURNING token, user_id, created_at, expires_at`
	row := s.pool.QueryRow(ctx, q, p.Token, p.UserID, p.ExpiresAt)
	return scanSession(row)
}

func (s *pgStore) GetSessionByToken(ctx context.Context, token string) (*Session, error) {
	const q = `SELECT token, user_id, created_at, expires_at
	           FROM sessions WHERE token = $1 AND expires_at > now()`
	return scanSession(s.pool.QueryRow(ctx, q, token))
}

func (s *pgStore) DeleteSession(ctx context.Context, token string) error {
	const q = `DELETE FROM sessions WHERE token = $1`
	_, err := s.pool.Exec(ctx, q, token)
	return err
}

func (s *pgStore) PruneExpiredSessions(ctx context.Context) error {
	const q = `DELETE FROM sessions WHERE expires_at <= now()`
	_, err := s.pool.Exec(ctx, q)
	return err
}

func scanSession(row pgx.Row) (*Session, error) {
	var sess Session
	err := row.Scan(&sess.Token, &sess.UserID, &sess.CreatedAt, &sess.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan session: %w", err)
	}
	return &sess, nil
}

// ---------------------------------------------------------------------------
// Extended queries
// ---------------------------------------------------------------------------

// RetryFailedTasksForJob re-queues all failed tasks in a job as pending,
// clearing their result fields. Only failed tasks are affected.
func (s *pgStore) RetryFailedTasksForJob(ctx context.Context, jobID string) error {
	const q = `UPDATE tasks SET
	    status         = 'pending',
	    agent_id       = NULL,
	    exit_code      = NULL,
	    frames_encoded = NULL,
	    avg_fps        = NULL,
	    output_size    = NULL,
	    duration_sec   = NULL,
	    vmaf_score     = NULL,
	    psnr           = NULL,
	    ssim           = NULL,
	    error_msg      = NULL,
	    started_at     = NULL,
	    completed_at   = NULL,
	    updated_at     = now()
	WHERE job_id = $1 AND status = 'failed'`
	_, err := s.pool.Exec(ctx, q, jobID)
	return err
}

// ListJobLogs returns task logs for all tasks in a job, ordered by timestamp.
func (s *pgStore) ListJobLogs(ctx context.Context, p ListJobLogsParams) ([]*TaskLog, error) {
	pageSize := p.PageSize
	if pageSize <= 0 || pageSize > 1000 {
		pageSize = 200
	}
	q := `SELECT id, task_id, job_id, stream, level, message, metadata, logged_at
	      FROM task_logs WHERE job_id = $1`
	args := []any{p.JobID}
	argN := 2
	if p.Stream != "" {
		q += fmt.Sprintf(` AND stream = $%d`, argN)
		args = append(args, p.Stream)
		argN++
	}
	if p.Cursor > 0 {
		q += fmt.Sprintf(` AND id > $%d`, argN)
		args = append(args, p.Cursor)
		argN++
	}
	q += fmt.Sprintf(` ORDER BY logged_at ASC, id ASC LIMIT $%d`, argN)
	args = append(args, pageSize)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("db: list job logs: %w", err)
	}
	defer rows.Close()
	return scanTaskLogs(rows)
}

// PruneOldTaskLogs deletes task log rows older than the given time.
func (s *pgStore) PruneOldTaskLogs(ctx context.Context, olderThan time.Time) error {
	const q = `DELETE FROM task_logs WHERE logged_at < $1`
	_, err := s.pool.Exec(ctx, q, olderThan)
	return err
}
