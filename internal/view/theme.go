// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"github.com/gdamore/goless/internal/ansi"
	"github.com/gdamore/tcell/v3/color"
)

// Theme controls how document content colors are rendered.
//
// Zero-valued fields preserve the built-in mapping. color.Reset explicitly
// maps a themed entry back to the terminal default color.
type Theme struct {
	DefaultFG color.Color
	DefaultBG color.Color
	ANSI      [16]color.Color
}

func (t Theme) resolveColor(c ansi.Color, foreground bool) color.Color {
	switch c.Kind {
	case ansi.ColorDefault:
		if foreground {
			if t.DefaultFG != 0 {
				return t.DefaultFG
			}
		} else if t.DefaultBG != 0 {
			return t.DefaultBG
		}
		return color.Default
	case ansi.ColorIndex:
		if c.Index < 16 {
			if mapped := t.ANSI[c.Index]; mapped != 0 {
				return mapped
			}
		}
		return color.PaletteColor(int(c.Index))
	case ansi.ColorRGB:
		return color.NewRGBColor(int32(c.R), int32(c.G), int32(c.B))
	default:
		return color.Default
	}
}
