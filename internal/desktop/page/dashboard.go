package page

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"log/slog"
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
)

// DashboardPage renders the main overview screen.
type DashboardPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	// Data
	queueSummary *client.QueueSummary
	throughput   []client.ThroughputPoint
	activity     []client.ActivityEvent
	agents       []client.Agent
	loading      bool
	errorMsg     string
	lastRefresh  time.Time

	// Widgets
	refreshBtn widget.Clickable
	list       widget.List
}

// NewDashboardPage constructs a DashboardPage.
func NewDashboardPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *DashboardPage {
	p := &DashboardPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	return p
}

// OnNavigatedTo triggers a data refresh when the page becomes active.
func (p *DashboardPage) OnNavigatedTo(_ map[string]string) {
	p.refresh()
}

// OnNavigatedFrom is a no-op for this page.
func (p *DashboardPage) OnNavigatedFrom() {}

func (p *DashboardPage) refresh() {
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

		qs, err := c.GetQueueSummary(ctx)
		if err != nil {
			p.errorMsg = "Failed to load queue: " + err.Error()
			p.logger.Error("dashboard queue", "err", err)
		} else {
			p.queueSummary = qs
		}

		tp, err := c.GetThroughput(ctx, 24)
		if err != nil {
			p.logger.Error("dashboard throughput", "err", err)
		} else {
			p.throughput = tp
		}

		act, err := c.GetRecentActivity(ctx, 10)
		if err != nil {
			p.logger.Error("dashboard activity", "err", err)
		} else {
			p.activity = act
		}

		agents, err := c.ListAgents(ctx)
		if err != nil {
			p.logger.Error("dashboard agents", "err", err)
		} else {
			p.agents = agents
		}

		p.loading = false
		p.lastRefresh = time.Now()
		p.window.Invalidate()
	}()
}

// Layout renders the dashboard page.
func (p *DashboardPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.refreshBtn.Clicked(gtx) {
		p.refresh()
	}

	return layout.Inset{
		Left:   unit.Dp(24),
		Right:  unit.Dp(24),
		Top:    unit.Dp(16),
		Bottom: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Header row.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.layoutHeader(gtx, th)
			}),
			// Stat cards row.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(16)}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						return p.layoutStatsCards(gtx, th)
					})
			}),
			// Agent overview | Activity feed.
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: unit.Dp(8)}.Layout(gtx,
							func(gtx layout.Context) layout.Dimensions {
								return p.layoutAgentOverview(gtx, th)
							})
					}),
					layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(8)}.Layout(gtx,
							func(gtx layout.Context) layout.Dimensions {
								return p.layoutActivityFeed(gtx, th)
							})
					}),
				)
			}),
		)
	})
}

func (p *DashboardPage) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.H5(th, "Dashboard").Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					user := p.state.User()
					if user == nil {
						return layout.Dimensions{}
					}
					lbl := material.Body2(th, fmt.Sprintf("Welcome, %s", user.Username))
					lbl.Color = desktopapp.ColorTextLight
					return lbl.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := "Refresh"
			if p.loading {
				label = "Loading..."
			}
			btn := material.Button(th, &p.refreshBtn, label)
			btn.Background = desktopapp.ColorPrimary
			return btn.Layout(gtx)
		}),
	)
}

func (p *DashboardPage) layoutStatsCards(gtx layout.Context, th *material.Theme) layout.Dimensions {
	pending := 0
	running := 0
	onlineAgents := 0
	totalAgents := len(p.agents)

	if p.queueSummary != nil {
		pending = p.queueSummary.Pending
		running = p.queueSummary.Running
	}
	for _, a := range p.agents {
		if a.Status == "idle" || a.Status == "running" {
			onlineAgents++
		}
	}

	completed := 0
	for _, t := range p.throughput {
		completed += t.Completed
	}

	return layout.Flex{Spacing: layout.SpaceEvenly}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: unit.Dp(8)}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return p.layoutStatCard(gtx, th, "Pending Jobs", fmt.Sprintf("%d", pending), desktopapp.ColorWarning)
				})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: unit.Dp(8)}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return p.layoutStatCard(gtx, th, "Running Jobs", fmt.Sprintf("%d", running), desktopapp.ColorPrimary)
				})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: unit.Dp(8)}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return p.layoutStatCard(gtx, th, "Agents Online",
						fmt.Sprintf("%d/%d", onlineAgents, totalAgents), desktopapp.ColorSuccess)
				})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return p.layoutStatCard(gtx, th, "Completed (24h)", fmt.Sprintf("%d", completed), desktopapp.ColorSecondary)
		}),
	)
}

func (p *DashboardPage) layoutStatCard(gtx layout.Context, th *material.Theme, title, value string, accentColor color.NRGBA) layout.Dimensions {
	cardH := gtx.Dp(unit.Dp(88))
	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, cardH)
	rr := gtx.Dp(unit.Dp(8))
	paint.FillShape(gtx.Ops, desktopapp.ColorSurface,
		clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

	// Top accent bar.
	accentRect := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Dp(unit.Dp(3)))
	paint.FillShape(gtx.Ops, accentColor,
		clip.RRect{Rect: accentRect, NE: rr, NW: rr}.Op(gtx.Ops))

	gtx.Constraints = layout.Exact(bounds.Size())

	return layout.Inset{
		Left:   unit.Dp(16),
		Right:  unit.Dp(16),
		Top:    unit.Dp(16),
		Bottom: unit.Dp(12),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, title)
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.H5(th, value).Layout(gtx)
			}),
		)
	})
}

func (p *DashboardPage) layoutAgentOverview(gtx layout.Context, th *material.Theme) layout.Dimensions {
	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	rr := gtx.Dp(unit.Dp(8))
	paint.FillShape(gtx.Ops, desktopapp.ColorSurface,
		clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

	return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(16), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.H6(th, "Agent Status").Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if len(p.agents) == 0 {
						lbl := material.Body2(th, "No agents registered")
						lbl.Color = desktopapp.ColorTextLight
						return lbl.Layout(gtx)
					}
					var agentList widget.List
					agentList.Axis = layout.Vertical
					return material.List(th, &agentList).Layout(gtx, len(p.agents),
						func(gtx layout.Context, i int) layout.Dimensions {
							return p.layoutAgentRow(gtx, th, p.agents[i])
						})
				}),
			)
		})
}

func (p *DashboardPage) layoutAgentRow(gtx layout.Context, th *material.Theme, agent client.Agent) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				// Status dot.
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					size := gtx.Dp(unit.Dp(8))
					bounds := image.Rect(0, 0, size, size)
					paint.FillShape(gtx.Ops, desktopapp.StatusColor(agent.Status),
						clip.Ellipse(bounds).Op(gtx.Ops))
					return layout.Dimensions{Size: image.Pt(size, size)}
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				// Hostname.
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return material.Body2(th, agent.Hostname).Layout(gtx)
				}),
				// Status label.
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(th, agent.Status)
					lbl.Color = desktopapp.StatusColor(agent.Status)
					return lbl.Layout(gtx)
				}),
			)
		})
}

func (p *DashboardPage) layoutActivityFeed(gtx layout.Context, th *material.Theme) layout.Dimensions {
	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	rr := gtx.Dp(unit.Dp(8))
	paint.FillShape(gtx.Ops, desktopapp.ColorSurface,
		clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

	return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(16), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.H6(th, "Recent Activity").Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if len(p.activity) == 0 {
						lbl := material.Body2(th, "No recent activity")
						lbl.Color = desktopapp.ColorTextLight
						return lbl.Layout(gtx)
					}
					var actList widget.List
					actList.Axis = layout.Vertical
					return material.List(th, &actList).Layout(gtx, len(p.activity),
						func(gtx layout.Context, i int) layout.Dimensions {
							return p.layoutActivityRow(gtx, th, p.activity[i])
						})
				}),
			)
		})
}

func (p *DashboardPage) layoutActivityRow(gtx layout.Context, th *material.Theme, event client.ActivityEvent) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
						// Status dot.
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							size := gtx.Dp(unit.Dp(8))
							bounds := image.Rect(0, 0, size, size)
							paint.FillShape(gtx.Ops, desktopapp.StatusColor(event.Status),
								clip.Ellipse(bounds).Op(gtx.Ops))
							return layout.Dimensions{Size: image.Pt(size, size)}
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						// Source path (truncated from the left).
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							path := event.SourcePath
							if len(path) > 40 {
								path = "..." + path[len(path)-37:]
							}
							lbl := material.Body2(th, path)
							lbl.MaxLines = 1
							return lbl.Layout(gtx)
						}),
						// Status label.
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							lbl := material.Caption(th, event.Status)
							lbl.Color = desktopapp.StatusColor(event.Status)
							return lbl.Layout(gtx)
						}),
					)
				}),
				// Timestamp.
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(th, event.ChangedAt.Format("15:04:05"))
					lbl.Color = desktopapp.ColorTextLight
					return lbl.Layout(gtx)
				}),
			)
		})
}
