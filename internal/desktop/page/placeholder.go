package page

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	desktopapp "github.com/badskater/encodeswarmr/internal/desktop/app"
)

// PlaceholderPage renders a "coming soon" message for unimplemented routes.
type PlaceholderPage struct {
	path string
}

// NewPlaceholderPage creates a placeholder for the given route path.
func NewPlaceholderPage(path string) *PlaceholderPage {
	return &PlaceholderPage{path: path}
}

func (p *PlaceholderPage) OnNavigatedTo(_ map[string]string) {}
func (p *PlaceholderPage) OnNavigatedFrom()                  {}

// Layout renders a centred "under construction" message.
func (p *PlaceholderPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{
			Axis:      layout.Vertical,
			Alignment: layout.Middle,
			Spacing:   layout.SpaceStart,
		}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.H4(th, p.path)
				lbl.Color = desktopapp.ColorText
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body1(th, "This page is under construction")
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
		)
	})
}
