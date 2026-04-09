// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import "github.com/gdamore/tcell/v3"

// KeyGroup selects a bundled set of pager key bindings.
type KeyGroup int

const (
	// DefaultKeyGroup selects the pager's default bundled key bindings.
	DefaultKeyGroup KeyGroup = iota
	// LessKeyGroup selects the pager's less-like bundled key bindings.
	LessKeyGroup
	// EmptyKeyGroup starts with no bundled key bindings.
	EmptyKeyGroup
)

// KeyContext selects which pager mode a key binding applies to.
type KeyContext int

const (
	// NormalKeyContext applies while the pager is showing document content.
	NormalKeyContext KeyContext = iota
	// HelpKeyContext applies while the built-in help view is visible.
	HelpKeyContext
	// PromptKeyContext applies while a /, ?, or : prompt is open.
	// Only KeyActionQuit, KeyActionCycleSearchCase, and KeyActionCycleSearchMode
	// are currently supported in this context.
	PromptKeyContext
)

// KeyAction identifies a pager action that can be triggered from a binding.
type KeyAction int

const (
	// KeyActionNone performs no pager action.
	KeyActionNone KeyAction = iota
	// KeyActionQuit exits the pager.
	KeyActionQuit
	// KeyActionScrollUp scrolls up by one row.
	KeyActionScrollUp
	// KeyActionScrollDown scrolls down by one row.
	KeyActionScrollDown
	// KeyActionScrollLeft scrolls left by a horizontal navigation step.
	KeyActionScrollLeft
	// KeyActionScrollRight scrolls right by a horizontal navigation step.
	KeyActionScrollRight
	// KeyActionScrollLeftFine scrolls left by one cell.
	KeyActionScrollLeftFine
	// KeyActionScrollRightFine scrolls right by one cell.
	KeyActionScrollRightFine
	// KeyActionHalfPageUp scrolls up by roughly half a page.
	KeyActionHalfPageUp
	// KeyActionHalfPageDown scrolls down by roughly half a page.
	KeyActionHalfPageDown
	// KeyActionPageUp scrolls up by roughly one page.
	KeyActionPageUp
	// KeyActionPageDown scrolls down by roughly one page.
	KeyActionPageDown
	// KeyActionGoLineStart jumps to the beginning of the current horizontal line.
	KeyActionGoLineStart
	// KeyActionGoLineEnd jumps to the end of the current horizontal line.
	KeyActionGoLineEnd
	// KeyActionGoTop jumps to the top of the document.
	KeyActionGoTop
	// KeyActionGoBottom jumps to the bottom of the document.
	KeyActionGoBottom
	// KeyActionToggleWrap toggles horizontal scrolling versus soft-wrap.
	KeyActionToggleWrap
	// KeyActionPromptSearchForward opens the forward search prompt.
	KeyActionPromptSearchForward
	// KeyActionPromptSearchBackward opens the backward search prompt.
	KeyActionPromptSearchBackward
	// KeyActionPromptCommand opens the command prompt.
	KeyActionPromptCommand
	// KeyActionSearchNext advances to the next search match.
	KeyActionSearchNext
	// KeyActionSearchPrev advances to the previous search match.
	KeyActionSearchPrev
	// KeyActionRefresh repaints the current screen without changing pager state.
	KeyActionRefresh
	// KeyActionToggleHelp shows or hides the built-in help.
	KeyActionToggleHelp
	// KeyActionFollow enables follow mode.
	KeyActionFollow
	// KeyActionStopFollow stops follow mode without quitting the pager.
	KeyActionStopFollow
	// KeyActionCycleSearchCase cycles smart, case-sensitive, and case-insensitive search behavior.
	KeyActionCycleSearchCase
	// KeyActionCycleSearchMode cycles substring, whole-word, and regex search behavior.
	KeyActionCycleSearchMode
)

// KeyStroke identifies a key in a specific pager context.
type KeyStroke struct {
	// Context selects which pager mode this key applies to.
	Context KeyContext
	// Key selects the tcell key code to match.
	Key tcell.Key
	// Rune selects the rune string for KeyRune bindings.
	Rune string
	// Modifiers must match exactly unless AnyModifier is true.
	Modifiers tcell.ModMask
	// AnyModifier ignores Modifiers and matches any modifier state.
	AnyModifier bool
}

// KeyBinding associates a key stroke with a pager action.
type KeyBinding struct {
	KeyStroke
	// Action selects which pager behavior the key triggers.
	// Not every action is valid in every KeyContext.
	Action KeyAction
}
