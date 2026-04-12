// Package app holds shared application state for the desktop manager.
package app

import (
	"log/slog"
	"os"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/badskater/encodeswarmr/internal/desktop/nav"
)

// Application is the top-level desktop application.
type Application struct {
	window  *app.Window
	theme   *material.Theme
	state   *State
	router  *nav.Router
	sidebar *nav.Sidebar
	logger  *slog.Logger
}

// NewApplication creates and configures the application window.
func NewApplication(logger *slog.Logger) *Application {
	w := new(app.Window)
	w.Option(
		app.Title("EncodeSwarmr Desktop"),
		app.Size(unit.Dp(1280), unit.Dp(800)),
		app.MinSize(unit.Dp(900), unit.Dp(600)),
	)

	th := NewTheme()
	r := nav.NewRouter(w)

	a := &Application{
		window:  w,
		theme:   th,
		state:   &State{},
		router:  r,
		sidebar: nav.NewSidebar(r),
		logger:  logger,
	}
	return a
}

// State returns the shared application state.
func (a *Application) State() *State { return a.state }

// Router returns the page router.
func (a *Application) Router() *nav.Router { return a.router }

// Window returns the underlying Gio window.
func (a *Application) Window() *app.Window { return a.window }

// Run starts the Gio event loop. Blocks until the window is closed.
func (a *Application) Run() error {
	var ops op.Ops
	for {
		switch e := a.window.Event().(type) {
		case app.DestroyEvent:
			a.state.Reset()
			if e.Err != nil {
				a.logger.Error("window destroyed with error", "err", e.Err)
			}
			os.Exit(0)
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (a *Application) layout(gtx layout.Context) layout.Dimensions {
	currentPath := a.router.CurrentPath()

	// The login page occupies the full window with no sidebar.
	if currentPath == "/login" || currentPath == "" {
		return a.router.Layout(gtx, a.theme)
	}

	// All other pages share a sidebar + content split layout.
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.sidebar.Layout(gtx, a.theme)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.router.Layout(gtx, a.theme)
		}),
	)
}
