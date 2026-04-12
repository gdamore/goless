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
	"github.com/gdamore/tcell/v3/color"
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
	doc           *model.Document
	cfg           Config
	mode          viewerMode
	prompt        *promptState
	history       [promptHistoryKindCount][]string
	message       string
	clearMessage  bool
	search        searchState
	text          Text
	keys          keyMap
	sourceLines   []model.Line
	lines         []model.Line
	lineMap       []int
	layout        layout.Result
	maxColumns    int
	width         int
	height        int
	rowOffset     int
	colOffset     int
	follow        bool
	overlay       *informationOverlay
	helpOffset    int
	helpColOffset int
}

// KeyResult summarizes how the viewer handled a key event.
type KeyResult struct {
	Handled bool
	Quit    bool
	Action  KeyAction
	Context KeyContext
}

// Position summarizes the current logical viewport state.
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

// SetTabWidth updates tab expansion during layout.
func (v *Viewer) SetTabWidth(width int) {
	if width <= 0 {
		width = 8
	}
	if v.cfg.TabWidth == width {
		return
	}
	v.ensureLayout()
	anchor := v.firstVisibleAnchor()
	v.cfg.TabWidth = width
	v.relayout()
	if v.follow {
		v.rowOffset = v.maxRowOffset()
		v.clampOffsets()
		return
	}
	v.restoreAnchor(anchor)
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
	v.maxColumns = v.computeMaxContentColumns()
	v.clampOffsets()
	v.relayout()
}

// SetHyperlinkHandler updates how parsed OSC 8 hyperlinks are rendered.
func (v *Viewer) SetHyperlinkHandler(handler HyperlinkHandler) {
	v.cfg.HyperlinkHandler = handler
}

// SetCommandHandler updates how unknown ':' commands are handled.
func (v *Viewer) SetCommandHandler(handler CommandHandler) {
	v.cfg.CommandHandler = handler
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

// SetShowStatus updates whether the status bar is shown.
func (v *Viewer) SetShowStatus(enabled bool) {
	if v.cfg.ShowStatus == enabled {
		return
	}
	v.ensureLayout()
	anchor := v.firstVisibleAnchor()
	v.cfg.ShowStatus = enabled
	v.relayout()
	v.clampHelpOffset()
	if v.follow {
		v.rowOffset = v.maxRowOffset()
		v.clampOffsets()
		return
	}
	v.restoreAnchor(anchor)
}

// SetDocument swaps the backing document while preserving the current viewer
// configuration and viewport offsets when possible.
func (v *Viewer) SetDocument(doc *model.Document) {
	if doc == nil {
		return
	}
	v.doc = doc
	v.relayout()
	if v.cfg.WrapMode == layout.NoWrap {
		v.relayout()
	}
	if v.follow {
		v.rowOffset = v.maxRowOffset()
		v.clampOffsets()
	}
}

// ShowInformation replaces the document view with a scrollable information overlay.
func (v *Viewer) ShowInformation(title, body string) {
	v.overlay = &informationOverlay{
		title: title,
		body:  body,
	}
	v.mode = modeHelp
	v.helpOffset = 0
	v.helpColOffset = 0
}

// HideInformation closes the current information overlay if one is visible.
func (v *Viewer) HideInformation() {
	v.overlay = nil
}

// ShowingInformation reports whether an information overlay is visible.
func (v *Viewer) ShowingInformation() bool {
	return v.overlay != nil
}

// SetText updates help text, status text, prompt text, and UI strings.
func (v *Viewer) SetText(text Text) {
	text = text.withDefaults()
	v.cfg.Text = text
	v.text = text
	v.clampHelpOffset()
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
		v.setTransientMessage(v.search.CompileError)
		return
	}
	if len(v.search.Matches) == 0 {
		v.setTransientMessage(v.text.SearchNotFound(v.search.Query))
		return
	}
	if v.search.Current < 0 || v.search.Current >= len(v.search.Matches) {
		v.search.Current = v.pickInitialMatch(v.search.Forward)
	}
	v.goToMatch(v.search.Current)
	v.setMessage(v.text.SearchMatchCount(v.search.Query, len(v.search.Matches)))
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
		v.setTransientMessage(v.search.CompileError)
		return
	}
	if len(v.search.Matches) == 0 {
		v.setTransientMessage(v.text.SearchNotFound(v.search.Query))
		return
	}
	if v.search.Current < 0 || v.search.Current >= len(v.search.Matches) {
		v.search.Current = v.pickInitialMatch(v.search.Forward)
	}
	v.goToMatch(v.search.Current)
	v.setMessage(v.text.SearchMatchCount(v.search.Query, len(v.search.Matches)))
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
	if v.follow {
		v.rowOffset = v.maxRowOffset()
		v.clampOffsets()
	}
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
	screen.HideCursor()
	screen.SetCursorStyle(tcell.CursorStyleDefault)

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
	for y := range bodyHeight {
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

// HandleMouse applies a mouse event and reports whether it was handled.
func (v *Viewer) HandleMouse(ev *tcell.EventMouse) bool {
	return v.HandleMouseResult(ev).Handled
}

// HandleKeyResult applies a key event and reports whether it was handled and whether the viewer should exit.
func (v *Viewer) HandleKeyResult(ev *tcell.EventKey) KeyResult {
	v.consumeTransientMessage()

	if v.follow && ev.Key() == tcell.KeyCtrlC {
		ctx := KeyContextNormal
		switch v.mode {
		case modeHelp:
			ctx = KeyContextHelp
		case modePrompt:
			ctx = KeyContextPrompt
		}
		v.StopFollow()
		return KeyResult{Handled: true, Action: KeyActionStopFollow, Context: ctx}
	}
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
	case actionScrollUpStep:
		v.ScrollUp(v.verticalScrollStep())
	case actionScrollDownStep:
		v.ScrollDown(v.verticalScrollStep())
	case actionScrollLeft:
		v.ScrollLeft(v.horizontalScrollStep())
	case actionScrollRight:
		v.ScrollRight(v.horizontalScrollStep())
	case actionScrollLeftFine:
		v.ScrollLeft(1)
	case actionScrollRightFine:
		v.ScrollRight(1)
	case actionHalfScreenLeft:
		v.ScrollLeft(v.halfHorizontalScrollStep())
	case actionHalfScreenRight:
		v.ScrollRight(v.halfHorizontalScrollStep())
	case actionHalfPageUp:
		v.HalfPageUp()
	case actionHalfPageDown:
		v.HalfPageDown()
	case actionPageUp:
		v.PageUp()
	case actionPageDown:
		v.PageDown()
	case actionGoLineStart:
		v.GoLineStart()
	case actionGoLineEnd:
		v.GoLineEnd()
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
	case actionRefresh:
	case actionToggleHelp:
		v.toggleHelp()
	case actionFollow:
		v.Follow()
	case actionStopFollow:
		v.StopFollow()
	case actionCycleSearchCase:
		v.CycleSearchCaseMode()
	case actionCycleSearchMode:
		v.CycleSearchMode()
	default:
		return KeyResult{}
	}

	return KeyResult{Handled: true, Action: actionToKeyAction(a), Context: KeyContextNormal}
}

// HandleMouseResult applies a mouse event and reports whether it was handled.
func (v *Viewer) HandleMouseResult(ev *tcell.EventMouse) MouseResult {
	if v.mode == modeHelp {
		return v.handleHelpMouse(ev)
	}
	if v.mode == modePrompt {
		return MouseResult{Context: KeyContextPrompt}
	}

	a := wheelAction(ev.Buttons())
	switch a {
	case actionScrollUp:
		v.ScrollUp(1)
	case actionScrollDown:
		v.ScrollDown(1)
	case actionScrollLeft:
		v.ScrollLeft(v.horizontalScrollStep())
	case actionScrollRight:
		v.ScrollRight(v.horizontalScrollStep())
	default:
		return MouseResult{Context: KeyContextNormal}
	}

	return MouseResult{Handled: true, Action: actionToKeyAction(a), Context: KeyContextNormal}
}

func (v *Viewer) consumeTransientMessage() {
	if !v.clearMessage {
		return
	}
	v.message = ""
	v.clearMessage = false
}

func (v *Viewer) setMessage(text string) {
	v.message = text
	v.clearMessage = false
}

func (v *Viewer) setTransientMessage(text string) {
	v.message = text
	v.clearMessage = text != ""
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
	v.ensureLayout()
	v.colOffset += max(n, 0)
	if maxOffset := v.maxColOffset(); v.colOffset > maxOffset {
		v.colOffset = maxOffset
	}
	v.relayout()
}

// ScrollLeft moves the viewport left in no-wrap mode.
func (v *Viewer) ScrollLeft(n int) {
	if v.cfg.WrapMode != layout.NoWrap {
		return
	}
	v.ensureLayout()
	v.colOffset -= max(n, 0)
	if v.colOffset < 0 {
		v.colOffset = 0
	}
	v.relayout()
}

func (v *Viewer) horizontalScrollStep() int {
	return max(1, min(8, v.bodyContentWidth()/4))
}

func (v *Viewer) verticalScrollStep() int {
	return max(1, min(8, v.scrollBodyHeight()/8))
}

func (v *Viewer) helpVerticalScrollStep() int {
	return max(1, min(8, v.helpPageStep()/8))
}

func (v *Viewer) halfHorizontalScrollStep() int {
	return max(v.bodyContentWidth()/2, 1)
}

// GoLineStart moves to the beginning of the current horizontal line in no-wrap mode.
func (v *Viewer) GoLineStart() {
	if v.cfg.WrapMode != layout.NoWrap {
		return
	}
	v.colOffset = 0
	v.relayout()
}

// GoLineEnd moves to the furthest reachable horizontal position in no-wrap mode.
func (v *Viewer) GoLineEnd() {
	if v.cfg.WrapMode != layout.NoWrap {
		return
	}
	v.ensureLayout()
	v.colOffset = v.maxColOffset()
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

// StopFollow disables follow mode while leaving the current viewport in place.
func (v *Viewer) StopFollow() {
	v.follow = false
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
	if !ok || rowIndex != len(v.layout.Rows)-1 {
		return false
	}
	row := v.layout.Rows[rowIndex]
	if row.LineIndex < 0 || row.LineIndex >= len(v.layout.Lines) {
		return false
	}
	return row.SourceCellEnd >= v.layout.Lines[row.LineIndex].TotalCells
}

// Position reports the current logical line, total logical line count, current
// logical display column, and maximum logical content span.
func (v *Viewer) Position() Position {
	v.ensureLayout()
	rowIndex, ok := v.positionRowIndex()
	return Position{
		Row:     v.logicalRowNumber(rowIndex, ok),
		Rows:    len(v.sourceLines),
		Column:  v.logicalColumnNumber(rowIndex, ok),
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
	v.maxColumns = v.computeMaxContentColumns()
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
	return max(v.maxScrollableColumns()-max(v.bodyContentWidth(), 1), 0)
}

func (v *Viewer) bodyHeight() int {
	_, _, _, h := v.contentRect()
	return h
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
	for y := range bodyHeight {
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
	iconOnStyle := v.cfg.Chrome.StatusIconOnStyle
	iconOffStyle := v.cfg.Chrome.StatusIconOffStyle
	screen.PutStrStyled(0, y, strings.Repeat(" ", max(v.width, 0)), style)

	leftOverflow, rightOverflow := v.statusOverflow()
	leftText, rightText := v.statusText()
	canScrollUp, canScrollDown := v.statusScrollable()

	rightIcons := make([]statusIcon, 0, 5)
	rightIcons = append(rightIcons, statusIcon{
		on:     v.statusModeHint(),
		off:    " ",
		active: true,
	})
	rightIcons = append(rightIcons, statusIcon{
		on:     v.text.StatusEOF,
		off:    v.text.StatusNotEOF,
		active: v.EOFVisible(),
	})
	rightIcons = append(rightIcons, statusIcon{
		on:     v.text.LeftOverflowOn,
		off:    v.text.LeftOverflowOff,
		active: leftOverflow,
	})
	rightIcons = append(rightIcons, statusIcon{
		on:     v.text.TopScrollableOn,
		off:    v.text.TopScrollableOff,
		active: canScrollUp,
	})
	rightIcons = append(rightIcons, statusIcon{
		on:     v.text.BottomScrollableOn,
		off:    v.text.BottomScrollableOff,
		active: canScrollDown,
	})
	if v.cfg.WrapMode == layout.NoWrap {
		rightIcons = append(rightIcons, statusIcon{
			on:     v.text.RightOverflowOn,
			off:    v.text.RightOverflowOff,
			active: rightOverflow,
		})
	}
	leftStart := 1
	rightIconsWidth := statusIconsWidth(rightIcons)
	rightIconsStart := max(v.width-rightIconsWidth, leftStart)
	rightTextLimit := max(rightIconsStart-1, leftStart)
	rightTextWidth := min(stringWidth(rightText), max(rightTextLimit-leftStart, 0))
	rightText = truncateToWidth(rightText, rightTextWidth)
	rightStart := max(rightTextLimit-stringWidth(rightText), leftStart)

	leftWidth := max(rightStart-leftStart-1, 0)
	if leftWidth > 0 {
		leftRendered := truncateToWidth(leftText, leftWidth)
		screen.PutStrStyled(leftStart, y, leftRendered, style)
		if keyText, ok := v.statusHelpHintKey(leftText, leftRendered); ok {
			screen.PutStrStyled(leftStart, y, keyText, v.cfg.Chrome.StatusHelpKeyStyle)
		}
		if v.follow {
			v.drawHintKeys(screen, leftStart, y, leftRendered, v.cfg.Chrome.StatusHelpKeyStyle, "Ctrl-C", "^C")
		}
	}
	if rightText != "" && rightStart < rightIconsStart {
		screen.PutStrStyled(rightStart, y, rightText, style)
		v.drawHintKeys(screen, rightStart, y, rightText, v.cfg.Chrome.StatusHelpKeyStyle, "F2:", "F3:")
	}
	if rightIconsWidth > 0 && rightIconsStart < v.width {
		drawStatusIcons(screen, rightIconsStart, y, rightIcons, iconOnStyle, iconOffStyle, style, v.width-rightIconsStart)
	}
}

func (v *Viewer) statusHelpHintKey(fullText, rendered string) (string, bool) {
	if fullText == "" || fullText != v.text.StatusHelpHint || rendered == "" {
		return "", false
	}
	key, _, ok := strings.Cut(fullText, " ")
	if !ok {
		key = fullText
	}
	key = truncateToWidth(key, stringWidth(rendered))
	if key == "" {
		return "", false
	}
	return key, true
}

func (v *Viewer) drawHintKeys(screen tcell.Screen, x, y int, text string, style tcell.Style, keys ...string) {
	for _, key := range keys {
		if key == "" {
			continue
		}
		idx := strings.Index(text, key)
		if idx < 0 {
			continue
		}
		keyX := x + stringWidth(text[:idx])
		screen.PutStrStyled(keyX, y, key, style)
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
	for y := range gutterHeight {
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
		label := padRightToWidth(fmt.Sprintf("%*d", gutterWidth-1, v.sourceLineIndex(row.LineIndex)+1), gutterWidth)
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
	if fg := overlay.GetForeground(); fg != color.Default {
		base = base.Foreground(fg)
	}
	if bg := overlay.GetBackground(); bg != color.Default {
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
		if uc := overlay.GetUnderlineColor(); uc != color.Default {
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
	digits := len(strconv.Itoa(max(len(v.sourceLines), 1)))
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
	height = max(v.height-v.bottomBarRows(), 0)

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
	if search.Query != "" {
		position := 0
		if len(search.Matches) > 0 && search.Current >= 0 {
			position = search.Current + 1
		}
		left = v.text.StatusSearchInfo(search.Query, position, len(search.Matches))
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
	if left == "" {
		left = v.text.StatusHelpHint
	}

	rowIndex, ok := v.positionRowIndex()
	current := v.logicalRowNumber(rowIndex, ok)
	column := v.logicalColumnNumber(rowIndex, ok)
	right = v.text.StatusPosition(current, len(v.sourceLines), column, v.maxContentColumns())
	if v.text.StatusLine != nil {
		return v.text.StatusLine(StatusInfo{
			Search:     v.SearchSnapshot(),
			Following:  v.follow,
			EOFVisible: v.EOFVisible(),
			Message:    v.message,
			Position: Position{
				Row:     current,
				Rows:    len(v.sourceLines),
				Column:  column,
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

func (v *Viewer) statusScrollable() (up, down bool) {
	v.ensureLayout()
	maxOffset := v.maxRowOffset()
	return v.rowOffset > 0, v.rowOffset < maxOffset
}

type statusIcon struct {
	on     string
	off    string
	active bool
}

func (i statusIcon) text() string {
	if i.active {
		return i.on
	}
	return i.off
}

func (i statusIcon) width() int {
	return max(stringWidth(i.on), stringWidth(i.off))
}

func statusIconsWidth(icons []statusIcon) int {
	if len(icons) == 0 {
		return 0
	}
	width := 1
	for _, icon := range icons {
		width += icon.width()
	}
	if len(icons) > 1 {
		width += len(icons) - 1
	}
	return width
}

func drawStatusIcons(screen tcell.Screen, x, y int, icons []statusIcon, onStyle, offStyle, sepStyle tcell.Style, maxWidth int) {
	used := 0
	for i, icon := range icons {
		iconWidth := icon.width()
		if iconWidth <= 0 || used >= maxWidth {
			continue
		}
		text := truncateToWidth(icon.text(), min(iconWidth, maxWidth-used))
		iconStyle := offStyle
		if icon.active {
			iconStyle = onStyle
		}
		screen.PutStrStyled(x+used, y, text, iconStyle)
		used += iconWidth
		if i < len(icons)-1 {
			if used >= maxWidth {
				return
			}
			screen.PutStrStyled(x+used, y, " ", sepStyle)
			used++
		}
	}
	if used < maxWidth {
		screen.PutStrStyled(x+used, y, " ", sepStyle)
	}
}

func (v *Viewer) statusOverflow() (left, right bool) {
	if v.cfg.WrapMode != layout.NoWrap {
		return false, false
	}

	return v.colOffset > 0, v.colOffset < v.maxColOffset()
}

func (v *Viewer) maxContentColumns() int {
	return v.maxColumns
}

func (v *Viewer) maxScrollableColumns() int {
	return max(v.maxContentColumns()-v.headerColumnWidth(v.rawContentWidth()), 0)
}

func (v *Viewer) computeMaxContentColumns() int {
	maxCells := 0
	for i, line := range v.layout.Lines {
		totalCells := line.TotalCells + v.trailingMarkerCellWidth(i)
		if totalCells > maxCells {
			maxCells = totalCells
		}
	}
	return maxCells
}

func (v *Viewer) positionRowIndex() (int, bool) {
	v.ensureLayout()
	if len(v.layout.Rows) == 0 {
		return 0, false
	}
	rowIndex := v.scrollableRowStartIndex() + v.rowOffset
	if rowIndex >= 0 && rowIndex < len(v.layout.Rows) {
		return rowIndex, true
	}
	rowIndex, _, ok := v.visibleLayoutRowAt(0)
	if !ok || rowIndex < 0 {
		return 0, false
	}
	return rowIndex, true
}

func (v *Viewer) logicalRowNumber(rowIndex int, ok bool) int {
	if !ok || rowIndex < 0 || rowIndex >= len(v.layout.Rows) {
		return 0
	}
	return v.sourceLineIndex(v.layout.Rows[rowIndex].LineIndex) + 1
}

func (v *Viewer) logicalColumnNumber(rowIndex int, ok bool) int {
	if !ok || rowIndex < 0 || rowIndex >= len(v.layout.Rows) {
		return 0
	}
	columns := v.maxContentColumns()
	if columns <= 0 {
		return 0
	}
	return min(v.layout.Rows[rowIndex].SourceCellStart+1, columns)
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
	screen.PutStrStyled(0, y, padRightToWidth("", v.width), style)

	prompt := ""
	cursorX := -1
	if v.prompt != nil {
		if left, right, leftCursorX, ok := v.builtInSearchPromptDisplay(v.width); ok {
			prompt = left
			cursorX = leftCursorX
			screen.PutStrStyled(0, y, truncateToWidth(left, v.width), style)
			if right != "" {
				right = truncateToWidth(right, v.width)
				start := max(v.width-stringWidth(right), 0)
				screen.PutStrStyled(start, y, right, style)
				v.drawHintKeys(screen, start, y, right, v.cfg.Chrome.StatusHelpKeyStyle, "F2:", "F3:")
			}
		} else if left, right, leftCursorX, ok := v.builtInSavePromptDisplay(v.width); ok {
			prompt = left
			cursorX = leftCursorX
			screen.PutStrStyled(0, y, truncateToWidth(left, v.width), style)
			if right != "" {
				right = truncateToWidth(right, v.width)
				start := max(v.width-stringWidth(right), 0)
				screen.PutStrStyled(start, y, right, style)
				v.drawHintKeys(screen, start, y, right, v.cfg.Chrome.StatusHelpKeyStyle, "F2:", "F3:")
			}
		} else {
			prompt, cursorX = v.promptDisplay(v.width)
			screen.PutStrStyled(0, y, padRightToWidth(prompt, v.width), style)
		}
	}
	if v.prompt != nil && v.prompt.errText != "" && v.width > 0 && v.text.PromptLine == nil {
		errText := "  " + v.prompt.errText
		paddedPrompt := truncateToWidth(prompt, v.width)
		start := stringWidth(paddedPrompt)
		if start < v.width {
			screen.PutStrStyled(start, y, truncateToWidth(errText, v.width-start), v.cfg.Chrome.PromptErrorStyle)
		}
	}
	if v.prompt != nil && v.text.PromptLine == nil && cursorX >= 0 && cursorX < v.width {
		cursorStyle := tcell.CursorStyleSteadyBar
		if v.prompt.editor.Overwrite() {
			cursorStyle = tcell.CursorStyleSteadyBlock
		}
		screen.SetCursorStyle(cursorStyle)
		screen.ShowCursor(cursorX, y)
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
		Input:       v.prompt.input(),
		Cursor:      v.prompt.cursor(),
		Overwrite:   v.prompt.editor.Overwrite(),
		Seeded:      v.prompt.seeded,
		Error:       v.prompt.errText,
		Search:      v.SearchSnapshot(),
		DefaultText: defaultText,
	})
}

func (v *Viewer) builtInSearchPromptDisplay(width int) (left, right string, cursorX int, ok bool) {
	if v.prompt == nil || v.text.PromptLine != nil {
		return "", "", -1, false
	}
	switch v.prompt.kind {
	case promptSearchForward:
		left, cursorX = v.builtInSearchPromptLeft(width, " /")
	case promptSearchBackward:
		left, cursorX = v.builtInSearchPromptLeft(width, " ?")
	default:
		return "", "", -1, false
	}
	if v.prompt.errText == "" {
		right = " " + v.searchModeHintText() + " "
		rightWidth := stringWidth(right)
		if rightWidth >= width {
			right = ""
		} else if stringWidth(left) > width-rightWidth {
			left, cursorX = v.builtInSearchPromptLeft(width-rightWidth, left[:2])
		}
	}
	return left, right, cursorX, true
}

func (v *Viewer) builtInSearchPromptLeft(width int, prefix string) (string, int) {
	return v.layoutPromptInput(prefix, width)
}

func (v *Viewer) builtInSavePromptDisplay(width int) (left, right string, cursorX int, ok bool) {
	if v.prompt == nil || v.text.PromptLine != nil || v.prompt.kind != promptSave {
		return "", "", -1, false
	}
	prefix := " " + v.prompt.prefix
	left, cursorX = v.layoutPromptInput(prefix, width)
	if v.prompt.errText != "" {
		return left, "", cursorX, true
	}
	if hint := v.saveModeHintText(); hint != "" {
		right = " " + hint + " "
	}
	rightWidth := stringWidth(right)
	if rightWidth >= width {
		right = ""
	} else if stringWidth(left) > width-rightWidth {
		left, cursorX = v.layoutPromptInput(prefix, width-rightWidth)
	}
	return left, right, cursorX, true
}

func (v *Viewer) displayPromptText(width int) string {
	text, _ := v.promptDisplay(width)
	return text
}

func (v *Viewer) promptDisplay(width int) (string, int) {
	if v.prompt == nil {
		return "", -1
	}
	if v.text.PromptLine != nil {
		return " " + v.promptText(), -1
	}

	prefix := " " + v.prompt.prefix
	return v.layoutPromptInput(prefix, width)
}

func toPromptKind(kind promptKind) PromptKind {
	switch kind {
	case promptSearchBackward:
		return PromptKindSearchBackward
	case promptCommand:
		return PromptKindCommand
	case promptSave:
		return PromptKindSave
	default:
		return PromptKindSearchForward
	}
}

func (v *Viewer) helpFrameTitle() string {
	title := v.informationTitle()
	if v.text.HelpClose != "" {
		if title != "" {
			title += "  "
		}
		title += v.text.HelpClose
	}
	return title
}

func (v *Viewer) helpPageStep() int {
	_, _, _, bodyHeight := v.contentRect()
	return max(bodyHeight, 1)
}

func wheelAction(buttons tcell.ButtonMask) action {
	switch {
	case buttons&tcell.WheelUp != 0:
		return actionScrollUp
	case buttons&tcell.WheelDown != 0:
		return actionScrollDown
	case buttons&tcell.WheelLeft != 0:
		return actionScrollLeft
	case buttons&tcell.WheelRight != 0:
		return actionScrollRight
	default:
		return actionNone
	}
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
	case actionRefresh:
	case actionScrollUp:
		v.helpOffset--
	case actionScrollDown:
		v.helpOffset++
	case actionScrollUpStep:
		v.helpOffset -= v.helpVerticalScrollStep()
	case actionScrollDownStep:
		v.helpOffset += v.helpVerticalScrollStep()
	case actionScrollLeft:
		v.helpColOffset -= v.horizontalScrollStep()
	case actionScrollRight:
		v.helpColOffset += v.horizontalScrollStep()
	case actionScrollLeftFine:
		v.helpColOffset--
	case actionScrollRightFine:
		v.helpColOffset++
	case actionHalfScreenLeft:
		v.helpColOffset -= v.halfHorizontalScrollStep()
	case actionHalfScreenRight:
		v.helpColOffset += v.halfHorizontalScrollStep()
	case actionPageUp:
		v.helpOffset -= v.helpPageStep()
	case actionPageDown:
		v.helpOffset += v.helpPageStep()
	case actionHalfPageUp:
		v.helpOffset -= max(v.helpPageStep()/2, 1)
	case actionHalfPageDown:
		v.helpOffset += max(v.helpPageStep()/2, 1)
	case actionGoLineStart:
		v.helpColOffset = 0
	case actionGoLineEnd:
		v.helpColOffset = v.helpLineMaxColOffset(v.helpOffset)
	case actionGoTop:
		v.helpOffset = 0
	case actionGoBottom:
		v.helpOffset = v.maxHelpOffset()
	default:
		return KeyResult{Context: KeyContextHelp}
	}
	v.clampHelpOffset()
	return KeyResult{Handled: true, Action: actionToKeyAction(a), Context: KeyContextHelp}
}

func (v *Viewer) handleHelpMouse(ev *tcell.EventMouse) MouseResult {
	a := wheelAction(ev.Buttons())
	switch a {
	case actionScrollUp:
		v.helpOffset--
	case actionScrollDown:
		v.helpOffset++
	case actionScrollLeft:
		v.helpColOffset -= v.horizontalScrollStep()
	case actionScrollRight:
		v.helpColOffset += v.horizontalScrollStep()
	default:
		return MouseResult{Context: KeyContextHelp}
	}
	v.clampHelpOffset()
	return MouseResult{Handled: true, Action: actionToKeyAction(a), Context: KeyContextHelp}
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
	case tcell.KeyLeft, tcell.KeyCtrlB:
		v.clearPromptErrorOnEdit()
		v.promptMoveLeft()
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	case tcell.KeyRight, tcell.KeyCtrlF:
		v.clearPromptErrorOnEdit()
		v.promptMoveRight()
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	case tcell.KeyHome, tcell.KeyCtrlA:
		if v.prompt != nil {
			v.clearPromptErrorOnEdit()
			v.prompt.seeded = false
			v.prompt.editor.MoveHome()
		}
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	case tcell.KeyEnd, tcell.KeyCtrlE:
		if v.prompt != nil {
			v.clearPromptErrorOnEdit()
			v.prompt.seeded = false
			v.prompt.editor.MoveEnd()
		}
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	case tcell.KeyUp:
		v.clearPromptErrorOnEdit()
		v.recallPromptHistory(-1)
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	case tcell.KeyDown:
		v.clearPromptErrorOnEdit()
		v.recallPromptHistory(1)
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	case tcell.KeyInsert:
		if v.prompt != nil {
			v.clearPromptErrorOnEdit()
			v.prompt.seeded = false
			v.prompt.editor.ToggleOverwrite()
		}
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if v.prompt != nil {
			v.clearPromptErrorOnEdit()
			if v.prompt.seeded {
				v.prompt.editor.Clear()
				v.prompt.seeded = false
			} else {
				v.prompt.editor.Backspace()
			}
		}
		v.updatePromptPreview()
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	case tcell.KeyDelete:
		if v.prompt != nil {
			v.clearPromptErrorOnEdit()
			if v.prompt.seeded {
				v.prompt.editor.Clear()
				v.prompt.seeded = false
			} else {
				v.prompt.editor.Delete()
			}
		}
		v.updatePromptPreview()
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	case tcell.KeyCtrlK:
		if v.prompt != nil {
			v.clearPromptErrorOnEdit()
			v.prompt.seeded = false
			v.prompt.editor.DeleteToEnd()
		}
		v.updatePromptPreview()
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	case tcell.KeyCtrlU:
		if v.prompt != nil {
			v.clearPromptErrorOnEdit()
			v.prompt.seeded = false
			v.prompt.editor.DeleteToStart()
		}
		v.updatePromptPreview()
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	case tcell.KeyCtrlW:
		if v.prompt != nil {
			v.clearPromptErrorOnEdit()
			v.prompt.seeded = false
			v.prompt.editor.DeleteWordBackward()
		}
		v.updatePromptPreview()
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	case tcell.KeyRune:
		if v.prompt != nil {
			v.clearPromptErrorOnEdit()
			if v.prompt.seeded {
				v.prompt.editor.SetText(ev.Str())
				v.prompt.seeded = false
			} else {
				v.prompt.editor.Insert(ev.Str())
			}
		}
		v.updatePromptPreview()
		return KeyResult{Handled: true, Context: KeyContextPrompt}
	}
	return KeyResult{Context: KeyContextPrompt}
}

func (v *Viewer) clearPromptErrorOnEdit() {
	if v.prompt == nil {
		return
	}
	if v.prompt.kind == promptSave {
		v.prompt.errText = ""
	}
}

func (v *Viewer) promptMoveLeft() {
	if v.prompt == nil {
		return
	}
	if v.prompt.seeded {
		v.prompt.seeded = false
		v.prompt.editor.MoveHome()
		return
	}
	v.prompt.editor.MoveLeft()
}

func (v *Viewer) promptMoveRight() {
	if v.prompt == nil {
		return
	}
	if v.prompt.seeded {
		v.prompt.seeded = false
		v.prompt.editor.MoveEnd()
		return
	}
	v.prompt.editor.MoveRight()
}

func (v *Viewer) layoutPromptInput(prefix string, width int) (string, int) {
	if v.prompt == nil || width <= 0 {
		return "", -1
	}
	prefix = truncateToWidth(prefix, width)
	prefixWidth := stringWidth(prefix)
	if prefixWidth >= width {
		return prefix, max(width-1, 0)
	}

	text, cursorX := clipPromptInput(v.prompt.input(), v.prompt.cursor(), width-prefixWidth)
	if cursorX >= 0 {
		cursorX += prefixWidth
	}
	return prefix + text, cursorX
}

func clipPromptInput(input string, cursor, width int) (string, int) {
	if width <= 0 {
		return "", -1
	}

	clusters := splitGraphemes(input)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(clusters) {
		cursor = len(clusters)
	}

	cursorWidth := stringWidth(strings.Join(clusters[:cursor], ""))
	inputWidth := stringWidth(input)
	if inputWidth <= width {
		return input, min(cursorWidth, width-1)
	}

	start := min(cursorWidth, max(inputWidth-width, 0))
	if start <= 0 {
		return truncateToWidth(input, width), min(cursorWidth, width-1)
	}

	clip := "…"
	contentWidth := max(width-stringWidth(clip), 0)
	if contentWidth <= 0 {
		return truncateToWidth(trimLeftToWidth(input, start), width), min(max(cursorWidth-start, 0), width-1)
	}
	if cursorWidth > start+contentWidth {
		start = cursorWidth - contentWidth
	}
	visible := truncateToWidth(trimLeftToWidth(input, start), contentWidth)
	return clip + visible, min(stringWidth(clip)+max(cursorWidth-start, 0), width-1)
}

func (v *Viewer) handlePromptMappedAction(a action) (KeyResult, bool) {
	switch a {
	case actionQuit:
		return KeyResult{Handled: true, Quit: true, Action: KeyActionQuit, Context: KeyContextPrompt}, true
	case actionCycleSearchCase:
		if v.prompt != nil && v.prompt.kind == promptSave {
			v.cycleSaveScope()
			return KeyResult{Handled: true, Context: KeyContextPrompt}, true
		}
		v.CycleSearchCaseMode()
		return KeyResult{Handled: true, Action: KeyActionCycleSearchCase, Context: KeyContextPrompt}, true
	case actionCycleSearchMode:
		if v.prompt != nil && v.prompt.kind == promptSave {
			v.cycleSaveFormat()
			return KeyResult{Handled: true, Context: KeyContextPrompt}, true
		}
		v.CycleSearchMode()
		return KeyResult{Handled: true, Action: KeyActionCycleSearchMode, Context: KeyContextPrompt}, true
	default:
		return KeyResult{}, false
	}
}

func (v *Viewer) toggleHelp() {
	if v.overlay != nil {
		v.overlay = nil
		v.mode = modeNormal
		return
	}
	if v.mode == modeHelp {
		v.mode = modeNormal
		return
	}
	v.mode = modeHelp
	v.helpOffset = 0
	v.helpColOffset = 0
}

func (v *Viewer) informationTitle() string {
	if v.overlay != nil {
		return v.overlay.title
	}
	return v.text.HelpTitle
}

func (v *Viewer) informationBody() string {
	if v.overlay != nil {
		return v.overlay.body
	}
	return v.text.HelpBody
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
	if style.GetForeground() == color.Default {
		style = style.Foreground(base.GetForeground())
	}
	if style.GetBackground() == color.Default {
		style = style.Background(base.GetBackground())
	}
	style = style.Attributes(base.GetAttributes() | overlay.GetAttributes())
	if overlay.GetUnderlineStyle() == tcell.UnderlineStyleNone && base.GetUnderlineStyle() != tcell.UnderlineStyleNone {
		style = style.Underline(base.GetUnderlineStyle(), base.GetUnderlineColor())
	}
	return style
}

type matchStylePreset struct {
	Fg             color.Color
	Bg             color.Color
	UnderlineStyle tcell.UnderlineStyle
	UnderlineColor color.Color
	Bold           bool
}

var (
	// These defaults use ANSI palette colors so they remain predictable on
	// terminals without reliable RGB support. They are kept as style pairs so
	// background changes always come with an explicit contrasting foreground.
	inactiveMatchStyle = matchStylePreset{
		Fg: color.PaletteColor(0),
		Bg: color.PaletteColor(6),
	}
	currentMatchStyle = matchStylePreset{
		Fg:             color.PaletteColor(7),
		Bg:             color.PaletteColor(4),
		UnderlineStyle: tcell.UnderlineStyleDouble,
		UnderlineColor: color.PaletteColor(5),
		Bold:           true,
	}

	statusBarStyle = tcell.StyleDefault.Foreground(color.PaletteColor(15)).Background(color.PaletteColor(2))
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
