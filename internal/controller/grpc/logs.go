package grpc

import (
	"io"
	"log/slog"
	"time"

	"github.com/badskater/encodeswarmr/internal/db"
	pb "github.com/badskater/encodeswarmr/internal/proto/encoderv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// StreamLogs receives a stream of log entries from the agent and persists
// each entry into the task_logs table.
func (s *Server) StreamLogs(stream grpc.ClientStreamingServer[pb.LogEntry, pb.Ack]) error {
	ctx := stream.Context()

	for {
		entry, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.Ack{Ok: true})
		}
		if err != nil {
			return status.Errorf(codes.Internal, "grpc streamlogs: recv: %v", err)
		}

		loggedAt := time.Now()
		if entry.GetTimestamp() != nil {
			loggedAt = entry.GetTimestamp().AsTime()
		}

		p := db.InsertTaskLogParams{
			TaskID:   entry.GetTaskId(),
			JobID:    entry.GetJobId(),
			Stream:   entry.GetStream(),
			Level:    entry.GetLevel(),
			Message:  entry.GetMessage(),
			Metadata: structToMap(entry.GetMetadata()),
			LoggedAt: &loggedAt,
		}

		if err := s.store.InsertTaskLog(ctx, p); err != nil {
			s.logger.LogAttrs(ctx, slog.LevelError, "failed to insert task log",
				slog.String("task_id", entry.GetTaskId()),
				slog.String("error", err.Error()),
			)
			return status.Errorf(codes.Internal, "grpc streamlogs: insert log: %v", err)
		}
	}
}

// structToMap converts a protobuf Struct to a plain Go map.
// Returns nil when the input is nil.
func structToMap(s *structpb.Struct) map[string]any {
	if s == nil {
		return nil
	}
	return s.AsMap()
}
