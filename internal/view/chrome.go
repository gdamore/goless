// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"github.com/gdamore/tcell/v3"
	tcolor "github.com/gdamore/tcell/v3/color"
)

// Frame defines the glyphs used to draw an optional border around the viewer
// body. An all-zero Frame disables border drawing.
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
	Title            string
	Frame            Frame
	BorderStyle      tcell.Style
	TitleStyle       tcell.Style
	StatusStyle      tcell.Style
	PromptStyle      tcell.Style
	PromptErrorStyle tcell.Style
}

func (c Chrome) withDefaults() Chrome {
	if c.BorderStyle == tcell.StyleDefault {
		c.BorderStyle = tcell.StyleDefault.Foreground(tcolor.PaletteColor(4))
	}
	if c.TitleStyle == tcell.StyleDefault {
		c.TitleStyle = tcell.StyleDefault.Foreground(tcolor.PaletteColor(15)).Bold(true)
	}
	if c.StatusStyle == tcell.StyleDefault {
		c.StatusStyle = statusBarStyle
	}
	if c.PromptStyle == tcell.StyleDefault {
		c.PromptStyle = tcell.StyleDefault.Reverse(true)
	}
	if c.PromptErrorStyle == tcell.StyleDefault {
		c.PromptErrorStyle = c.PromptStyle.Foreground(tcolor.Red)
	}
	return c
}
