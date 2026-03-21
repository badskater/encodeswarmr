package grpc

import (
	"io"
	"log/slog"

	pb "github.com/badskater/encodeswarmr/internal/proto/encoderv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SyncOfflineResults is called by an agent after it reconnects following a
// network outage. The agent replays its locally-buffered task results so the
// controller can reconcile state.
func (s *Server) SyncOfflineResults(stream grpc.ClientStreamingServer[pb.TaskResult, pb.SyncResponse]) error {
	ctx := stream.Context()

	var accepted int32
	var rejectedIDs []string

	for {
		result, err := stream.Recv()
		if err == io.EOF {
			s.logger.LogAttrs(ctx, slog.LevelInfo, "offline sync complete",
				slog.Int64("accepted", int64(accepted)),
				slog.Int("rejected", len(rejectedIDs)),
			)
			return stream.SendAndClose(&pb.SyncResponse{
				Accepted:        accepted,
				RejectedTaskIds: rejectedIDs,
			})
		}
		if err != nil {
			return status.Errorf(codes.Internal, "grpc syncoffline: recv: %v", err)
		}

		// Check whether this task was already finalised.
		task, err := s.store.GetTaskByID(ctx, result.GetTaskId())
		if err != nil {
			s.logger.LogAttrs(ctx, slog.LevelError, "failed to look up task for offline sync",
				slog.String("task_id", result.GetTaskId()),
				slog.String("error", err.Error()),
			)
			return status.Errorf(codes.Internal, "grpc syncoffline: get task: %v", err)
		}

		if (task.Status == "completed" || task.Status == "failed") && !result.GetOfflineResult() {
			rejectedIDs = append(rejectedIDs, result.GetTaskId())
			continue
		}

		// Process the result the same way as ReportResult.
		if err := s.processResult(ctx, result); err != nil {
			return err
		}
		if err := s.store.UpdateJobTaskCounts(ctx, result.GetJobId()); err != nil {
			return status.Errorf(codes.Internal, "grpc syncoffline: update job counts: %v", err)
		}
		if err := s.checkJobCompletion(ctx, result.GetJobId()); err != nil {
			return err
		}

		accepted++
	}
}
