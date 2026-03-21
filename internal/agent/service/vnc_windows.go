//go:build windows

package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	agentcfg "github.com/badskater/encodeswarmr/internal/agent/config"
)

// installAndConfigureVNC downloads (if InstallerURL is set) and silently
// installs TightVNC, then ensures the service is running on the configured
// port. It is idempotent — calling it when TightVNC is already installed and
// running is a no-op.
//
// Installer MSI must be TightVNC 2.x; the function passes the standard
// TightVNC MSI public properties for a silent, non-interactive install.
func installAndConfigureVNC(cfg agentcfg.VNCConfig, log *slog.Logger) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Password == "" {
		return fmt.Errorf("vnc: password must not be empty when VNC is enabled")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("vnc: invalid port %d", cfg.Port)
	}

	// Download the installer if a URL was provided and TightVNC is not yet installed.
	if cfg.InstallerURL != "" {
		installed, err := isTightVNCInstalled()
		if err != nil {
			log.Warn("vnc: could not check installation status", "err", err)
		}
		if !installed {
			log.Info("vnc: TightVNC not found, downloading installer", "url", cfg.InstallerURL)
			msiPath, err := downloadFile(cfg.InstallerURL)
			if err != nil {
				return fmt.Errorf("vnc: download installer: %w", err)
			}
			defer os.Remove(msiPath)

			log.Info("vnc: running silent MSI install")
			if err := runSilentMSI(msiPath, cfg.Password, cfg.Port); err != nil {
				return fmt.Errorf("vnc: silent install: %w", err)
			}
			log.Info("vnc: TightVNC installed successfully")
		} else {
			log.Info("vnc: TightVNC already installed, skipping download")
		}
	}

	// Apply/update password and port via the TightVNC configuration tool.
	if err := configureTightVNC(cfg.Password, cfg.Port, log); err != nil {
		return fmt.Errorf("vnc: configure: %w", err)
	}

	// Ensure the TightVNC service is started.
	if err := ensureVNCServiceRunning(log); err != nil {
		return fmt.Errorf("vnc: start service: %w", err)
	}

	log.Info("vnc: VNC server ready", "port", cfg.Port)
	return nil
}

// isTightVNCInstalled checks whether TightVNC Server is present in the
// standard installation paths.
func isTightVNCInstalled() (bool, error) {
	paths := []string{
		`C:\Program Files\TightVNC\tvnserver.exe`,
		`C:\Program Files (x86)\TightVNC\tvnserver.exe`,
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return true, nil
		}
	}
	return false, nil
}

// downloadFile downloads the resource at url to a temporary .msi file and
// returns its path. The caller is responsible for removing the file.
func downloadFile(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	f, err := os.CreateTemp("", "tightvnc-*.msi")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("write installer: %w", err)
	}
	return f.Name(), nil
}

// runSilentMSI executes msiexec to install the TightVNC MSI silently.
// It sets the VNC password and port via standard TightVNC MSI public properties.
func runSilentMSI(msiPath, password string, port int) error {
	logPath := filepath.Join(os.TempDir(), "tightvnc-install.log")
	args := []string{
		"/i", msiPath,
		"/quiet",
		"/norestart",
		fmt.Sprintf("/l*v %s", logPath),
		"ADDLOCAL=Server",
		"SET_USEVNCAUTHENTICATION=1",
		"VALUE_OF_USEVNCAUTHENTICATION=1",
		"SET_PASSWORD=1",
		fmt.Sprintf("VALUE_OF_PASSWORD=%s", password),
		"SET_CONTROLPASSWORD=1",
		fmt.Sprintf("VALUE_OF_CONTROLPASSWORD=%s", password),
		fmt.Sprintf("SET_RFBPORT=1"),
		fmt.Sprintf("VALUE_OF_RFBPORT=%d", port),
	}

	cmd := exec.Command("msiexec.exe", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("msiexec failed: %w (output: %s)", err, string(out))
	}
	return nil
}

// configureTightVNC updates the VNC password and port in the registry using
// the tvnserver command-line interface (available in TightVNC 2.x).
func configureTightVNC(password string, port int, log *slog.Logger) error {
	tvnserver := tightVNCServerPath()
	if tvnserver == "" {
		log.Warn("vnc: tvnserver.exe not found, skipping runtime configuration")
		return nil
	}

	// Set the VNC password (stored as an obfuscated registry value by tvnserver).
	passArgs := []string{"-setpass", password}
	if out, err := exec.Command(tvnserver, passArgs...).CombinedOutput(); err != nil {
		return fmt.Errorf("set password: %w (output: %s)", err, string(out))
	}

	// Set the RFB (VNC) port.
	portArgs := []string{"-controlapp", fmt.Sprintf("-rfbport:%d", port)}
	if out, err := exec.Command(tvnserver, portArgs...).CombinedOutput(); err != nil {
		// Non-fatal — port is also set during MSI install.
		log.Warn("vnc: could not update RFB port at runtime", "err", err, "output", string(out))
	}
	return nil
}

// ensureVNCServiceRunning starts the TightVNC service if it is not already running.
func ensureVNCServiceRunning(log *slog.Logger) error {
	tvnserver := tightVNCServerPath()
	if tvnserver == "" {
		// Try sc start as a fallback.
		out, err := exec.Command("sc", "start", "tvnserver").CombinedOutput()
		if err != nil {
			log.Warn("vnc: could not start tvnserver via sc", "err", err, "output", string(out))
		}
		return nil
	}

	// -start registers and starts the service; idempotent if already running.
	out, err := exec.Command(tvnserver, "-start").CombinedOutput()
	if err != nil {
		// Error code 1 (already running) is acceptable.
		log.Warn("vnc: tvnserver -start returned non-zero", "err", err, "output", string(out))
	}
	return nil
}

// tightVNCServerPath returns the full path to tvnserver.exe, or "" if not found.
func tightVNCServerPath() string {
	candidates := []string{
		`C:\Program Files\TightVNC\tvnserver.exe`,
		`C:\Program Files (x86)\TightVNC\tvnserver.exe`,
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
