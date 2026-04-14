package page

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	desktopapp "github.com/badskater/encodeswarmr/internal/desktop/app"
	"github.com/badskater/encodeswarmr/internal/desktop/client"
	"github.com/badskater/encodeswarmr/internal/desktop/nav"
	desktopwidget "github.com/badskater/encodeswarmr/internal/desktop/widget"
)

// ffmpegProgressRE matches key=value pairs in ffmpeg progress output.
// e.g. "fps= 24.5", "frame= 1234", "time=00:05:23.45", "speed= 2.1x"
var ffmpegProgressRE = regexp.MustCompile(`(?:fps|frame|time|speed)\s*=\s*\S+`)

// TaskDetailPage renders the detail view for a single encoding task with live log streaming.
type TaskDetailPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	taskID   string
	task     *client.Task
	logView  *desktopwidget.LogViewer
	loading  bool
	errorMsg string

	// Auto refresh
	refreshTicker *time.Ticker
	stopRefresh   chan struct{}

	// ffmpeg progress
	currentFPS   string
	currentFrame string
	currentTime  string
	currentSpeed string

	backBtn    widget.Clickable
	jobLinkBtn widget.Clickable
}

// NewTaskDetailPage constructs a TaskDetailPage.
func NewTaskDetailPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *TaskDetailPage {
	return &TaskDetailPage{
		state:   state,
		router:  router,
		window:  w,
		logger:  logger,
		logView: desktopwidget.NewLogViewer(),
	}
}

// OnNavigatedTo loads task data and starts the auto-refresh loop for running tasks.
func (p *TaskDetailPage) OnNavigatedTo(params map[string]string) {
	p.taskID = params["id"]
	p.task = nil
	p.errorMsg = ""
	p.currentFPS = ""
	p.currentFrame = ""
	p.currentTime = ""
	p.currentSpeed = ""
	p.logView.SetLines(nil)

	p.load()
}

// OnNavigatedFrom stops the refresh ticker when leaving the page.
func (p *TaskDetailPage) OnNavigatedFrom() {
	p.stopAutoRefresh()
}

func (p *TaskDetailPage) load() {
	if p.taskID == "" {
		return
	}
	c := p.state.Client()
	if c == nil {
		return
	}
	p.loading = true

	go func() {
		ctx := context.Background()

		task, err := c.GetTask(ctx, p.taskID)
		if err != nil {
			p.errorMsg = "Failed to load task: " + err.Error()
			p.logger.Error("task detail load", "id", p.taskID, "err", err)
			p.loading = false
			p.window.Invalidate()
			return
		}
		p.task = task

		logs, err := c.ListTaskLogs(ctx, p.taskID)
		if err != nil {
			p.logger.Warn("task logs load", "id", p.taskID, "err", err)
		} else {
			lines := make([]desktopwidget.LogLine, 0, len(logs))
			for _, entry := range logs {
				line := desktopwidget.LogLine{
					Level:   entry.Level,
					Stream:  entry.Stream,
					Message: entry.Message,
					Time:    entry.Timestamp.Format("15:04:05"),
				}
				lines = append(lines, line)
				p.extractFFmpegProgress(entry.Message)
			}
			p.logView.SetLines(lines)
		}

		p.loading = false
		p.window.Invalidate()

		// Start auto-refresh only if task is still running.
		if task.Status == client.TaskRunning || task.Status == client.TaskAssigned {
			p.startAutoRefresh()
		}
	}()
}

func (p *TaskDetailPage) startAutoRefresh() {
	// Ensure any existing ticker is stopped first.
	p.stopAutoRefresh()

	ticker := time.NewTicker(3 * time.Second)
	stop := make(chan struct{})
	p.refreshTicker = ticker
	p.stopRefresh = stop

	go func() {
		for {
			select {
			case <-ticker.C:
				p.refresh()
			case <-stop:
				ticker.Stop()
				return
			}
		}
	}()
}

func (p *TaskDetailPage) stopAutoRefresh() {
	if p.stopRefresh != nil {
		close(p.stopRefresh)
		p.stopRefresh = nil
		p.refreshTicker = nil
	}
}

func (p *TaskDetailPage) refresh() {
	c := p.state.Client()
	if c == nil {
		return
	}

	ctx := context.Background()

	task, err := c.GetTask(ctx, p.taskID)
	if err != nil {
		p.logger.Warn("task refresh", "id", p.taskID, "err", err)
		p.window.Invalidate()
		return
	}
	p.task = task

	logs, err := c.ListTaskLogs(ctx, p.taskID)
	if err != nil {
		p.logger.Warn("task logs refresh", "id", p.taskID, "err", err)
	} else {
		lines := make([]desktopwidget.LogLine, 0, len(logs))
		for _, entry := range logs {
			line := desktopwidget.LogLine{
				Level:   entry.Level,
				Stream:  entry.Stream,
				Message: entry.Message,
				Time:    entry.Timestamp.Format("15:04:05"),
			}
			lines = append(lines, line)
			p.extractFFmpegProgress(entry.Message)
		}
		p.logView.SetLines(lines)
	}

	// Stop auto-refresh once the task is no longer running.
	if task.Status != client.TaskRunning && task.Status != client.TaskAssigned {
		p.stopAutoRefresh()
	}

	p.window.Invalidate()
}

// extractFFmpegProgress parses a log message for ffmpeg progress key=value pairs.
func (p *TaskDetailPage) extractFFmpegProgress(msg string) {
	matches := ffmpegProgressRE.FindAllString(msg, -1)
	for _, m := range matches {
		parts := strings.SplitN(m, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "fps":
			p.currentFPS = val
		case "frame":
			p.currentFrame = val
		case "time":
			p.currentTime = val
		case "speed":
			p.currentSpeed = val
		}
	}
}

// Layout renders the task detail page.
func (p *TaskDetailPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.backBtn.Clicked(gtx) {
		p.router.Pop()
	}
	if p.jobLinkBtn.Clicked(gtx) && p.task != nil {
		p.router.Push("/jobs/detail", map[string]string{"id": p.task.JobID})
	}

	return layout.Inset{
		Left:   unit.Dp(24),
		Right:  unit.Dp(24),
		Top:    unit.Dp(16),
		Bottom: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.layoutHeader(gtx, th)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.layoutBody(gtx, th)
			}),
		)
	})
}

func (p *TaskDetailPage) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	title := "Task Detail"
	if p.taskID != "" {
		id := p.taskID
		if len(id) > 8 {
			id = id[:8]
		}
		title = "Task " + id
	}

	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(th, &p.backBtn, "< Back")
			btn.Background = desktopapp.ColorSecondary
			return btn.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.H5(th, title).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if p.task == nil {
				return layout.Dimensions{}
			}
			jobID := p.task.JobID
			if len(jobID) > 8 {
				jobID = jobID[:8]
			}
			btn := material.Button(th, &p.jobLinkBtn, "Job "+jobID)
			btn.Background = desktopapp.ColorPrimary
			return btn.Layout(gtx)
		}),
	)
}

func (p *TaskDetailPage) layoutBody(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.errorMsg != "" {
		lbl := material.Body1(th, p.errorMsg)
		lbl.Color = desktopapp.ColorDanger
		return lbl.Layout(gtx)
	}
	if p.loading && p.task == nil {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if p.task == nil {
		return layout.Dimensions{}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutInfoCard(gtx, th)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutFFmpegProgress(gtx, th)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(th, "Logs").Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return p.logView.Layout(gtx, th)
		}),
	)
}

func (p *TaskDetailPage) layoutInfoCard(gtx layout.Context, th *material.Theme) layout.Dimensions {
	t := p.task

	cardH := gtx.Dp(unit.Dp(160))
	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, cardH)
	rr := gtx.Dp(unit.Dp(8))
	paint.FillShape(gtx.Ops, desktopapp.ColorSurface,
		clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))
	gtx.Constraints = layout.Exact(bounds.Size())

	return layout.Inset{
		Top:    unit.Dp(16),
		Bottom: unit.Dp(16),
		Left:   unit.Dp(16),
		Right:  unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx,
			// Left column.
			layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.layoutInfoRow(gtx, th, "Chunk", fmt.Sprintf("%d", t.ChunkIndex))
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						agent := "-"
						if t.AgentID != nil {
							id := *t.AgentID
							if len(id) > 12 {
								id = id[:12]
							}
							agent = id
						}
						return p.layoutInfoRow(gtx, th, "Agent", agent)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Caption(th, "Status")
								lbl.Color = desktopapp.ColorTextLight
								return lbl.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layoutStatusBadge(gtx, th, t.Status)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						retries := fmt.Sprintf("%d", t.RetryCount)
						return p.layoutInfoRow(gtx, th, "Retries", retries)
					}),
				)
			}),
			// Right column.
			layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						frames := "-"
						if t.FramesEncoded != nil {
							frames = fmt.Sprintf("%d", *t.FramesEncoded)
						}
						return p.layoutInfoRow(gtx, th, "Frames", frames)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						fps := "-"
						if t.AvgFPS != nil {
							fps = fmt.Sprintf("%.2f", *t.AvgFPS)
						}
						return p.layoutInfoRow(gtx, th, "Avg FPS", fps)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						exit := "-"
						if t.ExitCode != nil {
							exit = fmt.Sprintf("%d", *t.ExitCode)
						}
						return p.layoutInfoRow(gtx, th, "Exit Code", exit)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						errMsg := "-"
						if t.ErrorMsg != nil && *t.ErrorMsg != "" {
							errMsg = *t.ErrorMsg
						}
						row := p.layoutInfoRow(gtx, th, "Error", errMsg)
						return row
					}),
				)
			}),
		)
	})
}

func (p *TaskDetailPage) layoutInfoRow(gtx layout.Context, th *material.Theme, label, value string) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, label)
			lbl.Color = desktopapp.ColorTextLight
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, value)
			lbl.MaxLines = 1
			return lbl.Layout(gtx)
		}),
	)
}

// layoutFFmpegProgress shows live ffmpeg progress stats if available.
func (p *TaskDetailPage) layoutFFmpegProgress(gtx layout.Context, th *material.Theme) layout.Dimensions {
	// Only show the progress bar if we have at least one ffmpeg metric.
	if p.currentFPS == "" && p.currentFrame == "" && p.currentTime == "" && p.currentSpeed == "" {
		return layout.Dimensions{}
	}

	cardH := gtx.Dp(unit.Dp(52))
	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, cardH)
	rr := gtx.Dp(unit.Dp(8))
	paint.FillShape(gtx.Ops, desktopapp.ColorSurface,
		clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))
	gtx.Constraints = layout.Exact(bounds.Size())

	return layout.Inset{
		Top:    unit.Dp(10),
		Bottom: unit.Dp(10),
		Left:   unit.Dp(16),
		Right:  unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Spacing: layout.SpaceEvenly}.Layout(gtx,
			layout.Flexed(0.25, func(gtx layout.Context) layout.Dimensions {
				return p.layoutProgressCell(gtx, th, "Frame", p.currentFrame)
			}),
			layout.Flexed(0.25, func(gtx layout.Context) layout.Dimensions {
				return p.layoutProgressCell(gtx, th, "FPS", p.currentFPS)
			}),
			layout.Flexed(0.25, func(gtx layout.Context) layout.Dimensions {
				return p.layoutProgressCell(gtx, th, "Time", p.currentTime)
			}),
			layout.Flexed(0.25, func(gtx layout.Context) layout.Dimensions {
				return p.layoutProgressCell(gtx, th, "Speed", p.currentSpeed)
			}),
		)
	})
}

func (p *TaskDetailPage) layoutProgressCell(gtx layout.Context, th *material.Theme, label, value string) layout.Dimensions {
	if value == "" {
		value = "-"
	}
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, value)
			lbl.Color = desktopapp.ColorPrimary
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, label)
			lbl.Color = desktopapp.ColorTextLight
			return lbl.Layout(gtx)
		}),
	)
}
