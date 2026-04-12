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

// Dialog is a modal confirmation dialog that renders an overlay with a centred
// card containing a title, message, and Cancel / Confirm buttons.
type Dialog struct {
	// Title is the heading shown at the top of the card.
	Title string
	// Message is the body text of the dialog.
	Message string
	// Visible controls whether the dialog is rendered.
	Visible bool

	// OnConfirm is called when the user clicks the Confirm button.
	OnConfirm func()
	// OnCancel is called when the user clicks the Cancel button or the overlay.
	OnCancel func()

	confirmBtn widget.Clickable
	cancelBtn  widget.Clickable
	overlay    widget.Clickable
}

// Show populates the dialog fields and makes it visible.
func (d *Dialog) Show(title, message string) {
	d.Title = title
	d.Message = message
	d.Visible = true
}

// Hide hides the dialog without firing any callback.
func (d *Dialog) Hide() {
	d.Visible = false
}

// Layout renders the dialog. When not visible it returns zero dimensions and
// does not consume any input events.
func (d *Dialog) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if !d.Visible {
		return layout.Dimensions{}
	}

	// Handle button clicks before drawing so the frame reflects dismissal.
	if d.confirmBtn.Clicked(gtx) {
		d.Visible = false
		if d.OnConfirm != nil {
			d.OnConfirm()
		}
	}
	if d.cancelBtn.Clicked(gtx) {
		d.Visible = false
		if d.OnCancel != nil {
			d.OnCancel()
		}
	}
	if d.overlay.Clicked(gtx) {
		d.Visible = false
		if d.OnCancel != nil {
			d.OnCancel()
		}
	}

	// Fill the entire available area with a semi-transparent scrim.
	fullBounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	paint.FillShape(gtx.Ops, colorScrim, clip.Rect(fullBounds).Op())

	// Place clickable on the scrim so clicking outside dismisses the dialog.
	_ = material.Clickable(gtx, &d.overlay, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: fullBounds.Max}
	})

	// Centre the card both horizontally and vertically.
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return d.layoutCard(gtx, th)
	})
}

func (d *Dialog) layoutCard(gtx layout.Context, th *material.Theme) layout.Dimensions {
	cardW := gtx.Dp(unit.Dp(400))
	if cardW > gtx.Constraints.Max.X-gtx.Dp(unit.Dp(32)) {
		cardW = gtx.Constraints.Max.X - gtx.Dp(unit.Dp(32))
	}
	gtx.Constraints = layout.Constraints{
		Min: image.Point{X: cardW, Y: 0},
		Max: image.Point{X: cardW, Y: gtx.Constraints.Max.Y},
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			bounds := image.Rect(0, 0, gtx.Constraints.Min.X, gtx.Constraints.Min.Y)
			rr := gtx.Dp(unit.Dp(12))
			paint.FillShape(gtx.Ops, colorDialogCard,
				clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))
			return layout.Dimensions{Size: bounds.Max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top:    unit.Dp(24),
				Bottom: unit.Dp(24),
				Left:   unit.Dp(24),
				Right:  unit.Dp(24),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					// Title.
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.H6(th, d.Title).Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
					// Message.
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, d.Message)
						lbl.Color = colorDialogMuted
						return lbl.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(24)}.Layout),
					// Button row — right-aligned.
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Spacing: layout.SpaceStart}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								btn := material.Button(th, &d.cancelBtn, "Cancel")
								btn.Background = colorBtnBackground
								btn.Color = colorBtnText
								return btn.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								btn := material.Button(th, &d.confirmBtn, "Confirm")
								btn.Background = colorConfirm
								return btn.Layout(gtx)
							}),
						)
					}),
				)
			})
		}),
	)
}

var (
	colorScrim      = color.NRGBA{R: 0, G: 0, B: 0, A: 128}
	colorDialogCard = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	colorDialogMuted = color.NRGBA{R: 107, G: 114, B: 128, A: 255}
	colorConfirm    = color.NRGBA{R: 59, G: 130, B: 246, A: 255} // blue-500
)
