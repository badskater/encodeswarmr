package widget

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// BarData is a single bar in a BarChart.
type BarData struct {
	Value float64
	Label string
}

// BarChart draws a simple vertical bar chart with value labels above each bar
// and axis labels below.
type BarChart struct {
	// Bars is the ordered list of bars to render.
	Bars []BarData
	// MaxValue is the reference maximum used to scale bar heights.
	// If zero, the chart auto-scales to the maximum value in Bars.
	MaxValue float64
	// BarColor is the fill colour for every bar.
	BarColor color.NRGBA
	// LabelFunc overrides the label shown beneath each bar.
	// When nil, BarData.Label is used.
	LabelFunc func(i int) string
}

// Layout draws the bar chart into gtx. The chart fills the available width and
// uses the available height from the constraints.
func (bc *BarChart) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if len(bc.Bars) == 0 {
		return layout.Dimensions{}
	}

	// Resolve effective max value.
	maxVal := bc.MaxValue
	if maxVal == 0 {
		for _, b := range bc.Bars {
			if b.Value > maxVal {
				maxVal = b.Value
			}
		}
	}
	if maxVal == 0 {
		maxVal = 1 // avoid division by zero
	}

	totalW := gtx.Constraints.Max.X
	totalH := gtx.Constraints.Max.Y
	if totalH <= 0 {
		totalH = gtx.Dp(unit.Dp(120))
	}

	const (
		labelH = unit.Dp(16) // height reserved below bars for axis labels
		valueH = unit.Dp(14) // height reserved above bars for value text
		hGapDp = unit.Dp(4)  // gap between bars
	)

	labelPx := gtx.Dp(labelH)
	valuePx := gtx.Dp(valueH)
	gapPx := gtx.Dp(hGapDp)

	barAreaH := totalH - labelPx - valuePx
	if barAreaH < 1 {
		barAreaH = 1
	}

	n := len(bc.Bars)
	barW := (totalW - gapPx*(n+1)) / n
	if barW < 1 {
		barW = 1
	}

	barColor := bc.BarColor
	if barColor == (color.NRGBA{}) {
		barColor = color.NRGBA{R: 59, G: 130, B: 246, A: 255} // blue-500
	}
	labelColor := color.NRGBA{R: 107, G: 114, B: 128, A: 255} // gray-500

	for i, bar := range bc.Bars {
		x0 := gapPx + i*(barW+gapPx)

		barH := int(float64(barAreaH) * (bar.Value / maxVal))
		if barH < 1 && bar.Value > 0 {
			barH = 1
		}
		barY := valuePx + (barAreaH - barH)

		// Draw bar rectangle.
		barRect := image.Rect(x0, barY, x0+barW, barY+barH)
		paint.FillShape(gtx.Ops, barColor, clip.Rect(barRect).Op())

		// Draw value label above bar.
		valStr := fmt.Sprintf("%.0f", bar.Value)
		valLabel := material.Caption(th, valStr)
		valLabel.Color = labelColor
		{
			stack := op.Offset(image.Point{X: x0, Y: barY - valuePx}).Push(gtx.Ops)
			subGtx := gtx
			subGtx.Constraints = layout.Constraints{
				Min: image.Point{},
				Max: image.Point{X: barW, Y: valuePx},
			}
			valLabel.Layout(subGtx)
			stack.Pop()
		}

		// Draw axis label below the bar area.
		axisStr := bar.Label
		if bc.LabelFunc != nil {
			axisStr = bc.LabelFunc(i)
		}
		axisLabel := material.Caption(th, axisStr)
		axisLabel.Color = labelColor
		axisLabel.MaxLines = 1
		{
			stack := op.Offset(image.Point{X: x0, Y: valuePx + barAreaH}).Push(gtx.Ops)
			subGtx := gtx
			subGtx.Constraints = layout.Constraints{
				Min: image.Point{},
				Max: image.Point{X: barW, Y: labelPx},
			}
			axisLabel.Layout(subGtx)
			stack.Pop()
		}
	}

	return layout.Dimensions{Size: image.Point{X: totalW, Y: totalH}}
}
