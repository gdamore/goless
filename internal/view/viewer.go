// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

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
	mode      viewerMode
	prompt    *promptState
	message   string
	search    searchState
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

	if v.height > 0 {
		switch {
		case v.mode == modePrompt:
			v.drawPrompt(screen, v.height-1)
		case v.cfg.ShowStatus:
			v.drawStatus(screen, v.height-1)
		}
	}

	// Be conservative for now. Some terminals mishandle incremental redraws
	// involving grapheme clusters or fallback-rendered cells, especially after
	// vertical motion. Sync avoids leaving stale display artifacts behind.
	screen.Sync()
}

// HandleKey applies minimal navigation and returns true when the viewer should exit.
func (v *Viewer) HandleKey(ev *tcell.EventKey) bool {
	if v.mode == modePrompt {
		return v.handlePromptKey(ev)
	}

	switch actionForKey(ev) {
	case actionQuit:
		return true
	case actionScrollUp:
		v.ScrollUp(1)
	case actionScrollDown:
		v.ScrollDown(1)
	case actionScrollLeft:
		v.ScrollLeft(1)
	case actionScrollRight:
		v.ScrollRight(1)
	case actionPageUp:
		v.PageUp()
	case actionPageDown:
		v.PageDown()
	case actionGoTop:
		v.GoTop()
	case actionGoBottom:
		v.GoBottom()
	case actionToggleWrap:
		v.ToggleWrap()
	case actionPromptSearchForward:
		v.beginPrompt(promptSearchForward)
	case actionPromptSearchBackward:
		v.beginPrompt(promptSearchBackward)
	case actionPromptCommand:
		v.beginPrompt(promptCommand)
	case actionSearchNext:
		v.repeatSearch(v.search.Forward)
	case actionSearchPrev:
		v.repeatSearch(!v.search.Forward)
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
		if anchor.LineIndex >= 0 && anchor.LineIndex < len(v.layout.Lines) {
			starts := v.layout.Lines[anchor.LineIndex].GraphemeCellStarts
			if anchor.GraphemeIndex >= 0 && anchor.GraphemeIndex < len(starts) {
				v.colOffset = starts[anchor.GraphemeIndex]
			} else {
				v.colOffset = 0
			}
		}
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
	v.rebuildSearch()
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
	if (v.cfg.ShowStatus || v.mode == modePrompt) && v.height > 1 {
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

func (v *Viewer) revealAnchor(anchor layout.Anchor) {
	rowIndex := v.layout.RowIndexForAnchor(anchor)
	if rowIndex < 0 {
		return
	}

	bodyHeight := max(v.bodyHeight(), 1)
	switch {
	case rowIndex < v.rowOffset:
		v.rowOffset = rowIndex
	case rowIndex >= v.rowOffset+bodyHeight:
		v.rowOffset = rowIndex - bodyHeight + 1
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
		matched, current := v.graphemeMatched(line, row.LineIndex, grapheme)
		if matched {
			cellStyle = applyMatchCellStyle(cellStyle, current)
		}
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
	searchInfo := ""
	if v.search.Query != "" {
		position := 0
		if len(v.search.Matches) > 0 && v.search.Current >= 0 {
			position = v.search.Current + 1
		}
		searchInfo = fmt.Sprintf("  /%s %d/%d", v.search.Query, position, len(v.search.Matches))
	}
	status := fmt.Sprintf(" %s  row %d/%d  col %d  bytes %d%s  q quit  / ? search  w wrap ",
		mode, current, len(v.layout.Rows), v.colOffset, v.doc.Len(), searchInfo)
	if v.message != "" {
		status += "  " + v.message
	}
	if len(status) < v.width {
		status += strings.Repeat(" ", v.width-len(status))
	}
	screen.PutStrStyled(0, y, truncateToWidth(status, v.width), style)
}

func (v *Viewer) drawPrompt(screen tcell.Screen, y int) {
	style := tcell.StyleDefault.Reverse(true)
	prompt := ""
	if v.prompt != nil {
		prompt = " " + v.prompt.String()
	}
	if len(prompt) < v.width {
		prompt += strings.Repeat(" ", v.width-len(prompt))
	}
	screen.PutStrStyled(0, y, truncateToWidth(prompt, v.width), style)
}

func (v *Viewer) handlePromptKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		v.cancelPrompt()
		return false
	case tcell.KeyCtrlC:
		return true
	case tcell.KeyEnter:
		v.commitPrompt()
		return false
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if v.prompt != nil && len(v.prompt.buffer) > 0 {
			v.prompt.buffer = v.prompt.buffer[:len(v.prompt.buffer)-1]
		}
		return false
	case tcell.KeyCtrlU:
		if v.prompt != nil {
			v.prompt.buffer = v.prompt.buffer[:0]
		}
		return false
	case tcell.KeyRune:
		if v.prompt != nil {
			v.prompt.buffer = append(v.prompt.buffer, []rune(ev.Str())...)
		}
		return false
	}
	return false
}

func (v *Viewer) setMessage(msg string) {
	v.message = msg
}

func styleForGrapheme(line model.Line, runeIndex int) ansi.Style {
	for _, run := range line.Styles {
		if runeIndex >= run.Start && runeIndex < run.End {
			return run.Style
		}
	}
	return ansi.DefaultStyle()
}

func applyMatchCellStyle(style tcell.Style, current bool) tcell.Style {
	preset := inactiveMatchStyle
	if current {
		preset = currentMatchStyle
	}

	style = style.Foreground(preset.Fg)
	style = style.Background(preset.Bg)
	if preset.UnderlineStyle != tcell.UnderlineStyleNone {
		style = style.Underline(preset.UnderlineStyle, preset.UnderlineColor)
	}
	style = style.Bold(preset.Bold)
	return style
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

type matchStylePreset struct {
	Fg             tcolor.Color
	Bg             tcolor.Color
	UnderlineStyle tcell.UnderlineStyle
	UnderlineColor tcolor.Color
	Bold           bool
}

var (
	// These defaults use ANSI palette colors so they remain predictable on
	// terminals without reliable RGB support. They are kept as style pairs so
	// background changes always come with an explicit contrasting foreground.
	inactiveMatchStyle = matchStylePreset{
		Fg: tcolor.PaletteColor(0),
		Bg: tcolor.PaletteColor(6),
	}
	currentMatchStyle = matchStylePreset{
		Fg:             tcolor.PaletteColor(7),
		Bg:             tcolor.PaletteColor(4),
		UnderlineStyle: tcell.UnderlineStyleDouble,
		UnderlineColor: tcolor.PaletteColor(5),
		Bold:           true,
	}
)

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
