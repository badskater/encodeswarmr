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

// SourceDetailPage renders the detail view for a single media source.
type SourceDetailPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	// Route params.
	sourceID string

	// Data
	source   *client.Source
	analysis *client.AnalysisResult
	loading  bool
	errorMsg string

	// Action state.
	analyzeInFlight    bool
	hdrDetectInFlight  bool
	actionError        string

	// Widgets
	backBtn      widget.Clickable
	analyzeBtn   widget.Clickable
	hdrDetectBtn widget.Clickable
	encodeBtn    widget.Clickable
	scrollList   widget.List
}

// NewSourceDetailPage constructs a SourceDetailPage.
func NewSourceDetailPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *SourceDetailPage {
	p := &SourceDetailPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.scrollList.Axis = layout.Vertical
	return p
}

// OnNavigatedTo loads the source when the page becomes active.
func (p *SourceDetailPage) OnNavigatedTo(params map[string]string) {
	p.sourceID = params["id"]
	p.source = nil
	p.analysis = nil
	p.errorMsg = ""
	p.actionError = ""
	p.load()
}

// OnNavigatedFrom is a no-op.
func (p *SourceDetailPage) OnNavigatedFrom() {}

func (p *SourceDetailPage) load() {
	if p.loading || p.sourceID == "" {
		return
	}
	c := p.state.Client()
	if c == nil {
		return
	}
	p.loading = true

	go func() {
		ctx := context.Background()

		src, err := c.GetSource(ctx, p.sourceID)
		if err != nil {
			p.errorMsg = "Failed to load source: " + err.Error()
			p.logger.Error("source detail load", "id", p.sourceID, "err", err)
			p.loading = false
			p.window.Invalidate()
			return
		}
		p.source = src

		// Attempt to load analysis; not fatal if absent.
		analysis, err := c.GetAnalysis(ctx, p.sourceID)
		if err != nil {
			p.logger.Warn("source analysis load", "id", p.sourceID, "err", err)
		} else {
			p.analysis = analysis
		}

		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *SourceDetailPage) doAnalyze() {
	if p.analyzeInFlight || p.source == nil {
		return
	}
	c := p.state.Client()
	if c == nil {
		return
	}
	p.analyzeInFlight = true

	go func() {
		_, err := c.AnalyzeSource(context.Background(), p.sourceID)
		if err != nil {
			p.actionError = "Analyze failed: " + err.Error()
			p.logger.Error("source analyze", "id", p.sourceID, "err", err)
		} else {
			p.actionError = ""
		}
		p.analyzeInFlight = false
		p.window.Invalidate()
	}()
}

func (p *SourceDetailPage) doHDRDetect() {
	if p.hdrDetectInFlight || p.source == nil {
		return
	}
	c := p.state.Client()
	if c == nil {
		return
	}
	p.hdrDetectInFlight = true

	go func() {
		_, err := c.HDRDetectSource(context.Background(), p.sourceID)
		if err != nil {
			p.actionError = "HDR detect failed: " + err.Error()
			p.logger.Error("source hdr detect", "id", p.sourceID, "err", err)
		} else {
			p.actionError = ""
			p.load()
		}
		p.hdrDetectInFlight = false
		p.window.Invalidate()
	}()
}

// Layout renders the source detail page.
func (p *SourceDetailPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.backBtn.Clicked(gtx) {
		p.router.Pop()
	}
	if p.analyzeBtn.Clicked(gtx) {
		p.doAnalyze()
	}
	if p.hdrDetectBtn.Clicked(gtx) {
		p.doHDRDetect()
	}
	if p.encodeBtn.Clicked(gtx) && p.source != nil {
		p.router.Push("/encode/config", map[string]string{"source_id": p.sourceID})
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

func (p *SourceDetailPage) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	title := "Source Detail"
	if p.source != nil {
		title = "Source: " + p.source.Filename
	}

	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(th, &p.backBtn, "< Back")
			btn.Background = desktopapp.ColorSecondary
			return btn.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			lbl := material.H5(th, title)
			lbl.MaxLines = 1
			return lbl.Layout(gtx)
		}),
	)
}

func (p *SourceDetailPage) layoutBody(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.errorMsg != "" {
		lbl := material.Body1(th, p.errorMsg)
		lbl.Color = desktopapp.ColorDanger
		return lbl.Layout(gtx)
	}
	if p.loading && p.source == nil {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if p.source == nil {
		return layout.Dimensions{}
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutInfoCard(gtx, th)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutActions(gtx, th)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if p.actionError != "" {
				lbl := material.Body2(th, p.actionError)
				lbl.Color = desktopapp.ColorDanger
				return lbl.Layout(gtx)
			}
			return layout.Dimensions{}
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return p.layoutAnalysisSection(gtx, th)
		}),
	)
}

func (p *SourceDetailPage) layoutInfoCard(gtx layout.Context, th *material.Theme) layout.Dimensions {
	src := p.source
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
						return p.layoutInfoRow(gtx, th, "Path", src.Path)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return p.layoutInfoRow(gtx, th, "Size", formatBytes(src.SizeBytes))
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						dur := "-"
						if src.DurationSec != nil {
							dur = formatDuration(*src.DurationSec)
						}
						return p.layoutInfoRow(gtx, th, "Duration", dur)
					}),
				)
			}),
			// Right column.
			layout.Flexed(0.5, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Caption(th, "State")
								lbl.Color = desktopapp.ColorTextLight
								return lbl.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layoutStatusBadge(gtx, th, src.State)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						vmaf := "-"
						if src.VMafScore != nil {
							vmaf = fmt.Sprintf("%.2f", *src.VMafScore)
						}
						return p.layoutInfoRow(gtx, th, "VMAF", vmaf)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						hdr := src.HDRType
						if hdr == "" {
							hdr = "SDR"
						}
						return p.layoutInfoRow(gtx, th, "HDR Type", hdr)
					}),
				)
			}),
		)
	})
}

func (p *SourceDetailPage) layoutInfoRow(gtx layout.Context, th *material.Theme, label, value string) layout.Dimensions {
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

func (p *SourceDetailPage) layoutActions(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := "Analyze"
			if p.analyzeInFlight {
				label = "Analyzing..."
			}
			btn := material.Button(th, &p.analyzeBtn, label)
			btn.Background = desktopapp.ColorPrimary
			return btn.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := "HDR Detect"
			if p.hdrDetectInFlight {
				label = "Detecting..."
			}
			btn := material.Button(th, &p.hdrDetectBtn, label)
			btn.Background = desktopapp.ColorSecondary
			return btn.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(th, &p.encodeBtn, "Encode")
			btn.Background = desktopapp.ColorSuccess
			return btn.Layout(gtx)
		}),
	)
}

func (p *SourceDetailPage) layoutAnalysisSection(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(th, "Analysis Results").Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if p.analysis == nil {
				lbl := material.Body2(th, "No analysis data available. Run Analyze to generate.")
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}
			return p.layoutAnalysisCard(gtx, th)
		}),
	)
}

func (p *SourceDetailPage) layoutAnalysisCard(gtx layout.Context, th *material.Theme) layout.Dimensions {
	a := p.analysis
	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	rr := gtx.Dp(unit.Dp(8))
	paint.FillShape(gtx.Ops, desktopapp.ColorSurface,
		clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

	return layout.Inset{
		Top: unit.Dp(16), Bottom: unit.Dp(16),
		Left: unit.Dp(16), Right: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, fmt.Sprintf("Type: %s", a.Type))
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.Summary == nil {
					return material.Body2(th, "No summary data.").Layout(gtx)
				}
				return p.layoutAnalysisSummary(gtx, th, a.Summary)
			}),
		)
	})
}

func (p *SourceDetailPage) layoutAnalysisSummary(gtx layout.Context, th *material.Theme, s *client.AnalysisSummary) layout.Dimensions {
	type kv struct{ k, v string }
	rows := []kv{}

	if s.Codec != nil {
		rows = append(rows, kv{"Codec", *s.Codec})
	}
	if s.Width != nil && s.Height != nil {
		rows = append(rows, kv{"Resolution", fmt.Sprintf("%d x %d", *s.Width, *s.Height)})
	}
	if s.DurationSec != nil {
		rows = append(rows, kv{"Duration", formatDuration(*s.DurationSec)})
	}
	if s.FrameCount != nil {
		rows = append(rows, kv{"Frames", fmt.Sprintf("%d", *s.FrameCount)})
	}
	if s.Mean != nil {
		rows = append(rows, kv{"VMAF Mean", fmt.Sprintf("%.2f", *s.Mean)})
	}
	if s.Min != nil {
		rows = append(rows, kv{"VMAF Min", fmt.Sprintf("%.2f", *s.Min)})
	}
	if s.Max != nil {
		rows = append(rows, kv{"VMAF Max", fmt.Sprintf("%.2f", *s.Max)})
	}
	if s.PSNR != nil {
		rows = append(rows, kv{"PSNR", fmt.Sprintf("%.2f dB", *s.PSNR)})
	}
	if s.SSIM != nil {
		rows = append(rows, kv{"SSIM", fmt.Sprintf("%.4f", *s.SSIM)})
	}
	if s.SceneCount != nil {
		rows = append(rows, kv{"Scenes", fmt.Sprintf("%d", *s.SceneCount)})
	}
	if s.BitRate != nil {
		rows = append(rows, kv{"Bit Rate", fmt.Sprintf("%d kbps", *s.BitRate/1000)})
	}

	if len(rows) == 0 {
		lbl := material.Body2(th, "No summary metrics.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}

	children := make([]layout.FlexChild, 0, len(rows)*2)
	for _, row := range rows {
		row := row
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Caption(th, row.k)
								lbl.Color = desktopapp.ColorTextLight
								return lbl.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return material.Body2(th, row.v).Layout(gtx)
							}),
						)
					},
				)
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
