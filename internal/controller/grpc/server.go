package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	pb "github.com/badskater/encodeswarmr/internal/proto/encoderv1"

	"github.com/badskater/encodeswarmr/internal/controller/api"
	"github.com/badskater/encodeswarmr/internal/controller/config"
	"github.com/badskater/encodeswarmr/internal/controller/engine"
	"github.com/badskater/encodeswarmr/internal/controller/webhooks"
	"github.com/badskater/encodeswarmr/internal/db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// LogPublisher is implemented by the API layer to push new task log entries
// to connected WebSocket clients in real-time.
type LogPublisher interface {
	// PublishTaskLog sends a log entry to any WebSocket clients subscribed to
	// the given task ID.
	PublishTaskLog(taskID string, entry any)
}

// Server implements the AgentService gRPC server.
type Server struct {
	pb.UnimplementedAgentServiceServer
	store           db.Store
	cfg             *config.GRPCConfig
	agentCfg        *config.AgentConfig
	validationCfg   engine.ValidationConfig
	logger          *slog.Logger
	webhooks        *webhooks.Service
	concatRunner    engine.ConcatRunner       // optional; triggers controller-side concat
	vmafRunner      engine.VMAFTargetRunner   // optional; triggers controller-side VMAF target encode
	logPublisher    LogPublisher              // optional; pushes log entries to WebSocket clients
}

// New creates a new gRPC Server.
func New(store db.Store, grpcCfg *config.GRPCConfig, agentCfg *config.AgentConfig, logger *slog.Logger, wh *webhooks.Service) *Server {
	return &Server{
		store:         store,
		cfg:           grpcCfg,
		agentCfg:      agentCfg,
		validationCfg: engine.DefaultValidationConfig(),
		logger:        logger,
		webhooks:      wh,
	}
}

// SetValidationConfig attaches output validation configuration.
func (s *Server) SetValidationConfig(cfg engine.ValidationConfig) { s.validationCfg = cfg }

// SetConcatRunner attaches a controller-side concat runner.  When set,
// the final ffmpeg concat step runs on the controller after all chunk tasks
// complete instead of being dispatched to an agent.
func (s *Server) SetConcatRunner(r engine.ConcatRunner) { s.concatRunner = r }

// SetLogPublisher attaches a real-time log publisher so StreamLogs pushes
// entries to WebSocket subscribers immediately after DB insertion.
func (s *Server) SetLogPublisher(p LogPublisher) { s.logPublisher = p }
// SetVMAFTargetRunner attaches a controller-side VMAF target runner.  When set,
// encode_vmaf_target flow tasks run on the controller using an iterative
// binary-search CRF encode loop after their predecessor tasks complete.
func (s *Server) SetVMAFTargetRunner(r engine.VMAFTargetRunner) { s.vmafRunner = r }

// Serve starts the gRPC server and blocks until ctx is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	opts, err := buildServerOptions(s.cfg)
	if err != nil {
		return fmt.Errorf("grpc server options: %w", err)
	}

	opts = append(opts,
		grpc.ChainUnaryInterceptor(s.loggingUnaryInterceptor),
		grpc.ChainStreamInterceptor(s.loggingStreamInterceptor),
	)

	srv := grpc.NewServer(opts...)
	pb.RegisterAgentServiceServer(srv, s)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("grpc listen %s: %w", addr, err)
	}

	s.logger.Info("gRPC server listening", "addr", addr)

	go func() {
		<-ctx.Done()
		s.logger.Info("gRPC server shutting down")
		srv.GracefulStop()
	}()

	if err := srv.Serve(lis); err != nil {
		return fmt.Errorf("grpc serve: %w", err)
	}
	return nil
}

// loggingUnaryInterceptor logs each unary RPC call with method, duration, and error.
func (s *Server) loggingUnaryInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	dur := time.Since(start)

	api.IncrGRPCRequest(info.FullMethod)

	if err != nil {
		s.logger.Warn("gRPC unary call",
			"method", info.FullMethod,
			"duration", dur,
			"error", err,
		)
	} else {
		s.logger.Debug("gRPC unary call",
			"method", info.FullMethod,
			"duration", dur,
		)
	}
	return resp, err
}

// loggingStreamInterceptor logs each streaming RPC call with method, duration, and error.
func (s *Server) loggingStreamInterceptor(
	srv any,
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()
	err := handler(srv, ss)
	dur := time.Since(start)

	api.IncrGRPCRequest(info.FullMethod)

	if err != nil {
		s.logger.Warn("gRPC stream call",
			"method", info.FullMethod,
			"duration", dur,
			"error", err,
		)
	} else {
		s.logger.Debug("gRPC stream call",
			"method", info.FullMethod,
			"duration", dur,
		)
	}
	return err
}

// buildServerOptions constructs the gRPC server options including TLS and keepalive.
func buildServerOptions(cfg *config.GRPCConfig) ([]grpc.ServerOption, error) {
	var opts []grpc.ServerOption

	// Keepalive parameters.
	opts = append(opts,
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     15 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Second,
			Time:                  5 * time.Minute,
			Timeout:               1 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)

	// TLS credentials (optional — empty CertFile means plaintext for dev).
	creds, err := buildTLSCredentials(&cfg.TLS)
	if err != nil {
		return nil, err
	}
	if creds != nil {
		opts = append(opts, grpc.Creds(creds))
	}

	return opts, nil
}

// buildTLSCredentials loads a server certificate and CA for mTLS.
// Returns nil if CertFile is empty (plaintext mode for development).
func buildTLSCredentials(cfg *config.TLSConfig) (credentials.TransportCredentials, error) {
	if cfg.CertFile == "" {
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("grpc tls load keypair: %w", err)
	}

	caCert, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("grpc tls read ca: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("grpc tls: failed to append CA certificate")
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(tlsCfg), nil
}
