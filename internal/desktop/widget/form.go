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

// TextField renders a labelled single-line text input.
// The label is drawn above the editor; a hint (placeholder) is shown inside the
// editor when it is empty.
func TextField(gtx layout.Context, th *material.Theme, editor *widget.Editor, label, hint string) layout.Dimensions {
	editor.SingleLine = true
	return formField(gtx, th, editor, label, hint, 1)
}

// TextArea renders a labelled multi-line text input.
// lines controls the approximate visible height (in lines of text).
func TextArea(gtx layout.Context, th *material.Theme, editor *widget.Editor, label string, lines int) layout.Dimensions {
	editor.SingleLine = false
	if lines <= 0 {
		lines = 4
	}
	return formField(gtx, th, editor, label, "", lines)
}

// formField is the shared implementation for TextField and TextArea.
func formField(gtx layout.Context, th *material.Theme, editor *widget.Editor, label, hint string, lines int) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Label row.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, label)
			lbl.Color = colorFormLabel
			return lbl.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		// Bordered editor.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutEditorBox(gtx, th, editor, hint, lines)
		}),
	)
}

// layoutEditorBox draws a bordered rectangle around the material editor.
func layoutEditorBox(gtx layout.Context, th *material.Theme, editor *widget.Editor, hint string, lines int) layout.Dimensions {
	lineH := gtx.Dp(unit.Dp(20)) // approximate line height
	minH := gtx.Dp(unit.Dp(4))*2 + lineH*lines
	if minH < gtx.Dp(unit.Dp(36)) {
		minH = gtx.Dp(unit.Dp(36))
	}

	gtx.Constraints.Min.Y = minH

	return layout.Stack{}.Layout(gtx,
		// Border background.
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			bounds := image.Rect(0, 0, gtx.Constraints.Min.X, gtx.Constraints.Min.Y)
			rr := gtx.Dp(unit.Dp(6))

			// Fill with input background.
			paint.FillShape(gtx.Ops, colorInputBg,
				clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

			// Draw a 1 dp border by painting a slightly inset shape on top — we
			// achieve the border effect by painting the border colour first, then
			// the fill colour over a 1 dp inset rectangle.
			border := image.Rect(0, 0, bounds.Max.X, bounds.Max.Y)
			paint.FillShape(gtx.Ops, colorInputBorder,
				clip.RRect{Rect: border, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

			innerBorder := gtx.Dp(unit.Dp(1))
			inner := image.Rect(innerBorder, innerBorder, bounds.Max.X-innerBorder, bounds.Max.Y-innerBorder)
			innerRR := gtx.Dp(unit.Dp(5))
			paint.FillShape(gtx.Ops, colorInputBg,
				clip.RRect{Rect: inner, SE: innerRR, SW: innerRR, NE: innerRR, NW: innerRR}.Op(gtx.Ops))

			return layout.Dimensions{Size: bounds.Max}
		}),
		// Editor content.
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top:    unit.Dp(8),
				Bottom: unit.Dp(8),
				Left:   unit.Dp(12),
				Right:  unit.Dp(12),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				ed := material.Editor(th, editor, hint)
				ed.Color = colorInputText
				ed.HintColor = colorInputHint
				return ed.Layout(gtx)
			})
		}),
	)
}

var (
	colorFormLabel   = color.NRGBA{R: 55, G: 65, B: 81, A: 255}    // gray-700
	colorInputBg     = color.NRGBA{R: 255, G: 255, B: 255, A: 255} // white
	colorInputBorder = color.NRGBA{R: 209, G: 213, B: 219, A: 255} // gray-300
	colorInputText   = color.NRGBA{R: 17, G: 24, B: 39, A: 255}    // gray-900
	colorInputHint   = color.NRGBA{R: 156, G: 163, B: 175, A: 255} // gray-400
)
