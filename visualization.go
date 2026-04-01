// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import "github.com/gdamore/tcell/v3"

// Visualization controls optional pager-added markers for otherwise hidden
// structure such as tabs, line endings, and end-of-content.
type Visualization struct {
	// ShowTabs replaces fully visible tab expansion with TabGlyph plus padding.
	ShowTabs bool
	// ShowNewlines appends NewlineGlyph after visible line endings.
	ShowNewlines bool
	// ShowCarriageReturns appends CarriageReturnGlyph for CRLF endings and
	// replaces standalone carriage-return control pictures in content.
	ShowCarriageReturns bool
	// ShowEOF appends EOFGlyph at the end of the final visible logical line.
	ShowEOF bool

	// TabGlyph is used when ShowTabs is enabled.
	TabGlyph string
	// NewlineGlyph is used when ShowNewlines is enabled.
	NewlineGlyph string
	// CarriageReturnGlyph is used when ShowCarriageReturns is enabled.
	CarriageReturnGlyph string
	// EOFGlyph is used when ShowEOF is enabled.
	EOFGlyph string

	// Style controls how pager-added markers are drawn.
	Style tcell.Style
}
