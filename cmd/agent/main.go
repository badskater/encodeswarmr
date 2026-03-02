package main

import (
	"log/slog"
	"os"

	"github.com/badskater/distributed-encoder/internal/agent/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	if err := service.Run(os.Args[1:]); err != nil {
		slog.Error("agent fatal", "err", err)
		os.Exit(1)
	}
}
