// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"strings"

	"github.com/gdamore/goless/internal/ansi"
	"github.com/gdamore/goless/internal/layout"
	"github.com/gdamore/goless/internal/model"
	"github.com/gdamore/tcell/v3"
	tcolor "github.com/gdamore/tcell/v3/color"
	"github.com/rivo/uniseg"
)

// Config controls viewer behavior.
type Config struct {
	TabWidth   int
	WrapMode   layout.WrapMode
	Chrome     Chrome
	ShowStatus bool
	Text       Text
}

// Viewer is a minimal document viewer built on the model and layout packages.
type Viewer struct {
	doc        *model.Document
	cfg        Config
	mode       viewerMode
	prompt     *promptState
	message    string
	search     searchState
	text       Text
	lines      []model.Line
	layout     layout.Result
	width      int
	height     int
	rowOffset  int
	colOffset  int
	helpOffset int
}

// New constructs a viewer for the given document.
func New(doc *model.Document, cfg Config) *Viewer {
	if cfg.TabWidth <= 0 {
		cfg.TabWidth = 8
	}
	cfg.Chrome = cfg.Chrome.withDefaults()
	cfg.Text = cfg.Text.withDefaults()
	return &Viewer{
		doc:  doc,
		cfg:  cfg,
		text: cfg.Text,
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
	v.clampHelpOffset()
}

// Refresh rebuilds the derived layout using the current document contents.
func (v *Viewer) Refresh() {
	v.relayout()
}

// Draw renders the current viewport.
func (v *Viewer) Draw(screen tcell.Screen) {
	v.ensureLayout()
	screen.Clear()

	if v.mode == modeHelp {
		v.drawFrame(screen, v.helpFrameTitle())
		v.drawHelp(screen)
		screen.Sync()
		return
	}

	v.drawFrame(screen, v.cfg.Chrome.Title)

	bodyX, bodyY, _, bodyHeight := v.contentRect()
	for y := 0; y < bodyHeight; y++ {
		rowIndex := v.rowOffset + y
		if rowIndex >= len(v.layout.Rows) {
			break
		}
		v.drawRow(screen, bodyX, bodyY+y, v.layout.Rows[rowIndex])
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
	if v.mode == modeHelp {
		return v.handleHelpKey(ev)
	}
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
	case actionToggleHelp:
		v.toggleHelp()
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
		Width:            max(v.contentWidth(), 1),
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
	return max(maxCells-max(v.contentWidth(), 1), 0)
}

func (v *Viewer) bodyHeight() int {
	_, _, _, h := v.contentRect()
	return h
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

func (v *Viewer) drawRow(screen tcell.Screen, baseX, y int, row layout.VisualRow) {
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
		if x >= v.contentWidth() {
			break
		}
		x += baseX

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

func (v *Viewer) drawFrame(screen tcell.Screen, title string) {
	frame := v.cfg.Chrome.Frame
	if !frame.enabled() || v.width <= 0 {
		return
	}
	if v.height-v.bottomBarRows() < 2 {
		return
	}

	topY := 0
	bottomY := v.height - v.bottomBarRows() - 1
	if bottomY <= topY {
		return
	}

	borderStyle := v.cfg.Chrome.BorderStyle
	titleStyle := v.cfg.Chrome.TitleStyle
	if v.width == 1 {
		screen.PutStrStyled(0, topY, fallback(frame.TopLeft, frame.Horizontal, frame.Vertical), borderStyle)
		if bottomY != topY {
			screen.PutStrStyled(0, bottomY, fallback(frame.BottomLeft, frame.Horizontal, frame.Vertical), borderStyle)
		}
		return
	}

	top := frameLine(v.width, frame.TopLeft, frame.Horizontal, frame.TopRight)
	bottom := frameLine(v.width, frame.BottomLeft, frame.Horizontal, frame.BottomRight)
	screen.PutStrStyled(0, topY, top, borderStyle)
	screen.PutStrStyled(0, bottomY, bottom, borderStyle)
	if label, x := frameTitleLabel(title, v.width); label != "" {
		screen.PutStrStyled(x, topY, label, titleStyle)
	}

	side := fallback(frame.Vertical, "│")
	for y := topY + 1; y < bottomY; y++ {
		screen.PutStrStyled(0, y, side, borderStyle)
		screen.PutStrStyled(v.width-1, y, side, borderStyle)
	}
}

func (v *Viewer) drawStatus(screen tcell.Screen, y int) {
	style := statusBarStyle
	screen.PutStrStyled(0, y, strings.Repeat(" ", max(v.width, 0)), style)

	leftOverflow, rightOverflow := v.statusOverflow()
	leftText, rightText := v.statusText()

	leftIndicatorWidth := 0
	if leftOverflow {
		leftIndicatorWidth = stringWidth(v.text.LeftOverflowIndicator)
		if leftIndicatorWidth > 0 && v.width > 1 {
			screen.PutStrStyled(1, y, truncateToWidth(v.text.LeftOverflowIndicator, v.width-1), style)
		}
	}

	rightIndicatorWidth := 0
	rightIndicatorStart := v.width - 1
	if rightOverflow {
		rightIndicatorWidth = stringWidth(v.text.RightOverflowIndicator)
		rightIndicatorStart = max(v.width-1-rightIndicatorWidth, 0)
		if rightIndicatorWidth > 0 && rightIndicatorStart < v.width-1 {
			screen.PutStrStyled(rightIndicatorStart, y, truncateToWidth(v.text.RightOverflowIndicator, v.width-1-rightIndicatorStart), style)
		}
	}

	leftStart := 2 + leftIndicatorWidth
	rightLimit := v.width - 1
	if rightOverflow {
		rightLimit = rightIndicatorStart - 1
	}
	if rightLimit < leftStart {
		rightLimit = leftStart
	}

	rightTextWidth := min(stringWidth(rightText), max(rightLimit-leftStart, 0))
	rightText = truncateToWidth(rightText, rightTextWidth)
	rightStart := rightLimit - stringWidth(rightText)
	if rightStart < leftStart {
		rightStart = leftStart
	}

	leftWidth := max(rightStart-leftStart-1, 0)
	if leftWidth > 0 {
		screen.PutStrStyled(leftStart, y, truncateToWidth(leftText, leftWidth), style)
	}
	if rightText != "" && rightStart < rightLimit {
		screen.PutStrStyled(rightStart, y, rightText, style)
	}
}

func (v *Viewer) contentRect() (x, y, width, height int) {
	width = v.width
	height = v.height - v.bottomBarRows()
	if height < 0 {
		height = 0
	}

	if v.mode == modeHelp && !v.cfg.Chrome.Frame.enabled() {
		y = 1
		height--
		if height < 0 {
			height = 0
		}
		return x, y, width, height
	}

	if v.cfg.Chrome.Frame.enabled() {
		x = 1
		y = 1
		width -= 2
		height -= 2
		if width < 0 {
			width = 0
		}
		if height < 0 {
			height = 0
		}
	}

	return x, y, width, height
}

func (v *Viewer) contentWidth() int {
	_, _, width, _ := v.contentRect()
	return width
}

func (v *Viewer) bottomBarRows() int {
	if v.mode == modeHelp {
		return 0
	}
	if v.height <= 0 {
		return 0
	}
	if (v.cfg.ShowStatus || v.mode == modePrompt) && v.height > 1 {
		return 1
	}
	return 0
}

func (v *Viewer) statusText() (left, right string) {
	searchInfo := ""
	if v.search.Query != "" {
		position := 0
		if len(v.search.Matches) > 0 && v.search.Current >= 0 {
			position = v.search.Current + 1
		}
		searchInfo = v.text.StatusSearchInfo(v.search.Query, position, len(v.search.Matches))
	}
	left = searchInfo
	if v.message != "" {
		if left != "" {
			left += "  "
		}
		left += v.message
	}

	current := 0
	if len(v.layout.Rows) > 0 {
		current = min(v.rowOffset+1, len(v.layout.Rows))
	}
	right = v.text.StatusPosition(current, len(v.layout.Rows), v.colOffset)
	return left, right
}

func (v *Viewer) statusOverflow() (left, right bool) {
	if v.cfg.WrapMode != layout.NoWrap {
		return false, false
	}

	return v.colOffset > 0, v.colOffset < v.maxColOffset()
}

func (v *Viewer) drawPrompt(screen tcell.Screen, y int) {
	style := tcell.StyleDefault.Reverse(true)
	prompt := ""
	if v.prompt != nil {
		prompt = " " + v.prompt.String()
	}
	screen.PutStrStyled(0, y, padRightToWidth(prompt, v.width), style)
}

func (v *Viewer) helpFrameTitle() string {
	title := v.text.HelpTitle
	if v.text.HelpClose != "" {
		if title != "" {
			title += "  "
		}
		title += v.text.HelpClose
	}
	return title
}

func (v *Viewer) handleHelpKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		v.toggleHelp()
		return false
	case tcell.KeyCtrlC:
		return true
	case tcell.KeyUp:
		v.helpOffset--
	case tcell.KeyDown:
		v.helpOffset++
	case tcell.KeyPgUp:
		v.helpOffset -= max(v.height-2, 1)
	case tcell.KeyPgDn:
		v.helpOffset += max(v.height-2, 1)
	case tcell.KeyHome:
		v.helpOffset = 0
	case tcell.KeyEnd:
		v.helpOffset = v.maxHelpOffset()
	case tcell.KeyF1:
		v.toggleHelp()
	case tcell.KeyRune:
		switch ev.Str() {
		case "q", "H":
			v.toggleHelp()
		}
	}
	v.clampHelpOffset()
	return false
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

func (v *Viewer) toggleHelp() {
	if v.mode == modeHelp {
		v.mode = modeNormal
		return
	}
	v.mode = modeHelp
	v.helpOffset = 0
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

	statusBarStyle = tcell.StyleDefault.Foreground(tcolor.PaletteColor(15)).Background(tcolor.PaletteColor(2))
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
	if stringWidth(s) <= width {
		return s
	}

	var (
		builder strings.Builder
		total   int
	)
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		cluster := gr.Str()
		clusterWidth := uniseg.StringWidth(cluster)
		if total+clusterWidth > width {
			break
		}
		builder.WriteString(cluster)
		total += clusterWidth
	}
	return builder.String()
}

func frameLine(width int, left, fill, right string) string {
	if width <= 0 {
		return ""
	}
	if width == 1 {
		return fallback(left, fill, right)
	}

	left = fallback(left, fill, " ")
	fill = fallback(fill, " ")
	right = fallback(right, fill, " ")
	return left + strings.Repeat(fill, width-2) + right
}

func fallback(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func frameTitleLabel(title string, width int) (label string, x int) {
	if title == "" || width <= 2 {
		return "", 0
	}
	label = " " + title + " "
	if stringWidth(label) > width-2 {
		label = truncateToWidth(label, width-2)
	}
	return label, 1
}

func padRightToWidth(s string, width int) string {
	s = truncateToWidth(s, width)
	if pad := width - stringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

func stringWidth(s string) int {
	return uniseg.StringWidth(s)
}
