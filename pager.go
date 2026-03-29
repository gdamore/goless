// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import (
	"io"

	"github.com/gdamore/goless/internal/layout"
	"github.com/gdamore/goless/internal/model"
	iview "github.com/gdamore/goless/internal/view"
	"github.com/gdamore/tcell/v3"
	"github.com/nicksnyder/go-i18n/v2/i18n"
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
	// TabWidth controls tab expansion during layout. Values <= 0 default to 8.
	TabWidth int
	// WrapMode selects horizontal scrolling or soft wrapping.
	WrapMode WrapMode
	// ShowStatus enables the status bar on the last screen row.
	ShowStatus bool
	// Localizer controls localization of pager UI strings. Nil uses the built-in English catalog.
	Localizer *i18n.Localizer
}

// Pager is an embeddable document pager backed by an appendable document model.
type Pager struct {
	doc    *model.Document
	viewer *iview.Viewer
}

// New constructs a Pager with the supplied configuration.
func New(cfg Config) *Pager {
	doc := model.NewDocument(defaultChunkSize)
	return &Pager{
		doc: doc,
		viewer: iview.New(doc, iview.Config{
			TabWidth:   cfg.TabWidth,
			WrapMode:   toInternalWrapMode(cfg.WrapMode),
			ShowStatus: cfg.ShowStatus,
			Localizer:  cfg.Localizer,
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
	return p.viewer.HandleKey(ev)
}

// ToggleWrap switches between horizontal scrolling and soft wrapping.
func (p *Pager) ToggleWrap() {
	p.viewer.ToggleWrap()
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

func toInternalWrapMode(mode WrapMode) layout.WrapMode {
	switch mode {
	case SoftWrap:
		return layout.SoftWrap
	default:
		return layout.NoWrap
	}
}
