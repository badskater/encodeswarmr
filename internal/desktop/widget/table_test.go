package widget

import (
	"testing"
)

// TestNewTable verifies default field values after construction.
func TestNewTable_Defaults(t *testing.T) {
	cols := []Column{
		{Title: "Name"},
		{Title: "Status"},
	}
	tbl := NewTable(cols)

	if len(tbl.Columns) != 2 {
		t.Errorf("Columns len = %d, want 2", len(tbl.Columns))
	}
	if tbl.Columns[0].Title != "Name" {
		t.Errorf("Columns[0].Title = %q, want %q", tbl.Columns[0].Title, "Name")
	}
	if tbl.PageSize != 25 {
		t.Errorf("PageSize = %d, want 25", tbl.PageSize)
	}
	if tbl.SortColumn != -1 {
		t.Errorf("SortColumn = %d, want -1", tbl.SortColumn)
	}
}

// TestNewTable_Empty verifies construction with no columns.
func TestNewTable_Empty(t *testing.T) {
	tbl := NewTable(nil)
	if tbl.Columns != nil {
		t.Errorf("Columns = %v, want nil", tbl.Columns)
	}
	if tbl.PageSize != 25 {
		t.Errorf("PageSize = %d, want 25", tbl.PageSize)
	}
	if tbl.SortColumn != -1 {
		t.Errorf("SortColumn = %d, want -1", tbl.SortColumn)
	}
}

// TestSetRowCount_UpdatesRowCount verifies that RowCount is updated correctly.
func TestSetRowCount_UpdatesRowCount(t *testing.T) {
	tbl := NewTable([]Column{{Title: "A"}})
	tbl.SetRowCount(10)
	if tbl.RowCount != 10 {
		t.Errorf("RowCount = %d, want 10", tbl.RowCount)
	}
}

// TestSetRowCount_AllocatesRowBtns verifies that rowBtns grows when count increases.
func TestSetRowCount_AllocatesRowBtns(t *testing.T) {
	tbl := NewTable([]Column{{Title: "A"}})

	tbl.SetRowCount(5)
	if len(tbl.rowBtns) < 5 {
		t.Errorf("rowBtns len = %d, want >= 5", len(tbl.rowBtns))
	}

	// Growing: rowBtns should grow to accommodate the larger count.
	tbl.SetRowCount(20)
	if len(tbl.rowBtns) < 20 {
		t.Errorf("rowBtns len = %d, want >= 20 after grow", len(tbl.rowBtns))
	}
}

// TestSetRowCount_DoesNotShrink verifies that rowBtns is not reallocated smaller.
func TestSetRowCount_DoesNotShrink(t *testing.T) {
	tbl := NewTable([]Column{{Title: "A"}})
	tbl.SetRowCount(20)
	lenAfterGrow := len(tbl.rowBtns)

	tbl.SetRowCount(5)
	if len(tbl.rowBtns) != lenAfterGrow {
		t.Errorf("rowBtns len = %d after shrink, want %d (should not shrink)", len(tbl.rowBtns), lenAfterGrow)
	}
	if tbl.RowCount != 5 {
		t.Errorf("RowCount = %d, want 5", tbl.RowCount)
	}
}

// TestColumnWidths_AllAuto verifies that auto-width columns share space equally.
func TestColumnWidths_AllAuto(t *testing.T) {
	tbl := NewTable([]Column{
		{Title: "A"},
		{Title: "B"},
		{Title: "C"},
	})
	widths := tbl.columnWidths()

	if len(widths) != 3 {
		t.Fatalf("widths len = %d, want 3", len(widths))
	}

	const wantEach = float32(1.0) / 3.0
	var total float32
	for i, w := range widths {
		total += w
		if w < wantEach-0.001 || w > wantEach+0.001 {
			t.Errorf("widths[%d] = %.4f, want ~%.4f", i, w, wantEach)
		}
	}
	if total < 0.999 || total > 1.001 {
		t.Errorf("total widths = %.4f, want ~1.0", total)
	}
}

// TestColumnWidths_MixedFixedAndAuto verifies that fixed columns use their explicit
// width and auto columns share the remaining space.
func TestColumnWidths_MixedFixedAndAuto(t *testing.T) {
	tbl := NewTable([]Column{
		{Title: "Fixed", Width: 0.3},
		{Title: "Auto1"},
		{Title: "Auto2"},
	})
	widths := tbl.columnWidths()

	if len(widths) != 3 {
		t.Fatalf("widths len = %d, want 3", len(widths))
	}

	if widths[0] < 0.299 || widths[0] > 0.301 {
		t.Errorf("widths[0] (fixed) = %.4f, want 0.3", widths[0])
	}

	// Remaining = 0.7 split equally between 2 auto cols.
	wantAuto := float32(0.7) / 2.0
	for _, i := range []int{1, 2} {
		if widths[i] < wantAuto-0.001 || widths[i] > wantAuto+0.001 {
			t.Errorf("widths[%d] (auto) = %.4f, want ~%.4f", i, widths[i], wantAuto)
		}
	}
}

// TestColumnWidths_AllFixed verifies that all-fixed columns use their explicit widths.
func TestColumnWidths_AllFixed(t *testing.T) {
	tbl := NewTable([]Column{
		{Title: "A", Width: 0.5},
		{Title: "B", Width: 0.3},
		{Title: "C", Width: 0.2},
	})
	widths := tbl.columnWidths()

	expected := []float32{0.5, 0.3, 0.2}
	for i, want := range expected {
		if widths[i] < want-0.001 || widths[i] > want+0.001 {
			t.Errorf("widths[%d] = %.4f, want %.4f", i, widths[i], want)
		}
	}
}

// TestColumnWidths_SingleColumn verifies that a single column gets full width.
func TestColumnWidths_SingleColumn(t *testing.T) {
	tbl := NewTable([]Column{{Title: "Only"}})
	widths := tbl.columnWidths()

	if len(widths) != 1 {
		t.Fatalf("widths len = %d, want 1", len(widths))
	}
	if widths[0] < 0.999 || widths[0] > 1.001 {
		t.Errorf("widths[0] = %.4f, want 1.0", widths[0])
	}
}

// TestColumnWidths_Empty verifies that an empty column list returns nil.
func TestColumnWidths_Empty(t *testing.T) {
	tbl := NewTable(nil)
	widths := tbl.columnWidths()
	if widths != nil {
		t.Errorf("columnWidths() = %v, want nil for empty columns", widths)
	}
}

// TestTotalPages verifies pagination arithmetic across edge cases.
func TestTotalPages(t *testing.T) {
	cases := []struct {
		name       string
		totalCount int64
		pageSize   int
		want       int
	}{
		{"zero rows default page", 0, 25, 1},
		{"exact full page", 25, 25, 1},
		{"one over full page", 26, 25, 2},
		{"four full pages", 100, 25, 4},
		{"zero pageSize", 50, 0, 1},
		{"single row", 1, 25, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tbl := NewTable([]Column{{Title: "X"}})
			tbl.TotalCount = tc.totalCount
			tbl.PageSize = tc.pageSize
			got := tbl.totalPages()
			if got != tc.want {
				t.Errorf("totalPages() = %d, want %d (TotalCount=%d, PageSize=%d)",
					got, tc.want, tc.totalCount, tc.pageSize)
			}
		})
	}
}
