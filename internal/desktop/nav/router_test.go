package nav

import (
	"testing"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/widget/material"
)

// mockPage is a test double that implements the Page interface and records
// navigation lifecycle calls for inspection.
type mockPage struct {
	navigatedTo   bool
	navigatedFrom bool
	lastParams    map[string]string
}

func (m *mockPage) OnNavigatedTo(params map[string]string) {
	m.navigatedTo = true
	m.lastParams = params
}

func (m *mockPage) OnNavigatedFrom() {
	m.navigatedFrom = true
}

func (m *mockPage) Layout(_ layout.Context, _ *material.Theme) layout.Dimensions {
	return layout.Dimensions{}
}

// newTestRouter creates a Router backed by a zero-value app.Window.
// app.Window.Invalidate is safe to call on a zero-value window (it is a no-op
// when the underlying driver is nil), so no display is needed for these tests.
func newTestRouter() *Router {
	return NewRouter(new(app.Window))
}

// TestNewRouter_EmptyStack verifies that a freshly created Router has no active page.
func TestNewRouter_EmptyStack(t *testing.T) {
	r := newTestRouter()

	if got := r.CurrentPath(); got != "" {
		t.Errorf("CurrentPath() = %q, want empty string on new router", got)
	}
	if len(r.stack) != 0 {
		t.Errorf("stack len = %d, want 0 on new router", len(r.stack))
	}
}

// TestRegisterAndPush_NavigatesToPage verifies that a registered path is navigable.
func TestRegisterAndPush_NavigatesToPage(t *testing.T) {
	r := newTestRouter()
	page := &mockPage{}
	r.Register("/home", func() Page { return page })

	r.Push("/home", nil)

	if got := r.CurrentPath(); got != "/home" {
		t.Errorf("CurrentPath() = %q, want %q", got, "/home")
	}
	if !page.navigatedTo {
		t.Error("OnNavigatedTo was not called after Push")
	}
}

// TestPush_UnregisteredPath_Ignored verifies that pushing an unknown path is silent.
func TestPush_UnregisteredPath_Ignored(t *testing.T) {
	r := newTestRouter()
	page := &mockPage{}
	r.Register("/home", func() Page { return page })
	r.Push("/home", nil)

	r.Push("/does-not-exist", nil)

	if got := r.CurrentPath(); got != "/home" {
		t.Errorf("CurrentPath() = %q after push of unregistered path, want %q", got, "/home")
	}
	if len(r.stack) != 1 {
		t.Errorf("stack len = %d, want 1 (unregistered push should be ignored)", len(r.stack))
	}
}

// TestPush_WithParams verifies that params are delivered to OnNavigatedTo.
func TestPush_WithParams(t *testing.T) {
	r := newTestRouter()
	page := &mockPage{}
	r.Register("/detail", func() Page { return page })

	params := map[string]string{"id": "42", "mode": "edit"}
	r.Push("/detail", params)

	if page.lastParams["id"] != "42" {
		t.Errorf("lastParams[id] = %q, want %q", page.lastParams["id"], "42")
	}
	if page.lastParams["mode"] != "edit" {
		t.Errorf("lastParams[mode] = %q, want %q", page.lastParams["mode"], "edit")
	}
}

// TestPush_CallsOnNavigatedFromOnPreviousPage verifies that the outgoing page is
// notified before the incoming page is activated.
func TestPush_CallsOnNavigatedFromOnPreviousPage(t *testing.T) {
	r := newTestRouter()

	first := &mockPage{}
	second := &mockPage{}

	r.Register("/first", func() Page { return first })
	r.Register("/second", func() Page { return second })

	r.Push("/first", nil)

	if first.navigatedFrom {
		t.Error("OnNavigatedFrom called on first page prematurely")
	}

	r.Push("/second", nil)

	if !first.navigatedFrom {
		t.Error("OnNavigatedFrom was not called on first page when second was pushed")
	}
	if !second.navigatedTo {
		t.Error("OnNavigatedTo was not called on second page")
	}
}

// TestPop_ReturnsToPreviousPage verifies that Pop restores the previous page and
// calls OnNavigatedTo on it.
func TestPop_ReturnsToPreviousPage(t *testing.T) {
	r := newTestRouter()

	first := &mockPage{}
	// second is created fresh via factory so we capture it through a closure.
	var second *mockPage

	r.Register("/first", func() Page { return first })
	r.Register("/second", func() Page {
		second = &mockPage{}
		return second
	})

	r.Push("/first", nil)
	r.Push("/second", nil)

	// Reset navigatedTo on first to distinguish the pop-restore call.
	first.navigatedTo = false

	r.Pop()

	if got := r.CurrentPath(); got != "/first" {
		t.Errorf("CurrentPath() = %q after Pop, want %q", got, "/first")
	}
	if !first.navigatedTo {
		t.Error("OnNavigatedTo was not called on first page after Pop")
	}
	if !second.navigatedFrom {
		t.Error("OnNavigatedFrom was not called on second page during Pop")
	}
}

// TestPop_SinglePage_NoOp verifies that Pop on a single-item stack is a no-op.
func TestPop_SinglePage_NoOp(t *testing.T) {
	r := newTestRouter()
	page := &mockPage{}
	r.Register("/only", func() Page { return page })
	r.Push("/only", nil)

	r.Pop()

	if got := r.CurrentPath(); got != "/only" {
		t.Errorf("CurrentPath() = %q after Pop on single page, want %q", got, "/only")
	}
	if len(r.stack) != 1 {
		t.Errorf("stack len = %d after Pop on single page, want 1", len(r.stack))
	}
}

// TestPop_EmptyStack_NoOp verifies that Pop on an empty stack does not panic.
func TestPop_EmptyStack_NoOp(t *testing.T) {
	r := newTestRouter()

	// Must not panic.
	r.Pop()

	if got := r.CurrentPath(); got != "" {
		t.Errorf("CurrentPath() = %q after Pop on empty stack, want empty", got)
	}
}

// TestReplace_ReplacesCurrent verifies that Replace swaps the current page without
// growing the stack.
func TestReplace_ReplacesCurrent(t *testing.T) {
	r := newTestRouter()

	first := &mockPage{}
	var replaced *mockPage

	r.Register("/first", func() Page { return first })
	r.Register("/replaced", func() Page {
		replaced = &mockPage{}
		return replaced
	})

	r.Push("/first", nil)
	depthAfterPush := len(r.stack)

	r.Replace("/replaced", nil)

	if len(r.stack) != depthAfterPush {
		t.Errorf("stack len = %d after Replace, want %d (stack depth should not change)",
			len(r.stack), depthAfterPush)
	}
	if got := r.CurrentPath(); got != "/replaced" {
		t.Errorf("CurrentPath() = %q after Replace, want %q", got, "/replaced")
	}
	if !first.navigatedFrom {
		t.Error("OnNavigatedFrom was not called on replaced page")
	}
	if !replaced.navigatedTo {
		t.Error("OnNavigatedTo was not called on replacement page")
	}
}

// TestNavigationSequence_PushAndPop verifies a multi-step A -> B -> C -> pop -> pop
// sequence ends on A with correct CurrentPath at each step.
func TestNavigationSequence_PushAndPop(t *testing.T) {
	r := newTestRouter()

	var pageA, pageB, pageC *mockPage

	r.Register("/a", func() Page { pageA = &mockPage{}; return pageA })
	r.Register("/b", func() Page { pageB = &mockPage{}; return pageB })
	r.Register("/c", func() Page { pageC = &mockPage{}; return pageC })

	r.Push("/a", nil)
	if got := r.CurrentPath(); got != "/a" {
		t.Errorf("step 1: CurrentPath() = %q, want %q", got, "/a")
	}

	r.Push("/b", nil)
	if got := r.CurrentPath(); got != "/b" {
		t.Errorf("step 2: CurrentPath() = %q, want %q", got, "/b")
	}

	r.Push("/c", nil)
	if got := r.CurrentPath(); got != "/c" {
		t.Errorf("step 3: CurrentPath() = %q, want %q", got, "/c")
	}
	if len(r.stack) != 3 {
		t.Errorf("stack len = %d after 3 pushes, want 3", len(r.stack))
	}

	// Silence unused-variable warnings: all pages were created by the factories.
	_ = pageA
	_ = pageB
	_ = pageC

	r.Pop()
	if got := r.CurrentPath(); got != "/b" {
		t.Errorf("after first pop: CurrentPath() = %q, want %q", got, "/b")
	}

	r.Pop()
	if got := r.CurrentPath(); got != "/a" {
		t.Errorf("after second pop: CurrentPath() = %q, want %q", got, "/a")
	}
	if len(r.stack) != 1 {
		t.Errorf("stack len = %d after popping to root, want 1", len(r.stack))
	}
}

// TestRegister_OverwritesFactory verifies that re-registering a path replaces the factory.
func TestRegister_OverwritesFactory(t *testing.T) {
	r := newTestRouter()

	callCount := 0
	r.Register("/page", func() Page { callCount++; return &mockPage{} })
	r.Register("/page", func() Page { callCount += 10; return &mockPage{} }) // overwrite

	r.Push("/page", nil)

	if callCount != 10 {
		t.Errorf("factory call signature: callCount = %d, want 10 (second factory should be used)", callCount)
	}
}
