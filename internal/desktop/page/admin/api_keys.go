package admin

import (
	"context"
	"log/slog"
	"strconv"
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

// APIKeysPage manages API keys and their rate limits.
type APIKeysPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	items    []client.APIKeyInfo
	loading  bool
	errorMsg string
	infoMsg  string
	list     widget.List

	refreshBtn widget.Clickable

	// Per-row rate-limit editing.
	rateLimitEditors []widget.Editor
	saveRateBtns     []widget.Clickable
}

// NewAPIKeysPage constructs an APIKeysPage.
func NewAPIKeysPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *APIKeysPage {
	p := &APIKeysPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.list.Axis = layout.Vertical
	return p
}

func (p *APIKeysPage) OnNavigatedTo(_ map[string]string) { p.refresh() }
func (p *APIKeysPage) OnNavigatedFrom()                  {}

func (p *APIKeysPage) refresh() {
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
		items, err := c.ListAPIKeys(context.Background())
		if err != nil {
			p.errorMsg = "Failed to load API keys: " + err.Error()
			p.logger.Error("api keys load", "err", err)
		} else {
			p.items = items
			p.rateLimitEditors = make([]widget.Editor, len(items))
			p.saveRateBtns = make([]widget.Clickable, len(items))
			for i, k := range items {
				p.rateLimitEditors[i].SingleLine = true
				p.rateLimitEditors[i].SetText(strconv.Itoa(k.RateLimit))
			}
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *APIKeysPage) saveRateLimit(i int) {
	c := p.state.Client()
	if c == nil || i >= len(p.items) {
		return
	}
	id := p.items[i].ID
	text := strings.TrimSpace(p.rateLimitEditors[i].Text())
	rl, err := strconv.Atoi(text)
	if err != nil || rl < 0 {
		p.errorMsg = "Invalid rate limit value"
		return
	}
	go func() {
		if err := c.UpdateAPIKeyRateLimit(context.Background(), id, rl); err != nil {
			p.errorMsg = "Update failed: " + err.Error()
			p.logger.Error("api key rate limit", "err", err)
			p.window.Invalidate()
			return
		}
		p.infoMsg = "Rate limit updated."
		p.refresh()
	}()
}

// Layout renders the API keys page.
func (p *APIKeysPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.refreshBtn.Clicked(gtx) {
		p.refresh()
	}
	for i := range p.saveRateBtns {
		if i < len(p.items) && p.saveRateBtns[i].Clicked(gtx) {
			p.saveRateLimit(i)
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
						return material.H5(th, "API Keys").Layout(gtx)
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
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.layoutTable(gtx, th)
			}),
		)
	})
}

var apiKeyCols = []colSpec{
	{"Name", 0.22},
	{"Rate Limit (req/min)", 0.20},
	{"Created", 0.15},
	{"Last Used", 0.15},
	{"Expires", 0.15},
	{"Save", 0.13},
}

func (p *APIKeysPage) layoutTable(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.loading && len(p.items) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.items) == 0 {
		lbl := material.Body1(th, "No API keys found.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutColHeader(gtx, th, apiKeyCols)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.items), func(gtx layout.Context, i int) layout.Dimensions {
				return p.layoutRow(gtx, th, i)
			})
		}),
	)
}

func (p *APIKeysPage) layoutRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	k := p.items[i]
	for len(p.rateLimitEditors) <= i {
		e := widget.Editor{}
		e.SingleLine = true
		e.SetText(strconv.Itoa(k.RateLimit))
		p.rateLimitEditors = append(p.rateLimitEditors, e)
		p.saveRateBtns = append(p.saveRateBtns, widget.Clickable{})
	}
	lastUsed := "-"
	if k.LastUsedAt != nil {
		lastUsed = k.LastUsedAt.Format("01-02 15:04")
	}
	expires := "-"
	if k.ExpiresAt != nil {
		expires = k.ExpiresAt.Format("01-02-06")
	}
	return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(apiKeyCols[0].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, k.Name)
				lbl.MaxLines = 1
				return lbl.Layout(gtx)
			}),
			layout.Flexed(apiKeyCols[1].flex, func(gtx layout.Context) layout.Dimensions {
				return material.Editor(th, &p.rateLimitEditors[i], "").Layout(gtx)
			}),
			layout.Flexed(apiKeyCols[2].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, k.CreatedAt.Format("01-02-06"))
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(apiKeyCols[3].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, lastUsed)
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(apiKeyCols[4].flex, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, expires)
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			}),
			layout.Flexed(apiKeyCols[5].flex, func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(th, &p.saveRateBtns[i], "Save")
				btn.Background = desktopapp.ColorPrimary
				return btn.Layout(gtx)
			}),
		)
	})
}
