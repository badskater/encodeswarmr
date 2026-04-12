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

// AgentDetailPage renders the detail view for a single agent.
type AgentDetailPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	// Route params.
	agentID string

	// Data
	health      *client.AgentHealthResponse
	recentTasks []client.RecentTask
	loading     bool
	errorMsg    string

	// Action state.
	drainInFlight   bool
	approveInFlight bool
	actionError     string

	// Widgets
	backBtn    widget.Clickable
	drainBtn   widget.Clickable
	approveBtn widget.Clickable
	taskList   widget.List
}

// NewAgentDetailPage constructs an AgentDetailPage.
func NewAgentDetailPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *AgentDetailPage {
	p := &AgentDetailPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.taskList.Axis = layout.Vertical
	return p
}

// OnNavigatedTo loads agent data when the page becomes active.
func (p *AgentDetailPage) OnNavigatedTo(params map[string]string) {
	p.agentID = params["id"]
	p.health = nil
	p.recentTasks = nil
	p.errorMsg = ""
	p.actionError = ""
	p.load()
}

// OnNavigatedFrom is a no-op.
func (p *AgentDetailPage) OnNavigatedFrom() {}

func (p *AgentDetailPage) load() {
	if p.loading || p.agentID == "" {
		return
	}
	c := p.state.Client()
	if c == nil {
		return
	}
	p.loading = true

	go func() {
		ctx := context.Background()

		health, err := c.GetAgentHealth(ctx, p.agentID)
		if err != nil {
			p.errorMsg = "Failed to load agent: " + err.Error()
			p.logger.Error("agent detail load", "id", p.agentID, "err", err)
			p.loading = false
			p.window.Invalidate()
			return
		}
		p.health = health

		tasks, err := c.ListAgentRecentTasks(ctx, p.agentID, 20)
		if err != nil {
			p.logger.Warn("agent recent tasks load", "id", p.agentID, "err", err)
		} else {
			p.recentTasks = tasks
		}

		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *AgentDetailPage) doDrain() {
	if p.drainInFlight || p.health == nil {
		return
	}
	c := p.state.Client()
	if c == nil {
		return
	}
	p.drainInFlight = true

	go func() {
		err := c.DrainAgent(context.Background(), p.agentID)
		if err != nil {
			p.actionError = "Drain failed: " + err.Error()
			p.logger.Error("agent drain", "id", p.agentID, "err", err)
		} else {
			p.actionError = ""
			p.load()
		}
		p.drainInFlight = false
		p.window.Invalidate()
	}()
}

func (p *AgentDetailPage) doApprove() {
	if p.approveInFlight || p.health == nil {
		return
	}
	c := p.state.Client()
	if c == nil {
		return
	}
	p.approveInFlight = true

	go func() {
		err := c.ApproveAgent(context.Background(), p.agentID)
		if err != nil {
			p.actionError = "Approve failed: " + err.Error()
			p.logger.Error("agent approve", "id", p.agentID, "err", err)
		} else {
			p.actionError = ""
			p.load()
		}
		p.approveInFlight = false
		p.window.Invalidate()
	}()
}

// Layout renders the agent detail page.
func (p *AgentDetailPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.backBtn.Clicked(gtx) {
		p.router.Pop()
	}
	if p.drainBtn.Clicked(gtx) {
		p.doDrain()
	}
	if p.approveBtn.Clicked(gtx) {
		p.doApprove()
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
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.layoutBody(gtx, th)
			}),
		)
	})
}

func (p *AgentDetailPage) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	title := "Agent Detail"
	if p.health != nil {
		title = "Agent: " + p.health.Agent.Hostname
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
	)
}

func (p *AgentDetailPage) layoutBody(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.errorMsg != "" {
		lbl := material.Body1(th, p.errorMsg)
		lbl.Color = desktopapp.ColorDanger
		return lbl.Layout(gtx)
	}
	if p.loading && p.health == nil {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if p.health == nil {
		return layout.Dimensions{}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Info card.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutInfoCard(gtx, th)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		// Action buttons.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutActions(gtx, th)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if p.actionError != "" {
				return layout.Inset{Top: unit.Dp(4)}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, p.actionError)
						lbl.Color = desktopapp.ColorDanger
						return lbl.Layout(gtx)
					})
			}
			return layout.Dimensions{}
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
		// Encoding stats.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutStats(gtx, th)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
		// Recent tasks.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(th, "Recent Tasks").Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutRecentTaskHeader(gtx, th)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return p.layoutRecentTaskTable(gtx, th)
		}),
	)
}

func (p *AgentDetailPage) layoutInfoCard(gtx layout.Context, th *material.Theme) layout.Dimensions {
	agent := p.health.Agent
	cardH := gtx.Dp(unit.Dp(160))
	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, cardH)
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
						return p.layoutInfoRow(gtx, th, "Hostname", agent.Hostname)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.layoutInfoRow(gtx, th, "IP", agent.IPAddress)
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
								return layoutStatusBadge(gtx, th, agent.Status)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.layoutInfoRow(gtx, th, "OS", agent.OSVersion)
					}),
				)
			}),
			// Right column.
			layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.layoutInfoRow(gtx, th, "CPUs", fmt.Sprintf("%d", agent.CPUCount))
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						ramGB := float64(agent.RAMMIB) / 1024
						return p.layoutInfoRow(gtx, th, "RAM", fmt.Sprintf("%.1f GB", ramGB))
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gpu := "-"
						if agent.GPUModel != nil {
							gpu = *agent.GPUModel
						}
						return p.layoutInfoRow(gtx, th, "GPU", gpu)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						hwAccel := formatHWAccel(agent)
						return p.layoutInfoRow(gtx, th, "HW Accel", hwAccel)
					}),
				)
			}),
		)
	})
}

// formatHWAccel returns a comma-separated string of available HW acceleration capabilities.
func formatHWAccel(a client.Agent) string {
	var caps []byte
	if a.NVENC {
		if len(caps) > 0 {
			caps = append(caps, ',', ' ')
		}
		caps = append(caps, "NVENC"...)
	}
	if a.QSV {
		if len(caps) > 0 {
			caps = append(caps, ',', ' ')
		}
		caps = append(caps, "QSV"...)
	}
	if a.AMF {
		if len(caps) > 0 {
			caps = append(caps, ',', ' ')
		}
		caps = append(caps, "AMF"...)
	}
	if len(caps) == 0 {
		return "None"
	}
	return string(caps)
}

func (p *AgentDetailPage) layoutInfoRow(gtx layout.Context, th *material.Theme, label, value string) layout.Dimensions {
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

func (p *AgentDetailPage) layoutActions(gtx layout.Context, th *material.Theme) layout.Dimensions {
	agent := p.health.Agent
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := "Drain"
			if p.drainInFlight {
				label = "Draining..."
			}
			btn := material.Button(th, &p.drainBtn, label)
			btn.Background = desktopapp.ColorWarning
			return btn.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// Show Approve button only for pending-approval agents.
			if agent.Status != client.AgentPendingApproval {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				label := "Approve"
				if p.approveInFlight {
					label = "Approving..."
				}
				btn := material.Button(th, &p.approveBtn, label)
				btn.Background = desktopapp.ColorSuccess
				return btn.Layout(gtx)
			})
		}),
	)
}

func (p *AgentDetailPage) layoutStats(gtx layout.Context, th *material.Theme) layout.Dimensions {
	stats := p.health.EncodingStats

	cardH := gtx.Dp(unit.Dp(90))
	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, cardH)
	rr := gtx.Dp(unit.Dp(8))
	paint.FillShape(gtx.Ops, desktopapp.ColorSurface,
		clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))
	gtx.Constraints = layout.Exact(bounds.Size())

	return layout.Inset{
		Top: unit.Dp(12), Bottom: unit.Dp(12),
		Left: unit.Dp(16), Right: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Spacing: layout.SpaceEvenly}.Layout(gtx,
			layout.Flexed(0.25, func(gtx layout.Context) layout.Dimensions {
				return p.layoutStatCell(gtx, th, "Total Tasks",
					fmt.Sprintf("%d", stats.TotalTasks))
			}),
			layout.Flexed(0.25, func(gtx layout.Context) layout.Dimensions {
				return p.layoutStatCell(gtx, th, "Completed",
					fmt.Sprintf("%d", stats.CompletedTasks))
			}),
			layout.Flexed(0.25, func(gtx layout.Context) layout.Dimensions {
				return p.layoutStatCell(gtx, th, "Failed",
					fmt.Sprintf("%d", stats.FailedTasks))
			}),
			layout.Flexed(0.25, func(gtx layout.Context) layout.Dimensions {
				return p.layoutStatCell(gtx, th, "Avg FPS",
					fmt.Sprintf("%.1f", stats.AvgFPS))
			}),
		)
	})
}

func (p *AgentDetailPage) layoutStatCell(gtx layout.Context, th *material.Theme, label, value string) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(th, value).Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, label)
			lbl.Color = desktopapp.ColorTextLight
			return lbl.Layout(gtx)
		}),
	)
}

// recentTaskCols defines the recent task table columns.
var recentTaskCols = []struct {
	title string
	flex  float32
}{
	{"Chunk", 0.08},
	{"Job ID", 0.20},
	{"Type", 0.12},
	{"Status", 0.12},
	{"FPS", 0.08},
	{"Frames", 0.12},
	{"Updated", 0.17},
}

func (p *AgentDetailPage) layoutRecentTaskHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(recentTaskCols))
	for _, col := range recentTaskCols {
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

func (p *AgentDetailPage) layoutRecentTaskTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if len(p.recentTasks) == 0 {
		lbl := material.Body2(th, "No recent tasks.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}
	return material.List(th, &p.taskList).Layout(gtx, len(p.recentTasks),
		func(gtx layout.Context, i int) layout.Dimensions {
			return p.layoutRecentTaskRow(gtx, th, i)
		})
}

func (p *AgentDetailPage) layoutRecentTaskRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	t := p.recentTasks[i]

	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
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
				layout.Flexed(recentTaskCols[0].flex, func(gtx layout.Context) layout.Dimensions {
					return material.Body2(th, fmt.Sprintf("%d", t.ChunkIndex)).Layout(gtx)
				}),
				// Job ID (short).
				layout.Flexed(recentTaskCols[1].flex, func(gtx layout.Context) layout.Dimensions {
					id := t.JobID
					if len(id) > 12 {
						id = id[:12]
					}
					lbl := material.Body2(th, id)
					lbl.MaxLines = 1
					return lbl.Layout(gtx)
				}),
				// Task type.
				layout.Flexed(recentTaskCols[2].flex, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, t.TaskType)
					lbl.MaxLines = 1
					return lbl.Layout(gtx)
				}),
				// Status badge.
				layout.Flexed(recentTaskCols[3].flex, func(gtx layout.Context) layout.Dimensions {
					return layoutStatusBadge(gtx, th, t.Status)
				}),
				// Avg FPS.
				layout.Flexed(recentTaskCols[4].flex, func(gtx layout.Context) layout.Dimensions {
					fps := "-"
					if t.AvgFPS != nil {
						fps = fmt.Sprintf("%.1f", *t.AvgFPS)
					}
					return material.Body2(th, fps).Layout(gtx)
				}),
				// Frames encoded.
				layout.Flexed(recentTaskCols[5].flex, func(gtx layout.Context) layout.Dimensions {
					frames := "-"
					if t.FramesEncoded != nil {
						frames = fmt.Sprintf("%d", *t.FramesEncoded)
					}
					return material.Body2(th, frames).Layout(gtx)
				}),
				// Updated at.
				layout.Flexed(recentTaskCols[6].flex, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body2(th, t.UpdatedAt.Format("01-02 15:04"))
					lbl.Color = desktopapp.ColorTextLight
					return lbl.Layout(gtx)
				}),
			)
		})
	})
}
