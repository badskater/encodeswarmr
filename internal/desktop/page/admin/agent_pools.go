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

// AgentPoolsPage manages agent pools.
type AgentPoolsPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	items    []client.AgentPool
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
	formColor  widget.Editor
	formTags   widget.Editor // comma-separated
	formSubmit widget.Clickable
	formCancel widget.Clickable
	formErr    string

	confirmDeleteID  string
	confirmDeleteBtn widget.Clickable
	cancelDeleteBtn  widget.Clickable
}

// NewAgentPoolsPage constructs an AgentPoolsPage.
func NewAgentPoolsPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *AgentPoolsPage {
	p := &AgentPoolsPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	p.formName.SingleLine = true
	p.formDesc.SingleLine = true
	p.formColor.SingleLine = true
	p.formTags.SingleLine = true
	return p
}

func (p *AgentPoolsPage) OnNavigatedTo(_ map[string]string) { p.refresh() }
func (p *AgentPoolsPage) OnNavigatedFrom()                  {}

func (p *AgentPoolsPage) refresh() {
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
		items, err := c.ListAgentPools(context.Background())
		if err != nil {
			p.errorMsg = "Failed to load agent pools: " + err.Error()
			p.logger.Error("agent pools load", "err", err)
		} else {
			p.items = items
			p.editBtns = make([]widget.Clickable, len(items))
			p.deleteBtns = make([]widget.Clickable, len(items))
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *AgentPoolsPage) openCreate() {
	p.editingID = ""
	p.formName.SetText("")
	p.formDesc.SetText("")
	p.formColor.SetText("#3B82F6")
	p.formTags.SetText("")
	p.formErr = ""
	p.showForm = true
}

func (p *AgentPoolsPage) openEdit(pool client.AgentPool) {
	p.editingID = pool.ID
	p.formName.SetText(pool.Name)
	p.formDesc.SetText(pool.Description)
	p.formColor.SetText(pool.Color)
	p.formTags.SetText(strings.Join(pool.Tags, ","))
	p.formErr = ""
	p.showForm = true
}

func (p *AgentPoolsPage) parseTags() []string {
	var tags []string
	for _, t := range strings.Split(p.formTags.Text(), ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

func (p *AgentPoolsPage) submitForm() {
	c := p.state.Client()
	if c == nil {
		return
	}
	name := strings.TrimSpace(p.formName.Text())
	desc := strings.TrimSpace(p.formDesc.Text())
	color := strings.TrimSpace(p.formColor.Text())
	tags := p.parseTags()
	if name == "" {
		p.formErr = "Name is required"
		return
	}
	editingID := p.editingID
	go func() {
		var err error
		if editingID == "" {
			_, err = c.CreateAgentPool(context.Background(), name, desc, color, tags)
		} else {
			_, err = c.UpdateAgentPool(context.Background(), editingID, name, desc, color, tags)
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

func (p *AgentPoolsPage) deleteConfirmed() {
	c := p.state.Client()
	if c == nil {
		return
	}
	id := p.confirmDeleteID
	p.confirmDeleteID = ""
	go func() {
		if err := c.DeleteAgentPool(context.Background(), id); err != nil {
			p.errorMsg = "Delete failed: " + err.Error()
			p.logger.Error("agent pool delete", "err", err)
			p.window.Invalidate()
			return
		}
		p.refresh()
	}()
}

// Layout renders the agent pools page.
func (p *AgentPoolsPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
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
						return material.H5(th, "Agent Pools").Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.createBtn, "New Pool")
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

func (p *AgentPoolsPage) layoutForm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	title := "New Agent Pool"
	if p.editingID != "" {
		title = "Edit Agent Pool"
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
				return labeledField(gtx, th, "Color (hex)", &p.formColor)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return labeledField(gtx, th, "Tags (comma-separated)", &p.formTags)
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

func (p *AgentPoolsPage) layoutDeleteConfirm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body1(th, "Delete this agent pool?")
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

var agentPoolCols = []colSpec{
	{"Name", 0.20},
	{"Description", 0.25},
	{"Tags", 0.22},
	{"Color", 0.10},
	{"Created", 0.12},
	{"Actions", 0.11},
}

func (p *AgentPoolsPage) layoutTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.loading && len(p.items) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.items) == 0 {
		lbl := material.Body1(th, "No agent pools found.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutColHeader(gtx, th, agentPoolCols)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.items), func(gtx layout.Context, i int) layout.Dimensions {
				return p.layoutRow(gtx, th, i)
			})
		}),
	)
}

func (p *AgentPoolsPage) layoutRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	pool := p.items[i]
	for len(p.editBtns) <= i {
		p.editBtns = append(p.editBtns, widget.Clickable{})
		p.deleteBtns = append(p.deleteBtns, widget.Clickable{})
	}
	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(agentPoolCols[0].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, pool.Name)
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}),
			layout.Flexed(agentPoolCols[1].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, pool.Description)
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(agentPoolCols[2].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, strings.Join(pool.Tags, ", "))
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(agentPoolCols[3].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, pool.Color)
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(agentPoolCols[4].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, pool.CreatedAt.Format("01-02-06"))
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(agentPoolCols[5].flex, func(gtx layout.Context) layout.Dimensions {
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
