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

func (s *pgStore) CountAdminUsers(ctx context.Context) (int64, error) {
	const q = `SELECT COUNT(*) FROM users WHERE role = 'admin'`
	var n int64
	if err := s.pool.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, fmt.Errorf("db: count admin users: %w", err)
	}
	return n, nil
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
		     agent_version, os_version, cpu_count, ram_mib, nvenc, qsv, amf, vnc_port)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
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
		    vnc_port      = EXCLUDED.vnc_port,
		    updated_at    = now()
		RETURNING id, name, hostname, ip_address, status, tags,
		          gpu_vendor, gpu_model, gpu_enabled,
		          agent_version, os_version, cpu_count, ram_mib,
		          nvenc, qsv, amf, vnc_port, api_key_hash, last_heartbeat,
		          created_at, updated_at`
	row := s.pool.QueryRow(ctx, q,
		p.Name, p.Hostname, p.IPAddress, p.Tags,
		p.GPUVendor, p.GPUModel, p.GPUEnabled,
		p.AgentVersion, p.OSVersion, p.CPUCount, p.RAMMIB,
		p.NVENC, p.QSV, p.AMF, p.VNCPort,
	)
	return scanAgent(row)
}

func (s *pgStore) GetAgentByID(ctx context.Context, id string) (*Agent, error) {
	const q = `SELECT id, name, hostname, ip_address, status, tags,
	                  gpu_vendor, gpu_model, gpu_enabled,
	                  agent_version, os_version, cpu_count, ram_mib,
	                  nvenc, qsv, amf, vnc_port, api_key_hash, last_heartbeat,
	                  created_at, updated_at
	           FROM agents WHERE id = $1`
	return scanAgent(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) GetAgentByName(ctx context.Context, name string) (*Agent, error) {
	const q = `SELECT id, name, hostname, ip_address, status, tags,
	                  gpu_vendor, gpu_model, gpu_enabled,
	                  agent_version, os_version, cpu_count, ram_mib,
	                  nvenc, qsv, amf, vnc_port, api_key_hash, last_heartbeat,
	                  created_at, updated_at
	           FROM agents WHERE name = $1`
	return scanAgent(s.pool.QueryRow(ctx, q, name))
}

func (s *pgStore) ListAgents(ctx context.Context) ([]*Agent, error) {
	const q = `SELECT id, name, hostname, ip_address, status, tags,
	                  gpu_vendor, gpu_model, gpu_enabled,
	                  agent_version, os_version, cpu_count, ram_mib,
	                  nvenc, qsv, amf, vnc_port, api_key_hash, last_heartbeat,
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

func (s *pgStore) UpdateAgentVNCPort(ctx context.Context, id string, port int) error {
	const q = `UPDATE agents SET vnc_port = $2, updated_at = now() WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id, port)
	if err != nil {
		return fmt.Errorf("db: update agent vnc_port: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkStaleAgents sets the status of agents whose last_heartbeat is older than
// olderThan to 'offline'. Returns the number of agents updated.
func (s *pgStore) MarkStaleAgents(ctx context.Context, olderThan time.Duration) (int64, error) {
	const q = `UPDATE agents SET status = 'offline', updated_at = now()
	           WHERE status IN ('idle', 'busy')
	             AND last_heartbeat < now() - $1::interval`
	ct, err := s.pool.Exec(ctx, q, olderThan.String())
	if err != nil {
		return 0, fmt.Errorf("db: mark stale agents: %w", err)
	}
	return ct.RowsAffected(), nil
}

func scanAgent(row pgx.Row) (*Agent, error) {
	var a Agent
	err := row.Scan(
		&a.ID, &a.Name, &a.Hostname, &a.IPAddress, &a.Status, &a.Tags,
		&a.GPUVendor, &a.GPUModel, &a.GPUEnabled,
		&a.AgentVersion, &a.OSVersion, &a.CPUCount, &a.RAMMIB,
		&a.NVENC, &a.QSV, &a.AMF, &a.VNCPort, &a.APIKeyHash, &a.LastHeartbeat,
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
		          hdr_type, dv_profile, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.Filename, p.UNCPath, p.SizeBytes, p.DetectedBy)
	return scanSource(row)
}

func (s *pgStore) GetSourceByID(ctx context.Context, id string) (*Source, error) {
	const q = `SELECT id, filename, unc_path, size_bytes, detected_by, state, vmaf_score,
	                  hdr_type, dv_profile, created_at, updated_at FROM sources WHERE id = $1`
	return scanSource(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) GetSourceByUNCPath(ctx context.Context, uncPath string) (*Source, error) {
	const q = `SELECT id, filename, unc_path, size_bytes, detected_by, state, vmaf_score,
	                  hdr_type, dv_profile, created_at, updated_at FROM sources WHERE unc_path = $1`
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
	             hdr_type, dv_profile, created_at, updated_at FROM sources`
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

func (s *pgStore) UpdateSourceHDR(ctx context.Context, p UpdateSourceHDRParams) error {
	const q = `UPDATE sources SET hdr_type = $2, dv_profile = $3, updated_at = now() WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, p.ID, p.HDRType, p.DVProfile)
	if err != nil {
		return fmt.Errorf("db: update source hdr: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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
		&src.HDRType, &src.DVProfile,
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
	cfgJSON, err := json.Marshal(p.EncodeConfig)
	if err != nil {
		return nil, fmt.Errorf("db: marshal encode_config: %w", err)
	}
	const q = `
		WITH ins AS (
		    INSERT INTO jobs (source_id, job_type, priority, target_tags, encode_config)
		    VALUES ($1, $2, $3, $4, $5)
		    RETURNING id, source_id, status, job_type, priority, target_tags,
		              tasks_total, tasks_pending, tasks_running, tasks_completed, tasks_failed,
		              encode_config, completed_at, failed_at, created_at, updated_at
		)
		SELECT ins.id, ins.source_id, ins.status, ins.job_type, ins.priority, ins.target_tags,
		       ins.tasks_total, ins.tasks_pending, ins.tasks_running, ins.tasks_completed, ins.tasks_failed,
		       ins.encode_config, ins.completed_at, ins.failed_at, ins.created_at, ins.updated_at,
		       COALESCE(s.unc_path, '') AS source_path
		FROM ins LEFT JOIN sources s ON ins.source_id = s.id`
	row := s.pool.QueryRow(ctx, q, p.SourceID, p.JobType, p.Priority, p.TargetTags, cfgJSON)
	return scanJob(row)
}

func (s *pgStore) GetJobByID(ctx context.Context, id string) (*Job, error) {
	const q = `SELECT j.id, j.source_id, j.status, j.job_type, j.priority, j.target_tags,
	                  j.tasks_total, j.tasks_pending, j.tasks_running, j.tasks_completed, j.tasks_failed,
	                  j.encode_config, j.completed_at, j.failed_at, j.created_at, j.updated_at,
	                  COALESCE(s.unc_path, '') AS source_path
	           FROM jobs j LEFT JOIN sources s ON j.source_id = s.id
	           WHERE j.id = $1`
	return scanJob(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) ListJobs(ctx context.Context, f ListJobsFilter) ([]*Job, int64, error) {
	pageSize := f.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	var (
		countConds []string
		countArgs  []any
		argN       = 1
	)
	if f.Status != "" {
		countConds = append(countConds, fmt.Sprintf("j.status = $%d", argN))
		countArgs = append(countArgs, f.Status)
		argN++
	}
	if f.Search != "" {
		countConds = append(countConds, fmt.Sprintf(
			"(j.id::text ILIKE $%d OR COALESCE(s.unc_path,'') ILIKE $%d)", argN, argN))
		countArgs = append(countArgs, "%"+f.Search+"%")
		argN++
	}

	countQ := `SELECT COUNT(*) FROM jobs j LEFT JOIN sources s ON j.source_id = s.id`
	if len(countConds) > 0 {
		countQ += " WHERE " + joinConditions(countConds)
	}
	var total int64
	if err := s.pool.QueryRow(ctx, countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("db: count jobs: %w", err)
	}

	// Reset for main query (argN reused from above)
	var (
		whereConds []string
		args       []any
	)
	argN = 1
	if f.Status != "" {
		whereConds = append(whereConds, fmt.Sprintf("j.status = $%d", argN))
		args = append(args, f.Status)
		argN++
	}
	if f.Search != "" {
		whereConds = append(whereConds, fmt.Sprintf(
			"(j.id::text ILIKE $%d OR COALESCE(s.unc_path,'') ILIKE $%d)", argN, argN))
		args = append(args, "%"+f.Search+"%")
		argN++
	}
	if f.Cursor != "" {
		whereConds = append(whereConds, fmt.Sprintf("j.id > $%d", argN))
		args = append(args, f.Cursor)
		argN++
	}

	q := `SELECT j.id, j.source_id, j.status, j.job_type, j.priority, j.target_tags,
	             j.tasks_total, j.tasks_pending, j.tasks_running, j.tasks_completed, j.tasks_failed,
	             j.encode_config, j.completed_at, j.failed_at, j.created_at, j.updated_at,
	             COALESCE(s.unc_path, '') AS source_path
	      FROM jobs j LEFT JOIN sources s ON j.source_id = s.id`
	if len(whereConds) > 0 {
		q += " WHERE " + joinConditions(whereConds)
	}
	q += fmt.Sprintf(` ORDER BY j.priority DESC, j.created_at ASC LIMIT $%d`, argN)
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

// GetJobsNeedingExpansion returns queued jobs that have not yet been expanded
// into tasks (tasks_total = 0) and have a non-empty encode_config.
func (s *pgStore) GetJobsNeedingExpansion(ctx context.Context) ([]*Job, error) {
	const q = `SELECT j.id, j.source_id, j.status, j.job_type, j.priority, j.target_tags,
	                  j.tasks_total, j.tasks_pending, j.tasks_running, j.tasks_completed, j.tasks_failed,
	                  j.encode_config, j.completed_at, j.failed_at, j.created_at, j.updated_at,
	                  COALESCE(s.unc_path, '') AS source_path
	           FROM jobs j LEFT JOIN sources s ON j.source_id = s.id
	           WHERE j.status = 'queued' AND j.tasks_total = 0
	           ORDER BY j.priority DESC, j.created_at ASC`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db: get jobs needing expansion: %w", err)
	}
	defer rows.Close()
	var out []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
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
	var rawCfg []byte
	err := row.Scan(
		&j.ID, &j.SourceID, &j.Status, &j.JobType, &j.Priority, &j.TargetTags,
		&j.TasksTotal, &j.TasksPending, &j.TasksRunning, &j.TasksCompleted, &j.TasksFailed,
		&rawCfg, &j.CompletedAt, &j.FailedAt, &j.CreatedAt, &j.UpdatedAt,
		&j.SourcePath,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan job: %w", err)
	}
	if len(rawCfg) > 0 {
		if err := json.Unmarshal(rawCfg, &j.EncodeConfig); err != nil {
			return nil, fmt.Errorf("db: unmarshal encode_config: %w", err)
		}
	}
	return &j, nil
}

// joinConditions joins WHERE clause conditions with AND.
func joinConditions(conds []string) string {
	result := ""
	for i, c := range conds {
		if i > 0 {
			result += " AND "
		}
		result += c
	}
	return result
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
		INSERT INTO tasks (job_id, chunk_index, task_type, source_path, output_path, variables)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, job_id, chunk_index, task_type, status, agent_id,
		          script_dir, source_path, output_path, variables,
		          exit_code, frames_encoded, avg_fps, output_size, duration_sec,
		          vmaf_score, psnr, ssim, error_msg,
		          started_at, completed_at, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q,
		p.JobID, p.ChunkIndex, p.TaskType, p.SourcePath, p.OutputPath, varsJSON,
	)
	return scanTask(row)
}

func (s *pgStore) GetTaskByID(ctx context.Context, id string) (*Task, error) {
	const q = `SELECT id, job_id, chunk_index, task_type, status, agent_id,
	                  script_dir, source_path, output_path, variables,
	                  exit_code, frames_encoded, avg_fps, output_size, duration_sec,
	                  vmaf_score, psnr, ssim, error_msg,
	                  started_at, completed_at, created_at, updated_at
	           FROM tasks WHERE id = $1`
	return scanTask(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) ListTasksByJob(ctx context.Context, jobID string) ([]*Task, error) {
	const q = `SELECT id, job_id, chunk_index, task_type, status, agent_id,
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
//
// Concat tasks (task_type = 'concat') are only eligible once all non-concat
// sibling tasks within the same job have reached a terminal status
// (completed, failed, or cancelled).
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
		      -- Concat tasks may only run once every non-concat task in the same
		      -- job has reached a terminal state (completed/failed/cancelled).
		      AND  (
		          t.task_type <> 'concat'
		          OR NOT EXISTS (
		              SELECT 1 FROM tasks t2
		              WHERE  t2.job_id    = t.job_id
		                AND  t2.task_type <> 'concat'
		                AND  t2.status    NOT IN ('completed', 'failed', 'cancelled')
		          )
		      )
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
		RETURNING tasks.id, tasks.job_id, tasks.chunk_index, tasks.task_type, tasks.status, tasks.agent_id,
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

func (s *pgStore) SetTaskScriptDir(ctx context.Context, id, scriptDir string) error {
	const q = `UPDATE tasks SET script_dir = $2, updated_at = now() WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id, scriptDir)
	if err != nil {
		return fmt.Errorf("db: set task script_dir: %w", err)
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

func (s *pgStore) DeleteTasksByJobID(ctx context.Context, jobID string) error {
	const q = `DELETE FROM tasks WHERE job_id = $1`
	_, err := s.pool.Exec(ctx, q, jobID)
	if err != nil {
		return fmt.Errorf("db: delete tasks for job %s: %w", jobID, err)
	}
	return nil
}

func scanTask(row pgx.Row) (*Task, error) {
	var t Task
	var rawVars []byte
	err := row.Scan(
		&t.ID, &t.JobID, &t.ChunkIndex, &t.TaskType, &t.Status, &t.AgentID,
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
	row := s.pool.QueryRow(ctx, q, p.Name, p.Provider, p.URL, p.Secret, p.Events)
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

func (s *pgStore) ListWebhookDeliveries(ctx context.Context, webhookID string, limit, offset int) ([]*WebhookDelivery, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	const q = `SELECT id, webhook_id, event, response_code, success, attempt, error_msg, delivered_at
	           FROM webhook_deliveries WHERE webhook_id = $1
	           ORDER BY delivered_at DESC LIMIT $2 OFFSET $3`
	rows, err := s.pool.Query(ctx, q, webhookID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("db: list webhook deliveries: %w", err)
	}
	defer rows.Close()
	var out []*WebhookDelivery
	for rows.Next() {
		var d WebhookDelivery
		if err := rows.Scan(
			&d.ID, &d.WebhookID, &d.Event,
			&d.ResponseCode, &d.Success, &d.Attempt, &d.ErrorMsg, &d.DeliveredAt,
		); err != nil {
			return nil, fmt.Errorf("db: scan webhook delivery: %w", err)
		}
		out = append(out, &d)
	}
	return out, rows.Err()
}

func scanWebhook(row pgx.Row) (*Webhook, error) {
	var w Webhook
	err := row.Scan(&w.ID, &w.Name, &w.Provider, &w.URL, &w.Secret,
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
// Enrollment Tokens
// ---------------------------------------------------------------------------

func (s *pgStore) CreateEnrollmentToken(ctx context.Context, p CreateEnrollmentTokenParams) (*EnrollmentToken, error) {
	const q = `
		INSERT INTO enrollment_tokens (token, created_by, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, token, created_by, used_by, used_at, expires_at, created_at`
	row := s.pool.QueryRow(ctx, q, p.Token, p.CreatedBy, p.ExpiresAt)
	return scanEnrollmentToken(row)
}

func (s *pgStore) GetEnrollmentToken(ctx context.Context, token string) (*EnrollmentToken, error) {
	const q = `SELECT id, token, created_by, used_by, used_at, expires_at, created_at
	           FROM enrollment_tokens
	           WHERE token = $1 AND expires_at > now() AND used_at IS NULL`
	return scanEnrollmentToken(s.pool.QueryRow(ctx, q, token))
}

func (s *pgStore) ConsumeEnrollmentToken(ctx context.Context, p ConsumeEnrollmentTokenParams) error {
	const q = `UPDATE enrollment_tokens
	           SET used_by = $2, used_at = now()
	           WHERE token = $1 AND used_at IS NULL AND expires_at > now()`
	ct, err := s.pool.Exec(ctx, q, p.Token, p.AgentID)
	if err != nil {
		return fmt.Errorf("db: consume enrollment token: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) ListEnrollmentTokens(ctx context.Context) ([]*EnrollmentToken, error) {
	const q = `SELECT id, token, created_by, used_by, used_at, expires_at, created_at
	           FROM enrollment_tokens ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db: list enrollment tokens: %w", err)
	}
	defer rows.Close()
	var out []*EnrollmentToken
	for rows.Next() {
		t, err := scanEnrollmentToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *pgStore) DeleteEnrollmentToken(ctx context.Context, id string) error {
	const q = `DELETE FROM enrollment_tokens WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("db: delete enrollment token: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) PruneExpiredEnrollmentTokens(ctx context.Context) error {
	const q = `DELETE FROM enrollment_tokens WHERE expires_at <= now()`
	_, err := s.pool.Exec(ctx, q)
	return err
}

func scanEnrollmentToken(row pgx.Row) (*EnrollmentToken, error) {
	var t EnrollmentToken
	err := row.Scan(
		&t.ID, &t.Token, &t.CreatedBy,
		&t.UsedBy, &t.UsedAt,
		&t.ExpiresAt, &t.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan enrollment token: %w", err)
	}
	return &t, nil
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

// Ping verifies the database connection is alive by acquiring a connection
// and issuing a lightweight query.
func (s *pgStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// ---------------------------------------------------------------------------
// Path Mappings
// ---------------------------------------------------------------------------

func (s *pgStore) CreatePathMapping(ctx context.Context, p CreatePathMappingParams) (*PathMapping, error) {
	const q = `
		INSERT INTO path_mappings (name, windows_prefix, linux_prefix)
		VALUES ($1, $2, $3)
		RETURNING id, name, windows_prefix, linux_prefix, enabled, created_at, updated_at`
	return scanPathMapping(s.pool.QueryRow(ctx, q, p.Name, p.WindowsPrefix, p.LinuxPrefix))
}

func (s *pgStore) GetPathMappingByID(ctx context.Context, id string) (*PathMapping, error) {
	const q = `SELECT id, name, windows_prefix, linux_prefix, enabled, created_at, updated_at
	           FROM path_mappings WHERE id = $1`
	return scanPathMapping(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) ListPathMappings(ctx context.Context) ([]*PathMapping, error) {
	const q = `SELECT id, name, windows_prefix, linux_prefix, enabled, created_at, updated_at
	           FROM path_mappings ORDER BY name`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db: list path mappings: %w", err)
	}
	defer rows.Close()
	var out []*PathMapping
	for rows.Next() {
		m, err := scanPathMapping(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *pgStore) UpdatePathMapping(ctx context.Context, p UpdatePathMappingParams) (*PathMapping, error) {
	const q = `UPDATE path_mappings
	           SET name = $2, windows_prefix = $3, linux_prefix = $4, enabled = $5, updated_at = now()
	           WHERE id = $1
	           RETURNING id, name, windows_prefix, linux_prefix, enabled, created_at, updated_at`
	return scanPathMapping(s.pool.QueryRow(ctx, q, p.ID, p.Name, p.WindowsPrefix, p.LinuxPrefix, p.Enabled))
}

func (s *pgStore) DeletePathMapping(ctx context.Context, id string) error {
	const q = `DELETE FROM path_mappings WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("db: delete path mapping: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanPathMapping(row pgx.Row) (*PathMapping, error) {
	var m PathMapping
	err := row.Scan(
		&m.ID, &m.Name, &m.WindowsPrefix, &m.LinuxPrefix,
		&m.Enabled, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan path mapping: %w", err)
	}
	return &m, nil
}
