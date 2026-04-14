// Package widget provides reusable UI components for the desktop manager.
package widget

import (
	"fmt"
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"image/color"
)

// Column defines a single column in a Table.
type Column struct {
	// Title is displayed in the header row.
	Title string
	// Width is the fractional share of available width (0–1). Zero means auto/equal share.
	Width float32
	// Sortable marks the column as clickable for sorting.
	Sortable bool
}

// CellFunc renders one cell. row and col are zero-based data indices.
type CellFunc func(gtx layout.Context, th *material.Theme, row, col int) layout.Dimensions

// RowClickFunc is invoked when the user clicks a data row.
type RowClickFunc func(row int)

// Table is a paginated, sortable data table widget.
type Table struct {
	// Public configuration — set before first Layout call.
	Columns      []Column
	RowCount     int   // number of rows in the current page
	TotalCount   int64 // total rows across all pages
	PageSize     int
	CurrentPage  int
	RenderCell   CellFunc
	OnRowClick   RowClickFunc
	OnPageChange func(page int)

	// Sort state — set by the caller after receiving OnSort.
	SortColumn int
	SortAsc    bool
	OnSort     func(col int, asc bool)

	// Internal widget state.
	headerBtns []widget.Clickable
	rowBtns    []widget.Clickable
	prevBtn    widget.Clickable
	nextBtn    widget.Clickable
	list       widget.List
}

// NewTable creates a Table with the given column definitions and a default page
// size of 25. SortColumn is initialised to -1 (no sort).
func NewTable(columns []Column) *Table {
	t := &Table{
		Columns:    columns,
		PageSize:   25,
		SortColumn: -1,
	}
	t.headerBtns = make([]widget.Clickable, len(columns))
	t.list.Axis = layout.Vertical
	return t
}

// SetRowCount updates the number of data rows on the current page and
// re-allocates the per-row clickable buttons when needed.
func (t *Table) SetRowCount(count int) {
	t.RowCount = count
	if len(t.rowBtns) < count {
		t.rowBtns = make([]widget.Clickable, count)
	}
}

// Layout renders the full table widget: card background, header, rows, and
// pagination footer.
func (t *Table) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	// Process header sort clicks first so they are reflected this frame.
	for i := range t.headerBtns {
		if t.Columns[i].Sortable && t.headerBtns[i].Clicked(gtx) {
			if t.OnSort != nil {
				asc := true
				if t.SortColumn == i {
					asc = !t.SortAsc
				}
				t.OnSort(i, asc)
			}
		}
	}

	// Process pagination clicks.
	if t.prevBtn.Clicked(gtx) && t.CurrentPage > 0 {
		if t.OnPageChange != nil {
			t.OnPageChange(t.CurrentPage - 1)
		}
	}
	totalPages := t.totalPages()
	if t.nextBtn.Clicked(gtx) && t.CurrentPage < totalPages-1 {
		if t.OnPageChange != nil {
			t.OnPageChange(t.CurrentPage + 1)
		}
	}

	// Process row clicks.
	for i := 0; i < t.RowCount && i < len(t.rowBtns); i++ {
		if t.rowBtns[i].Clicked(gtx) && t.OnRowClick != nil {
			t.OnRowClick(i)
		}
	}

	// Card background.
	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	rr := gtx.Dp(unit.Dp(8))
	paint.FillShape(gtx.Ops, colorTableCard,
		clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return t.layoutHeader(gtx, th)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return t.layoutRows(gtx, th)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return t.layoutFooter(gtx, th)
		}),
	)
}

// layoutHeader draws the column title row with optional sort indicators.
func (t *Table) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	headerH := gtx.Dp(unit.Dp(40))
	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, headerH)
	rr := gtx.Dp(unit.Dp(8))
	paint.FillShape(gtx.Ops, colorTableHeader,
		clip.RRect{Rect: bounds, NE: rr, NW: rr}.Op(gtx.Ops))

	gtx.Constraints = layout.Exact(image.Point{X: gtx.Constraints.Max.X, Y: headerH})

	return layout.Inset{
		Left:  unit.Dp(12),
		Right: unit.Dp(12),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, len(t.Columns))
		widths := t.columnWidths()

		for i := range t.Columns {
			i := i
			weight := widths[i]
			children[i] = layout.Flexed(weight, func(gtx layout.Context) layout.Dimensions {
				return t.layoutHeaderCell(gtx, th, i)
			})
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
	})
}

func (t *Table) layoutHeaderCell(gtx layout.Context, th *material.Theme, col int) layout.Dimensions {
	c := t.Columns[col]
	title := c.Title
	if c.Sortable && t.SortColumn == col {
		if t.SortAsc {
			title += " ↑"
		} else {
			title += " ↓"
		}
	}

	inner := func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Top:    unit.Dp(10),
			Bottom: unit.Dp(10),
			Right:  unit.Dp(8),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, title)
			lbl.Color = colorHeaderText
			lbl.MaxLines = 1
			return lbl.Layout(gtx)
		})
	}

	if c.Sortable {
		return material.Clickable(gtx, &t.headerBtns[col], inner)
	}
	return inner(gtx)
}

// layoutRows renders the scrollable body of data rows.
func (t *Table) layoutRows(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if t.RowCount == 0 {
		return layout.Inset{
			Left:   unit.Dp(16),
			Right:  unit.Dp(16),
			Top:    unit.Dp(24),
			Bottom: unit.Dp(24),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, "No data")
			lbl.Color = colorMuted
			return lbl.Layout(gtx)
		})
	}

	return material.List(th, &t.list).Layout(gtx, t.RowCount,
		func(gtx layout.Context, rowIdx int) layout.Dimensions {
			return t.layoutRow(gtx, th, rowIdx)
		},
	)
}

func (t *Table) layoutRow(gtx layout.Context, th *material.Theme, rowIdx int) layout.Dimensions {
	rowH := gtx.Dp(unit.Dp(44))

	// Alternating row background.
	var bg color.NRGBA
	if rowIdx%2 == 0 {
		bg = colorRowEven
	} else {
		bg = colorRowOdd
	}
	rowBounds := image.Rect(0, 0, gtx.Constraints.Max.X, rowH)
	paint.FillShape(gtx.Ops, bg, clip.Rect(rowBounds).Op())

	rowGtx := gtx
	rowGtx.Constraints = layout.Exact(image.Point{X: gtx.Constraints.Max.X, Y: rowH})

	inner := func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{
			Left:  unit.Dp(12),
			Right: unit.Dp(12),
		}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, len(t.Columns))
			widths := t.columnWidths()

			for i := range t.Columns {
				i, rowIdx := i, rowIdx
				weight := widths[i]
				children[i] = layout.Flexed(weight, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{
						Top:    unit.Dp(10),
						Bottom: unit.Dp(10),
						Right:  unit.Dp(8),
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						if t.RenderCell != nil {
							return t.RenderCell(gtx, th, rowIdx, i)
						}
						return layout.Dimensions{}
					})
				})
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, children...)
		})
	}

	if rowIdx < len(t.rowBtns) {
		return material.Clickable(rowGtx, &t.rowBtns[rowIdx], inner)
	}
	return inner(rowGtx)
}

// layoutFooter renders the pagination controls.
func (t *Table) layoutFooter(gtx layout.Context, th *material.Theme) layout.Dimensions {
	footerH := gtx.Dp(unit.Dp(48))
	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, footerH)
	rr := gtx.Dp(unit.Dp(8))
	paint.FillShape(gtx.Ops, colorTableHeader,
		clip.RRect{Rect: bounds, SE: rr, SW: rr}.Op(gtx.Ops))

	gtx.Constraints = layout.Exact(image.Point{X: gtx.Constraints.Max.X, Y: footerH})

	return layout.Inset{
		Left:  unit.Dp(16),
		Right: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		totalPages := t.totalPages()
		start := t.CurrentPage*t.PageSize + 1
		end := start + t.RowCount - 1
		if t.RowCount == 0 {
			start = 0
			end = 0
		}
		info := fmt.Sprintf("%d–%d of %d", start, end, t.TotalCount)

		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, info)
				lbl.Color = colorMuted
				return lbl.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(th, &t.prevBtn, "←")
				btn.Background = colorBtnBackground
				btn.Color = colorBtnText
				if t.CurrentPage == 0 {
					btn.Color = colorMuted
				}
				return btn.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				page := fmt.Sprintf("%d / %d", t.CurrentPage+1, max(totalPages, 1))
				lbl := material.Caption(th, page)
				lbl.Color = colorMuted
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := material.Button(th, &t.nextBtn, "→")
				btn.Background = colorBtnBackground
				btn.Color = colorBtnText
				if t.CurrentPage >= totalPages-1 {
					btn.Color = colorMuted
				}
				return btn.Layout(gtx)
			}),
		)
	})
}

// columnWidths returns a normalised weight for each column. Columns with an
// explicit Width > 0 use that value; remaining space is split equally among
// auto-width columns.
func (t *Table) columnWidths() []float32 {
	if len(t.Columns) == 0 {
		return nil
	}
	widths := make([]float32, len(t.Columns))
	autoCount := 0
	fixed := float32(0)
	for i, c := range t.Columns {
		if c.Width > 0 {
			widths[i] = c.Width
			fixed += c.Width
		} else {
			autoCount++
		}
	}
	if autoCount > 0 {
		remaining := float32(1) - fixed
		if remaining < 0 {
			remaining = 0
		}
		share := remaining / float32(autoCount)
		for i, c := range t.Columns {
			if c.Width == 0 {
				widths[i] = share
			}
		}
	}
	return widths
}

func (t *Table) totalPages() int {
	if t.PageSize <= 0 {
		return 1
	}
	pages := int(t.TotalCount) / t.PageSize
	if int(t.TotalCount)%t.PageSize != 0 {
		pages++
	}
	if pages == 0 {
		pages = 1
	}
	return pages
}

// max returns the larger of a and b.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Internal colour constants for the table widget.
var (
	colorTableCard   = color.NRGBA{R: 255, G: 255, B: 255, A: 255}       // white card
	colorTableHeader = color.NRGBA{R: 243, G: 244, B: 246, A: 255}       // gray-100
	colorHeaderText  = color.NRGBA{R: 55, G: 65, B: 81, A: 255}          // gray-700
	colorRowEven     = color.NRGBA{R: 255, G: 255, B: 255, A: 255}       // white
	colorRowOdd      = color.NRGBA{R: 249, G: 250, B: 251, A: 255}       // gray-50
	colorMuted       = color.NRGBA{R: 107, G: 114, B: 128, A: 255}       // gray-500
	colorBtnBackground = color.NRGBA{R: 229, G: 231, B: 235, A: 255}     // gray-200
	colorBtnText       = color.NRGBA{R: 55, G: 65, B: 81, A: 255}        // gray-700
)
