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

// AutoScalingPage manages auto-scaling settings.
type AutoScalingPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	settings *client.AutoScalingSettings
	loading  bool
	errorMsg string
	infoMsg  string

	// Form fields.
	enabledCheck    widget.Bool
	formWebhookURL  widget.Editor
	formScaleUp     widget.Editor
	formScaleDown   widget.Editor
	formCooldown    widget.Editor
	saveBtn         widget.Clickable
	testWebhookBtn  widget.Clickable
	refreshBtn      widget.Clickable
}

// NewAutoScalingPage constructs an AutoScalingPage.
func NewAutoScalingPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *AutoScalingPage {
	p := &AutoScalingPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.formWebhookURL.SingleLine = true
	p.formScaleUp.SingleLine = true
	p.formScaleDown.SingleLine = true
	p.formCooldown.SingleLine = true
	return p
}

func (p *AutoScalingPage) OnNavigatedTo(_ map[string]string) { p.refresh() }
func (p *AutoScalingPage) OnNavigatedFrom()                  {}

func (p *AutoScalingPage) refresh() {
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
		settings, err := c.GetAutoScaling(context.Background())
		if err != nil {
			p.errorMsg = "Failed to load auto-scaling settings: " + err.Error()
			p.logger.Error("auto scaling load", "err", err)
		} else {
			p.settings = settings
			p.populateForm(settings)
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *AutoScalingPage) populateForm(s *client.AutoScalingSettings) {
	p.enabledCheck.Value = s.Enabled
	p.formWebhookURL.SetText(s.WebhookURL)
	p.formScaleUp.SetText(strconv.FormatFloat(s.ScaleUpThreshold, 'f', 2, 64))
	p.formScaleDown.SetText(strconv.FormatFloat(s.ScaleDownThreshold, 'f', 2, 64))
	p.formCooldown.SetText(strconv.Itoa(s.CooldownSeconds))
}

func (p *AutoScalingPage) save() {
	c := p.state.Client()
	if c == nil {
		return
	}
	webhookURL := strings.TrimSpace(p.formWebhookURL.Text())
	scaleUpStr := strings.TrimSpace(p.formScaleUp.Text())
	scaleDownStr := strings.TrimSpace(p.formScaleDown.Text())
	cooldownStr := strings.TrimSpace(p.formCooldown.Text())

	scaleUp, err := strconv.ParseFloat(scaleUpStr, 64)
	if err != nil {
		p.errorMsg = "Invalid scale-up threshold"
		return
	}
	scaleDown, err := strconv.ParseFloat(scaleDownStr, 64)
	if err != nil {
		p.errorMsg = "Invalid scale-down threshold"
		return
	}
	cooldown, err := strconv.Atoi(cooldownStr)
	if err != nil {
		p.errorMsg = "Invalid cooldown value"
		return
	}
	body := map[string]any{
		"enabled":              p.enabledCheck.Value,
		"webhook_url":          webhookURL,
		"scale_up_threshold":   scaleUp,
		"scale_down_threshold": scaleDown,
		"cooldown_seconds":     cooldown,
	}
	go func() {
		if _, err := c.UpdateAutoScaling(context.Background(), body); err != nil {
			p.errorMsg = "Save failed: " + err.Error()
			p.logger.Error("auto scaling save", "err", err)
			p.window.Invalidate()
			return
		}
		p.infoMsg = "Settings saved."
		p.refresh()
	}()
}

func (p *AutoScalingPage) testWebhook() {
	c := p.state.Client()
	if c == nil {
		return
	}
	go func() {
		if err := c.TestAutoScalingWebhook(context.Background()); err != nil {
			p.errorMsg = "Webhook test failed: " + err.Error()
		} else {
			p.infoMsg = "Webhook test sent."
		}
		p.window.Invalidate()
	}()
}

// Layout renders the auto-scaling settings page.
func (p *AutoScalingPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.refreshBtn.Clicked(gtx) {
		p.refresh()
	}
	if p.saveBtn.Clicked(gtx) {
		p.save()
	}
	if p.testWebhookBtn.Clicked(gtx) {
		p.testWebhook()
	}

	return layout.Inset{
		Left: unit.Dp(24), Right: unit.Dp(24),
		Top: unit.Dp(16), Bottom: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.H5(th, "Auto-Scaling Settings").Layout(gtx)
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
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.loading && p.settings == nil {
					return material.Body1(th, "Loading...").Layout(gtx)
				}
				return p.layoutForm(gtx, th)
			}),
		)
	})
}

func (p *AutoScalingPage) layoutForm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return material.CheckBox(th, &p.enabledCheck, "Enabled").Layout(gtx)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labeledField(gtx, th, "Webhook URL", &p.formWebhookURL)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labeledField(gtx, th, "Scale Up Threshold (queue depth ratio)", &p.formScaleUp)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labeledField(gtx, th, "Scale Down Threshold (queue depth ratio)", &p.formScaleDown)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labeledField(gtx, th, "Cooldown (seconds)", &p.formCooldown)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(th, &p.saveBtn, "Save Settings")
					btn.Background = desktopapp.ColorPrimary
					return btn.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(th, &p.testWebhookBtn, "Test Webhook")
					btn.Background = desktopapp.ColorSecondary
					return btn.Layout(gtx)
				}),
			)
		}),
	)
}
