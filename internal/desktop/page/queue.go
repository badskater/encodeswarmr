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

// QueuePage is the queue manager screen.
type QueuePage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	queueStatus *client.QueueStatus
	pendingJobs []client.Job
	loading     bool
	errorMsg    string

	pauseBtn   widget.Clickable
	refreshBtn widget.Clickable
	upBtns     []widget.Clickable
	downBtns   []widget.Clickable
	list       widget.List
}

// NewQueuePage constructs a QueuePage.
func NewQueuePage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *QueuePage {
	p := &QueuePage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	return p
}

// OnNavigatedTo loads queue data when the page becomes active.
func (p *QueuePage) OnNavigatedTo(_ map[string]string) {
	p.load()
}

// OnNavigatedFrom is a no-op.
func (p *QueuePage) OnNavigatedFrom() {}

func (p *QueuePage) load() {
	if p.loading {
		return
	}
	c := p.state.Client()
	if c == nil {
		return
	}
	p.loading = true
	p.errorMsg = ""

	go func() {
		ctx := context.Background()

		status, err := c.GetQueueStatus(ctx)
		if err != nil {
			p.errorMsg = "Failed to load queue status: " + err.Error()
			p.logger.Error("queue status load", "err", err)
		} else {
			p.queueStatus = status
		}

		jobs, err := c.ListJobs(ctx, client.JobQueued, "")
		if err != nil {
			p.logger.Error("queue jobs load", "err", err)
		} else {
			p.pendingJobs = jobs
			n := len(jobs)
			p.upBtns = make([]widget.Clickable, n)
			p.downBtns = make([]widget.Clickable, n)
		}

		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *QueuePage) togglePause() {
	c := p.state.Client()
	if c == nil {
		return
	}

	paused := p.queueStatus != nil && p.queueStatus.Paused

	go func() {
		ctx := context.Background()
		var err error
		if paused {
			_, err = c.ResumeQueue(ctx)
		} else {
			_, err = c.PauseQueue(ctx)
		}
		if err != nil {
			p.errorMsg = "Queue toggle failed: " + err.Error()
			p.logger.Error("queue toggle", "err", err)
			p.window.Invalidate()
			return
		}
		p.load()
	}()
}

func (p *QueuePage) adjustPriority(idx int, delta int) {
	if idx < 0 || idx >= len(p.pendingJobs) {
		return
	}
	c := p.state.Client()
	if c == nil {
		return
	}
	job := p.pendingJobs[idx]
	newPriority := job.Priority + delta
	if newPriority < 0 {
		newPriority = 0
	}

	go func() {
		if err := c.UpdateJobPriority(context.Background(), job.ID, newPriority); err != nil {
			p.errorMsg = "Priority update failed: " + err.Error()
			p.logger.Error("priority update", "err", err)
			p.window.Invalidate()
			return
		}
		p.load()
	}()
}

// Layout renders the queue manager page.
func (p *QueuePage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.refreshBtn.Clicked(gtx) {
		p.load()
	}
	if p.pauseBtn.Clicked(gtx) {
		p.togglePause()
	}

	for i := range p.upBtns {
		if i < len(p.pendingJobs) && p.upBtns[i].Clicked(gtx) {
			p.adjustPriority(i, 1)
		}
	}
	for i := range p.downBtns {
		if i < len(p.pendingJobs) && p.downBtns[i].Clicked(gtx) {
			p.adjustPriority(i, -1)
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
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.layoutStatus(gtx, th)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.errorMsg != "" {
					lbl := material.Body1(th, p.errorMsg)
					lbl.Color = desktopapp.ColorDanger
					return lbl.Layout(gtx)
				}
				return layout.Dimensions{}
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.layoutTable(gtx, th)
			}),
		)
	})
}

func (p *QueuePage) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.H5(th, "Queue Manager").Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			paused := p.queueStatus != nil && p.queueStatus.Paused
			label := "Pause Queue"
			bg := desktopapp.ColorWarning
			if paused {
				label = "Resume Queue"
				bg = desktopapp.ColorSuccess
			}
			btn := material.Button(th, &p.pauseBtn, label)
			btn.Background = bg
			return btn.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := "Refresh"
			if p.loading {
				label = "Loading..."
			}
			btn := material.Button(th, &p.refreshBtn, label)
			btn.Background = desktopapp.ColorSecondary
			return btn.Layout(gtx)
		}),
	)
}

func (p *QueuePage) layoutStatus(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.queueStatus == nil {
		return layout.Dimensions{}
	}
	qs := p.queueStatus

	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Dp(unit.Dp(64)))
	rr := gtx.Dp(unit.Dp(8))
	paint.FillShape(gtx.Ops, desktopapp.ColorSurface,
		clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))
	gtx.Constraints = layout.Exact(bounds.Size())

	return layout.Inset{
		Left:   unit.Dp(16),
		Right:  unit.Dp(16),
		Top:    unit.Dp(12),
		Bottom: unit.Dp(12),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				state := "Running"
				stateColor := desktopapp.ColorSuccess
				if qs.Paused {
					state = "Paused"
					stateColor = desktopapp.ColorWarning
				}
				lbl := material.Body1(th, "State: "+state)
				lbl.Color = stateColor
				return lbl.Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return material.Body2(th, fmt.Sprintf("Pending: %d", qs.Pending)).Layout(gtx)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return material.Body2(th, fmt.Sprintf("Running: %d", qs.Running)).Layout(gtx)
			}),
			layout.Flexed(2, func(gtx layout.Context) layout.Dimensions {
				if qs.EstimatedCompletion == "" {
					return layout.Dimensions{}
				}
				lbl := material.Body2(th, "Est. completion: "+qs.EstimatedCompletion)
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
		)
	})
}

// queueCols defines the queue table columns.
var queueCols = []struct {
	title string
	flex  float32
}{
	{"Source Path", 0.50},
	{"Priority", 0.15},
	{"Created", 0.20},
	{"Adjust", 0.15},
}

func (p *QueuePage) layoutTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.loading && len(p.pendingJobs) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.pendingJobs) == 0 {
		lbl := material.Body1(th, "No jobs queued.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutTableHeader(gtx, th)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.pendingJobs),
				func(gtx layout.Context, i int) layout.Dimensions {
					return p.layoutRow(gtx, th, i)
				})
		}),
	)
}

func (p *QueuePage) layoutTableHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(queueCols))
	for _, col := range queueCols {
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

func (p *QueuePage) layoutRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	job := p.pendingJobs[i]

	for len(p.upBtns) <= i {
		p.upBtns = append(p.upBtns, widget.Clickable{})
		p.downBtns = append(p.downBtns, widget.Clickable{})
	}

	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
		rr := gtx.Dp(unit.Dp(4))
		paint.FillShape(gtx.Ops, desktopapp.ColorSurface,
			clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

		return layout.Inset{
			Top: unit.Dp(8), Bottom: unit.Dp(8),
			Left: unit.Dp(8), Right: unit.Dp(8),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				// Source path.
				layout.Flexed(queueCols[0].flex, func(gtx layout.Context) layout.Dimensions {
					path := job.SourcePath
					if len(path) > 60 {
						path = "..." + path[len(path)-57:]
					}
					lbl := material.Body2(th, path)
					lbl.MaxLines = 1
					return lbl.Layout(gtx)
				}),
				// Priority.
				layout.Flexed(queueCols[1].flex, func(gtx layout.Context) layout.Dimensions {
					return material.Body2(th, fmt.Sprintf("%d", job.Priority)).Layout(gtx)
				}),
				// Created at.
				layout.Flexed(queueCols[2].flex, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, job.CreatedAt.Format("2006-01-02 15:04"))
					lbl.Color = desktopapp.ColorTextLight
					return lbl.Layout(gtx)
				}),
				// Up/Down buttons.
				layout.Flexed(queueCols[3].flex, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							btn := material.Button(th, &p.upBtns[i], "▲")
							btn.Background = desktopapp.ColorSecondary
							return btn.Layout(gtx)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							btn := material.Button(th, &p.downBtns[i], "▼")
							btn.Background = desktopapp.ColorSecondary
							return btn.Layout(gtx)
						}),
					)
				}),
			)
		})
	})
}
