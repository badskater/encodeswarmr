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

// EncodingProfilesPage manages encoding profiles.
type EncodingProfilesPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	items    []client.EncodingProfile
	loading  bool
	errorMsg string
	list     widget.List

	createBtn  widget.Clickable
	refreshBtn widget.Clickable

	editBtns   []widget.Clickable
	deleteBtns []widget.Clickable

	showForm   bool
	editingID  string
	formName   widget.Editor
	formDesc   widget.Editor
	formCont   widget.Editor // container
	formSubmit widget.Clickable
	formCancel widget.Clickable
	formErr    string

	confirmDeleteID  string
	confirmDeleteBtn widget.Clickable
	cancelDeleteBtn  widget.Clickable
}

// NewEncodingProfilesPage constructs an EncodingProfilesPage.
func NewEncodingProfilesPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *EncodingProfilesPage {
	p := &EncodingProfilesPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	p.formName.SingleLine = true
	p.formDesc.SingleLine = true
	p.formCont.SingleLine = true
	return p
}

func (p *EncodingProfilesPage) OnNavigatedTo(_ map[string]string) { p.refresh() }
func (p *EncodingProfilesPage) OnNavigatedFrom()                  {}

func (p *EncodingProfilesPage) refresh() {
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
		items, err := c.ListEncodingProfiles(context.Background())
		if err != nil {
			p.errorMsg = "Failed to load encoding profiles: " + err.Error()
			p.logger.Error("encoding profiles load", "err", err)
		} else {
			p.items = items
			p.editBtns = make([]widget.Clickable, len(items))
			p.deleteBtns = make([]widget.Clickable, len(items))
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *EncodingProfilesPage) openCreate() {
	p.editingID = ""
	p.formName.SetText("")
	p.formDesc.SetText("")
	p.formCont.SetText("mkv")
	p.formErr = ""
	p.showForm = true
}

func (p *EncodingProfilesPage) openEdit(ep client.EncodingProfile) {
	p.editingID = ep.ID
	p.formName.SetText(ep.Name)
	p.formDesc.SetText(ep.Description)
	p.formCont.SetText(ep.Container)
	p.formErr = ""
	p.showForm = true
}

func (p *EncodingProfilesPage) submitForm() {
	c := p.state.Client()
	if c == nil {
		return
	}
	name := strings.TrimSpace(p.formName.Text())
	desc := strings.TrimSpace(p.formDesc.Text())
	cont := strings.TrimSpace(p.formCont.Text())
	if name == "" {
		p.formErr = "Name is required"
		return
	}
	body := map[string]any{
		"name":        name,
		"description": desc,
		"container":   cont,
		"settings":    map[string]any{},
	}
	editingID := p.editingID
	go func() {
		var err error
		if editingID == "" {
			_, err = c.CreateEncodingProfile(context.Background(), body)
		} else {
			_, err = c.UpdateEncodingProfile(context.Background(), editingID, body)
		}
		if err != nil {
			p.formErr = err.Error()
			p.window.Invalidate()
			return
		}
		p.showForm = false
		p.refresh()
	}()
}

func (p *EncodingProfilesPage) deleteConfirmed() {
	c := p.state.Client()
	if c == nil {
		return
	}
	id := p.confirmDeleteID
	p.confirmDeleteID = ""
	go func() {
		if err := c.DeleteEncodingProfile(context.Background(), id); err != nil {
			p.errorMsg = "Delete failed: " + err.Error()
			p.logger.Error("encoding profile delete", "err", err)
			p.window.Invalidate()
			return
		}
		p.refresh()
	}()
}

// Layout renders the encoding profiles page.
func (p *EncodingProfilesPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
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
	for i := range p.editBtns {
		if i < len(p.items) && p.editBtns[i].Clicked(gtx) {
			p.openEdit(p.items[i])
		}
	}
	for i := range p.deleteBtns {
		if i < len(p.items) && p.deleteBtns[i].Clicked(gtx) {
			p.confirmDeleteID = p.items[i].ID
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
						return material.H5(th, "Encoding Profiles").Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.createBtn, "New Profile")
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

func (p *EncodingProfilesPage) layoutForm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	title := "New Encoding Profile"
	if p.editingID != "" {
		title = "Edit Encoding Profile"
	}
	return layout.Inset{Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.H6(th, title).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return labeledField(gtx, th, "Name", &p.formName)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return labeledField(gtx, th, "Description", &p.formDesc)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return labeledField(gtx, th, "Container (mkv/mp4/etc)", &p.formCont)
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
						btn := material.Button(th, &p.formSubmit, "Save")
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

func (p *EncodingProfilesPage) layoutDeleteConfirm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body1(th, "Delete this encoding profile?")
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

var encodingProfileCols = []colSpec{
	{"Name", 0.25},
	{"Container", 0.12},
	{"Description", 0.30},
	{"Created By", 0.15},
	{"Actions", 0.18},
}

func (p *EncodingProfilesPage) layoutTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.loading && len(p.items) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.items) == 0 {
		lbl := material.Body1(th, "No encoding profiles found.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutColHeader(gtx, th, encodingProfileCols)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.items), func(gtx layout.Context, i int) layout.Dimensions {
				return p.layoutRow(gtx, th, i)
			})
		}),
	)
}

func (p *EncodingProfilesPage) layoutRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	ep := p.items[i]
	for len(p.editBtns) <= i {
		p.editBtns = append(p.editBtns, widget.Clickable{})
		p.deleteBtns = append(p.deleteBtns, widget.Clickable{})
	}
	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(encodingProfileCols[0].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, ep.Name)
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}),
			layout.Flexed(encodingProfileCols[1].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, ep.Container)
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(encodingProfileCols[2].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, ep.Description)
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(encodingProfileCols[3].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, ep.CreatedBy)
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(encodingProfileCols[4].flex, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.editBtns[i], "Edit")
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
