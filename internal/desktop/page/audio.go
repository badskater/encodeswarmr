package page

import (
	"context"
	"fmt"
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

// AudioPage displays the built-in audio encoding presets.
type AudioPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	presets  []client.AudioPreset
	loading  bool
	errorMsg string

	refreshBtn widget.Clickable
	list       widget.List
}

// NewAudioPage constructs an AudioPage.
func NewAudioPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *AudioPage {
	p := &AudioPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	return p
}

// OnNavigatedTo loads presets when the page becomes active.
func (p *AudioPage) OnNavigatedTo(_ map[string]string) {
	p.load()
}

// OnNavigatedFrom is a no-op.
func (p *AudioPage) OnNavigatedFrom() {}

func (p *AudioPage) load() {
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
		presets, err := c.ListAudioPresets(context.Background())
		if err != nil {
			p.errorMsg = "Failed to load audio presets: " + err.Error()
			p.logger.Error("audio presets load", "err", err)
		} else {
			p.presets = presets
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

// Layout renders the audio page.
func (p *AudioPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.refreshBtn.Clicked(gtx) {
		p.load()
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
				return p.layoutList(gtx, th)
			}),
		)
	})
}

func (p *AudioPage) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.H5(th, "Audio Conversion").Layout(gtx)
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

// audioCols defines the audio preset table columns.
var audioCols = []struct {
	title string
	flex  float32
}{
	{"Name", 0.18},
	{"Category", 0.12},
	{"Codec", 0.10},
	{"Bitrate", 0.10},
	{"Description", 0.32},
	{"Tags", 0.18},
}

func (p *AudioPage) layoutList(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.errorMsg != "" {
		lbl := material.Body1(th, p.errorMsg)
		lbl.Color = desktopapp.ColorDanger
		return lbl.Layout(gtx)
	}
	if p.loading && len(p.presets) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.presets) == 0 {
		lbl := material.Body1(th, "No audio presets found.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutTableHeader(gtx, th)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.presets),
				func(gtx layout.Context, i int) layout.Dimensions {
					return p.layoutPresetRow(gtx, th, p.presets[i])
				})
		}),
	)
}

func (p *AudioPage) layoutTableHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(audioCols))
	for _, col := range audioCols {
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

func (p *AudioPage) layoutPresetRow(gtx layout.Context, th *material.Theme, preset client.AudioPreset) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			// Name.
			layout.Flexed(audioCols[0].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, preset.Name)
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}),
			// Category.
			layout.Flexed(audioCols[1].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, preset.Category)
				lbl.Color = desktopapp.ColorPrimary
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}),
			// Codec.
			layout.Flexed(audioCols[2].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, preset.Codec)
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}),
			// Bitrate.
			layout.Flexed(audioCols[3].flex, func(gtx layout.Context) layout.Dimensions {
				br := "-"
				if preset.Bitrate != nil {
					br = *preset.Bitrate
				}
				lbl := material.Body2(th, br)
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			// Description.
			layout.Flexed(audioCols[4].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, preset.Description)
				lbl.MaxLines = 2
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			// Tags.
			layout.Flexed(audioCols[5].flex, func(gtx layout.Context) layout.Dimensions {
				tags := strings.Join(preset.Tags, ", ")
				if tags == "" {
					tags = "-"
				}
				sampleRate := ""
				if preset.SampleRate != nil {
					sampleRate = fmt.Sprintf(" %dHz", *preset.SampleRate)
				}
				lbl := material.Caption(th, tags+sampleRate)
				lbl.Color = desktopapp.ColorTextLight
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}),
		)
	})
}
