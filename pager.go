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

// Config configures a Pager.
type Config struct {
	TabWidth    int                        // TabWidth controls tab expansion during layout. Values <= 0 default to 8.
	WrapMode    WrapMode                   // WrapMode selects horizontal scrolling or soft wrapping.
	SearchCase  SearchCaseMode             // SearchCase selects smart-case, case-sensitive, or case-insensitive literal search behavior.
	SearchMode  SearchMode                 // SearchMode selects substring, whole-word, or regex search behavior.
	Theme       Theme                      // Theme remaps content default colors and ANSI 0-15 without affecting chrome.
	KeyGroup    KeyGroup                   // KeyGroup selects a bundled set of key bindings.
	UnbindKeys  []KeyStroke                // UnbindKeys removes exact bindings from the selected key group.
	KeyBindings []KeyBinding               // KeyBindings prepend custom bindings ahead of bundled defaults.
	RenderMode  RenderMode                 // RenderMode controls how escapes and control sequences are presented.
	Chrome      Chrome                     // Chrome configures optional body framing and title display.
	ShowStatus  bool                       // ShowStatus enables the status bar on the last screen row.
	CaptureKey  func(*tcell.EventKey) bool // CaptureKey reserves keys for the embedder before normal pager handling.

	// Text controls user-facing text, help content, and UI indicators.
	// Zero values are filled from DefaultText.
	Text Text
}

// KeyResult summarizes how the pager handled a key event.
type KeyResult struct {
	Handled bool
	Quit    bool
}

// Position summarizes the current visible pager viewport.
type Position struct {
	Row    int
	Rows   int
	Column int
}

// Pager is an embeddable document pager backed by an appendable document model.
type Pager struct {
	doc        *model.Document
	viewer     *iview.Viewer
	captureKey func(*tcell.EventKey) bool
}

// New constructs a Pager with the supplied configuration.
//
// The zero value of Config is valid. Missing optional configuration such as
// text bundles, key groups, and tab width are filled with pager defaults.
func New(cfg Config) *Pager {
	doc := model.NewDocumentWithMode(defaultChunkSize, toInternalRenderMode(cfg.RenderMode))
	return &Pager{
		doc:        doc,
		captureKey: cfg.CaptureKey,
		viewer: iview.New(doc, iview.Config{
			TabWidth:   cfg.TabWidth,
			WrapMode:   toInternalWrapMode(cfg.WrapMode),
			SearchCase: toInternalSearchCaseMode(cfg.SearchCase),
			SearchMode: toInternalSearchMode(cfg.SearchMode),
			Theme:      toInternalTheme(cfg.Theme),
			KeyGroup:   toInternalKeyGroup(cfg.KeyGroup),
			KeyUnbind:  toInternalKeyStrokes(cfg.UnbindKeys),
			KeyBind:    toInternalKeyBindings(cfg.KeyBindings),
			Chrome:     toInternalChrome(cfg.Chrome),
			ShowStatus: cfg.ShowStatus,
			Text:       toInternalText(cfg.Text),
		}),
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

// HandleKey applies a key event and reports whether the caller should exit.
func (p *Pager) HandleKey(ev *tcell.EventKey) bool {
	return p.HandleKeyResult(ev).Quit
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
	}
}

// ToggleWrap switches between horizontal scrolling and soft wrapping.
func (p *Pager) ToggleWrap() {
	p.viewer.ToggleWrap()
}

// SetWrapMode updates the pager wrap mode while preserving the current anchor.
func (p *Pager) SetWrapMode(mode WrapMode) {
	p.viewer.SetWrapMode(toInternalWrapMode(mode))
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

// Follow pins the viewport to the end of the document as new content arrives.
func (p *Pager) Follow() {
	p.viewer.Follow()
}

// Following reports whether follow mode is active.
func (p *Pager) Following() bool {
	return p.viewer.Following()
}

// SetSearchCaseMode updates the default case behavior for new and active searches.
func (p *Pager) SetSearchCaseMode(mode SearchCaseMode) {
	p.viewer.SetSearchCaseMode(toInternalSearchCaseMode(mode))
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
	p.viewer.SetSearchMode(toInternalSearchMode(mode))
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
	state := p.viewer.SearchSnapshot()
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

// JumpToLine moves the viewport to the requested logical line.
func (p *Pager) JumpToLine(lineNumber int) bool {
	return p.viewer.JumpToLine(lineNumber)
}

// Position reports the current visible row, total row count, and horizontal offset.
func (p *Pager) Position() Position {
	pos := p.viewer.Position()
	return Position{
		Row:    pos.Row,
		Rows:   pos.Rows,
		Column: pos.Column,
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
	case KeyActionPageUp:
		return iview.KeyActionPageUp
	case KeyActionPageDown:
		return iview.KeyActionPageDown
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
	if text.LeftOverflowIndicator == "" {
		text.LeftOverflowIndicator = defaults.LeftOverflowIndicator
	}
	if text.RightOverflowIndicator == "" {
		text.RightOverflowIndicator = defaults.RightOverflowIndicator
	}

	return iview.Text{
		HelpTitle:        text.HelpTitle,
		HelpClose:        text.HelpClose,
		HelpBody:         text.HelpBody,
		StatusSearchInfo: text.StatusSearchInfo,
		StatusPosition:   text.StatusPosition,
		FollowMode:       text.FollowMode,
		StatusLine: func(info iview.StatusInfo) (left, right string) {
			if text.StatusLine == nil {
				return info.DefaultLeft, info.DefaultRight
			}
			return text.StatusLine(StatusInfo{
				Search:       toPublicSearchState(info.Search),
				Following:    info.Following,
				Message:      info.Message,
				Position:     Position{Row: info.Position.Row, Rows: info.Position.Rows, Column: info.Position.Column},
				DefaultLeft:  info.DefaultLeft,
				DefaultRight: info.DefaultRight,
			})
		},
		SearchEmpty:            text.SearchEmpty,
		SearchNotFound:         text.SearchNotFound,
		SearchMatchCount:       text.SearchMatchCount,
		SearchNone:             text.SearchNone,
		CommandUnknown:         text.CommandUnknown,
		CommandLineStart:       text.CommandLineStart,
		CommandOutOfRange:      text.CommandOutOfRange,
		CommandLine:            text.CommandLine,
		LeftOverflowIndicator:  text.LeftOverflowIndicator,
		RightOverflowIndicator: text.RightOverflowIndicator,
		PromptLine: func(info iview.PromptInfo) string {
			if text.PromptLine == nil {
				return info.DefaultText
			}
			return text.PromptLine(PromptInfo{
				Kind:        toPublicPromptKind(info.Kind),
				Prefix:      info.Prefix,
				Input:       info.Input,
				Error:       info.Error,
				Search:      toPublicSearchState(info.Search),
				DefaultText: info.DefaultText,
			})
		},
	}
}

func toInternalChrome(chrome Chrome) iview.Chrome {
	return iview.Chrome{
		TitleAlign:       toInternalTitleAlign(chrome.TitleAlign),
		Title:            chrome.Title,
		BorderStyle:      chrome.BorderStyle,
		TitleStyle:       chrome.TitleStyle,
		StatusStyle:      chrome.StatusStyle,
		PromptStyle:      chrome.PromptStyle,
		PromptErrorStyle: chrome.PromptErrorStyle,
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
	case iview.PromptKindSearchBackward:
		return PromptSearchBackward
	case iview.PromptKindCommand:
		return PromptCommand
	default:
		return PromptSearchForward
	}
}
