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
	HelpTitle string
	HelpClose string
	HelpBody  string

	StatusSearchInfo func(query string, current, total int) string
	StatusPosition   func(current, total, column int) string
	FollowMode       string

	SearchEmpty      string
	SearchNotFound   func(query string) string
	SearchMatchCount func(query string, count int) string
	SearchNone       string

	CommandUnknown    func(command string) string
	CommandLineStart  string
	CommandOutOfRange func(line int) string
	CommandLine       func(line int) string

	LeftOverflowIndicator  string
	RightOverflowIndicator string
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
		StatusPosition: func(current, total, column int) string {
			return fmt.Sprintf("row %d/%d  col %d", current, total, column)
		},
		FollowMode:  "follow",
		SearchEmpty: "empty search",
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
