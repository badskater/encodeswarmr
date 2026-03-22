// Package cli implements the controller command-line interface using cobra.
package cli

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/badskater/encodeswarmr/internal/controller/analysis"
	"github.com/badskater/encodeswarmr/internal/controller/api"
	"github.com/badskater/encodeswarmr/internal/controller/auth"
	"github.com/badskater/encodeswarmr/internal/controller/config"
	"github.com/badskater/encodeswarmr/internal/controller/engine"
	controllergrpc "github.com/badskater/encodeswarmr/internal/controller/grpc"
	"github.com/badskater/encodeswarmr/internal/controller/ha"
	"github.com/badskater/encodeswarmr/internal/controller/notifications"
	"github.com/badskater/encodeswarmr/internal/controller/mediaserver"
	"github.com/badskater/encodeswarmr/internal/controller/webhooks"
	"github.com/badskater/encodeswarmr/internal/db"
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
	root.AddCommand(
		newServerCmd(ctx),
		newRunCmd(ctx),
		newAgentCmd(ctx),
		newSourceCmd(ctx),
		newTemplateCmd(ctx),
		newJobCmd(ctx),
		newTaskCmd(ctx),
		newUserCmd(ctx),
		newWebhookCmd(ctx),
		newTLSCmd(),
	)
	return root
}

// ---------------------------------------------------------------------------
// server / run — start the controller
// ---------------------------------------------------------------------------

func newServerCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Start the controller server (HTTP + gRPC)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServer(ctx, cfgFile)
		},
	}
}

func newRunCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Start the controller server (alias for server)",
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

	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.Logging.Level)); err != nil {
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	store, pool, err := db.New(ctx, cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("cli: open db: %w", err)
	}
	defer pool.Close()

	if err := db.Migrate(cfg.Database.URL); err != nil {
		return fmt.Errorf("cli: migrate db: %w", err)
	}
	logger.Info("database migrations applied")

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Start HA leader election.  nodeID defaults to hostname; override via the
	// HA_NODE_ID environment variable for deployments where multiple replicas
	// share the same hostname (e.g. Docker Swarm / Kubernetes).
	nodeID, _ := os.Hostname()
	if envID := os.Getenv("HA_NODE_ID"); envID != "" {
		nodeID = envID
	}
	ldr := ha.NewLeader(pool, nodeID, logger)
	ldr.Start(ctx)
	defer ldr.Stop()
	logger.Info("ha leader election started", "node_id", nodeID)

	authSvc, err := auth.NewService(ctx, store, &cfg.Auth, logger)
	if err != nil {
		return fmt.Errorf("cli: init auth: %w", err)
	}

	// Start webhook delivery service.
	whSvc := webhooks.New(store, webhooks.Config{
		WorkerCount:     cfg.Webhooks.WorkerCount,
		DeliveryTimeout: cfg.Webhooks.DeliveryTimeout,
		MaxRetries:      cfg.Webhooks.MaxRetries,
	}, logger)
	// Attach email sender when SMTP is configured.
	emailSender := notifications.NewEmailSender(cfg.SMTP, logger)
	if emailSender != nil {
		whSvc.SetEmailSender(emailSender)
		logger.Info("email notifications enabled", "smtp_host", cfg.SMTP.Host)
	}
	whSvc.Start(ctx)
	logger.Info("webhook delivery service started", "workers", cfg.Webhooks.WorkerCount)

	// Start HTTP API server.
	httpSrv, err := api.New(store, authSvc, cfg, logger, whSvc, ldr)
	if err != nil {
		return fmt.Errorf("create api server: %w", err)
	}
	httpErrCh := make(chan error, 1)
	go func() {
		logger.Info("starting HTTP server", "addr", fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port))
		httpErrCh <- httpSrv.Serve(ctx)
	}()

	// Start gRPC server.
	grpcSrv := controllergrpc.New(store, &cfg.GRPC, &cfg.Agent, logger, whSvc)
	grpcErrCh := make(chan error, 1)
	go func() {
		logger.Info("starting gRPC server", "addr", fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.GRPC.Port))
		grpcErrCh <- grpcSrv.Serve(ctx)
	}()

	// Bootstrap path mappings from config (idempotent — only inserts missing ones).
	bootstrapPathMappings(ctx, store, cfg, logger)

	// Start core engine (job expansion + stale agent detection + log retention).
	eng := engine.New(store, engine.Config{
		DispatchInterval:   cfg.Agent.DispatchInterval,
		StaleThreshold:     cfg.Agent.HeartbeatTimeout,
		ScriptBaseDir:      cfg.Agent.ScriptBaseDir,
		LogRetention:       cfg.Logging.TaskLogRetention,
		LogCleanupInterval: cfg.Logging.TaskLogCleanupInterval,
	}, logger)

	// Attach controller-side analysis runner when configured.
	analysisRunner := analysis.New(store, analysis.Config{
		FFmpegBin:   cfg.Analysis.FFmpegBin,
		FFprobeBin:  cfg.Analysis.FFprobeBin,
		DoviToolBin: cfg.Analysis.DoviToolBin,
		Concurrency: cfg.Analysis.Concurrency,
	}, logger)
	eng.SetAnalysisRunner(analysisRunner)
	eng.SetConcatRunner(analysisRunner)
	grpcSrv.SetConcatRunner(analysisRunner)

	// Attach media server manager when servers are configured.
	if len(cfg.MediaServers) > 0 {
		mediaMgr := mediaserver.New(cfg.MediaServers, logger)
		grpcSrv.SetMediaManager(mediaMgr, cfg.MediaServers)
		logger.Info("media server manager attached", "count", len(cfg.MediaServers))
	}

	logger.Info("controller-side analysis runner attached",
		"ffmpeg", cfg.Analysis.FFmpegBin,
		"ffprobe", cfg.Analysis.FFprobeBin,
		"concurrency", cfg.Analysis.Concurrency,
	)

	// Attach auto-scaling hook when enabled.
	asHook := engine.NewAutoScalingHook(func() config.AutoScalingConfig { return cfg.AutoScaling }, logger)
	eng.SetAutoScalingHook(asHook)
	if cfg.AutoScaling.Enabled {
		logger.Info("auto-scaling hooks enabled",
			"scale_up_threshold", cfg.AutoScaling.ScaleUpThreshold,
			"scale_down_threshold", cfg.AutoScaling.ScaleDownThreshold,
			"cooldown_seconds", cfg.AutoScaling.CooldownSeconds,
		)
	}
	// Configure post-encode output validation.
	validationCfg := engine.ValidationConfig{
		Enabled:          cfg.Validation.Enabled,
		FFprobeBin:       cfg.Analysis.FFprobeBin,
		MinDurationRatio: cfg.Validation.MinDurationRatio,
	}
	grpcSrv.SetValidationConfig(validationCfg)
	logger.Info("output validation configured",
		"enabled", validationCfg.Enabled,
		"min_duration_ratio", validationCfg.MinDurationRatio,
	)

	eng.Start(ctx)
	logger.Info("core engine started",
		"dispatch_interval", cfg.Agent.DispatchInterval,
		"stale_threshold", cfg.Agent.HeartbeatTimeout,
	)

	// Start job archival loop if enabled.
	eng.StartArchivalLoop(ctx, engine.ArchiveConfig{
		Enabled:       cfg.Archive.Enabled,
		RetentionDays: cfg.Archive.RetentionDays,
	})
	if cfg.Archive.Enabled {
		logger.Info("job archival loop started",
			"retention_days", cfg.Archive.RetentionDays,
		)
	}

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
		return nil
	case err := <-httpErrCh:
		if err != nil {
			return fmt.Errorf("cli: http server: %w", err)
		}
		return nil
	case err := <-grpcErrCh:
		if err != nil {
			return fmt.Errorf("cli: grpc server: %w", err)
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// bootstrapPathMappings inserts any path mappings defined in the config file
// that do not already exist in the database (matched by name).  This lets
// operators seed the initial set of mappings via config, then manage them at
// runtime through the UI or API without config-file changes.
func bootstrapPathMappings(ctx context.Context, store db.Store, cfg *config.Config, logger *slog.Logger) {
	if len(cfg.Analysis.PathMappings) == 0 {
		return
	}

	existing, err := store.ListPathMappings(ctx)
	if err != nil {
		logger.Warn("bootstrap path mappings: list failed", "err", err)
		return
	}

	existingNames := make(map[string]bool, len(existing))
	for _, m := range existing {
		existingNames[m.Name] = true
	}

	for _, pm := range cfg.Analysis.PathMappings {
		if pm.Name == "" || pm.Windows == "" || pm.Linux == "" {
			continue
		}
		if existingNames[pm.Name] {
			continue // already exists — do not overwrite operator changes
		}
		if _, err := store.CreatePathMapping(ctx, db.CreatePathMappingParams{
			Name:          pm.Name,
			WindowsPrefix: pm.Windows,
			LinuxPrefix:   pm.Linux,
		}); err != nil {
			logger.Warn("bootstrap path mappings: create failed",
				"name", pm.Name, "err", err)
		} else {
			logger.Info("bootstrap path mapping created",
				"name", pm.Name,
				"windows", pm.Windows,
				"linux", pm.Linux,
			)
		}
	}
}

// openStore loads config and opens a database connection.
// The returned closer must be called when done.
func openStore(ctx context.Context, cfgPath string) (db.Store, func(), error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}
	store, pool, err := db.New(ctx, cfg.Database.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("open db: %w", err)
	}
	return store, pool.Close, nil
}

// newTabWriter returns a tabwriter writing to stdout with standard padding.
func newTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
}

// ---------------------------------------------------------------------------
// controller agent
// ---------------------------------------------------------------------------

func newAgentCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage encoding agents",
	}
	cmd.AddCommand(
		newAgentListCmd(ctx),
		newAgentApproveCmd(ctx),
		newAgentEnableCmd(ctx),
		newAgentDisableCmd(ctx),
	)
	return cmd
}

func newAgentListCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all agents",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			agents, err := store.ListAgents(ctx)
			if err != nil {
				return fmt.Errorf("list agents: %w", err)
			}
			tw := newTabWriter()
			fmt.Fprintln(tw, "ID\tNAME\tHOSTNAME\tSTATUS\tTAGS\tLAST HEARTBEAT")
			for _, a := range agents {
				hb := "never"
				if a.LastHeartbeat != nil {
					hb = a.LastHeartbeat.Format(time.DateTime)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
					a.ID, a.Name, a.Hostname, a.Status,
					strings.Join(a.Tags, ","), hb)
			}
			return tw.Flush()
		},
	}
}

func newAgentApproveCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "approve <name>",
		Short: "Approve a pending agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			agent, err := store.GetAgentByName(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get agent: %w", err)
			}
			if err := store.UpdateAgentStatus(ctx, agent.ID, "idle"); err != nil {
				return fmt.Errorf("approve agent: %w", err)
			}
			fmt.Printf("agent %q approved\n", args[0])
			return nil
		},
	}
}

func newAgentEnableCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <name>",
		Short: "Enable an agent (set status to idle)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			agent, err := store.GetAgentByName(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get agent: %w", err)
			}
			if err := store.UpdateAgentStatus(ctx, agent.ID, "idle"); err != nil {
				return fmt.Errorf("enable agent: %w", err)
			}
			fmt.Printf("agent %q enabled\n", args[0])
			return nil
		},
	}
}

func newAgentDisableCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <name>",
		Short: "Disable an agent (set status to draining)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			agent, err := store.GetAgentByName(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get agent: %w", err)
			}
			if err := store.UpdateAgentStatus(ctx, agent.ID, "draining"); err != nil {
				return fmt.Errorf("disable agent: %w", err)
			}
			fmt.Printf("agent %q disabled\n", args[0])
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// controller source
// ---------------------------------------------------------------------------

func newSourceCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "source",
		Short: "Manage source files",
	}
	cmd.AddCommand(
		newSourceListCmd(ctx),
		newSourceStatusCmd(ctx),
	)
	return cmd
}

func newSourceListCmd(ctx context.Context) *cobra.Command {
	var state string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List source files",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			sources, _, err := store.ListSources(ctx, db.ListSourcesFilter{State: state, PageSize: 200})
			if err != nil {
				return fmt.Errorf("list sources: %w", err)
			}
			tw := newTabWriter()
			fmt.Fprintln(tw, "ID\tFILENAME\tSTATE\tVMAF\tSIZE")
			for _, s := range sources {
				vmaf := "-"
				if s.VMafScore != nil {
					vmaf = fmt.Sprintf("%.2f", *s.VMafScore)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					s.ID, s.Filename, s.State, vmaf, fmtBytes(s.SizeBytes))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&state, "state", "", "filter by state")
	return cmd
}

func newSourceStatusCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "status <id>",
		Short: "Show source detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			src, err := store.GetSourceByID(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get source: %w", err)
			}
			tw := newTabWriter()
			fmt.Fprintf(tw, "ID:\t%s\n", src.ID)
			fmt.Fprintf(tw, "Filename:\t%s\n", src.Filename)
			fmt.Fprintf(tw, "UNC Path:\t%s\n", src.UNCPath)
			fmt.Fprintf(tw, "State:\t%s\n", src.State)
			fmt.Fprintf(tw, "Size:\t%s\n", fmtBytes(src.SizeBytes))
			vmaf := "-"
			if src.VMafScore != nil {
				vmaf = fmt.Sprintf("%.4f", *src.VMafScore)
			}
			fmt.Fprintf(tw, "VMAF:\t%s\n", vmaf)
			fmt.Fprintf(tw, "Created:\t%s\n", src.CreatedAt.Format(time.DateTime))
			return tw.Flush()
		},
	}
}

// ---------------------------------------------------------------------------
// controller template
// ---------------------------------------------------------------------------

func newTemplateCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage script templates",
	}
	cmd.AddCommand(
		newTemplateListCmd(ctx),
		newTemplateAddCmd(ctx),
		newTemplateDeleteCmd(ctx),
	)
	return cmd
}

func newTemplateListCmd(ctx context.Context) *cobra.Command {
	var tplType string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List script templates",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			tpls, err := store.ListTemplates(ctx, tplType)
			if err != nil {
				return fmt.Errorf("list templates: %w", err)
			}
			tw := newTabWriter()
			fmt.Fprintln(tw, "ID\tNAME\tTYPE\tEXT\tDESCRIPTION")
			for _, t := range tpls {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					t.ID, t.Name, t.Type, t.Extension, t.Description)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&tplType, "type", "", "filter by type (run_script, frameserver)")
	return cmd
}

func newTemplateAddCmd(ctx context.Context) *cobra.Command {
	var tplType, filePath, ext, description string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a script template from a file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" {
				return fmt.Errorf("--file is required")
			}
			content, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read template file: %w", err)
			}
			if ext == "" {
				ext = strings.TrimPrefix(filepath.Ext(filePath), ".")
				if ext == "" {
					ext = "txt"
				}
			}
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			tpl, err := store.CreateTemplate(ctx, db.CreateTemplateParams{
				Name:        args[0],
				Description: description,
				Type:        tplType,
				Extension:   ext,
				Content:     string(content),
			})
			if err != nil {
				return fmt.Errorf("create template: %w", err)
			}
			fmt.Printf("template created: %s\n", tpl.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&tplType, "type", "run_script", "template type (run_script, frameserver)")
	cmd.Flags().StringVar(&filePath, "file", "", "path to template file")
	cmd.Flags().StringVar(&ext, "ext", "", "file extension (default: inferred from file)")
	cmd.Flags().StringVar(&description, "desc", "", "template description")
	return cmd
}

func newTemplateDeleteCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a template",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			if err := store.DeleteTemplate(ctx, args[0]); err != nil {
				return fmt.Errorf("delete template: %w", err)
			}
			fmt.Printf("template %q deleted\n", args[0])
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// controller job
// ---------------------------------------------------------------------------

func newJobCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job",
		Short: "Manage encoding jobs",
	}
	cmd.AddCommand(
		newJobListCmd(ctx),
		newJobStatusCmd(ctx),
		newJobCancelCmd(ctx),
		newJobRetryCmd(ctx),
	)
	return cmd
}

func newJobListCmd(ctx context.Context) *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List jobs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			jobs, _, err := store.ListJobs(ctx, db.ListJobsFilter{Status: status, PageSize: 200})
			if err != nil {
				return fmt.Errorf("list jobs: %w", err)
			}
			tw := newTabWriter()
			fmt.Fprintln(tw, "ID\tTYPE\tSTATUS\tPRIORITY\tTOTAL\tDONE\tFAILED\tCREATED")
			for _, j := range jobs {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%d\t%d\t%d\t%s\n",
					j.ID, j.JobType, j.Status, j.Priority,
					j.TasksTotal, j.TasksCompleted, j.TasksFailed,
					j.CreatedAt.Format(time.DateTime))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by status")
	return cmd
}

func newJobStatusCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "status <id>",
		Short: "Show job detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			job, err := store.GetJobByID(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get job: %w", err)
			}
			tw := newTabWriter()
			fmt.Fprintf(tw, "ID:\t%s\n", job.ID)
			fmt.Fprintf(tw, "Type:\t%s\n", job.JobType)
			fmt.Fprintf(tw, "Status:\t%s\n", job.Status)
			fmt.Fprintf(tw, "Priority:\t%d\n", job.Priority)
			fmt.Fprintf(tw, "Source ID:\t%s\n", job.SourceID)
			fmt.Fprintf(tw, "Tasks:\ttotal=%d pending=%d running=%d completed=%d failed=%d\n",
				job.TasksTotal, job.TasksPending, job.TasksRunning,
				job.TasksCompleted, job.TasksFailed)
			fmt.Fprintf(tw, "Created:\t%s\n", job.CreatedAt.Format(time.DateTime))
			if job.CompletedAt != nil {
				fmt.Fprintf(tw, "Completed:\t%s\n", job.CompletedAt.Format(time.DateTime))
			}
			return tw.Flush()
		},
	}
}

func newJobCancelCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <id>",
		Short: "Cancel a job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			if err := store.UpdateJobStatus(ctx, args[0], "cancelled"); err != nil {
				return fmt.Errorf("cancel job: %w", err)
			}
			if err := store.CancelPendingTasksForJob(ctx, args[0]); err != nil {
				return fmt.Errorf("cancel pending tasks: %w", err)
			}
			fmt.Printf("job %q cancelled\n", args[0])
			return nil
		},
	}
}

func newJobRetryCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "retry <id>",
		Short: "Re-queue failed tasks in a job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			if err := store.RetryFailedTasksForJob(ctx, args[0]); err != nil {
				return fmt.Errorf("retry tasks: %w", err)
			}
			if err := store.UpdateJobStatus(ctx, args[0], "queued"); err != nil {
				return fmt.Errorf("update job status: %w", err)
			}
			fmt.Printf("job %q re-queued\n", args[0])
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// controller task
// ---------------------------------------------------------------------------

func newTaskCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage encoding tasks",
	}
	cmd.AddCommand(
		newTaskListCmd(ctx),
		newTaskStatusCmd(ctx),
	)
	return cmd
}

func newTaskListCmd(ctx context.Context) *cobra.Command {
	var jobID string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks for a job",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if jobID == "" {
				return fmt.Errorf("--job is required")
			}
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			tasks, err := store.ListTasksByJob(ctx, jobID)
			if err != nil {
				return fmt.Errorf("list tasks: %w", err)
			}
			tw := newTabWriter()
			fmt.Fprintln(tw, "ID\tCHUNK\tSTATUS\tAGENT\tFPS\tEXIT")
			for _, t := range tasks {
				agent := "-"
				if t.AgentID != nil {
					agent = *t.AgentID
				}
				fps := "-"
				if t.AvgFPS != nil {
					fps = fmt.Sprintf("%.1f", *t.AvgFPS)
				}
				exit := "-"
				if t.ExitCode != nil {
					exit = fmt.Sprintf("%d", *t.ExitCode)
				}
				fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%s\n",
					t.ID, t.ChunkIndex, t.Status, agent, fps, exit)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&jobID, "job", "", "job ID")
	return cmd
}

func newTaskStatusCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "status <id>",
		Short: "Show task detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			task, err := store.GetTaskByID(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get task: %w", err)
			}
			tw := newTabWriter()
			fmt.Fprintf(tw, "ID:\t%s\n", task.ID)
			fmt.Fprintf(tw, "Job ID:\t%s\n", task.JobID)
			fmt.Fprintf(tw, "Chunk:\t%d\n", task.ChunkIndex)
			fmt.Fprintf(tw, "Status:\t%s\n", task.Status)
			fmt.Fprintf(tw, "Script Dir:\t%s\n", task.ScriptDir)
			fmt.Fprintf(tw, "Source:\t%s\n", task.SourcePath)
			fmt.Fprintf(tw, "Output:\t%s\n", task.OutputPath)
			if task.AvgFPS != nil {
				fmt.Fprintf(tw, "Avg FPS:\t%.1f\n", *task.AvgFPS)
			}
			if task.ExitCode != nil {
				fmt.Fprintf(tw, "Exit Code:\t%d\n", *task.ExitCode)
			}
			if task.ErrorMsg != nil {
				fmt.Fprintf(tw, "Error:\t%s\n", *task.ErrorMsg)
			}
			return tw.Flush()
		},
	}
}

// ---------------------------------------------------------------------------
// controller user
// ---------------------------------------------------------------------------

func newUserCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Manage controller users",
	}
	cmd.AddCommand(
		newUserListCmd(ctx),
		newUserAddCmd(ctx),
		newUserDeleteCmd(ctx),
		newUserSetRoleCmd(ctx),
	)
	return cmd
}

func newUserListCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List users",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			users, err := store.ListUsers(ctx)
			if err != nil {
				return fmt.Errorf("list users: %w", err)
			}
			tw := newTabWriter()
			fmt.Fprintln(tw, "ID\tUSERNAME\tEMAIL\tROLE\tCREATED")
			for _, u := range users {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					u.ID, u.Username, u.Email, u.Role,
					u.CreatedAt.Format(time.DateTime))
			}
			return tw.Flush()
		},
	}
}

func newUserAddCmd(ctx context.Context) *cobra.Command {
	var role, email, password string
	cmd := &cobra.Command{
		Use:   "add <username>",
		Short: "Create a local user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if password == "" {
				return fmt.Errorf("--password is required")
			}
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			user, err := store.CreateUser(ctx, db.CreateUserParams{
				Username: args[0],
				Email:    email,
				Role:     role,
			})
			if err != nil {
				return fmt.Errorf("create user: %w", err)
			}
			fmt.Printf("user created: %s (%s)\n", user.Username, user.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&role, "role", "viewer", "role (viewer, operator, admin)")
	cmd.Flags().StringVar(&email, "email", "", "email address")
	cmd.Flags().StringVar(&password, "password", "", "password (required)")
	return cmd
}

func newUserDeleteCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <username>",
		Short: "Delete a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			user, err := store.GetUserByUsername(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get user: %w", err)
			}
			if err := store.DeleteUser(ctx, user.ID); err != nil {
				return fmt.Errorf("delete user: %w", err)
			}
			fmt.Printf("user %q deleted\n", args[0])
			return nil
		},
	}
}

func newUserSetRoleCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "set-role <username> <role>",
		Short: "Change a user's role",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			user, err := store.GetUserByUsername(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get user: %w", err)
			}
			if err := store.UpdateUserRole(ctx, user.ID, args[1]); err != nil {
				return fmt.Errorf("set role: %w", err)
			}
			fmt.Printf("user %q role set to %q\n", args[0], args[1])
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// controller webhook
// ---------------------------------------------------------------------------

func newWebhookCmd(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook",
		Short: "Manage webhooks",
	}
	cmd.AddCommand(
		newWebhookListCmd(ctx),
		newWebhookAddCmd(ctx),
		newWebhookDeleteCmd(ctx),
		newWebhookTestCmd(ctx),
	)
	return cmd
}

func newWebhookListCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List webhooks",
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			whs, err := store.ListWebhooks(ctx)
			if err != nil {
				return fmt.Errorf("list webhooks: %w", err)
			}
			tw := newTabWriter()
			fmt.Fprintln(tw, "ID\tNAME\tPROVIDER\tENABLED\tEVENTS")
			for _, wh := range whs {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%v\t%s\n",
					wh.ID, wh.Name, wh.Provider, wh.Enabled,
					strings.Join(wh.Events, ","))
			}
			return tw.Flush()
		},
	}
}

func newWebhookAddCmd(ctx context.Context) *cobra.Command {
	var provider, url, events, secret string
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a webhook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if provider == "" || url == "" || events == "" {
				return fmt.Errorf("--provider, --url, and --events are required")
			}
			eventList := strings.Split(events, ",")
			params := db.CreateWebhookParams{
				Name:     args[0],
				Provider: provider,
				URL:      url,
				Events:   eventList,
			}
			if secret != "" {
				params.Secret = &secret
			}
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			wh, err := store.CreateWebhook(ctx, params)
			if err != nil {
				return fmt.Errorf("create webhook: %w", err)
			}
			fmt.Printf("webhook created: %s\n", wh.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "provider (discord, teams, slack)")
	cmd.Flags().StringVar(&url, "url", "", "webhook URL")
	cmd.Flags().StringVar(&events, "events", "", "comma-separated event types")
	cmd.Flags().StringVar(&secret, "secret", "", "HMAC signing secret")
	return cmd
}

func newWebhookDeleteCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a webhook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			if err := store.DeleteWebhook(ctx, args[0]); err != nil {
				return fmt.Errorf("delete webhook: %w", err)
			}
			fmt.Printf("webhook %q deleted\n", args[0])
			return nil
		},
	}
}

func newWebhookTestCmd(ctx context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "test <id>",
		Short: "Send a test delivery to a webhook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, close, err := openStore(ctx, cfgFile)
			if err != nil {
				return err
			}
			defer close()
			wh, err := store.GetWebhookByID(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get webhook: %w", err)
			}
			payload := []byte(`{"event":"test","message":"Test notification from encodeswarmr"}`)
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Post(wh.URL, "application/json", bytes.NewReader(payload))
			if err != nil {
				return fmt.Errorf("test delivery failed: %w", err)
			}
			defer resp.Body.Close()
			io.Copy(io.Discard, resp.Body)
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				fmt.Printf("test delivery succeeded (HTTP %d)\n", resp.StatusCode)
			} else {
				fmt.Printf("test delivery returned HTTP %d\n", resp.StatusCode)
			}
			return nil
		},
	}
}

// ---------------------------------------------------------------------------
// controller tls
// ---------------------------------------------------------------------------

func newTLSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tls",
		Short: "TLS certificate utilities",
	}
	cmd.AddCommand(newTLSGenerateCmd())
	return cmd
}

func newTLSGenerateCmd() *cobra.Command {
	var cn, outDir string
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a self-signed TLS certificate and key",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cn == "" {
				return fmt.Errorf("--cn is required")
			}
			if outDir == "" {
				outDir = "."
			}
			return generateSelfSigned(cn, outDir)
		},
	}
	cmd.Flags().StringVar(&cn, "cn", "", "common name (e.g. controller.internal)")
	cmd.Flags().StringVar(&outDir, "out", ".", "output directory")
	return cmd
}

// generateSelfSigned creates a self-signed ECDSA P-256 certificate valid for
// 10 years and writes tls.crt and tls.key to outDir.
func generateSelfSigned(cn, outDir string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate serial: %w", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:         true,
		DNSNames:     []string{cn},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	certPath := filepath.Join(outDir, "tls.crt")
	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create cert file: %w", err)
	}
	defer certFile.Close()
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("encode cert: %w", err)
	}

	keyPath := filepath.Join(outDir, "tls.key")
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create key file: %w", err)
	}
	defer keyFile.Close()
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}
	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		return fmt.Errorf("encode key: %w", err)
	}

	fmt.Printf("certificate: %s\n", certPath)
	fmt.Printf("private key: %s\n", keyPath)
	return nil
}

// ---------------------------------------------------------------------------
// Utilities
// ---------------------------------------------------------------------------

func fmtBytes(n int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case n >= GB:
		return fmt.Sprintf("%.1fGB", float64(n)/GB)
	case n >= MB:
		return fmt.Sprintf("%.1fMB", float64(n)/MB)
	case n >= KB:
		return fmt.Sprintf("%.1fKB", float64(n)/KB)
	default:
		return fmt.Sprintf("%dB", n)
	}
}
