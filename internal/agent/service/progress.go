package service

import (
	"context"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	pb "github.com/badskater/encodeswarmr/internal/proto/encoderv1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// progressMetric holds parsed encoder progress from a single stdout line.
type progressMetric struct {
	Frame       int64
	TotalFrames int64
	FPS         float32
	ETASec      int32
	Percent     float32 // 0-100
}

// Compiled regexes for each encoder's progress output format.
var (
	reX265  = regexp.MustCompile(`\[(\d+\.\d+)%\]\s+(\d+)/(\d+)\s+frames,\s+([\d.]+)\s+fps`)
	reX264  = regexp.MustCompile(`^(\d+)/(\d+)\s+frames,\s+([\d.]+)\s+fps`)
	reSVT   = regexp.MustCompile(`Encoding frame\s+(\d+)/(\d+)`)
	reFFmpeg = regexp.MustCompile(`frame=\s*(\d+)\s+fps=\s*([\d.]+)`)
	reETA   = regexp.MustCompile(`eta\s+(\d+):(\d+):(\d+)`)
)

// parseProgress tries each encoder regex in order and returns the first match,
// or nil if no pattern matches.
func parseProgress(line string) *progressMetric {
	// x265: [12.3%] 1200/9750 frames, 24.50 fps, eta 0:01:23
	if m := reX265.FindStringSubmatch(line); m != nil {
		pct, err := strconv.ParseFloat(m[1], 32)
		if err != nil {
			return nil
		}
		frame, err := strconv.ParseInt(m[2], 10, 64)
		if err != nil {
			return nil
		}
		total, err := strconv.ParseInt(m[3], 10, 64)
		if err != nil {
			return nil
		}
		fps, err := strconv.ParseFloat(m[4], 32)
		if err != nil {
			return nil
		}
		pm := &progressMetric{
			Frame:       frame,
			TotalFrames: total,
			FPS:         float32(fps),
			Percent:     float32(pct),
		}
		// Parse optional ETA from the same line.
		if em := reETA.FindStringSubmatch(line); em != nil {
			h, _ := strconv.Atoi(em[1])
			mi, _ := strconv.Atoi(em[2])
			s, _ := strconv.Atoi(em[3])
			pm.ETASec = int32(h*3600 + mi*60 + s)
		}
		return pm
	}

	// x264: 1200/9750 frames, 48.22 fps
	if m := reX264.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
		frame, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			return nil
		}
		total, err := strconv.ParseInt(m[2], 10, 64)
		if err != nil {
			return nil
		}
		fps, err := strconv.ParseFloat(m[3], 32)
		if err != nil {
			return nil
		}
		var pct float32
		if total > 0 {
			pct = float32(frame) / float32(total) * 100
		}
		return &progressMetric{
			Frame:       frame,
			TotalFrames: total,
			FPS:         float32(fps),
			Percent:     pct,
		}
	}

	// SVT-AV1: Encoding frame 1200/9750
	if m := reSVT.FindStringSubmatch(line); m != nil {
		frame, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			return nil
		}
		total, err := strconv.ParseInt(m[2], 10, 64)
		if err != nil {
			return nil
		}
		var pct float32
		if total > 0 {
			pct = float32(frame) / float32(total) * 100
		}
		return &progressMetric{
			Frame:       frame,
			TotalFrames: total,
			Percent:     pct,
		}
	}

	// FFmpeg: frame= 1200 fps= 24 size=...
	if m := reFFmpeg.FindStringSubmatch(line); m != nil {
		frame, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			return nil
		}
		fps, err := strconv.ParseFloat(m[2], 32)
		if err != nil {
			return nil
		}
		return &progressMetric{
			Frame:       frame,
			TotalFrames: 0,
			FPS:         float32(fps),
		}
	}

	return nil
}

// progressStreamer batches progress updates and flushes to the controller
// every 5 seconds.
type progressStreamer struct {
	client    pb.AgentServiceClient
	taskID    string
	jobID     string
	log       *slog.Logger
	ch        chan *progressMetric
	gpuMetric func() *gpuSample // may be nil
}

// newProgressStreamer creates a new progressStreamer. Send metrics to the
// returned streamer's ch channel; call start to begin flushing.
func newProgressStreamer(client pb.AgentServiceClient, taskID, jobID string, log *slog.Logger, gpuFn func() *gpuSample) *progressStreamer {
	return &progressStreamer{
		client:    client,
		taskID:    taskID,
		jobID:     jobID,
		log:       log,
		ch:        make(chan *progressMetric, 64),
		gpuMetric: gpuFn,
	}
}

// start opens the ReportProgress stream and drains ch in 5-second batches.
// Returns a cancel func. Call cancel() to stop; it waits for the goroutine to
// finish.
func (ps *progressStreamer) start(ctx context.Context) (cancel func()) {
	ctx, ctxCancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		ps.run(ctx)
	}()

	return func() {
		ctxCancel()
		wg.Wait()
	}
}

// run is the internal loop that opens the gRPC stream and sends batched
// progress updates every 5 seconds.
func (ps *progressStreamer) run(ctx context.Context) {
	stream, err := ps.client.ReportProgress(ctx)
	if err != nil {
		ps.log.Error("failed to open progress stream", "error", err)
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if _, err := stream.CloseAndRecv(); err != nil {
				ps.log.Error("closing progress stream", "error", err)
			}
			return
		case <-ticker.C:
			// Drain channel, keep only the latest metric.
			var latest *progressMetric
			for {
				select {
				case m := <-ps.ch:
					latest = m
				default:
					goto drained
				}
			}
		drained:
			if latest == nil {
				continue
			}
			update := &pb.ProgressUpdate{
				TaskId:      ps.taskID,
				JobId:       ps.jobID,
				Frame:       latest.Frame,
				TotalFrames: latest.TotalFrames,
				Fps:         latest.FPS,
				EtaSec:      latest.ETASec,
				Quality:     0,
				Timestamp:   timestamppb.Now(),
			}
			if err := stream.Send(update); err != nil {
				ps.log.Error("sending progress update", "error", err)
				return
			}
		}
	}
}
