// Package cli implements the controller command-line interface using cobra.
package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/badskater/distributed-encoder/internal/controller/config"
	controllergrpc "github.com/badskater/distributed-encoder/internal/controller/grpc"
	"github.com/badskater/distributed-encoder/internal/db"
	"github.com/spf13/cobra"
)

var cfgFile string

// Execute builds and runs the root command.
func Execute(ctx context.Context) error {
	return newRootCmd(ctx).Execute()
}

func newRootCmd(ctx context.Context) *cobra.Command {
	root := &cobra.Command{
		Use:   "controller",
		Short: "Distributed encoder controller",
	}
	root.PersistentFlags().StringVar(&cfgFile, "config", "config.yaml", "config file path")
	root.AddCommand(newServerCmd(ctx))
	return root
}

func newServerCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Start the controller server (HTTP + gRPC)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServer(ctx, cfgFile)
		},
	}
}

func runServer(ctx context.Context, cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("cli: load config: %w", err)
	}

	// Configure slog level from config.
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.Logging.Level)); err != nil {
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	// Open database connection and run migrations.
	store, pool, err := db.New(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("cli: open db: %w", err)
	}
	defer pool.Close()

	if err := db.Migrate(cfg.Database.URL); err != nil {
		return fmt.Errorf("cli: migrate db: %w", err)
	}
	logger.Info("database migrations applied")

	// Catch OS signals for graceful shutdown.
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start gRPC server.
	grpcSrv := controllergrpc.New(store, &cfg.GRPC, &cfg.Agent, logger)
	grpcErrCh := make(chan error, 1)
	go func() {
		logger.Info("starting gRPC server", "addr", fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.GRPC.Port))
		grpcErrCh <- grpcSrv.Serve(ctx)
	}()

	// Block until a signal or gRPC server error.
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
		return nil
	case err := <-grpcErrCh:
		if err != nil {
			return fmt.Errorf("cli: grpc server: %w", err)
		}
		return nil
	}
}
