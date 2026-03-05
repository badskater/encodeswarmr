package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	agentcfg "github.com/badskater/distributed-encoder/internal/agent/config"
	pb "github.com/badskater/distributed-encoder/internal/proto/encoderv1"
)

const serviceName = "distributed-encoder-agent"

// defaultConfigPath returns the platform-appropriate default config file path.
func defaultConfigPath() string {
	if runtime.GOOS == "windows" {
		return `C:\ProgramData\distributed-encoder\agent.yaml`
	}
	return "/etc/distributed-encoder/agent.yaml"
}

// usageText returns the platform-appropriate usage string.
func usageText() string {
	if runtime.GOOS == "windows" {
		return `Usage:
  distencoder-agent <subcommand> [flags]

Subcommands:
  install    Install as Windows Service
  uninstall  Remove the Windows Service
  start      Start the Windows Service
  stop       Stop the Windows Service
  run        Run in foreground (default if no subcommand or when started by SCM)

Flags (all subcommands):
  --config <path>    Config file path (default: C:\ProgramData\distributed-encoder\agent.yaml)
  --debug            Enable debug logging
  --http-debug       Start local debug HTTP server on :9080 (run subcommand only)
`
	}
	return `Usage:
  distencoder-agent <subcommand> [flags]

Subcommands:
  install    Install as systemd service
  uninstall  Remove the systemd service
  start      Start the systemd service
  stop       Stop the systemd service
  run        Run in foreground (default if no subcommand)

Flags (all subcommands):
  --config <path>    Config file path (default: /etc/distributed-encoder/agent.yaml)
  --debug            Enable debug logging
  --http-debug       Start local debug HTTP server on :9080 (run subcommand only)
`
}

// parsedArgs holds the result of parsing CLI arguments.
type parsedArgs struct {
	subcommand string
	configPath string
	debug      bool
	httpDebug  bool
}

// parseArgs walks the argument list extracting flags and the subcommand.
func parseArgs(args []string) parsedArgs {
	p := parsedArgs{
		configPath: defaultConfigPath(),
	}

	i := 0
	for i < len(args) {
		arg := args[i]

		// --config=<path>
		if strings.HasPrefix(arg, "--config=") {
			p.configPath = strings.TrimPrefix(arg, "--config=")
			i++
			continue
		}
		// --config <path>
		if arg == "--config" {
			if i+1 < len(args) {
				i++
				p.configPath = args[i]
			}
			i++
			continue
		}
		if arg == "--debug" {
			p.debug = true
			i++
			continue
		}
		if arg == "--http-debug" {
			p.httpDebug = true
			i++
			continue
		}

		// First non-flag argument is the subcommand.
		if p.subcommand == "" && !strings.HasPrefix(arg, "--") {
			p.subcommand = arg
		}
		i++
	}
	return p
}

// Run is the agent service entrypoint. It parses CLI subcommands and flags,
// then dispatches to the appropriate handler.
func Run(args []string) error {
	parsed := parseArgs(args)

	// Configure logging level.
	logLevel := slog.LevelInfo
	if parsed.debug {
		logLevel = slog.LevelDebug
	}
	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(log)

	switch parsed.subcommand {
	case "install":
		return installService(serviceName, parsed.configPath)
	case "uninstall":
		return uninstallService(serviceName)
	case "start":
		return startService(serviceName)
	case "stop":
		return stopService(serviceName)
	case "run", "":
		return runAgent(parsed, log)
	case "help", "--help", "-h":
		fmt.Print(usageText())
		return nil
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", parsed.subcommand)
		fmt.Print(usageText())
		return fmt.Errorf("unknown subcommand: %s", parsed.subcommand)
	}
}

// runAgent loads config and starts the agent either as a Windows Service or
// in the foreground.
func runAgent(parsed parsedArgs, log *slog.Logger) error {
	cfg, err := agentcfg.Load(parsed.configPath)
	if err != nil {
		return fmt.Errorf("loading config from %s: %w", parsed.configPath, err)
	}

	runFn := func(ctx context.Context) error {
		r := &runner{
			cfg:   cfg,
			log:   log,
			state: pb.AgentState_AGENT_STATE_IDLE,
		}

		if parsed.httpDebug {
			go startDebugServer(ctx, r, log)
		}

		return r.run(ctx)
	}

	if isWindowsService() {
		log.Info("starting as Windows Service", "name", serviceName)
		return runAsWindowsService(serviceName, runFn)
	}

	// Foreground mode: handle interrupt signals.
	log.Info("starting in foreground mode")
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	return runFn(ctx)
}
