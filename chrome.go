// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import "github.com/gdamore/tcell/v3"

// TitleAlign controls where frame titles are placed on the top border.
type TitleAlign int

const (
	// TitleAlignLeft places the title near the left frame edge.
	TitleAlignLeft TitleAlign = iota
	// TitleAlignCenter centers the title in the top frame border.
	TitleAlignCenter
	// TitleAlignRight places the title near the right frame edge.
	TitleAlignRight
)

// Frame defines the glyphs used to draw an optional border around the pager
// body. An all-zero Frame disables border drawing. When a frame is enabled,
// each field is rendered exactly as configured; empty strings remain empty.
type Frame struct {
	// Horizontal is the glyph repeated across the top and bottom frame edges.
	Horizontal string
	// Vertical is the glyph repeated across the left and right frame edges.
	Vertical string
	// TopLeft is the glyph drawn at the upper-left frame corner.
	TopLeft string
	// TopRight is the glyph drawn at the upper-right frame corner.
	TopRight string
	// BottomLeft is the glyph drawn at the lower-left frame corner.
	BottomLeft string
	// BottomRight is the glyph drawn at the lower-right frame corner.
	BottomRight string
}

// Chrome configures optional decorative chrome around the pager body.
// Frame glyphs are drawn exactly as configured, so embedders can intentionally
// leave pieces blank when that suits the design.
type Chrome struct {
	// Title is rendered into the top border area when a frame is enabled.
	Title string
	// TitleAlign controls whether the title is placed at the left, center, or right.
	TitleAlign TitleAlign
	// Frame selects the glyphs used to draw the optional body border.
	Frame Frame
	// BorderStyle controls the style used when drawing the frame glyphs.
	BorderStyle tcell.Style
	// TitleStyle controls the style used when drawing the frame title.
	TitleStyle tcell.Style
	// StatusStyle controls the style used when drawing the status bar.
	StatusStyle tcell.Style
	// StatusIconOnStyle controls the style used for the active icons in the status bar.
	StatusIconOnStyle tcell.Style
	// StatusIconOffStyle controls the style used for the inactive icons in the status bar.
	StatusIconOffStyle tcell.Style
	// StatusHelpKeyStyle controls the style used for the leading key token in the built-in idle help hint.
	StatusHelpKeyStyle tcell.Style
	// LineNumberStyle controls the style used when drawing the optional line-number gutter.
	LineNumberStyle tcell.Style
	// HeaderStyle controls the style used when drawing fixed header rows and columns.
	HeaderStyle tcell.Style
	// PinnedRowGlyph is the single-column marker drawn in chrome for pinned rows.
	// An empty string falls back to PinnedGlyph, and then disables the marker.
	PinnedRowGlyph string
	// PinnedColumnGlyph is the single-column marker drawn in chrome for pinned columns.
	// An empty string falls back to PinnedGlyph, and then disables the marker.
	PinnedColumnGlyph string
	// PinnedGlyph is the legacy single-column marker used for both pinned rows
	// and pinned columns when the more specific glyphs are not set.
	PinnedGlyph string
	// PinnedStyle controls the style used for pin indicators in the chrome.
	// A nil or zero value uses a bold overlay.
	PinnedStyle tcell.Style
	// RestylePinned transforms the style used for cells that belong to pinned rows or columns.
	// A nil function leaves the cell style unchanged.
	RestylePinned func(tcell.Style) tcell.Style
	// PromptStyle controls the style used when drawing the prompt line.
	PromptStyle tcell.Style
	// PromptErrorStyle controls the style used for prompt-side error text.
	PromptErrorStyle tcell.Style
}

// SingleFrame returns a single-line box drawing frame.
func SingleFrame() Frame {
	return Frame{
		Horizontal:  "─",
		Vertical:    "│",
		TopLeft:     "┌",
		TopRight:    "┐",
		BottomLeft:  "└",
		BottomRight: "┘",
	}
}

// RoundedFrame returns a rounded-corner box drawing frame.
func RoundedFrame() Frame {
	return Frame{
		Horizontal:  "─",
		Vertical:    "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "╰",
		BottomRight: "╯",
	}
}
