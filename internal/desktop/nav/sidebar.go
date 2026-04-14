package nav

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// SidebarItem represents a navigation link in the sidebar.
type SidebarItem struct {
	Label string
	Path  string
	btn   widget.Clickable
}

// SidebarGroup is a labelled group of navigation items.
type SidebarGroup struct {
	Label string
	Items []SidebarItem
}

// Sidebar is the left navigation panel shown when a user is logged in.
type Sidebar struct {
	router *Router
	groups []SidebarGroup
	width  unit.Dp
	list   widget.List
}

var (
	sidebarBg       = color.NRGBA{R: 17, G: 24, B: 39, A: 255}    // Gray-900
	sidebarText     = color.NRGBA{R: 209, G: 213, B: 219, A: 255} // Gray-300
	sidebarActive   = color.NRGBA{R: 55, G: 65, B: 81, A: 255}    // Gray-700
	sidebarActiveTx = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	sidebarGroupTx  = color.NRGBA{R: 156, G: 163, B: 175, A: 255} // Gray-400
	sidebarTitleTx  = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
)

// NewSidebar creates the sidebar with all navigation groups pre-populated.
func NewSidebar(router *Router) *Sidebar {
	s := &Sidebar{
		router: router,
		width:  unit.Dp(220),
		groups: []SidebarGroup{
			{
				Label: "OPERATIONS",
				Items: []SidebarItem{
					{Label: "Dashboard", Path: "/dashboard"},
					{Label: "Sources", Path: "/sources"},
					{Label: "Jobs", Path: "/jobs"},
					{Label: "Queue", Path: "/queue"},
					{Label: "Agents", Path: "/agents"},
					{Label: "Audio", Path: "/audio"},
					{Label: "Flows", Path: "/flows"},
					{Label: "Files", Path: "/files"},
					{Label: "Sessions", Path: "/sessions"},
				},
			},
			{
				Label: "ADMINISTRATION",
				Items: []SidebarItem{
					{Label: "Templates", Path: "/admin/templates"},
					{Label: "Variables", Path: "/admin/variables"},
					{Label: "Webhooks", Path: "/admin/webhooks"},
					{Label: "Users", Path: "/admin/users"},
					{Label: "API Keys", Path: "/admin/api-keys"},
					{Label: "Agent Pools", Path: "/admin/agent-pools"},
					{Label: "Path Mappings", Path: "/admin/path-mappings"},
					{Label: "Tokens", Path: "/admin/tokens"},
					{Label: "Schedules", Path: "/admin/schedules"},
					{Label: "Plugins", Path: "/admin/plugins"},
					{Label: "Encoding Rules", Path: "/admin/encoding-rules"},
					{Label: "Encoding Profiles", Path: "/admin/encoding-profiles"},
					{Label: "Watch Folders", Path: "/admin/watch-folders"},
					{Label: "Auto-Scaling", Path: "/admin/auto-scaling"},
					{Label: "Notifications", Path: "/admin/notifications"},
					{Label: "Audit Export", Path: "/admin/audit-export"},
				},
			},
		},
	}
	s.list.Axis = layout.Vertical
	return s
}

// Layout draws the sidebar and handles navigation click events.
func (s *Sidebar) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	width := gtx.Dp(s.width)

	// Fill sidebar background.
	rect := image.Rect(0, 0, width, gtx.Constraints.Max.Y)
	paint.FillShape(gtx.Ops, sidebarBg, clip.Rect(rect).Op())

	gtx.Constraints = layout.Exact(image.Point{X: width, Y: gtx.Constraints.Max.Y})

	currentPath := s.router.CurrentPath()

	// Calculate total list items: 1 title + per group (1 label + N items).
	totalItems := 1
	for _, g := range s.groups {
		totalItems += 1 + len(g.Items)
	}

	return layout.Inset{Top: unit.Dp(16)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &s.list).Layout(gtx, totalItems,
				func(gtx layout.Context, index int) layout.Dimensions {
					if index == 0 {
						return s.layoutTitle(gtx, th)
					}
					index--

					for gi := range s.groups {
						if index == 0 {
							return s.layoutGroupLabel(gtx, th, s.groups[gi].Label)
						}
						index--
						if index < len(s.groups[gi].Items) {
							return s.layoutItem(gtx, th, &s.groups[gi].Items[index], currentPath)
						}
						index -= len(s.groups[gi].Items)
					}
					return layout.Dimensions{}
				},
			)
		},
	)
}

func (s *Sidebar) layoutTitle(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{
		Left: unit.Dp(16), Right: unit.Dp(16), Bottom: unit.Dp(24),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		lbl := material.H6(th, "EncodeSwarmr")
		lbl.Color = sidebarTitleTx
		return lbl.Layout(gtx)
	})
}

func (s *Sidebar) layoutGroupLabel(gtx layout.Context, th *material.Theme, label string) layout.Dimensions {
	return layout.Inset{
		Left: unit.Dp(16), Right: unit.Dp(16), Top: unit.Dp(16), Bottom: unit.Dp(8),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		lbl := material.Caption(th, label)
		lbl.Color = sidebarGroupTx
		return lbl.Layout(gtx)
	})
}

func (s *Sidebar) layoutItem(gtx layout.Context, th *material.Theme, item *SidebarItem, currentPath string) layout.Dimensions {
	active := item.Path == currentPath

	// Handle navigation click before rendering so the frame reflects the new state.
	if item.btn.Clicked(gtx) {
		s.router.Push(item.Path, nil)
	}

	return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Bottom: unit.Dp(2)}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return material.Clickable(gtx, &item.btn, func(gtx layout.Context) layout.Dimensions {
				if active {
					bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Dp(unit.Dp(36)))
					rr := gtx.Dp(unit.Dp(6))
					paint.FillShape(gtx.Ops, sidebarActive,
						clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))
				}

				return layout.Inset{
					Left:   unit.Dp(12),
					Right:  unit.Dp(12),
					Top:    unit.Dp(8),
					Bottom: unit.Dp(8),
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					lbl := material.Body1(th, item.Label)
					if active {
						lbl.Color = sidebarActiveTx
					} else {
						lbl.Color = sidebarText
					}
					return lbl.Layout(gtx)
				})
			})
		},
	)
}
