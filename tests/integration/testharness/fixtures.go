//go:build integration

package testharness

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"os"
	"testing"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/auth"
	"github.com/badskater/encodeswarmr/internal/controller/config"
	"github.com/badskater/encodeswarmr/internal/db"
)

// CreateAdminUser creates an admin user via the db.Store directly, hashes the
// password, and then logs in through auth.Service to obtain a session token.
// Returns the created user and session token.
//
// The username is derived from t.Name() to ensure uniqueness across parallel
// tests that share the same database instance.
func CreateAdminUser(t *testing.T, store db.Store, authSvc *auth.Service) (*db.User, string) {
	t.Helper()
	ctx := context.Background()

	// Use a short deterministic suffix so each parallel test gets a unique user.
	username := "admin-" + shortID()
	email := username + "@test.local"
	const password = "testpassword1"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("fixtures: hash password: %v", err)
	}

	user, err := store.CreateUser(ctx, db.CreateUserParams{
		Username:     username,
		Email:        email,
		Role:         "admin",
		PasswordHash: &hash,
	})
	if err != nil {
		t.Fatalf("fixtures: create admin user: %v", err)
	}

	sess, err := authSvc.Login(ctx, username, password)
	if err != nil {
		t.Fatalf("fixtures: login admin: %v", err)
	}

	return user, sess.Token
}

// NewAuthService creates a minimal auth.Service suitable for tests.
// OIDC is disabled; SessionTTL is set to 24 hours.
func NewAuthService(t *testing.T, store db.Store) *auth.Service {
	t.Helper()
	ctx := context.Background()
	cfg := &config.AuthConfig{
		SessionTTL: 24 * time.Hour,
		OIDC:       config.OIDCConfig{Enabled: false},
	}
	svc, err := auth.NewService(ctx, store, cfg, slog.Default())
	if err != nil {
		t.Fatalf("fixtures: new auth service: %v", err)
	}
	return svc
}

// CreateTestSource inserts a source row with sensible defaults and returns it.
func CreateTestSource(t *testing.T, store db.Store) *db.Source {
	t.Helper()
	ctx := context.Background()

	src, err := store.CreateSource(ctx, db.CreateSourceParams{
		Filename:  fmt.Sprintf("test_%s.mkv", shortID()),
		UNCPath:   fmt.Sprintf(`\\nas01\media\test_%s.mkv`, shortID()),
		SizeBytes: 1024 * 1024 * 500, // 500 MiB
	})
	if err != nil {
		t.Fatalf("fixtures: create source: %v", err)
	}
	return src
}

// CreateTestJob inserts a job for the given source and returns it.
func CreateTestJob(t *testing.T, store db.Store, sourceID string) *db.Job {
	t.Helper()
	ctx := context.Background()

	job, err := store.CreateJob(ctx, db.CreateJobParams{
		SourceID:   sourceID,
		JobType:    "encode",
		Priority:   0,
		TargetTags: []string{},
		EncodeConfig: db.EncodeConfig{
			OutputRoot:      `\\nas01\output`,
			OutputExtension: "mkv",
		},
	})
	if err != nil {
		t.Fatalf("fixtures: create job: %v", err)
	}
	return job
}

// CreateTestTemplate inserts a template and returns it.
func CreateTestTemplate(t *testing.T, store db.Store) *db.Template {
	t.Helper()
	ctx := context.Background()

	tmpl, err := store.CreateTemplate(ctx, db.CreateTemplateParams{
		Name:        fmt.Sprintf("test-template-%s", shortID()),
		Description: "Integration test template",
		Type:        "run_script",
		Extension:   "bat",
		Content:     "@echo off\necho hello",
	})
	if err != nil {
		t.Fatalf("fixtures: create template: %v", err)
	}
	return tmpl
}

// AuthenticatedClient returns an *http.Client carrying the given session token
// as a cookie named "session".
func AuthenticatedClient(t *testing.T, baseURL, sessionToken string) *http.Client {
	t.Helper()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("fixtures: cookiejar: %v", err)
	}

	client := &http.Client{Jar: jar}

	// Pre-seed the cookie jar by parsing a synthetic Set-Cookie header so
	// the client sends the session cookie on every request to baseURL.
	req, err := http.NewRequest(http.MethodGet, baseURL, nil)
	if err != nil {
		t.Fatalf("fixtures: build base request: %v", err)
	}

	cookieHeader := fmt.Sprintf("session=%s; Path=/", sessionToken)
	resp := &http.Response{
		Header:  make(http.Header),
		Request: req,
	}
	resp.Header.Set("Set-Cookie", cookieHeader)
	jar.SetCookies(req.URL, resp.Cookies())

	return client
}

// WriteTaskScript creates a minimal run script in scriptDir that exits with
// the given code.  On Windows-style tests the file is named run.bat; when
// the current OS is not Windows it is named run.sh.
//
// The function creates scriptDir if it does not already exist.
func WriteTaskScript(t *testing.T, scriptDir string, exitCode int) {
	t.Helper()

	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("fixtures: mkdirall scriptdir: %v", err)
	}

	batPath := scriptDir + "/run.bat"
	batContent := fmt.Sprintf("@echo off\nexit /b %d\n", exitCode)
	if err := os.WriteFile(batPath, []byte(batContent), 0o644); err != nil {
		t.Fatalf("fixtures: write run.bat: %v", err)
	}

	shPath := scriptDir + "/run.sh"
	shContent := fmt.Sprintf("#!/bin/sh\nexit %d\n", exitCode)
	if err := os.WriteFile(shPath, []byte(shContent), 0o644); err != nil {
		t.Fatalf("fixtures: write run.sh: %v", err)
	}
}

// WaitFor polls fn every 100 ms until it returns true or timeout is reached.
// If timeout is reached the test is failed with a descriptive message.
func WaitFor(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("fixtures: WaitFor timed out after %s", timeout)
}

// WaitForJobStatus polls the DB until the job with jobID reaches status or
// timeout expires.
func WaitForJobStatus(t *testing.T, store db.Store, jobID, status string, timeout time.Duration) {
	t.Helper()
	ctx := context.Background()
	WaitFor(t, timeout, func() bool {
		job, err := store.GetJobByID(ctx, jobID)
		if err != nil {
			return false
		}
		return job.Status == status
	})
}

// WaitForAgentStatus polls the DB until an agent with agentName reaches
// status or timeout expires.
func WaitForAgentStatus(t *testing.T, store db.Store, agentName, status string, timeout time.Duration) {
	t.Helper()
	ctx := context.Background()
	WaitFor(t, timeout, func() bool {
		agent, err := store.GetAgentByName(ctx, agentName)
		if err != nil {
			return false
		}
		return agent.Status == status
	})
}

// shortID returns a short random hex string suitable for unique test names.
func shortID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "xxxx"
	}
	return hex.EncodeToString(b)
}
