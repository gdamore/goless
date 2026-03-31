// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import "github.com/gdamore/tcell/v3"

// KeyGroup selects a bundled set of viewer key bindings.
type KeyGroup int

const (
	// KeyGroupLess selects less-like bundled bindings.
	KeyGroupLess KeyGroup = iota
)

type keyBinding struct {
	key    tcell.Key
	rune   string
	mod    tcell.ModMask
	anyMod bool
	action action
}

type keyMap struct {
	normal []keyBinding
	help   []keyBinding
}

func defaultKeyMap(group KeyGroup) keyMap {
	switch group {
	case KeyGroupLess:
		fallthrough
	default:
		return lessKeyMap()
	}
}

func lessKeyMap() keyMap {
	return keyMap{
		normal: []keyBinding{
			{key: tcell.KeyEscape, action: actionQuit},
			{key: tcell.KeyCtrlC, action: actionQuit},
			{key: tcell.KeyUp, action: actionScrollUp},
			{key: tcell.KeyDown, action: actionScrollDown},
			{key: tcell.KeyLeft, action: actionScrollLeft},
			{key: tcell.KeyRight, action: actionScrollRight},
			{key: tcell.KeyPgUp, action: actionPageUp},
			{key: tcell.KeyPgDn, action: actionPageDown},
			{key: tcell.KeyHome, action: actionGoTop},
			{key: tcell.KeyEnd, action: actionGoBottom},
			{key: tcell.KeyF2, action: actionCycleSearchCase},
			{key: tcell.KeyF3, action: actionCycleSearchMode},
			{key: tcell.KeyF1, action: actionToggleHelp},
			{key: tcell.KeyRune, rune: "q", action: actionQuit},
			{key: tcell.KeyRune, rune: "j", action: actionScrollDown},
			{key: tcell.KeyRune, rune: "k", action: actionScrollUp},
			{key: tcell.KeyRune, rune: "h", action: actionScrollLeft},
			{key: tcell.KeyRune, rune: "l", action: actionScrollRight},
			{key: tcell.KeyRune, rune: " ", action: actionPageDown},
			{key: tcell.KeyRune, rune: "f", action: actionPageDown},
			{key: tcell.KeyRune, rune: "b", action: actionPageUp},
			{key: tcell.KeyRune, rune: "g", action: actionGoTop},
			{key: tcell.KeyRune, rune: "G", action: actionGoBottom},
			{key: tcell.KeyRune, rune: "w", action: actionToggleWrap},
			{key: tcell.KeyRune, rune: "/", action: actionPromptSearchForward},
			{key: tcell.KeyRune, rune: "?", action: actionPromptSearchBackward},
			{key: tcell.KeyRune, rune: ":", action: actionPromptCommand},
			{key: tcell.KeyRune, rune: "n", action: actionSearchNext},
			{key: tcell.KeyRune, rune: "N", action: actionSearchPrev},
			{key: tcell.KeyRune, rune: "H", action: actionToggleHelp},
			{key: tcell.KeyRune, rune: "F", action: actionFollow},
		},
		help: []keyBinding{
			{key: tcell.KeyEscape, action: actionToggleHelp},
			{key: tcell.KeyCtrlC, action: actionQuit},
			{key: tcell.KeyUp, action: actionScrollUp},
			{key: tcell.KeyDown, action: actionScrollDown},
			{key: tcell.KeyPgUp, action: actionPageUp},
			{key: tcell.KeyPgDn, action: actionPageDown},
			{key: tcell.KeyHome, action: actionGoTop},
			{key: tcell.KeyEnd, action: actionGoBottom},
			{key: tcell.KeyF2, action: actionCycleSearchCase},
			{key: tcell.KeyF3, action: actionCycleSearchMode},
			{key: tcell.KeyF1, action: actionToggleHelp},
			{key: tcell.KeyRune, rune: "q", action: actionToggleHelp},
			{key: tcell.KeyRune, rune: "H", action: actionToggleHelp},
		},
	}
}

func (m keyMap) normalAction(ev *tcell.EventKey) action {
	return actionForBindings(m.normal, ev)
}

func (m keyMap) helpAction(ev *tcell.EventKey) action {
	return actionForBindings(m.help, ev)
}

func actionForBindings(bindings []keyBinding, ev *tcell.EventKey) action {
	for _, binding := range bindings {
		if binding.matches(ev) {
			return binding.action
		}
	}
	return actionNone
}

func (b keyBinding) matches(ev *tcell.EventKey) bool {
	if ev.Key() != b.key {
		return false
	}
	if !b.anyMod && ev.Modifiers() != b.mod {
		return false
	}
	if b.key == tcell.KeyRune && ev.Str() != b.rune {
		return false
	}
	return true
}
