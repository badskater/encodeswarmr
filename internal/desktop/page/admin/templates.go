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

// TemplatesPage manages script templates (avs/vpy/bat).
type TemplatesPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	items    []client.Template
	loading  bool
	errorMsg string
	list     widget.List

	// Header actions.
	createBtn  widget.Clickable
	refreshBtn widget.Clickable

	// Per-row action buttons.
	editBtns   []widget.Clickable
	deleteBtns []widget.Clickable

	// Create/edit form.
	showForm    bool
	editingID   string
	formName    widget.Editor
	formType    widget.Editor
	formDesc    widget.Editor
	formContent widget.Editor
	formSubmit  widget.Clickable
	formCancel  widget.Clickable
	formErr     string

	// Delete confirmation.
	confirmDeleteID  string
	confirmDeleteBtn widget.Clickable
	cancelDeleteBtn  widget.Clickable
}

// NewTemplatesPage constructs a TemplatesPage.
func NewTemplatesPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *TemplatesPage {
	p := &TemplatesPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	p.formName.SingleLine = true
	p.formType.SingleLine = true
	p.formDesc.SingleLine = true
	return p
}

// OnNavigatedTo loads templates when the page becomes active.
func (p *TemplatesPage) OnNavigatedTo(_ map[string]string) { p.refresh() }

// OnNavigatedFrom is a no-op.
func (p *TemplatesPage) OnNavigatedFrom() {}

func (p *TemplatesPage) refresh() {
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
		items, err := c.ListTemplates(context.Background())
		if err != nil {
			p.errorMsg = "Failed to load templates: " + err.Error()
			p.logger.Error("templates load", "err", err)
		} else {
			p.items = items
			p.editBtns = make([]widget.Clickable, len(items))
			p.deleteBtns = make([]widget.Clickable, len(items))
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *TemplatesPage) openCreate() {
	p.editingID = ""
	p.formName.SetText("")
	p.formType.SetText("avs")
	p.formDesc.SetText("")
	p.formContent.SetText("")
	p.formErr = ""
	p.showForm = true
}

func (p *TemplatesPage) openEdit(t client.Template) {
	p.editingID = t.ID
	p.formName.SetText(t.Name)
	p.formType.SetText(t.Type)
	desc := ""
	if t.Description != nil {
		desc = *t.Description
	}
	p.formDesc.SetText(desc)
	p.formContent.SetText(t.Content)
	p.formErr = ""
	p.showForm = true
}

func (p *TemplatesPage) submitForm() {
	c := p.state.Client()
	if c == nil {
		return
	}
	name := strings.TrimSpace(p.formName.Text())
	typ := strings.TrimSpace(p.formType.Text())
	desc := strings.TrimSpace(p.formDesc.Text())
	content := p.formContent.Text()
	if name == "" || typ == "" {
		p.formErr = "Name and type are required"
		return
	}
	body := map[string]any{
		"name":        name,
		"type":        typ,
		"description": desc,
		"content":     content,
	}
	editingID := p.editingID
	go func() {
		var err error
		if editingID == "" {
			_, err = c.CreateTemplate(context.Background(), body)
		} else {
			_, err = c.UpdateTemplate(context.Background(), editingID, body)
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

func (p *TemplatesPage) deleteConfirmed() {
	c := p.state.Client()
	if c == nil {
		return
	}
	id := p.confirmDeleteID
	p.confirmDeleteID = ""
	go func() {
		if err := c.DeleteTemplate(context.Background(), id); err != nil {
			p.errorMsg = "Delete failed: " + err.Error()
			p.logger.Error("template delete", "err", err)
			p.window.Invalidate()
			return
		}
		p.refresh()
	}()
}

// Layout renders the templates management page.
func (p *TemplatesPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	// Header button events.
	if p.createBtn.Clicked(gtx) {
		p.openCreate()
	}
	if p.refreshBtn.Clicked(gtx) {
		p.refresh()
	}
	// Form events.
	if p.formSubmit.Clicked(gtx) {
		p.submitForm()
	}
	if p.formCancel.Clicked(gtx) {
		p.showForm = false
		p.formErr = ""
	}
	// Delete confirmation events.
	if p.confirmDeleteBtn.Clicked(gtx) {
		p.deleteConfirmed()
	}
	if p.cancelDeleteBtn.Clicked(gtx) {
		p.confirmDeleteID = ""
	}
	// Per-row events.
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

func (p *TemplatesPage) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.H5(th, "Templates").Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(th, &p.createBtn, "New Template")
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
}

func (p *TemplatesPage) layoutForm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	title := "New Template"
	if p.editingID != "" {
		title = "Edit Template"
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
				return labeledField(gtx, th, "Type (avs/vpy/bat)", &p.formType)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return labeledField(gtx, th, "Description", &p.formDesc)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, "Content")
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				ed := material.Editor(th, &p.formContent, "")
				gtx.Constraints.Min.Y = gtx.Dp(unit.Dp(80))
				return ed.Layout(gtx)
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

func (p *TemplatesPage) layoutDeleteConfirm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body1(th, "Delete this template? This cannot be undone.")
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

var templateCols = []colSpec{
	{"Name", 0.25},
	{"Type", 0.10},
	{"Description", 0.35},
	{"Updated", 0.15},
	{"Actions", 0.15},
}

func (p *TemplatesPage) layoutTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.loading && len(p.items) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.items) == 0 {
		lbl := material.Body1(th, "No templates found.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutColHeader(gtx, th, templateCols)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.items), func(gtx layout.Context, i int) layout.Dimensions {
				return p.layoutRow(gtx, th, i)
			})
		}),
	)
}

func (p *TemplatesPage) layoutRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	t := p.items[i]
	for len(p.editBtns) <= i {
		p.editBtns = append(p.editBtns, widget.Clickable{})
		p.deleteBtns = append(p.deleteBtns, widget.Clickable{})
	}
	desc := ""
	if t.Description != nil {
		desc = *t.Description
	}
	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(templateCols[0].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, t.Name)
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}),
			layout.Flexed(templateCols[1].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, t.Type)
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(templateCols[2].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, desc)
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(templateCols[3].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, t.UpdatedAt.Format("01-02 15:04"))
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(templateCols[4].flex, func(gtx layout.Context) layout.Dimensions {
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
