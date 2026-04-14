package page

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"strings"

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

// jobFilterTab describes a single status-filter tab.
type jobFilterTab struct {
	label  string
	status string // empty string means "all"
}

var jobFilterTabs = []jobFilterTab{
	{label: "All", status: ""},
	{label: "Queued", status: client.JobQueued},
	{label: "Running", status: client.JobRunning},
	{label: "Completed", status: client.JobCompleted},
	{label: "Failed", status: client.JobFailed},
	{label: "Cancelled", status: client.JobCancelled},
}

// JobsPage renders the full jobs list with status filter tabs.
type JobsPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	// Data
	jobs     []client.Job
	loading  bool
	errorMsg string

	// Filter state – index into jobFilterTabs.
	activeTab  int
	filterBtns []widget.Clickable

	// Widgets
	refreshBtn widget.Clickable
	list       widget.List
	rowBtns    []widget.Clickable
}

// NewJobsPage constructs a JobsPage.
func NewJobsPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *JobsPage {
	p := &JobsPage{
		state:      state,
		router:     router,
		window:     w,
		logger:     logger,
		filterBtns: make([]widget.Clickable, len(jobFilterTabs)),
	}
	p.list.Axis = layout.Vertical
	return p
}

// OnNavigatedTo loads jobs when the page becomes active.
func (p *JobsPage) OnNavigatedTo(_ map[string]string) {
	p.load()
}

// OnNavigatedFrom is a no-op.
func (p *JobsPage) OnNavigatedFrom() {}

func (p *JobsPage) load() {
	if p.loading {
		return
	}
	c := p.state.Client()
	if c == nil {
		return
	}
	p.loading = true
	p.errorMsg = ""

	statusFilter := jobFilterTabs[p.activeTab].status

	go func() {
		jobs, err := c.ListJobs(context.Background(), statusFilter, "")
		if err != nil {
			p.errorMsg = "Failed to load jobs: " + err.Error()
			p.logger.Error("jobs load", "err", err)
		} else {
			p.jobs = jobs
			p.rowBtns = make([]widget.Clickable, len(jobs))
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

// Layout renders the jobs page.
func (p *JobsPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	// Handle refresh button.
	if p.refreshBtn.Clicked(gtx) {
		p.load()
	}

	// Handle filter tab clicks.
	for i := range p.filterBtns {
		if p.filterBtns[i].Clicked(gtx) && i != p.activeTab {
			p.activeTab = i
			p.jobs = nil
			p.load()
		}
	}

	// Handle row clicks.
	for i := range p.rowBtns {
		if i < len(p.jobs) && p.rowBtns[i].Clicked(gtx) {
			p.router.Push("/jobs/detail", map[string]string{"id": p.jobs[i].ID})
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
				return p.layoutFilterTabs(gtx, th)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.layoutTable(gtx, th)
			}),
		)
	})
}

func (p *JobsPage) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.H5(th, "Jobs").Layout(gtx)
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

func (p *JobsPage) layoutFilterTabs(gtx layout.Context, th *material.Theme) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(jobFilterTabs))
	for i := range jobFilterTabs {
		i := i
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return p.layoutFilterTab(gtx, th, i)
			})
		}))
	}
	return layout.Flex{}.Layout(gtx, children...)
}

func (p *JobsPage) layoutFilterTab(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	active := i == p.activeTab
	bg := desktopapp.ColorSurface
	fg := desktopapp.ColorTextLight
	if active {
		bg = desktopapp.ColorPrimary
		fg = desktopapp.ColorBackground
	}

	return widget.Border{
		Color:        desktopapp.ColorBorder,
		CornerRadius: unit.Dp(4),
		Width:        unit.Dp(1),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.Clickable(gtx, &p.filterBtns[i], func(gtx layout.Context) layout.Dimensions {
			// Fill background.
			bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
			rr := gtx.Dp(unit.Dp(4))
			paint.FillShape(gtx.Ops, bg,
				clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

			return layout.Inset{
				Top: unit.Dp(6), Bottom: unit.Dp(6),
				Left: unit.Dp(12), Right: unit.Dp(12),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, jobFilterTabs[i].label)
				lbl.Color = fg
				return lbl.Layout(gtx)
			})
		})
	})
}

func (p *JobsPage) layoutTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.errorMsg != "" {
		lbl := material.Body1(th, p.errorMsg)
		lbl.Color = desktopapp.ColorDanger
		return lbl.Layout(gtx)
	}
	if p.loading && len(p.jobs) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.jobs) == 0 {
		lbl := material.Body1(th, "No jobs found.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}

	// Column header widths (fractional).
	cols := []struct {
		title string
		flex  float32
	}{
		{"Source Path", 0.30},
		{"Type", 0.10},
		{"Status", 0.12},
		{"Priority", 0.08},
		{"Progress", 0.15},
		{"Created", 0.17},
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Header row.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutTableHeader(gtx, th, cols)
		}),
		// Data rows.
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.jobs),
				func(gtx layout.Context, i int) layout.Dimensions {
					return p.layoutJobRow(gtx, th, i, cols)
				})
		}),
	)
}

func (p *JobsPage) layoutTableHeader(gtx layout.Context, th *material.Theme, cols []struct {
	title string
	flex  float32
}) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(cols))
	for _, col := range cols {
		col := col
		children = append(children, layout.Flexed(col.flex, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(th, col.title)
					lbl.Color = desktopapp.ColorTextLight
					return lbl.Layout(gtx)
				})
		}))
	}
	return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx, children...)
	})
}

func (p *JobsPage) layoutJobRow(gtx layout.Context, th *material.Theme, i int, cols []struct {
	title string
	flex  float32
}) layout.Dimensions {
	job := p.jobs[i]

	// Ensure row button slice is big enough.
	for len(p.rowBtns) <= i {
		p.rowBtns = append(p.rowBtns, widget.Clickable{})
	}

	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.Clickable(gtx, &p.rowBtns[i], func(gtx layout.Context) layout.Dimensions {
			// Row background.
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
					layout.Flexed(cols[0].flex, func(gtx layout.Context) layout.Dimensions {
						path := job.SourcePath
						if len(path) > 45 {
							path = "..." + path[len(path)-42:]
						}
						lbl := material.Body2(th, path)
						lbl.MaxLines = 1
						return lbl.Layout(gtx)
					}),
					// Job type.
					layout.Flexed(cols[1].flex, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, job.JobType)
						lbl.MaxLines = 1
						return lbl.Layout(gtx)
					}),
					// Status badge.
					layout.Flexed(cols[2].flex, func(gtx layout.Context) layout.Dimensions {
						return layoutStatusBadge(gtx, th, job.Status)
					}),
					// Priority.
					layout.Flexed(cols[3].flex, func(gtx layout.Context) layout.Dimensions {
						return material.Body2(th, fmt.Sprintf("%d", job.Priority)).Layout(gtx)
					}),
					// Progress.
					layout.Flexed(cols[4].flex, func(gtx layout.Context) layout.Dimensions {
						progress := fmt.Sprintf("%d / %d", job.TasksCompleted, job.TasksTotal)
						lbl := material.Body2(th, progress)
						return lbl.Layout(gtx)
					}),
					// Created at.
					layout.Flexed(cols[5].flex, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, job.CreatedAt.Format("2006-01-02 15:04"))
						lbl.Color = desktopapp.ColorTextLight
						return lbl.Layout(gtx)
					}),
				)
			})
		})
	})
}

// layoutStatusBadge renders a small coloured pill for the given status string.
// It is a package-level helper shared across pages in this package.
func layoutStatusBadge(gtx layout.Context, th *material.Theme, status string) layout.Dimensions {
	col := desktopapp.StatusColor(status)
	label := strings.ToUpper(status[:1]) + status[1:]

	return layout.Inset{Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return widget.Border{
			Color:        col,
			CornerRadius: unit.Dp(3),
			Width:        unit.Dp(1),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top: unit.Dp(2), Bottom: unit.Dp(2),
				Left: unit.Dp(6), Right: unit.Dp(6),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, label)
				lbl.Color = col
				return lbl.Layout(gtx)
			})
		})
	})
}
