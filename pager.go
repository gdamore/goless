// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import (
	"io"

	"github.com/gdamore/goless/internal/ansi"
	"github.com/gdamore/goless/internal/layout"
	"github.com/gdamore/goless/internal/model"
	iview "github.com/gdamore/goless/internal/view"
	"github.com/gdamore/tcell/v3"
)

const defaultChunkSize = 32 * 1024

// WrapMode controls whether logical lines are horizontally scrolled or soft-wrapped.
type WrapMode int

const (
	// NoWrap preserves logical lines and allows horizontal scrolling.
	NoWrap WrapMode = WrapMode(layout.NoWrap)

	// SoftWrap wraps logical lines to the current viewport width.
	SoftWrap WrapMode = WrapMode(layout.SoftWrap)
)

// Config is a compatibility construction bundle for Pager.
//
// Prefer passing explicit Option values to New. Config remains accepted by New
// because it implements the sealed Option interface.
type Config struct {
	TabWidth          int                         // TabWidth controls tab expansion during layout. Values <= 0 default to 8.
	WrapMode          WrapMode                    // WrapMode selects horizontal scrolling or soft wrapping.
	SearchCase        SearchCaseMode              // SearchCase selects smart-case, case-sensitive, or case-insensitive literal search behavior.
	SearchMode        SearchMode                  // SearchMode selects substring, whole-word, or regex search behavior.
	SqueezeBlankLines bool                        // SqueezeBlankLines collapses consecutive empty logical lines into a single visible line at view time.
	LineNumbers       bool                        // LineNumbers enables an adaptive line-number gutter.
	HeaderLines       int                         // HeaderLines pins the first N logical lines at the top of the viewport.
	HeaderColumns     int                         // HeaderColumns pins the first N display columns at the left edge of the viewport.
	Theme             Theme                       // Theme remaps content default colors and ANSI 0-15 without affecting chrome.
	Visualization     Visualization               // Visualization overlays optional markers for tabs, line endings, carriage returns, and EOF.
	HyperlinkHandler  HyperlinkHandler            // HyperlinkHandler controls how parsed OSC 8 hyperlink spans are rendered.
	CommandHandler    func(Command) CommandResult // CommandHandler handles unknown ':' commands after built-in pager commands decline them.
	KeyGroup          KeyGroup                    // KeyGroup selects a bundled set of key bindings.
	UnbindKeys        []KeyStroke                 // UnbindKeys removes exact bindings from the selected key group.
	KeyBindings       []KeyBinding                // KeyBindings prepend custom bindings ahead of bundled defaults.
	RenderMode        RenderMode                  // RenderMode controls how escapes and control sequences are presented.
	Chrome            Chrome                      // Chrome configures optional body framing and title display.
	ShowStatus        bool                        // ShowStatus enables the status bar on the last screen row.
	CaptureKey        func(*tcell.EventKey) bool  // CaptureKey reserves keys for the embedder before normal pager handling.

	// Text controls user-facing text, help content, and UI indicators.
	// Zero values are filled from DefaultText.
	Text Text
}

// KeyResult summarizes how the pager handled a key event.
type KeyResult struct {
	Handled bool
	Quit    bool
	Action  KeyAction
	Context KeyContext
}

// MouseResult summarizes how the pager handled a mouse event.
type MouseResult struct {
	Handled bool
	Action  KeyAction
	Context KeyContext
}

// Command describes a ':' command entered through the built-in prompt.
type Command struct {
	Raw  string
	Name string
	Args []string
}

// CommandResult describes how an embedder handled a ':' command.
type CommandResult struct {
	Handled    bool
	Quit       bool
	Message    string
	KeepPrompt bool
}

// Position summarizes the current logical pager viewport.
// Row and Column are 1-based logical coordinates when content is present, so
// (1, 1) is the start of the first logical line. An empty viewport reports
// Row=0 and Column=0.
type Position struct {
	Row int
	// Rows is the total logical line count.
	Rows int
	// Column is the 1-based logical display column number, or 0 when no content is visible.
	Column int
	// Columns is the maximum logical content width in columns.
	Columns int
}

// Pager is an embeddable document pager backed by an appendable document model.
type Pager struct {
	doc        *model.Document
	viewer     *iview.Viewer
	captureKey func(*tcell.EventKey) bool
	chunkSize  int
	renderMode ansi.RenderMode
}

// New constructs a Pager with the supplied options.
//
// The zero value behavior matches an empty option list. Missing optional
// configuration such as text bundles, key groups, and tab width are filled
// with pager defaults.
func New(opts ...Option) *Pager {
	var cfg Config
	for _, opt := range opts {
		if opt != nil {
			opt.apply(&cfg)
		}
	}
	return newPagerFromConfig(cfg)
}

func newPagerFromConfig(cfg Config) *Pager {
	renderMode := toInternalRenderMode(cfg.RenderMode)
	doc := model.NewDocumentWithMode(defaultChunkSize, renderMode)
	return &Pager{
		doc:        doc,
		captureKey: cfg.CaptureKey,
		chunkSize:  defaultChunkSize,
		renderMode: renderMode,
		viewer: iview.New(doc, iview.Config{
			TabWidth:          cfg.TabWidth,
			WrapMode:          toInternalWrapMode(cfg.WrapMode),
			SearchCase:        toInternalSearchCaseMode(cfg.SearchCase),
			SearchMode:        toInternalSearchMode(cfg.SearchMode),
			SqueezeBlankLines: cfg.SqueezeBlankLines,
			LineNumbers:       cfg.LineNumbers,
			HeaderLines:       cfg.HeaderLines,
			HeaderColumns:     cfg.HeaderColumns,
			Theme:             toInternalTheme(cfg.Theme),
			Visualization:     toInternalVisualization(cfg.Visualization),
			HyperlinkHandler:  toInternalHyperlinkHandler(cfg.HyperlinkHandler),
			CommandHandler:    toInternalCommandHandler(cfg.CommandHandler),
			KeyGroup:          toInternalKeyGroup(cfg.KeyGroup),
			KeyUnbind:         toInternalKeyStrokes(cfg.UnbindKeys),
			KeyBind:           toInternalKeyBindings(cfg.KeyBindings),
			Chrome:            toInternalChrome(cfg.Chrome),
			ShowStatus:        cfg.ShowStatus,
			Text:              toInternalText(cfg.Text),
		}),
	}
}

// Configure applies runtime-safe options to an existing pager.
func (p *Pager) Configure(opts ...RuntimeOption) {
	for _, opt := range opts {
		if opt != nil {
			opt.applyRuntime(p)
		}
	}
}

// Append appends raw bytes to the pager document and refreshes the derived layout.
func (p *Pager) Append(data []byte) error {
	if err := p.doc.Append(data); err != nil {
		return err
	}
	p.viewer.Refresh()
	return nil
}

// AppendString appends UTF-8 text to the pager document and refreshes the derived layout.
func (p *Pager) AppendString(text string) error {
	return p.Append([]byte(text))
}

// ReadFrom appends data read from r until EOF and refreshes the derived layout.
func (p *Pager) ReadFrom(r io.Reader) (int64, error) {
	buf := make([]byte, defaultChunkSize)
	var total int64
	for {
		n, err := r.Read(buf)
		if n > 0 {
			total += int64(n)
			if appendErr := p.doc.Append(buf[:n]); appendErr != nil {
				return total, appendErr
			}
		}
		if err == io.EOF {
			p.viewer.Refresh()
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}

// ReloadFrom replaces the current document with data read from r and refreshes
// the derived layout. The current viewport offsets, size, and chrome are
// preserved when possible and clamped against the new content when necessary.
func (p *Pager) ReloadFrom(r io.Reader) (int64, error) {
	doc := model.NewDocumentWithMode(p.chunkSize, p.renderMode)
	buf := make([]byte, p.chunkSize)
	var total int64
	for {
		n, err := r.Read(buf)
		if n > 0 {
			total += int64(n)
			if appendErr := doc.Append(buf[:n]); appendErr != nil {
				return total, appendErr
			}
		}
		if err == io.EOF {
			p.doc = doc
			p.viewer.SetDocument(doc)
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}

// Flush finalizes any incomplete parser state and refreshes the derived layout.
func (p *Pager) Flush() {
	p.doc.Flush()
	p.viewer.Refresh()
}

// Len returns the number of raw bytes stored in the pager document.
func (p *Pager) Len() int64 {
	return p.doc.Len()
}

// SetSize updates the pager viewport size.
func (p *Pager) SetSize(width, height int) {
	p.viewer.SetSize(width, height)
}

// Draw renders the current pager viewport to screen.
func (p *Pager) Draw(screen tcell.Screen) {
	p.viewer.Draw(screen)
}

// Refresh rebuilds the pager layout against the current document contents.
func (p *Pager) Refresh() {
	p.viewer.Refresh()
}

// SetTheme updates how document content colors are rendered.
func (p *Pager) SetTheme(theme Theme) {
	p.Configure(WithTheme(theme))
}

// SetLineNumbers updates whether the adaptive line-number gutter is shown.
func (p *Pager) SetLineNumbers(enabled bool) {
	p.Configure(WithLineNumbers(enabled))
}

// ToggleLineNumbers shows or hides the adaptive line-number gutter.
func (p *Pager) ToggleLineNumbers() {
	p.viewer.ToggleLineNumbers()
}

// LineNumbers reports whether the adaptive line-number gutter is enabled.
func (p *Pager) LineNumbers() bool {
	return p.viewer.LineNumbers()
}

// SetSqueezeBlankLines updates whether repeated blank lines are collapsed in the current view.
func (p *Pager) SetSqueezeBlankLines(enabled bool) {
	p.Configure(WithSqueezeBlankLines(enabled))
}

// SqueezeBlankLines reports whether repeated blank lines are collapsed in the current view.
func (p *Pager) SqueezeBlankLines() bool {
	return p.viewer.SqueezeBlankLines()
}

// SetHeaderLines updates how many leading logical lines stay fixed at the top of the viewport.
func (p *Pager) SetHeaderLines(count int) {
	p.Configure(WithHeaderLines(count))
}

// HeaderLines reports how many leading logical lines are fixed at the top of the viewport.
func (p *Pager) HeaderLines() int {
	return p.viewer.HeaderLines()
}

// SetHeaderColumns updates how many leading display columns stay fixed at the left edge of the viewport.
func (p *Pager) SetHeaderColumns(count int) {
	p.Configure(WithHeaderColumns(count))
}

// HeaderColumns reports how many leading display columns are fixed at the left edge of the viewport.
func (p *Pager) HeaderColumns() int {
	return p.viewer.HeaderColumns()
}

// SetVisualization updates how hidden structure markers are drawn.
func (p *Pager) SetVisualization(visual Visualization) {
	p.Configure(WithVisualization(visual))
}

// SetHyperlinkHandler updates how parsed OSC 8 hyperlinks are rendered.
func (p *Pager) SetHyperlinkHandler(handler HyperlinkHandler) {
	p.Configure(WithHyperlinkHandler(handler))
}

// SetChrome updates frame, title, and prompt/status styling.
func (p *Pager) SetChrome(chrome Chrome) {
	p.Configure(WithChrome(chrome))
}

// ShowInformation replaces the document view with a scrollable information overlay.
func (p *Pager) ShowInformation(title, body string) {
	p.viewer.ShowInformation(title, body)
}

// HideInformation closes the current information overlay if one is visible.
func (p *Pager) HideInformation() {
	p.viewer.HideInformation()
}

// ShowingInformation reports whether an information overlay is visible.
func (p *Pager) ShowingInformation() bool {
	return p.viewer.ShowingInformation()
}

// HandleKey applies a key event and reports whether the caller should exit.
func (p *Pager) HandleKey(ev *tcell.EventKey) bool {
	return p.HandleKeyResult(ev).Quit
}

// HandleMouse applies a mouse event and reports whether it was handled.
// Callers must enable mouse reporting on their tcell screen to receive wheel events.
func (p *Pager) HandleMouse(ev *tcell.EventMouse) bool {
	return p.HandleMouseResult(ev).Handled
}

// HandleKeyResult applies a key event and reports whether it was handled and whether the caller should exit.
func (p *Pager) HandleKeyResult(ev *tcell.EventKey) KeyResult {
	if p.captureKey != nil && p.captureKey(ev) {
		return KeyResult{}
	}
	result := p.viewer.HandleKeyResult(ev)
	return KeyResult{
		Handled: result.Handled,
		Quit:    result.Quit,
		Action:  KeyAction(result.Action),
		Context: KeyContext(result.Context),
	}
}

// HandleMouseResult applies a mouse event and reports whether it was handled.
// Callers must enable mouse reporting on their tcell screen to receive wheel events.
func (p *Pager) HandleMouseResult(ev *tcell.EventMouse) MouseResult {
	result := p.viewer.HandleMouseResult(ev)
	return MouseResult{
		Handled: result.Handled,
		Action:  KeyAction(result.Action),
		Context: KeyContext(result.Context),
	}
}

// ToggleWrap switches between horizontal scrolling and soft wrapping.
func (p *Pager) ToggleWrap() {
	p.viewer.ToggleWrap()
}

// SetWrapMode updates the pager wrap mode while preserving the current anchor.
func (p *Pager) SetWrapMode(mode WrapMode) {
	p.Configure(WithWrapMode(mode))
}

// WrapMode reports the current wrap mode.
func (p *Pager) WrapMode() WrapMode {
	return WrapMode(p.viewer.WrapMode())
}

// ScrollDown moves the viewport down by n rows.
func (p *Pager) ScrollDown(n int) {
	p.viewer.ScrollDown(n)
}

// ScrollUp moves the viewport up by n rows.
func (p *Pager) ScrollUp(n int) {
	p.viewer.ScrollUp(n)
}

// ScrollLeft moves the viewport left by n cells in no-wrap mode.
func (p *Pager) ScrollLeft(n int) {
	p.viewer.ScrollLeft(n)
}

// ScrollRight moves the viewport right by n cells in no-wrap mode.
func (p *Pager) ScrollRight(n int) {
	p.viewer.ScrollRight(n)
}

// GoLineStart moves to the beginning of the current horizontal line in no-wrap mode.
func (p *Pager) GoLineStart() {
	p.viewer.GoLineStart()
}

// GoLineEnd moves to the end of the current horizontal line in no-wrap mode.
func (p *Pager) GoLineEnd() {
	p.viewer.GoLineEnd()
}

// HalfPageDown moves the viewport down by roughly half a page.
func (p *Pager) HalfPageDown() {
	p.viewer.HalfPageDown()
}

// HalfPageUp moves the viewport up by roughly half a page.
func (p *Pager) HalfPageUp() {
	p.viewer.HalfPageUp()
}

// PageDown moves the viewport down by roughly one page.
func (p *Pager) PageDown() {
	p.viewer.PageDown()
}

// PageUp moves the viewport up by roughly one page.
func (p *Pager) PageUp() {
	p.viewer.PageUp()
}

// GoTop moves the viewport to the beginning of the document.
func (p *Pager) GoTop() {
	p.viewer.GoTop()
}

// GoBottom moves the viewport to the end of the document.
func (p *Pager) GoBottom() {
	p.viewer.GoBottom()
}

// GoPercent moves the viewport near the requested document percentage.
func (p *Pager) GoPercent(percent int) bool {
	return p.viewer.GoPercent(percent)
}

// Follow pins the viewport to the end of the document as new content arrives.
func (p *Pager) Follow() {
	p.viewer.Follow()
}

// StopFollow disables follow mode while leaving the viewport where it is.
func (p *Pager) StopFollow() {
	p.viewer.StopFollow()
}

// Following reports whether follow mode is active.
func (p *Pager) Following() bool {
	return p.viewer.Following()
}

// EOFVisible reports whether the end of the document is currently visible.
func (p *Pager) EOFVisible() bool {
	return p.viewer.EOFVisible()
}

// SetSearchCaseMode updates the default case behavior for new and active searches.
func (p *Pager) SetSearchCaseMode(mode SearchCaseMode) {
	p.Configure(WithSearchCaseMode(mode))
}

// SearchCaseMode reports the default case behavior for searches.
func (p *Pager) SearchCaseMode() SearchCaseMode {
	return SearchCaseMode(p.viewer.SearchCaseMode())
}

// CycleSearchCaseMode rotates through smart-case, case-sensitive, and case-insensitive search modes.
func (p *Pager) CycleSearchCaseMode() SearchCaseMode {
	return SearchCaseMode(p.viewer.CycleSearchCaseMode())
}

// SetSearchMode updates whether searches use substring, whole-word, or regex matching.
func (p *Pager) SetSearchMode(mode SearchMode) {
	p.Configure(WithSearchMode(mode))
}

// SearchMode reports whether searches use substring, whole-word, or regex matching.
func (p *Pager) SearchMode() SearchMode {
	return SearchMode(p.viewer.SearchMode())
}

// CycleSearchMode rotates through substring, whole-word, and regex search modes.
func (p *Pager) CycleSearchMode() SearchMode {
	return SearchMode(p.viewer.CycleSearchMode())
}

// SearchState reports the current committed or preview search state.
func (p *Pager) SearchState() SearchState {
	return toPublicSearchState(p.viewer.SearchSnapshot())
}

// SearchForward starts a forward search and reports whether any match exists.
func (p *Pager) SearchForward(query string) bool {
	return p.viewer.SearchForward(query)
}

// SearchForwardWithCase starts a forward search with the supplied case mode.
func (p *Pager) SearchForwardWithCase(query string, mode SearchCaseMode) bool {
	return p.viewer.SearchForwardWithCase(query, toInternalSearchCaseMode(mode))
}

// SearchBackward starts a backward search and reports whether any match exists.
func (p *Pager) SearchBackward(query string) bool {
	return p.viewer.SearchBackward(query)
}

// SearchBackwardWithCase starts a backward search with the supplied case mode.
func (p *Pager) SearchBackwardWithCase(query string, mode SearchCaseMode) bool {
	return p.viewer.SearchBackwardWithCase(query, toInternalSearchCaseMode(mode))
}

// SearchNext advances to the next match in the active search direction.
func (p *Pager) SearchNext() bool {
	return p.viewer.SearchNext()
}

// SearchPrev advances to the previous match relative to the active search direction.
func (p *Pager) SearchPrev() bool {
	return p.viewer.SearchPrev()
}

// ClearSearch removes any active search state.
func (p *Pager) ClearSearch() {
	p.viewer.ClearSearch()
}

// JumpToLine moves the viewport to the requested logical line number.
func (p *Pager) JumpToLine(lineNumber int) bool {
	return p.viewer.JumpToLine(lineNumber)
}

// Position reports the current logical line, total logical line count,
// current logical column number, and maximum logical column span.
func (p *Pager) Position() Position {
	pos := p.viewer.Position()
	return Position{
		Row:     pos.Row,
		Rows:    pos.Rows,
		Column:  pos.Column,
		Columns: pos.Columns,
	}
}

func toInternalWrapMode(mode WrapMode) layout.WrapMode {
	switch mode {
	case SoftWrap:
		return layout.SoftWrap
	default:
		return layout.NoWrap
	}
}

func toInternalSearchCaseMode(mode SearchCaseMode) iview.SearchCaseMode {
	switch mode {
	case SearchCaseSensitive:
		return iview.SearchCaseSensitive
	case SearchCaseInsensitive:
		return iview.SearchCaseInsensitive
	default:
		return iview.SearchSmartCase
	}
}

func toInternalSearchMode(mode SearchMode) iview.SearchMode {
	switch mode {
	case SearchWholeWord:
		return iview.SearchWholeWord
	case SearchRegex:
		return iview.SearchRegex
	default:
		return iview.SearchSubstring
	}
}

func toInternalKeyGroup(group KeyGroup) iview.KeyGroup {
	switch group {
	case EmptyKeyGroup:
		return iview.KeyGroupEmpty
	case LessKeyGroup:
		return iview.KeyGroupLess
	default:
		return iview.KeyGroupLess
	}
}

func toInternalKeyContext(ctx KeyContext) iview.KeyContext {
	switch ctx {
	case HelpKeyContext:
		return iview.KeyContextHelp
	case PromptKeyContext:
		return iview.KeyContextPrompt
	default:
		return iview.KeyContextNormal
	}
}

func toInternalKeyStrokes(strokes []KeyStroke) []iview.KeyStroke {
	if len(strokes) == 0 {
		return nil
	}
	result := make([]iview.KeyStroke, 0, len(strokes))
	for _, stroke := range strokes {
		result = append(result, iview.KeyStroke{
			Context:     toInternalKeyContext(stroke.Context),
			Key:         stroke.Key,
			Rune:        stroke.Rune,
			Modifiers:   stroke.Modifiers,
			AnyModifier: stroke.AnyModifier,
		})
	}
	return result
}

func toInternalKeyAction(a KeyAction) iview.KeyAction {
	switch a {
	case KeyActionQuit:
		return iview.KeyActionQuit
	case KeyActionScrollUp:
		return iview.KeyActionScrollUp
	case KeyActionScrollDown:
		return iview.KeyActionScrollDown
	case KeyActionScrollLeft:
		return iview.KeyActionScrollLeft
	case KeyActionScrollRight:
		return iview.KeyActionScrollRight
	case KeyActionScrollLeftFine:
		return iview.KeyActionScrollLeftFine
	case KeyActionScrollRightFine:
		return iview.KeyActionScrollRightFine
	case KeyActionHalfPageUp:
		return iview.KeyActionHalfPageUp
	case KeyActionHalfPageDown:
		return iview.KeyActionHalfPageDown
	case KeyActionPageUp:
		return iview.KeyActionPageUp
	case KeyActionPageDown:
		return iview.KeyActionPageDown
	case KeyActionGoLineStart:
		return iview.KeyActionGoLineStart
	case KeyActionGoLineEnd:
		return iview.KeyActionGoLineEnd
	case KeyActionGoTop:
		return iview.KeyActionGoTop
	case KeyActionGoBottom:
		return iview.KeyActionGoBottom
	case KeyActionToggleWrap:
		return iview.KeyActionToggleWrap
	case KeyActionPromptSearchForward:
		return iview.KeyActionPromptSearchForward
	case KeyActionPromptSearchBackward:
		return iview.KeyActionPromptSearchBackward
	case KeyActionPromptCommand:
		return iview.KeyActionPromptCommand
	case KeyActionSearchNext:
		return iview.KeyActionSearchNext
	case KeyActionSearchPrev:
		return iview.KeyActionSearchPrev
	case KeyActionRefresh:
		return iview.KeyActionRefresh
	case KeyActionToggleHelp:
		return iview.KeyActionToggleHelp
	case KeyActionFollow:
		return iview.KeyActionFollow
	case KeyActionCycleSearchCase:
		return iview.KeyActionCycleSearchCase
	case KeyActionCycleSearchMode:
		return iview.KeyActionCycleSearchMode
	default:
		return iview.KeyActionNone
	}
}

func toInternalKeyBindings(bindings []KeyBinding) []iview.KeyBinding {
	if len(bindings) == 0 {
		return nil
	}
	result := make([]iview.KeyBinding, 0, len(bindings))
	for _, binding := range bindings {
		result = append(result, iview.KeyBinding{
			KeyStroke: iview.KeyStroke{
				Context:     toInternalKeyContext(binding.Context),
				Key:         binding.Key,
				Rune:        binding.Rune,
				Modifiers:   binding.Modifiers,
				AnyModifier: binding.AnyModifier,
			},
			Action: toInternalKeyAction(binding.Action),
		})
	}
	return result
}

func toInternalTheme(theme Theme) iview.Theme {
	return iview.Theme{
		DefaultFG: theme.DefaultFG,
		DefaultBG: theme.DefaultBG,
		ANSI:      theme.ANSI,
	}
}

func toInternalVisualization(visual Visualization) iview.Visualization {
	return iview.Visualization{
		ShowTabs:            visual.ShowTabs,
		ShowNewlines:        visual.ShowNewlines,
		ShowCarriageReturns: visual.ShowCarriageReturns,
		ShowEOF:             visual.ShowEOF,
		TabGlyph:            visual.TabGlyph,
		NewlineGlyph:        visual.NewlineGlyph,
		CarriageReturnGlyph: visual.CarriageReturnGlyph,
		EOFGlyph:            visual.EOFGlyph,
		Style:               visual.Style,
		StyleSet:            visual.StyleSet,
	}
}

func toInternalHyperlinkHandler(handler HyperlinkHandler) iview.HyperlinkHandler {
	if handler == nil {
		return nil
	}
	return func(info iview.HyperlinkInfo) iview.HyperlinkDecision {
		decision := handler(HyperlinkInfo{
			Target: info.Target,
			ID:     info.ID,
			Text:   info.Text,
			Style:  info.Style,
		})
		return iview.HyperlinkDecision{
			Live:     decision.Live,
			Target:   decision.Target,
			Style:    decision.Style,
			StyleSet: decision.StyleSet,
		}
	}
}

func toInternalCommandHandler(handler func(Command) CommandResult) iview.CommandHandler {
	if handler == nil {
		return nil
	}
	return func(cmd iview.Command) iview.CommandResult {
		result := handler(Command{
			Raw:  cmd.Raw,
			Name: cmd.Name,
			Args: append([]string(nil), cmd.Args...),
		})
		return iview.CommandResult{
			Handled:    result.Handled,
			Quit:       result.Quit,
			Message:    result.Message,
			KeepPrompt: result.KeepPrompt,
		}
	}
}

func toInternalRenderMode(mode RenderMode) ansi.RenderMode {
	switch mode {
	case RenderLiteral:
		return ansi.RenderLiteral
	case RenderPresentation:
		return ansi.RenderPresentation
	default:
		return ansi.RenderHybrid
	}
}

func toInternalText(text Text) iview.Text {
	defaults := DefaultText()

	if text.HelpTitle == "" {
		text.HelpTitle = defaults.HelpTitle
	}
	if text.HelpClose == "" {
		text.HelpClose = defaults.HelpClose
	}
	if text.HelpBody == "" {
		text.HelpBody = defaults.HelpBody
	}
	if text.StatusSearchInfo == nil {
		text.StatusSearchInfo = defaults.StatusSearchInfo
	}
	if text.StatusPosition == nil {
		text.StatusPosition = defaults.StatusPosition
	}
	if text.FollowMode == "" {
		text.FollowMode = defaults.FollowMode
	}
	if text.StatusEOF == "" {
		text.StatusEOF = defaults.StatusEOF
	}
	if text.StatusNotEOF == "" {
		text.StatusNotEOF = defaults.StatusNotEOF
	}
	if text.SearchEmpty == "" {
		text.SearchEmpty = defaults.SearchEmpty
	}
	if text.SearchNotFound == nil {
		text.SearchNotFound = defaults.SearchNotFound
	}
	if text.SearchMatchCount == nil {
		text.SearchMatchCount = defaults.SearchMatchCount
	}
	if text.SearchNone == "" {
		text.SearchNone = defaults.SearchNone
	}
	if text.CommandUnknown == nil {
		text.CommandUnknown = defaults.CommandUnknown
	}
	if text.CommandLineStart == "" {
		text.CommandLineStart = defaults.CommandLineStart
	}
	if text.CommandOutOfRange == nil {
		text.CommandOutOfRange = defaults.CommandOutOfRange
	}
	if text.CommandLine == nil {
		text.CommandLine = defaults.CommandLine
	}
	if text.LeftOverflowOn == "" {
		text.LeftOverflowOn = defaults.LeftOverflowOn
	}
	if text.LeftOverflowOff == "" {
		text.LeftOverflowOff = defaults.LeftOverflowOff
	}
	if text.RightOverflowOn == "" {
		text.RightOverflowOn = defaults.RightOverflowOn
	}
	if text.RightOverflowOff == "" {
		text.RightOverflowOff = defaults.RightOverflowOff
	}
	if text.TopScrollableOn == "" {
		text.TopScrollableOn = defaults.TopScrollableOn
	}
	if text.TopScrollableOff == "" {
		text.TopScrollableOff = defaults.TopScrollableOff
	}
	if text.BottomScrollableOn == "" {
		text.BottomScrollableOn = defaults.BottomScrollableOn
	}
	if text.BottomScrollableOff == "" {
		text.BottomScrollableOff = defaults.BottomScrollableOff
	}

	itext := iview.Text{
		HelpTitle:           text.HelpTitle,
		HelpClose:           text.HelpClose,
		HelpBody:            text.HelpBody,
		StatusSearchInfo:    text.StatusSearchInfo,
		StatusPosition:      text.StatusPosition,
		StatusHelpHint:      text.StatusHelpHint,
		HideStatusHelpHint:  text.HideStatusHelpHint,
		FollowMode:          text.FollowMode,
		StatusEOF:           text.StatusEOF,
		StatusNotEOF:        text.StatusNotEOF,
		SearchEmpty:         text.SearchEmpty,
		SearchNotFound:      text.SearchNotFound,
		SearchMatchCount:    text.SearchMatchCount,
		SearchNone:          text.SearchNone,
		CommandUnknown:      text.CommandUnknown,
		CommandLineStart:    text.CommandLineStart,
		CommandOutOfRange:   text.CommandOutOfRange,
		CommandLine:         text.CommandLine,
		LeftOverflowOn:      text.LeftOverflowOn,
		LeftOverflowOff:     text.LeftOverflowOff,
		RightOverflowOn:     text.RightOverflowOn,
		RightOverflowOff:    text.RightOverflowOff,
		TopScrollableOn:     text.TopScrollableOn,
		TopScrollableOff:    text.TopScrollableOff,
		BottomScrollableOn:  text.BottomScrollableOn,
		BottomScrollableOff: text.BottomScrollableOff,
	}
	if text.StatusLine != nil {
		itext.StatusLine = func(info iview.StatusInfo) (left, right string) {
			return text.StatusLine(StatusInfo{
				Search:       toPublicSearchState(info.Search),
				Following:    info.Following,
				EOFVisible:   info.EOFVisible,
				Message:      info.Message,
				Position:     Position{Row: info.Position.Row, Rows: info.Position.Rows, Column: info.Position.Column, Columns: info.Position.Columns},
				DefaultLeft:  info.DefaultLeft,
				DefaultRight: info.DefaultRight,
			})
		}
	}
	if text.PromptLine != nil {
		itext.PromptLine = func(info iview.PromptInfo) string {
			return text.PromptLine(PromptInfo{
				Kind:        toPublicPromptKind(info.Kind),
				Prefix:      info.Prefix,
				Input:       info.Input,
				Cursor:      info.Cursor,
				Overwrite:   info.Overwrite,
				Seeded:      info.Seeded,
				Error:       info.Error,
				Search:      toPublicSearchState(info.Search),
				DefaultText: info.DefaultText,
			})
		}
	}
	return itext
}

func toInternalChrome(chrome Chrome) iview.Chrome {
	return iview.Chrome{
		TitleAlign:         toInternalTitleAlign(chrome.TitleAlign),
		Title:              chrome.Title,
		BorderStyle:        chrome.BorderStyle,
		TitleStyle:         chrome.TitleStyle,
		StatusStyle:        chrome.StatusStyle,
		StatusIconOnStyle:  chrome.StatusIconOnStyle,
		StatusIconOffStyle: chrome.StatusIconOffStyle,
		StatusHelpKeyStyle: chrome.StatusHelpKeyStyle,
		LineNumberStyle:    chrome.LineNumberStyle,
		HeaderStyle:        chrome.HeaderStyle,
		PromptStyle:        chrome.PromptStyle,
		PromptErrorStyle:   chrome.PromptErrorStyle,
		Frame: iview.Frame{
			Horizontal:  chrome.Frame.Horizontal,
			Vertical:    chrome.Frame.Vertical,
			TopLeft:     chrome.Frame.TopLeft,
			TopRight:    chrome.Frame.TopRight,
			BottomLeft:  chrome.Frame.BottomLeft,
			BottomRight: chrome.Frame.BottomRight,
		},
	}
}

func toInternalTitleAlign(align TitleAlign) iview.TitleAlign {
	switch align {
	case TitleAlignCenter:
		return iview.TitleAlignCenter
	case TitleAlignRight:
		return iview.TitleAlignRight
	default:
		return iview.TitleAlignLeft
	}
}

func toPublicSearchState(state iview.SearchSnapshot) SearchState {
	return SearchState{
		Query:        state.Query,
		Forward:      state.Forward,
		CaseMode:     SearchCaseMode(state.CaseMode),
		Mode:         SearchMode(state.Mode),
		MatchCount:   state.MatchCount,
		CurrentMatch: state.CurrentMatch,
		CompileError: state.CompileError,
		Preview:      state.Preview,
	}
}

func toPublicPromptKind(kind iview.PromptKind) PromptKind {
	switch kind {
	case iview.PromptKindSearchForward:
		return PromptSearchForward
	case iview.PromptKindSearchBackward:
		return PromptSearchBackward
	case iview.PromptKindCommand:
		return PromptCommand
	default:
		return PromptSearchForward
	}
}
