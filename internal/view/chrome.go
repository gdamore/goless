// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
)

// TitleAlign controls where frame titles are placed on the top border.
type TitleAlign int

const (
	TitleAlignLeft TitleAlign = iota
	TitleAlignCenter
	TitleAlignRight
)

// Frame defines the glyphs used to draw an optional border around the viewer
// body. An all-zero Frame disables border drawing. When a frame is enabled,
// each field is rendered exactly as configured.
type Frame struct {
	Horizontal  string
	Vertical    string
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
}

func (f Frame) enabled() bool {
	return f.Horizontal != "" || f.Vertical != "" || f.TopLeft != "" || f.TopRight != "" || f.BottomLeft != "" || f.BottomRight != ""
}

// Chrome configures optional decorative chrome around the viewer body.
type Chrome struct {
	TitleAlign         TitleAlign
	Title              string
	Frame              Frame
	BorderStyle        tcell.Style
	TitleStyle         tcell.Style
	StatusStyle        tcell.Style
	StatusIconOnStyle  tcell.Style
	StatusIconOffStyle tcell.Style
	StatusHelpKeyStyle tcell.Style
	LineNumberStyle    tcell.Style
	HeaderStyle        tcell.Style
	PromptStyle        tcell.Style
	PromptErrorStyle   tcell.Style
}

func (c Chrome) withDefaults() Chrome {
	if c.BorderStyle == tcell.StyleDefault {
		c.BorderStyle = tcell.StyleDefault.Foreground(color.PaletteColor(4))
	}
	if c.TitleStyle == tcell.StyleDefault {
		c.TitleStyle = tcell.StyleDefault.Foreground(color.PaletteColor(15)).Bold(true)
	}
	if c.StatusStyle == tcell.StyleDefault {
		c.StatusStyle = statusBarStyle
	}
	if c.StatusIconOnStyle == tcell.StyleDefault {
		c.StatusIconOnStyle = c.StatusStyle.Foreground(color.PaletteColor(11)).Dim(true)
	}
	if c.StatusIconOffStyle == tcell.StyleDefault {
		c.StatusIconOffStyle = c.StatusStyle.Dim(true)
	}
	if c.StatusHelpKeyStyle == tcell.StyleDefault {
		c.StatusHelpKeyStyle = c.StatusStyle.Foreground(color.PaletteColor(11)).Bold(true)
	}
	if c.LineNumberStyle == tcell.StyleDefault {
		c.LineNumberStyle = tcell.StyleDefault.Foreground(color.PaletteColor(8)).Dim(true)
	}
	if c.HeaderStyle == tcell.StyleDefault {
		c.HeaderStyle = tcell.StyleDefault.Bold(true)
	}
	if c.PromptStyle == tcell.StyleDefault {
		c.PromptStyle = tcell.StyleDefault.Reverse(true)
	}
	if c.PromptErrorStyle == tcell.StyleDefault {
		c.PromptErrorStyle = c.PromptStyle.Foreground(color.Red)
	}
	return c
}
