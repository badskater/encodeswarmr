package notifications

import (
	"bufio"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"

	"github.com/badskater/encodeswarmr/internal/controller/config"
)

// ---------------------------------------------------------------------------
// NewEmailSender
// ---------------------------------------------------------------------------

func TestNewEmailSender_EmptyHost_ReturnsNil(t *testing.T) {
	cfg := config.SMTPConfig{Host: ""}
	sender := NewEmailSender(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if sender != nil {
		t.Errorf("NewEmailSender(empty host) = %v, want nil", sender)
	}
}

func TestNewEmailSender_ValidHost_ReturnsNonNil(t *testing.T) {
	cfg := config.SMTPConfig{Host: "smtp.example.com", Port: 25}
	sender := NewEmailSender(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if sender == nil {
		t.Fatal("NewEmailSender(valid host) = nil, want non-nil")
	}
}

func TestNewEmailSender_ConfigStored(t *testing.T) {
	cfg := config.SMTPConfig{
		Host:        "mail.example.com",
		Port:        587,
		Username:    "user@example.com",
		FromAddress: "noreply@example.com",
	}
	sender := NewEmailSender(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if sender == nil {
		t.Fatal("expected non-nil sender")
	}
	if sender.cfg.Host != cfg.Host {
		t.Errorf("cfg.Host = %q, want %q", sender.cfg.Host, cfg.Host)
	}
	if sender.cfg.Port != cfg.Port {
		t.Errorf("cfg.Port = %d, want %d", sender.cfg.Port, cfg.Port)
	}
}

// ---------------------------------------------------------------------------
// RenderJobCompleted
// ---------------------------------------------------------------------------

func TestRenderJobCompleted_ReturnsNonEmptyHTML(t *testing.T) {
	html, err := RenderJobCompleted("job-123", `\\nas\share\video.mkv`)
	if err != nil {
		t.Fatalf("RenderJobCompleted: %v", err)
	}
	if html == "" {
		t.Error("RenderJobCompleted returned empty string")
	}
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("expected HTML document in output")
	}
}

func TestRenderJobCompleted_ContainsJobID(t *testing.T) {
	jobID := "job-abc-123"
	html, err := RenderJobCompleted(jobID, "")
	if err != nil {
		t.Fatalf("RenderJobCompleted: %v", err)
	}
	if !strings.Contains(html, jobID) {
		t.Errorf("output does not contain job ID %q", jobID)
	}
}

func TestRenderJobCompleted_ContainsSourcePath(t *testing.T) {
	src := `\\nas\share\movie.mkv`
	html, err := RenderJobCompleted("j1", src)
	if err != nil {
		t.Fatalf("RenderJobCompleted: %v", err)
	}
	if !strings.Contains(html, src) {
		t.Errorf("output does not contain source path %q", src)
	}
}

func TestRenderJobCompleted_EmptySourcePath_NoError(t *testing.T) {
	html, err := RenderJobCompleted("j1", "")
	if err != nil {
		t.Fatalf("RenderJobCompleted with empty source: %v", err)
	}
	if html == "" {
		t.Error("expected non-empty HTML even with empty source path")
	}
}

// ---------------------------------------------------------------------------
// RenderJobFailed
// ---------------------------------------------------------------------------

func TestRenderJobFailed_ReturnsNonEmptyHTML(t *testing.T) {
	html, err := RenderJobFailed("job-999", `\\nas\share\fail.mkv`, "encoder crashed")
	if err != nil {
		t.Fatalf("RenderJobFailed: %v", err)
	}
	if html == "" {
		t.Error("RenderJobFailed returned empty string")
	}
}

func TestRenderJobFailed_ContainsJobIDAndDetail(t *testing.T) {
	jobID := "fail-job-42"
	detail := "exit code 1"
	html, err := RenderJobFailed(jobID, "", detail)
	if err != nil {
		t.Fatalf("RenderJobFailed: %v", err)
	}
	if !strings.Contains(html, jobID) {
		t.Errorf("output does not contain job ID %q", jobID)
	}
	if !strings.Contains(html, detail) {
		t.Errorf("output does not contain detail %q", detail)
	}
}

func TestRenderJobFailed_EmptyDetail_NoError(t *testing.T) {
	html, err := RenderJobFailed("j1", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if html == "" {
		t.Error("expected non-empty HTML")
	}
}

// ---------------------------------------------------------------------------
// RenderAgentStale
// ---------------------------------------------------------------------------

func TestRenderAgentStale_ReturnsNonEmptyHTML(t *testing.T) {
	html, err := RenderAgentStale("worker-01")
	if err != nil {
		t.Fatalf("RenderAgentStale: %v", err)
	}
	if html == "" {
		t.Error("RenderAgentStale returned empty string")
	}
}

func TestRenderAgentStale_ContainsAgentName(t *testing.T) {
	agentName := "encoder-node-7"
	html, err := RenderAgentStale(agentName)
	if err != nil {
		t.Fatalf("RenderAgentStale: %v", err)
	}
	if !strings.Contains(html, agentName) {
		t.Errorf("output does not contain agent name %q", agentName)
	}
}

func TestRenderAgentStale_ContainsHTMLDoc(t *testing.T) {
	html, err := RenderAgentStale("agent-x")
	if err != nil {
		t.Fatalf("RenderAgentStale: %v", err)
	}
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("expected HTML doctype in output")
	}
}

// ---------------------------------------------------------------------------
// buildMessage
// ---------------------------------------------------------------------------

func TestBuildMessage_ContainsHeaders(t *testing.T) {
	msg := buildMessage("from@example.com", "to@example.com", "Test Subject", "<p>body</p>")
	s := string(msg)

	checks := []string{
		"From: from@example.com",
		"To: to@example.com",
		"Subject: Test Subject",
		"MIME-Version: 1.0",
		"Content-Type: text/html",
		"<p>body</p>",
	}
	for _, want := range checks {
		if !strings.Contains(s, want) {
			t.Errorf("message missing %q", want)
		}
	}
}

// ---------------------------------------------------------------------------
// Send — mock SMTP server
// ---------------------------------------------------------------------------

// startMockSMTP starts a minimal SMTP server on localhost:0.
// It accepts one connection, reads EHLO/MAIL FROM/RCPT TO/DATA/QUIT, and
// returns the captured "MAIL FROM" and "RCPT TO" values via channels.
func startMockSMTP(t *testing.T) (addr string, mailFrom <-chan string, rcptTo <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("mock SMTP: listen: %v", err)
	}

	fromCh := make(chan string, 1)
	rcptCh := make(chan string, 1)

	go func() {
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		w := bufio.NewWriter(conn)
		r := bufio.NewReader(conn)

		// Greeting
		w.WriteString("220 localhost ESMTP\r\n")
		w.Flush()

		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			upper := strings.ToUpper(line)

			switch {
			case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
				w.WriteString("250 OK\r\n")
			case strings.HasPrefix(upper, "MAIL FROM:"):
				// Extract address from "MAIL FROM:<addr>"
				addr := strings.TrimPrefix(line, "MAIL FROM:")
				addr = strings.Trim(addr, "<>")
				fromCh <- addr
				w.WriteString("250 OK\r\n")
			case strings.HasPrefix(upper, "RCPT TO:"):
				addr := strings.TrimPrefix(line, "RCPT TO:")
				addr = strings.Trim(addr, "<>")
				rcptCh <- addr
				w.WriteString("250 OK\r\n")
			case strings.HasPrefix(upper, "DATA"):
				w.WriteString("354 Start input\r\n")
				w.Flush()
				// Read until \r\n.\r\n
				for {
					dataLine, err := r.ReadString('\n')
					if err != nil || strings.TrimSpace(dataLine) == "." {
						break
					}
				}
				w.WriteString("250 OK\r\n")
			case strings.HasPrefix(upper, "QUIT"):
				w.WriteString("221 Bye\r\n")
				w.Flush()
				return
			default:
				w.WriteString("500 Unknown command\r\n")
			}
			w.Flush()
		}
	}()

	return ln.Addr().String(), fromCh, rcptCh
}

func TestSend_PlainSMTP_MailFromRcptTo(t *testing.T) {
	addr, mailFromCh, rcptToCh := startMockSMTP(t)

	parts := strings.SplitN(addr, ":", 2)
	port := 0
	if len(parts) == 2 {
		for _, b := range parts[1] {
			if b >= '0' && b <= '9' {
				port = port*10 + int(b-'0')
			}
		}
	}

	cfg := config.SMTPConfig{
		Host:        "127.0.0.1",
		Port:        port,
		FromAddress: "sender@example.com",
		TLSEnabled:  false,
	}
	sender := NewEmailSender(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if sender == nil {
		t.Fatal("expected non-nil sender")
	}

	if err := sender.Send("recipient@example.com", "Test", "<p>Hello</p>"); err != nil {
		t.Fatalf("Send: %v", err)
	}

	from := <-mailFromCh
	if !strings.Contains(from, "sender@example.com") {
		t.Errorf("MAIL FROM = %q, want sender@example.com", from)
	}

	rcpt := <-rcptToCh
	if !strings.Contains(rcpt, "recipient@example.com") {
		t.Errorf("RCPT TO = %q, want recipient@example.com", rcpt)
	}
}

func TestSend_NilSender_NoError(t *testing.T) {
	var sender *EmailSender
	if err := sender.Send("to@example.com", "subj", "body"); err != nil {
		t.Errorf("nil sender Send: want nil error, got %v", err)
	}
}

func TestSend_UnreachableHost_ReturnsError(t *testing.T) {
	cfg := config.SMTPConfig{
		Host:        "127.0.0.1",
		Port:        1, // port 1 is not SMTP
		FromAddress: "from@example.com",
		TLSEnabled:  false,
	}
	sender := NewEmailSender(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := sender.Send("to@example.com", "subj", "body"); err == nil {
		t.Error("expected error connecting to port 1")
	}
}
