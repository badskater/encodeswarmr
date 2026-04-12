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

// StatusBadge renders a status string as a coloured pill badge.
// The background is a semi-transparent tint of the status colour and the text
// uses the fully-opaque status colour.
func StatusBadge(gtx layout.Context, th *material.Theme, status string) layout.Dimensions {
	fg := statusColor(status)

	// Semi-transparent background tint.
	bg := fg
	bg.A = 30

	// Measure the text first so we can size the pill around it.
	lbl := material.Caption(th, status)
	lbl.Color = fg

	const hPad = unit.Dp(8)
	const vPad = unit.Dp(3)

	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			bounds := image.Rect(0, 0, gtx.Constraints.Min.X, gtx.Constraints.Min.Y)
			rr := gtx.Dp(unit.Dp(100)) // fully rounded
			paint.FillShape(gtx.Ops, bg,
				clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))
			return layout.Dimensions{Size: bounds.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top:    vPad,
				Bottom: vPad,
				Left:   hPad,
				Right:  hPad,
			}.Layout(gtx, lbl.Layout)
		}),
	)
}

// statusColor maps a status string to its representative colour. The mapping
// mirrors app.StatusColor so the badge is consistent with the rest of the UI
// without importing the app package (which would create a circular dependency).
func statusColor(status string) color.NRGBA {
	switch status {
	case "idle":
		return color.NRGBA{R: 34, G: 197, B: 94, A: 255} // green-500
	case "running", "assigned":
		return color.NRGBA{R: 59, G: 130, B: 246, A: 255} // blue-500
	case "offline", "failed", "cancelled":
		return color.NRGBA{R: 239, G: 68, B: 68, A: 255} // red-500
	case "draining", "waiting", "queued", "pending":
		return color.NRGBA{R: 234, G: 179, B: 8, A: 255} // yellow-500
	case "completed":
		return color.NRGBA{R: 16, G: 185, B: 129, A: 255} // emerald-500
	default:
		return color.NRGBA{R: 107, G: 114, B: 128, A: 255} // gray-500
	}
}
