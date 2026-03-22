package grpc

import (
	"io"
	"log/slog"

	pb "github.com/badskater/encodeswarmr/internal/proto/encoderv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ReportProgress receives a stream of ProgressUpdate messages from an agent
// while it is encoding a task. The first update for each task triggers a
// status transition to "running" in the database.
func (s *Server) ReportProgress(stream grpc.ClientStreamingServer[pb.ProgressUpdate, pb.Ack]) error {
	ctx := stream.Context()
	seen := make(map[string]struct{})

	for {
		update, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.Ack{Ok: true})
		}
		if err != nil {
			return status.Errorf(codes.Internal, "grpc reportprogress: recv: %v", err)
		}

		s.logger.LogAttrs(ctx, slog.LevelDebug, "progress update",
			slog.String("task_id", update.GetTaskId()),
			slog.String("job_id", update.GetJobId()),
			slog.Int64("frame", update.GetFrame()),
			slog.Int64("total_frames", update.GetTotalFrames()),
			slog.Float64("fps", float64(update.GetFps())),
			slog.Int64("eta_sec", int64(update.GetEtaSec())),
		)

		// Mark task as running on the first progress update we see for it.
		if _, ok := seen[update.GetTaskId()]; !ok {
			seen[update.GetTaskId()] = struct{}{}
			if err := s.store.UpdateTaskStatus(ctx, update.GetTaskId(), "running"); err != nil {
				s.logger.LogAttrs(ctx, slog.LevelError, "failed to update task status",
					slog.String("task_id", update.GetTaskId()),
					slog.String("error", err.Error()),
				)
				return status.Errorf(codes.Internal, "grpc reportprogress: update task status: %v", err)
			}
		}
	}
}
