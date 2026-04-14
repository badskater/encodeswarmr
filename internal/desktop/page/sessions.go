package page

import (
	"context"
	"fmt"
	"log/slog"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	desktopapp "github.com/badskater/encodeswarmr/internal/desktop/app"
	"github.com/badskater/encodeswarmr/internal/desktop/client"
	"github.com/badskater/encodeswarmr/internal/desktop/nav"
)

// SessionsPage displays active user sessions with per-row delete actions.
type SessionsPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	sessions []client.UserSession
	loading  bool
	errorMsg string

	refreshBtn widget.Clickable
	list       widget.List
	deleteBtns []widget.Clickable

	// Delete confirmation.
	confirmDeleteToken string
	confirmDeleteBtn   widget.Clickable
	cancelDeleteBtn    widget.Clickable
}

// NewSessionsPage constructs a SessionsPage.
func NewSessionsPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *SessionsPage {
	p := &SessionsPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	return p
}

// OnNavigatedTo loads sessions when the page becomes active.
func (p *SessionsPage) OnNavigatedTo(_ map[string]string) {
	p.load()
}

// OnNavigatedFrom is a no-op.
func (p *SessionsPage) OnNavigatedFrom() {}

func (p *SessionsPage) load() {
	if p.loading {
		return
	}
	c := p.state.Client()
	if c == nil {
		return
	}
	p.loading = true
	p.errorMsg = ""

	go func() {
		sessions, err := c.ListSessions(context.Background())
		if err != nil {
			p.errorMsg = "Failed to load sessions: " + err.Error()
			p.logger.Error("sessions load", "err", err)
		} else {
			p.sessions = sessions
			p.deleteBtns = make([]widget.Clickable, len(sessions))
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *SessionsPage) deleteConfirmed() {
	c := p.state.Client()
	if c == nil {
		return
	}
	token := p.confirmDeleteToken
	p.confirmDeleteToken = ""

	go func() {
		if err := c.DeleteSession(context.Background(), token); err != nil {
			p.errorMsg = "Delete failed: " + err.Error()
			p.logger.Error("session delete", "err", err)
			p.window.Invalidate()
			return
		}
		p.load()
	}()
}

// Layout renders the sessions page.
func (p *SessionsPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.refreshBtn.Clicked(gtx) {
		p.load()
	}
	if p.confirmDeleteBtn.Clicked(gtx) {
		p.deleteConfirmed()
	}
	if p.cancelDeleteBtn.Clicked(gtx) {
		p.confirmDeleteToken = ""
	}

	for i := range p.deleteBtns {
		if i < len(p.sessions) && p.deleteBtns[i].Clicked(gtx) {
			p.confirmDeleteToken = p.sessions[i].Token
		}
	}

	return layout.Inset{
		Left:   unit.Dp(24),
		Right:  unit.Dp(24),
		Top:    unit.Dp(16),
		Bottom: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.layoutHeader(gtx, th)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.errorMsg != "" {
					lbl := material.Body1(th, p.errorMsg)
					lbl.Color = desktopapp.ColorDanger
					return lbl.Layout(gtx)
				}
				return layout.Dimensions{}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.confirmDeleteToken != "" {
					return p.layoutDeleteConfirm(gtx, th)
				}
				return layout.Dimensions{}
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.layoutTable(gtx, th)
			}),
		)
	})
}

func (p *SessionsPage) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.H5(th, "Active Sessions").Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := "Refresh"
			if p.loading {
				label = "Loading..."
			}
			btn := material.Button(th, &p.refreshBtn, label)
			btn.Background = desktopapp.ColorSecondary
			return btn.Layout(gtx)
		}),
	)
}

func (p *SessionsPage) layoutDeleteConfirm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body1(th, "Revoke this session? The user will be logged out.")
				lbl.Color = desktopapp.ColorDanger
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.confirmDeleteBtn, "Revoke")
						btn.Background = desktopapp.ColorDanger
						return btn.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.cancelDeleteBtn, "Cancel")
						btn.Background = desktopapp.ColorSecondary
						return btn.Layout(gtx)
					}),
				)
			}),
		)
	})
}

// sessionCols defines the sessions table columns.
var sessionCols = []struct {
	title string
	flex  float32
}{
	{"Token", 0.30},
	{"User ID", 0.25},
	{"Created", 0.18},
	{"Expires", 0.18},
	{"", 0.09},
}

func (p *SessionsPage) layoutTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.loading && len(p.sessions) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.sessions) == 0 {
		lbl := material.Body1(th, "No active sessions.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutTableHeader(gtx, th)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.sessions),
				func(gtx layout.Context, i int) layout.Dimensions {
					return p.layoutRow(gtx, th, i)
				})
		}),
	)
}

func (p *SessionsPage) layoutTableHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(sessionCols))
	for _, col := range sessionCols {
		col := col
		children = append(children, layout.Flexed(col.flex, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, col.title)
			lbl.Color = desktopapp.ColorTextLight
			return lbl.Layout(gtx)
		}))
	}
	return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx, children...)
	})
}

func (p *SessionsPage) layoutRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	sess := p.sessions[i]

	for len(p.deleteBtns) <= i {
		p.deleteBtns = append(p.deleteBtns, widget.Clickable{})
	}

	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			// Masked token.
			layout.Flexed(sessionCols[0].flex, func(gtx layout.Context) layout.Dimensions {
				masked := maskToken(sess.Token)
				lbl := material.Body2(th, masked)
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			// User ID.
			layout.Flexed(sessionCols[1].flex, func(gtx layout.Context) layout.Dimensions {
				uid := sess.UserID
				if len(uid) > 8 {
					uid = uid[:8] + "..."
				}
				lbl := material.Body2(th, uid)
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}),
			// Created.
			layout.Flexed(sessionCols[2].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, sess.CreatedAt.Format("01-02 15:04"))
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			// Expires.
			layout.Flexed(sessionCols[3].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, sess.ExpiresAt.Format("01-02 15:04"))
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			// Delete button.
			layout.Flexed(sessionCols[4].flex, func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(th, &p.deleteBtns[i], "Del")
				btn.Background = desktopapp.ColorDanger
				return btn.Layout(gtx)
			}),
		)
	})
}

// maskToken shows only the first 4 and last 4 characters of a token.
func maskToken(token string) string {
	if len(token) <= 8 {
		return "****"
	}
	return fmt.Sprintf("%s...%s", token[:4], token[len(token)-4:])
}
