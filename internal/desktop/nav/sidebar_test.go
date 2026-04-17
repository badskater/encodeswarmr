package nav

import (
	"testing"

	"gioui.org/app"
)

// ---------------------------------------------------------------------------
// NewSidebar — structure verification (no rendering required)
// ---------------------------------------------------------------------------

func TestNewSidebar_GroupCount(t *testing.T) {
	t.Parallel()
	r := NewRouter(new(app.Window))
	s := NewSidebar(r)
	if len(s.groups) != 2 {
		t.Errorf("len(groups) = %d, want 2 (OPERATIONS + ADMINISTRATION)", len(s.groups))
	}
}

func TestNewSidebar_OperationsGroup(t *testing.T) {
	t.Parallel()
	r := NewRouter(new(app.Window))
	s := NewSidebar(r)
	ops := s.groups[0]
	if ops.Label != "OPERATIONS" {
		t.Errorf("groups[0].Label = %q, want OPERATIONS", ops.Label)
	}
	// Minimum expected items in the Operations group.
	expectedPaths := []string{
		"/dashboard", "/sources", "/jobs", "/queue",
		"/agents", "/audio", "/flows", "/files", "/sessions",
	}
	pathSet := make(map[string]bool)
	for _, item := range ops.Items {
		pathSet[item.Path] = true
	}
	for _, want := range expectedPaths {
		if !pathSet[want] {
			t.Errorf("Operations group missing item with path %q", want)
		}
	}
}

func TestNewSidebar_AdministrationGroup(t *testing.T) {
	t.Parallel()
	r := NewRouter(new(app.Window))
	s := NewSidebar(r)
	admin := s.groups[1]
	if admin.Label != "ADMINISTRATION" {
		t.Errorf("groups[1].Label = %q, want ADMINISTRATION", admin.Label)
	}
	expectedPaths := []string{
		"/admin/templates", "/admin/variables", "/admin/webhooks",
		"/admin/users", "/admin/api-keys", "/admin/agent-pools",
		"/admin/path-mappings", "/admin/tokens", "/admin/schedules",
		"/admin/plugins", "/admin/encoding-rules", "/admin/encoding-profiles",
		"/admin/watch-folders", "/admin/auto-scaling", "/admin/notifications",
		"/admin/audit-export",
	}
	pathSet := make(map[string]bool)
	for _, item := range admin.Items {
		pathSet[item.Path] = true
	}
	for _, want := range expectedPaths {
		if !pathSet[want] {
			t.Errorf("Administration group missing item with path %q", want)
		}
	}
}

func TestNewSidebar_AllItemsHaveNonEmptyLabels(t *testing.T) {
	t.Parallel()
	r := NewRouter(new(app.Window))
	s := NewSidebar(r)
	for gi, group := range s.groups {
		for ii, item := range group.Items {
			if item.Label == "" {
				t.Errorf("groups[%d].items[%d].Label is empty (path=%q)", gi, ii, item.Path)
			}
		}
	}
}

func TestNewSidebar_AllItemsHaveNonEmptyPaths(t *testing.T) {
	t.Parallel()
	r := NewRouter(new(app.Window))
	s := NewSidebar(r)
	for gi, group := range s.groups {
		for ii, item := range group.Items {
			if item.Path == "" {
				t.Errorf("groups[%d].items[%d].Path is empty (label=%q)", gi, ii, item.Label)
			}
		}
	}
}

func TestNewSidebar_WidthIsNonZero(t *testing.T) {
	t.Parallel()
	r := NewRouter(new(app.Window))
	s := NewSidebar(r)
	if s.width <= 0 {
		t.Errorf("width = %v, want > 0", s.width)
	}
}

func TestNewSidebar_RouterIsStored(t *testing.T) {
	t.Parallel()
	r := NewRouter(new(app.Window))
	s := NewSidebar(r)
	if s.router != r {
		t.Error("router field does not point to the router passed to NewSidebar")
	}
}

func TestNewSidebar_TotalItemCount(t *testing.T) {
	t.Parallel()
	r := NewRouter(new(app.Window))
	s := NewSidebar(r)
	total := 0
	for _, g := range s.groups {
		total += len(g.Items)
	}
	// 9 operations + 16 administration = 25 items.
	if total != 25 {
		t.Errorf("total sidebar items = %d, want 25", total)
	}
}

func TestSidebarItem_PathsAreUnique(t *testing.T) {
	t.Parallel()
	r := NewRouter(new(app.Window))
	s := NewSidebar(r)
	seen := make(map[string]bool)
	for _, g := range s.groups {
		for _, item := range g.Items {
			if seen[item.Path] {
				t.Errorf("duplicate sidebar path %q", item.Path)
			}
			seen[item.Path] = true
		}
	}
}
