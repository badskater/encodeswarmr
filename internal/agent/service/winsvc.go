//go:build windows

package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// windowsService implements the svc.Handler interface so the agent can run as
// a Windows Service managed by the SCM.
type windowsService struct {
	name string
	run  func(ctx context.Context) error
}

// Execute is called by the Windows Service Control Manager. It starts the
// agent in a goroutine and translates SCM commands (stop, shutdown) into
// context cancellation.
func (ws *windowsService) Execute(args []string, req <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	status <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- ws.run(ctx)
	}()

	status <- svc.Status{State: svc.Running, Accepts: accepted}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				slog.Error("service run error", "error", err)
				status <- svc.Status{State: svc.StopPending}
				return true, 1
			}
			status <- svc.Status{State: svc.StopPending}
			return false, 0

		case cr := <-req:
			switch cr.Cmd {
			case svc.Interrogate:
				status <- cr.CurrentStatus
				// Send twice per MSDN recommendation.
				time.Sleep(100 * time.Millisecond)
				status <- cr.CurrentStatus
			case svc.Stop, svc.Shutdown:
				slog.Info("service stop requested")
				status <- svc.Status{State: svc.StopPending}
				cancel()
				// Wait for the run function to finish.
				<-errCh
				return false, 0
			default:
				slog.Warn("unexpected service control request", "cmd", cr.Cmd)
			}
		}
	}
}

// runAsWindowsService registers and runs the agent as a Windows Service.
func runAsWindowsService(name string, run func(ctx context.Context) error) error {
	err := svc.Run(name, &windowsService{name: name, run: run})
	if err != nil {
		return fmt.Errorf("windows service run: %w", err)
	}
	return nil
}

// isWindowsService returns true when the process is running under the Windows
// Service Control Manager.
func isWindowsService() bool {
	ok, _ := svc.IsWindowsService()
	return ok
}

// installService registers the agent as a Windows Service.
func installService(name, configPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connecting to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.CreateService(name, exePath, mgr.Config{
		StartType:   mgr.StartAutomatic,
		DisplayName: "Distributed Encoder Agent",
		Description: "Distributed video encoding agent",
	}, "--config", configPath)
	if err != nil {
		return fmt.Errorf("creating service: %w", err)
	}
	defer s.Close()

	slog.Info("service installed", "name", name)
	return nil
}

// uninstallService removes the Windows Service registration.
func uninstallService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connecting to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("opening service: %w", err)
	}
	defer s.Close()

	if err := s.Delete(); err != nil {
		return fmt.Errorf("deleting service: %w", err)
	}

	slog.Info("service uninstalled", "name", name)
	return nil
}

// startService starts the Windows Service via the SCM.
func startService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connecting to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("opening service: %w", err)
	}
	defer s.Close()

	if err := s.Start(); err != nil {
		return fmt.Errorf("starting service: %w", err)
	}

	slog.Info("service started", "name", name)
	return nil
}

// stopService sends a stop control command to the Windows Service.
func stopService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connecting to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("opening service: %w", err)
	}
	defer s.Close()

	_, err = s.Control(svc.Stop)
	if err != nil {
		return fmt.Errorf("stopping service: %w", err)
	}

	slog.Info("service stop requested", "name", name)
	return nil
}
