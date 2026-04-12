// Package app holds shared application state for the desktop manager.
package app

import (
	"image/color"

	"gioui.org/font/gofont"
	"gioui.org/text"
	"gioui.org/widget/material"
)

// Additional colours used in the sidebar and page backgrounds.
// Core palette colours (ColorPrimary, ColorSuccess, etc.) are declared in state.go.
var (
	ColorBackground    = color.NRGBA{R: 249, G: 250, B: 251, A: 255} // Gray-50
	ColorText          = color.NRGBA{R: 17, G: 24, B: 39, A: 255}    // Gray-900
	ColorSidebar       = color.NRGBA{R: 17, G: 24, B: 39, A: 255}    // Gray-900
	ColorSidebarText   = color.NRGBA{R: 209, G: 213, B: 219, A: 255} // Gray-300
	ColorSidebarActive = color.NRGBA{R: 55, G: 65, B: 81, A: 255}    // Gray-700
)

// NewTheme creates the application's Material theme with the project palette.
func NewTheme() *material.Theme {
	th := material.NewTheme()
	th.Palette.Bg = ColorBackground
	th.Palette.Fg = ColorText
	th.Palette.ContrastBg = ColorPrimary
	th.Palette.ContrastFg = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	return th
}
