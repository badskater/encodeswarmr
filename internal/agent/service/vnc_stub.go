//go:build !windows

package service

import (
	"log/slog"

	agentcfg "github.com/badskater/encodeswarmr/internal/agent/config"
)

// installAndConfigureVNC is a no-op on non-Windows platforms.
// VNC installation via the agent is only supported on Windows.
func installAndConfigureVNC(cfg agentcfg.VNCConfig, log *slog.Logger) error {
	if cfg.Enabled {
		log.Warn("vnc: VNC auto-install is only supported on Windows; configure your VNC server manually")
	}
	return nil
}
