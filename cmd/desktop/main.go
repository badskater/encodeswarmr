package main

import (
	"log/slog"
	"os"

	"gioui.org/app"

	desktopapp "github.com/badskater/encodeswarmr/internal/desktop/app"
	"github.com/badskater/encodeswarmr/internal/desktop/page"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	logger.Info("starting EncodeSwarmr Desktop", "version", Version)

	application := desktopapp.NewApplication(logger)

	// Register all page factories before the event loop starts.
	page.RegisterAll(application.Router(), application.State(), application.Window(), logger)

	// Navigate to the login page as the initial route.
	application.Router().Push("/login", nil)

	go func() {
		if err := application.Run(); err != nil {
			logger.Error("application error", "err", err)
			os.Exit(1)
		}
	}()

	app.Main()
}
