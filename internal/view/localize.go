// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"github.com/gdamore/goless/catalog"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var (
	msgHelpTitle            = catalog.HelpTitle
	msgHelpClose            = catalog.HelpClose
	msgHelpNavigationHeader = catalog.HelpNavigationHeader
	msgHelpNavigationMove   = catalog.HelpNavigationMove
	msgHelpNavigationHorz   = catalog.HelpNavigationHorz
	msgHelpNavigationPage   = catalog.HelpNavigationPage
	msgHelpNavigationEnds   = catalog.HelpNavigationEnds
	msgHelpNavigationWrap   = catalog.HelpNavigationWrap
	msgHelpSearchHeader     = catalog.HelpSearchHeader
	msgHelpSearchForward    = catalog.HelpSearchForward
	msgHelpSearchBackward   = catalog.HelpSearchBackward
	msgHelpSearchRepeat     = catalog.HelpSearchRepeat
	msgHelpCommandsHeader   = catalog.HelpCommandsHeader
	msgHelpCommandsJump     = catalog.HelpCommandsJump
	msgHelpGeneralHeader    = catalog.HelpGeneralHeader
	msgHelpGeneralHelp      = catalog.HelpGeneralHelp
	msgHelpGeneralQuit      = catalog.HelpGeneralQuit
	msgStatusLine           = catalog.StatusLine
	msgStatusSearchInfo     = catalog.StatusSearchInfo
	msgModeScroll           = catalog.ModeScroll
	msgModeWrap             = catalog.ModeWrap
	msgPromptEmptySearch    = catalog.PromptEmptySearch
	msgPromptNotFound       = catalog.PromptNotFound
	msgPromptMatchCount     = catalog.PromptMatchCount
	msgPromptNoSearch       = catalog.PromptNoSearch
	msgCommandUnknown       = catalog.CommandUnknown
	msgCommandLineStart     = catalog.CommandLineStart
	msgCommandOutOfRange    = catalog.CommandOutOfRange
	msgCommandLine          = catalog.CommandLine
)

func defaultLocalizer() *i18n.Localizer {
	return i18n.NewLocalizer(catalog.NewBundle(), language.English.String())
}
