// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

type viewerMode int

const (
	modeNormal viewerMode = iota
	modePrompt
	modeHelp
)

type promptKind int

const (
	promptSearchForward promptKind = iota
	promptSearchBackward
	promptCommand
)

type promptState struct {
	kind    promptKind
	prefix  string
	buffer  []rune
	preview *searchState
}

func (p *promptState) String() string {
	if p == nil {
		return ""
	}
	return p.prefix + string(p.buffer)
}

type action int

const (
	actionNone action = iota
	actionQuit
	actionScrollUp
	actionScrollDown
	actionScrollLeft
	actionScrollRight
	actionPageUp
	actionPageDown
	actionGoTop
	actionGoBottom
	actionToggleWrap
	actionPromptSearchForward
	actionPromptSearchBackward
	actionPromptCommand
	actionSearchNext
	actionSearchPrev
	actionToggleHelp
	actionFollow
	actionCycleSearchCase
)
