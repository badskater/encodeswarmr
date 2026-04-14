// LogViewer implements a scrollable, searchable log display with optional
// follow mode (auto-scroll to the latest line). It strips ANSI escape codes
// and colours each line according to its log level.
package widget

import (
	"image"
	"image/color"
	"regexp"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// LogLine represents a single log entry for display.
type LogLine struct {
	Level   string
	Stream  string
	Message string
	Time    string
}

// LogViewer is a scrollable log viewer with search and follow mode.
type LogViewer struct {
	lines      []LogLine
	filtered   []int // indices into lines that match filter
	list       widget.List
	searchBar  SearchBar
	followMode bool
	followBtn  widget.Clickable
	filterText string

	// ANSI strip regex
	ansiRegex *regexp.Regexp
}

// NewLogViewer creates a LogViewer with follow mode enabled by default.
func NewLogViewer() *LogViewer {
	lv := &LogViewer{
		followMode: true,
		ansiRegex:  regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`),
	}
	lv.list.Axis = layout.Vertical
	lv.searchBar = *NewSearchBar()
	lv.searchBar.OnSearch = func(q string) {
		lv.filterText = q
		lv.updateFilter()
	}
	return lv
}

// SetLines replaces all log lines and re-applies the current filter.
func (lv *LogViewer) SetLines(lines []LogLine) {
	lv.lines = lines
	lv.updateFilter()
}

// AppendLine appends a single log line and re-applies the current filter.
func (lv *LogViewer) AppendLine(line LogLine) {
	lv.lines = append(lv.lines, line)
	lv.updateFilter()
}

// StripANSI removes ANSI escape sequences from s.
func (lv *LogViewer) StripANSI(s string) string {
	return lv.ansiRegex.ReplaceAllString(s, "")
}

func (lv *LogViewer) updateFilter() {
	if lv.filterText == "" {
		lv.filtered = nil // show all
		return
	}
	query := strings.ToLower(lv.filterText)
	lv.filtered = make([]int, 0)
	for i, l := range lv.lines {
		if strings.Contains(strings.ToLower(l.Message), query) {
			lv.filtered = append(lv.filtered, i)
		}
	}
}

func (lv *LogViewer) lineCount() int {
	if lv.filtered != nil {
		return len(lv.filtered)
	}
	return len(lv.lines)
}

func (lv *LogViewer) getLine(i int) LogLine {
	if lv.filtered != nil {
		return lv.lines[lv.filtered[i]]
	}
	return lv.lines[i]
}

// logLevelColor returns the display color for a log level string.
func logLevelColor(level string) color.NRGBA {
	switch strings.ToLower(level) {
	case "error", "fatal":
		return color.NRGBA{R: 239, G: 68, B: 68, A: 255} // red-500
	case "warn", "warning":
		return color.NRGBA{R: 234, G: 179, B: 8, A: 255} // yellow-500
	case "debug", "trace":
		return color.NRGBA{R: 156, G: 163, B: 175, A: 255} // gray-400
	default: // info and anything else
		return color.NRGBA{R: 209, G: 213, B: 219, A: 255} // gray-300
	}
}

// Layout renders the log viewer: search bar, follow toggle, then the log list.
func (lv *LogViewer) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	// Handle follow mode toggle.
	if lv.followBtn.Clicked(gtx) {
		lv.followMode = !lv.followMode
	}

	// If follow mode is on, scroll the list to the last item before layout.
	n := lv.lineCount()
	if lv.followMode && n > 0 {
		lv.list.ScrollTo(n - 1)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		// Toolbar: search bar + follow button.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return lv.searchBar.Layout(gtx, th)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := "Follow: OFF"
					if lv.followMode {
						label = "Follow: ON"
					}
					btn := material.Button(th, &lv.followBtn, label)
					if lv.followMode {
						btn.Background = color.NRGBA{R: 59, G: 130, B: 246, A: 255} // blue-500
					} else {
						btn.Background = color.NRGBA{R: 107, G: 114, B: 128, A: 255} // gray-500
					}
					return btn.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		// Log line list.
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return lv.layoutLines(gtx, th)
		}),
	)
}

func (lv *LogViewer) layoutLines(gtx layout.Context, th *material.Theme) layout.Dimensions {
	n := lv.lineCount()
	if n == 0 {
		lbl := material.Body2(th, "No log entries.")
		lbl.Color = color.NRGBA{R: 156, G: 163, B: 175, A: 255}
		return lbl.Layout(gtx)
	}

	// Draw a dark background behind the log area.
	bgColor := color.NRGBA{R: 17, G: 24, B: 39, A: 255} // gray-900
	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	rr := gtx.Dp(unit.Dp(6))
	paint.FillShape(gtx.Ops, bgColor,
		clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

	return layout.Inset{
		Top:    unit.Dp(8),
		Bottom: unit.Dp(8),
		Left:   unit.Dp(8),
		Right:  unit.Dp(8),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.List(th, &lv.list).Layout(gtx, n,
			func(gtx layout.Context, i int) layout.Dimensions {
				return lv.layoutLine(gtx, th, lv.getLine(i))
			})
	})
}

func (lv *LogViewer) layoutLine(gtx layout.Context, th *material.Theme, line LogLine) layout.Dimensions {
	msg := lv.StripANSI(line.Message)
	levelColor := logLevelColor(line.Level)
	tsColor := color.NRGBA{R: 107, G: 114, B: 128, A: 255} // gray-500

	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Alignment: layout.Start}.Layout(gtx,
			// Timestamp.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Caption(th, line.Time)
				lbl.Color = tsColor
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			// Level badge.
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if line.Level == "" {
					return layout.Dimensions{}
				}
				lbl := material.Caption(th, strings.ToUpper(line.Level))
				lbl.Color = levelColor
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			// Message (monospace via Body2).
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(th, msg)
				lbl.Color = color.NRGBA{R: 229, G: 231, B: 235, A: 255} // gray-200
				return lbl.Layout(gtx)
			}),
		)
	})
}
