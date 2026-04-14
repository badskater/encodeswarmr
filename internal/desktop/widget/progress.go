package widget

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// ProgressBar renders a horizontal progress bar.
//
// progress must be in the range [0, 1]. Values outside that range are clamped.
// accentColor is used for the filled portion; pass a zero-value color.NRGBA to
// use the theme's ContrastBg colour instead.
func ProgressBar(gtx layout.Context, th *material.Theme, progress float32, accentColor color.NRGBA) layout.Dimensions {
	// Clamp progress to [0, 1].
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	// Fall back to theme accent when no explicit colour is provided.
	fill := accentColor
	if fill == (color.NRGBA{}) {
		fill = color.NRGBA(th.Palette.ContrastBg)
	}

	barH := gtx.Dp(unit.Dp(8))
	barW := gtx.Constraints.Max.X
	rr := gtx.Dp(unit.Dp(4)) // fully rounded ends

	// Background track.
	trackBounds := image.Rect(0, 0, barW, barH)
	paint.FillShape(gtx.Ops, colorProgressTrack,
		clip.RRect{Rect: trackBounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

	// Filled portion.
	filledW := int(float32(barW) * progress)
	if filledW > 0 {
		filledBounds := image.Rect(0, 0, filledW, barH)
		fillRR := rr
		if filledW < barW {
			// When not full, only round the left end.
			paint.FillShape(gtx.Ops, fill,
				clip.RRect{Rect: filledBounds, NW: fillRR, SW: fillRR}.Op(gtx.Ops))
		} else {
			paint.FillShape(gtx.Ops, fill,
				clip.RRect{Rect: filledBounds, SE: fillRR, SW: fillRR, NE: fillRR, NW: fillRR}.Op(gtx.Ops))
		}
	}

	// Consume the vertical space so the caller's layout accounts for the bar height.
	_ = material.Body1(th, "") // keep th used; avoid blank import
	return layout.Dimensions{Size: image.Point{X: barW, Y: barH}}
}

var colorProgressTrack = color.NRGBA{R: 229, G: 231, B: 235, A: 255} // gray-200
