//go:build linux

package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const systemdUnitTemplate = `[Unit]
Description=Distributed Encoder Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{.ExecPath}} run --config {{.ConfigPath}}
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier={{.ServiceName}}

[Install]
WantedBy=multi-user.target
`

// runAsWindowsService is a stub on Linux — the agent always runs in foreground
// mode (or as a systemd service managed externally by systemd itself).
func runAsWindowsService(_ string, _ func(ctx context.Context) error) error {
	return fmt.Errorf("not applicable on Linux: use 'systemctl start %s'", serviceName)
}

// isWindowsService always returns false on Linux.
func isWindowsService() bool {
	return false
}

// installService writes a systemd unit file and enables the service.
func installService(name, configPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}

	unitPath := filepath.Join("/etc/systemd/system", name+".service")

	f, err := os.OpenFile(unitPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("creating unit file %s: %w (run as root?)", unitPath, err)
	}
	defer f.Close()

	tmpl, err := template.New("unit").Parse(systemdUnitTemplate)
	if err != nil {
		return fmt.Errorf("parsing unit template: %w", err)
	}
	if err := tmpl.Execute(f, struct {
		ExecPath    string
		ConfigPath  string
		ServiceName string
	}{
		ExecPath:    exePath,
		ConfigPath:  configPath,
		ServiceName: name,
	}); err != nil {
		return fmt.Errorf("writing unit file: %w", err)
	}

	// Reload systemd and enable the service.
	if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w: %s", err, out)
	}
	if out, err := exec.Command("systemctl", "enable", name).CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl enable %s: %w: %s", name, err, out)
	}

	slog.Info("service installed", "name", name, "unit", unitPath)
	return nil
}

// uninstallService disables and removes the systemd unit file.
func uninstallService(name string) error {
	// Disable first (best-effort stop).
	if out, err := exec.Command("systemctl", "disable", "--now", name).CombinedOutput(); err != nil {
		slog.Warn("systemctl disable failed", "error", err, "output", string(out))
	}

	unitPath := filepath.Join("/etc/systemd/system", name+".service")
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing unit file %s: %w", unitPath, err)
	}

	if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w: %s", err, out)
	}

	slog.Info("service uninstalled", "name", name)
	return nil
}

// startService starts the systemd service.
func startService(name string) error {
	if out, err := exec.Command("systemctl", "start", name).CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl start %s: %w: %s", name, err, out)
	}
	slog.Info("service started", "name", name)
	return nil
}

// stopService stops the systemd service.
func stopService(name string) error {
	if out, err := exec.Command("systemctl", "stop", name).CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl stop %s: %w: %s", name, err, out)
	}
	slog.Info("service stopped", "name", name)
	return nil
}
