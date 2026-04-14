package admin

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	desktopapp "github.com/badskater/encodeswarmr/internal/desktop/app"
)

// colSpec describes a table column title and flex weight.
type colSpec struct {
	title string
	flex  float32
}

// layoutColHeader renders a row of column header labels.
func layoutColHeader(gtx layout.Context, th *material.Theme, cols []colSpec) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(cols))
	for _, c := range cols {
		c := c
		children = append(children, layout.Flexed(c.flex, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, c.title)
			lbl.Color = desktopapp.ColorTextLight
			return lbl.Layout(gtx)
		}))
	}
	return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx, children...)
	})
}

// labeledField renders a caption + single-line editor field.
func labeledField(gtx layout.Context, th *material.Theme, label string, ed *widget.Editor) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, label)
			lbl.Color = desktopapp.ColorTextLight
			return lbl.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Editor(th, ed, "").Layout(gtx)
		}),
	)
}

// boolStr returns "Yes" or "No" for a bool.
func boolStr(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
