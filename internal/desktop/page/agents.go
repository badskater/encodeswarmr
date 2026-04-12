package page

import (
	"context"
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

// AgentsPage renders the list of registered encoding agents.
type AgentsPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	// Data
	agents   []client.Agent
	loading  bool
	errorMsg string

	// Widgets
	refreshBtn widget.Clickable
	list       widget.List
	rowBtns    []widget.Clickable
}

// NewAgentsPage constructs an AgentsPage.
func NewAgentsPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *AgentsPage {
	p := &AgentsPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	return p
}

// OnNavigatedTo loads agents when the page becomes active.
func (p *AgentsPage) OnNavigatedTo(_ map[string]string) {
	p.load()
}

// OnNavigatedFrom is a no-op.
func (p *AgentsPage) OnNavigatedFrom() {}

func (p *AgentsPage) load() {
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
		agents, err := c.ListAgents(context.Background())
		if err != nil {
			p.errorMsg = "Failed to load agents: " + err.Error()
			p.logger.Error("agents load", "err", err)
		} else {
			p.agents = agents
			p.rowBtns = make([]widget.Clickable, len(agents))
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

// Layout renders the agents page.
func (p *AgentsPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.refreshBtn.Clicked(gtx) {
		p.load()
	}

	// Row click handling.
	for i := range p.rowBtns {
		if i < len(p.agents) && p.rowBtns[i].Clicked(gtx) {
			p.router.Push("/agents/detail", map[string]string{"id": p.agents[i].ID})
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
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.layoutTable(gtx, th)
			}),
		)
	})
}

func (p *AgentsPage) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.H5(th, "Agents").Layout(gtx)
		}),
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

// agentCols defines agent table column layout.
var agentCols = []struct {
	title string
	flex  float32
}{
	{"Hostname", 0.20},
	{"IP Address", 0.15},
	{"Status", 0.12},
	{"Version", 0.12},
	{"OS", 0.15},
	{"GPU", 0.15},
	{"Last Heartbeat", 0.13},
}

func (p *AgentsPage) layoutTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.errorMsg != "" {
		lbl := material.Body1(th, p.errorMsg)
		lbl.Color = desktopapp.ColorDanger
		return lbl.Layout(gtx)
	}
	if p.loading && len(p.agents) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.agents) == 0 {
		lbl := material.Body1(th, "No agents registered.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutTableHeader(gtx, th)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.agents),
				func(gtx layout.Context, i int) layout.Dimensions {
					return p.layoutAgentRow(gtx, th, i)
				})
		}),
	)
}

func (p *AgentsPage) layoutTableHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(agentCols))
	for _, col := range agentCols {
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

func (p *AgentsPage) layoutAgentRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	agent := p.agents[i]

	for len(p.rowBtns) <= i {
		p.rowBtns = append(p.rowBtns, widget.Clickable{})
	}

	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.Clickable(gtx, &p.rowBtns[i], func(gtx layout.Context) layout.Dimensions {
			bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
			rr := gtx.Dp(unit.Dp(4))
			paint.FillShape(gtx.Ops, desktopapp.ColorSurface,
				clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

			return layout.Inset{
				Top: unit.Dp(8), Bottom: unit.Dp(8),
				Left: unit.Dp(8), Right: unit.Dp(8),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					// Hostname.
					layout.Flexed(agentCols[0].flex, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							// Status dot.
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								size := gtx.Dp(unit.Dp(8))
								b := image.Rect(0, 0, size, size)
								paint.FillShape(gtx.Ops, desktopapp.StatusColor(agent.Status),
									clip.Ellipse(b).Op(gtx.Ops))
								return layout.Dimensions{Size: image.Pt(size, size)}
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(th, agent.Hostname)
								lbl.MaxLines = 1
								return lbl.Layout(gtx)
							}),
						)
					}),
					// IP address.
					layout.Flexed(agentCols[1].flex, func(gtx layout.Context) layout.Dimensions {
						return material.Body2(th, agent.IPAddress).Layout(gtx)
					}),
					// Status badge.
					layout.Flexed(agentCols[2].flex, func(gtx layout.Context) layout.Dimensions {
						return layoutStatusBadge(gtx, th, agent.Status)
					}),
					// Agent version.
					layout.Flexed(agentCols[3].flex, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, agent.AgentVersion)
						lbl.MaxLines = 1
						return lbl.Layout(gtx)
					}),
					// OS version.
					layout.Flexed(agentCols[4].flex, func(gtx layout.Context) layout.Dimensions {
						os := agent.OSVersion
						if len(os) > 20 {
							os = os[:20]
						}
						lbl := material.Body2(th, os)
						lbl.MaxLines = 1
						return lbl.Layout(gtx)
					}),
					// GPU info.
					layout.Flexed(agentCols[5].flex, func(gtx layout.Context) layout.Dimensions {
						gpu := "-"
						if agent.GPUModel != nil {
							gpu = *agent.GPUModel
							if len(gpu) > 20 {
								gpu = gpu[:20]
							}
						}
						lbl := material.Body2(th, gpu)
						lbl.MaxLines = 1
						return lbl.Layout(gtx)
					}),
					// Last heartbeat.
					layout.Flexed(agentCols[6].flex, func(gtx layout.Context) layout.Dimensions {
						ts := "-"
						if agent.LastHeartbeat != nil {
							ts = agent.LastHeartbeat.Format("01-02 15:04")
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
