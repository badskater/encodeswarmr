package page

import (
	"context"
	"image"
	"log/slog"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	desktopapp "github.com/badskater/encodeswarmr/internal/desktop/app"
	"github.com/badskater/encodeswarmr/internal/desktop/client"
	"github.com/badskater/encodeswarmr/internal/desktop/nav"
	"github.com/badskater/encodeswarmr/internal/desktop/profile"
)

// LoginPage renders the connection / authentication screen.
type LoginPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	// Form fields
	urlEditor      widget.Editor
	usernameEditor widget.Editor
	passwordEditor widget.Editor
	apiKeyEditor   widget.Editor
	loginBtn       widget.Clickable
	saveProfileBtn widget.Clickable
	errorMsg       string
	loading        bool

	// Auth mode toggle
	useAPIKey     bool
	toggleAuthBtn widget.Clickable

	// Profile management
	profileStore      *profile.Store
	profileBtns       []widget.Clickable
	deleteProfileBtns []widget.Clickable
}

// NewLoginPage constructs a LoginPage and loads saved profiles.
func NewLoginPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *LoginPage {
	store, err := profile.NewStore()
	if err != nil {
		logger.Error("failed to load profiles", "err", err)
	}

	p := &LoginPage{
		state:        state,
		router:       router,
		window:       w,
		logger:       logger,
		profileStore: store,
	}
	p.urlEditor.SingleLine = true
	p.usernameEditor.SingleLine = true
	p.passwordEditor.SingleLine = true
	p.passwordEditor.Mask = '*'
	p.apiKeyEditor.SingleLine = true
	p.apiKeyEditor.Mask = '*'

	// Default URL hint.
	p.urlEditor.SetText("http://localhost:8080")

	p.updateProfileButtons()
	return p
}

// OnNavigatedTo resets transient state when the page becomes active.
func (p *LoginPage) OnNavigatedTo(_ map[string]string) {
	p.errorMsg = ""
	p.loading = false
}

// OnNavigatedFrom is a no-op for this page.
func (p *LoginPage) OnNavigatedFrom() {}

func (p *LoginPage) updateProfileButtons() {
	if p.profileStore == nil {
		return
	}
	profiles := p.profileStore.Profiles()
	p.profileBtns = make([]widget.Clickable, len(profiles))
	p.deleteProfileBtns = make([]widget.Clickable, len(profiles))
}

// Layout renders the login page.
func (p *LoginPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	p.handleEvents(gtx)

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = gtx.Dp(unit.Dp(480))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// Title block.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: unit.Dp(32)}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.H4(th, "EncodeSwarmr")
								lbl.Alignment = text.Middle
								return lbl.Layout(gtx)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								lbl := material.Body2(th, "Desktop Manager")
								lbl.Alignment = text.Middle
								lbl.Color = desktopapp.ColorTextLight
								return lbl.Layout(gtx)
							}),
						)
					},
				)
			}),
			// Saved profiles.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.layoutProfiles(gtx, th)
			}),
			// Login form card.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.layoutCard(gtx, th)
			}),
		)
	})
}

func (p *LoginPage) layoutProfiles(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.profileStore == nil {
		return layout.Dimensions{}
	}
	profiles := p.profileStore.Profiles()
	if len(profiles) == 0 {
		return layout.Dimensions{}
	}

	items := make([]layout.FlexChild, 0, len(profiles)+1)

	items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, "SAVED PROFILES")
				lbl.Color = desktopapp.ColorTextLight
				return lbl.Layout(gtx)
			})
	}))

	for i := range profiles {
		idx := i
		items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if p.profileBtns[idx].Clicked(gtx) {
				p.loadProfile(profiles[idx])
			}
			return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx,
				func(gtx layout.Context) layout.Dimensions {
					return material.Clickable(gtx, &p.profileBtns[idx],
						func(gtx layout.Context) layout.Dimensions {
							rowH := gtx.Dp(unit.Dp(44))
							bounds := image.Rect(0, 0, gtx.Constraints.Max.X, rowH)
							rr := gtx.Dp(unit.Dp(6))
							paint.FillShape(gtx.Ops, desktopapp.ColorSurface,
								clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

							return layout.Inset{
								Left:   unit.Dp(12),
								Right:  unit.Dp(12),
								Top:    unit.Dp(10),
								Bottom: unit.Dp(10),
							}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												lbl := material.Body2(th, profiles[idx].Name)
												lbl.Font.Weight = font.Bold
												return lbl.Layout(gtx)
											}),
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												lbl := material.Caption(th, profiles[idx].URL)
												lbl.Color = desktopapp.ColorTextLight
												return lbl.Layout(gtx)
											}),
										)
									}),
								)
							})
						},
					)
				},
			)
		}))
	}

	return layout.Inset{Bottom: unit.Dp(16)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, items...)
		})
}

func (p *LoginPage) layoutCard(gtx layout.Context, th *material.Theme) layout.Dimensions {
	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	rr := gtx.Dp(unit.Dp(8))
	paint.FillShape(gtx.Ops, desktopapp.ColorSurface,
		clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

	return layout.Inset{
		Left:   unit.Dp(24),
		Right:  unit.Dp(24),
		Top:    unit.Dp(24),
		Bottom: unit.Dp(24),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			// URL field.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.layoutField(gtx, th, "Controller URL", &p.urlEditor)
			}),
			// Auth mode toggle.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.toggleAuthBtn.Clicked(gtx) {
					p.useAPIKey = !p.useAPIKey
				}
				label := "Using Password"
				if p.useAPIKey {
					label = "Using API Key"
				}
				return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.toggleAuthBtn, label)
						btn.Background = desktopapp.ColorMuted
						return btn.Layout(gtx)
					})
			}),
			// Username (session mode).
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.useAPIKey {
					return layout.Dimensions{}
				}
				return p.layoutField(gtx, th, "Username", &p.usernameEditor)
			}),
			// Password (session mode).
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.useAPIKey {
					return layout.Dimensions{}
				}
				return p.layoutField(gtx, th, "Password", &p.passwordEditor)
			}),
			// API Key (key mode).
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !p.useAPIKey {
					return layout.Dimensions{}
				}
				return p.layoutField(gtx, th, "API Key", &p.apiKeyEditor)
			}),
			// Error message.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.errorMsg == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, p.errorMsg)
						lbl.Color = desktopapp.ColorDanger
						return lbl.Layout(gtx)
					})
			}),
			// Action buttons.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.saveProfileBtn, "Save Profile")
						btn.Background = desktopapp.ColorSecondary
						return btn.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						label := "Connect"
						if p.loading {
							label = "Connecting..."
						}
						btn := material.Button(th, &p.loginBtn, label)
						btn.Background = desktopapp.ColorPrimary
						return btn.Layout(gtx)
					}),
				)
			}),
		)
	})
}

func (p *LoginPage) layoutField(gtx layout.Context, th *material.Theme, label string, editor *widget.Editor) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					lbl := material.Caption(th, label)
					lbl.Color = desktopapp.ColorTextLight
					return lbl.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					border := widget.Border{
						Color:        desktopapp.ColorBorder,
						CornerRadius: unit.Dp(4),
						Width:        unit.Dp(1),
					}
					return border.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{
							Left:   unit.Dp(8),
							Right:  unit.Dp(8),
							Top:    unit.Dp(8),
							Bottom: unit.Dp(8),
						}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							ed := material.Editor(th, editor, "")
							return ed.Layout(gtx)
						})
					})
				}),
			)
		})
}

func (p *LoginPage) handleEvents(gtx layout.Context) {
	if p.loading {
		return
	}
	if p.loginBtn.Clicked(gtx) {
		p.doLogin()
	}
	if p.saveProfileBtn.Clicked(gtx) {
		p.doSaveProfile()
	}
}

func (p *LoginPage) loadProfile(prof profile.Profile) {
	p.urlEditor.SetText(prof.URL)
	if prof.AuthMode == "apikey" {
		p.useAPIKey = true
		p.apiKeyEditor.SetText(prof.APIKey)
	} else {
		p.useAPIKey = false
		p.usernameEditor.SetText(prof.Username)
	}
}

func (p *LoginPage) doSaveProfile() {
	if p.profileStore == nil {
		return
	}
	url := p.urlEditor.Text()
	if url == "" {
		p.errorMsg = "URL is required"
		return
	}

	prof := profile.Profile{
		Name: url,
		URL:  url,
	}
	if p.useAPIKey {
		prof.AuthMode = "apikey"
		prof.APIKey = p.apiKeyEditor.Text()
	} else {
		prof.AuthMode = "session"
		prof.Username = p.usernameEditor.Text()
	}

	if err := p.profileStore.Add(prof); err != nil {
		p.errorMsg = "Failed to save profile: " + err.Error()
		return
	}
	p.updateProfileButtons()
	p.errorMsg = ""
}

func (p *LoginPage) doLogin() {
	url := p.urlEditor.Text()
	if url == "" {
		p.errorMsg = "Controller URL is required"
		return
	}

	p.loading = true
	p.errorMsg = ""

	go func() {
		c := client.New(url)
		ctx := context.Background()

		if p.useAPIKey {
			key := p.apiKeyEditor.Text()
			if key == "" {
				p.errorMsg = "API Key is required"
				p.loading = false
				p.window.Invalidate()
				return
			}
			c.SetAPIKey(key)
		} else {
			username := p.usernameEditor.Text()
			password := p.passwordEditor.Text()
			if username == "" || password == "" {
				p.errorMsg = "Username and password are required"
				p.loading = false
				p.window.Invalidate()
				return
			}
			if err := c.Login(ctx, username, password); err != nil {
				p.errorMsg = "Login failed: " + err.Error()
				p.loading = false
				p.window.Invalidate()
				return
			}
		}

		// Verify connectivity by fetching the current user.
		user, err := c.GetMe(ctx)
		if err != nil {
			p.errorMsg = "Connection failed: " + err.Error()
			p.loading = false
			p.window.Invalidate()
			return
		}

		p.state.SetClient(c)
		p.state.SetUser(user)
		p.state.SetProfileName(url)

		// Establish WebSocket for live events (non-fatal if it fails).
		ws, err := c.ConnectWS(ctx, p.logger)
		if err != nil {
			p.logger.Warn("websocket connection failed", "err", err)
		} else {
			p.state.SetWSClient(ws)
		}

		p.loading = false
		p.router.Replace("/dashboard", nil)
		p.window.Invalidate()
	}()
}
