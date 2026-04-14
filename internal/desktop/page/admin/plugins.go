package admin

import (
	"context"
	"log/slog"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	desktopapp "github.com/badskater/encodeswarmr/internal/desktop/app"
	"github.com/badskater/encodeswarmr/internal/desktop/client"
	"github.com/badskater/encodeswarmr/internal/desktop/nav"
)

// PluginsPage lists installed plugins and allows toggling them.
type PluginsPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	items    []client.Plugin
	loading  bool
	errorMsg string
	list     widget.List

	refreshBtn widget.Clickable
	toggleBtns []widget.Clickable
}

// NewPluginsPage constructs a PluginsPage.
func NewPluginsPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *PluginsPage {
	p := &PluginsPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	return p
}

func (p *PluginsPage) OnNavigatedTo(_ map[string]string) { p.refresh() }
func (p *PluginsPage) OnNavigatedFrom()                  {}

func (p *PluginsPage) refresh() {
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
		items, err := c.ListPlugins(context.Background())
		if err != nil {
			p.errorMsg = "Failed to load plugins: " + err.Error()
			p.logger.Error("plugins load", "err", err)
		} else {
			p.items = items
			p.toggleBtns = make([]widget.Clickable, len(items))
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *PluginsPage) togglePlugin(plugin client.Plugin) {
	c := p.state.Client()
	if c == nil {
		return
	}
	enable := !plugin.Enabled
	go func() {
		if _, err := c.TogglePlugin(context.Background(), plugin.Name, enable); err != nil {
			p.errorMsg = "Toggle failed: " + err.Error()
			p.logger.Error("plugin toggle", "err", err)
			p.window.Invalidate()
			return
		}
		p.refresh()
	}()
}

// Layout renders the plugins page.
func (p *PluginsPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.refreshBtn.Clicked(gtx) {
		p.refresh()
	}
	for i := range p.toggleBtns {
		if i < len(p.items) && p.toggleBtns[i].Clicked(gtx) {
			p.togglePlugin(p.items[i])
		}
	}

	return layout.Inset{
		Left: unit.Dp(24), Right: unit.Dp(24),
		Top: unit.Dp(16), Bottom: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.H5(th, "Plugins").Layout(gtx)
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

var pluginCols = []colSpec{
	{"Name", 0.20},
	{"Version", 0.10},
	{"Description", 0.35},
	{"Author", 0.15},
	{"Enabled", 0.10},
	{"Action", 0.10},
}

func (p *PluginsPage) layoutTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.loading && len(p.items) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.items) == 0 {
		lbl := material.Body1(th, "No plugins installed.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutColHeader(gtx, th, pluginCols)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.items), func(gtx layout.Context, i int) layout.Dimensions {
				return p.layoutRow(gtx, th, i)
			})
		}),
	)
}

func (p *PluginsPage) layoutRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	pl := p.items[i]
	for len(p.toggleBtns) <= i {
		p.toggleBtns = append(p.toggleBtns, widget.Clickable{})
	}
	author := "-"
	if pl.Author != nil {
		author = *pl.Author
	}
	toggleLabel := "Disable"
	if !pl.Enabled {
		toggleLabel = "Enable"
	}
	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(pluginCols[0].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, pl.Name)
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}),
			layout.Flexed(pluginCols[1].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, pl.Version)
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(pluginCols[2].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, pl.Description)
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(pluginCols[3].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, author)
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(pluginCols[4].flex, func(gtx layout.Context) layout.Dimensions {
				col := desktopapp.ColorDanger
				if pl.Enabled {
					col = desktopapp.ColorSuccess
				}
				lbl := material.Body2(th, boolStr(pl.Enabled))
				lbl.Color = col
				return lbl.Layout(gtx)
			}),
			layout.Flexed(pluginCols[5].flex, func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(th, &p.toggleBtns[i], toggleLabel)
				btn.Background = desktopapp.ColorSecondary
				return btn.Layout(gtx)
			}),
		)
	})
}
