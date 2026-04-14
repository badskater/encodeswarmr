package page

import (
	"context"
	"fmt"
	"image"
	"log/slog"

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
)

// JobDetailPage renders the detail view for a single job and its tasks.
type JobDetailPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	// Route params.
	jobID string

	// Data
	detail   *client.JobDetail
	loading  bool
	errorMsg string

	// Action state.
	cancelInFlight bool
	retryInFlight  bool
	actionError    string

	// Widgets
	backBtn    widget.Clickable
	cancelBtn  widget.Clickable
	retryBtn   widget.Clickable
	taskList   widget.List
	taskBtns   []widget.Clickable
}

// NewJobDetailPage constructs a JobDetailPage.
func NewJobDetailPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *JobDetailPage {
	p := &JobDetailPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.taskList.Axis = layout.Vertical
	return p
}

// OnNavigatedTo loads the job when the page becomes active.
func (p *JobDetailPage) OnNavigatedTo(params map[string]string) {
	p.jobID = params["id"]
	p.detail = nil
	p.errorMsg = ""
	p.actionError = ""
	p.load()
}

// OnNavigatedFrom is a no-op.
func (p *JobDetailPage) OnNavigatedFrom() {}

func (p *JobDetailPage) load() {
	if p.loading || p.jobID == "" {
		return
	}
	c := p.state.Client()
	if c == nil {
		return
	}
	p.loading = true

	go func() {
		detail, err := c.GetJob(context.Background(), p.jobID)
		if err != nil {
			p.errorMsg = "Failed to load job: " + err.Error()
			p.logger.Error("job detail load", "id", p.jobID, "err", err)
		} else {
			p.detail = detail
			p.taskBtns = make([]widget.Clickable, len(detail.Tasks))
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *JobDetailPage) doCancel() {
	if p.cancelInFlight || p.detail == nil {
		return
	}
	c := p.state.Client()
	if c == nil {
		return
	}
	p.cancelInFlight = true

	go func() {
		err := c.CancelJob(context.Background(), p.jobID)
		if err != nil {
			p.actionError = "Cancel failed: " + err.Error()
			p.logger.Error("job cancel", "id", p.jobID, "err", err)
		} else {
			p.actionError = ""
			p.load()
		}
		p.cancelInFlight = false
		p.window.Invalidate()
	}()
}

func (p *JobDetailPage) doRetry() {
	if p.retryInFlight || p.detail == nil {
		return
	}
	c := p.state.Client()
	if c == nil {
		return
	}
	p.retryInFlight = true

	go func() {
		err := c.RetryJob(context.Background(), p.jobID)
		if err != nil {
			p.actionError = "Retry failed: " + err.Error()
			p.logger.Error("job retry", "id", p.jobID, "err", err)
		} else {
			p.actionError = ""
			p.load()
		}
		p.retryInFlight = false
		p.window.Invalidate()
	}()
}

// Layout renders the job detail page.
func (p *JobDetailPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.backBtn.Clicked(gtx) {
		p.router.Pop()
	}
	if p.cancelBtn.Clicked(gtx) {
		p.doCancel()
	}
	if p.retryBtn.Clicked(gtx) {
		p.doRetry()
	}

	// Task row click handling.
	if p.detail != nil {
		for i := range p.taskBtns {
			if i < len(p.detail.Tasks) && p.taskBtns[i].Clicked(gtx) {
				p.router.Push("/tasks/detail", map[string]string{"id": p.detail.Tasks[i].ID})
			}
		}
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

func (p *JobDetailPage) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	title := "Job Detail"
	if p.jobID != "" {
		id := p.jobID
		if len(id) > 8 {
			id = id[:8]
		}
		title = "Job " + id
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
			if p.detail == nil {
				return layout.Dimensions{}
			}
			label := "Cancel"
			if p.cancelInFlight {
				label = "Cancelling..."
			}
			btn := material.Button(th, &p.cancelBtn, label)
			btn.Background = desktopapp.ColorDanger
			return btn.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if p.detail == nil {
				return layout.Dimensions{}
			}
			label := "Retry"
			if p.retryInFlight {
				label = "Retrying..."
			}
			btn := material.Button(th, &p.retryBtn, label)
			btn.Background = desktopapp.ColorWarning
			return btn.Layout(gtx)
		}),
	)
}

func (p *JobDetailPage) layoutBody(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.errorMsg != "" {
		lbl := material.Body1(th, p.errorMsg)
		lbl.Color = desktopapp.ColorDanger
		return lbl.Layout(gtx)
	}
	if p.loading && p.detail == nil {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if p.detail == nil {
		return layout.Dimensions{}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutInfoCard(gtx, th)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if p.actionError != "" {
				lbl := material.Body2(th, p.actionError)
				lbl.Color = desktopapp.ColorDanger
				return lbl.Layout(gtx)
			}
			return layout.Dimensions{}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(th, "Tasks").Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutTaskTableHeader(gtx, th)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return p.layoutTaskTable(gtx, th)
		}),
	)
}

func (p *JobDetailPage) layoutInfoCard(gtx layout.Context, th *material.Theme) layout.Dimensions {
	job := p.detail.Job

	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Dp(unit.Dp(140)))
	rr := gtx.Dp(unit.Dp(8))
	paint.FillShape(gtx.Ops, desktopapp.ColorSurface,
		clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))
	gtx.Constraints = layout.Exact(bounds.Size())

	return layout.Inset{
		Top: unit.Dp(16), Bottom: unit.Dp(16),
		Left: unit.Dp(16), Right: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx,
			// Left column.
			layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.layoutInfoRow(gtx, th, "Source", job.SourcePath)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.layoutInfoRow(gtx, th, "Type", job.JobType)
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
								return layoutStatusBadge(gtx, th, job.Status)
							}),
						)
					}),
				)
			}),
			// Right column.
			layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.layoutInfoRow(gtx, th, "Priority", fmt.Sprintf("%d", job.Priority))
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.layoutInfoRow(gtx, th, "Progress",
							fmt.Sprintf("%d / %d tasks", job.TasksCompleted, job.TasksTotal))
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.layoutInfoRow(gtx, th, "Created", job.CreatedAt.Format("2006-01-02 15:04:05"))
					}),
				)
			}),
		)
	})
}

func (p *JobDetailPage) layoutInfoRow(gtx layout.Context, th *material.Theme, label, value string) layout.Dimensions {
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

// taskCols defines the task table columns.
var taskCols = []struct {
	title string
	flex  float32
}{
	{"Chunk", 0.08},
	{"Agent", 0.15},
	{"Status", 0.12},
	{"Frames", 0.10},
	{"FPS", 0.08},
	{"Started", 0.17},
	{"Completed", 0.17},
}

func (p *JobDetailPage) layoutTaskTableHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(taskCols))
	for _, col := range taskCols {
		col := col
		children = append(children, layout.Flexed(col.flex, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, col.title)
			lbl.Color = desktopapp.ColorTextLight
			return lbl.Layout(gtx)
		}))
	}
	return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx, children...)
	})
}

func (p *JobDetailPage) layoutTaskTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	tasks := p.detail.Tasks
	if len(tasks) == 0 {
		lbl := material.Body2(th, "No tasks yet.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}

	return material.List(th, &p.taskList).Layout(gtx, len(tasks),
		func(gtx layout.Context, i int) layout.Dimensions {
			return p.layoutTaskRow(gtx, th, i, tasks)
		})
}

func (p *JobDetailPage) layoutTaskRow(gtx layout.Context, th *material.Theme, i int, tasks []client.Task) layout.Dimensions {
	t := tasks[i]

	for len(p.taskBtns) <= i {
		p.taskBtns = append(p.taskBtns, widget.Clickable{})
	}

	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.Clickable(gtx, &p.taskBtns[i], func(gtx layout.Context) layout.Dimensions {
			bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
			rr := gtx.Dp(unit.Dp(4))
			paint.FillShape(gtx.Ops, desktopapp.ColorSurface,
				clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

			return layout.Inset{
				Top: unit.Dp(6), Bottom: unit.Dp(6),
				Left: unit.Dp(8), Right: unit.Dp(8),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					// Chunk index.
					layout.Flexed(taskCols[0].flex, func(gtx layout.Context) layout.Dimensions {
						return material.Body2(th, fmt.Sprintf("%d", t.ChunkIndex)).Layout(gtx)
					}),
					// Agent ID (short).
					layout.Flexed(taskCols[1].flex, func(gtx layout.Context) layout.Dimensions {
						agent := "-"
						if t.AgentID != nil {
							id := *t.AgentID
							if len(id) > 8 {
								id = id[:8]
							}
							agent = id
						}
						lbl := material.Body2(th, agent)
						lbl.MaxLines = 1
						return lbl.Layout(gtx)
					}),
					// Status badge.
					layout.Flexed(taskCols[2].flex, func(gtx layout.Context) layout.Dimensions {
						return layoutStatusBadge(gtx, th, t.Status)
					}),
					// Frames encoded.
					layout.Flexed(taskCols[3].flex, func(gtx layout.Context) layout.Dimensions {
						frames := "-"
						if t.FramesEncoded != nil {
							frames = fmt.Sprintf("%d", *t.FramesEncoded)
						}
						return material.Body2(th, frames).Layout(gtx)
					}),
					// Avg FPS.
					layout.Flexed(taskCols[4].flex, func(gtx layout.Context) layout.Dimensions {
						fps := "-"
						if t.AvgFPS != nil {
							fps = fmt.Sprintf("%.1f", *t.AvgFPS)
						}
						return material.Body2(th, fps).Layout(gtx)
					}),
					// Started at.
					layout.Flexed(taskCols[5].flex, func(gtx layout.Context) layout.Dimensions {
						ts := "-"
						if t.StartedAt != nil {
							ts = t.StartedAt.Format("01-02 15:04")
						}
						lbl := material.Body2(th, ts)
						lbl.Color = desktopapp.ColorTextLight
						return lbl.Layout(gtx)
					}),
					// Completed at.
					layout.Flexed(taskCols[6].flex, func(gtx layout.Context) layout.Dimensions {
						ts := "-"
						if t.CompletedAt != nil {
							ts = t.CompletedAt.Format("01-02 15:04")
						}
						lbl := material.Body2(th, ts)
						lbl.Color = desktopapp.ColorTextLight
						return lbl.Layout(gtx)
					}),
				)
			})
		})
	})
}
