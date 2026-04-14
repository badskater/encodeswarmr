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

// NotificationsPage manages notification preferences.
type NotificationsPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	prefs    *client.NotificationPrefs
	loading  bool
	errorMsg string
	infoMsg  string

	// Preference checkboxes.
	checkOnComplete    widget.Bool
	checkOnFailed      widget.Bool
	checkOnAgentStale  widget.Bool
	checkWebhookUser   widget.Bool
	checkEmail         widget.Bool
	formEmailAddr      widget.Editor

	// Buttons.
	saveBtn        widget.Clickable
	refreshBtn     widget.Clickable
	testEmailBtn   widget.Clickable
	testTelegramBtn widget.Clickable
	testPushoverBtn widget.Clickable
	testNtfyBtn    widget.Clickable
}

// NewNotificationsPage constructs a NotificationsPage.
func NewNotificationsPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *NotificationsPage {
	p := &NotificationsPage{
		state:  state,
		router: router,
		window: w,
		logger: logger,
	}
	p.formEmailAddr.SingleLine = true
	return p
}

func (p *NotificationsPage) OnNavigatedTo(_ map[string]string) { p.refresh() }
func (p *NotificationsPage) OnNavigatedFrom()                  {}

func (p *NotificationsPage) refresh() {
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
		prefs, err := c.GetNotificationPrefs(context.Background())
		if err != nil {
			p.errorMsg = "Failed to load notification prefs: " + err.Error()
			p.logger.Error("notification prefs load", "err", err)
		} else {
			p.prefs = prefs
			p.populateForm(prefs)
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *NotificationsPage) populateForm(prefs *client.NotificationPrefs) {
	p.checkOnComplete.Value = prefs.NotifyOnJobComplete
	p.checkOnFailed.Value = prefs.NotifyOnJobFailed
	p.checkOnAgentStale.Value = prefs.NotifyOnAgentStale
	p.checkWebhookUser.Value = prefs.WebhookFilterUserOnly
	p.checkEmail.Value = prefs.NotifyEmail
	p.formEmailAddr.SetText(prefs.EmailAddress)
}

func (p *NotificationsPage) save() {
	c := p.state.Client()
	if c == nil {
		return
	}
	email := strings.TrimSpace(p.formEmailAddr.Text())
	body := map[string]any{
		"notify_on_job_complete":   p.checkOnComplete.Value,
		"notify_on_job_failed":     p.checkOnFailed.Value,
		"notify_on_agent_stale":    p.checkOnAgentStale.Value,
		"webhook_filter_user_only": p.checkWebhookUser.Value,
		"notify_email":             p.checkEmail.Value,
		"email_address":            email,
	}
	go func() {
		if _, err := c.UpdateNotificationPrefs(context.Background(), body); err != nil {
			p.errorMsg = "Save failed: " + err.Error()
			p.logger.Error("notification prefs save", "err", err)
			p.window.Invalidate()
			return
		}
		p.infoMsg = "Preferences saved."
		p.refresh()
	}()
}

func (p *NotificationsPage) testEmail() {
	c := p.state.Client()
	if c == nil {
		return
	}
	to := strings.TrimSpace(p.formEmailAddr.Text())
	go func() {
		if err := c.TestEmail(context.Background(), to); err != nil {
			p.errorMsg = "Email test failed: " + err.Error()
		} else {
			p.infoMsg = "Test email sent."
		}
		p.window.Invalidate()
	}()
}

func (p *NotificationsPage) testTelegram() {
	c := p.state.Client()
	if c == nil {
		return
	}
	go func() {
		if err := c.TestTelegram(context.Background()); err != nil {
			p.errorMsg = "Telegram test failed: " + err.Error()
		} else {
			p.infoMsg = "Telegram test sent."
		}
		p.window.Invalidate()
	}()
}

func (p *NotificationsPage) testPushover() {
	c := p.state.Client()
	if c == nil {
		return
	}
	go func() {
		if err := c.TestPushover(context.Background()); err != nil {
			p.errorMsg = "Pushover test failed: " + err.Error()
		} else {
			p.infoMsg = "Pushover test sent."
		}
		p.window.Invalidate()
	}()
}

func (p *NotificationsPage) testNtfy() {
	c := p.state.Client()
	if c == nil {
		return
	}
	go func() {
		if err := c.TestNtfy(context.Background()); err != nil {
			p.errorMsg = "Ntfy test failed: " + err.Error()
		} else {
			p.infoMsg = "Ntfy test sent."
		}
		p.window.Invalidate()
	}()
}

// Layout renders the notifications settings page.
func (p *NotificationsPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.refreshBtn.Clicked(gtx) {
		p.refresh()
	}
	if p.saveBtn.Clicked(gtx) {
		p.save()
	}
	if p.testEmailBtn.Clicked(gtx) {
		p.testEmail()
	}
	if p.testTelegramBtn.Clicked(gtx) {
		p.testTelegram()
	}
	if p.testPushoverBtn.Clicked(gtx) {
		p.testPushover()
	}
	if p.testNtfyBtn.Clicked(gtx) {
		p.testNtfy()
	}

	return layout.Inset{
		Left: unit.Dp(24), Right: unit.Dp(24),
		Top: unit.Dp(16), Bottom: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return material.H5(th, "Notification Settings").Layout(gtx)
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
				if p.loading && p.prefs == nil {
					return material.Body1(th, "Loading...").Layout(gtx)
				}
				return p.layoutForm(gtx, th)
			}),
		)
	})
}

func (p *NotificationsPage) layoutForm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(th, "Preferences").Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.CheckBox(th, &p.checkOnComplete, "Notify on job complete").Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.CheckBox(th, &p.checkOnFailed, "Notify on job failed").Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.CheckBox(th, &p.checkOnAgentStale, "Notify on agent stale").Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.CheckBox(th, &p.checkWebhookUser, "Webhook: only my jobs").Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.CheckBox(th, &p.checkEmail, "Email notifications").Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return labeledField(gtx, th, "Email address", &p.formEmailAddr)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(th, &p.saveBtn, "Save Preferences")
			btn.Background = desktopapp.ColorPrimary
			return btn.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(24)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.H6(th, "Test Notifications").Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(th, &p.testEmailBtn, "Test Email")
					btn.Background = desktopapp.ColorSecondary
					return btn.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(th, &p.testTelegramBtn, "Test Telegram")
					btn.Background = desktopapp.ColorSecondary
					return btn.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(th, &p.testPushoverBtn, "Test Pushover")
					btn.Background = desktopapp.ColorSecondary
					return btn.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := material.Button(th, &p.testNtfyBtn, "Test Ntfy")
					btn.Background = desktopapp.ColorSecondary
					return btn.Layout(gtx)
				}),
			)
		}),
	)
}
