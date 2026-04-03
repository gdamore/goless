// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import (
	"fmt"

	"github.com/gdamore/goless/internal/defaults"
)

// Text controls the user-facing text and indicators used by the pager UI.
// Embedders can replace any of these values, including the help content.
type Text struct {
	// HelpTitle is shown in the help overlay title area.
	HelpTitle string
	// HelpClose is shown in the help overlay as a close hint.
	HelpClose string
	// HelpBody is the body text rendered in the help overlay.
	HelpBody string

	// StatusSearchInfo formats the left-side status text for an active search.
	StatusSearchInfo func(query string, current, total int) string
	// StatusPosition formats the right-side position text in the status bar.
	StatusPosition func(current, total, column, columns int) string
	// StatusHelpHint is shown on the left side of the status bar when no other status text is active.
	StatusHelpHint string
	// HideStatusHelpHint suppresses the built-in idle help hint even when StatusHelpHint is empty.
	HideStatusHelpHint bool
	// FollowMode is appended to the status bar while follow mode is active.
	FollowMode string
	// StatusLine can override the full left and right status bar text.
	// When nil, the pager assembles the built-in status line from the fields above.
	StatusLine func(StatusInfo) (left, right string)

	// SearchEmpty is shown when an empty search is submitted.
	SearchEmpty string
	// SearchNotFound formats the message shown when no search match is found.
	SearchNotFound func(query string) string
	// SearchMatchCount formats the message shown after a successful search.
	SearchMatchCount func(query string, count int) string
	// SearchNone is shown when repeat-search is invoked without an active query.
	SearchNone string

	// CommandUnknown formats the message shown for an unknown ':' command.
	CommandUnknown func(command string) string
	// CommandLineStart is shown when a line jump less than 1 is requested.
	CommandLineStart string
	// CommandOutOfRange formats the message shown for an out-of-range line jump.
	CommandOutOfRange func(line int) string
	// CommandLine formats the message shown after a successful line jump.
	CommandLine func(line int) string

	// LeftOverflowIndicator marks undisplayed content to the left in no-wrap mode.
	LeftOverflowIndicator string
	// RightOverflowIndicator marks undisplayed content to the right in no-wrap mode.
	RightOverflowIndicator string
	// PromptLine can override the full built-in prompt text.
	// When nil, the pager uses the built-in prefix plus the current input buffer.
	PromptLine func(PromptInfo) string
}

// DefaultText returns the built-in pager text bundle.
func DefaultText() Text {
	return Text{
		HelpTitle: "Help",
		HelpClose: "Esc/q/H/F1 close",
		HelpBody:  defaults.HelpBody,
		StatusSearchInfo: func(query string, current, total int) string {
			return fmt.Sprintf("/%s %d/%d", query, current, total)
		},
		StatusPosition: func(current, total, column, columns int) string {
			return fmt.Sprintf("row %d/%d  col %d/%d", current, total, column, columns)
		},
		StatusHelpHint: "F1 Help",
		FollowMode:     "follow",
		SearchEmpty:    "empty search",
		SearchNotFound: func(query string) string {
			return fmt.Sprintf("%s not found", query)
		},
		SearchMatchCount: func(query string, count int) string {
			return fmt.Sprintf("%s: %d matches", query, count)
		},
		SearchNone: "no active search",
		CommandUnknown: func(command string) string {
			return fmt.Sprintf("unknown command: %s", command)
		},
		CommandLineStart: "line numbers start at 1",
		CommandOutOfRange: func(line int) string {
			return fmt.Sprintf("line %d out of range", line)
		},
		CommandLine: func(line int) string {
			return fmt.Sprintf("line %d", line)
		},
		LeftOverflowIndicator:  "◀",
		RightOverflowIndicator: "▶",
	}
}
