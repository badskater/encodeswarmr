//go:build !windows && !linux

package service

import (
	"context"
	"errors"
	"fmt"
)

// runAsWindowsService is a stub for non-Windows platforms.
func runAsWindowsService(_ string, _ func(ctx context.Context) error) error {
	return fmt.Errorf("windows service not supported on this platform")
}

// isWindowsService always returns false on non-Windows platforms.
func isWindowsService() bool {
	return false
}

// installService is a stub for non-Windows platforms.
func installService(_, _ string) error {
	return errors.New("only supported on Windows")
}

// uninstallService is a stub for non-Windows platforms.
func uninstallService(_ string) error {
	return errors.New("only supported on Windows")
}

// startService is a stub for non-Windows platforms.
func startService(_ string) error {
	return errors.New("only supported on Windows")
}

// stopService is a stub for non-Windows platforms.
func stopService(_ string) error {
	return errors.New("only supported on Windows")
}
