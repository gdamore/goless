// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import "github.com/gdamore/tcell/v3"

// Frame defines the glyphs used to draw an optional border around the pager
// body. An all-zero Frame disables border drawing.
type Frame struct {
	Horizontal  string
	Vertical    string
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
}

// Chrome configures optional decorative chrome around the pager body.
type Chrome struct {
	Title       string
	Frame       Frame
	BorderStyle tcell.Style
	TitleStyle  tcell.Style
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
