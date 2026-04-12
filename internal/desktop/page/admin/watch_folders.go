package admin

import (
	"context"
	"log/slog"
	"strings"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	desktopapp "github.com/badskater/encodeswarmr/internal/desktop/app"
	"github.com/badskater/encodeswarmr/internal/desktop/client"
	"github.com/badskater/encodeswarmr/internal/desktop/nav"
)

// WatchFoldersPage manages watch folder configuration.
type WatchFoldersPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	items    []client.WatchFolder
	loading  bool
	errorMsg string
	infoMsg  string
	list     widget.List

	refreshBtn widget.Clickable
	toggleBtns []widget.Clickable
	scanBtns   []widget.Clickable
}

// NewWatchFoldersPage constructs a WatchFoldersPage.
func NewWatchFoldersPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *WatchFoldersPage {
	p := &WatchFoldersPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	return p
}

func (p *WatchFoldersPage) OnNavigatedTo(_ map[string]string) { p.refresh() }
func (p *WatchFoldersPage) OnNavigatedFrom()                  {}

func (p *WatchFoldersPage) refresh() {
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
		items, err := c.ListWatchFolders(context.Background())
		if err != nil {
			p.errorMsg = "Failed to load watch folders: " + err.Error()
			p.logger.Error("watch folders load", "err", err)
		} else {
			p.items = items
			p.toggleBtns = make([]widget.Clickable, len(items))
			p.scanBtns = make([]widget.Clickable, len(items))
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *WatchFoldersPage) toggleFolder(wf client.WatchFolder) {
	c := p.state.Client()
	if c == nil {
		return
	}
	go func() {
		if err := c.ToggleWatchFolder(context.Background(), wf.Name, !wf.Enabled); err != nil {
			p.errorMsg = "Toggle failed: " + err.Error()
			p.logger.Error("watch folder toggle", "err", err)
			p.window.Invalidate()
			return
		}
		p.refresh()
	}()
}

func (p *WatchFoldersPage) triggerScan(wf client.WatchFolder) {
	c := p.state.Client()
	if c == nil {
		return
	}
	go func() {
		if err := c.ScanWatchFolder(context.Background(), wf.Name); err != nil {
			p.errorMsg = "Scan failed: " + err.Error()
			p.logger.Error("watch folder scan", "err", err)
			p.window.Invalidate()
			return
		}
		p.infoMsg = "Scan triggered for " + wf.Name
		p.window.Invalidate()
	}()
}

// Layout renders the watch folders page.
func (p *WatchFoldersPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.refreshBtn.Clicked(gtx) {
		p.refresh()
	}
	for i := range p.toggleBtns {
		if i < len(p.items) && p.toggleBtns[i].Clicked(gtx) {
			p.toggleFolder(p.items[i])
		}
	}
	for i := range p.scanBtns {
		if i < len(p.items) && p.scanBtns[i].Clicked(gtx) {
			p.triggerScan(p.items[i])
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
						return material.H5(th, "Watch Folders").Layout(gtx)
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
				if p.infoMsg != "" {
					lbl := material.Body1(th, p.infoMsg)
					lbl.Color = desktopapp.ColorSuccess
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

var watchFolderCols = []colSpec{
	{"Name", 0.15},
	{"Path", 0.25},
	{"Patterns", 0.18},
	{"Enabled", 0.08},
	{"Last Scan", 0.14},
	{"Actions", 0.20},
}

func (p *WatchFoldersPage) layoutTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.loading && len(p.items) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.items) == 0 {
		lbl := material.Body1(th, "No watch folders configured.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutColHeader(gtx, th, watchFolderCols)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.items), func(gtx layout.Context, i int) layout.Dimensions {
				return p.layoutRow(gtx, th, i)
			})
		}),
	)
}

func (p *WatchFoldersPage) layoutRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	wf := p.items[i]
	for len(p.toggleBtns) <= i {
		p.toggleBtns = append(p.toggleBtns, widget.Clickable{})
		p.scanBtns = append(p.scanBtns, widget.Clickable{})
	}
	toggleLabel := "Disable"
	if !wf.Enabled {
		toggleLabel = "Enable"
	}
	lastScan := "-"
	if wf.LastScan != nil {
		lastScan = wf.LastScan.Format("01-02 15:04")
	}
	patterns := strings.Join(wf.FilePatterns, ", ")
	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(watchFolderCols[0].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, wf.Name)
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}),
			layout.Flexed(watchFolderCols[1].flex, func(gtx layout.Context) layout.Dimensions {
				path := wf.Path
				if len(path) > 30 {
					path = "..." + path[len(path)-27:]
				}
				lbl := material.Body2(th, path)
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(watchFolderCols[2].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, patterns)
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(watchFolderCols[3].flex, func(gtx layout.Context) layout.Dimensions {
				col := desktopapp.ColorDanger
				if wf.Enabled {
					col = desktopapp.ColorSuccess
				}
				lbl := material.Body2(th, boolStr(wf.Enabled))
				lbl.Color = col
				return lbl.Layout(gtx)
			}),
			layout.Flexed(watchFolderCols[4].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, lastScan)
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(watchFolderCols[5].flex, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.scanBtns[i], "Scan")
						btn.Background = desktopapp.ColorPrimary
						return btn.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.toggleBtns[i], toggleLabel)
						btn.Background = desktopapp.ColorSecondary
						return btn.Layout(gtx)
					}),
				)
			}),
		)
	})
}
