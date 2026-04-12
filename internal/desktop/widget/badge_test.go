package widget

import (
	"image/color"
	"testing"
)

// TestStatusColor verifies that each known status string maps to the correct colour.
func TestStatusColor_Idle(t *testing.T) {
	want := color.NRGBA{R: 34, G: 197, B: 94, A: 255} // green-500
	got := statusColor("idle")
	if got != want {
		t.Errorf("statusColor(%q) = %v, want %v", "idle", got, want)
	}
}

func TestStatusColor_Running(t *testing.T) {
	want := color.NRGBA{R: 59, G: 130, B: 246, A: 255} // blue-500
	got := statusColor("running")
	if got != want {
		t.Errorf("statusColor(%q) = %v, want %v", "running", got, want)
	}
}

func TestStatusColor_Failed(t *testing.T) {
	want := color.NRGBA{R: 239, G: 68, B: 68, A: 255} // red-500
	got := statusColor("failed")
	if got != want {
		t.Errorf("statusColor(%q) = %v, want %v", "failed", got, want)
	}
}

func TestStatusColor_Queued(t *testing.T) {
	want := color.NRGBA{R: 234, G: 179, B: 8, A: 255} // yellow-500
	got := statusColor("queued")
	if got != want {
		t.Errorf("statusColor(%q) = %v, want %v", "queued", got, want)
	}
}

func TestStatusColor_Completed(t *testing.T) {
	want := color.NRGBA{R: 16, G: 185, B: 129, A: 255} // emerald-500
	got := statusColor("completed")
	if got != want {
		t.Errorf("statusColor(%q) = %v, want %v", "completed", got, want)
	}
}

func TestStatusColor_Unknown(t *testing.T) {
	want := color.NRGBA{R: 107, G: 114, B: 128, A: 255} // gray-500 (muted)
	got := statusColor("unknown")
	if got != want {
		t.Errorf("statusColor(%q) = %v, want %v", "unknown", got, want)
	}
}

// TestStatusColor_Empty verifies that an empty string falls through to the default.
func TestStatusColor_EmptyString(t *testing.T) {
	want := color.NRGBA{R: 107, G: 114, B: 128, A: 255} // gray-500 (default/muted)
	got := statusColor("")
	if got != want {
		t.Errorf("statusColor(%q) = %v, want %v", "", got, want)
	}
}

// TestStatusColor_AllKnownStatuses exercises all explicitly matched statuses in a
// table-driven style and verifies they are not the default gray.
func TestStatusColor_AllKnownStatuses(t *testing.T) {
	gray := color.NRGBA{R: 107, G: 114, B: 128, A: 255}

	knownStatuses := []string{
		"idle", "running", "assigned",
		"offline", "failed", "cancelled",
		"draining", "waiting", "queued", "pending",
		"completed",
	}
	for _, status := range knownStatuses {
		got := statusColor(status)
		if got == gray {
			t.Errorf("statusColor(%q) returned default gray; expected a distinct colour", status)
		}
	}
}

// TestStatusColor_Assigned verifies that "assigned" maps to blue (same as "running").
func TestStatusColor_Assigned(t *testing.T) {
	want := color.NRGBA{R: 59, G: 130, B: 246, A: 255} // blue-500
	got := statusColor("assigned")
	if got != want {
		t.Errorf("statusColor(%q) = %v, want %v", "assigned", got, want)
	}
}

// TestStatusColor_Pending verifies that "pending" maps to yellow (same as "queued").
func TestStatusColor_Pending(t *testing.T) {
	want := color.NRGBA{R: 234, G: 179, B: 8, A: 255} // yellow-500
	got := statusColor("pending")
	if got != want {
		t.Errorf("statusColor(%q) = %v, want %v", "pending", got, want)
	}
}
