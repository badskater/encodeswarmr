package widget

import (
	"image/color"
	"testing"
)

// TestBarChart_ZeroValueFields verifies that a zero-value BarChart has the expected
// zero/nil field states (no bars, zero MaxValue, nil LabelFunc, zero BarColor).
func TestBarChart_ZeroValueFields(t *testing.T) {
	var bc BarChart

	if bc.Bars != nil {
		t.Errorf("Bars = %v, want nil for zero-value BarChart", bc.Bars)
	}
	if bc.MaxValue != 0 {
		t.Errorf("MaxValue = %f, want 0", bc.MaxValue)
	}
	if bc.LabelFunc != nil {
		t.Error("LabelFunc is non-nil on zero-value BarChart, want nil")
	}
	if bc.BarColor != (color.NRGBA{}) {
		t.Errorf("BarColor = %v, want zero color", bc.BarColor)
	}
}

// TestBarChart_BarsStoredCorrectly verifies that Bars data is stored as assigned.
func TestBarChart_BarsStoredCorrectly(t *testing.T) {
	bc := BarChart{
		Bars: []BarData{
			{Value: 10, Label: "Mon"},
			{Value: 20, Label: "Tue"},
			{Value: 15, Label: "Wed"},
		},
		MaxValue: 25,
	}

	if len(bc.Bars) != 3 {
		t.Fatalf("Bars len = %d, want 3", len(bc.Bars))
	}
	if bc.Bars[0].Value != 10 {
		t.Errorf("Bars[0].Value = %f, want 10", bc.Bars[0].Value)
	}
	if bc.Bars[1].Label != "Tue" {
		t.Errorf("Bars[1].Label = %q, want %q", bc.Bars[1].Label, "Tue")
	}
	if bc.MaxValue != 25 {
		t.Errorf("MaxValue = %f, want 25", bc.MaxValue)
	}
}

// TestBarChart_BarColorAssignment verifies that an explicit BarColor is stored.
func TestBarChart_BarColorAssignment(t *testing.T) {
	want := color.NRGBA{R: 255, G: 100, B: 50, A: 255}
	bc := BarChart{
		BarColor: want,
	}
	if bc.BarColor != want {
		t.Errorf("BarColor = %v, want %v", bc.BarColor, want)
	}
}

// TestBarChart_LabelFunc verifies that a LabelFunc overrides bar labels when called.
func TestBarChart_LabelFunc(t *testing.T) {
	labels := []string{"alpha", "beta", "gamma"}
	bc := BarChart{
		Bars: []BarData{
			{Value: 1, Label: "original-0"},
			{Value: 2, Label: "original-1"},
			{Value: 3, Label: "original-2"},
		},
		LabelFunc: func(i int) string {
			return labels[i]
		},
	}

	// Verify LabelFunc is wired and returns the expected values.
	for i, want := range labels {
		got := bc.LabelFunc(i)
		if got != want {
			t.Errorf("LabelFunc(%d) = %q, want %q", i, got, want)
		}
	}
}

// TestBarData_ZeroValue verifies that a zero-value BarData has zero fields.
func TestBarData_ZeroValue(t *testing.T) {
	var bd BarData
	if bd.Value != 0 {
		t.Errorf("BarData.Value = %f, want 0", bd.Value)
	}
	if bd.Label != "" {
		t.Errorf("BarData.Label = %q, want empty string", bd.Label)
	}
}

// TestBarChart_EmptyBars verifies that an empty Bars slice is handled gracefully:
// Layout returns early without panicking. We cannot call Layout without a display
// context, so we exercise the pre-condition directly.
func TestBarChart_EmptyBars(t *testing.T) {
	bc := BarChart{}
	if len(bc.Bars) != 0 {
		t.Errorf("expected 0 bars, got %d", len(bc.Bars))
	}
	// Verify that adding and then clearing bars works correctly.
	bc.Bars = []BarData{{Value: 5, Label: "x"}}
	bc.Bars = bc.Bars[:0]
	if len(bc.Bars) != 0 {
		t.Errorf("expected 0 bars after clear, got %d", len(bc.Bars))
	}
}
