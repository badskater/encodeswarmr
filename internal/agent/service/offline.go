package service

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// offlineResult represents a task result buffered locally when the controller
// is unreachable.
type offlineResult struct {
	ID        int64
	TaskID    string
	JobID     string
	Success   bool
	ExitCode  int32
	ErrorMsg  string
	CreatedAt time.Time
}

// offlineLog represents a log line buffered locally when the controller
// is unreachable.
type offlineLog struct {
	ID        int64
	TaskID    string
	JobID     string
	Stream    string
	Level     string
	Message   string
	CreatedAt time.Time
}

// offlineStore wraps a SQLite database used to journal task results when the
// gRPC connection to the controller is unavailable.
type offlineStore struct {
	db *sql.DB
}

// newOfflineStore opens (or creates) the SQLite database at path and ensures
// the schema exists.
func newOfflineStore(path string) (*offlineStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening offline db: %w", err)
	}

	const resultsSchema = `CREATE TABLE IF NOT EXISTS offline_results (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id    TEXT    NOT NULL,
		job_id     TEXT    NOT NULL,
		success    INTEGER NOT NULL,
		exit_code  INTEGER NOT NULL,
		error_msg  TEXT    NOT NULL DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		synced     INTEGER NOT NULL DEFAULT 0
	)`
	if _, err := db.Exec(resultsSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating offline_results table: %w", err)
	}

	const logsSchema = `CREATE TABLE IF NOT EXISTS offline_logs (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id    TEXT    NOT NULL,
		job_id     TEXT    NOT NULL,
		stream     TEXT    NOT NULL,
		level      TEXT    NOT NULL DEFAULT 'info',
		message    TEXT    NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		synced     INTEGER NOT NULL DEFAULT 0
	)`
	if _, err := db.Exec(logsSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating offline_logs table: %w", err)
	}

	return &offlineStore{db: db}, nil
}

// saveResult inserts a task result into the offline journal.
func (o *offlineStore) saveResult(taskID, jobID string, success bool, exitCode int32, errorMsg string) error {
	const q = `INSERT INTO offline_results (task_id, job_id, success, exit_code, error_msg) VALUES (?, ?, ?, ?, ?)`
	successInt := 0
	if success {
		successInt = 1
	}
	_, err := o.db.Exec(q, taskID, jobID, successInt, exitCode, errorMsg)
	return err
}

// pendingResults returns all results that have not yet been synced to the
// controller.
func (o *offlineStore) pendingResults() ([]offlineResult, error) {
	const q = `SELECT id, task_id, job_id, success, exit_code, error_msg, created_at FROM offline_results WHERE synced = 0 ORDER BY id`
	rows, err := o.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []offlineResult
	for rows.Next() {
		var r offlineResult
		var successInt int
		if err := rows.Scan(&r.ID, &r.TaskID, &r.JobID, &successInt, &r.ExitCode, &r.ErrorMsg, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Success = successInt != 0
		results = append(results, r)
	}
	return results, rows.Err()
}

// markSynced flags a result as successfully delivered to the controller.
func (o *offlineStore) markSynced(id int64) error {
	_, err := o.db.Exec(`UPDATE offline_results SET synced = 1 WHERE id = ?`, id)
	return err
}

// saveLog inserts a log line into the offline journal.
func (o *offlineStore) saveLog(taskID, jobID, stream, level, message string) error {
	const q = `INSERT INTO offline_logs (task_id, job_id, stream, level, message) VALUES (?, ?, ?, ?, ?)`
	_, err := o.db.Exec(q, taskID, jobID, stream, level, message)
	return err
}

// pendingLogs returns all log lines not yet synced to the controller.
func (o *offlineStore) pendingLogs() ([]offlineLog, error) {
	const q = `SELECT id, task_id, job_id, stream, level, message, created_at FROM offline_logs WHERE synced = 0 ORDER BY id`
	rows, err := o.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []offlineLog
	for rows.Next() {
		var l offlineLog
		if err := rows.Scan(&l.ID, &l.TaskID, &l.JobID, &l.Stream, &l.Level, &l.Message, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// markLogsSynced flags log lines as delivered to the controller.
func (o *offlineStore) markLogsSynced(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	q := "UPDATE offline_logs SET synced = 1 WHERE id IN ("
	args := make([]any, len(ids))
	for i, id := range ids {
		if i > 0 {
			q += ","
		}
		q += "?"
		args[i] = id
	}
	q += ")"
	_, err := o.db.Exec(q, args...)
	return err
}

// close releases the database connection.
func (o *offlineStore) close() error {
	return o.db.Close()
}
