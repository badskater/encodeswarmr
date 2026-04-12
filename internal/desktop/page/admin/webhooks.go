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

// WebhooksPage manages notification webhooks.
type WebhooksPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	items    []client.Webhook
	loading  bool
	errorMsg string
	infoMsg  string
	list     widget.List

	createBtn  widget.Clickable
	refreshBtn widget.Clickable

	editBtns   []widget.Clickable
	deleteBtns []widget.Clickable
	testBtns   []widget.Clickable
	toggleBtns []widget.Clickable

	showForm   bool
	editingID  string
	formName   widget.Editor
	formProv   widget.Editor
	formURL    widget.Editor
	formEvents widget.Editor // comma-separated
	formSubmit widget.Clickable
	formCancel widget.Clickable
	formErr    string

	confirmDeleteID  string
	confirmDeleteBtn widget.Clickable
	cancelDeleteBtn  widget.Clickable
}

// NewWebhooksPage constructs a WebhooksPage.
func NewWebhooksPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *WebhooksPage {
	p := &WebhooksPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	p.formName.SingleLine = true
	p.formProv.SingleLine = true
	p.formURL.SingleLine = true
	p.formEvents.SingleLine = true
	return p
}

func (p *WebhooksPage) OnNavigatedTo(_ map[string]string) { p.refresh() }
func (p *WebhooksPage) OnNavigatedFrom()                  {}

func (p *WebhooksPage) refresh() {
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
		items, err := c.ListWebhooks(context.Background())
		if err != nil {
			p.errorMsg = "Failed to load webhooks: " + err.Error()
			p.logger.Error("webhooks load", "err", err)
		} else {
			p.items = items
			n := len(items)
			p.editBtns = make([]widget.Clickable, n)
			p.deleteBtns = make([]widget.Clickable, n)
			p.testBtns = make([]widget.Clickable, n)
			p.toggleBtns = make([]widget.Clickable, n)
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *WebhooksPage) openCreate() {
	p.editingID = ""
	p.formName.SetText("")
	p.formProv.SetText("discord")
	p.formURL.SetText("")
	p.formEvents.SetText("job.complete,job.failed")
	p.formErr = ""
	p.showForm = true
}

func (p *WebhooksPage) openEdit(wh client.Webhook) {
	p.editingID = wh.ID
	p.formName.SetText(wh.Name)
	p.formProv.SetText(wh.Provider)
	p.formURL.SetText(wh.URL)
	p.formEvents.SetText(strings.Join(wh.Events, ","))
	p.formErr = ""
	p.showForm = true
}

func (p *WebhooksPage) submitForm() {
	c := p.state.Client()
	if c == nil {
		return
	}
	name := strings.TrimSpace(p.formName.Text())
	prov := strings.TrimSpace(p.formProv.Text())
	u := strings.TrimSpace(p.formURL.Text())
	evStr := strings.TrimSpace(p.formEvents.Text())
	if name == "" || u == "" {
		p.formErr = "Name and URL are required"
		return
	}
	events := []string{}
	for _, e := range strings.Split(evStr, ",") {
		e = strings.TrimSpace(e)
		if e != "" {
			events = append(events, e)
		}
	}
	body := map[string]any{
		"name":     name,
		"provider": prov,
		"url":      u,
		"events":   events,
		"enabled":  true,
	}
	editingID := p.editingID
	go func() {
		var err error
		if editingID == "" {
			_, err = c.CreateWebhook(context.Background(), body)
		} else {
			_, err = c.UpdateWebhook(context.Background(), editingID, body)
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

func (p *WebhooksPage) testWebhook(id string) {
	c := p.state.Client()
	if c == nil {
		return
	}
	go func() {
		if err := c.TestWebhook(context.Background(), id); err != nil {
			p.infoMsg = "Test failed: " + err.Error()
		} else {
			p.infoMsg = "Test delivery sent."
		}
		p.window.Invalidate()
	}()
}

func (p *WebhooksPage) toggleWebhook(wh client.Webhook) {
	c := p.state.Client()
	if c == nil {
		return
	}
	body := map[string]any{
		"name":     wh.Name,
		"provider": wh.Provider,
		"url":      wh.URL,
		"events":   wh.Events,
		"enabled":  !wh.Enabled,
	}
	go func() {
		if _, err := c.UpdateWebhook(context.Background(), wh.ID, body); err != nil {
			p.errorMsg = "Toggle failed: " + err.Error()
			p.window.Invalidate()
			return
		}
		p.refresh()
	}()
}

func (p *WebhooksPage) deleteConfirmed() {
	c := p.state.Client()
	if c == nil {
		return
	}
	id := p.confirmDeleteID
	p.confirmDeleteID = ""
	go func() {
		if err := c.DeleteWebhook(context.Background(), id); err != nil {
			p.errorMsg = "Delete failed: " + err.Error()
			p.logger.Error("webhook delete", "err", err)
			p.window.Invalidate()
			return
		}
		p.refresh()
	}()
}

// Layout renders the webhooks page.
func (p *WebhooksPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
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
	for i := range p.testBtns {
		if i < len(p.items) && p.testBtns[i].Clicked(gtx) {
			p.testWebhook(p.items[i].ID)
		}
	}
	for i := range p.toggleBtns {
		if i < len(p.items) && p.toggleBtns[i].Clicked(gtx) {
			p.toggleWebhook(p.items[i])
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
						return material.H5(th, "Webhooks").Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.createBtn, "New Webhook")
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
				if p.infoMsg != "" {
					lbl := material.Body1(th, p.infoMsg)
					lbl.Color = desktopapp.ColorSuccess
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

func (p *WebhooksPage) layoutForm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	title := "New Webhook"
	if p.editingID != "" {
		title = "Edit Webhook"
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
				return labeledField(gtx, th, "Provider (discord/slack/teams)", &p.formProv)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return labeledField(gtx, th, "URL", &p.formURL)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return labeledField(gtx, th, "Events (comma-separated)", &p.formEvents)
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

func (p *WebhooksPage) layoutDeleteConfirm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body1(th, "Delete this webhook?")
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

var webhookCols = []colSpec{
	{"Name", 0.18},
	{"Provider", 0.10},
	{"URL", 0.25},
	{"Events", 0.17},
	{"Enabled", 0.08},
	{"Actions", 0.22},
}

func (p *WebhooksPage) layoutTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.loading && len(p.items) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.items) == 0 {
		lbl := material.Body1(th, "No webhooks configured.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutColHeader(gtx, th, webhookCols)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.items), func(gtx layout.Context, i int) layout.Dimensions {
				return p.layoutRow(gtx, th, i)
			})
		}),
	)
}

func (p *WebhooksPage) layoutRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	wh := p.items[i]
	for len(p.editBtns) <= i {
		p.editBtns = append(p.editBtns, widget.Clickable{})
		p.deleteBtns = append(p.deleteBtns, widget.Clickable{})
		p.testBtns = append(p.testBtns, widget.Clickable{})
		p.toggleBtns = append(p.toggleBtns, widget.Clickable{})
	}
	toggleLabel := "Disable"
	if !wh.Enabled {
		toggleLabel = "Enable"
	}
	urlDisplay := wh.URL
	if len(urlDisplay) > 30 {
		urlDisplay = urlDisplay[:30] + "..."
	}
	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(webhookCols[0].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, wh.Name)
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}),
			layout.Flexed(webhookCols[1].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, wh.Provider)
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(webhookCols[2].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, urlDisplay)
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(webhookCols[3].flex, func(gtx layout.Context) layout.Dimensions {
				ev := strings.Join(wh.Events, ", ")
				lbl := material.Caption(th, ev)
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(webhookCols[4].flex, func(gtx layout.Context) layout.Dimensions {
				col := desktopapp.ColorDanger
				if wh.Enabled {
					col = desktopapp.ColorSuccess
				}
				lbl := material.Body2(th, boolStr(wh.Enabled))
				lbl.Color = col
				return lbl.Layout(gtx)
			}),
			layout.Flexed(webhookCols[5].flex, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.testBtns[i], "Test")
						btn.Background = desktopapp.ColorPrimary
						return btn.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
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
