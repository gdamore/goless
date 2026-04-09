// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

type viewerMode int

const (
	modeNormal viewerMode = iota
	modePrompt
	modeHelp
)

type informationOverlay struct {
	title string
	body  string
}

type promptKind int

const (
	promptSearchForward promptKind = iota
	promptSearchBackward
	promptCommand
)

type promptState struct {
	kind    promptKind
	prefix  string
	editor  lineEditor
	seeded  bool
	preview *searchState
	errText string
	history promptHistoryState
}

func (p *promptState) String() string {
	if p == nil {
		return ""
	}
	return p.prefix + p.input()
}

func (p *promptState) input() string {
	if p == nil {
		return ""
	}
	return p.editor.String()
}

func (p *promptState) cursor() int {
	if p == nil {
		return 0
	}
	return p.editor.Cursor()
}

type promptHistoryKind int

const (
	promptHistorySearch promptHistoryKind = iota
	promptHistoryCommand
	promptHistoryKindCount
)

type promptHistoryState struct {
	kind  promptHistoryKind
	index int
	draft string
}

type action int

const (
	actionNone action = iota
	actionQuit
	actionScrollUp
	actionScrollDown
	actionScrollLeft
	actionScrollRight
	actionScrollLeftFine
	actionScrollRightFine
	actionHalfPageUp
	actionHalfPageDown
	actionPageUp
	actionPageDown
	actionGoLineStart
	actionGoLineEnd
	actionGoTop
	actionGoBottom
	actionToggleWrap
	actionPromptSearchForward
	actionPromptSearchBackward
	actionPromptCommand
	actionSearchNext
	actionSearchPrev
	actionRefresh
	actionToggleHelp
	actionFollow
	actionStopFollow
	actionCycleSearchCase
	actionCycleSearchMode
)

// MouseResult summarizes how the viewer handled a mouse event.
type MouseResult struct {
	Handled bool
	Action  KeyAction
	Context KeyContext
}
