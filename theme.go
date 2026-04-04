// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import "github.com/gdamore/tcell/v3/color"

// Theme controls how document content colors are rendered.
//
// It applies only to pager content, not to chrome such as frames, status bars,
// prompts, or match highlighting.
//
// DefaultFG and DefaultBG replace the ANSI "default" foreground and background.
// ANSI remaps only the base 16 ANSI indexed colors. Indexed colors >= 16 and
// explicit RGB colors are preserved exactly.
//
// Zero-valued fields preserve the built-in mapping. Use color.Reset to map a
// theme entry back to the terminal's own default color explicitly.
type Theme struct {
	// DefaultFG replaces the ANSI default foreground color.
	// Zero preserves the built-in mapping. color.Reset forces the terminal default.
	DefaultFG color.Color
	// DefaultBG replaces the ANSI default background color.
	// Zero preserves the built-in mapping. color.Reset forces the terminal default.
	DefaultBG color.Color
	// ANSI remaps ANSI indexed colors 0 through 15.
	// Zero preserves the built-in mapping for that slot.
	ANSI [16]color.Color
}
