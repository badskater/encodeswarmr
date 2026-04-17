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
		          upgrade_requested, COALESCE(update_channel, 'stable'), created_at, updated_at`
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
	                  upgrade_requested, COALESCE(update_channel, 'stable'), created_at, updated_at
	           FROM agents WHERE id = $1`
	return scanAgent(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) GetAgentByName(ctx context.Context, name string) (*Agent, error) {
	const q = `SELECT id, name, hostname, ip_address, status, tags,
	                  gpu_vendor, gpu_model, gpu_enabled,
	                  agent_version, os_version, cpu_count, ram_mib,
	                  nvenc, qsv, amf, vnc_port, api_key_hash, last_heartbeat,
	                  upgrade_requested, COALESCE(update_channel, 'stable'), created_at, updated_at
	           FROM agents WHERE name = $1`
	return scanAgent(s.pool.QueryRow(ctx, q, name))
}

func (s *pgStore) ListAgents(ctx context.Context) ([]*Agent, error) {
	const q = `SELECT id, name, hostname, ip_address, status, tags,
	                  gpu_vendor, gpu_model, gpu_enabled,
	                  agent_version, os_version, cpu_count, ram_mib,
	                  nvenc, qsv, amf, vnc_port, api_key_hash, last_heartbeat,
	                  upgrade_requested, COALESCE(update_channel, 'stable'), created_at, updated_at
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
		&a.UpgradeRequested, &a.UpdateChannel, &a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan agent: %w", err)
	}
	return &a, nil
}

// UpdateAgentTags replaces the entire tags array for an agent.
func (s *pgStore) UpdateAgentTags(ctx context.Context, p UpdateAgentTagsParams) error {
	const q = `UPDATE agents SET tags = $2, updated_at = now() WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, p.ID, p.Tags)
	if err != nil {
		return fmt.Errorf("db: update agent tags: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Agent Pools
// ---------------------------------------------------------------------------

func (s *pgStore) CreateAgentPool(ctx context.Context, p CreateAgentPoolParams) (*AgentPool, error) {
	const q = `
		INSERT INTO agent_pools (name, description, tags, color)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, description, tags, color, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.Name, p.Description, p.Tags, p.Color)
	return scanAgentPool(row)
}

func (s *pgStore) GetAgentPoolByID(ctx context.Context, id string) (*AgentPool, error) {
	const q = `SELECT id, name, description, tags, color, created_at, updated_at
	           FROM agent_pools WHERE id = $1`
	return scanAgentPool(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) ListAgentPools(ctx context.Context) ([]*AgentPool, error) {
	const q = `SELECT id, name, description, tags, color, created_at, updated_at
	           FROM agent_pools ORDER BY name`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db: list agent pools: %w", err)
	}
	defer rows.Close()
	var out []*AgentPool
	for rows.Next() {
		ap, err := scanAgentPool(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ap)
	}
	return out, rows.Err()
}

func (s *pgStore) UpdateAgentPool(ctx context.Context, p UpdateAgentPoolParams) (*AgentPool, error) {
	const q = `
		UPDATE agent_pools
		SET name = $2, description = $3, tags = $4, color = $5, updated_at = now()
		WHERE id = $1
		RETURNING id, name, description, tags, color, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.ID, p.Name, p.Description, p.Tags, p.Color)
	return scanAgentPool(row)
}

func (s *pgStore) DeleteAgentPool(ctx context.Context, id string) error {
	const q = `DELETE FROM agent_pools WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("db: delete agent pool: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanAgentPool(row pgx.Row) (*AgentPool, error) {
	var ap AgentPool
	err := row.Scan(&ap.ID, &ap.Name, &ap.Description, &ap.Tags, &ap.Color, &ap.CreatedAt, &ap.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan agent pool: %w", err)
	}
	return &ap, nil
}

// ---------------------------------------------------------------------------
// Sources
// ---------------------------------------------------------------------------

func (s *pgStore) CreateSource(ctx context.Context, p CreateSourceParams) (*Source, error) {
	const q = `
		INSERT INTO sources (filename, unc_path, size_bytes, detected_by, cloud_uri)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, filename, unc_path, size_bytes, detected_by, state, vmaf_score,
		          cloud_uri, hdr_type, dv_profile, thumbnails, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.Filename, p.UNCPath, p.SizeBytes, p.DetectedBy, p.CloudURI)
	return scanSource(row)
}

func (s *pgStore) GetSourceByID(ctx context.Context, id string) (*Source, error) {
	const q = `SELECT id, filename, unc_path, size_bytes, detected_by, state, vmaf_score,
	                  cloud_uri, hdr_type, dv_profile, thumbnails, created_at, updated_at FROM sources WHERE id = $1`
	return scanSource(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) GetSourceByUNCPath(ctx context.Context, uncPath string) (*Source, error) {
	const q = `SELECT id, filename, unc_path, size_bytes, detected_by, state, vmaf_score,
	                  cloud_uri, hdr_type, dv_profile, thumbnails, created_at, updated_at FROM sources WHERE unc_path = $1`
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
	             cloud_uri, hdr_type, dv_profile, thumbnails, created_at, updated_at FROM sources`
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

func (s *pgStore) UpdateSourceThumbnails(ctx context.Context, p UpdateSourceThumbnailsParams) error {
	thumbsJSON, err := json.Marshal(p.Thumbnails)
	if err != nil {
		return fmt.Errorf("db: marshal thumbnails: %w", err)
	}
	const q = `UPDATE sources SET thumbnails = $2, updated_at = now() WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, p.ID, thumbsJSON)
	if err != nil {
		return fmt.Errorf("db: update source thumbnails: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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
	var thumbsJSON []byte
	err := row.Scan(
		&src.ID, &src.Filename, &src.UNCPath, &src.SizeBytes,
		&src.DetectedBy, &src.State, &src.VMafScore,
		&src.CloudURI,
		&src.HDRType, &src.DVProfile,
		&thumbsJSON,
		&src.CreatedAt, &src.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan source: %w", err)
	}
	if len(thumbsJSON) > 0 {
		_ = json.Unmarshal(thumbsJSON, &src.Thumbnails)
	}
	if src.Thumbnails == nil {
		src.Thumbnails = []string{}
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
	var audioCfgJSON []byte
	if p.AudioConfig != nil {
		audioCfgJSON, err = json.Marshal(p.AudioConfig)
		if err != nil {
			return nil, fmt.Errorf("db: marshal audio_config: %w", err)
		}
	}

	// Jobs with an unmet dependency start in "waiting"; all others start in
	// "queued" (the default set by the DB column).
	initialStatus := "queued"
	if p.DependsOn != nil {
		// Check whether the dependency is already completed — if so, skip waiting.
		var depStatus string
		chkQ := `SELECT status FROM jobs WHERE id = $1`
		if err := s.pool.QueryRow(ctx, chkQ, *p.DependsOn).Scan(&depStatus); err == nil && depStatus != "completed" {
			initialStatus = "waiting"
		}
	}

	const q = `
		WITH ins AS (
		    INSERT INTO jobs (source_id, job_type, priority, target_tags, encode_config, audio_config, max_retries, depends_on, chain_group, status)
		    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		    RETURNING id, source_id, status, job_type, priority, target_tags,
		              tasks_total, tasks_pending, tasks_running, tasks_completed, tasks_failed,
		              encode_config, audio_config, max_retries, depends_on, chain_group,
		              completed_at, failed_at, created_at, updated_at
		)
		SELECT ins.id, ins.source_id, ins.status, ins.job_type, ins.priority, ins.target_tags,
		       ins.tasks_total, ins.tasks_pending, ins.tasks_running, ins.tasks_completed, ins.tasks_failed,
		       ins.encode_config, ins.audio_config, ins.max_retries, ins.depends_on, ins.chain_group,
		       ins.completed_at, ins.failed_at, ins.created_at, ins.updated_at,
		       COALESCE(s.unc_path, '') AS source_path
		FROM ins LEFT JOIN sources s ON ins.source_id = s.id`
	row := s.pool.QueryRow(ctx, q,
		p.SourceID, p.JobType, p.Priority, p.TargetTags, cfgJSON, audioCfgJSON,
		p.MaxRetries, p.DependsOn, p.ChainGroup, initialStatus,
	)
	return scanJob(row)
}

func (s *pgStore) GetJobByID(ctx context.Context, id string) (*Job, error) {
	const q = `SELECT j.id, j.source_id, j.status, j.job_type, j.priority, j.target_tags,
	                  j.tasks_total, j.tasks_pending, j.tasks_running, j.tasks_completed, j.tasks_failed,
	                  j.encode_config, j.audio_config, j.max_retries, j.depends_on, j.chain_group,
	                  j.completed_at, j.failed_at, j.created_at, j.updated_at,
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
	             j.encode_config, j.audio_config, j.max_retries, j.depends_on, j.chain_group,
	             j.completed_at, j.failed_at, j.created_at, j.updated_at,
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
// into tasks (tasks_total = 0).  Jobs in "waiting" status (unmet dependency)
// are excluded — they become eligible once UnblockDependentJobs promotes them.
func (s *pgStore) GetJobsNeedingExpansion(ctx context.Context) ([]*Job, error) {
	const q = `SELECT j.id, j.source_id, j.status, j.job_type, j.priority, j.target_tags,
	                  j.tasks_total, j.tasks_pending, j.tasks_running, j.tasks_completed, j.tasks_failed,
	                  j.encode_config, j.audio_config, j.max_retries, j.depends_on, j.chain_group,
	                  j.completed_at, j.failed_at, j.created_at, j.updated_at,
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

// UpdateJobPriority sets the priority of a job.
func (s *pgStore) UpdateJobPriority(ctx context.Context, p UpdateJobPriorityParams) error {
	const q = `UPDATE jobs SET priority = $2, updated_at = now() WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, p.ID, p.Priority)
	if err != nil {
		return fmt.Errorf("db: update job priority: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListPendingJobs returns all queued jobs ordered by priority desc, then created_at asc.
// Used by the QueueManager UI to display and reorder the pending queue.
func (s *pgStore) ListPendingJobs(ctx context.Context) ([]*Job, error) {
	const q = `
		SELECT j.id, j.source_id, j.status, j.job_type, j.priority, j.target_tags,
		       j.tasks_total, j.tasks_pending, j.tasks_running, j.tasks_completed, j.tasks_failed,
		       j.encode_config, j.audio_config, j.max_retries, j.depends_on, j.chain_group,
		       j.completed_at, j.failed_at, j.created_at, j.updated_at,
		       COALESCE(s.unc_path, '') AS source_path
		FROM jobs j
		LEFT JOIN sources s ON s.id = j.source_id
		WHERE j.status IN ('queued', 'waiting')
		ORDER BY j.priority DESC, j.created_at ASC`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db: list pending jobs: %w", err)
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
	var rawAudioCfg []byte
	err := row.Scan(
		&j.ID, &j.SourceID, &j.Status, &j.JobType, &j.Priority, &j.TargetTags,
		&j.TasksTotal, &j.TasksPending, &j.TasksRunning, &j.TasksCompleted, &j.TasksFailed,
		&rawCfg, &rawAudioCfg, &j.MaxRetries, &j.DependsOn, &j.ChainGroup,
		&j.CompletedAt, &j.FailedAt, &j.CreatedAt, &j.UpdatedAt,
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
	if len(rawAudioCfg) > 0 {
		j.AudioConfig = &AudioConfig{}
		if err := json.Unmarshal(rawAudioCfg, j.AudioConfig); err != nil {
			return nil, fmt.Errorf("db: unmarshal audio_config: %w", err)
		}
	}
	return &j, nil
}

// UnblockDependentJobs transitions jobs whose depends_on predecessor is now
// "completed" from "waiting" to "queued" so the engine can expand them.
// This is called atomically after UpdateJobStatus("completed").
func (s *pgStore) UnblockDependentJobs(ctx context.Context, completedJobID string) error {
	const q = `UPDATE jobs SET status = 'queued', updated_at = now()
	           WHERE depends_on = $1 AND status = 'waiting'`
	_, err := s.pool.Exec(ctx, q, completedJobID)
	if err != nil {
		return fmt.Errorf("db: unblock dependent jobs: %w", err)
	}
	return nil
}

// ListJobsByChainGroup returns all jobs in a chain group ordered by creation time.
func (s *pgStore) ListJobsByChainGroup(ctx context.Context, chainGroup string) ([]*Job, error) {
	const q = `SELECT j.id, j.source_id, j.status, j.job_type, j.priority, j.target_tags,
	                  j.tasks_total, j.tasks_pending, j.tasks_running, j.tasks_completed, j.tasks_failed,
	                  j.encode_config, j.audio_config, j.max_retries, j.depends_on, j.chain_group,
	                  j.completed_at, j.failed_at, j.created_at, j.updated_at,
	                  COALESCE(s.unc_path, '') AS source_path
	           FROM jobs j LEFT JOIN sources s ON j.source_id = s.id
	           WHERE j.chain_group = $1
	           ORDER BY j.created_at ASC`
	rows, err := s.pool.Query(ctx, q, chainGroup)
	if err != nil {
		return nil, fmt.Errorf("db: list jobs by chain group: %w", err)
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
		          retry_count, retry_after,
		          preemptible, preempted_at,
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
	                  retry_count, retry_after,
	                  preemptible, preempted_at,
	                  started_at, completed_at, created_at, updated_at
	           FROM tasks WHERE id = $1`
	return scanTask(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) ListTasksByJob(ctx context.Context, jobID string) ([]*Task, error) {
	const q = `SELECT id, job_id, chunk_index, task_type, status, agent_id,
	                  script_dir, source_path, output_path, variables,
	                  exit_code, frames_encoded, avg_fps, output_size, duration_sec,
	                  vmaf_score, psnr, ssim, error_msg,
	                  retry_count, retry_after,
	                  preemptible, preempted_at,
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
		      -- Concat tasks are handled by the controller-side ConcatRunner and
		      -- must never be dispatched to an agent.
		      AND  t.task_type <> 'concat'
		      -- Respect retry backoff: only claim if retry_after has elapsed.
		      AND  (t.retry_after IS NULL OR t.retry_after <= now())
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
		          tasks.retry_count, tasks.retry_after,
		          tasks.preemptible, tasks.preempted_at,
		          tasks.started_at, tasks.completed_at, tasks.created_at, tasks.updated_at`
	row := s.pool.QueryRow(ctx, q, agentID, tags)
	t, err := scanTask(row)
	if errors.Is(err, ErrNotFound) {
		return nil, nil // no work available
	}
	return t, err
}

// ClaimConcatTask atomically transitions a concat task from "pending" to
// "running".  Returns ErrNotFound if the task does not exist or is no longer
// in the pending state (i.e., another goroutine already claimed it).
func (s *pgStore) ClaimConcatTask(ctx context.Context, id string) error {
	const q = `UPDATE tasks SET status = 'running', started_at = now(), updated_at = now()
	           WHERE id = $1 AND status = 'pending'`
	ct, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("db: claim concat task: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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
		&t.RetryCount, &t.RetryAfter,
		&t.Preemptible, &t.PreemptedAt,
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

// ---------------------------------------------------------------------------
// Audit Log
// ---------------------------------------------------------------------------

func (s *pgStore) CreateAuditEntry(ctx context.Context, p CreateAuditEntryParams) error {
	const q = `INSERT INTO audit_log (user_id, username, action, resource, resource_id, detail, ip_address)
	           VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := s.pool.Exec(ctx, q, p.UserID, p.Username, p.Action, p.Resource, p.ResourceID, p.Detail, p.IPAddress)
	return err
}

func (s *pgStore) ListAuditLog(ctx context.Context, limit, offset int) ([]*AuditEntry, int, error) {
	const countQ = `SELECT COUNT(*) FROM audit_log`
	var total int
	if err := s.pool.QueryRow(ctx, countQ).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("db: count audit log: %w", err)
	}
	const q = `SELECT id, user_id, username, action, resource, resource_id, detail, ip_address, logged_at
	           FROM audit_log ORDER BY logged_at DESC LIMIT $1 OFFSET $2`
	rows, err := s.pool.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("db: list audit log: %w", err)
	}
	defer rows.Close()
	var entries []*AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.Username, &e.Action,
			&e.Resource, &e.ResourceID, &e.Detail, &e.IPAddress, &e.LoggedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("db: scan audit entry: %w", err)
		}
		entries = append(entries, &e)
	}
	return entries, total, rows.Err()
}

func scanAuditEntry(row pgx.Row) (*AuditEntry, error) {
	var e AuditEntry
	err := row.Scan(
		&e.ID, &e.UserID, &e.Username, &e.Action,
		&e.Resource, &e.ResourceID, &e.Detail, &e.IPAddress, &e.LoggedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan audit entry: %w", err)
	}
	return &e, nil
}

// ---------------------------------------------------------------------------
// Agent Metrics
// ---------------------------------------------------------------------------

func (s *pgStore) InsertAgentMetric(ctx context.Context, p InsertAgentMetricParams) error {
	const q = `INSERT INTO agent_metrics (agent_id, cpu_pct, gpu_pct, mem_pct)
	           VALUES ($1, $2, $3, $4)`
	_, err := s.pool.Exec(ctx, q, p.AgentID, p.CPUPct, p.GPUPct, p.MemPct)
	return err
}

func (s *pgStore) ListAgentMetrics(ctx context.Context, agentID string, since time.Time) ([]*AgentMetric, error) {
	const q = `SELECT id, agent_id, cpu_pct, gpu_pct, mem_pct, recorded_at
	           FROM agent_metrics
	           WHERE agent_id = $1 AND recorded_at >= $2
	           ORDER BY recorded_at ASC`
	rows, err := s.pool.Query(ctx, q, agentID, since)
	if err != nil {
		return nil, fmt.Errorf("db: list agent metrics: %w", err)
	}
	defer rows.Close()
	var out []*AgentMetric
	for rows.Next() {
		var m AgentMetric
		if err := rows.Scan(&m.ID, &m.AgentID, &m.CPUPct, &m.GPUPct, &m.MemPct, &m.RecordedAt); err != nil {
			return nil, fmt.Errorf("db: scan agent metric: %w", err)
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Schedules
// ---------------------------------------------------------------------------

func (s *pgStore) CreateSchedule(ctx context.Context, p CreateScheduleParams) (*Schedule, error) {
	const q = `
		INSERT INTO schedules (name, cron_expr, job_template, enabled, next_run_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, cron_expr, job_template, enabled, last_run_at, next_run_at, created_at`
	row := s.pool.QueryRow(ctx, q, p.Name, p.CronExpr, p.JobTemplate, p.Enabled, p.NextRunAt)
	return scanSchedule(row)
}

func (s *pgStore) GetScheduleByID(ctx context.Context, id string) (*Schedule, error) {
	const q = `SELECT id, name, cron_expr, job_template, enabled, last_run_at, next_run_at, created_at
	           FROM schedules WHERE id = $1`
	return scanSchedule(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) ListSchedules(ctx context.Context) ([]*Schedule, error) {
	const q = `SELECT id, name, cron_expr, job_template, enabled, last_run_at, next_run_at, created_at
	           FROM schedules ORDER BY name`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db: list schedules: %w", err)
	}
	defer rows.Close()
	var out []*Schedule
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (s *pgStore) UpdateSchedule(ctx context.Context, p UpdateScheduleParams) (*Schedule, error) {
	const q = `
		UPDATE schedules
		SET name = $2, cron_expr = $3, job_template = $4, enabled = $5, next_run_at = $6
		WHERE id = $1
		RETURNING id, name, cron_expr, job_template, enabled, last_run_at, next_run_at, created_at`
	row := s.pool.QueryRow(ctx, q, p.ID, p.Name, p.CronExpr, p.JobTemplate, p.Enabled, p.NextRunAt)
	sc, err := scanSchedule(row)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return sc, nil
}

func (s *pgStore) DeleteSchedule(ctx context.Context, id string) error {
	const q = `DELETE FROM schedules WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("db: delete schedule: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListDueSchedules returns enabled schedules whose next_run_at is at or before now.
func (s *pgStore) ListDueSchedules(ctx context.Context) ([]*Schedule, error) {
	const q = `SELECT id, name, cron_expr, job_template, enabled, last_run_at, next_run_at, created_at
	           FROM schedules
	           WHERE enabled = true AND next_run_at <= now()
	           ORDER BY next_run_at ASC`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db: list due schedules: %w", err)
	}
	defer rows.Close()
	var out []*Schedule
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// MarkScheduleRun updates last_run_at and next_run_at after a schedule fires.
func (s *pgStore) MarkScheduleRun(ctx context.Context, p MarkScheduleRunParams) error {
	const q = `UPDATE schedules SET last_run_at = $2, next_run_at = $3 WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, p.ID, p.LastRunAt, p.NextRunAt)
	if err != nil {
		return fmt.Errorf("db: mark schedule run: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Estimation
// ---------------------------------------------------------------------------

// GetAvgFPSStats returns the aggregate average avg_fps and number of completed
// encode tasks for the given source.  Tasks with a NULL avg_fps are excluded.
// Returns (0, 0, nil) when there are no matching rows.
func (s *pgStore) GetAvgFPSStats(ctx context.Context, sourceID string) (float64, int64, error) {
	const q = `
		SELECT COALESCE(AVG(t.avg_fps), 0), COUNT(*)
		FROM tasks t
		JOIN jobs j ON t.job_id = j.id
		WHERE j.source_id = $1
		  AND j.job_type = 'encode'
		  AND t.task_type = ''
		  AND t.status = 'completed'
		  AND t.avg_fps IS NOT NULL
		  AND t.avg_fps > 0`
	var avgFPS float64
	var count int64
	err := s.pool.QueryRow(ctx, q, sourceID).Scan(&avgFPS, &count)
	if err != nil {
		return 0, 0, fmt.Errorf("db: get avg fps stats: %w", err)
	}
	return avgFPS, count, nil
}

func scanSchedule(row pgx.Row) (*Schedule, error) {
	var sc Schedule
	var rawTmpl []byte
	err := row.Scan(
		&sc.ID, &sc.Name, &sc.CronExpr, &rawTmpl, &sc.Enabled,
		&sc.LastRunAt, &sc.NextRunAt, &sc.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan schedule: %w", err)
	}
	sc.JobTemplate = rawTmpl
	return &sc, nil
}

// ---------------------------------------------------------------------------
// API Keys
// ---------------------------------------------------------------------------

func (s *pgStore) CreateAPIKey(ctx context.Context, p CreateAPIKeyParams) (*APIKey, error) {
	const q = `
		INSERT INTO api_keys (user_id, name, key_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, name, COALESCE(rate_limit, 0), created_at, last_used_at, expires_at`
	row := s.pool.QueryRow(ctx, q, p.UserID, p.Name, p.KeyHash, p.ExpiresAt)
	return scanAPIKey(row)
}

// GetAPIKeyByHash looks up an API key by its SHA-256 hash.  It also verifies
// the key has not expired (when expires_at is set).
func (s *pgStore) GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	const q = `
		SELECT id, user_id, name, COALESCE(rate_limit, 0), created_at, last_used_at, expires_at
		FROM api_keys
		WHERE key_hash = $1
		  AND (expires_at IS NULL OR expires_at > now())`
	return scanAPIKey(s.pool.QueryRow(ctx, q, keyHash))
}

func (s *pgStore) ListAPIKeysByUser(ctx context.Context, userID string) ([]*APIKey, error) {
	const q = `
		SELECT id, user_id, name, COALESCE(rate_limit, 0), created_at, last_used_at, expires_at
		FROM api_keys
		WHERE user_id = $1
		ORDER BY created_at`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("db: list api keys: %w", err)
	}
	defer rows.Close()
	var out []*APIKey
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *pgStore) DeleteAPIKey(ctx context.Context, id string) error {
	const q = `DELETE FROM api_keys WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("db: delete api key: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Notification Preferences
// ---------------------------------------------------------------------------

// GetNotificationPrefs returns the notification preferences for the given user.
// Returns ErrNotFound if no preferences row exists yet (use defaults in that case).
func (s *pgStore) GetNotificationPrefs(ctx context.Context, userID string) (*NotificationPrefs, error) {
	const q = `SELECT id, user_id, notify_on_job_complete, notify_on_job_failed,
	                  notify_on_agent_stale, webhook_filter_user_only,
	                  email_address, notify_email, created_at, updated_at
	           FROM notification_preferences WHERE user_id = $1`
	return scanNotificationPrefs(s.pool.QueryRow(ctx, q, userID))
}

// UpsertNotificationPrefs creates or updates the notification preferences for a user.
func (s *pgStore) UpsertNotificationPrefs(ctx context.Context, p UpsertNotificationPrefsParams) error {
	const q = `
		INSERT INTO notification_preferences
		    (user_id, notify_on_job_complete, notify_on_job_failed,
		     notify_on_agent_stale, webhook_filter_user_only,
		     email_address, notify_email)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id) DO UPDATE SET
		    notify_on_job_complete   = EXCLUDED.notify_on_job_complete,
		    notify_on_job_failed     = EXCLUDED.notify_on_job_failed,
		    notify_on_agent_stale    = EXCLUDED.notify_on_agent_stale,
		    webhook_filter_user_only = EXCLUDED.webhook_filter_user_only,
		    email_address            = EXCLUDED.email_address,
		    notify_email             = EXCLUDED.notify_email,
		    updated_at               = now()`
	_, err := s.pool.Exec(ctx, q,
		p.UserID, p.NotifyOnJobComplete, p.NotifyOnJobFailed,
		p.NotifyOnAgentStale, p.WebhookFilterUserOnly,
		p.EmailAddress, p.NotifyEmail,
	)
	if err != nil {
		return fmt.Errorf("db: upsert notification prefs: %w", err)
	}
	return nil
}

// ListUsersWithEmailNotifications returns notification preferences for all
// users that have email notifications enabled. Used by the webhook service to
// dispatch emails alongside webhook deliveries.
func (s *pgStore) ListUsersWithEmailNotifications(ctx context.Context) ([]*NotificationPrefs, error) {
	const q = `SELECT id, user_id, notify_on_job_complete, notify_on_job_failed,
	                  notify_on_agent_stale, webhook_filter_user_only,
	                  email_address, notify_email, created_at, updated_at
	           FROM notification_preferences
	           WHERE notify_email = true AND email_address <> ''
	           ORDER BY user_id`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db: list users with email notifications: %w", err)
	}
	defer rows.Close()
	var out []*NotificationPrefs
	for rows.Next() {
		np, err := scanNotificationPrefs(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, np)
	}
	return out, rows.Err()
}

func scanNotificationPrefs(row pgx.Row) (*NotificationPrefs, error) {
	var np NotificationPrefs
	err := row.Scan(
		&np.ID, &np.UserID,
		&np.NotifyOnJobComplete, &np.NotifyOnJobFailed,
		&np.NotifyOnAgentStale, &np.WebhookFilterUserOnly,
		&np.EmailAddress, &np.NotifyEmail,
		&np.CreatedAt, &np.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan notification prefs: %w", err)
	}
	return &np, nil
}

// ---------------------------------------------------------------------------
// Agent upgrade flag
// ---------------------------------------------------------------------------

// SetAgentUpgradeRequested sets or clears the upgrade_requested flag for the
// given agent. Use requested=true to request an upgrade, false to clear it.
func (s *pgStore) SetAgentUpgradeRequested(ctx context.Context, agentID string, requested bool) error {
	const q = `UPDATE agents SET upgrade_requested = $2, updated_at = now() WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, agentID, requested)
	if err != nil {
		return fmt.Errorf("db: set agent upgrade_requested: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *pgStore) UpdateAPIKeyLastUsed(ctx context.Context, id string) error {
	const q = `UPDATE api_keys SET last_used_at = now() WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("db: update api key last used: %w", err)
	}
	return nil
}

func scanAPIKey(row pgx.Row) (*APIKey, error) {
	var k APIKey
	err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.RateLimit, &k.CreatedAt, &k.LastUsedAt, &k.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan api key: %w", err)
	}
	return &k, nil
}

// ClearAgentUpgradeRequested clears the upgrade_requested flag for the given agent.
func (s *pgStore) ClearAgentUpgradeRequested(ctx context.Context, agentID string) error {
	return s.SetAgentUpgradeRequested(ctx, agentID, false)
}

// ---------------------------------------------------------------------------
// Task retry with exponential backoff
// ---------------------------------------------------------------------------

// RetryTaskWithBackoff inserts a new pending task row that is a copy of the
// failed task, with retry_count incremented and retry_after set to
// now() + (2^retryCount * 30 seconds).  The original failed task is left
// unchanged.  Returns the newly created task row.
// ---------------------------------------------------------------------------
// Dashboard metrics
// ---------------------------------------------------------------------------

// GetThroughputStats returns the count of completed jobs grouped by hour over
// the last N hours.
func (s *pgStore) GetThroughputStats(ctx context.Context, hours int) ([]*ThroughputPoint, error) {
	if hours <= 0 {
		hours = 24
	}
	const q = `
		SELECT date_trunc('hour', updated_at) AS hour, count(*) AS count
		FROM   jobs
		WHERE  status = 'completed'
		  AND  updated_at > now() - ($1 * interval '1 hour')
		GROUP  BY 1
		ORDER  BY 1`
	rows, err := s.pool.Query(ctx, q, hours)
	if err != nil {
		return nil, fmt.Errorf("db: get throughput stats: %w", err)
	}
	defer rows.Close()
	var out []*ThroughputPoint
	for rows.Next() {
		var p ThroughputPoint
		if err := rows.Scan(&p.Hour, &p.Count); err != nil {
			return nil, fmt.Errorf("db: get throughput stats scan: %w", err)
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// GetQueueStats returns the current counts of pending and running tasks, plus
// an estimated time to completion based on average task duration.
func (s *pgStore) GetQueueStats(ctx context.Context) (*QueueStats, error) {
	// Count pending and running tasks.
	const countQ = `
		SELECT status, count(*)
		FROM   tasks
		WHERE  status IN ('pending', 'running')
		GROUP  BY status`
	rows, err := s.pool.Query(ctx, countQ)
	if err != nil {
		return nil, fmt.Errorf("db: get queue stats count: %w", err)
	}
	defer rows.Close()

	stats := &QueueStats{}
	for rows.Next() {
		var status string
		var cnt int64
		if err := rows.Scan(&status, &cnt); err != nil {
			return nil, fmt.Errorf("db: get queue stats scan: %w", err)
		}
		switch status {
		case "pending":
			stats.Pending = cnt
		case "running":
			stats.Running = cnt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Estimate completion from average task duration of recently completed tasks.
	const avgQ = `
		SELECT COALESCE(avg(extract(epoch FROM (completed_at - started_at))), 0)
		FROM   tasks
		WHERE  status = 'completed'
		  AND  started_at IS NOT NULL
		  AND  completed_at IS NOT NULL
		  AND  completed_at > now() - interval '24 hours'`
	var avgSec float64
	if err := s.pool.QueryRow(ctx, avgQ).Scan(&avgSec); err != nil {
		return nil, fmt.Errorf("db: get queue stats avg: %w", err)
	}

	remaining := float64(stats.Pending + stats.Running)
	if avgSec > 0 && remaining > 0 {
		stats.EstimatedMinutes = (remaining * avgSec) / 60.0
	}

	return stats, nil
}

// GetRecentActivity returns the most recent job status changes, joining to
// sources to include the source name.
func (s *pgStore) GetRecentActivity(ctx context.Context, limit int) ([]*ActivityEvent, error) {
	if limit <= 0 {
		limit = 10
	}
	const q = `
		SELECT j.id, j.status, s.name AS source_name, j.updated_at
		FROM   jobs j
		JOIN   sources s ON j.source_id = s.id
		ORDER  BY j.updated_at DESC
		LIMIT  $1`
	rows, err := s.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("db: get recent activity: %w", err)
	}
	defer rows.Close()
	var out []*ActivityEvent
	for rows.Next() {
		var e ActivityEvent
		if err := rows.Scan(&e.JobID, &e.Status, &e.SourceName, &e.Timestamp); err != nil {
			return nil, fmt.Errorf("db: get recent activity scan: %w", err)
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

func (s *pgStore) RetryTaskWithBackoff(ctx context.Context, taskID string, retryCount int) (*Task, error) {
	// backoffSeconds = 2^retryCount * 30 (capped at 1 hour = 3600 s).
	backoffSeconds := (1 << retryCount) * 30
	if backoffSeconds > 3600 {
		backoffSeconds = 3600
	}

	const q = `
		INSERT INTO tasks
		    (job_id, chunk_index, task_type, source_path, output_path, variables,
		     retry_count, retry_after)
		SELECT job_id,
		       (SELECT COALESCE(MAX(t2.chunk_index), 0) + 1 FROM tasks t2 WHERE t2.job_id = t.job_id),
		       task_type, source_path, output_path, variables,
		       $2, now() + ($3 * interval '1 second')
		FROM   tasks t
		WHERE  t.id = $1
		RETURNING id, job_id, chunk_index, task_type, status, agent_id,
		          script_dir, source_path, output_path, variables,
		          exit_code, frames_encoded, avg_fps, output_size, duration_sec,
		          vmaf_score, psnr, ssim, error_msg,
		          retry_count, retry_after,
		          preemptible, preempted_at,
		          started_at, completed_at, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, taskID, retryCount+1, backoffSeconds)
	t, err := scanTask(row)
	if err != nil {
		return nil, fmt.Errorf("db: retry task with backoff: %w", err)
	}
	return t, nil
}

// RetryTaskWithBackoffJitter creates a new retry task with exponential backoff
// plus an additional jitter delay in seconds.
func (s *pgStore) RetryTaskWithBackoffJitter(ctx context.Context, taskID string, retryCount int, jitterSec int) (*Task, error) {
	// backoffSeconds = 2^retryCount * 30 (capped at 1 hour = 3600 s) + jitter.
	backoffSeconds := (1 << retryCount) * 30
	if backoffSeconds > 3600 {
		backoffSeconds = 3600
	}
	backoffSeconds += jitterSec

	const q = `
		INSERT INTO tasks
		    (job_id, chunk_index, task_type, source_path, output_path, variables,
		     retry_count, retry_after)
		SELECT job_id,
		       (SELECT COALESCE(MAX(t2.chunk_index), 0) + 1 FROM tasks t2 WHERE t2.job_id = t.job_id),
		       task_type, source_path, output_path, variables,
		       $2, now() + ($3 * interval '1 second')
		FROM   tasks t
		WHERE  t.id = $1
		RETURNING id, job_id, chunk_index, task_type, status, agent_id,
		          script_dir, source_path, output_path, variables,
		          exit_code, frames_encoded, avg_fps, output_size, duration_sec,
		          vmaf_score, psnr, ssim, error_msg,
		          retry_count, retry_after,
		          started_at, completed_at, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, taskID, retryCount+1, backoffSeconds)
	t, err := scanTask(row)
	if err != nil {
		return nil, fmt.Errorf("db: retry task with backoff jitter: %w", err)
	}
	return t, nil
}

// ---------------------------------------------------------------------------
// Flows
// ---------------------------------------------------------------------------

func (s *pgStore) CreateFlow(ctx context.Context, p CreateFlowParams) (*Flow, error) {
	graph := p.Graph
	if len(graph) == 0 {
		graph = json.RawMessage(`{"nodes":[],"edges":[]}`)
	}
	const q = `
		INSERT INTO flows (name, description, graph)
		VALUES ($1, $2, $3)
		RETURNING id, name, description, graph, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.Name, p.Description, []byte(graph))
	return scanFlow(row)
}

func (s *pgStore) GetFlowByID(ctx context.Context, id string) (*Flow, error) {
	const q = `
		SELECT id, name, description, graph, created_at, updated_at
		FROM flows WHERE id = $1`
	return scanFlow(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) ListFlows(ctx context.Context) ([]*Flow, error) {
	const q = `
		SELECT id, name, description, graph, created_at, updated_at
		FROM flows ORDER BY updated_at DESC`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db: list flows: %w", err)
	}
	defer rows.Close()
	var out []*Flow
	for rows.Next() {
		f, err := scanFlow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *pgStore) UpdateFlow(ctx context.Context, p UpdateFlowParams) (*Flow, error) {
	graph := p.Graph
	if len(graph) == 0 {
		graph = json.RawMessage(`{"nodes":[],"edges":[]}`)
	}
	const q = `
		UPDATE flows
		SET name = $2, description = $3, graph = $4, updated_at = now()
		WHERE id = $1
		RETURNING id, name, description, graph, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.ID, p.Name, p.Description, []byte(graph))
	f, err := scanFlow(row)
	if err != nil {
		return nil, fmt.Errorf("db: update flow: %w", err)
	}
	return f, nil
}

func (s *pgStore) DeleteFlow(ctx context.Context, id string) error {
	const q = `DELETE FROM flows WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("db: delete flow: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanFlow(row pgx.Row) (*Flow, error) {
	var f Flow
	var graph []byte
	err := row.Scan(
		&f.ID, &f.Name, &f.Description, &graph,
		&f.CreatedAt, &f.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan flow: %w", err)
	}
	if len(graph) > 0 {
		f.Graph = json.RawMessage(graph)
	} else {
		f.Graph = json.RawMessage(`{"nodes":[],"edges":[]}`)
	}
	return &f, nil
}

// ---------------------------------------------------------------------------
// Job Archive
// ---------------------------------------------------------------------------

// ArchiveOldJobs moves completed/failed jobs older than the given duration
// to job_archive (along with their tasks and task_logs) in a single
// transaction.  Returns the number of jobs archived.
func (s *pgStore) ArchiveOldJobs(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("db: archive jobs: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Archive task logs for eligible jobs.
	const archiveLogs = `
		INSERT INTO task_log_archive
		    (id, task_id, job_id, stream, level, message, metadata, logged_at)
		SELECT id, task_id, job_id, stream, level, message, metadata, logged_at
		FROM   task_logs
		WHERE  job_id IN (
		           SELECT id FROM jobs
		           WHERE  status IN ('completed','failed','cancelled')
		           AND    updated_at < $1
		       )`
	if _, err := tx.Exec(ctx, archiveLogs, cutoff); err != nil {
		return 0, fmt.Errorf("db: archive job logs: %w", err)
	}

	// Delete task logs for those jobs.
	const deleteLogs = `
		DELETE FROM task_logs
		WHERE job_id IN (
		    SELECT id FROM jobs
		    WHERE  status IN ('completed','failed','cancelled')
		    AND    updated_at < $1
		)`
	if _, err := tx.Exec(ctx, deleteLogs, cutoff); err != nil {
		return 0, fmt.Errorf("db: delete archived logs: %w", err)
	}

	// Archive tasks for eligible jobs.
	const archiveTasks = `
		INSERT INTO task_archive
		    (id, job_id, chunk_index, task_type, status, agent_id,
		     script_dir, source_path, output_path, variables,
		     exit_code, frames_encoded, avg_fps, output_size, duration_sec,
		     vmaf_score, psnr, ssim, error_msg,
		     retry_count, retry_after,
		     started_at, completed_at, created_at, updated_at)
		SELECT id, job_id, chunk_index, task_type, status, agent_id,
		       script_dir, source_path, output_path, variables,
		       exit_code, frames_encoded, avg_fps, output_size, duration_sec,
		       vmaf_score, psnr, ssim, error_msg,
		       retry_count, retry_after,
		       started_at, completed_at, created_at, updated_at
		FROM   tasks
		WHERE  job_id IN (
		           SELECT id FROM jobs
		           WHERE  status IN ('completed','failed','cancelled')
		           AND    updated_at < $1
		       )`
	if _, err := tx.Exec(ctx, archiveTasks, cutoff); err != nil {
		return 0, fmt.Errorf("db: archive tasks: %w", err)
	}

	// Delete tasks for those jobs (cascades handled by task_logs already removed).
	const deleteTasks = `
		DELETE FROM tasks
		WHERE job_id IN (
		    SELECT id FROM jobs
		    WHERE  status IN ('completed','failed','cancelled')
		    AND    updated_at < $1
		)`
	if _, err := tx.Exec(ctx, deleteTasks, cutoff); err != nil {
		return 0, fmt.Errorf("db: delete archived tasks: %w", err)
	}

	// Archive jobs.
	const archiveJobs = `
		INSERT INTO job_archive
		    (id, source_id, status, job_type, priority, target_tags,
		     tasks_total, tasks_pending, tasks_running, tasks_completed, tasks_failed,
		     encode_config, max_retries,
		     completed_at, failed_at, created_at, updated_at)
		SELECT id, source_id, status, job_type, priority, target_tags,
		       tasks_total, tasks_pending, tasks_running, tasks_completed, tasks_failed,
		       encode_config, max_retries,
		       completed_at, failed_at, created_at, updated_at
		FROM   jobs
		WHERE  status IN ('completed','failed','cancelled')
		AND    updated_at < $1`
	ct, err := tx.Exec(ctx, archiveJobs, cutoff)
	if err != nil {
		return 0, fmt.Errorf("db: archive jobs: %w", err)
	}
	archived := ct.RowsAffected()

	// Delete archived jobs from the active table.
	const deleteJobs = `
		DELETE FROM jobs
		WHERE  status IN ('completed','failed','cancelled')
		AND    updated_at < $1`
	if _, err := tx.Exec(ctx, deleteJobs, cutoff); err != nil {
		return 0, fmt.Errorf("db: delete archived jobs: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("db: archive jobs: commit: %w", err)
	}

	return archived, nil
}

// ListArchivedJobs returns a paginated list of archived jobs ordered by
// created_at descending. The source_path column is populated as an empty
// string because archived jobs may reference deleted sources.
func (s *pgStore) ListArchivedJobs(ctx context.Context, f ListJobsFilter) ([]*Job, int64, error) {
	pageSize := f.PageSize
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}

	var (
		whereConds []string
		args       []any
		argN       = 1
	)
	if f.Status != "" {
		whereConds = append(whereConds, fmt.Sprintf("status = $%d", argN))
		args = append(args, f.Status)
		argN++
	}
	if f.Search != "" {
		whereConds = append(whereConds, fmt.Sprintf("id::text ILIKE $%d", argN))
		args = append(args, "%"+f.Search+"%")
		argN++
	}

	whereClause := ""
	if len(whereConds) > 0 {
		whereClause = "WHERE " + joinConditions(whereConds)
	}

	countQ := `SELECT COUNT(*) FROM job_archive ` + whereClause
	var total int64
	if err := s.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("db: count archived jobs: %w", err)
	}

	if f.Cursor != "" {
		whereConds = append(whereConds, fmt.Sprintf("created_at < $%d", argN))
		args = append(args, f.Cursor)
		argN++
	}

	if len(whereConds) > 0 {
		whereClause = "WHERE " + joinConditions(whereConds)
	}
	args = append(args, pageSize+1)

	listQ := fmt.Sprintf(`
		SELECT id, source_id, status, job_type, priority, target_tags,
		       tasks_total, tasks_pending, tasks_running, tasks_completed, tasks_failed,
		       encode_config, max_retries, completed_at, failed_at, created_at, updated_at,
		       '' AS source_path
		FROM   job_archive
		%s
		ORDER BY created_at DESC
		LIMIT $%d`, whereClause, argN)

	rows, err := s.pool.Query(ctx, listQ, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("db: list archived jobs: %w", err)
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
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("db: list archived jobs rows: %w", err)
	}
	return out, total, nil
}

// ExportJobs returns all active jobs matching the filter (no pagination limit).
func (s *pgStore) ExportJobs(ctx context.Context, f ExportJobsFilter) ([]*Job, error) {
	return exportJobsFromTable(ctx, s.pool, "jobs", "sources", f)
}

// ExportArchivedJobs returns all archived jobs matching the filter.
func (s *pgStore) ExportArchivedJobs(ctx context.Context, f ExportJobsFilter) ([]*Job, error) {
	return exportJobsFromTable(ctx, s.pool, "job_archive", "", f)
}

// exportJobsFromTable is shared logic for ExportJobs and ExportArchivedJobs.
// When sourcesTable is empty, source_path is returned as an empty string.
func exportJobsFromTable(ctx context.Context, pool poolIface, table, sourcesTable string, f ExportJobsFilter) ([]*Job, error) {
	var (
		conds []string
		args  []any
		argN  = 1
	)

	if f.Status != "" {
		conds = append(conds, fmt.Sprintf("j.status = $%d", argN))
		args = append(args, f.Status)
		argN++
	}
	if !f.From.IsZero() {
		conds = append(conds, fmt.Sprintf("j.created_at >= $%d", argN))
		args = append(args, f.From)
		argN++
	}
	if !f.To.IsZero() {
		conds = append(conds, fmt.Sprintf("j.created_at <= $%d", argN))
		args = append(args, f.To)
		argN++
	}

	whereClause := ""
	if len(conds) > 0 {
		whereClause = "WHERE " + joinConditions(conds)
	}

	var q string
	if sourcesTable != "" {
		q = fmt.Sprintf(`
			SELECT j.id, j.source_id, j.status, j.job_type, j.priority, j.target_tags,
			       j.tasks_total, j.tasks_pending, j.tasks_running, j.tasks_completed, j.tasks_failed,
			       j.encode_config, j.max_retries, j.completed_at, j.failed_at, j.created_at, j.updated_at,
			       COALESCE(s.unc_path, '') AS source_path
			FROM   %s j LEFT JOIN %s s ON j.source_id = s.id
			%s
			ORDER BY j.created_at DESC`, table, sourcesTable, whereClause)
	} else {
		q = fmt.Sprintf(`
			SELECT id, source_id, status, job_type, priority, target_tags,
			       tasks_total, tasks_pending, tasks_running, tasks_completed, tasks_failed,
			       encode_config, max_retries, completed_at, failed_at, created_at, updated_at,
			       '' AS source_path
			FROM   %s j
			%s
			ORDER BY created_at DESC`, table, whereClause)
	}

	rows, err := pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("db: export jobs from %s: %w", table, err)
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
// Template Versions
// ---------------------------------------------------------------------------

func (s *pgStore) CreateTemplateVersion(ctx context.Context, p CreateTemplateVersionParams) (*TemplateVersion, error) {
	const q = `
		INSERT INTO template_versions (template_id, version, content, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, template_id, version, content, created_at, created_by`
	row := s.pool.QueryRow(ctx, q, p.TemplateID, p.Version, p.Content, p.CreatedBy)
	return scanTemplateVersion(row)
}

func (s *pgStore) ListTemplateVersions(ctx context.Context, templateID string) ([]*TemplateVersion, error) {
	const q = `SELECT id, template_id, version, content, created_at, created_by
	           FROM template_versions WHERE template_id = $1 ORDER BY version DESC`
	rows, err := s.pool.Query(ctx, q, templateID)
	if err != nil {
		return nil, fmt.Errorf("db: list template versions: %w", err)
	}
	defer rows.Close()
	var out []*TemplateVersion
	for rows.Next() {
		v, err := scanTemplateVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *pgStore) GetTemplateVersion(ctx context.Context, templateID string, version int) (*TemplateVersion, error) {
	const q = `SELECT id, template_id, version, content, created_at, created_by
	           FROM template_versions WHERE template_id = $1 AND version = $2`
	return scanTemplateVersion(s.pool.QueryRow(ctx, q, templateID, version))
}

// GetLatestTemplateVersion returns the highest version number for the given
// template, or 0 if no versions exist yet.
func (s *pgStore) GetLatestTemplateVersion(ctx context.Context, templateID string) (int, error) {
	const q = `SELECT COALESCE(MAX(version), 0) FROM template_versions WHERE template_id = $1`
	var v int
	if err := s.pool.QueryRow(ctx, q, templateID).Scan(&v); err != nil {
		return 0, fmt.Errorf("db: get latest template version: %w", err)
	}
	return v, nil
}

func scanTemplateVersion(row pgx.Row) (*TemplateVersion, error) {
	var v TemplateVersion
	err := row.Scan(&v.ID, &v.TemplateID, &v.Version, &v.Content, &v.CreatedAt, &v.CreatedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan template version: %w", err)
	}
	return &v, nil
}

// ---------------------------------------------------------------------------
// Task Preemption
// ---------------------------------------------------------------------------

// PreemptTask marks a running or assigned task as "preempted": it records the
// current time, resets the task to "pending" so it can be picked up again, and
// clears the agent assignment.
func (s *pgStore) PreemptTask(ctx context.Context, taskID string) error {
	const q = `
		UPDATE tasks
		SET status       = 'pending',
		    preempted_at = now(),
		    agent_id     = NULL,
		    started_at   = NULL,
		    updated_at   = now()
		WHERE id = $1
		  AND status IN ('assigned', 'running')`
	ct, err := s.pool.Exec(ctx, q, taskID)
	if err != nil {
		return fmt.Errorf("db: preempt task: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ResetTask resets an operator-specified task back to "pending" so the engine
// can reassign it.  Unlike PreemptTask (which only operates on running/assigned
// tasks), ResetTask can act on any non-terminal status.  Terminal statuses
// ("completed", "failed") are never touched — the caller must check those
// before calling this function.
func (s *pgStore) ResetTask(ctx context.Context, taskID string) error {
	const q = `
		UPDATE tasks
		SET status     = 'pending',
		    agent_id   = NULL,
		    started_at = NULL,
		    updated_at = now()
		WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, taskID)
	if err != nil {
		return fmt.Errorf("db: reset task: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Encoding Stats (cost estimation learning)
// ---------------------------------------------------------------------------

// UpsertEncodingStats updates the running average FPS, size-per-minute, and
// standard deviation for a (codec, resolution, preset) combination using an
// incremental mean / Welford's online variance algorithm.
func (s *pgStore) UpsertEncodingStats(ctx context.Context, p UpsertEncodingStatsParams) error {
	// Incremental Welford update expressed in SQL:
	//   new_mean = old_mean + (new_val - old_mean) / new_count
	//   We approximate stddev via (running sum-of-sq-diffs) stored as fps_stddev * sample_count.
	const q = `
		INSERT INTO encoding_stats (codec, resolution, preset, avg_fps, avg_size_per_min, sample_count, fps_stddev)
		VALUES ($1, $2, $3, $4, $5, 1, 0)
		ON CONFLICT (codec, resolution, preset) DO UPDATE SET
		    sample_count    = encoding_stats.sample_count + 1,
		    avg_fps         = encoding_stats.avg_fps + ($4 - encoding_stats.avg_fps) / (encoding_stats.sample_count + 1),
		    avg_size_per_min = encoding_stats.avg_size_per_min + ($5 - encoding_stats.avg_size_per_min) / (encoding_stats.sample_count + 1),
		    fps_stddev      = SQRT(GREATEST(0,
		                        (encoding_stats.fps_stddev * encoding_stats.fps_stddev * encoding_stats.sample_count
		                         + ($4 - encoding_stats.avg_fps) * ($4 - (encoding_stats.avg_fps + ($4 - encoding_stats.avg_fps) / (encoding_stats.sample_count + 1))))
		                        / (encoding_stats.sample_count + 1)
		                     )),
		    updated_at      = now()`
	_, err := s.pool.Exec(ctx, q, p.Codec, p.Resolution, p.Preset, p.NewFPS, p.NewSizePerMin)
	if err != nil {
		return fmt.Errorf("db: upsert encoding stats: %w", err)
	}
	return nil
}

func (s *pgStore) GetEncodingStats(ctx context.Context, codec, resolution, preset string) (*EncodingStats, error) {
	const q = `SELECT id, codec, resolution, preset, avg_fps, avg_size_per_min,
	                  sample_count, fps_stddev, updated_at
	           FROM encoding_stats WHERE codec = $1 AND resolution = $2 AND preset = $3`
	row := s.pool.QueryRow(ctx, q, codec, resolution, preset)
	var es EncodingStats
	err := row.Scan(&es.ID, &es.Codec, &es.Resolution, &es.Preset,
		&es.AvgFPS, &es.AvgSizePerMin, &es.SampleCount, &es.FPSStddev, &es.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: get encoding stats: %w", err)
	}
	return &es, nil
}

// ---------------------------------------------------------------------------
// Agent FPS (adaptive chunking)
// ---------------------------------------------------------------------------

// GetAgentAvgFPS returns the average encoding FPS of the most-recent 20
// completed encode tasks executed by the given agent.  Returns 0 when no
// history is available.
func (s *pgStore) GetAgentAvgFPS(ctx context.Context, agentID string) (float64, error) {
	const q = `
		SELECT COALESCE(AVG(avg_fps), 0)
		FROM (
		    SELECT avg_fps
		    FROM tasks
		    WHERE agent_id = $1
		      AND status = 'completed'
		      AND avg_fps IS NOT NULL
		      AND avg_fps > 0
		    ORDER BY completed_at DESC
		    LIMIT 20
		) sub`
	var fps float64
	if err := s.pool.QueryRow(ctx, q, agentID).Scan(&fps); err != nil {
		return 0, fmt.Errorf("db: get agent avg fps: %w", err)
	}
	return fps, nil
}

// ---------------------------------------------------------------------------
// Encoding Rules
// ---------------------------------------------------------------------------

func (s *pgStore) CreateEncodingRule(ctx context.Context, p CreateEncodingRuleParams) (*EncodingRule, error) {
	condsJSON, err := json.Marshal(p.Conditions)
	if err != nil {
		return nil, fmt.Errorf("db: marshal conditions: %w", err)
	}
	actionsJSON, err := json.Marshal(p.Actions)
	if err != nil {
		return nil, fmt.Errorf("db: marshal actions: %w", err)
	}
	const q = `
		INSERT INTO encoding_rules (name, priority, conditions, actions, enabled)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, priority, conditions, actions, enabled, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.Name, p.Priority, condsJSON, actionsJSON, p.Enabled)
	return scanEncodingRule(row)
}

func (s *pgStore) GetEncodingRuleByID(ctx context.Context, id string) (*EncodingRule, error) {
	const q = `SELECT id, name, priority, conditions, actions, enabled, created_at, updated_at
	           FROM encoding_rules WHERE id = $1`
	return scanEncodingRule(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) ListEncodingRules(ctx context.Context) ([]*EncodingRule, error) {
	const q = `SELECT id, name, priority, conditions, actions, enabled, created_at, updated_at
	           FROM encoding_rules ORDER BY priority ASC, name ASC`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db: list encoding rules: %w", err)
	}
	defer rows.Close()
	var out []*EncodingRule
	for rows.Next() {
		r, err := scanEncodingRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *pgStore) UpdateEncodingRule(ctx context.Context, p UpdateEncodingRuleParams) (*EncodingRule, error) {
	condsJSON, err := json.Marshal(p.Conditions)
	if err != nil {
		return nil, fmt.Errorf("db: marshal conditions: %w", err)
	}
	actionsJSON, err := json.Marshal(p.Actions)
	if err != nil {
		return nil, fmt.Errorf("db: marshal actions: %w", err)
	}
	const q = `UPDATE encoding_rules
	           SET name = $2, priority = $3, conditions = $4, actions = $5, enabled = $6, updated_at = now()
	           WHERE id = $1
	           RETURNING id, name, priority, conditions, actions, enabled, created_at, updated_at`
	return scanEncodingRule(s.pool.QueryRow(ctx, q, p.ID, p.Name, p.Priority, condsJSON, actionsJSON, p.Enabled))
}

func (s *pgStore) DeleteEncodingRule(ctx context.Context, id string) error {
	const q = `DELETE FROM encoding_rules WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("db: delete encoding rule: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanEncodingRule(row pgx.Row) (*EncodingRule, error) {
	var r EncodingRule
	var condsJSON, actionsJSON []byte
	err := row.Scan(
		&r.ID, &r.Name, &r.Priority, &condsJSON, &actionsJSON,
		&r.Enabled, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan encoding rule: %w", err)
	}
	if err := json.Unmarshal(condsJSON, &r.Conditions); err != nil {
		return nil, fmt.Errorf("db: unmarshal conditions: %w", err)
	}
	if err := json.Unmarshal(actionsJSON, &r.Actions); err != nil {
		return nil, fmt.Errorf("db: unmarshal actions: %w", err)
	}
	return &r, nil
}

// ---------------------------------------------------------------------------
// Sources — watch folder extensions
// ---------------------------------------------------------------------------

// UpdateSourceWatch updates the watch_folder and category fields on a source.
func (s *pgStore) UpdateSourceWatch(ctx context.Context, p UpdateSourceWatchParams) error {
	const q = `UPDATE sources SET watch_folder = $2, category = $3, updated_at = now() WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, p.ID, p.WatchFolder, p.Category)
	if err != nil {
		return fmt.Errorf("db: update source watch: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Encoding Profiles
// ---------------------------------------------------------------------------

func (s *pgStore) CreateEncodingProfile(ctx context.Context, p CreateEncodingProfileParams) (*EncodingProfile, error) {
	const q = `
		INSERT INTO encoding_profiles (name, description, container, settings, audio_config, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, description, container, settings, audio_config, created_by, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.Name, p.Description, p.Container, p.Settings, nullableJSON(p.AudioConfig), p.CreatedBy)
	return scanEncodingProfile(row)
}

func (s *pgStore) GetEncodingProfileByID(ctx context.Context, id string) (*EncodingProfile, error) {
	const q = `SELECT id, name, description, container, settings, audio_config, created_by, created_at, updated_at
	           FROM encoding_profiles WHERE id = $1`
	return scanEncodingProfile(s.pool.QueryRow(ctx, q, id))
}

func (s *pgStore) ListEncodingProfiles(ctx context.Context) ([]*EncodingProfile, error) {
	const q = `SELECT id, name, description, container, settings, audio_config, created_by, created_at, updated_at
	           FROM encoding_profiles ORDER BY name`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("db: list encoding profiles: %w", err)
	}
	defer rows.Close()
	var out []*EncodingProfile
	for rows.Next() {
		ep, err := scanEncodingProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ep)
	}
	return out, rows.Err()
}

func (s *pgStore) UpdateEncodingProfile(ctx context.Context, p UpdateEncodingProfileParams) (*EncodingProfile, error) {
	const q = `
		UPDATE encoding_profiles
		SET name = $2, description = $3, container = $4, settings = $5, audio_config = $6, updated_at = now()
		WHERE id = $1
		RETURNING id, name, description, container, settings, audio_config, created_by, created_at, updated_at`
	row := s.pool.QueryRow(ctx, q, p.ID, p.Name, p.Description, p.Container, p.Settings, nullableJSON(p.AudioConfig))
	return scanEncodingProfile(row)
}

func (s *pgStore) DeleteEncodingProfile(ctx context.Context, id string) error {
	const q = `DELETE FROM encoding_profiles WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("db: delete encoding profile: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanEncodingProfile(row pgx.Row) (*EncodingProfile, error) {
	var p EncodingProfile
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Container, &p.Settings, &p.AudioConfig, &p.CreatedBy, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("db: scan encoding profile: %w", err)
	}
	return &p, nil
}

// nullableJSON returns nil when b is empty, otherwise b — used to store
// optional JSONB columns without a zero-value byte slice.
func nullableJSON(b json.RawMessage) interface{} {
	if len(b) == 0 {
		return nil
	}
	return b
}

// ---------------------------------------------------------------------------
// Agent update channel
// ---------------------------------------------------------------------------

func (s *pgStore) UpdateAgentChannel(ctx context.Context, p UpdateAgentChannelParams) error {
	const q = `UPDATE agents SET update_channel = $2, updated_at = now() WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, p.ID, p.Channel)
	if err != nil {
		return fmt.Errorf("db: update agent channel: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Agent encoding stats (health deep-dive)
// ---------------------------------------------------------------------------

func (s *pgStore) GetAgentEncodingStats(ctx context.Context, agentID string) (*AgentEncodingStats, error) {
	const q = `
		SELECT
			$1::text AS agent_id,
			COUNT(*) AS total_tasks,
			COUNT(*) FILTER (WHERE status = 'completed') AS completed_tasks,
			COUNT(*) FILTER (WHERE status = 'failed') AS failed_tasks,
			COALESCE(AVG(avg_fps) FILTER (WHERE avg_fps IS NOT NULL AND avg_fps > 0), 0) AS avg_fps,
			COALESCE(SUM(frames_encoded), 0) AS total_frames
		FROM tasks
		WHERE agent_id = $1`
	var st AgentEncodingStats
	err := s.pool.QueryRow(ctx, q, agentID).Scan(
		&st.AgentID, &st.TotalTasks, &st.CompletedTasks, &st.FailedTasks,
		&st.AvgFPS, &st.TotalFrames,
	)
	if err != nil {
		return nil, fmt.Errorf("db: get agent encoding stats: %w", err)
	}
	return &st, nil
}

func (s *pgStore) ListRecentTasksByAgent(ctx context.Context, agentID string, limit int) ([]*Task, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	const q = `SELECT id, job_id, chunk_index, task_type, status, agent_id, script_dir,
	                  source_path, output_path, variables, exit_code,
	                  frames_encoded, avg_fps, output_size, duration_sec,
	                  vmaf_score, psnr, ssim, error_msg, retry_count, retry_after,
	                  preemptible, preempted_at, started_at, completed_at, created_at, updated_at
	           FROM tasks WHERE agent_id = $1 ORDER BY updated_at DESC LIMIT $2`
	rows, err := s.pool.Query(ctx, q, agentID, limit)
	if err != nil {
		return nil, fmt.Errorf("db: list recent tasks by agent: %w", err)
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

// ---------------------------------------------------------------------------
// API key rate limit
// ---------------------------------------------------------------------------

func (s *pgStore) UpdateAPIKeyRateLimit(ctx context.Context, p UpdateAPIKeyRateLimitParams) error {
	const q = `UPDATE api_keys SET rate_limit = $2 WHERE id = $1`
	ct, err := s.pool.Exec(ctx, q, p.ID, p.RateLimit)
	if err != nil {
		return fmt.Errorf("db: update api key rate limit: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Audit log extended
// ---------------------------------------------------------------------------

func (s *pgStore) ListAuditLogByUser(ctx context.Context, userID string, limit, offset int) ([]*AuditEntry, int, error) {
	const countQ = `SELECT COUNT(*) FROM audit_log WHERE user_id = $1`
	var total int
	if err := s.pool.QueryRow(ctx, countQ, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("db: count audit log by user: %w", err)
	}
	const q = `SELECT id, user_id, username, action, resource, resource_id, detail, ip_address, logged_at
	           FROM audit_log WHERE user_id = $1
	           ORDER BY logged_at DESC LIMIT $2 OFFSET $3`
	rows, err := s.pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("db: list audit log by user: %w", err)
	}
	defer rows.Close()
	var out []*AuditEntry
	for rows.Next() {
		e, err := scanAuditEntry(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func (s *pgStore) ExportAuditLog(ctx context.Context, limit int) ([]*AuditEntry, error) {
	if limit <= 0 || limit > 100000 {
		limit = 10000
	}
	const q = `SELECT id, user_id, username, action, resource, resource_id, detail, ip_address, logged_at
	           FROM audit_log ORDER BY logged_at DESC LIMIT $1`
	rows, err := s.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("db: export audit log: %w", err)
	}
	defer rows.Close()
	var out []*AuditEntry
	for rows.Next() {
		e, err := scanAuditEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Sessions extended
// ---------------------------------------------------------------------------

func (s *pgStore) ListSessionsByUser(ctx context.Context, userID string) ([]*Session, error) {
	const q = `SELECT token, user_id, created_at, expires_at
	           FROM sessions WHERE user_id = $1 AND expires_at > now()
	           ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("db: list sessions by user: %w", err)
	}
	defer rows.Close()
	var out []*Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// DeleteSessionByID removes a session by its token (used as the ID from the
// sessions management API).
func (s *pgStore) DeleteSessionByID(ctx context.Context, sessionID string) error {
	const q = `DELETE FROM sessions WHERE token = $1`
	ct, err := s.pool.Exec(ctx, q, sessionID)
	if err != nil {
		return fmt.Errorf("db: delete session by id: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
