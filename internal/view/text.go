// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"fmt"

	"github.com/gdamore/goless/internal/defaults"
)

// Text controls user-facing text and indicators inside the viewer.
// The public pager API maps its text bundle onto this type.
type Text struct {
	HelpTitle string
	HelpClose string
	HelpBody  string

	StatusSearchInfo   func(query string, current, total int) string
	StatusPosition     func(current, total, column, columns int) string
	StatusHelpHint     string
	HideStatusHelpHint bool
	FollowMode         string
	StatusEOF          string
	StatusNotEOF       string
	StatusLine         func(StatusInfo) (left, right string)

	SearchEmpty      string
	SearchNotFound   func(query string) string
	SearchMatchCount func(query string, count int) string
	SearchNone       string

	CommandUnknown    func(command string) string
	CommandLineStart  string
	CommandOutOfRange func(line int) string
	CommandLine       func(line int) string

	LeftOverflowOn      string
	LeftOverflowOff     string
	RightOverflowOn     string
	RightOverflowOff    string
	TopScrollableOn     string
	TopScrollableOff    string
	BottomScrollableOn  string
	BottomScrollableOff string
	PromptLine          func(PromptInfo) string
}

func (t Text) withDefaults() Text {
	defaults := defaultText()

	if t.HelpTitle == "" {
		t.HelpTitle = defaults.HelpTitle
	}
	if t.HelpClose == "" {
		t.HelpClose = defaults.HelpClose
	}
	if t.HelpBody == "" {
		t.HelpBody = defaults.HelpBody
	}
	if t.StatusSearchInfo == nil {
		t.StatusSearchInfo = defaults.StatusSearchInfo
	}
	if t.StatusPosition == nil {
		t.StatusPosition = defaults.StatusPosition
	}
	if !t.HideStatusHelpHint && t.StatusHelpHint == "" {
		t.StatusHelpHint = defaults.StatusHelpHint
	}
	if t.FollowMode == "" {
		t.FollowMode = defaults.FollowMode
	}
	if t.StatusEOF == "" {
		t.StatusEOF = defaults.StatusEOF
	}
	if t.StatusNotEOF == "" {
		t.StatusNotEOF = defaults.StatusNotEOF
	}
	if t.SearchEmpty == "" {
		t.SearchEmpty = defaults.SearchEmpty
	}
	if t.SearchNotFound == nil {
		t.SearchNotFound = defaults.SearchNotFound
	}
	if t.SearchMatchCount == nil {
		t.SearchMatchCount = defaults.SearchMatchCount
	}
	if t.SearchNone == "" {
		t.SearchNone = defaults.SearchNone
	}
	if t.CommandUnknown == nil {
		t.CommandUnknown = defaults.CommandUnknown
	}
	if t.CommandLineStart == "" {
		t.CommandLineStart = defaults.CommandLineStart
	}
	if t.CommandOutOfRange == nil {
		t.CommandOutOfRange = defaults.CommandOutOfRange
	}
	if t.CommandLine == nil {
		t.CommandLine = defaults.CommandLine
	}
	if t.LeftOverflowOn == "" {
		t.LeftOverflowOn = defaults.LeftOverflowOn
	}
	if t.LeftOverflowOff == "" {
		t.LeftOverflowOff = defaults.LeftOverflowOff
	}
	if t.RightOverflowOn == "" {
		t.RightOverflowOn = defaults.RightOverflowOn
	}
	if t.RightOverflowOff == "" {
		t.RightOverflowOff = defaults.RightOverflowOff
	}
	if t.TopScrollableOn == "" {
		t.TopScrollableOn = defaults.TopScrollableOn
	}
	if t.TopScrollableOff == "" {
		t.TopScrollableOff = defaults.TopScrollableOff
	}
	if t.BottomScrollableOn == "" {
		t.BottomScrollableOn = defaults.BottomScrollableOn
	}
	if t.BottomScrollableOff == "" {
		t.BottomScrollableOff = defaults.BottomScrollableOff
	}

	return t
}

func defaultText() Text {
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
		StatusEOF:      "∎",
		StatusNotEOF:   " ",
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
		LeftOverflowOn:      "◀",
		LeftOverflowOff:     " ",
		RightOverflowOn:     "▶",
		RightOverflowOff:    " ",
		TopScrollableOn:     "▲",
		TopScrollableOff:    " ",
		BottomScrollableOn:  "▼",
		BottomScrollableOff: " ",
	}
}
