// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"fmt"
	"strconv"
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
	TabWidth          int
	WrapMode          layout.WrapMode
	SearchCase        SearchCaseMode
	SearchMode        SearchMode
	SqueezeBlankLines bool
	LineNumbers       bool
	HeaderLines       int
	HeaderColumns     int
	Theme             Theme
	Visualization     Visualization
	HyperlinkHandler  HyperlinkHandler
	CommandHandler    CommandHandler
	KeyGroup          KeyGroup
	KeyUnbind         []KeyStroke
	KeyBind           []KeyBinding
	Chrome            Chrome
	ShowStatus        bool
	Text              Text
}

// Viewer is a minimal document viewer built on the model and layout packages.
type Viewer struct {
	doc         *model.Document
	cfg         Config
	mode        viewerMode
	prompt      *promptState
	message     string
	search      searchState
	text        Text
	keys        keyMap
	sourceLines []model.Line
	lines       []model.Line
	lineMap     []int
	layout      layout.Result
	width       int
	height      int
	rowOffset   int
	colOffset   int
	follow      bool
	helpOffset  int
}

// KeyResult summarizes how the viewer handled a key event.
type KeyResult struct {
	Handled bool
	Quit    bool
	Action  KeyAction
	Context KeyContext
}

// Position summarizes the current visible viewport state.
type Position struct {
	Row     int
	Rows    int
	Column  int
	Columns int
}

// New constructs a viewer for the given document.
func New(doc *model.Document, cfg Config) *Viewer {
	if cfg.TabWidth <= 0 {
		cfg.TabWidth = 8
	}
	cfg.SearchCase = normalizeSearchCaseMode(cfg.SearchCase)
	cfg.SearchMode = normalizeSearchMode(cfg.SearchMode)
	cfg.Visualization = cfg.Visualization.withDefaults()
	cfg.Chrome = cfg.Chrome.withDefaults()
	cfg.Text = cfg.Text.withDefaults()
	return &Viewer{
		doc:  doc,
		cfg:  cfg,
		text: cfg.Text,
		keys: defaultKeyMap(cfg.KeyGroup).withOverrides(cfg.KeyUnbind, cfg.KeyBind),
	}
}

// SetTheme updates how document content colors are rendered.
func (v *Viewer) SetTheme(theme Theme) {
	v.cfg.Theme = theme
}

// SetLineNumbers updates whether the adaptive line-number gutter is shown.
func (v *Viewer) SetLineNumbers(enabled bool) {
	if v.cfg.LineNumbers == enabled {
		return
	}
	v.ensureLayout()
	anchor := v.firstVisibleAnchor()
	v.cfg.LineNumbers = enabled
	v.relayout()
	if v.follow {
		v.rowOffset = v.maxRowOffset()
		v.clampOffsets()
		return
	}
	v.restoreAnchor(anchor)
}

// ToggleLineNumbers shows or hides the adaptive line-number gutter.
func (v *Viewer) ToggleLineNumbers() {
	v.SetLineNumbers(!v.cfg.LineNumbers)
}

// LineNumbers reports whether the adaptive line-number gutter is enabled.
func (v *Viewer) LineNumbers() bool {
	return v.cfg.LineNumbers
}

// SetSqueezeBlankLines updates whether repeated blank lines are collapsed in the current view.
func (v *Viewer) SetSqueezeBlankLines(enabled bool) {
	if v.cfg.SqueezeBlankLines == enabled {
		return
	}
	v.ensureLayout()
	anchor := v.firstVisibleSourceAnchor()
	v.cfg.SqueezeBlankLines = enabled
	v.relayout()
	if v.follow {
		v.rowOffset = v.maxRowOffset()
		v.clampOffsets()
		return
	}
	v.restoreSourceAnchor(anchor)
}

// SqueezeBlankLines reports whether repeated blank lines are collapsed in the current view.
func (v *Viewer) SqueezeBlankLines() bool {
	return v.cfg.SqueezeBlankLines
}

// SetHeaderLines updates how many leading logical lines stay fixed at the top of the viewport.
func (v *Viewer) SetHeaderLines(count int) {
	if count < 0 {
		count = 0
	}
	if v.cfg.HeaderLines == count {
		return
	}
	v.ensureLayout()
	anchor := v.firstVisibleAnchor()
	v.cfg.HeaderLines = count
	v.relayout()
	if v.follow {
		v.rowOffset = v.maxRowOffset()
		v.clampOffsets()
		return
	}
	v.restoreAnchor(anchor)
}

// HeaderLines reports how many leading logical lines stay fixed at the top of the viewport.
func (v *Viewer) HeaderLines() int {
	return v.cfg.HeaderLines
}

// SetHeaderColumns updates how many leading display columns stay fixed at the left edge of the viewport.
func (v *Viewer) SetHeaderColumns(count int) {
	if count < 0 {
		count = 0
	}
	if v.cfg.HeaderColumns == count {
		return
	}
	v.ensureLayout()
	anchor := v.firstVisibleAnchor()
	v.cfg.HeaderColumns = count
	v.relayout()
	if v.follow {
		v.rowOffset = v.maxRowOffset()
		v.clampOffsets()
		return
	}
	v.restoreAnchor(anchor)
}

// HeaderColumns reports how many leading display columns stay fixed at the left edge of the viewport.
func (v *Viewer) HeaderColumns() int {
	return v.cfg.HeaderColumns
}

// SetVisualization updates how hidden structure markers are drawn.
func (v *Viewer) SetVisualization(visual Visualization) {
	v.cfg.Visualization = visual.withDefaults()
	v.ensureLayout()
	v.clampOffsets()
	v.relayout()
}

// SetHyperlinkHandler updates how parsed OSC 8 hyperlinks are rendered.
func (v *Viewer) SetHyperlinkHandler(handler HyperlinkHandler) {
	v.cfg.HyperlinkHandler = handler
}

// SetChrome updates frame, title, and prompt/status styling.
func (v *Viewer) SetChrome(chrome Chrome) {
	v.ensureLayout()
	anchor := v.firstVisibleAnchor()
	v.cfg.Chrome = chrome.withDefaults()
	v.relayout()
	v.clampHelpOffset()
	if v.follow {
		v.rowOffset = v.maxRowOffset()
		v.clampOffsets()
		return
	}
	v.restoreAnchor(anchor)
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
	}
	if v.search.Query == "" {
		return
	}

	v.search.CaseMode = mode
	v.rebuildSearch()
	if v.search.CompileError != "" {
		v.message = v.search.CompileError
		return
	}
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

// SetSearchMode updates whether searches use substring, whole-word, or regex matching.
func (v *Viewer) SetSearchMode(mode SearchMode) {
	mode = normalizeSearchMode(mode)
	if v.cfg.SearchMode == mode && (v.search.Query == "" || v.search.Mode == mode) {
		return
	}

	v.cfg.SearchMode = mode
	v.updatePromptPrefix()
	if v.mode == modePrompt && v.prompt != nil {
		v.updatePromptPreview()
	}
	if v.search.Query == "" {
		return
	}

	v.search.Mode = mode
	v.rebuildSearch()
	if v.search.CompileError != "" {
		v.message = v.search.CompileError
		return
	}
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

// SearchMode reports whether searches use substring, whole-word, or regex matching.
func (v *Viewer) SearchMode() SearchMode {
	return v.cfg.SearchMode
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
	screen.Fill(' ', v.toTCellStyle(ansi.DefaultStyle()))

	if v.mode == modeHelp {
		v.drawFrame(screen, v.helpFrameTitle())
		v.drawHelp(screen)
		screen.Sync()
		return
	}

	v.drawFrame(screen, v.cfg.Chrome.Title)

	bodyX, bodyY, _, bodyHeight := v.contentRect()
	v.drawLineNumberGutter(screen)
	lineHyperlinks := v.resolveVisibleHyperlinks()
	for y := 0; y < bodyHeight; y++ {
		rowIndex, header, ok := v.visibleLayoutRowAt(y)
		if !ok {
			break
		}
		row := v.layout.Rows[rowIndex]
		v.drawHeaderColumns(screen, bodyY+y, row, header, lineHyperlinks)
		v.drawRow(screen, bodyX, bodyY+y, row, header, lineHyperlinks)
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

	a := v.keys.normalAction(ev)
	switch a {
	case actionQuit:
		return KeyResult{Handled: true, Quit: true, Action: KeyActionQuit, Context: KeyContextNormal}
	case actionScrollUp:
		v.ScrollUp(1)
	case actionScrollDown:
		v.ScrollDown(1)
	case actionScrollLeft:
		v.ScrollLeft(1)
	case actionScrollRight:
		v.ScrollRight(1)
	case actionHalfPageUp:
		v.HalfPageUp()
	case actionHalfPageDown:
		v.HalfPageDown()
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
	case actionCycleSearchMode:
		v.CycleSearchMode()
	default:
		return KeyResult{}
	}

	return KeyResult{Handled: true, Action: KeyAction(a), Context: KeyContextNormal}
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
	wasFollowing := v.follow
	v.rowOffset += max(n, 0)
	v.clampOffsets()
	if wasFollowing {
		v.updateFollowAtBottom()
	}
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
	step := max(v.scrollBodyHeight()-1, 1)
	v.ScrollDown(step)
}

// HalfPageDown moves the viewport down by roughly half a page.
func (v *Viewer) HalfPageDown() {
	step := max(v.scrollBodyHeight()/2, 1)
	v.ScrollDown(step)
}

// PageUp moves the viewport up by roughly one page.
func (v *Viewer) PageUp() {
	step := max(v.scrollBodyHeight()-1, 1)
	v.ScrollUp(step)
}

// HalfPageUp moves the viewport up by roughly half a page.
func (v *Viewer) HalfPageUp() {
	step := max(v.scrollBodyHeight()/2, 1)
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

// GoPercent moves the viewport so the first visible row is near the requested percentage.
func (v *Viewer) GoPercent(percent int) bool {
	v.ensureLayout()
	if v.scrollableRowCount() == 0 {
		return false
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	v.follow = false
	v.rowOffset = ((v.scrollableRowCount() - 1) * percent) / 100
	v.clampOffsets()
	return true
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

// EOFVisible reports whether the last layout row is currently visible.
func (v *Viewer) EOFVisible() bool {
	v.ensureLayout()
	if len(v.layout.Rows) == 0 {
		return true
	}
	rowIndex, ok := v.lastVisibleLayoutRow()
	return ok && rowIndex >= len(v.layout.Rows)-1
}

// Position reports the current visible row, total row count, horizontal offset, and maximum column span.
func (v *Viewer) Position() Position {
	v.ensureLayout()
	return Position{
		Row:     v.firstVisibleRow(),
		Rows:    len(v.layout.Rows),
		Column:  v.colOffset,
		Columns: v.maxContentColumns(),
	}
}

func (v *Viewer) ensureLayout() {
	if v.layout.Rows == nil {
		v.relayout()
	}
}

func (v *Viewer) relayout() {
	v.sourceLines = v.doc.Lines()
	v.lines, v.lineMap = squeezeBlankLines(v.sourceLines, v.cfg.SqueezeBlankLines)
	v.layout = layout.Build(v.lines, layout.Config{
		Width:            max(v.bodyContentWidth(), 1),
		TabWidth:         v.cfg.TabWidth,
		WrapMode:         v.cfg.WrapMode,
		HorizontalOffset: v.horizontalOffset(),
		LeadingColumns:   v.headerColumnWidth(v.rawContentWidth()),
	})
	v.rebuildSearch()
	v.clampOffsets()
}

func (v *Viewer) horizontalOffset() int {
	if v.cfg.WrapMode == layout.NoWrap {
		return v.headerColumnWidth(v.rawContentWidth()) + max(v.colOffset, 0)
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
	if v.scrollBodyHeight() <= 0 {
		return 0
	}
	return max(v.scrollableRowCount()-v.scrollBodyHeight(), 0)
}

func (v *Viewer) maxColOffset() int {
	if v.cfg.WrapMode != layout.NoWrap {
		return 0
	}
	return max(v.maxContentColumns()-max(v.bodyContentWidth(), 1), 0)
}

func (v *Viewer) bodyHeight() int {
	_, _, _, h := v.contentRect()
	return h
}

func (v *Viewer) firstVisibleRow() int {
	v.ensureLayout()
	if len(v.layout.Rows) == 0 {
		return 0
	}
	if v.visibleHeaderRowCount() > 0 {
		rowIndex := v.scrollableRowStartIndex() + v.rowOffset
		if rowIndex >= 0 && rowIndex < len(v.layout.Rows) {
			return rowIndex + 1
		}
	}
	rowIndex, _, ok := v.visibleLayoutRowAt(0)
	if !ok || rowIndex < 0 {
		return 0
	}
	return min(rowIndex+1, len(v.layout.Rows))
}

func (v *Viewer) headerLineCount() int {
	return min(max(v.cfg.HeaderLines, 0), len(v.lines))
}

func (v *Viewer) headerRowCount() int {
	count := 0
	for _, row := range v.layout.Rows {
		if row.LineIndex >= v.headerLineCount() {
			break
		}
		count++
	}
	return count
}

func (v *Viewer) visibleHeaderRowCount() int {
	return min(v.headerRowCount(), v.bodyHeight())
}

func (v *Viewer) scrollBodyHeight() int {
	return max(v.bodyHeight()-v.visibleHeaderRowCount(), 0)
}

func (v *Viewer) scrollableRowStartIndex() int {
	return v.headerRowCount()
}

func (v *Viewer) scrollableRowCount() int {
	return max(len(v.layout.Rows)-v.scrollableRowStartIndex(), 0)
}

func (v *Viewer) visibleLayoutRowAt(y int) (rowIndex int, header bool, ok bool) {
	if y < 0 || y >= v.bodyHeight() {
		return 0, false, false
	}
	headerRows := v.visibleHeaderRowCount()
	if y < headerRows {
		rowIndex = y
		return rowIndex, true, rowIndex < len(v.layout.Rows)
	}
	rowIndex = v.scrollableRowStartIndex() + v.rowOffset + (y - headerRows)
	if rowIndex < 0 || rowIndex >= len(v.layout.Rows) {
		return 0, false, false
	}
	return rowIndex, false, true
}

func (v *Viewer) lastVisibleLayoutRow() (int, bool) {
	for y := v.bodyHeight() - 1; y >= 0; y-- {
		rowIndex, _, ok := v.visibleLayoutRowAt(y)
		if ok {
			return rowIndex, true
		}
	}
	return 0, false
}

func (v *Viewer) firstVisibleAnchor() layout.Anchor {
	rowIndex := v.scrollableRowStartIndex() + v.rowOffset
	if len(v.layout.Rows) == 0 || rowIndex < 0 || rowIndex >= len(v.layout.Rows) {
		return layout.Anchor{}
	}
	return v.layout.AnchorForRow(rowIndex)
}

func (v *Viewer) firstVisibleSourceAnchor() layout.Anchor {
	anchor := v.firstVisibleAnchor()
	anchor.LineIndex = v.sourceLineIndex(anchor.LineIndex)
	return anchor
}

func (v *Viewer) restoreAnchor(anchor layout.Anchor) {
	if rowIndex := v.layout.RowIndexForAnchor(anchor); rowIndex >= 0 {
		v.rowOffset = max(rowIndex-v.scrollableRowStartIndex(), 0)
	}
	v.clampOffsets()
}

func (v *Viewer) restoreSourceAnchor(anchor layout.Anchor) {
	anchor.LineIndex = v.displayLineIndexForSource(anchor.LineIndex)
	v.restoreAnchor(anchor)
}

func (v *Viewer) revealAnchor(anchor layout.Anchor) {
	rowIndex := v.layout.RowIndexForAnchor(anchor)
	if rowIndex < 0 {
		return
	}

	bodyHeight := max(v.bodyHeight(), 1)
	headerRows := v.visibleHeaderRowCount()
	scrollBodyHeight := max(bodyHeight-headerRows, 1)
	scrollRowIndex := rowIndex - v.scrollableRowStartIndex()
	switch {
	case scrollRowIndex < v.rowOffset:
		v.rowOffset = scrollRowIndex
	case scrollRowIndex >= v.rowOffset+scrollBodyHeight:
		v.rowOffset = scrollRowIndex - scrollBodyHeight + 1
	}
	v.clampOffsets()
}

func (v *Viewer) drawRow(screen tcell.Screen, baseX, y int, row layout.VisualRow, header bool, lineHyperlinks map[int]rowHyperlinks) {
	if row.LineIndex < 0 || row.LineIndex >= len(v.lines) {
		return
	}
	line := v.lines[row.LineIndex]
	hyperlinks := lineHyperlinks[row.LineIndex]
	rowStyle := v.rowBaseStyle(header)
	screen.PutStrStyled(baseX, y, padRightToWidth("", v.contentWidth()), rowStyle)
	for _, segment := range row.Segments {
		v.drawSegment(screen, baseX, y, row, line, segment, header, hyperlinks, v.contentWidth())
	}
	v.drawTrailingMarkers(screen, baseX, y, row, line, rowStyle, header)
}

func (v *Viewer) drawHeaderColumns(screen tcell.Screen, y int, row layout.VisualRow, header bool, lineHyperlinks map[int]rowHyperlinks) {
	baseX, _, width, _ := v.headerColumnRect()
	if width <= 0 || row.LineIndex < 0 || row.LineIndex >= len(v.lines) {
		return
	}

	style := v.rowBaseStyle(true)
	screen.PutStrStyled(baseX, y, padRightToWidth("", width), style)
	line := v.lines[row.LineIndex]
	hyperlinks := lineHyperlinks[row.LineIndex]
	for _, segment := range v.headerColumnSegments(row, width) {
		v.drawSegment(screen, baseX, y, row, line, segment, true, hyperlinks, width)
	}
}

func (v *Viewer) drawSegment(screen tcell.Screen, baseX, y int, row layout.VisualRow, line model.Line, segment layout.VisualSegment, header bool, hyperlinks rowHyperlinks, maxWidth int) {
	if segment.LogicalGraphemeIndex < 0 || segment.LogicalGraphemeIndex >= len(line.Graphemes) {
		return
	}
	grapheme := line.Graphemes[segment.LogicalGraphemeIndex]
	cellStyle := v.toTCellStyle(styleForGrapheme(line, grapheme.RuneStart))
	if header {
		cellStyle = applyChromeStyle(cellStyle, v.cfg.Chrome.HeaderStyle)
	}
	if hyperlink, ok := hyperlinks.byGrapheme[segment.LogicalGraphemeIndex]; ok {
		cellStyle = applyHyperlinkDecisionStyle(cellStyle, hyperlink.decision, hyperlink.span)
	}
	matched, current := v.graphemeMatched(line, row.LineIndex, grapheme)
	if matched {
		cellStyle = applyMatchCellStyle(cellStyle, current)
	}

	x := segment.RenderedCellFrom
	if x >= maxWidth {
		return
	}
	x += baseX

	if display, style, ok := v.visualizedSegment(row, line, segment, grapheme, cellStyle); ok {
		screen.PutStrStyled(x, y, truncateToWidth(display, maxWidth-segment.RenderedCellFrom), style)
		return
	}

	if segment.Display != "" {
		screen.PutStrStyled(x, y, truncateToWidth(segment.Display, maxWidth-segment.RenderedCellFrom), cellStyle)
		return
	}

	if grapheme.Text == "\t" {
		width := min(segment.RenderedCellTo-segment.RenderedCellFrom, maxWidth-segment.RenderedCellFrom)
		screen.PutStrStyled(x, y, strings.Repeat(" ", width), cellStyle)
		return
	}

	screen.Put(x, y, grapheme.Text, cellStyle)
}

func (v *Viewer) headerColumnSegments(row layout.VisualRow, width int) []layout.VisualSegment {
	if width <= 0 || row.LineIndex < 0 || row.LineIndex >= len(v.layout.Lines) {
		return nil
	}
	if v.cfg.WrapMode == layout.SoftWrap && row.SourceCellStart > width {
		return nil
	}

	info := v.layout.Lines[row.LineIndex]
	segments := make([]layout.VisualSegment, 0, width+1)
	for i := range info.GraphemeCellStarts {
		sourceStart := info.GraphemeCellStarts[i]
		sourceEnd := info.GraphemeCellEnds[i]
		if sourceStart >= width {
			break
		}
		if sourceEnd <= 0 {
			continue
		}

		segment := layout.VisualSegment{
			LogicalGraphemeIndex: i,
			SourceCellStart:      sourceStart,
			SourceCellEnd:        sourceEnd,
			RenderedCellFrom:     sourceStart,
			RenderedCellTo:       min(sourceEnd, width),
		}
		if sourceEnd > width {
			visibleWidth := max(width-sourceStart, 0)
			if visibleWidth <= 0 {
				continue
			}
			if v.lines[row.LineIndex].Graphemes[i].Text == "\t" {
				segment.Display = strings.Repeat(" ", visibleWidth)
			} else {
				segment.Display = ">"
				segment.RenderedCellTo = sourceStart + 1
			}
		}
		segments = append(segments, segment)
	}
	return segments
}

type hyperlinkSpan struct {
	startGrapheme   int
	endGrapheme     int
	sourceCellStart int
	sourceCellEnd   int
	target          string
	id              string
	text            string
	baseStyle       tcell.Style
}

type resolvedHyperlink struct {
	span     hyperlinkSpan
	decision HyperlinkDecision
}

type rowHyperlinks struct {
	byGrapheme map[int]resolvedHyperlink
}

func (v *Viewer) resolveVisibleHyperlinks() map[int]rowHyperlinks {
	if v.cfg.HyperlinkHandler == nil {
		return nil
	}
	result := make(map[int]rowHyperlinks)
	bodyHeight := max(v.bodyHeight(), 0)
	for y := 0; y < bodyHeight; y++ {
		rowIndex, _, ok := v.visibleLayoutRowAt(y)
		if !ok {
			break
		}
		lineIndex := v.layout.Rows[rowIndex].LineIndex
		if lineIndex < 0 || lineIndex >= len(v.lines) {
			continue
		}
		if _, ok := result[lineIndex]; ok {
			continue
		}
		result[lineIndex] = v.resolveLineHyperlinks(lineIndex, v.lines[lineIndex])
	}
	return result
}

func (v *Viewer) resolveLineHyperlinks(lineIndex int, line model.Line) rowHyperlinks {
	if v.cfg.HyperlinkHandler == nil || lineIndex < 0 || lineIndex >= len(v.layout.Lines) {
		return rowHyperlinks{}
	}

	result := rowHyperlinks{
		byGrapheme: make(map[int]resolvedHyperlink),
	}

	for _, span := range v.hyperlinkSpans(lineIndex, line) {
		decision := v.cfg.HyperlinkHandler(HyperlinkInfo{
			Target: span.target,
			ID:     span.id,
			Text:   span.text,
			Style:  span.baseStyle,
		})
		resolved := resolvedHyperlink{
			span:     span,
			decision: decision,
		}
		for i := span.startGrapheme; i < span.endGrapheme; i++ {
			result.byGrapheme[i] = resolved
		}
	}
	return result
}

func (v *Viewer) hyperlinkSpans(lineIndex int, line model.Line) []hyperlinkSpan {
	if lineIndex < 0 || lineIndex >= len(v.layout.Lines) || len(line.Graphemes) == 0 {
		return nil
	}
	layoutLine := v.layout.Lines[lineIndex]
	spans := make([]hyperlinkSpan, 0)
	for i := 0; i < len(line.Graphemes); {
		style := styleForGrapheme(line, line.Graphemes[i].RuneStart)
		if style.URL == "" {
			i++
			continue
		}
		start := i
		for i < len(line.Graphemes) {
			nextStyle := styleForGrapheme(line, line.Graphemes[i].RuneStart)
			if nextStyle.URL != style.URL || nextStyle.URLID != style.URLID {
				break
			}
			i++
		}
		end := i
		startByte := line.Graphemes[start].ByteStart
		endByte := line.Graphemes[end-1].ByteEnd
		spans = append(spans, hyperlinkSpan{
			startGrapheme:   start,
			endGrapheme:     end,
			sourceCellStart: layoutLine.GraphemeCellStarts[start],
			sourceCellEnd:   layoutLine.GraphemeCellEnds[end-1],
			target:          style.URL,
			id:              style.URLID,
			text:            line.Text[startByte:endByte],
			baseStyle:       v.toTCellStyle(style),
		})
	}
	return spans
}

func applyHyperlinkDecisionStyle(style tcell.Style, decision HyperlinkDecision, span hyperlinkSpan) tcell.Style {
	if decision.StyleSet {
		style = decision.Style
	}
	if !decision.Live {
		return style.Url("").UrlId("")
	}
	target := decision.Target
	if target == "" {
		target = span.target
	}
	if target == "" {
		return style.Url("").UrlId("")
	}
	style = style.Url(target)
	if span.id != "" {
		style = style.UrlId(span.id)
	}
	return style
}

func (v *Viewer) visualizedSegment(row layout.VisualRow, line model.Line, segment layout.VisualSegment, grapheme model.Grapheme, baseStyle tcell.Style) (string, tcell.Style, bool) {
	if grapheme.Text == "\t" && v.cfg.Visualization.ShowTabs && segment.Display == "" {
		width := segment.RenderedCellTo - segment.RenderedCellFrom
		return padMarkerGlyph(v.cfg.Visualization.TabGlyph, width), v.visualizationStyle(baseStyle), true
	}
	if grapheme.Text == "\u240d" && v.cfg.Visualization.ShowCarriageReturns {
		width := max(segment.RenderedCellTo-segment.RenderedCellFrom, 1)
		return padMarkerGlyph(v.cfg.Visualization.CarriageReturnGlyph, width), v.visualizationStyle(baseStyle), true
	}
	return "", tcell.StyleDefault, false
}

func (v *Viewer) drawTrailingMarkers(screen tcell.Screen, baseX, y int, row layout.VisualRow, line model.Line, rowStyle tcell.Style, header bool) {
	markers := v.trailingMarkers(row, line)
	if markers == "" {
		return
	}
	width := v.contentWidth()
	if width <= 0 {
		return
	}
	start := v.layout.Lines[row.LineIndex].TotalCells - row.SourceCellStart
	if start >= width {
		return
	}
	if start < 0 {
		markers = trimLeftToWidth(markers, -start)
		start = 0
	}
	if markers == "" {
		return
	}
	style := v.visualizationStyle(rowStyle)
	screen.PutStrStyled(baseX+start, y, truncateToWidth(markers, width-start), style)
}

func (v *Viewer) trailingMarkers(row layout.VisualRow, line model.Line) string {
	if row.LineIndex < 0 || row.LineIndex >= len(v.layout.Lines) {
		return ""
	}
	if v.cfg.WrapMode == layout.SoftWrap && v.layout.Lines[row.LineIndex].TotalCells != row.SourceCellEnd {
		return ""
	}
	return v.lineEndMarkers(row.LineIndex, line)
}

func (v *Viewer) lineEndMarkers(lineIndex int, line model.Line) string {
	var marker strings.Builder
	if line.Ending == model.LineEndingCRLF && v.cfg.Visualization.ShowCarriageReturns {
		marker.WriteString(v.cfg.Visualization.CarriageReturnGlyph)
	}
	if line.Ending != model.LineEndingNone && v.cfg.Visualization.ShowNewlines {
		marker.WriteString(v.cfg.Visualization.NewlineGlyph)
	}
	if v.cfg.Visualization.ShowEOF && lineIndex == len(v.lines)-1 {
		marker.WriteString(v.cfg.Visualization.EOFGlyph)
	}
	return marker.String()
}

func (v *Viewer) trailingMarkerCellWidth(lineIndex int) int {
	if lineIndex < 0 || lineIndex >= len(v.lines) {
		return 0
	}
	return stringWidth(v.lineEndMarkers(lineIndex, v.lines[lineIndex]))
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
	if label, x := frameTitleLabel(title, v.width, v.cfg.Chrome.TitleAlign); label != "" {
		screen.PutStrStyled(x, topY, label, titleStyle)
	}

	side := fallback(frame.Vertical, "│")
	for y := topY + 1; y < bottomY; y++ {
		screen.PutStrStyled(0, y, side, borderStyle)
		screen.PutStrStyled(v.width-1, y, side, borderStyle)
	}
}

func (v *Viewer) drawStatus(screen tcell.Screen, y int) {
	style := v.cfg.Chrome.StatusStyle
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

func (v *Viewer) drawLineNumberGutter(screen tcell.Screen) {
	if v.mode == modeHelp || !v.cfg.LineNumbers {
		return
	}
	gutterX, gutterY, gutterWidth, gutterHeight := v.gutterRect()
	if gutterWidth <= 0 || gutterHeight <= 0 {
		return
	}

	blank := padRightToWidth("", gutterWidth)
	for y := 0; y < gutterHeight; y++ {
		rowIndex, header, ok := v.visibleLayoutRowAt(y)
		rowFillStyle := applyChromeStyle(v.rowBaseStyle(header), v.cfg.Chrome.LineNumberStyle)
		rowTextStyle := rowFillStyle
		screen.PutStrStyled(gutterX, gutterY+y, blank, rowFillStyle)
		if !ok {
			continue
		}
		row := v.layout.Rows[rowIndex]
		if !v.shouldShowLineNumber(rowIndex) {
			continue
		}
		label := padRightToWidth(fmt.Sprintf("%*d", gutterWidth-1, row.LineIndex+1), gutterWidth)
		screen.PutStrStyled(gutterX, gutterY+y, label, rowTextStyle)
	}
}

func (v *Viewer) rowBaseStyle(header bool) tcell.Style {
	style := v.toTCellStyle(ansi.DefaultStyle())
	if header {
		style = applyChromeStyle(style, v.cfg.Chrome.HeaderStyle)
	}
	return style
}

func applyChromeStyle(base, overlay tcell.Style) tcell.Style {
	if fg := overlay.GetForeground(); fg != tcolor.Default {
		base = base.Foreground(fg)
	}
	if bg := overlay.GetBackground(); bg != tcolor.Default {
		base = base.Background(bg)
	}
	if overlay.HasBold() {
		base = base.Bold(true)
	}
	if overlay.HasDim() {
		base = base.Dim(true)
	}
	if overlay.HasItalic() {
		base = base.Italic(true)
	}
	if overlay.HasReverse() {
		base = base.Reverse(true)
	}
	if ul := overlay.GetUnderlineStyle(); ul != tcell.UnderlineStyleNone {
		if uc := overlay.GetUnderlineColor(); uc != tcolor.Default {
			base = base.Underline(ul, uc)
		} else {
			base = base.Underline(ul)
		}
	}
	return base
}

func (v *Viewer) shouldShowLineNumber(rowIndex int) bool {
	if rowIndex < 0 || rowIndex >= len(v.layout.Rows) {
		return false
	}
	if v.cfg.WrapMode == layout.NoWrap || rowIndex == 0 {
		return true
	}
	return v.layout.Rows[rowIndex-1].LineIndex != v.layout.Rows[rowIndex].LineIndex
}

func (v *Viewer) lineNumberGutterWidth(available int) int {
	if !v.cfg.LineNumbers || v.mode == modeHelp || available <= 1 {
		return 0
	}
	digits := len(strconv.Itoa(max(len(v.lines), 1)))
	width := digits + 1
	return min(width, available-1)
}

func (v *Viewer) headerColumnWidth(available int) int {
	if v.mode == modeHelp || available <= 1 {
		return 0
	}
	return min(max(v.cfg.HeaderColumns, 0), available-1)
}

func (v *Viewer) gutterRect() (x, y, width, height int) {
	x, y, totalWidth, height := v.baseContentRect()
	width = v.lineNumberGutterWidth(totalWidth)
	return x, y, width, height
}

func (v *Viewer) rawContentWidth() int {
	_, _, width, _ := v.baseContentRect()
	width -= v.lineNumberGutterWidth(width)
	if width < 0 {
		width = 0
	}
	return width
}

func (v *Viewer) headerColumnRect() (x, y, width, height int) {
	x, y, width, height = v.baseContentRect()
	gutterWidth := v.lineNumberGutterWidth(width)
	x += gutterWidth
	width -= gutterWidth
	if width < 0 {
		width = 0
	}
	headerWidth := v.headerColumnWidth(width)
	return x, y, headerWidth, height
}

func (v *Viewer) contentRect() (x, y, width, height int) {
	x, y, width, height = v.baseContentRect()
	gutterWidth := v.lineNumberGutterWidth(width)
	headerWidth := v.headerColumnWidth(width - gutterWidth)
	width -= gutterWidth
	width -= headerWidth
	if width < 0 {
		width = 0
	}
	x += gutterWidth + headerWidth
	return x, y, width, height
}

func (v *Viewer) baseContentRect() (x, y, width, height int) {
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

func (v *Viewer) bodyContentWidth() int {
	return v.contentWidth()
}

func (v *Viewer) sourceLineIndex(displayLine int) int {
	if displayLine < 0 {
		return 0
	}
	if displayLine < len(v.lineMap) {
		return v.lineMap[displayLine]
	}
	if len(v.sourceLines) == 0 {
		return 0
	}
	return len(v.sourceLines) - 1
}

func (v *Viewer) displayLineIndexForSource(sourceLine int) int {
	if sourceLine <= 0 || len(v.lineMap) == 0 {
		return 0
	}
	if sourceLine >= len(v.sourceLines) {
		return len(v.lineMap) - 1
	}
	displayLine := 0
	for i, mapped := range v.lineMap {
		if mapped > sourceLine {
			break
		}
		displayLine = i
	}
	return displayLine
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
	if search.Query != "" || normalizeSearchCaseMode(v.cfg.SearchCase) != SearchSmartCase || normalizeSearchMode(v.cfg.SearchMode) != SearchSubstring {
		left = "search:" + v.searchModeLabel()
		if search.Query != "" {
			left = "search:" + searchModeLabel(search.CaseMode, search.Mode)
			position := 0
			if len(search.Matches) > 0 && search.Current >= 0 {
				position = search.Current + 1
			}
			left += "  " + v.text.StatusSearchInfo(search.Query, position, len(search.Matches))
		}
	}
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

	current := v.firstVisibleRow()
	right = v.text.StatusPosition(current, len(v.layout.Rows), v.colOffset, v.maxContentColumns())
	if modeHint := v.statusModeHint(); modeHint != "" {
		if right != "" {
			right += "  " + modeHint
		} else {
			right = modeHint
		}
	}
	if v.text.StatusLine != nil {
		return v.text.StatusLine(StatusInfo{
			Search:    v.SearchSnapshot(),
			Following: v.follow,
			Message:   v.message,
			Position: Position{
				Row:     current,
				Rows:    len(v.layout.Rows),
				Column:  v.colOffset,
				Columns: v.maxContentColumns(),
			},
			DefaultLeft:  left,
			DefaultRight: right,
		})
	}
	return left, right
}

func (v *Viewer) statusModeHint() string {
	if v.cfg.WrapMode == layout.SoftWrap {
		return "↪"
	}
	return "⇆"
}

func (v *Viewer) statusOverflow() (left, right bool) {
	if v.cfg.WrapMode != layout.NoWrap {
		return false, false
	}

	return v.colOffset > 0, v.colOffset < v.maxColOffset()
}

func (v *Viewer) maxContentColumns() int {
	frozen := v.headerColumnWidth(v.rawContentWidth())
	maxCells := 0
	for i, line := range v.layout.Lines {
		totalCells := line.TotalCells + v.trailingMarkerCellWidth(i)
		if totalCells > maxCells {
			maxCells = totalCells
		}
	}
	return max(maxCells-frozen, 0)
}

func squeezeBlankLines(lines []model.Line, enabled bool) ([]model.Line, []int) {
	if len(lines) == 0 {
		return nil, nil
	}

	if !enabled {
		lineMap := make([]int, len(lines))
		for i := range lines {
			lineMap[i] = i
		}
		return lines, lineMap
	}

	squeezed := make([]model.Line, 0, len(lines))
	lineMap := make([]int, 0, len(lines))
	prevBlank := false
	for i, line := range lines {
		blank := len(line.Graphemes) == 0
		if blank && prevBlank {
			continue
		}
		squeezed = append(squeezed, line)
		lineMap = append(lineMap, i)
		prevBlank = blank
	}
	return squeezed, lineMap
}

func (v *Viewer) drawPrompt(screen tcell.Screen, y int) {
	style := v.cfg.Chrome.PromptStyle
	prompt := ""
	if v.prompt != nil {
		prompt = " " + v.promptText()
	}
	screen.PutStrStyled(0, y, padRightToWidth(prompt, v.width), style)
	if v.prompt != nil && v.prompt.errText != "" && v.width > 0 && !strings.Contains(prompt, v.prompt.errText) {
		errText := "  " + v.prompt.errText
		paddedPrompt := truncateToWidth(prompt, v.width)
		start := stringWidth(paddedPrompt)
		if start < v.width {
			screen.PutStrStyled(start, y, truncateToWidth(errText, v.width-start), v.cfg.Chrome.PromptErrorStyle)
		}
	}
}

func (v *Viewer) promptText() string {
	if v.prompt == nil {
		return ""
	}
	defaultText := v.prompt.String()
	if v.text.PromptLine == nil {
		return defaultText
	}
	return v.text.PromptLine(PromptInfo{
		Kind:        toPromptKind(v.prompt.kind),
		Prefix:      v.prompt.prefix,
		Input:       string(v.prompt.buffer),
		Error:       v.prompt.errText,
		Search:      v.SearchSnapshot(),
		DefaultText: defaultText,
	})
}

func toPromptKind(kind promptKind) PromptKind {
	switch kind {
	case promptSearchBackward:
		return PromptKindSearchBackward
	case promptCommand:
		return PromptKindCommand
	default:
		return PromptKindSearchForward
	}
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
	a := v.keys.helpAction(ev)
	switch a {
	case actionQuit:
		return KeyResult{Handled: true, Quit: true, Action: KeyActionQuit, Context: KeyContextHelp}
	case actionToggleHelp:
		v.toggleHelp()
		return KeyResult{Handled: true, Action: KeyActionToggleHelp, Context: KeyContextHelp}
	case actionCycleSearchCase:
		v.CycleSearchCaseMode()
		return KeyResult{Handled: true, Action: KeyActionCycleSearchCase, Context: KeyContextHelp}
	case actionCycleSearchMode:
		v.CycleSearchMode()
		return KeyResult{Handled: true, Action: KeyActionCycleSearchMode, Context: KeyContextHelp}
	case actionScrollUp:
		v.helpOffset--
	case actionScrollDown:
		v.helpOffset++
	case actionPageUp:
		v.helpOffset -= max(v.height-2, 1)
	case actionPageDown:
		v.helpOffset += max(v.height-2, 1)
	case actionHalfPageUp:
		v.helpOffset -= max((v.height-2)/2, 1)
	case actionHalfPageDown:
		v.helpOffset += max((v.height-2)/2, 1)
	case actionGoTop:
		v.helpOffset = 0
	case actionGoBottom:
		v.helpOffset = v.maxHelpOffset()
	default:
		return KeyResult{}
	}
	v.clampHelpOffset()
	return KeyResult{Handled: true, Action: KeyAction(a), Context: KeyContextHelp}
}

func (v *Viewer) handlePromptKey(ev *tcell.EventKey) KeyResult {
	if result, ok := v.handlePromptMappedAction(v.keys.promptAction(ev)); ok {
		return result
	}

	switch ev.Key() {
	case tcell.KeyEscape:
		v.cancelPrompt()
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	case tcell.KeyEnter:
		return v.commitPrompt()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if v.prompt != nil && len(v.prompt.buffer) > 0 {
			v.prompt.buffer = v.prompt.buffer[:len(v.prompt.buffer)-1]
		}
		v.updatePromptPreview()
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	case tcell.KeyCtrlU:
		if v.prompt != nil {
			v.prompt.buffer = v.prompt.buffer[:0]
		}
		v.updatePromptPreview()
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	case tcell.KeyRune:
		if v.prompt != nil {
			v.prompt.buffer = append(v.prompt.buffer, []rune(ev.Str())...)
		}
		v.updatePromptPreview()
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	}
	return KeyResult{}
}

func (v *Viewer) handlePromptMappedAction(a action) (KeyResult, bool) {
	switch a {
	case actionQuit:
		return KeyResult{Handled: true, Quit: true, Action: KeyActionQuit, Context: KeyContextPrompt}, true
	case actionCycleSearchCase:
		v.CycleSearchCaseMode()
		return KeyResult{Handled: true, Action: KeyActionCycleSearchCase, Context: KeyContextPrompt}, true
	case actionCycleSearchMode:
		v.CycleSearchMode()
		return KeyResult{Handled: true, Action: KeyActionCycleSearchMode, Context: KeyContextPrompt}, true
	default:
		return KeyResult{}, false
	}
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

func (v *Viewer) toTCellStyle(style ansi.Style) tcell.Style {
	tstyle := tcell.StyleDefault
	tstyle = tstyle.Foreground(v.cfg.Theme.resolveColor(style.Fg, true))
	tstyle = tstyle.Background(v.cfg.Theme.resolveColor(style.Bg, false))
	tstyle = tstyle.Bold(style.Bold)
	tstyle = tstyle.Dim(style.Dim)
	tstyle = tstyle.Italic(style.Italic)
	if style.Underline != ansi.UnderlineStyleNone {
		ulStyle := toTCellUnderlineStyle(style.Underline)
		if style.UnderlineColor.Kind == ansi.ColorDefault {
			tstyle = tstyle.Underline(ulStyle)
		} else {
			tstyle = tstyle.Underline(ulStyle, v.cfg.Theme.resolveColor(style.UnderlineColor, true))
		}
	}
	tstyle = tstyle.StrikeThrough(style.Strike)
	tstyle = tstyle.Blink(style.Blink)
	tstyle = tstyle.Reverse(style.Reverse)
	return tstyle
}

func toTCellUnderlineStyle(style ansi.UnderlineStyle) tcell.UnderlineStyle {
	switch style {
	case ansi.UnderlineStyleSolid:
		return tcell.UnderlineStyleSolid
	case ansi.UnderlineStyleDouble:
		return tcell.UnderlineStyleDouble
	case ansi.UnderlineStyleCurly:
		return tcell.UnderlineStyleCurly
	case ansi.UnderlineStyleDotted:
		return tcell.UnderlineStyleDotted
	case ansi.UnderlineStyleDashed:
		return tcell.UnderlineStyleDashed
	default:
		return tcell.UnderlineStyleNone
	}
}

func (v *Viewer) visualizationStyle(base tcell.Style) tcell.Style {
	overlay := v.cfg.Visualization.Style
	style := overlay
	if style.GetForeground() == tcolor.Default {
		style = style.Foreground(base.GetForeground())
	}
	if style.GetBackground() == tcolor.Default {
		style = style.Background(base.GetBackground())
	}
	style = style.Attributes(base.GetAttributes() | overlay.GetAttributes())
	if overlay.GetUnderlineStyle() == tcell.UnderlineStyleNone && base.GetUnderlineStyle() != tcell.UnderlineStyleNone {
		style = style.Underline(base.GetUnderlineStyle(), base.GetUnderlineColor())
	}
	return style
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

func trimLeftToWidth(s string, skip int) string {
	if skip <= 0 {
		return s
	}

	gr := uniseg.NewGraphemes(s)
	consumed := 0
	for gr.Next() {
		cluster := gr.Str()
		clusterWidth := uniseg.StringWidth(cluster)
		if consumed+clusterWidth > skip {
			hidden := skip - consumed
			start, end := gr.Positions()
			if hidden == 0 {
				return s[start:]
			}
			return strings.Repeat(" ", max(clusterWidth-hidden, 0)) + s[end:]
		}
		consumed += clusterWidth
	}
	return ""
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

func frameTitleLabel(title string, width int, align TitleAlign) (label string, x int) {
	if title == "" || width <= 2 {
		return "", 0
	}
	label = " " + title + " "
	if stringWidth(label) > width-2 {
		label = truncateToWidth(label, width-2)
	}
	labelWidth := stringWidth(label)
	switch align {
	case TitleAlignCenter:
		x = max((width-labelWidth)/2, 1)
	case TitleAlignRight:
		x = max(width-1-labelWidth, 1)
	default:
		x = 1
	}
	if x+labelWidth > width-1 {
		x = max(width-1-labelWidth, 1)
	}
	return label, x
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

func padMarkerGlyph(glyph string, width int) string {
	return padRightToWidth(glyph, width)
}
