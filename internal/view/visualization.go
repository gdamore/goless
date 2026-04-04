// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
)

// Visualization controls optional viewer-added markers for otherwise hidden
// structure such as tabs, line endings, and end-of-content.
type Visualization struct {
	ShowTabs            bool
	ShowNewlines        bool
	ShowCarriageReturns bool
	ShowEOF             bool
	TabGlyph            string
	NewlineGlyph        string
	CarriageReturnGlyph string
	EOFGlyph            string
	Style               tcell.Style
	StyleSet            bool
}

func (v Visualization) withDefaults() Visualization {
	if v.TabGlyph == "" {
		v.TabGlyph = "→"
	}
	if v.NewlineGlyph == "" {
		v.NewlineGlyph = "↵"
	}
	if v.CarriageReturnGlyph == "" {
		v.CarriageReturnGlyph = "␍"
	}
	if v.EOFGlyph == "" {
		v.EOFGlyph = "∎"
	}
	if !v.StyleSet {
		v.Style = tcell.StyleDefault.Foreground(color.PaletteColor(8))
	}
	return v
}
