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

// PathMappingsPage manages UNC-to-Linux path mappings.
type PathMappingsPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	items    []client.PathMapping
	loading  bool
	errorMsg string
	list     widget.List

	createBtn  widget.Clickable
	refreshBtn widget.Clickable

	editBtns   []widget.Clickable
	deleteBtns []widget.Clickable
	toggleBtns []widget.Clickable

	showForm      bool
	editingID     string
	formName      widget.Editor
	formWinPrefix widget.Editor
	formLinPrefix widget.Editor
	formSubmit    widget.Clickable
	formCancel    widget.Clickable
	formErr       string

	confirmDeleteID  string
	confirmDeleteBtn widget.Clickable
	cancelDeleteBtn  widget.Clickable
}

// NewPathMappingsPage constructs a PathMappingsPage.
func NewPathMappingsPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *PathMappingsPage {
	p := &PathMappingsPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	p.formName.SingleLine = true
	p.formWinPrefix.SingleLine = true
	p.formLinPrefix.SingleLine = true
	return p
}

func (p *PathMappingsPage) OnNavigatedTo(_ map[string]string) { p.refresh() }
func (p *PathMappingsPage) OnNavigatedFrom()                  {}

func (p *PathMappingsPage) refresh() {
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
		items, err := c.ListPathMappings(context.Background())
		if err != nil {
			p.errorMsg = "Failed to load path mappings: " + err.Error()
			p.logger.Error("path mappings load", "err", err)
		} else {
			p.items = items
			p.editBtns = make([]widget.Clickable, len(items))
			p.deleteBtns = make([]widget.Clickable, len(items))
			p.toggleBtns = make([]widget.Clickable, len(items))
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *PathMappingsPage) openCreate() {
	p.editingID = ""
	p.formName.SetText("")
	p.formWinPrefix.SetText(`\\nas\share`)
	p.formLinPrefix.SetText("/mnt/nas")
	p.formErr = ""
	p.showForm = true
}

func (p *PathMappingsPage) openEdit(pm client.PathMapping) {
	p.editingID = pm.ID
	p.formName.SetText(pm.Name)
	p.formWinPrefix.SetText(pm.WindowsPrefix)
	p.formLinPrefix.SetText(pm.LinuxPrefix)
	p.formErr = ""
	p.showForm = true
}

func (p *PathMappingsPage) submitForm() {
	c := p.state.Client()
	if c == nil {
		return
	}
	name := strings.TrimSpace(p.formName.Text())
	win := strings.TrimSpace(p.formWinPrefix.Text())
	lin := strings.TrimSpace(p.formLinPrefix.Text())
	if name == "" || win == "" || lin == "" {
		p.formErr = "All fields are required"
		return
	}
	editingID := p.editingID
	go func() {
		var err error
		if editingID == "" {
			_, err = c.CreatePathMapping(context.Background(), name, win, lin, true)
		} else {
			body := map[string]any{
				"name":           name,
				"windows_prefix": win,
				"linux_prefix":   lin,
			}
			_, err = c.UpdatePathMapping(context.Background(), editingID, body)
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

func (p *PathMappingsPage) toggleMapping(pm client.PathMapping) {
	c := p.state.Client()
	if c == nil {
		return
	}
	body := map[string]any{
		"name":           pm.Name,
		"windows_prefix": pm.WindowsPrefix,
		"linux_prefix":   pm.LinuxPrefix,
		"enabled":        !pm.Enabled,
	}
	go func() {
		if _, err := c.UpdatePathMapping(context.Background(), pm.ID, body); err != nil {
			p.errorMsg = "Toggle failed: " + err.Error()
			p.logger.Error("path mapping toggle", "err", err)
			p.window.Invalidate()
			return
		}
		p.refresh()
	}()
}

func (p *PathMappingsPage) deleteConfirmed() {
	c := p.state.Client()
	if c == nil {
		return
	}
	id := p.confirmDeleteID
	p.confirmDeleteID = ""
	go func() {
		if err := c.DeletePathMapping(context.Background(), id); err != nil {
			p.errorMsg = "Delete failed: " + err.Error()
			p.logger.Error("path mapping delete", "err", err)
			p.window.Invalidate()
			return
		}
		p.refresh()
	}()
}

// Layout renders the path mappings page.
func (p *PathMappingsPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
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
	for i := range p.toggleBtns {
		if i < len(p.items) && p.toggleBtns[i].Clicked(gtx) {
			p.toggleMapping(p.items[i])
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
						return material.H5(th, "Path Mappings").Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.createBtn, "New Mapping")
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

func (p *PathMappingsPage) layoutForm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	title := "New Path Mapping"
	if p.editingID != "" {
		title = "Edit Path Mapping"
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
				return labeledField(gtx, th, "Windows Prefix (e.g. \\\\nas\\share)", &p.formWinPrefix)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return labeledField(gtx, th, "Linux Prefix (e.g. /mnt/nas)", &p.formLinPrefix)
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

func (p *PathMappingsPage) layoutDeleteConfirm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body1(th, "Delete this path mapping?")
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

var pathMappingCols = []colSpec{
	{"Name", 0.15},
	{"Windows Prefix", 0.25},
	{"Linux Prefix", 0.25},
	{"Enabled", 0.10},
	{"Created", 0.12},
	{"Actions", 0.13},
}

func (p *PathMappingsPage) layoutTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.loading && len(p.items) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.items) == 0 {
		lbl := material.Body1(th, "No path mappings configured.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutColHeader(gtx, th, pathMappingCols)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.items), func(gtx layout.Context, i int) layout.Dimensions {
				return p.layoutRow(gtx, th, i)
			})
		}),
	)
}

func (p *PathMappingsPage) layoutRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	pm := p.items[i]
	for len(p.editBtns) <= i {
		p.editBtns = append(p.editBtns, widget.Clickable{})
		p.deleteBtns = append(p.deleteBtns, widget.Clickable{})
		p.toggleBtns = append(p.toggleBtns, widget.Clickable{})
	}
	toggleLabel := "Disable"
	if !pm.Enabled {
		toggleLabel = "Enable"
	}
	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(pathMappingCols[0].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, pm.Name)
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}),
			layout.Flexed(pathMappingCols[1].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, pm.WindowsPrefix)
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(pathMappingCols[2].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, pm.LinuxPrefix)
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(pathMappingCols[3].flex, func(gtx layout.Context) layout.Dimensions {
				col := desktopapp.ColorDanger
				if pm.Enabled {
					col = desktopapp.ColorSuccess
				}
				lbl := material.Body2(th, boolStr(pm.Enabled))
				lbl.Color = col
				return lbl.Layout(gtx)
			}),
			layout.Flexed(pathMappingCols[4].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, pm.CreatedAt.Format("01-02-06"))
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(pathMappingCols[5].flex, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.toggleBtns[i], toggleLabel)
						btn.Background = desktopapp.ColorSecondary
						return btn.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
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
