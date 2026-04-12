package admin

import (
	"context"
	"log/slog"
	"strings"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	desktopapp "github.com/badskater/encodeswarmr/internal/desktop/app"
	"github.com/badskater/encodeswarmr/internal/desktop/client"
	"github.com/badskater/encodeswarmr/internal/desktop/nav"
)

// UsersPage manages user accounts.
type UsersPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	items    []client.User
	loading  bool
	errorMsg string
	list     widget.List

	createBtn  widget.Clickable
	refreshBtn widget.Clickable

	deleteBtns      []widget.Clickable
	roleAdminBtns   []widget.Clickable
	roleEncoderBtns []widget.Clickable
	roleViewerBtns  []widget.Clickable

	showForm    bool
	formUser    widget.Editor
	formEmail   widget.Editor
	formRole    widget.Editor
	formPass    widget.Editor
	formSubmit  widget.Clickable
	formCancel  widget.Clickable
	formErr     string

	confirmDeleteID  string
	confirmDeleteBtn widget.Clickable
	cancelDeleteBtn  widget.Clickable
}

// NewUsersPage constructs a UsersPage.
func NewUsersPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *UsersPage {
	p := &UsersPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	p.formUser.SingleLine = true
	p.formEmail.SingleLine = true
	p.formRole.SingleLine = true
	p.formPass.SingleLine = true
	return p
}

func (p *UsersPage) OnNavigatedTo(_ map[string]string) { p.refresh() }
func (p *UsersPage) OnNavigatedFrom()                  {}

func (p *UsersPage) refresh() {
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
		items, err := c.ListUsers(context.Background())
		if err != nil {
			p.errorMsg = "Failed to load users: " + err.Error()
			p.logger.Error("users load", "err", err)
		} else {
			p.items = items
			n := len(items)
			p.deleteBtns = make([]widget.Clickable, n)
			p.roleAdminBtns = make([]widget.Clickable, n)
			p.roleEncoderBtns = make([]widget.Clickable, n)
			p.roleViewerBtns = make([]widget.Clickable, n)
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *UsersPage) openCreate() {
	p.formUser.SetText("")
	p.formEmail.SetText("")
	p.formRole.SetText("viewer")
	p.formPass.SetText("")
	p.formErr = ""
	p.showForm = true
}

func (p *UsersPage) submitForm() {
	c := p.state.Client()
	if c == nil {
		return
	}
	username := strings.TrimSpace(p.formUser.Text())
	email := strings.TrimSpace(p.formEmail.Text())
	role := strings.TrimSpace(p.formRole.Text())
	pass := p.formPass.Text()
	if username == "" || email == "" || pass == "" {
		p.formErr = "Username, email and password are required"
		return
	}
	go func() {
		_, err := c.CreateUser(context.Background(), username, email, role, pass)
		if err != nil {
			p.formErr = err.Error()
			p.window.Invalidate()
			return
		}
		p.showForm = false
		p.refresh()
	}()
}

func (p *UsersPage) changeRole(id, role string) {
	c := p.state.Client()
	if c == nil {
		return
	}
	go func() {
		if err := c.UpdateUserRole(context.Background(), id, role); err != nil {
			p.errorMsg = "Role update failed: " + err.Error()
			p.logger.Error("user role update", "err", err)
			p.window.Invalidate()
			return
		}
		p.refresh()
	}()
}

func (p *UsersPage) deleteConfirmed() {
	c := p.state.Client()
	if c == nil {
		return
	}
	id := p.confirmDeleteID
	p.confirmDeleteID = ""
	go func() {
		if err := c.DeleteUser(context.Background(), id); err != nil {
			p.errorMsg = "Delete failed: " + err.Error()
			p.logger.Error("user delete", "err", err)
			p.window.Invalidate()
			return
		}
		p.refresh()
	}()
}

// Layout renders the users page.
func (p *UsersPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.createBtn.Clicked(gtx) {
		p.openCreate()
	}
	if p.refreshBtn.Clicked(gtx) {
		p.refresh()
	}
	if p.formSubmit.Clicked(gtx) {
		p.submitForm()
	}
	if p.formCancel.Clicked(gtx) {
		p.showForm = false
		p.formErr = ""
	}
	if p.confirmDeleteBtn.Clicked(gtx) {
		p.deleteConfirmed()
	}
	if p.cancelDeleteBtn.Clicked(gtx) {
		p.confirmDeleteID = ""
	}
	for i := range p.deleteBtns {
		if i < len(p.items) && p.deleteBtns[i].Clicked(gtx) {
			p.confirmDeleteID = p.items[i].ID
		}
	}
	for i := range p.roleAdminBtns {
		if i < len(p.items) && p.roleAdminBtns[i].Clicked(gtx) {
			p.changeRole(p.items[i].ID, "admin")
		}
	}
	for i := range p.roleEncoderBtns {
		if i < len(p.items) && p.roleEncoderBtns[i].Clicked(gtx) {
			p.changeRole(p.items[i].ID, "encoder")
		}
	}
	for i := range p.roleViewerBtns {
		if i < len(p.items) && p.roleViewerBtns[i].Clicked(gtx) {
			p.changeRole(p.items[i].ID, "viewer")
		}
	}

	return layout.Inset{
		Left: unit.Dp(24), Right: unit.Dp(24),
		Top: unit.Dp(16), Bottom: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.H5(th, "Users").Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.createBtn, "New User")
						btn.Background = desktopapp.ColorPrimary
						return btn.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
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
				if p.confirmDeleteID != "" {
					return p.layoutDeleteConfirm(gtx, th)
				}
				return layout.Dimensions{}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.showForm {
					return p.layoutForm(gtx, th)
				}
				return layout.Dimensions{}
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.layoutTable(gtx, th)
			}),
		)
	})
}

func (p *UsersPage) layoutForm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.H6(th, "New User").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return labeledField(gtx, th, "Username", &p.formUser)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return labeledField(gtx, th, "Email", &p.formEmail)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return labeledField(gtx, th, "Role (admin/encoder/viewer)", &p.formRole)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return labeledField(gtx, th, "Password", &p.formPass)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.formErr != "" {
					lbl := material.Body2(th, p.formErr)
					lbl.Color = desktopapp.ColorDanger
					return lbl.Layout(gtx)
				}
				return layout.Dimensions{}
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.formSubmit, "Create")
						btn.Background = desktopapp.ColorPrimary
						return btn.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.formCancel, "Cancel")
						btn.Background = desktopapp.ColorSecondary
						return btn.Layout(gtx)
					}),
				)
			}),
		)
	})
}

func (p *UsersPage) layoutDeleteConfirm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body1(th, "Delete this user? This cannot be undone.")
				lbl.Color = desktopapp.ColorDanger
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.confirmDeleteBtn, "Delete")
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

var userCols = []colSpec{
	{"Username", 0.20},
	{"Email", 0.25},
	{"Role", 0.12},
	{"Created", 0.15},
	{"Actions", 0.28},
}

func (p *UsersPage) layoutTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.loading && len(p.items) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.items) == 0 {
		lbl := material.Body1(th, "No users found.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutColHeader(gtx, th, userCols)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.items), func(gtx layout.Context, i int) layout.Dimensions {
				return p.layoutRow(gtx, th, i)
			})
		}),
	)
}

func (p *UsersPage) layoutRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	u := p.items[i]
	for len(p.deleteBtns) <= i {
		p.deleteBtns = append(p.deleteBtns, widget.Clickable{})
		p.roleAdminBtns = append(p.roleAdminBtns, widget.Clickable{})
		p.roleEncoderBtns = append(p.roleEncoderBtns, widget.Clickable{})
		p.roleViewerBtns = append(p.roleViewerBtns, widget.Clickable{})
	}
	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(userCols[0].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, u.Username)
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}),
			layout.Flexed(userCols[1].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, u.Email)
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(userCols[2].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, u.Role)
				lbl.Color = desktopapp.ColorPrimary
				return lbl.Layout(gtx)
			}),
			layout.Flexed(userCols[3].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, u.CreatedAt.Format("01-02-06"))
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(userCols[4].flex, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.roleAdminBtns[i], "Admin")
						btn.Background = desktopapp.ColorWarning
						return btn.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.roleEncoderBtns[i], "Encoder")
						btn.Background = desktopapp.ColorSecondary
						return btn.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.roleViewerBtns[i], "Viewer")
						btn.Background = desktopapp.ColorSecondary
						return btn.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.deleteBtns[i], "Del")
						btn.Background = desktopapp.ColorDanger
						return btn.Layout(gtx)
					}),
				)
			}),
		)
	})
}
