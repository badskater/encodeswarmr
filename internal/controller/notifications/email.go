// Package notifications provides email delivery for system events.
package notifications

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"log/slog"
	"net/smtp"

	"github.com/badskater/encodeswarmr/internal/controller/config"
)

// EmailSender delivers HTML email messages via SMTP.
type EmailSender struct {
	cfg    config.SMTPConfig
	logger *slog.Logger
}

// NewEmailSender creates an EmailSender using the provided SMTP configuration.
// Returns nil when the SMTP host is empty (email disabled).
func NewEmailSender(cfg config.SMTPConfig, logger *slog.Logger) *EmailSender {
	if cfg.Host == "" {
		return nil
	}
	return &EmailSender{cfg: cfg, logger: logger}
}

// Send delivers a single HTML email to the given recipient.
func (e *EmailSender) Send(to, subject, body string) error {
	if e == nil {
		return nil
	}
	addr := fmt.Sprintf("%s:%d", e.cfg.Host, e.cfg.Port)

	msg := buildMessage(e.cfg.FromAddress, to, subject, body)

	var auth smtp.Auth
	if e.cfg.Username != "" {
		auth = smtp.PlainAuth("", e.cfg.Username, e.cfg.Password, e.cfg.Host)
	}

	var err error
	if e.cfg.TLSEnabled {
		err = sendTLS(addr, e.cfg.Host, auth, e.cfg.FromAddress, to, msg)
	} else {
		err = smtp.SendMail(addr, auth, e.cfg.FromAddress, []string{to}, msg)
	}
	if err != nil {
		e.logger.Warn("email: send failed",
			slog.String("to", to),
			slog.String("subject", subject),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("email: send: %w", err)
	}
	e.logger.Info("email: sent",
		slog.String("to", to),
		slog.String("subject", subject),
	)
	return nil
}

// sendTLS establishes an implicit TLS connection (port 465) and delivers mail.
func sendTLS(addr, host string, auth smtp.Auth, from, to string, msg []byte) error {
	tlsCfg := &tls.Config{ServerName: host}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer c.Quit() //nolint:errcheck

	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := wc.Write(msg); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	return wc.Close()
}

// buildMessage constructs an RFC 2822 MIME message with an HTML body.
func buildMessage(from, to, subject, htmlBody string) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: text/html; charset=UTF-8\r\n")
	fmt.Fprintf(&buf, "\r\n")
	fmt.Fprintf(&buf, "%s", htmlBody)
	return buf.Bytes()
}

// ---------------------------------------------------------------------------
// HTML email templates
// ---------------------------------------------------------------------------

// templateData holds the dynamic fields injected into all email templates.
type templateData struct {
	JobID      string
	SourcePath string
	Status     string
	Detail     string
	AgentName  string
}

var (
	tmplJobCompleted = template.Must(template.New("job_completed").Parse(`<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;color:#333;max-width:600px;margin:0 auto;padding:20px">
  <h2 style="color:#16a34a">Job Completed</h2>
  <p>Job <strong>{{.JobID}}</strong> has completed successfully.</p>
  {{if .SourcePath}}<p><strong>Source:</strong> {{.SourcePath}}</p>{{end}}
  <hr style="border:none;border-top:1px solid #e5e7eb;margin:20px 0">
  <p style="color:#6b7280;font-size:12px">Sent by EncodeSwarmr</p>
</body>
</html>`))

	tmplJobFailed = template.Must(template.New("job_failed").Parse(`<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;color:#333;max-width:600px;margin:0 auto;padding:20px">
  <h2 style="color:#dc2626">Job Failed</h2>
  <p>Job <strong>{{.JobID}}</strong> has failed.</p>
  {{if .SourcePath}}<p><strong>Source:</strong> {{.SourcePath}}</p>{{end}}
  {{if .Detail}}<p><strong>Detail:</strong> {{.Detail}}</p>{{end}}
  <hr style="border:none;border-top:1px solid #e5e7eb;margin:20px 0">
  <p style="color:#6b7280;font-size:12px">Sent by EncodeSwarmr</p>
</body>
</html>`))

	tmplAgentStale = template.Must(template.New("agent_stale").Parse(`<!DOCTYPE html>
<html>
<body style="font-family:sans-serif;color:#333;max-width:600px;margin:0 auto;padding:20px">
  <h2 style="color:#d97706">Agent Stale</h2>
  <p>Agent <strong>{{.AgentName}}</strong> has stopped sending heartbeats and has been marked offline.</p>
  <hr style="border:none;border-top:1px solid #e5e7eb;margin:20px 0">
  <p style="color:#6b7280;font-size:12px">Sent by EncodeSwarmr</p>
</body>
</html>`))
)

// RenderJobCompleted returns an HTML email body for a job.completed event.
func RenderJobCompleted(jobID, sourcePath string) (string, error) {
	return render(tmplJobCompleted, templateData{JobID: jobID, SourcePath: sourcePath})
}

// RenderJobFailed returns an HTML email body for a job.failed event.
func RenderJobFailed(jobID, sourcePath, detail string) (string, error) {
	return render(tmplJobFailed, templateData{JobID: jobID, SourcePath: sourcePath, Detail: detail})
}

// RenderAgentStale returns an HTML email body for an agent.stale event.
func RenderAgentStale(agentName string) (string, error) {
	return render(tmplAgentStale, templateData{AgentName: agentName})
}

func render(t *template.Template, data templateData) (string, error) {
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render template %s: %w", t.Name(), err)
	}
	return buf.String(), nil
}

