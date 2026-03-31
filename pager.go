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
	TabWidth   int                        // TabWidth controls tab expansion during layout. Values <= 0 default to 8.
	WrapMode   WrapMode                   // WrapMode selects horizontal scrolling or soft wrapping.
	KeyGroup   KeyGroup                   // KeyGroup selects a bundled set of key bindings.
	RenderMode RenderMode                 // RenderMode controls how escapes and control sequences are presented.
	Chrome     Chrome                     // Chrome configures optional body framing and title display.
	ShowStatus bool                       // ShowStatus enables the status bar on the last screen row.
	CaptureKey func(*tcell.EventKey) bool // CaptureKey reserves keys for the embedder before normal pager handling.

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
			KeyGroup:   toInternalKeyGroup(cfg.KeyGroup),
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

// SearchForward starts a forward literal search and reports whether any match exists.
func (p *Pager) SearchForward(query string) bool {
	return p.viewer.SearchForward(query)
}

// SearchBackward starts a backward literal search and reports whether any match exists.
func (p *Pager) SearchBackward(query string) bool {
	return p.viewer.SearchBackward(query)
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

func toInternalKeyGroup(group KeyGroup) iview.KeyGroup {
	switch group {
	case LessKeyGroup:
		return iview.KeyGroupLess
	default:
		return iview.KeyGroupLess
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
		HelpTitle:              text.HelpTitle,
		HelpClose:              text.HelpClose,
		HelpBody:               text.HelpBody,
		StatusSearchInfo:       text.StatusSearchInfo,
		StatusPosition:         text.StatusPosition,
		FollowMode:             text.FollowMode,
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
	}
}

func toInternalChrome(chrome Chrome) iview.Chrome {
	return iview.Chrome{
		Title:       chrome.Title,
		BorderStyle: chrome.BorderStyle,
		TitleStyle:  chrome.TitleStyle,
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
