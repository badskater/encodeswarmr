package page

import (
	"context"
	"fmt"
	"image"
	"log/slog"
	"path"
	"strings"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	desktopapp "github.com/badskater/encodeswarmr/internal/desktop/app"
	"github.com/badskater/encodeswarmr/internal/desktop/client"
	"github.com/badskater/encodeswarmr/internal/desktop/nav"
)

// FileManagerPage provides a tree-style file browser.
type FileManagerPage struct {
	state  *desktopapp.State
	router *nav.Router
	window *app.Window
	logger *slog.Logger

	currentPath string
	entries     []client.FileEntry
	selectedIdx int
	fileInfo    *client.FileInfo
	loading     bool
	errorMsg    string

	pathEditor widget.Editor
	browseBtn  widget.Clickable
	parentBtn  widget.Clickable
	refreshBtn widget.Clickable
	moveBtns   []widget.Clickable
	deleteBtns []widget.Clickable
	entryBtns  []widget.Clickable
	list       widget.List

	// Move dialog state.
	showMove    bool
	moveSrc     string
	moveDstEdit widget.Editor
	moveSubmit  widget.Clickable
	moveCancel  widget.Clickable
	moveErr     string

	// Delete confirmation state.
	confirmDeletePath string
	confirmDeleteBtn  widget.Clickable
	cancelDeleteBtn   widget.Clickable
}

// NewFileManagerPage constructs a FileManagerPage.
func NewFileManagerPage(state *desktopapp.State, router *nav.Router, w *app.Window, logger *slog.Logger) *FileManagerPage {
	p := &FileManagerPage{
		state:       state,
		router:      router,
		window:      w,
		logger:      logger,
		currentPath: "/",
		selectedIdx: -1,
	}
	p.list.Axis = layout.Vertical
	p.pathEditor.SingleLine = true
	p.moveDstEdit.SingleLine = true
	return p
}

// OnNavigatedTo loads the root directory when the page becomes active.
func (p *FileManagerPage) OnNavigatedTo(params map[string]string) {
	if dir, ok := params["path"]; ok && dir != "" {
		p.currentPath = dir
	}
	p.pathEditor.SetText(p.currentPath)
	p.browse(p.currentPath)
}

// OnNavigatedFrom is a no-op.
func (p *FileManagerPage) OnNavigatedFrom() {}

func (p *FileManagerPage) browse(dir string) {
	if p.loading {
		return
	}
	c := p.state.Client()
	if c == nil {
		return
	}
	p.loading = true
	p.errorMsg = ""
	p.fileInfo = nil
	p.selectedIdx = -1

	go func() {
		resp, err := c.BrowseFiles(context.Background(), dir)
		if err != nil {
			p.errorMsg = "Browse failed: " + err.Error()
			p.logger.Error("file browse", "path", dir, "err", err)
		} else {
			p.currentPath = resp.Path
			p.entries = resp.Entries
			n := len(resp.Entries)
			p.entryBtns = make([]widget.Clickable, n)
			p.moveBtns = make([]widget.Clickable, n)
			p.deleteBtns = make([]widget.Clickable, n)
		}
		p.loading = false
		p.window.Invalidate()
	}()
}

func (p *FileManagerPage) loadFileInfo(entry client.FileEntry) {
	c := p.state.Client()
	if c == nil {
		return
	}

	go func() {
		info, err := c.GetFileInfo(context.Background(), entry.Path)
		if err != nil {
			p.logger.Error("file info", "path", entry.Path, "err", err)
			p.fileInfo = nil
		} else {
			p.fileInfo = info
		}
		p.window.Invalidate()
	}()
}

func (p *FileManagerPage) confirmDelete(filePath string) {
	p.confirmDeletePath = filePath
	p.window.Invalidate()
}

func (p *FileManagerPage) doDelete() {
	c := p.state.Client()
	if c == nil {
		return
	}
	filePath := p.confirmDeletePath
	p.confirmDeletePath = ""

	go func() {
		if err := c.DeleteFile(context.Background(), filePath); err != nil {
			p.errorMsg = "Delete failed: " + err.Error()
			p.logger.Error("file delete", "path", filePath, "err", err)
			p.window.Invalidate()
			return
		}
		p.browse(p.currentPath)
	}()
}

func (p *FileManagerPage) doMove() {
	c := p.state.Client()
	if c == nil {
		return
	}
	dst := strings.TrimSpace(p.moveDstEdit.Text())
	if dst == "" {
		p.moveErr = "Destination path is required"
		return
	}
	src := p.moveSrc
	p.showMove = false

	go func() {
		_, err := c.MoveFile(context.Background(), src, dst)
		if err != nil {
			p.errorMsg = "Move failed: " + err.Error()
			p.logger.Error("file move", "src", src, "dst", dst, "err", err)
			p.window.Invalidate()
			return
		}
		p.browse(p.currentPath)
	}()
}

// Layout renders the file manager page.
func (p *FileManagerPage) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	// Navigation controls.
	if p.browseBtn.Clicked(gtx) {
		dir := strings.TrimSpace(p.pathEditor.Text())
		if dir != "" {
			p.browse(dir)
		}
	}
	if p.parentBtn.Clicked(gtx) {
		parent := path.Dir(p.currentPath)
		if parent == p.currentPath {
			parent = "/"
		}
		p.pathEditor.SetText(parent)
		p.browse(parent)
	}
	if p.refreshBtn.Clicked(gtx) {
		p.browse(p.currentPath)
	}

	// Entry clicks.
	for i := range p.entryBtns {
		if i < len(p.entries) && p.entryBtns[i].Clicked(gtx) {
			entry := p.entries[i]
			if entry.IsDir {
				p.pathEditor.SetText(entry.Path)
				p.browse(entry.Path)
			} else {
				p.selectedIdx = i
				p.loadFileInfo(entry)
			}
		}
	}

	// Action buttons.
	for i := range p.moveBtns {
		if i < len(p.entries) && p.moveBtns[i].Clicked(gtx) {
			p.moveSrc = p.entries[i].Path
			p.moveDstEdit.SetText(p.entries[i].Path)
			p.moveErr = ""
			p.showMove = true
		}
	}
	for i := range p.deleteBtns {
		if i < len(p.entries) && p.deleteBtns[i].Clicked(gtx) {
			p.confirmDelete(p.entries[i].Path)
		}
	}

	// Move form.
	if p.moveSubmit.Clicked(gtx) {
		p.doMove()
	}
	if p.moveCancel.Clicked(gtx) {
		p.showMove = false
		p.moveErr = ""
	}

	// Delete confirmation.
	if p.confirmDeleteBtn.Clicked(gtx) {
		p.doDelete()
	}
	if p.cancelDeleteBtn.Clicked(gtx) {
		p.confirmDeletePath = ""
	}

	return layout.Inset{
		Left:   unit.Dp(24),
		Right:  unit.Dp(24),
		Top:    unit.Dp(16),
		Bottom: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.layoutHeader(gtx, th)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.layoutPathBar(gtx, th)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.errorMsg != "" {
					lbl := material.Body1(th, p.errorMsg)
					lbl.Color = desktopapp.ColorDanger
					return lbl.Layout(gtx)
				}
				return layout.Dimensions{}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.confirmDeletePath != "" {
					return p.layoutDeleteConfirm(gtx, th)
				}
				return layout.Dimensions{}
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.showMove {
					return p.layoutMoveForm(gtx, th)
				}
				return layout.Dimensions{}
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.layoutContent(gtx, th)
			}),
		)
	})
}

func (p *FileManagerPage) layoutHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.H5(th, "File Manager").Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := "Refresh"
			if p.loading {
				label = "Loading..."
			}
			btn := material.Button(th, &p.refreshBtn, label)
			btn.Background = desktopapp.ColorSecondary
			return btn.Layout(gtx)
		}),
	)
}

func (p *FileManagerPage) layoutPathBar(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(th, &p.parentBtn, "↑ Up")
			btn.Background = desktopapp.ColorSecondary
			return btn.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return widget.Border{
				Color:        desktopapp.ColorBorder,
				CornerRadius: unit.Dp(4),
				Width:        unit.Dp(1),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{
					Left:   unit.Dp(8),
					Right:  unit.Dp(8),
					Top:    unit.Dp(6),
					Bottom: unit.Dp(6),
				}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					ed := material.Editor(th, &p.pathEditor, "/path/to/browse")
					return ed.Layout(gtx)
				})
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := material.Button(th, &p.browseBtn, "Go")
			btn.Background = desktopapp.ColorPrimary
			return btn.Layout(gtx)
		}),
	)
}

func (p *FileManagerPage) layoutContent(gtx layout.Context, th *material.Theme) layout.Dimensions {
	hasSel := p.selectedIdx >= 0 && p.selectedIdx < len(p.entries) && !p.entries[p.selectedIdx].IsDir

	if !hasSel {
		return p.layoutFileList(gtx, th)
	}

	// Split: file list on the left, info panel on the right.
	return layout.Flex{}.Layout(gtx,
		layout.Flexed(0.6, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return p.layoutFileList(gtx, th)
			})
		}),
		layout.Flexed(0.4, func(gtx layout.Context) layout.Dimensions {
			return p.layoutInfoPanel(gtx, th)
		}),
	)
}

func (p *FileManagerPage) layoutFileList(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if p.loading && len(p.entries) == 0 {
		return material.Body1(th, "Loading...").Layout(gtx)
	}
	if len(p.entries) == 0 {
		lbl := material.Body1(th, "Directory is empty.")
		lbl.Color = desktopapp.ColorTextLight
		return lbl.Layout(gtx)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return p.layoutFileListHeader(gtx, th)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return material.List(th, &p.list).Layout(gtx, len(p.entries),
				func(gtx layout.Context, i int) layout.Dimensions {
					return p.layoutEntryRow(gtx, th, i)
				})
		}),
	)
}

// fileCols defines the file list columns.
var fileCols = []struct {
	title string
	flex  float32
}{
	{"Name", 0.40},
	{"Size", 0.12},
	{"Type", 0.10},
	{"Modified", 0.22},
	{"", 0.16},
}

func (p *FileManagerPage) layoutFileListHeader(gtx layout.Context, th *material.Theme) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(fileCols))
	for _, col := range fileCols {
		col := col
		children = append(children, layout.Flexed(col.flex, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Caption(th, col.title)
			lbl.Color = desktopapp.ColorTextLight
			return lbl.Layout(gtx)
		}))
	}
	return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{}.Layout(gtx, children...)
	})
}

func (p *FileManagerPage) layoutEntryRow(gtx layout.Context, th *material.Theme, i int) layout.Dimensions {
	entry := p.entries[i]
	isSelected := i == p.selectedIdx

	for len(p.entryBtns) <= i {
		p.entryBtns = append(p.entryBtns, widget.Clickable{})
		p.moveBtns = append(p.moveBtns, widget.Clickable{})
		p.deleteBtns = append(p.deleteBtns, widget.Clickable{})
	}

	return layout.Inset{Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return material.Clickable(gtx, &p.entryBtns[i], func(gtx layout.Context) layout.Dimensions {
			bg := desktopapp.ColorSurface
			if isSelected {
				bg = desktopapp.ColorPrimary
				bg.A = 60
			}
			bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
			rr := gtx.Dp(unit.Dp(4))
			paint.FillShape(gtx.Ops, bg,
				clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

			return layout.Inset{
				Top: unit.Dp(6), Bottom: unit.Dp(6),
				Left: unit.Dp(8), Right: unit.Dp(8),
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					// Name with dir/file indicator.
					layout.Flexed(fileCols[0].flex, func(gtx layout.Context) layout.Dimensions {
						prefix := "  "
						if entry.IsDir {
							prefix = "📁 "
						}
						lbl := material.Body2(th, prefix+entry.Name)
						lbl.MaxLines = 1
						return lbl.Layout(gtx)
					}),
					// Size.
					layout.Flexed(fileCols[1].flex, func(gtx layout.Context) layout.Dimensions {
						size := "-"
						if !entry.IsDir {
							size = formatBytes(entry.Size)
						}
						lbl := material.Body2(th, size)
						lbl.Color = desktopapp.ColorTextLight
						return lbl.Layout(gtx)
					}),
					// Type.
					layout.Flexed(fileCols[2].flex, func(gtx layout.Context) layout.Dimensions {
						typ := "file"
						if entry.IsDir {
							typ = "dir"
						}
						lbl := material.Caption(th, typ)
						lbl.Color = desktopapp.ColorTextLight
						return lbl.Layout(gtx)
					}),
					// Modified.
					layout.Flexed(fileCols[3].flex, func(gtx layout.Context) layout.Dimensions {
						lbl := material.Body2(th, entry.ModTime.Format("2006-01-02 15:04"))
						lbl.Color = desktopapp.ColorTextLight
						return lbl.Layout(gtx)
					}),
					// Action buttons.
					layout.Flexed(fileCols[4].flex, func(gtx layout.Context) layout.Dimensions {
						if entry.IsDir {
							return layout.Dimensions{}
						}
						return layout.Flex{}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								btn := material.Button(th, &p.moveBtns[i], "Mv")
								btn.Background = desktopapp.ColorSecondary
								return btn.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								btn := material.Button(th, &p.deleteBtns[i], "Del")
								btn.Background = desktopapp.ColorDanger
								return btn.Layout(gtx)
							}),
						)
					}),
				)
			})
		})
	})
}

func (p *FileManagerPage) layoutInfoPanel(gtx layout.Context, th *material.Theme) layout.Dimensions {
	bounds := image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	rr := gtx.Dp(unit.Dp(8))
	paint.FillShape(gtx.Ops, desktopapp.ColorSurface,
		clip.RRect{Rect: bounds, SE: rr, SW: rr, NE: rr, NW: rr}.Op(gtx.Ops))

	return layout.Inset{
		Left: unit.Dp(16), Right: unit.Dp(16),
		Top: unit.Dp(16), Bottom: unit.Dp(16),
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		if p.selectedIdx < 0 || p.selectedIdx >= len(p.entries) {
			return layout.Dimensions{}
		}
		entry := p.entries[p.selectedIdx]

		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.H6(th, "File Info").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.infoRow(gtx, th, "Name", entry.Name)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.infoRow(gtx, th, "Path", entry.Path)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.infoRow(gtx, th, "Size", formatBytes(entry.Size))
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.infoRow(gtx, th, "Extension", entry.Ext)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.infoRow(gtx, th, "Modified", entry.ModTime.Format("2006-01-02 15:04:05"))
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				isVideo := "No"
				if entry.IsVideo {
					isVideo = "Yes"
				}
				return p.infoRow(gtx, th, "Video", isVideo)
			}),
		)
	})
}

func (p *FileManagerPage) infoRow(gtx layout.Context, th *material.Theme, label, value string) layout.Dimensions {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, label+": ")
			lbl.Color = desktopapp.ColorTextLight
			return lbl.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			lbl := material.Body2(th, value)
			lbl.MaxLines = 1
			return lbl.Layout(gtx)
		}),
	)
}

func (p *FileManagerPage) layoutDeleteConfirm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				msg := fmt.Sprintf("Delete %q? This cannot be undone.", p.confirmDeletePath)
				lbl := material.Body1(th, msg)
				lbl.Color = desktopapp.ColorDanger
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.confirmDeleteBtn, "Delete")
						btn.Background = desktopapp.ColorDanger
						return btn.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.cancelDeleteBtn, "Cancel")
						btn.Background = desktopapp.ColorSecondary
						return btn.Layout(gtx)
					}),
				)
			}),
		)
	})
}

func (p *FileManagerPage) layoutMoveForm(gtx layout.Context, th *material.Theme) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.H6(th, fmt.Sprintf("Move: %s", p.moveSrc)).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						lbl := material.Caption(th, "Destination Path")
						lbl.Color = desktopapp.ColorTextLight
						return lbl.Layout(gtx)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return material.Editor(th, &p.moveDstEdit, "/new/path/file.ext").Layout(gtx)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if p.moveErr != "" {
					lbl := material.Body2(th, p.moveErr)
					lbl.Color = desktopapp.ColorDanger
					return lbl.Layout(gtx)
				}
				return layout.Dimensions{}
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.moveSubmit, "Move")
						btn.Background = desktopapp.ColorPrimary
						return btn.Layout(gtx)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						btn := material.Button(th, &p.moveCancel, "Cancel")
						btn.Background = desktopapp.ColorSecondary
						return btn.Layout(gtx)
					}),
				)
			}),
		)
	})
}

