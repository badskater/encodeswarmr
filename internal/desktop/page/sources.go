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

// SourcesPage renders the list of media sources with search and filtering.
type SourcesPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	// Data
	sources    []client.Source
	filtered   []client.Source
	loading    bool
	errorMsg   string

	// Search
	searchEditor     widget.Editor
	searchQuery      string
	searchPrevText   string

	// Widgets
	refreshBtn widget.Clickable
	list       widget.List
	rowBtns    []widget.Clickable
}

// NewSourcesPage constructs a SourcesPage.
func NewSourcesPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *SourcesPage {
	p := &SourcesPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	p.searchEditor.SingleLine = true
	return p
}

// OnNavigatedTo loads sources when the page becomes active.
func (p *SourcesPage) OnNavigatedTo(_ map[string]string) {
	p.load()
}

// OnNavigatedFrom is a no-op.
func (p *SourcesPage) OnNavigatedFrom() {}

func (p *SourcesPage) load() {
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
		sources, err := c.ListSources(context.Background(), "", "", 0)
		if err != nil {
			p.errorMsg = "Failed to load sources: " + err.Error()
			p.logger.Error("sources load", "err", err)
		} else {
			p.sources = sources
			p.applyFilter()
			p.rowBtns = make([]widget.Clickable, len(p.filtered))
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *SourcesPage) applyFilter() {
	q := strings.ToLower(p.searchQuery)
	if q == "" {
		p.filtered = p.sources
		return
	}
	p.filtered = p.filtered[:0]
	for _, s := range p.sources {
		if strings.Contains(strings.ToLower(s.Path), q) ||
			strings.Contains(strings.ToLower(s.Filename), q) {
			p.filtered = append(p.filtered, s)
		}
	}
}

// Layout renders the sources page.
func (p *SourcesPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.refreshBtn.Clicked(gtx) {
		p.load()
	}

	// Detect search text changes.
	if current := p.searchEditor.Text(); current != p.searchPrevText {
		p.searchPrevText = current
		p.searchQuery = current
		p.applyFilter()
		p.rowBtns = make([]widget.Clickable, len(p.filtered))
	}

	// Row click handling.
	for i := range p.rowBtns {
		if i < len(p.filtered) && p.rowBtns[i].Clicked(gtx) {
			p.router.Push("/sources/detail", map[string]string{"id": p.filtered[i].ID})
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
				return p.layoutSearchBar(gtx, th)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.layoutTable(gtx, th)
			}),
		)
	})
}

func (p *SourcesPage) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.H5(th, "Sources").Layout(gtx)
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

func (p *SourcesPage) layoutSearchBar(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return widget.Border{
		Color:        desktopapp.ColorBorder,
		CornerRadius: unit.Dp(4),
		Width:        unit.Dp(1),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top: unit.Dp(6), Bottom: unit.Dp(6),
			Left: unit.Dp(10), Right: unit.Dp(10),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			ed := material.Editor(th, &p.searchEditor, "Search by path...")
			ed.Color = desktopapp.ColorText
			return ed.Layout(gtx)
		})
	})
}

// sourceCols defines source table column layout.
var sourceCols = []struct {
	title string
	flex  float32
}{
	{"Filename", 0.22},
	{"Path", 0.28},
	{"State", 0.10},
	{"Size", 0.10},
	{"Duration", 0.10},
	{"VMAF", 0.08},
	{"HDR", 0.07},
	{"Created", 0.13},
}

func (p *SourcesPage) layoutTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.errorMsg != "" {
		lbl := material.Body1(th, p.errorMsg)
		lbl.Color = desktopapp.ColorDanger
		return lbl.Layout(gtx)
	}
	if p.loading && len(p.filtered) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.filtered) == 0 {
		lbl := material.Body1(th, "No sources found.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutTableHeader(gtx, th)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.filtered),
				func(gtx layout.Context, i int) layout.Dimensions {
					return p.layoutSourceRow(gtx, th, i)
				})
		}),
	)
}

func (p *SourcesPage) layoutTableHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(sourceCols))
	for _, col := range sourceCols {
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

func (p *SourcesPage) layoutSourceRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	src := p.filtered[i]

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
					// Filename.
					layout.Flexed(sourceCols[0].flex, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, src.Filename)
						lbl.MaxLines = 1
						return lbl.Layout(gtx)
					}),
					// Path.
					layout.Flexed(sourceCols[1].flex, func(gtx layout.Context) layout.Dimensions {
						path := src.Path
						if len(path) > 40 {
							path = "..." + path[len(path)-37:]
						}
						lbl := material.Body2(th, path)
						lbl.MaxLines = 1
						return lbl.Layout(gtx)
					}),
					// State badge.
					layout.Flexed(sourceCols[2].flex, func(gtx layout.Context) layout.Dimensions {
						return layoutStatusBadge(gtx, th, src.State)
					}),
					// Size.
					layout.Flexed(sourceCols[3].flex, func(gtx layout.Context) layout.Dimensions {
						return material.Body2(th, formatBytes(src.SizeBytes)).Layout(gtx)
					}),
					// Duration.
					layout.Flexed(sourceCols[4].flex, func(gtx layout.Context) layout.Dimensions {
						dur := "-"
						if src.DurationSec != nil {
							dur = formatDuration(*src.DurationSec)
						}
						return material.Body2(th, dur).Layout(gtx)
					}),
					// VMAF score.
					layout.Flexed(sourceCols[5].flex, func(gtx layout.Context) layout.Dimensions {
						vmaf := "-"
						if src.VMafScore != nil {
							vmaf = fmt.Sprintf("%.1f", *src.VMafScore)
						}
						return material.Body2(th, vmaf).Layout(gtx)
					}),
					// HDR type.
					layout.Flexed(sourceCols[6].flex, func(gtx layout.Context) layout.Dimensions {
						hdr := src.HDRType
						if hdr == "" {
							hdr = "-"
						}
						lbl := material.Body2(th, hdr)
						lbl.MaxLines = 1
						return lbl.Layout(gtx)
					}),
					// Created at.
					layout.Flexed(sourceCols[7].flex, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, src.CreatedAt.Format("2006-01-02"))
						lbl.Color = desktopapp.ColorTextLight
						return lbl.Layout(gtx)
					}),
				)
			})
		})
	})
}

// formatBytes formats a byte count as a human-readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// formatDuration formats seconds as h:mm:ss or m:ss.
func formatDuration(sec float64) string {
	total := int(sec)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
