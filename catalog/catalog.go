// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

// Package catalog exposes the built-in UI message catalog used by goless.
// Applications can use these message IDs and default English strings to
// construct their own go-i18n bundles without requiring goless itself to own
// every locale translation.
package catalog

import (
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var (
	HelpTitle            = &i18n.Message{ID: "help.title", Other: "Help"}
	HelpClose            = &i18n.Message{ID: "help.close", Other: "Esc/q/H/F1 close"}
	HelpNavigationHeader = &i18n.Message{ID: "help.navigation.header", Other: "Navigation"}
	HelpNavigationMove   = &i18n.Message{ID: "help.navigation.move", Other: "Up/Down, j/k: move"}
	HelpNavigationHorz   = &i18n.Message{ID: "help.navigation.horizontal", Other: "Left/Right, h/l: scroll horizontally"}
	HelpNavigationPage   = &i18n.Message{ID: "help.navigation.page", Other: "PgUp/PgDn, b/f, Space: page"}
	HelpNavigationEnds   = &i18n.Message{ID: "help.navigation.ends", Other: "g/G: top/bottom"}
	HelpNavigationWrap   = &i18n.Message{ID: "help.navigation.wrap", Other: "w: toggle wrap"}
	HelpSearchHeader     = &i18n.Message{ID: "help.search.header", Other: "Search"}
	HelpSearchForward    = &i18n.Message{ID: "help.search.forward", Other: "/: search forward"}
	HelpSearchBackward   = &i18n.Message{ID: "help.search.backward", Other: "?: search backward"}
	HelpSearchRepeat     = &i18n.Message{ID: "help.search.repeat", Other: "n/N: next/previous match"}
	HelpCommandsHeader   = &i18n.Message{ID: "help.commands.header", Other: "Commands"}
	HelpCommandsJump     = &i18n.Message{ID: "help.commands.jump", Other: ":123: jump to line 123"}
	HelpGeneralHeader    = &i18n.Message{ID: "help.general.header", Other: "General"}
	HelpGeneralHelp      = &i18n.Message{ID: "help.general.help", Other: "H or F1: help"}
	HelpGeneralQuit      = &i18n.Message{ID: "help.general.quit", Other: "q or Esc: quit"}

	StatusLine       = &i18n.Message{ID: "status.line", Other: "{{.SearchInfo}}"}
	StatusPosition   = &i18n.Message{ID: "status.position", Other: "row {{.Current}}/{{.Total}}  col {{.Column}}"}
	StatusSearchInfo = &i18n.Message{ID: "status.search_info", Other: "  /{{.Query}} {{.Current}}/{{.Total}}"}
	ModeScroll       = &i18n.Message{ID: "status.mode.scroll", Other: "SCROLL"}
	ModeWrap         = &i18n.Message{ID: "status.mode.wrap", Other: "WRAP"}

	PromptEmptySearch = &i18n.Message{ID: "search.empty", Other: "empty search"}
	PromptNotFound    = &i18n.Message{ID: "search.not_found", Other: "{{.Query}} not found"}
	PromptMatchCount  = &i18n.Message{ID: "search.match_count", Other: "{{.Query}}: {{.Count}} matches"}
	PromptNoSearch    = &i18n.Message{ID: "search.none", Other: "no active search"}
	CommandUnknown    = &i18n.Message{ID: "command.unknown", Other: "unknown command: {{.Command}}"}
	CommandLineStart  = &i18n.Message{ID: "command.line_start", Other: "line numbers start at 1"}
	CommandOutOfRange = &i18n.Message{ID: "command.out_of_range", Other: "line {{.Line}} out of range"}
	CommandLine       = &i18n.Message{ID: "command.line", Other: "line {{.Line}}"}
)

var allMessages = []*i18n.Message{
	HelpTitle,
	HelpClose,
	HelpNavigationHeader,
	HelpNavigationMove,
	HelpNavigationHorz,
	HelpNavigationPage,
	HelpNavigationEnds,
	HelpNavigationWrap,
	HelpSearchHeader,
	HelpSearchForward,
	HelpSearchBackward,
	HelpSearchRepeat,
	HelpCommandsHeader,
	HelpCommandsJump,
	HelpGeneralHeader,
	HelpGeneralHelp,
	HelpGeneralQuit,
	StatusLine,
	StatusPosition,
	StatusSearchInfo,
	ModeScroll,
	ModeWrap,
	PromptEmptySearch,
	PromptNotFound,
	PromptMatchCount,
	PromptNoSearch,
	CommandUnknown,
	CommandLineStart,
	CommandOutOfRange,
	CommandLine,
}

// Messages returns the built-in catalog messages.
func Messages() []*i18n.Message {
	return append([]*i18n.Message(nil), allMessages...)
}

// NewBundle returns a go-i18n bundle preloaded with the built-in English catalog.
func NewBundle() *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	bundle.AddMessages(language.English, allMessages...)
	return bundle
}
