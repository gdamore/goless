// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import "github.com/gdamore/tcell/v3"

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
	kind   promptKind
	prefix string
	buffer []rune
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
)

func actionForKey(ev *tcell.EventKey) action {
	switch ev.Key() {
	case tcell.KeyEscape, tcell.KeyCtrlC:
		return actionQuit
	case tcell.KeyUp:
		return actionScrollUp
	case tcell.KeyDown:
		return actionScrollDown
	case tcell.KeyLeft:
		return actionScrollLeft
	case tcell.KeyRight:
		return actionScrollRight
	case tcell.KeyPgUp:
		return actionPageUp
	case tcell.KeyPgDn:
		return actionPageDown
	case tcell.KeyHome:
		return actionGoTop
	case tcell.KeyEnd:
		return actionGoBottom
	case tcell.KeyF1:
		return actionToggleHelp
	case tcell.KeyRune:
		switch ev.Str() {
		case "q":
			return actionQuit
		case "j":
			return actionScrollDown
		case "k":
			return actionScrollUp
		case "h":
			return actionScrollLeft
		case "l":
			return actionScrollRight
		case " ", "f":
			return actionPageDown
		case "b":
			return actionPageUp
		case "g":
			return actionGoTop
		case "G":
			return actionGoBottom
		case "w":
			return actionToggleWrap
		case "/":
			return actionPromptSearchForward
		case "?":
			return actionPromptSearchBackward
		case ":":
			return actionPromptCommand
		case "n":
			return actionSearchNext
		case "N":
			return actionSearchPrev
		case "H":
			return actionToggleHelp
		case "F":
			return actionFollow
		}
	}
	return actionNone
}
