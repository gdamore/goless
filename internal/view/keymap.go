// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import "github.com/gdamore/tcell/v3"

// KeyGroup selects a bundled set of viewer key bindings.
type KeyGroup int

const (
	// KeyGroupLess selects less-like bundled bindings.
	KeyGroupLess KeyGroup = iota
	// KeyGroupEmpty starts with no bundled bindings.
	KeyGroupEmpty
)

type keyContext int

const (
	keyContextNormal keyContext = iota
	keyContextHelp
	keyContextPrompt
)

// KeyContext selects which viewer mode a key binding applies to.
type KeyContext = keyContext

const (
	KeyContextNormal = keyContextNormal
	KeyContextHelp   = keyContextHelp
	KeyContextPrompt = keyContextPrompt
)

// KeyAction identifies a viewer action that can be triggered from a binding.
type KeyAction = action

const (
	KeyActionNone                 = actionNone
	KeyActionQuit                 = actionQuit
	KeyActionScrollUp             = actionScrollUp
	KeyActionScrollDown           = actionScrollDown
	KeyActionScrollLeft           = actionScrollLeft
	KeyActionScrollRight          = actionScrollRight
	KeyActionScrollLeftFine       = actionScrollLeftFine
	KeyActionScrollRightFine      = actionScrollRightFine
	KeyActionHalfPageUp           = actionHalfPageUp
	KeyActionHalfPageDown         = actionHalfPageDown
	KeyActionPageUp               = actionPageUp
	KeyActionPageDown             = actionPageDown
	KeyActionGoLineStart          = actionGoLineStart
	KeyActionGoLineEnd            = actionGoLineEnd
	KeyActionGoTop                = actionGoTop
	KeyActionGoBottom             = actionGoBottom
	KeyActionToggleWrap           = actionToggleWrap
	KeyActionPromptSearchForward  = actionPromptSearchForward
	KeyActionPromptSearchBackward = actionPromptSearchBackward
	KeyActionPromptCommand        = actionPromptCommand
	KeyActionSearchNext           = actionSearchNext
	KeyActionSearchPrev           = actionSearchPrev
	KeyActionToggleHelp           = actionToggleHelp
	KeyActionFollow               = actionFollow
	KeyActionCycleSearchCase      = actionCycleSearchCase
	KeyActionCycleSearchMode      = actionCycleSearchMode
)

// KeyStroke identifies a key in a specific viewer context.
type KeyStroke struct {
	Context     KeyContext
	Key         tcell.Key
	Rune        string
	Modifiers   tcell.ModMask
	AnyModifier bool
}

// KeyBinding associates a key stroke with a viewer action.
type KeyBinding struct {
	KeyStroke
	Action KeyAction
}

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
	prompt []keyBinding
}

func defaultKeyMap(group KeyGroup) keyMap {
	switch group {
	case KeyGroupEmpty:
		return keyMap{}
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
			{key: tcell.KeyLeft, mod: tcell.ModShift, action: actionScrollLeftFine},
			{key: tcell.KeyRight, mod: tcell.ModShift, action: actionScrollRightFine},
			{key: tcell.KeyPgUp, action: actionPageUp},
			{key: tcell.KeyPgDn, action: actionPageDown},
			{key: tcell.KeyHome, action: actionGoLineStart},
			{key: tcell.KeyEnd, action: actionGoLineEnd},
			{key: tcell.KeyF2, action: actionCycleSearchCase},
			{key: tcell.KeyF3, action: actionCycleSearchMode},
			{key: tcell.KeyF1, action: actionToggleHelp},
			{key: tcell.KeyRune, rune: "q", action: actionQuit},
			{key: tcell.KeyRune, rune: "j", action: actionScrollDown},
			{key: tcell.KeyRune, rune: "k", action: actionScrollUp},
			{key: tcell.KeyRune, rune: "h", action: actionScrollLeft},
			{key: tcell.KeyRune, rune: "l", action: actionScrollRight},
			{key: tcell.KeyRune, rune: "<", action: actionScrollLeftFine},
			{key: tcell.KeyRune, rune: ">", action: actionScrollRightFine},
			{key: tcell.KeyRune, rune: "u", mod: tcell.ModCtrl, action: actionHalfPageUp},
			{key: tcell.KeyRune, rune: "d", mod: tcell.ModCtrl, action: actionHalfPageDown},
			{key: tcell.KeyRune, rune: "u", action: actionHalfPageUp},
			{key: tcell.KeyRune, rune: "d", action: actionHalfPageDown},
			{key: tcell.KeyRune, rune: "0", action: actionGoLineStart},
			{key: tcell.KeyRune, rune: "$", action: actionGoLineEnd},
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
			{key: tcell.KeyLeft, action: actionScrollLeft},
			{key: tcell.KeyRight, action: actionScrollRight},
			{key: tcell.KeyLeft, mod: tcell.ModShift, action: actionScrollLeftFine},
			{key: tcell.KeyRight, mod: tcell.ModShift, action: actionScrollRightFine},
			{key: tcell.KeyRune, rune: "j", action: actionScrollDown},
			{key: tcell.KeyRune, rune: "k", action: actionScrollUp},
			{key: tcell.KeyRune, rune: "h", action: actionScrollLeft},
			{key: tcell.KeyRune, rune: "l", action: actionScrollRight},
			{key: tcell.KeyRune, rune: "<", action: actionScrollLeftFine},
			{key: tcell.KeyRune, rune: ">", action: actionScrollRightFine},
			{key: tcell.KeyRune, rune: "u", mod: tcell.ModCtrl, action: actionHalfPageUp},
			{key: tcell.KeyRune, rune: "d", mod: tcell.ModCtrl, action: actionHalfPageDown},
			{key: tcell.KeyRune, rune: "u", action: actionHalfPageUp},
			{key: tcell.KeyRune, rune: "d", action: actionHalfPageDown},
			{key: tcell.KeyPgUp, action: actionPageUp},
			{key: tcell.KeyPgDn, action: actionPageDown},
			{key: tcell.KeyRune, rune: " ", action: actionPageDown},
			{key: tcell.KeyRune, rune: "f", action: actionPageDown},
			{key: tcell.KeyRune, rune: "b", action: actionPageUp},
			{key: tcell.KeyHome, action: actionGoLineStart},
			{key: tcell.KeyEnd, action: actionGoLineEnd},
			{key: tcell.KeyRune, rune: "0", action: actionGoLineStart},
			{key: tcell.KeyRune, rune: "$", action: actionGoLineEnd},
			{key: tcell.KeyRune, rune: "g", action: actionGoTop},
			{key: tcell.KeyRune, rune: "G", action: actionGoBottom},
			{key: tcell.KeyF2, action: actionCycleSearchCase},
			{key: tcell.KeyF3, action: actionCycleSearchMode},
			{key: tcell.KeyF1, action: actionToggleHelp},
			{key: tcell.KeyRune, rune: "q", action: actionToggleHelp},
			{key: tcell.KeyRune, rune: "H", action: actionToggleHelp},
		},
		prompt: []keyBinding{
			{key: tcell.KeyCtrlC, action: actionQuit},
			{key: tcell.KeyF2, action: actionCycleSearchCase},
			{key: tcell.KeyF3, action: actionCycleSearchMode},
		},
	}
}

func (m keyMap) withOverrides(unbind []KeyStroke, bind []KeyBinding) keyMap {
	for _, stroke := range unbind {
		switch stroke.Context {
		case KeyContextHelp:
			m.help = removeBindings(m.help, stroke)
		case KeyContextPrompt:
			m.prompt = removeBindings(m.prompt, stroke)
		default:
			m.normal = removeBindings(m.normal, stroke)
		}
	}

	var prependNormal []keyBinding
	var prependHelp []keyBinding
	var prependPrompt []keyBinding
	for _, binding := range bind {
		if !bindingAllowedInContext(binding.Context, binding.Action) {
			continue
		}
		converted := keyBinding{
			key:    binding.Key,
			rune:   binding.Rune,
			mod:    binding.Modifiers,
			anyMod: binding.AnyModifier,
			action: action(binding.Action),
		}
		switch binding.Context {
		case KeyContextHelp:
			prependHelp = append(prependHelp, converted)
		case KeyContextPrompt:
			prependPrompt = append(prependPrompt, converted)
		default:
			prependNormal = append(prependNormal, converted)
		}
	}

	if len(prependNormal) > 0 {
		m.normal = append(prependNormal, m.normal...)
	}
	if len(prependHelp) > 0 {
		m.help = append(prependHelp, m.help...)
	}
	if len(prependPrompt) > 0 {
		m.prompt = append(prependPrompt, m.prompt...)
	}

	return m
}

func bindingAllowedInContext(ctx KeyContext, a KeyAction) bool {
	switch ctx {
	case KeyContextPrompt:
		switch a {
		case KeyActionQuit, KeyActionCycleSearchCase, KeyActionCycleSearchMode:
			return true
		default:
			return false
		}
	default:
		return true
	}
}

func (m keyMap) normalAction(ev *tcell.EventKey) action {
	return actionForBindings(m.normal, ev)
}

func (m keyMap) helpAction(ev *tcell.EventKey) action {
	return actionForBindings(m.help, ev)
}

func (m keyMap) promptAction(ev *tcell.EventKey) action {
	return actionForBindings(m.prompt, ev)
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
	if ctrlKeyBindingMatches(b, ev) {
		return true
	}
	if ev.Key() != b.key {
		return false
	}
	if !b.anyMod && modifiersForMatch(ev, b.key) != b.mod {
		return false
	}
	if b.key == tcell.KeyRune && ev.Str() != b.rune {
		return false
	}
	return true
}

func modifiersForMatch(ev *tcell.EventKey, key tcell.Key) tcell.ModMask {
	mod := ev.Modifiers()
	if key == tcell.KeyRune {
		mod &^= tcell.ModShift
	}
	return mod
}

func ctrlKeyBindingMatches(b keyBinding, ev *tcell.EventKey) bool {
	if b.key != tcell.KeyRune || b.mod != tcell.ModCtrl || len(b.rune) != 1 {
		return false
	}
	want, ok := ctrlKeyForRune(rune(b.rune[0]))
	if !ok {
		return false
	}
	if ev.Key() != want {
		return false
	}
	if b.anyMod {
		return true
	}
	return ev.Modifiers() == tcell.ModCtrl || ev.Modifiers() == tcell.ModNone
}

func ctrlKeyForRune(r rune) (tcell.Key, bool) {
	switch {
	case r >= 'a' && r <= 'z':
		return tcell.Key(int(tcell.KeyCtrlA) + int(r-'a')), true
	case r >= 'A' && r <= 'Z':
		return tcell.Key(int(tcell.KeyCtrlA) + int(r-'A')), true
	default:
		return tcell.KeyRune, false
	}
}

func removeBindings(bindings []keyBinding, stroke KeyStroke) []keyBinding {
	if len(bindings) == 0 {
		return nil
	}
	filtered := bindings[:0]
	for _, binding := range bindings {
		if binding.key == stroke.Key &&
			binding.rune == stroke.Rune &&
			binding.mod == stroke.Modifiers &&
			binding.anyMod == stroke.AnyModifier {
			continue
		}
		filtered = append(filtered, binding)
	}
	return filtered
}
