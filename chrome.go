// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import "github.com/gdamore/tcell/v3"

// Frame defines the glyphs used to draw an optional border around the pager
// body. An all-zero Frame disables border drawing.
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
type Chrome struct {
	// Title is rendered into the top border area when a frame is enabled.
	Title string
	// Frame selects the glyphs used to draw the optional body border.
	Frame Frame
	// BorderStyle controls the style used when drawing the frame glyphs.
	BorderStyle tcell.Style
	// TitleStyle controls the style used when drawing the frame title.
	TitleStyle tcell.Style
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
