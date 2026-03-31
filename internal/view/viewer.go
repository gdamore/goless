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
	SearchCase SearchCaseMode
	SearchWord SearchWordMode
	KeyGroup   KeyGroup
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
	keys       keyMap
	lines      []model.Line
	layout     layout.Result
	width      int
	height     int
	rowOffset  int
	colOffset  int
	follow     bool
	helpOffset int
}

// KeyResult summarizes how the viewer handled a key event.
type KeyResult struct {
	Handled bool
	Quit    bool
}

// Position summarizes the current visible viewport state.
type Position struct {
	Row    int
	Rows   int
	Column int
}

// New constructs a viewer for the given document.
func New(doc *model.Document, cfg Config) *Viewer {
	if cfg.TabWidth <= 0 {
		cfg.TabWidth = 8
	}
	cfg.SearchCase = normalizeSearchCaseMode(cfg.SearchCase)
	cfg.SearchWord = normalizeSearchWordMode(cfg.SearchWord)
	cfg.Chrome = cfg.Chrome.withDefaults()
	cfg.Text = cfg.Text.withDefaults()
	return &Viewer{
		doc:  doc,
		cfg:  cfg,
		text: cfg.Text,
		keys: defaultKeyMap(cfg.KeyGroup),
	}
}

// SetSearchCaseMode updates the default case behavior for new and active searches.
func (v *Viewer) SetSearchCaseMode(mode SearchCaseMode) {
	mode = normalizeSearchCaseMode(mode)
	if v.cfg.SearchCase == mode && (v.search.Query == "" || v.search.CaseMode == mode) {
		return
	}

	v.cfg.SearchCase = mode
	v.updatePromptPrefix()
	if v.mode == modePrompt && v.prompt != nil {
		v.updatePromptPreview()
		if v.prompt.preview != nil {
			return
		}
	}
	if v.search.Query == "" {
		return
	}

	v.search.CaseMode = mode
	v.rebuildSearch()
	if len(v.search.Matches) == 0 {
		v.message = v.text.SearchNotFound(v.search.Query)
		return
	}
	if v.search.Current < 0 || v.search.Current >= len(v.search.Matches) {
		v.search.Current = v.pickInitialMatch(v.search.Forward)
	}
	v.goToMatch(v.search.Current)
	v.message = v.text.SearchMatchCount(v.search.Query, len(v.search.Matches))
}

// SearchCaseMode reports the default case behavior for searches.
func (v *Viewer) SearchCaseMode() SearchCaseMode {
	return v.cfg.SearchCase
}

// SetSearchWordMode updates whether searches match substrings or whole words.
func (v *Viewer) SetSearchWordMode(mode SearchWordMode) {
	mode = normalizeSearchWordMode(mode)
	if v.cfg.SearchWord == mode && (v.search.Query == "" || v.search.WordMode == mode) {
		return
	}

	v.cfg.SearchWord = mode
	v.updatePromptPrefix()
	if v.mode == modePrompt && v.prompt != nil {
		v.updatePromptPreview()
		if v.prompt.preview != nil {
			return
		}
	}
	if v.search.Query == "" {
		return
	}

	v.search.WordMode = mode
	v.rebuildSearch()
	if len(v.search.Matches) == 0 {
		v.message = v.text.SearchNotFound(v.search.Query)
		return
	}
	if v.search.Current < 0 || v.search.Current >= len(v.search.Matches) {
		v.search.Current = v.pickInitialMatch(v.search.Forward)
	}
	v.goToMatch(v.search.Current)
	v.message = v.text.SearchMatchCount(v.search.Query, len(v.search.Matches))
}

// SearchWordMode reports whether searches match substrings or whole words.
func (v *Viewer) SearchWordMode() SearchWordMode {
	return v.cfg.SearchWord
}

// SetWrapMode updates the viewer wrap mode while preserving the current anchor.
func (v *Viewer) SetWrapMode(mode layout.WrapMode) {
	if mode != layout.SoftWrap {
		mode = layout.NoWrap
	}
	if v.cfg.WrapMode == mode {
		return
	}

	v.ensureLayout()
	anchor := v.firstVisibleAnchor()
	if mode == layout.NoWrap {
		v.colOffset = 0
		if anchor.LineIndex >= 0 && anchor.LineIndex < len(v.layout.Lines) {
			starts := v.layout.Lines[anchor.LineIndex].GraphemeCellStarts
			if anchor.GraphemeIndex >= 0 && anchor.GraphemeIndex < len(starts) {
				v.colOffset = starts[anchor.GraphemeIndex]
			}
		}
	}

	v.cfg.WrapMode = mode
	v.relayout()
	if v.follow {
		v.rowOffset = v.maxRowOffset()
		v.clampOffsets()
		return
	}
	v.restoreAnchor(anchor)
}

// WrapMode reports the current wrap mode.
func (v *Viewer) WrapMode() layout.WrapMode {
	return v.cfg.WrapMode
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
	if v.follow {
		v.rowOffset = v.maxRowOffset()
		v.clampOffsets()
	}
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
	return v.HandleKeyResult(ev).Quit
}

// HandleKeyResult applies a key event and reports whether it was handled and whether the viewer should exit.
func (v *Viewer) HandleKeyResult(ev *tcell.EventKey) KeyResult {
	if v.mode == modeHelp {
		return v.handleHelpKey(ev)
	}
	if v.mode == modePrompt {
		return v.handlePromptKey(ev)
	}

	switch v.keys.normalAction(ev) {
	case actionQuit:
		return KeyResult{Handled: true, Quit: true}
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
		if len(v.search.Matches) > 0 {
			v.follow = false
		}
		v.repeatSearch(v.search.Forward)
	case actionSearchPrev:
		if len(v.search.Matches) > 0 {
			v.follow = false
		}
		v.repeatSearch(!v.search.Forward)
	case actionToggleHelp:
		v.toggleHelp()
	case actionFollow:
		v.Follow()
	case actionCycleSearchCase:
		v.CycleSearchCaseMode()
	case actionCycleSearchWord:
		v.CycleSearchWordMode()
	default:
		return KeyResult{}
	}

	return KeyResult{Handled: true}
}

// ToggleWrap switches between horizontal scrolling and soft wrap modes.
func (v *Viewer) ToggleWrap() {
	if v.cfg.WrapMode == layout.NoWrap {
		v.SetWrapMode(layout.SoftWrap)
		return
	}
	v.SetWrapMode(layout.NoWrap)
}

// ScrollDown moves the viewport down.
func (v *Viewer) ScrollDown(n int) {
	v.ensureLayout()
	v.rowOffset += max(n, 0)
	v.clampOffsets()
	v.updateFollowAtBottom()
}

// ScrollUp moves the viewport up.
func (v *Viewer) ScrollUp(n int) {
	v.ensureLayout()
	if n > 0 {
		v.follow = false
	}
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
	v.follow = false
	v.rowOffset = 0
	v.clampOffsets()
}

// GoBottom moves the viewport to the end of the document.
func (v *Viewer) GoBottom() {
	v.ensureLayout()
	v.follow = false
	v.rowOffset = v.maxRowOffset()
	v.clampOffsets()
}

// Follow enables follow mode and pins the viewport to the end of the document.
func (v *Viewer) Follow() {
	v.GoBottom()
	v.follow = true
}

// Following reports whether follow mode is active.
func (v *Viewer) Following() bool {
	return v.follow
}

// Position reports the current visible row, total row count, and horizontal offset.
func (v *Viewer) Position() Position {
	v.ensureLayout()

	row := 0
	if len(v.layout.Rows) > 0 {
		row = min(v.rowOffset+1, len(v.layout.Rows))
	}
	return Position{
		Row:    row,
		Rows:   len(v.layout.Rows),
		Column: v.colOffset,
	}
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
	search := v.activeSearch()
	searchInfo := "search:" + v.searchModeLabel()
	if search.Query != "" {
		position := 0
		if len(search.Matches) > 0 && search.Current >= 0 {
			position = search.Current + 1
		}
		searchInfo += "  " + v.text.StatusSearchInfo(search.Query, position, len(search.Matches))
	}
	left = searchInfo
	if v.follow {
		if left != "" {
			left = v.text.FollowMode + "  " + left
		} else {
			left = v.text.FollowMode
		}
	}
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

func (v *Viewer) handleHelpKey(ev *tcell.EventKey) KeyResult {
	switch v.keys.helpAction(ev) {
	case actionQuit:
		return KeyResult{Handled: true, Quit: true}
	case actionToggleHelp:
		v.toggleHelp()
		return KeyResult{Handled: true}
	case actionCycleSearchCase:
		v.CycleSearchCaseMode()
		return KeyResult{Handled: true}
	case actionCycleSearchWord:
		v.CycleSearchWordMode()
		return KeyResult{Handled: true}
	case actionScrollUp:
		v.helpOffset--
	case actionScrollDown:
		v.helpOffset++
	case actionPageUp:
		v.helpOffset -= max(v.height-2, 1)
	case actionPageDown:
		v.helpOffset += max(v.height-2, 1)
	case actionGoTop:
		v.helpOffset = 0
	case actionGoBottom:
		v.helpOffset = v.maxHelpOffset()
	default:
		return KeyResult{}
	}
	v.clampHelpOffset()
	return KeyResult{Handled: true}
}

func (v *Viewer) handlePromptKey(ev *tcell.EventKey) KeyResult {
	switch ev.Key() {
	case tcell.KeyEscape:
		v.cancelPrompt()
		return KeyResult{Handled: true}
	case tcell.KeyCtrlC:
		return KeyResult{Handled: true, Quit: true}
	case tcell.KeyF2:
		v.CycleSearchCaseMode()
		return KeyResult{Handled: true}
	case tcell.KeyF3:
		v.CycleSearchWordMode()
		return KeyResult{Handled: true}
	case tcell.KeyEnter:
		v.commitPrompt()
		return KeyResult{Handled: true}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if v.prompt != nil && len(v.prompt.buffer) > 0 {
			v.prompt.buffer = v.prompt.buffer[:len(v.prompt.buffer)-1]
		}
		v.updatePromptPreview()
		return KeyResult{Handled: true}
	case tcell.KeyCtrlU:
		if v.prompt != nil {
			v.prompt.buffer = v.prompt.buffer[:0]
		}
		v.updatePromptPreview()
		return KeyResult{Handled: true}
	case tcell.KeyRune:
		if v.prompt != nil {
			v.prompt.buffer = append(v.prompt.buffer, []rune(ev.Str())...)
		}
		v.updatePromptPreview()
		return KeyResult{Handled: true}
	}
	return KeyResult{}
}

func (v *Viewer) toggleHelp() {
	if v.mode == modeHelp {
		v.mode = modeNormal
		return
	}
	v.mode = modeHelp
	v.helpOffset = 0
}

func (v *Viewer) updateFollowAtBottom() {
	v.follow = v.rowOffset == v.maxRowOffset()
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
