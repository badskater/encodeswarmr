// Package ha implements active-passive high-availability for the controller.
//
// Leader election is performed via a PostgreSQL session-level advisory lock.
// Only one controller instance holds the lock at any time; all others remain
// in standby and will acquire the lock as soon as the current leader's
// connection drops or its heartbeat goroutine explicitly releases it.
//
// Advisory lock key: a fixed int64 derived from the application name so it
// never collides with application-level locks.
//
// Usage:
//
//	l := ha.NewLeader(pool, nodeID, logger)
//	l.Start(ctx)          // non-blocking; begins heartbeat loop
//	defer l.Stop()
//	if l.IsLeader() { ... }
package ha

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// lockKey is the fixed PostgreSQL advisory lock key used cluster-wide.
// Value is derived from the ASCII sum of "encodeswarmr" to avoid collisions.
const lockKey = int64(0x646973_74656E63) // "distenc"

// heartbeatInterval is how often the leader attempts to re-confirm the lock.
const heartbeatInterval = 5 * time.Second

// Leader manages advisory-lock-based leader election for a controller node.
type Leader struct {
	pool   *pgxpool.Pool
	nodeID string
	logger *slog.Logger

	// isLeaderVal is 1 when this node holds the advisory lock, 0 otherwise.
	// All reads and writes use atomic operations so IsLeader() is lock-free.
	isLeaderVal atomic.Int32

	stopCh chan struct{}
	doneCh chan struct{}
}

// NewLeader creates a Leader that uses pool for advisory lock operations.
// nodeID should be unique per controller instance (e.g. hostname or UUID).
func NewLeader(pool *pgxpool.Pool, nodeID string, logger *slog.Logger) *Leader {
	return &Leader{
		pool:   pool,
		nodeID: nodeID,
		logger: logger,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// IsLeader reports whether this node currently holds the leader lock.
// Safe to call from any goroutine without blocking.
func (l *Leader) IsLeader() bool {
	return l.isLeaderVal.Load() == 1
}

// NodeID returns the unique identifier for this controller node.
func (l *Leader) NodeID() string {
	return l.nodeID
}

// Start launches the background heartbeat loop and returns immediately.
// The loop runs until Stop is called or ctx is cancelled.
func (l *Leader) Start(ctx context.Context) {
	go l.loop(ctx)
}

// Stop signals the heartbeat loop to exit and waits for it to finish.
// It also releases the advisory lock if held, allowing standby nodes to
// acquire leadership without waiting for a connection timeout.
func (l *Leader) Stop() {
	close(l.stopCh)
	<-l.doneCh
}

// loop is the internal heartbeat goroutine. It attempts to acquire (or
// re-confirm) the advisory lock every heartbeatInterval. On each iteration:
//   - If not yet leader: call pg_try_advisory_lock; on success, log and set flag.
//   - If already leader: call pg_try_advisory_lock again; the function is
//     idempotent for the same session, so it returns true while the connection
//     is healthy. On failure, log and clear the flag.
//
// The lock is session-scoped, so it is automatically released when the
// pgxpool connection is closed (e.g. on controller shutdown or crash).
func (l *Leader) loop(ctx context.Context) {
	defer close(l.doneCh)

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	// Attempt acquisition immediately on startup rather than waiting one full
	// heartbeat interval.
	l.tryAcquire(ctx)

	for {
		select {
		case <-l.stopCh:
			l.release(ctx)
			return
		case <-ctx.Done():
			l.release(ctx)
			return
		case <-ticker.C:
			l.tryAcquire(ctx)
		}
	}
}

// tryAcquire calls pg_try_advisory_lock. If the lock is acquired (or
// already held by this session) the leader flag is set. If the call fails
// or returns false, the leader flag is cleared.
func (l *Leader) tryAcquire(ctx context.Context) {
	tctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var acquired bool
	err := l.pool.QueryRow(tctx, "SELECT pg_try_advisory_lock($1)", lockKey).Scan(&acquired)
	if err != nil {
		l.logger.Error("ha: advisory lock query failed", "node_id", l.nodeID, "error", err)
		if l.isLeaderVal.Swap(0) == 1 {
			l.logger.Warn("ha: lost leadership due to DB error", "node_id", l.nodeID)
		}
		return
	}

	if acquired {
		if l.isLeaderVal.Swap(1) == 0 {
			l.logger.Info("ha: became leader", "node_id", l.nodeID)
		}
	} else {
		if l.isLeaderVal.Swap(0) == 1 {
			l.logger.Warn("ha: lost leadership (lock held by another node)", "node_id", l.nodeID)
		}
	}
}

// release explicitly unlocks the advisory lock so standby nodes can promote
// immediately. Errors are logged but not fatal — the lock will be released
// automatically when the underlying connection closes.
//
// A fresh Background context is used intentionally: the parent ctx may
// already be cancelled at the point release is called (shutdown path).
func (l *Leader) release(_ context.Context) {
	if !l.IsLeader() {
		return
	}
	rctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var released bool
	err := l.pool.QueryRow(rctx, "SELECT pg_advisory_unlock($1)", lockKey).Scan(&released)
	if err != nil {
		l.logger.Warn("ha: advisory unlock failed (lock will expire with connection)", "node_id", l.nodeID, "error", err)
		return
	}
	l.isLeaderVal.Store(0)
	if released {
		l.logger.Info("ha: advisory lock released", "node_id", l.nodeID)
	}
}
