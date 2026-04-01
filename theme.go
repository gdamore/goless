// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import tcolor "github.com/gdamore/tcell/v3/color"

// Theme controls how document content colors are rendered.
//
// It applies only to pager content, not to chrome such as frames, status bars,
// prompts, or match highlighting.
//
// DefaultFG and DefaultBG replace the ANSI "default" foreground and background.
// ANSI remaps only the base 16 ANSI indexed colors. Indexed colors >= 16 and
// explicit RGB colors are preserved exactly.
//
// Zero-valued fields preserve the built-in mapping.
type Theme struct {
	// DefaultFG replaces the ANSI default foreground color.
	DefaultFG tcolor.Color
	// DefaultBG replaces the ANSI default background color.
	DefaultBG tcolor.Color
	// ANSI remaps ANSI indexed colors 0 through 15.
	ANSI [16]tcolor.Color
}
