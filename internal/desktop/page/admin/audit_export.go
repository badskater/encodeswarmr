package admin

import (
	"context"
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

// AuditExportPage shows recent audit log entries and allows export.
type AuditExportPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	entries  []client.AuditEntry
	loading  bool
	errorMsg string
	infoMsg  string
	list     widget.List

	refreshBtn    widget.Clickable
	exportCSVBtn  widget.Clickable
	exportJSONBtn widget.Clickable
}

// NewAuditExportPage constructs an AuditExportPage.
func NewAuditExportPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *AuditExportPage {
	p := &AuditExportPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	return p
}

func (p *AuditExportPage) OnNavigatedTo(_ map[string]string) { p.refresh() }
func (p *AuditExportPage) OnNavigatedFrom()                  {}

func (p *AuditExportPage) refresh() {
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
		col, err := c.ListAuditLog(context.Background(), 100, 0)
		if err != nil {
			p.errorMsg = "Failed to load audit log: " + err.Error()
			p.logger.Error("audit log load", "err", err)
		} else {
			p.entries = col.Items
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *AuditExportPage) exportURL(format string) string {
	c := p.state.Client()
	if c == nil {
		return ""
	}
	return c.AuditLogExportURL(format, 10000)
}

// Layout renders the audit export page.
func (p *AuditExportPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.refreshBtn.Clicked(gtx) {
		p.refresh()
	}
	if p.exportCSVBtn.Clicked(gtx) {
		url := p.exportURL("csv")
		if url != "" {
			p.infoMsg = "Export URL: " + url
		}
	}
	if p.exportJSONBtn.Clicked(gtx) {
		url := p.exportURL("json")
		if url != "" {
			p.infoMsg = "Export URL: " + url
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
						return material.H5(th, "Audit Log").Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.exportCSVBtn, "Export CSV")
						btn.Background = desktopapp.ColorSecondary
						return btn.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.exportJSONBtn, "Export JSON")
						btn.Background = desktopapp.ColorSecondary
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
					lbl := material.Body2(th, p.infoMsg)
					lbl.Color = desktopapp.ColorPrimary
					return lbl.Layout(gtx)
				}
				return layout.Dimensions{}
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.layoutTable(gtx, th)
			}),
		)
	})
}

var auditCols = []colSpec{
	{"Username", 0.15},
	{"Action", 0.15},
	{"Resource", 0.15},
	{"Resource ID", 0.20},
	{"IP", 0.15},
	{"Timestamp", 0.20},
}

func (p *AuditExportPage) layoutTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.loading && len(p.entries) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.entries) == 0 {
		lbl := material.Body1(th, "No audit entries found.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutColHeader(gtx, th, auditCols)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.entries), func(gtx layout.Context, i int) layout.Dimensions {
				return p.layoutRow(gtx, th, i)
			})
		}),
	)
}

func (p *AuditExportPage) layoutRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	e := p.entries[i]
	resID := e.ResourceID
	if len(resID) > 12 {
		resID = resID[:12] + "..."
	}
	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(auditCols[0].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, e.Username)
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}),
			layout.Flexed(auditCols[1].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, e.Action)
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorPrimary
				return lbl.Layout(gtx)
			}),
			layout.Flexed(auditCols[2].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, e.Resource)
				lbl.MaxLines = 1
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(auditCols[3].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, resID)
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(auditCols[4].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, e.IPAddress)
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(auditCols[5].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, e.LoggedAt.Format("01-02 15:04:05"))
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
		)
	})
}
