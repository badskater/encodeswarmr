//go:build integration

// Package testharness provides shared helpers for integration tests:
// Postgres setup via testcontainers or TEST_DATABASE_URL, fixture builders,
// and polling utilities.
package testharness

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	tc "github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/badskater/distributed-encoder/internal/db"
)

// SetupPostgres returns a DSN, a db.Store, and the underlying *pgxpool.Pool
// connected to a test Postgres instance.
//
// If TEST_DATABASE_URL is set, that DSN is used directly and no container is
// started.  Otherwise a throwaway Postgres 16 container is started via
// testcontainers-go.
//
// Migrations are applied (idempotent).  t.Cleanup is registered to close the
// pool (and terminate the container when one was started).
func SetupPostgres(t *testing.T) (dsn string, store db.Store, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	if envDSN := os.Getenv("TEST_DATABASE_URL"); envDSN != "" {
		dsn = envDSN
	} else {
		dsn = startContainer(t, ctx)
	}

	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("testharness: migrate: %v", err)
	}

	var err error
	store, pool, err = db.New(ctx, dsn)
	if err != nil {
		t.Fatalf("testharness: open db: %v", err)
	}

	t.Cleanup(pool.Close)
	return dsn, store, pool
}

// startContainer spins up a postgres:16-alpine testcontainer and returns its DSN.
// A t.Cleanup is registered to terminate the container.
func startContainer(t *testing.T, ctx context.Context) string {
	t.Helper()

	const (
		dbName = "distencoder_test"
		dbUser = "distencoder"
		dbPass = "test"
	)

	ctr, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase(dbName),
		tcpostgres.WithUsername(dbUser),
		tcpostgres.WithPassword(dbPass),
		tc.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("testharness: start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if terr := ctr.Terminate(ctx); terr != nil {
			t.Logf("testharness: terminate postgres container: %v", terr)
		}
	})

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("testharness: container host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("testharness: container port: %v", err)
	}

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPass, host, port.Port(), dbName,
	)
}

// TruncateAll removes all rows from application tables so each test starts
// with an empty slate.  Called at the top of every test function that uses
// the shared Postgres instance.
func TruncateAll(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	const q = `TRUNCATE
		agent_metrics,
		audit_log,
		webhook_deliveries,
		task_logs,
		tasks,
		jobs,
		sources,
		agents,
		sessions,
		users,
		templates,
		variables,
		webhooks,
		enrollment_tokens,
		analysis_results,
		path_mappings
	CASCADE`

	if _, err := pool.Exec(ctx, q); err != nil {
		t.Fatalf("testharness: truncate all: %v", err)
	}
}
