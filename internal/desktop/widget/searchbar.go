package widget

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// SearchBar is a search input with a clear (×) button on the right.
// OnSearch is called with the current query text whenever the editor content
// changes or the clear button is pressed.
type SearchBar struct {
	// Editor holds the text state. It is exported so callers can seed the value.
	Editor widget.Editor

	// OnSearch is invoked with the current query string when the editor is
	// modified or when the clear button is clicked.
	OnSearch func(query string)

	clearBtn    widget.Clickable
	prevContent string // track changes to fire OnSearch
}

// NewSearchBar allocates a SearchBar and configures it for single-line input.
func NewSearchBar() *SearchBar {
	s := &SearchBar{}
	s.Editor.SingleLine = true
	return s
}

// Layout renders the search bar and processes events.
func (s *SearchBar) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	// Handle clear button.
	if s.clearBtn.Clicked(gtx) {
		s.Editor.SetText("")
		s.prevContent = ""
		if s.OnSearch != nil {
			s.OnSearch("")
		}
	}

	// Detect text changes and fire the callback.
	current := s.Editor.Text()
	if current != s.prevContent {
		s.prevContent = current
		if s.OnSearch != nil {
			s.OnSearch(current)
		}
	}

	// Draw background.
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			bounds := image.Rect(0, 0, gtx.Constraints.Min.X, gtx.Constraints.Min.Y)
			rr := gtx.Dp(unit.Dp(6))
			paint.FillShape(gtx.Ops, colorInputBorder,
				clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))
			bp := gtx.Dp(unit.Dp(1))
			inner := image.Rect(bp, bp, bounds.Max.X-bp, bounds.Max.Y-bp)
			innerRR := gtx.Dp(unit.Dp(5))
			paint.FillShape(gtx.Ops, colorInputBg,
				clip.RRect{Rect: inner, SE: innerRR, SW: innerRR, NE: innerRR, NW: innerRR}.Op(gtx.Ops))
			return layout.Dimensions{Size: bounds.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top:    unit.Dp(8),
				Bottom: unit.Dp(8),
				Left:   unit.Dp(12),
				Right:  unit.Dp(8),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					// Search icon (text proxy).
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, "⌕")
						lbl.Color = colorSearchIcon
						return lbl.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					// Text editor.
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						ed := material.Editor(th, &s.Editor, "Search…")
						ed.Color = colorInputText
						ed.HintColor = colorInputHint
						return ed.Layout(gtx)
					}),
					// Clear button — only show when there is text.
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if s.Editor.Text() == "" {
							return layout.Dimensions{}
						}
						return material.Clickable(gtx, &s.clearBtn, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{
								Left:  unit.Dp(8),
								Right: unit.Dp(4),
							}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(th, "×")
								lbl.Color = colorSearchIcon
								return lbl.Layout(gtx)
							})
						})
					}),
				)
			})
		}),
	)
}

var colorSearchIcon = color.NRGBA{R: 156, G: 163, B: 175, A: 255} // gray-400
