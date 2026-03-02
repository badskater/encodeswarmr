// Package sqlite provides the agent-side offline journal.
//
// When the agent cannot reach the controller it writes completed task results
// and buffered progress updates to a local SQLite database.  On reconnect the
// agent replays the journal through the gRPC SyncOfflineResults RPC.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	// Pure-Go SQLite driver — no CGO required.
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS offline_results (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id      TEXT    NOT NULL,
    job_id       TEXT    NOT NULL,
    success      INTEGER NOT NULL DEFAULT 0,
    exit_code    INTEGER NOT NULL DEFAULT 0,
    error_msg    TEXT    NOT NULL DEFAULT '',
    metrics_json TEXT    NOT NULL DEFAULT '{}',
    started_at   TEXT    NOT NULL,
    completed_at TEXT    NOT NULL,
    synced       INTEGER NOT NULL DEFAULT 0,
    created_at   TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_offline_results_unsynced
    ON offline_results (synced) WHERE synced = 0;

CREATE TABLE IF NOT EXISTS offline_logs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id    TEXT    NOT NULL,
    job_id     TEXT    NOT NULL,
    stream     TEXT    NOT NULL DEFAULT 'stdout',
    level      TEXT    NOT NULL DEFAULT 'info',
    message    TEXT    NOT NULL,
    metadata   TEXT,
    logged_at  TEXT    NOT NULL,
    synced     INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_offline_logs_unsynced
    ON offline_logs (synced) WHERE synced = 0;
`

// Journal is the agent-side offline store.
type Journal struct {
	db *sql.DB
}

// OfflineResult holds a task result entry pending sync.
type OfflineResult struct {
	ID          int64
	TaskID      string
	JobID       string
	Success     bool
	ExitCode    int
	ErrorMsg    string
	Metrics     map[string]any
	StartedAt   time.Time
	CompletedAt time.Time
}

// OfflineLog holds a log entry pending sync.
type OfflineLog struct {
	ID       int64
	TaskID   string
	JobID    string
	Stream   string
	Level    string
	Message  string
	Metadata map[string]any
	LoggedAt time.Time
}

// Open opens (or creates) a SQLite journal at the given path and applies the
// schema.
func Open(path string) (*Journal, error) {
	db, err := sql.Open("sqlite", path+"?_journal=WAL&_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %s: %w", path, err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: apply schema: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite: one writer at a time
	return &Journal{db: db}, nil
}

// Close closes the underlying database connection.
func (j *Journal) Close() error {
	return j.db.Close()
}

// WriteResult persists a completed task result to the journal.
func (j *Journal) WriteResult(ctx context.Context, r OfflineResult) error {
	metricsJSON, err := json.Marshal(r.Metrics)
	if err != nil {
		return fmt.Errorf("sqlite: marshal metrics: %w", err)
	}
	const q = `INSERT INTO offline_results
	               (task_id, job_id, success, exit_code, error_msg,
	                metrics_json, started_at, completed_at)
	           VALUES (?,?,?,?,?,?,?,?)`
	_, err = j.db.ExecContext(ctx, q,
		r.TaskID, r.JobID,
		boolToInt(r.Success), r.ExitCode, r.ErrorMsg,
		string(metricsJSON),
		r.StartedAt.UTC().Format(time.RFC3339Nano),
		r.CompletedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("sqlite: write result: %w", err)
	}
	return nil
}

// WriteLog persists a single log line to the journal.
func (j *Journal) WriteLog(ctx context.Context, l OfflineLog) error {
	var metaJSON *string
	if l.Metadata != nil {
		b, err := json.Marshal(l.Metadata)
		if err != nil {
			return fmt.Errorf("sqlite: marshal log metadata: %w", err)
		}
		s := string(b)
		metaJSON = &s
	}
	const q = `INSERT INTO offline_logs (task_id, job_id, stream, level, message, metadata, logged_at)
	           VALUES (?,?,?,?,?,?,?)`
	_, err := j.db.ExecContext(ctx, q,
		l.TaskID, l.JobID, l.Stream, l.Level, l.Message, metaJSON,
		l.LoggedAt.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("sqlite: write log: %w", err)
	}
	return nil
}

// UnsynedResults returns all result rows that have not yet been confirmed by
// the controller.
func (j *Journal) UnsyncedResults(ctx context.Context) ([]OfflineResult, error) {
	const q = `SELECT id, task_id, job_id, success, exit_code, error_msg,
	                  metrics_json, started_at, completed_at
	           FROM offline_results WHERE synced = 0 ORDER BY id`
	rows, err := j.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list unsynced results: %w", err)
	}
	defer rows.Close()

	var out []OfflineResult
	for rows.Next() {
		var (
			r           OfflineResult
			successInt  int
			metricsJSON string
			startedStr  string
			completedStr string
		)
		if err := rows.Scan(
			&r.ID, &r.TaskID, &r.JobID,
			&successInt, &r.ExitCode, &r.ErrorMsg,
			&metricsJSON, &startedStr, &completedStr,
		); err != nil {
			return nil, fmt.Errorf("sqlite: scan result: %w", err)
		}
		r.Success = successInt != 0
		if err := json.Unmarshal([]byte(metricsJSON), &r.Metrics); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal metrics: %w", err)
		}
		r.StartedAt, _ = time.Parse(time.RFC3339Nano, startedStr)
		r.CompletedAt, _ = time.Parse(time.RFC3339Nano, completedStr)
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkResultSynced marks a result row as confirmed by the controller.
func (j *Journal) MarkResultSynced(ctx context.Context, id int64) error {
	const q = `UPDATE offline_results SET synced = 1 WHERE id = ?`
	_, err := j.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("sqlite: mark result synced: %w", err)
	}
	return nil
}

// MarkLogsSynced marks all log rows for the given task as synced.
func (j *Journal) MarkLogsSynced(ctx context.Context, taskID string) error {
	const q = `UPDATE offline_logs SET synced = 1 WHERE task_id = ? AND synced = 0`
	_, err := j.db.ExecContext(ctx, q, taskID)
	if err != nil {
		return fmt.Errorf("sqlite: mark logs synced: %w", err)
	}
	return nil
}

// Prunesynced removes rows that have already been confirmed.  Call
// periodically (e.g. once per hour) to keep the journal file small.
func (j *Journal) PruneSynced(ctx context.Context) error {
	if _, err := j.db.ExecContext(ctx, `DELETE FROM offline_results WHERE synced = 1`); err != nil {
		return fmt.Errorf("sqlite: prune results: %w", err)
	}
	if _, err := j.db.ExecContext(ctx, `DELETE FROM offline_logs WHERE synced = 1`); err != nil {
		return fmt.Errorf("sqlite: prune logs: %w", err)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
