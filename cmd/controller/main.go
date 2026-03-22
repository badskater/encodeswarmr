package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/badskater/encodeswarmr/internal/controller/cli"
)

// Version is set at build time via -ldflags "-X main.Version=..."
var Version = "dev"

func main() {
	ctx := context.Background()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := cli.Execute(ctx); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}
