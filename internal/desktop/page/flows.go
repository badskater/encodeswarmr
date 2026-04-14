package page

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"log/slog"
	"strings"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	desktopapp "github.com/badskater/encodeswarmr/internal/desktop/app"
	"github.com/badskater/encodeswarmr/internal/desktop/client"
	"github.com/badskater/encodeswarmr/internal/desktop/nav"
)

// FlowsPage lists encoding flows with create and delete actions.
type FlowsPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	flows    []client.Flow
	loading  bool
	errorMsg string

	createBtn  widget.Clickable
	refreshBtn widget.Clickable
	list       widget.List
	rowBtns    []widget.Clickable
	deleteBtns []widget.Clickable

	// Inline create form.
	showForm    bool
	formName    widget.Editor
	formDesc    widget.Editor
	formSubmit  widget.Clickable
	formCancel  widget.Clickable
	formErr     string

	// Delete confirmation.
	confirmDeleteID  string
	confirmDeleteBtn widget.Clickable
	cancelDeleteBtn  widget.Clickable
}

// NewFlowsPage constructs a FlowsPage.
func NewFlowsPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *FlowsPage {
	p := &FlowsPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	p.formName.SingleLine = true
	p.formDesc.SingleLine = true
	return p
}

// OnNavigatedTo loads flows when the page becomes active.
func (p *FlowsPage) OnNavigatedTo(_ map[string]string) {
	p.load()
}

// OnNavigatedFrom is a no-op.
func (p *FlowsPage) OnNavigatedFrom() {}

func (p *FlowsPage) load() {
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
		flows, err := c.ListFlows(context.Background())
		if err != nil {
			p.errorMsg = "Failed to load flows: " + err.Error()
			p.logger.Error("flows load", "err", err)
		} else {
			p.flows = flows
			n := len(flows)
			p.rowBtns = make([]widget.Clickable, n)
			p.deleteBtns = make([]widget.Clickable, n)
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *FlowsPage) openCreate() {
	p.formName.SetText("")
	p.formDesc.SetText("")
	p.formErr = ""
	p.showForm = true
}

func (p *FlowsPage) submitCreate() {
	c := p.state.Client()
	if c == nil {
		return
	}
	name := strings.TrimSpace(p.formName.Text())
	desc := strings.TrimSpace(p.formDesc.Text())
	if name == "" {
		p.formErr = "Name is required"
		return
	}

	go func() {
		body := map[string]any{
			"name":        name,
			"description": desc,
			"nodes":       json.RawMessage("[]"),
			"edges":       json.RawMessage("[]"),
		}
		_, err := c.CreateFlow(context.Background(), body)
		if err != nil {
			p.formErr = err.Error()
			p.window.Invalidate()
			return
		}
		p.showForm = false
		p.load()
	}()
}

func (p *FlowsPage) deleteConfirmed() {
	c := p.state.Client()
	if c == nil {
		return
	}
	id := p.confirmDeleteID
	p.confirmDeleteID = ""

	go func() {
		if err := c.DeleteFlow(context.Background(), id); err != nil {
			p.errorMsg = "Delete failed: " + err.Error()
			p.logger.Error("flow delete", "err", err)
			p.window.Invalidate()
			return
		}
		p.load()
	}()
}

// Layout renders the flows list page.
func (p *FlowsPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.createBtn.Clicked(gtx) {
		p.openCreate()
	}
	if p.refreshBtn.Clicked(gtx) {
		p.load()
	}
	if p.formSubmit.Clicked(gtx) {
		p.submitCreate()
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

	for i := range p.rowBtns {
		if i < len(p.flows) && p.rowBtns[i].Clicked(gtx) {
			p.router.Push("/flows/detail", map[string]string{"id": p.flows[i].ID})
		}
	}
	for i := range p.deleteBtns {
		if i < len(p.flows) && p.deleteBtns[i].Clicked(gtx) {
			p.confirmDeleteID = p.flows[i].ID
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

func (p *FlowsPage) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.H5(th, "Flows").Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(th, &p.createBtn, "Create")
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

func (p *FlowsPage) layoutForm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.H6(th, "New Flow").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, "Name")
						lbl.Color = desktopapp.ColorTextLight
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Editor(th, &p.formName, "Flow name").Layout(gtx)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, "Description")
						lbl.Color = desktopapp.ColorTextLight
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Editor(th, &p.formDesc, "Optional description").Layout(gtx)
					}),
				)
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

func (p *FlowsPage) layoutDeleteConfirm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body1(th, "Delete this flow? This cannot be undone.")
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

// flowCols defines the flows table columns.
var flowCols = []struct {
	title string
	flex  float32
}{
	{"Name", 0.22},
	{"Description", 0.30},
	{"Nodes", 0.10},
	{"Edges", 0.10},
	{"Updated", 0.18},
	{"", 0.10},
}

func (p *FlowsPage) layoutTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.loading && len(p.flows) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.flows) == 0 {
		lbl := material.Body1(th, "No flows found. Create one to get started.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutTableHeader(gtx, th)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.flows),
				func(gtx layout.Context, i int) layout.Dimensions {
					return p.layoutRow(gtx, th, i)
				})
		}),
	)
}

func (p *FlowsPage) layoutTableHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(flowCols))
	for _, col := range flowCols {
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

func (p *FlowsPage) layoutRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	flow := p.flows[i]

	for len(p.rowBtns) <= i {
		p.rowBtns = append(p.rowBtns, widget.Clickable{})
		p.deleteBtns = append(p.deleteBtns, widget.Clickable{})
	}

	nodeCount := countJSONArray(flow.Nodes)
	edgeCount := countJSONArray(flow.Edges)

	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.Clickable(gtx, &p.rowBtns[i], func(gtx layout.Context) layout.Dimensions {
			bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
			rr := gtx.Dp(unit.Dp(4))
			paint.FillShape(gtx.Ops, desktopapp.ColorSurface,
				clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

			return layout.Inset{
				Top: unit.Dp(8), Bottom: unit.Dp(8),
				Left: unit.Dp(8), Right: unit.Dp(8),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					// Name.
					layout.Flexed(flowCols[0].flex, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, flow.Name)
						lbl.MaxLines = 1
						return lbl.Layout(gtx)
					}),
					// Description.
					layout.Flexed(flowCols[1].flex, func(gtx layout.Context) layout.Dimensions {
						desc := flow.Description
						if desc == "" {
							desc = "-"
						}
						lbl := material.Body2(th, desc)
						lbl.MaxLines = 1
						lbl.Color = desktopapp.ColorTextLight
						return lbl.Layout(gtx)
					}),
					// Node count.
					layout.Flexed(flowCols[2].flex, func(gtx layout.Context) layout.Dimensions {
						return material.Body2(th, fmt.Sprintf("%d", nodeCount)).Layout(gtx)
					}),
					// Edge count.
					layout.Flexed(flowCols[3].flex, func(gtx layout.Context) layout.Dimensions {
						return material.Body2(th, fmt.Sprintf("%d", edgeCount)).Layout(gtx)
					}),
					// Updated at.
					layout.Flexed(flowCols[4].flex, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, flow.UpdatedAt.Format("2006-01-02 15:04"))
						lbl.Color = desktopapp.ColorTextLight
						return lbl.Layout(gtx)
					}),
					// Delete button.
					layout.Flexed(flowCols[5].flex, func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.deleteBtns[i], "Del")
						btn.Background = desktopapp.ColorDanger
						return btn.Layout(gtx)
					}),
				)
			})
		})
	})
}

// countJSONArray counts elements in a raw JSON array without full unmarshalling.
func countJSONArray(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return 0
	}
	return len(arr)
}
