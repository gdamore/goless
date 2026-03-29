package view

import (
	"fmt"
	"strings"

	"github.com/gdamore/goless/internal/ansi"
	"github.com/gdamore/goless/internal/layout"
	"github.com/gdamore/goless/internal/model"
	"github.com/gdamore/tcell/v3"
	tcolor "github.com/gdamore/tcell/v3/color"
)

// Config controls viewer behavior.
type Config struct {
	TabWidth   int
	WrapMode   layout.WrapMode
	ShowStatus bool
}

// Viewer is a minimal document viewer built on the model and layout packages.
type Viewer struct {
	doc       *model.Document
	cfg       Config
	lines     []model.Line
	layout    layout.Result
	width     int
	height    int
	rowOffset int
	colOffset int
}

// New constructs a viewer for the given document.
func New(doc *model.Document, cfg Config) *Viewer {
	if cfg.TabWidth <= 0 {
		cfg.TabWidth = 8
	}
	return &Viewer{
		doc: doc,
		cfg: cfg,
	}
}

// SetSize updates the viewport size.
func (v *Viewer) SetSize(width, height int) {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	v.width = width
	v.height = height
	v.relayout()
}

// Draw renders the current viewport.
func (v *Viewer) Draw(screen tcell.Screen) {
	v.ensureLayout()
	screen.Clear()

	bodyHeight := v.bodyHeight()
	for y := 0; y < bodyHeight; y++ {
		rowIndex := v.rowOffset + y
		if rowIndex >= len(v.layout.Rows) {
			break
		}
		v.drawRow(screen, y, v.layout.Rows[rowIndex])
	}

	if v.cfg.ShowStatus && v.height > 0 {
		v.drawStatus(screen, v.height-1)
	}

	screen.Show()
}

// HandleKey applies minimal navigation and returns true when the viewer should exit.
func (v *Viewer) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		return true
	case tcell.KeyUp:
		v.ScrollUp(1)
	case tcell.KeyDown:
		v.ScrollDown(1)
	case tcell.KeyLeft:
		v.ScrollLeft(1)
	case tcell.KeyRight:
		v.ScrollRight(1)
	case tcell.KeyPgUp:
		v.PageUp()
	case tcell.KeyPgDn:
		v.PageDown()
	case tcell.KeyHome:
		v.GoTop()
	case tcell.KeyEnd:
		v.GoBottom()
	case tcell.KeyRune:
		switch ev.Str() {
		case "q":
			return true
		case "j":
			v.ScrollDown(1)
		case "k":
			v.ScrollUp(1)
		case "h":
			v.ScrollLeft(1)
		case "l":
			v.ScrollRight(1)
		case " ", "f":
			v.PageDown()
		case "b":
			v.PageUp()
		case "g":
			v.GoTop()
		case "G":
			v.GoBottom()
		case "w":
			v.ToggleWrap()
		}
	}

	return false
}

// ToggleWrap switches between horizontal scrolling and soft wrap modes.
func (v *Viewer) ToggleWrap() {
	v.ensureLayout()
	anchor := v.firstVisibleAnchor()
	if v.cfg.WrapMode == layout.NoWrap {
		v.cfg.WrapMode = layout.SoftWrap
	} else {
		v.cfg.WrapMode = layout.NoWrap
	}
	v.relayout()
	v.restoreAnchor(anchor)
}

// ScrollDown moves the viewport down.
func (v *Viewer) ScrollDown(n int) {
	v.ensureLayout()
	v.rowOffset += max(n, 0)
	v.clampOffsets()
}

// ScrollUp moves the viewport up.
func (v *Viewer) ScrollUp(n int) {
	v.ensureLayout()
	v.rowOffset -= max(n, 0)
	v.clampOffsets()
}

// ScrollRight moves the viewport right in no-wrap mode.
func (v *Viewer) ScrollRight(n int) {
	if v.cfg.WrapMode != layout.NoWrap {
		return
	}
	v.colOffset += max(n, 0)
	v.relayout()
}

// ScrollLeft moves the viewport left in no-wrap mode.
func (v *Viewer) ScrollLeft(n int) {
	if v.cfg.WrapMode != layout.NoWrap {
		return
	}
	v.colOffset -= max(n, 0)
	v.relayout()
}

// PageDown moves the viewport down by roughly one page.
func (v *Viewer) PageDown() {
	step := max(v.bodyHeight()-1, 1)
	v.ScrollDown(step)
}

// PageUp moves the viewport up by roughly one page.
func (v *Viewer) PageUp() {
	step := max(v.bodyHeight()-1, 1)
	v.ScrollUp(step)
}

// GoTop moves the viewport to the beginning of the document.
func (v *Viewer) GoTop() {
	v.rowOffset = 0
	v.clampOffsets()
}

// GoBottom moves the viewport to the end of the document.
func (v *Viewer) GoBottom() {
	v.ensureLayout()
	v.rowOffset = v.maxRowOffset()
	v.clampOffsets()
}

func (v *Viewer) ensureLayout() {
	if v.layout.Rows == nil {
		v.relayout()
	}
}

func (v *Viewer) relayout() {
	v.lines = v.doc.Lines()
	v.layout = layout.Build(v.lines, layout.Config{
		Width:            max(v.width, 1),
		TabWidth:         v.cfg.TabWidth,
		WrapMode:         v.cfg.WrapMode,
		HorizontalOffset: v.horizontalOffset(),
	})
	v.clampOffsets()
}

func (v *Viewer) horizontalOffset() int {
	if v.cfg.WrapMode == layout.NoWrap {
		return max(v.colOffset, 0)
	}
	return 0
}

func (v *Viewer) clampOffsets() {
	if v.rowOffset < 0 {
		v.rowOffset = 0
	}
	if maxOffset := v.maxRowOffset(); v.rowOffset > maxOffset {
		v.rowOffset = maxOffset
	}
	if v.colOffset < 0 {
		v.colOffset = 0
	}
	if maxOffset := v.maxColOffset(); v.colOffset > maxOffset {
		v.colOffset = maxOffset
	}
}

func (v *Viewer) maxRowOffset() int {
	return max(len(v.layout.Rows)-v.bodyHeight(), 0)
}

func (v *Viewer) maxColOffset() int {
	if v.cfg.WrapMode != layout.NoWrap {
		return 0
	}

	maxCells := 0
	for _, line := range v.layout.Lines {
		if line.TotalCells > maxCells {
			maxCells = line.TotalCells
		}
	}
	return max(maxCells-max(v.width, 1), 0)
}

func (v *Viewer) bodyHeight() int {
	if v.height <= 0 {
		return 0
	}
	if v.cfg.ShowStatus && v.height > 1 {
		return v.height - 1
	}
	return v.height
}

func (v *Viewer) firstVisibleAnchor() layout.Anchor {
	if len(v.layout.Rows) == 0 {
		return layout.Anchor{}
	}
	if v.rowOffset < 0 || v.rowOffset >= len(v.layout.Rows) {
		return layout.Anchor{}
	}
	return v.layout.AnchorForRow(v.rowOffset)
}

func (v *Viewer) restoreAnchor(anchor layout.Anchor) {
	if rowIndex := v.layout.RowIndexForAnchor(anchor); rowIndex >= 0 {
		v.rowOffset = rowIndex
	}
	v.clampOffsets()
}

func (v *Viewer) drawRow(screen tcell.Screen, y int, row layout.VisualRow) {
	if row.LineIndex < 0 || row.LineIndex >= len(v.lines) {
		return
	}
	line := v.lines[row.LineIndex]
	for _, segment := range row.Segments {
		if segment.LogicalGraphemeIndex < 0 || segment.LogicalGraphemeIndex >= len(line.Graphemes) {
			continue
		}
		grapheme := line.Graphemes[segment.LogicalGraphemeIndex]
		ansiStyle := styleForGrapheme(line, grapheme.RuneStart)
		cellStyle := toTCellStyle(ansiStyle)
		x := segment.RenderedCellFrom
		if x >= v.width {
			break
		}

		if segment.Display != "" {
			screen.PutStrStyled(x, y, segment.Display, cellStyle)
			continue
		}

		if grapheme.Text == "\t" {
			screen.PutStrStyled(x, y, strings.Repeat(" ", segment.RenderedCellTo-segment.RenderedCellFrom), cellStyle)
			continue
		}

		screen.Put(x, y, grapheme.Text, cellStyle)
	}
}

func (v *Viewer) drawStatus(screen tcell.Screen, y int) {
	style := tcell.StyleDefault.Reverse(true)
	mode := "SCROLL"
	if v.cfg.WrapMode == layout.SoftWrap {
		mode = "WRAP"
	}

	current := 0
	if len(v.layout.Rows) > 0 {
		current = min(v.rowOffset+1, len(v.layout.Rows))
	}
	status := fmt.Sprintf(" %s  row %d/%d  col %d  bytes %d  q quit  w wrap ",
		mode, current, len(v.layout.Rows), v.colOffset, v.doc.Len())
	if len(status) < v.width {
		status += strings.Repeat(" ", v.width-len(status))
	}
	screen.PutStrStyled(0, y, truncateToWidth(status, v.width), style)
}

func styleForGrapheme(line model.Line, runeIndex int) ansi.Style {
	for _, run := range line.Styles {
		if runeIndex >= run.Start && runeIndex < run.End {
			return run.Style
		}
	}
	return ansi.DefaultStyle()
}

func toTCellStyle(style ansi.Style) tcell.Style {
	tstyle := tcell.StyleDefault
	tstyle = tstyle.Foreground(toTCellColor(style.Fg))
	tstyle = tstyle.Background(toTCellColor(style.Bg))
	tstyle = tstyle.Bold(style.Bold)
	tstyle = tstyle.Dim(style.Dim)
	tstyle = tstyle.Italic(style.Italic)
	tstyle = tstyle.Underline(style.Underline)
	tstyle = tstyle.Blink(style.Blink)
	tstyle = tstyle.Reverse(style.Reverse)
	return tstyle
}

func toTCellColor(c ansi.Color) tcolor.Color {
	switch c.Kind {
	case ansi.ColorIndex:
		return tcolor.PaletteColor(int(c.Index))
	case ansi.ColorRGB:
		return tcolor.NewRGBColor(int32(c.R), int32(c.G), int32(c.B))
	default:
		return tcolor.Default
	}
}

func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(s) <= width {
		return s
	}
	return s[:width]
}
