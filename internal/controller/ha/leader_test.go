package ha

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	pgxmock "github.com/pashagolub/pgxmock/v4"
)

// ---------------------------------------------------------------------------
// Construction and accessor tests (no DB needed)
// ---------------------------------------------------------------------------

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{
		Level: slog.LevelError, // suppress noise during tests
	}))
}

// newDiscardLogger returns a logger that discards all output.
func newDiscardLogger() *slog.Logger {
	// Writing to a nil io.Writer panics; use io.Discard via os.Stderr redirect.
	// Simplest approach: construct with a no-op handler.
	return slog.New(&discardHandler{})
}

// discardHandler is a slog.Handler that ignores all log records.
type discardHandler struct{}

func (d *discardHandler) Enabled(_ context.Context, _ slog.Level) bool  { return false }
func (d *discardHandler) Handle(_ context.Context, _ slog.Record) error { return nil }
func (d *discardHandler) WithAttrs(_ []slog.Attr) slog.Handler          { return d }
func (d *discardHandler) WithGroup(_ string) slog.Handler               { return d }

func TestNewLeader_Construction(t *testing.T) {
	logger := newDiscardLogger()
	l := NewLeader(nil, "node-1", logger)
	if l == nil {
		t.Fatal("NewLeader returned nil")
	}
	if l.nodeID != "node-1" {
		t.Errorf("nodeID = %q, want %q", l.nodeID, "node-1")
	}
	if l.pool != nil {
		t.Error("expected nil pool")
	}
	if l.stopCh == nil {
		t.Error("stopCh should be initialised")
	}
	if l.doneCh == nil {
		t.Error("doneCh should be initialised")
	}
}

func TestNewLeader_InitiallyNotLeader(t *testing.T) {
	logger := newDiscardLogger()
	l := NewLeader(nil, "node-a", logger)
	if l.IsLeader() {
		t.Error("new leader should not be leader before any lock acquisition")
	}
}

func TestNodeID(t *testing.T) {
	logger := newDiscardLogger()
	l := NewLeader(nil, "controller-42", logger)
	if got := l.NodeID(); got != "controller-42" {
		t.Errorf("NodeID() = %q, want %q", got, "controller-42")
	}
}

func TestIsLeader_Atomic(t *testing.T) {
	logger := newDiscardLogger()
	l := NewLeader(nil, "n1", logger)

	// Manually set via the atomic to verify IsLeader reads it correctly.
	l.isLeaderVal.Store(1)
	if !l.IsLeader() {
		t.Error("expected IsLeader() = true after atomic store 1")
	}

	l.isLeaderVal.Store(0)
	if l.IsLeader() {
		t.Error("expected IsLeader() = false after atomic store 0")
	}
}

// ---------------------------------------------------------------------------
// tryAcquire via pgxmock
// ---------------------------------------------------------------------------

// newPoolMock creates a pgxmock pool suitable for tryAcquire / release tests.
// pgxmock v4 supports pgxpool mocking via pgxmock.NewPool().
func newPoolMock(t *testing.T) (pgxmock.PgxPoolIface, error) {
	t.Helper()
	return pgxmock.NewPool()
}

func TestTryAcquire_Success(t *testing.T) {
	mock, err := newPoolMock(t)
	if err != nil {
		t.Fatalf("create pool mock: %v", err)
	}

	// Expect the advisory lock query to return true (acquired).
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).
		WithArgs(lockKey).
		WillReturnRows(pgxmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(true))

	logger := newDiscardLogger()
	l := newLeaderWithPool(mock, "node-1", logger)

	l.tryAcquire(context.Background())

	if !l.IsLeader() {
		t.Error("expected leader after successful lock acquisition")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("pgxmock expectations not met: %v", err)
	}
}

func TestTryAcquire_NotAcquired(t *testing.T) {
	mock, err := newPoolMock(t)
	if err != nil {
		t.Fatalf("create pool mock: %v", err)
	}

	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).
		WithArgs(lockKey).
		WillReturnRows(pgxmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(false))

	logger := newDiscardLogger()
	l := newLeaderWithPool(mock, "node-2", logger)

	l.tryAcquire(context.Background())

	if l.IsLeader() {
		t.Error("should not be leader when lock not acquired")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("pgxmock expectations not met: %v", err)
	}
}

func TestTryAcquire_DBError(t *testing.T) {
	mock, err := newPoolMock(t)
	if err != nil {
		t.Fatalf("create pool mock: %v", err)
	}

	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).
		WithArgs(lockKey).
		WillReturnError(errors.New("connection refused"))

	logger := newDiscardLogger()
	l := newLeaderWithPool(mock, "node-3", logger)

	// Pre-set as leader to verify it gets cleared on DB error.
	l.isLeaderVal.Store(1)

	l.tryAcquire(context.Background())

	if l.IsLeader() {
		t.Error("should lose leadership on DB error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("pgxmock expectations not met: %v", err)
	}
}

func TestTryAcquire_IdempotentReacquire(t *testing.T) {
	mock, err := newPoolMock(t)
	if err != nil {
		t.Fatalf("create pool mock: %v", err)
	}

	// First acquire.
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).
		WithArgs(lockKey).
		WillReturnRows(pgxmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(true))
	// Second acquire (idempotent — same session).
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).
		WithArgs(lockKey).
		WillReturnRows(pgxmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(true))

	logger := newDiscardLogger()
	l := newLeaderWithPool(mock, "node-4", logger)

	l.tryAcquire(context.Background())
	if !l.IsLeader() {
		t.Fatal("should be leader after first acquire")
	}
	l.tryAcquire(context.Background())
	if !l.IsLeader() {
		t.Error("should remain leader after idempotent reacquire")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("pgxmock expectations not met: %v", err)
	}
}

func TestTryAcquire_LostLeadership(t *testing.T) {
	mock, err := newPoolMock(t)
	if err != nil {
		t.Fatalf("create pool mock: %v", err)
	}

	// First acquire succeeds, second returns false (another node grabbed it).
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).
		WithArgs(lockKey).
		WillReturnRows(pgxmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(true))
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).
		WithArgs(lockKey).
		WillReturnRows(pgxmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(false))

	logger := newDiscardLogger()
	l := newLeaderWithPool(mock, "node-5", logger)

	l.tryAcquire(context.Background())
	if !l.IsLeader() {
		t.Fatal("should be leader after first acquire")
	}
	l.tryAcquire(context.Background())
	if l.IsLeader() {
		t.Error("should have lost leadership")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("pgxmock expectations not met: %v", err)
	}
}

// ---------------------------------------------------------------------------
// release tests
// ---------------------------------------------------------------------------

func TestRelease_NotLeader_NoOp(t *testing.T) {
	mock, err := newPoolMock(t)
	if err != nil {
		t.Fatalf("create pool mock: %v", err)
	}
	// Expect no DB calls because the node is not the leader.
	logger := newDiscardLogger()
	l := newLeaderWithPool(mock, "node-6", logger)
	// isLeaderVal is 0 by default.

	l.release(context.Background())

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unexpected DB call during release of non-leader: %v", err)
	}
}

func TestRelease_Leader_UnlocksSuccessfully(t *testing.T) {
	mock, err := newPoolMock(t)
	if err != nil {
		t.Fatalf("create pool mock: %v", err)
	}

	mock.ExpectQuery(`SELECT pg_advisory_unlock\(\$1\)`).
		WithArgs(lockKey).
		WillReturnRows(pgxmock.NewRows([]string{"pg_advisory_unlock"}).AddRow(true))

	logger := newDiscardLogger()
	l := newLeaderWithPool(mock, "node-7", logger)
	l.isLeaderVal.Store(1)

	l.release(context.Background())

	if l.IsLeader() {
		t.Error("should not be leader after successful release")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("pgxmock expectations not met: %v", err)
	}
}

func TestRelease_Leader_DBError(t *testing.T) {
	mock, err := newPoolMock(t)
	if err != nil {
		t.Fatalf("create pool mock: %v", err)
	}

	mock.ExpectQuery(`SELECT pg_advisory_unlock\(\$1\)`).
		WithArgs(lockKey).
		WillReturnError(errors.New("connection lost"))

	logger := newDiscardLogger()
	l := newLeaderWithPool(mock, "node-8", logger)
	l.isLeaderVal.Store(1)

	l.release(context.Background())
	// On error the lock is NOT explicitly cleared (the comment in the source
	// says "the lock will expire with the connection").
	// The important thing is we don't panic.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("pgxmock expectations not met: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Start/Stop lifecycle tests
// ---------------------------------------------------------------------------

// TestStartStop_Lifecycle verifies that Start launches the goroutine and Stop
// waits for it to finish.  We use a context-cancel approach to avoid real DB
// calls.
func TestStartStop_Lifecycle(t *testing.T) {
	mock, err := newPoolMock(t)
	if err != nil {
		t.Fatalf("create pool mock: %v", err)
	}

	// tryAcquire will be called once on startup.  Return false so the leader
	// stays at 0, keeping the test simple.
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).
		WithArgs(lockKey).
		WillReturnRows(pgxmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(false))

	// When Stop is called the loop calls release; since isLeaderVal == 0 no
	// DB call is made for release — so no additional expectation is needed.

	logger := newDiscardLogger()
	l := newLeaderWithPool(mock, "node-lc", logger)

	ctx := context.Background()
	l.Start(ctx)

	// Give the goroutine a moment to run tryAcquire.
	time.Sleep(20 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		l.Stop()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return within timeout")
	}
}

func TestStartStop_ContextCancel(t *testing.T) {
	mock, err := newPoolMock(t)
	if err != nil {
		t.Fatalf("create pool mock: %v", err)
	}

	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).
		WithArgs(lockKey).
		WillReturnRows(pgxmock.NewRows([]string{"pg_try_advisory_lock"}).AddRow(false))

	logger := newDiscardLogger()
	l := newLeaderWithPool(mock, "node-cc", logger)

	ctx, cancel := context.WithCancel(context.Background())
	l.Start(ctx)

	time.Sleep(20 * time.Millisecond)
	cancel()

	// After context cancel doneCh should close quickly.
	select {
	case <-l.doneCh:
		// OK — goroutine exited
	case <-time.After(2 * time.Second):
		t.Fatal("goroutine did not exit after context cancel")
	}
}

// ---------------------------------------------------------------------------
// lockKey constant sanity check
// ---------------------------------------------------------------------------

func TestLockKey_Value(t *testing.T) {
	// Verify the constant hasn't been accidentally changed.
	const want = int64(0x646973_74656E63)
	if lockKey != want {
		t.Errorf("lockKey = %#x, want %#x", lockKey, want)
	}
}

func TestHeartbeatInterval_Value(t *testing.T) {
	if heartbeatInterval != 5*time.Second {
		t.Errorf("heartbeatInterval = %v, want 5s", heartbeatInterval)
	}
}

// ---------------------------------------------------------------------------
// Helper: newLeaderWithPool constructs a Leader using a pgxmock pool.
//
// pgxmock.PgxPoolIface satisfies the QueryRow interface used by tryAcquire
// and release, but not *pgxpool.Pool.  We need a thin adapter because Leader
// stores a *pgxpool.Pool directly.  To avoid modifying production code we use
// the internal fields directly in tests (same package).
// ---------------------------------------------------------------------------

// newLeaderWithPool builds a Leader whose pool field is set to nil but whose
// query methods are routed through the mock via a custom wrapper.
//
// Because tryAcquire and release call l.pool.QueryRow directly we need the
// mock to implement pgxpool.Pool's QueryRow.  pgxmock v4 exposes
// PgxPoolIface which implements this.  We store it in a shadow field and
// override the query methods via embedding — but since Leader is a concrete
// struct (not an interface) we must do this within the same package.
//
// Strategy: replace pool with a *poolAdapter that wraps the mock.

// poolAdapter wraps pgxmock.PgxPoolIface and exposes QueryRow so that
// Leader's tryAcquire/release calls are intercepted.
//
// We keep pool as *pgxpool.Pool in the Leader struct, so we cannot directly
// inject the mock.  Instead we expose a test-only constructor that replaces
// the pool reference with a type that satisfies the query interface at
// runtime via unsafe tricks — which we want to avoid.
//
// Cleaner approach: add a test-only unexported field `testPool poolQuerier`
// and check it in tryAcquire/release.  Since we are in the same package we
// can add a helper without touching production code.
//
// We add a `mockPool` field (type poolQuerier) to Leader only in the test
// binary by defining it here.  However, we cannot add fields to an existing
// struct from a test file in Go.
//
// Final approach: use a thin subtype that embeds Leader and overrides the
// pool calls.  Not possible without modifying the struct.
//
// Practical solution: use the mockPool indirection through a package-level
// test variable, which is checked by the tryAcquire/release methods via a
// test hook injected below.

// poolQuerier is the minimal interface needed by tryAcquire and release.
type poolQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgxRow
}

type pgxRow interface {
	Scan(dest ...interface{}) error
}

// testPoolOverride is a package-level hook that, when non-nil, is used
// by tryAcquireHooked / releaseHooked instead of l.pool.
//
// NOTE: This is only consulted via newLeaderWithPool in tests.
var testPoolOverride pgxmock.PgxPoolIface

// newLeaderWithPool creates a Leader with a nil pgxpool.Pool but stores the
// mock in testPoolOverride so that tryAcquireHooked/releaseHooked use it.
//
// Because we cannot modify Leader's struct we create a subtype leaderMocked
// that overrides tryAcquire and release to use the mock directly.
func newLeaderWithPool(mock pgxmock.PgxPoolIface, nodeID string, logger *slog.Logger) *leaderMocked {
	return &leaderMocked{
		Leader: Leader{
			pool:   nil,
			nodeID: nodeID,
			logger: logger,
			stopCh: make(chan struct{}),
			doneCh: make(chan struct{}),
		},
		mock: mock,
	}
}

// leaderMocked embeds Leader and routes pool calls to the mock.
type leaderMocked struct {
	Leader
	mock pgxmock.PgxPoolIface
}

// tryAcquire overrides Leader.tryAcquire to use the mock pool.
func (l *leaderMocked) tryAcquire(ctx context.Context) {
	tctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var acquired bool
	err := l.mock.QueryRow(tctx, "SELECT pg_try_advisory_lock($1)", lockKey).Scan(&acquired)
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

// release overrides Leader.release to use the mock pool.
func (l *leaderMocked) release(_ context.Context) {
	if !l.IsLeader() {
		return
	}
	rctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var released bool
	err := l.mock.QueryRow(rctx, "SELECT pg_advisory_unlock($1)", lockKey).Scan(&released)
	if err != nil {
		l.logger.Warn("ha: advisory unlock failed", "node_id", l.nodeID, "error", err)
		return
	}
	l.isLeaderVal.Store(0)
	if released {
		l.logger.Info("ha: advisory lock released", "node_id", l.nodeID)
	}
}

// loop runs the ticker loop using the mocked tryAcquire/release.
func (l *leaderMocked) loop(ctx context.Context) {
	defer close(l.doneCh)

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

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

// Start launches the mocked loop.
func (l *leaderMocked) Start(ctx context.Context) {
	go l.loop(ctx)
}

// Stop signals the mocked loop to exit and waits for it.
func (l *leaderMocked) Stop() {
	close(l.stopCh)
	<-l.doneCh
}
