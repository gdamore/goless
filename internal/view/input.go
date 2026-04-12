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
	promptSave
)

type promptState struct {
	kind        promptKind
	prefix      string
	editor      lineEditor
	seeded      bool
	preview     *searchState
	export      ExportOptions
	saveConfirm *saveConfirmState
	errText     string
	history     promptHistoryState
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
	promptHistorySave
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
	actionScrollUpStep
	actionScrollDownStep
	actionScrollLeft
	actionScrollRight
	actionScrollLeftFine
	actionScrollRightFine
	actionHalfScreenLeft
	actionHalfScreenRight
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

func actionToKeyAction(a action) KeyAction {
	switch a {
	case actionQuit:
		return KeyActionQuit
	case actionScrollUp:
		return KeyActionScrollUp
	case actionScrollDown:
		return KeyActionScrollDown
	case actionScrollUpStep:
		return KeyActionScrollUpStep
	case actionScrollDownStep:
		return KeyActionScrollDownStep
	case actionScrollLeft:
		return KeyActionScrollLeft
	case actionScrollRight:
		return KeyActionScrollRight
	case actionScrollLeftFine:
		return KeyActionScrollLeftFine
	case actionScrollRightFine:
		return KeyActionScrollRightFine
	case actionHalfScreenLeft:
		return KeyActionHalfScreenLeft
	case actionHalfScreenRight:
		return KeyActionHalfScreenRight
	case actionHalfPageUp:
		return KeyActionHalfPageUp
	case actionHalfPageDown:
		return KeyActionHalfPageDown
	case actionPageUp:
		return KeyActionPageUp
	case actionPageDown:
		return KeyActionPageDown
	case actionGoLineStart:
		return KeyActionGoLineStart
	case actionGoLineEnd:
		return KeyActionGoLineEnd
	case actionGoTop:
		return KeyActionGoTop
	case actionGoBottom:
		return KeyActionGoBottom
	case actionToggleWrap:
		return KeyActionToggleWrap
	case actionPromptSearchForward:
		return KeyActionPromptSearchForward
	case actionPromptSearchBackward:
		return KeyActionPromptSearchBackward
	case actionPromptCommand:
		return KeyActionPromptCommand
	case actionSearchNext:
		return KeyActionSearchNext
	case actionSearchPrev:
		return KeyActionSearchPrev
	case actionRefresh:
		return KeyActionRefresh
	case actionToggleHelp:
		return KeyActionToggleHelp
	case actionFollow:
		return KeyActionFollow
	case actionStopFollow:
		return KeyActionStopFollow
	case actionCycleSearchCase:
		return KeyActionCycleSearchCase
	case actionCycleSearchMode:
		return KeyActionCycleSearchMode
	default:
		return KeyActionNone
	}
}

// MouseResult summarizes how the viewer handled a mouse event.
type MouseResult struct {
	Handled bool
	Action  KeyAction
	Context KeyContext
}
