// Package nav provides a simple stack-based page router for the desktop app.
package nav

import (
	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/widget/material"
)

// Page is implemented by every screen in the application.
type Page interface {
	// OnNavigatedTo is called when the page becomes the active page.
	// params carries any route parameters supplied by the caller.
	OnNavigatedTo(params map[string]string)
	// OnNavigatedFrom is called when the page is about to be replaced.
	OnNavigatedFrom()
	// Layout renders the page into gtx.
	Layout(gtx layout.Context, th *material.Theme) layout.Dimensions
}

// PageFactory is a function that constructs a Page on demand.
type PageFactory func() Page

// Router manages a stack of pages and drives navigation.
type Router struct {
	window   *app.Window
	registry map[string]PageFactory
	stack    []entry
}

type entry struct {
	path   string
	params map[string]string
	page   Page
}

// NewRouter creates a Router bound to the given window.
func NewRouter(w *app.Window) *Router {
	return &Router{
		window:   w,
		registry: make(map[string]PageFactory),
	}
}

// Register adds a factory for the given path pattern.
func (r *Router) Register(path string, factory PageFactory) {
	r.registry[path] = factory
}

// Push navigates to path, keeping the current page on the stack.
func (r *Router) Push(path string, params map[string]string) {
	r.navigate(path, params, false)
}

// Replace navigates to path, replacing the current page.
func (r *Router) Replace(path string, params map[string]string) {
	r.navigate(path, params, true)
}

// Pop returns to the previous page in the stack.
func (r *Router) Pop() {
	if len(r.stack) <= 1 {
		return
	}
	top := r.stack[len(r.stack)-1]
	top.page.OnNavigatedFrom()
	r.stack = r.stack[:len(r.stack)-1]
	if len(r.stack) > 0 {
		r.stack[len(r.stack)-1].page.OnNavigatedTo(r.stack[len(r.stack)-1].params)
	}
	r.window.Invalidate()
}

// CurrentPath returns the path of the active page.
func (r *Router) CurrentPath() string {
	if len(r.stack) == 0 {
		return ""
	}
	return r.stack[len(r.stack)-1].path
}

// Layout renders the active page.
func (r *Router) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if len(r.stack) == 0 {
		return layout.Dimensions{}
	}
	return r.stack[len(r.stack)-1].page.Layout(gtx, th)
}

func (r *Router) navigate(path string, params map[string]string, replace bool) {
	factory, ok := r.registry[path]
	if !ok {
		return
	}

	if len(r.stack) > 0 {
		r.stack[len(r.stack)-1].page.OnNavigatedFrom()
		if replace {
			r.stack = r.stack[:len(r.stack)-1]
		}
	}

	page := factory()
	e := entry{path: path, params: params, page: page}
	r.stack = append(r.stack, e)
	page.OnNavigatedTo(params)
	r.window.Invalidate()
}
