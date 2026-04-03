// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/goless/internal/ansi"
	"github.com/gdamore/goless/internal/layout"
	"github.com/gdamore/goless/internal/model"
	"github.com/gdamore/tcell/v3"
	tcolor "github.com/gdamore/tcell/v3/color"
	"github.com/gdamore/tcell/v3/vt"
)

func TestToggleWrapPreservesAnchor(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("ab界cd")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(3, 2)
	v.colOffset = 2
	v.relayout()

	before := v.firstVisibleAnchor()
	if got, want := before.GraphemeIndex, 2; got != want {
		t.Fatalf("anchor grapheme = %d, want %d", got, want)
	}

	v.ToggleWrap()
	after := v.firstVisibleAnchor()
	if got, want := after, before; got != want {
		t.Fatalf("anchor after toggle = %+v, want %+v", got, want)
	}
	if got, want := v.cfg.WrapMode, layout.SoftWrap; got != want {
		t.Fatalf("wrap mode = %v, want %v", got, want)
	}
}

func TestLessKeyMapGoBottom(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\nfour\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	v.HandleKey(keyRune("G"))

	if got, want := v.firstVisibleAnchor().LineIndex, 3; got != want {
		t.Fatalf("visible line after G = %d, want %d", got, want)
	}
	if v.follow {
		t.Fatalf("follow after G = true, want false")
	}
}

func TestLessKeyMapHalfPageMoves(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\nfour\nfive\nsix\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 4)

	v.HandleKey(keyRune("d"))
	if got, want := v.Position().Row, 2; got != want {
		t.Fatalf("Position().Row after d = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("u"))
	if got, want := v.Position().Row, 1; got != want {
		t.Fatalf("Position().Row after u = %d, want %d", got, want)
	}

	v.HandleKey(keyCtrlRune("d"))
	if got, want := v.Position().Row, 2; got != want {
		t.Fatalf("Position().Row after Ctrl-D = %d, want %d", got, want)
	}

	v.HandleKey(keyCtrlRune("u"))
	if got, want := v.Position().Row, 1; got != want {
		t.Fatalf("Position().Row after Ctrl-U = %d, want %d", got, want)
	}

	v.HandleKey(keyKey(tcell.KeyCtrlD))
	if got, want := v.Position().Row, 2; got != want {
		t.Fatalf("Position().Row after KeyCtrlD = %d, want %d", got, want)
	}

	v.HandleKey(keyKey(tcell.KeyCtrlU))
	if got, want := v.Position().Row, 1; got != want {
		t.Fatalf("Position().Row after KeyCtrlU = %d, want %d", got, want)
	}
}

func TestLessKeyMapHelpScrollsAndCloses(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("hello\nworld\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 4)
	v.mode = modeHelp

	v.HandleKey(keyKey(tcell.KeyDown))
	if got, want := v.helpOffset, 1; got != want {
		t.Fatalf("help offset after down = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("H"))
	if got, want := v.mode, modeNormal; got != want {
		t.Fatalf("mode after H = %v, want %v", got, want)
	}
}

func TestLessKeyMapHelpUsesNormalNavigationKeys(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Text: Text{
			HelpBody: "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\n",
		},
	})
	v.SetSize(20, 4)
	v.mode = modeHelp

	v.HandleKey(keyRune("j"))
	if got, want := v.helpOffset, 1; got != want {
		t.Fatalf("help offset after j = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("d"))
	if got, want := v.helpOffset, 2; got != want {
		t.Fatalf("help offset after d = %d, want %d", got, want)
	}

	v.HandleKey(keyRune(" "))
	if got, want := v.helpOffset, 5; got != want {
		t.Fatalf("help offset after space = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("b"))
	if got, want := v.helpOffset, 2; got != want {
		t.Fatalf("help offset after b = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("G"))
	if got, want := v.helpOffset, v.maxHelpOffset(); got != want {
		t.Fatalf("help offset after G = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("g"))
	if got, want := v.helpOffset, 0; got != want {
		t.Fatalf("help offset after g = %d, want %d", got, want)
	}
}

func TestViewerRuntimeSettersUpdateConfigAndState(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("a\tb\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	v.SetTheme(Theme{DefaultFG: tcolor.Green, DefaultBG: tcolor.Navy})
	if got, want := v.cfg.Theme.DefaultFG, tcolor.Green; got != want {
		t.Fatalf("theme fg = %v, want %v", got, want)
	}
	if got, want := v.cfg.Theme.DefaultBG, tcolor.Navy; got != want {
		t.Fatalf("theme bg = %v, want %v", got, want)
	}

	v.SetTabWidth(-1)
	if got, want := v.cfg.TabWidth, 8; got != want {
		t.Fatalf("tab width = %d, want %d", got, want)
	}

	v.SetVisualization(Visualization{ShowTabs: true, TabGlyph: ">"})
	if !v.cfg.Visualization.ShowTabs {
		t.Fatal("visualization ShowTabs = false, want true")
	}

	handler := func(HyperlinkInfo) HyperlinkDecision { return HyperlinkDecision{Live: true} }
	v.SetHyperlinkHandler(handler)
	if v.cfg.HyperlinkHandler == nil {
		t.Fatal("hyperlink handler = nil, want set")
	}

	commandHandler := func(Command) CommandResult { return CommandResult{Handled: true} }
	v.SetCommandHandler(commandHandler)
	if v.cfg.CommandHandler == nil {
		t.Fatal("command handler = nil, want set")
	}

	frame := Frame{
		Horizontal:  "-",
		Vertical:    "|",
		TopLeft:     "+",
		TopRight:    "+",
		BottomLeft:  "+",
		BottomRight: "+",
	}
	chrome := Chrome{Frame: frame, Title: "Demo"}
	v.SetChrome(chrome)
	if got, want := v.cfg.Chrome.Title, "Demo"; got != want {
		t.Fatalf("chrome title = %q, want %q", got, want)
	}
	if got, want := v.cfg.Chrome.Frame.TopLeft, frame.TopLeft; got != want {
		t.Fatalf("chrome frame top-left = %q, want %q", got, want)
	}

	v.SetShowStatus(false)
	if v.cfg.ShowStatus {
		t.Fatal("ShowStatus = true, want false")
	}

	v.SetText(Text{StatusHelpHint: "Custom", FollowMode: "tail"})
	if got, want := v.text.StatusHelpHint, "Custom"; got != want {
		t.Fatalf("status help hint = %q, want %q", got, want)
	}
	if got, want := v.text.FollowMode, "tail"; got != want {
		t.Fatalf("follow mode text = %q, want %q", got, want)
	}
}

func TestLessKeyMapHelpUsesHorizontalNavigationKeys(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Text: Text{
			HelpBody: "abcdefghijklmnopqrstuvwxyz\n",
		},
	})
	v.SetSize(20, 4)
	v.mode = modeHelp

	v.HandleKey(keyKey(tcell.KeyRight))
	if got, want := v.helpColOffset, 5; got != want {
		t.Fatalf("help col offset after Right = %d, want %d", got, want)
	}

	v.HandleKey(keyKeyMod(tcell.KeyLeft, tcell.ModShift))
	if got, want := v.helpColOffset, 4; got != want {
		t.Fatalf("help col offset after Shift-Left = %d, want %d", got, want)
	}

	v.HandleKey(keyRune(">"))
	if got, want := v.helpColOffset, 5; got != want {
		t.Fatalf("help col offset after > = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("<"))
	if got, want := v.helpColOffset, 4; got != want {
		t.Fatalf("help col offset after < = %d, want %d", got, want)
	}
}

func TestLessKeyMapHelpLineAndDocumentNavigationAreDistinct(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Text: Text{
			HelpBody: "abcdefghijklmnopqrstuvwxyz\none\ntwo\nthree\nfour\nfive\nsix\n",
		},
	})
	v.SetSize(20, 4)
	v.mode = modeHelp

	v.HandleKey(keyKey(tcell.KeyEnd))
	if got, want := v.helpColOffset, 6; got != want {
		t.Fatalf("help col offset after End = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("0"))
	if got, want := v.helpColOffset, 0; got != want {
		t.Fatalf("help col offset after 0 = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("G"))
	if got, want := v.helpOffset, v.maxHelpOffset(); got != want {
		t.Fatalf("help offset after G = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("g"))
	if got, want := v.helpOffset, 0; got != want {
		t.Fatalf("help offset after g = %d, want %d", got, want)
	}
}

func TestHelpGoLineEndUsesCurrentLineWidth(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Text: Text{
			HelpBody: "abcdefghijklmnopqrstuv\nabcdefghijklmnopqrstuvwxyz0123456789\n",
		},
	})
	v.SetSize(20, 4)
	v.mode = modeHelp

	v.HandleKey(keyKey(tcell.KeyEnd))
	if got, want := v.helpColOffset, 2; got != want {
		t.Fatalf("help col offset after End on first line = %d, want %d", got, want)
	}

	v.helpOffset = 1
	v.HandleKey(keyKey(tcell.KeyEnd))
	if got, want := v.helpColOffset, 16; got != want {
		t.Fatalf("help col offset after End on second line = %d, want %d", got, want)
	}
}

func TestCommandPercentJump(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 4)

	v.HandleKey(keyRune(":"))
	for _, s := range []string{"5", "0", "%"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.Position().Row, 5; got != want {
		t.Fatalf("Position().Row after :50%% = %d, want %d", got, want)
	}
}

func TestSetNumbersCommand(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 4)

	for _, command := range []string{"set numbers on", "set numbers toggle"} {
		v.HandleKey(keyRune(":"))
		for _, s := range command {
			v.HandleKey(keyRune(string(s)))
		}
		v.HandleKey(keyKey(tcell.KeyEnter))
	}

	if v.LineNumbers() {
		t.Fatalf("LineNumbers after on+toggle = true, want false")
	}
}

func TestSetHeadersCommand(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 4)

	v.HandleKey(keyRune(":"))
	for _, s := range "set headers 1" {
		v.HandleKey(keyRune(string(s)))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.HeaderLines(), 1; got != want {
		t.Fatalf("HeaderLines after :set headers 1 = %d, want %d", got, want)
	}
}

func TestSetHeadersCommandClearsPriorInvalidMessage(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 4)

	v.HandleKey(keyRune(":"))
	for _, s := range "set header on" {
		v.HandleKey(keyRune(string(s)))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))
	if !strings.Contains(v.message, "set header on") {
		t.Fatalf("message after invalid command = %q, want it to mention invalid command", v.message)
	}

	v.HandleKey(keyRune(":"))
	for _, s := range "set headers on" {
		v.HandleKey(keyRune(string(s)))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.HeaderLines(), 1; got != want {
		t.Fatalf("HeaderLines after :set headers on = %d, want %d", got, want)
	}
	if strings.Contains(v.message, "set header on") {
		t.Fatalf("message after valid command = %q, still mentions prior invalid command", v.message)
	}
}

func TestSetSqueezeCommand(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\n\n\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 4)

	for _, command := range []string{"set squeeze on", "set squeeze toggle"} {
		v.HandleKey(keyRune(":"))
		for _, s := range command {
			v.HandleKey(keyRune(string(s)))
		}
		v.HandleKey(keyKey(tcell.KeyEnter))
	}

	if v.SqueezeBlankLines() {
		t.Fatal("SqueezeBlankLines after on+toggle = true, want false")
	}
}

func TestSetHeaderColumnsCommand(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 4)

	v.HandleKey(keyRune(":"))
	for _, s := range "set headercols 2" {
		v.HandleKey(keyRune(string(s)))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.HeaderColumns(), 2; got != want {
		t.Fatalf("HeaderColumns after :set headercols 2 = %d, want %d", got, want)
	}
}

func TestKeyBindingMatchesRequireExactNoModifierByDefault(t *testing.T) {
	binding := keyBinding{
		key:    tcell.KeyRune,
		rune:   "q",
		mod:    tcell.ModNone,
		action: actionQuit,
	}

	if !binding.matches(tcell.NewEventKey(tcell.KeyRune, "q", tcell.ModNone)) {
		t.Fatalf("unmodified q should match")
	}
	if binding.matches(tcell.NewEventKey(tcell.KeyRune, "q", tcell.ModAlt)) {
		t.Fatalf("modified q should not match exact unmodified binding")
	}
}

func TestKeyBindingMatchesShiftedRuneWithoutExplicitShiftModifier(t *testing.T) {
	binding := keyBinding{
		key:    tcell.KeyRune,
		rune:   "$",
		mod:    tcell.ModNone,
		action: actionGoLineEnd,
	}

	if !binding.matches(tcell.NewEventKey(tcell.KeyRune, "$", tcell.ModShift)) {
		t.Fatalf("shifted $ should match rune binding without explicit shift modifier")
	}
}

func TestKeyBindingMatchesExplicitShiftedRuneBinding(t *testing.T) {
	binding := keyBinding{
		key:    tcell.KeyRune,
		rune:   "$",
		mod:    tcell.ModShift,
		action: actionGoLineEnd,
	}

	if !binding.matches(tcell.NewEventKey(tcell.KeyRune, "$", tcell.ModShift)) {
		t.Fatalf("shifted $ should match rune binding that explicitly requires shift")
	}
}

func TestKeyBindingAnyModifierWildcard(t *testing.T) {
	binding := keyBinding{
		key:    tcell.KeyF1,
		anyMod: true,
		action: actionToggleHelp,
	}

	if !binding.matches(tcell.NewEventKey(tcell.KeyF1, "", tcell.ModAlt)) {
		t.Fatalf("F1 with modifiers should match wildcard binding")
	}
}

func TestPromptBindingsRejectUnsupportedActions(t *testing.T) {
	m := defaultKeyMap(KeyGroupLess).withOverrides(nil, []KeyBinding{
		{
			KeyStroke: KeyStroke{Context: KeyContextPrompt, Key: tcell.KeyF5},
			Action:    KeyActionScrollDown,
		},
		{
			KeyStroke: KeyStroke{Context: KeyContextPrompt, Key: tcell.KeyF6},
			Action:    KeyActionCycleSearchMode,
		},
	})

	if got, want := m.promptAction(tcell.NewEventKey(tcell.KeyF5, "", tcell.ModNone)), actionNone; got != want {
		t.Fatalf("promptAction(F5) = %v, want %v for unsupported prompt action", got, want)
	}
	if got, want := m.promptAction(tcell.NewEventKey(tcell.KeyF6, "", tcell.ModNone)), actionCycleSearchMode; got != want {
		t.Fatalf("promptAction(F6) = %v, want %v for supported prompt action", got, want)
	}
}

func TestToggleWrapToNoWrapRestoresHorizontalAnchor(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("ab界cd")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.SoftWrap})
	v.SetSize(3, 2)
	v.rowOffset = 1
	v.relayout()

	before := v.firstVisibleAnchor()
	if got, want := before.GraphemeIndex, 2; got != want {
		t.Fatalf("anchor grapheme = %d, want %d", got, want)
	}

	v.ToggleWrap()

	if got, want := v.cfg.WrapMode, layout.NoWrap; got != want {
		t.Fatalf("wrap mode = %v, want %v", got, want)
	}
	if got, want := v.colOffset, 2; got != want {
		t.Fatalf("col offset = %d, want %d", got, want)
	}
	after := v.firstVisibleAnchor()
	if got, want := after, before; got != want {
		t.Fatalf("anchor after toggle = %+v, want %+v", got, want)
	}
}

func TestScrollRightClampsToContent(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdef")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(3, 4)
	v.ScrollRight(100)

	if got, want := v.colOffset, 3; got != want {
		t.Fatalf("col offset = %d, want %d", got, want)
	}
}

func TestHorizontalScrollStepUsesQuarterWidthCappedAtEight(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdefghijklmnopqrstuvwxyz\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(20, 4)

	v.HandleKey(keyKey(tcell.KeyRight))
	if got, want := v.colOffset, 5; got != want {
		t.Fatalf("col offset after Right = %d, want %d", got, want)
	}

	v.HandleKey(keyKey(tcell.KeyLeft))
	if got, want := v.colOffset, 0; got != want {
		t.Fatalf("col offset after Left = %d, want %d", got, want)
	}
}

func TestAngleBracketsRemainFineHorizontalScroll(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdefghijklmnopqrstuvwxyz\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(20, 4)

	v.HandleKey(keyRune(">"))
	if got, want := v.colOffset, 1; got != want {
		t.Fatalf("col offset after > = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("<"))
	if got, want := v.colOffset, 0; got != want {
		t.Fatalf("col offset after < = %d, want %d", got, want)
	}

	v.HandleKey(keyRuneMod(">", tcell.ModShift))
	if got, want := v.colOffset, 1; got != want {
		t.Fatalf("col offset after shifted > = %d, want %d", got, want)
	}

	v.HandleKey(keyRuneMod("<", tcell.ModShift))
	if got, want := v.colOffset, 0; got != want {
		t.Fatalf("col offset after shifted < = %d, want %d", got, want)
	}
}

func TestShiftArrowKeysRemainFineHorizontalScroll(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdefghijklmnopqrstuvwxyz\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(20, 4)

	v.HandleKey(keyKeyMod(tcell.KeyRight, tcell.ModShift))
	if got, want := v.colOffset, 1; got != want {
		t.Fatalf("col offset after Shift-Right = %d, want %d", got, want)
	}

	v.HandleKey(keyKeyMod(tcell.KeyLeft, tcell.ModShift))
	if got, want := v.colOffset, 0; got != want {
		t.Fatalf("col offset after Shift-Left = %d, want %d", got, want)
	}
}

func TestGoLineStartAndEnd(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdefghij\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(5, 4)

	v.GoLineEnd()
	if got, want := v.colOffset, 5; got != want {
		t.Fatalf("col offset after GoLineEnd = %d, want %d", got, want)
	}

	v.GoLineStart()
	if got, want := v.colOffset, 0; got != want {
		t.Fatalf("col offset after GoLineStart = %d, want %d", got, want)
	}

	v.HandleKey(keyKey(tcell.KeyEnd))
	if got, want := v.colOffset, 5; got != want {
		t.Fatalf("col offset after End = %d, want %d", got, want)
	}

	v.HandleKey(keyKey(tcell.KeyHome))
	if got, want := v.colOffset, 0; got != want {
		t.Fatalf("col offset after Home = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("$"))
	if got, want := v.colOffset, 5; got != want {
		t.Fatalf("col offset after $ = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("0"))
	if got, want := v.colOffset, 0; got != want {
		t.Fatalf("col offset after 0 = %d, want %d", got, want)
	}
}

func TestPromptSearchFindsAndRepeatsForward(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\nbeta\nalpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"a", "l", "p", "h", "a"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.search.Query, "alpha"; got != want {
		t.Fatalf("search query = %q, want %q", got, want)
	}
	if got, want := len(v.search.Matches), 2; got != want {
		t.Fatalf("match count = %d, want %d", got, want)
	}
	if got, want := v.search.Current, 0; got != want {
		t.Fatalf("current match = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("n"))
	if got, want := v.search.Current, 1; got != want {
		t.Fatalf("current match after n = %d, want %d", got, want)
	}
	if got, want := v.firstVisibleAnchor().LineIndex, 2; got != want {
		t.Fatalf("visible line after n = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("N"))
	if got, want := v.search.Current, 0; got != want {
		t.Fatalf("current match after N = %d, want %d", got, want)
	}
}

func TestPromptSearchUsesSmartCaseByDefault(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("Alpha\nbeta\nALPHA\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"a", "l", "p", "h", "a"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := len(v.search.Matches), 2; got != want {
		t.Fatalf("match count = %d, want %d", got, want)
	}
}

func TestPromptSearchProgressivelyNarrowsMatches(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\nalpine\nbeta\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	v.HandleKey(keyRune("/"))
	v.HandleKey(keyRune("a"))
	if v.prompt == nil || v.prompt.preview == nil {
		t.Fatal("prompt preview = nil, want live search preview")
	}
	if got, want := len(v.prompt.preview.Matches), 4; got != want {
		t.Fatalf("preview match count after /a = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("l"))
	v.HandleKey(keyRune("p"))
	if got, want := len(v.prompt.preview.Matches), 2; got != want {
		t.Fatalf("preview match count after /alp = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("h"))
	if got, want := len(v.prompt.preview.Matches), 1; got != want {
		t.Fatalf("preview match count after /alph = %d, want %d", got, want)
	}
}

func TestPromptSearchCancelRestoresCommittedSearch(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\nalpine\nbeta\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	if !v.SearchForward("alpha") {
		t.Fatal("SearchForward(alpha) = false, want true")
	}

	v.HandleKey(keyRune("/"))
	v.HandleKey(keyRune("a"))
	v.HandleKey(keyRune("l"))
	v.HandleKey(keyRune("p"))
	v.HandleKey(keyKey(tcell.KeyEscape))

	if got, want := v.search.Query, "alpha"; got != want {
		t.Fatalf("search query after Esc = %q, want %q", got, want)
	}
	if got, want := len(v.search.Matches), 1; got != want {
		t.Fatalf("match count after Esc = %d, want %d", got, want)
	}
}

func TestPromptSearchRespectsCaseSensitiveMode(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("Alpha\nbeta\nALPHA\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, SearchCase: SearchCaseSensitive, ShowStatus: true})
	v.SetSize(20, 2)

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"a", "l", "p", "h", "a"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := len(v.search.Matches), 0; got != want {
		t.Fatalf("match count = %d, want %d", got, want)
	}
}

func TestF2CyclesSearchCaseMode(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	v.HandleKey(keyKey(tcell.KeyF2))
	if got, want := v.SearchCaseMode(), SearchCaseSensitive; got != want {
		t.Fatalf("SearchCaseMode after first F2 = %v, want %v", got, want)
	}
	v.HandleKey(keyKey(tcell.KeyF2))
	if got, want := v.SearchCaseMode(), SearchCaseInsensitive; got != want {
		t.Fatalf("SearchCaseMode after second F2 = %v, want %v", got, want)
	}
	v.HandleKey(keyKey(tcell.KeyF2))
	if got, want := v.SearchCaseMode(), SearchSmartCase; got != want {
		t.Fatalf("SearchCaseMode after third F2 = %v, want %v", got, want)
	}
}

func TestF3CyclesSearchMode(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	v.HandleKey(keyKey(tcell.KeyF3))
	if got, want := v.SearchMode(), SearchWholeWord; got != want {
		t.Fatalf("SearchMode after first F3 = %v, want %v", got, want)
	}
	v.HandleKey(keyKey(tcell.KeyF3))
	if got, want := v.SearchMode(), SearchRegex; got != want {
		t.Fatalf("SearchMode after second F3 = %v, want %v", got, want)
	}
	v.HandleKey(keyKey(tcell.KeyF3))
	if got, want := v.SearchMode(), SearchSubstring; got != want {
		t.Fatalf("SearchMode after third F3 = %v, want %v", got, want)
	}
}

func TestF2UpdatesPromptPrefix(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.HandleKey(keyRune("/"))
	if got, want := v.prompt.String(), "/"; got != want {
		t.Fatalf("prompt prefix = %q, want %q", got, want)
	}

	v.HandleKey(keyKey(tcell.KeyF2))
	if got, want := v.prompt.String(), "/"; got != want {
		t.Fatalf("prompt prefix after F2 = %q, want %q", got, want)
	}

	v.HandleKey(keyKey(tcell.KeyF3))
	if got, want := v.prompt.String(), "/"; got != want {
		t.Fatalf("prompt prefix after F3 = %q, want %q", got, want)
	}

	v.HandleKey(keyKey(tcell.KeyF3))
	if got, want := v.prompt.String(), "/"; got != want {
		t.Fatalf("prompt prefix after second F3 = %q, want %q", got, want)
	}
}

func TestReenterSearchPromptPrefillsActiveQuery(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\nbeta\nalpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	if !v.SearchForward("alpha") {
		t.Fatal("SearchForward(alpha) = false, want true")
	}

	v.HandleKey(keyRune("/"))
	if got, want := v.prompt.input(), "alpha"; got != want {
		t.Fatalf("prompt buffer after reopening / = %q, want %q", got, want)
	}
	if !v.prompt.seeded {
		t.Fatal("prompt seeded after reopening / = false, want true")
	}
	if v.prompt.preview == nil || v.prompt.preview.Query != "alpha" {
		t.Fatalf("prompt preview after reopening / = %#v, want query alpha", v.prompt.preview)
	}
	v.HandleKey(keyRune("b"))
	if got, want := v.prompt.input(), "b"; got != want {
		t.Fatalf("prompt buffer after typing over seeded query = %q, want %q", got, want)
	}

	v.cancelPrompt()
	v.HandleKey(keyRune("?"))
	if got, want := v.prompt.input(), "alpha"; got != want {
		t.Fatalf("prompt buffer after reopening ? = %q, want %q", got, want)
	}
}

func TestPromptEditorSupportsInlineCursorEditing(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(20, 2)

	v.HandleKey(keyRune(":"))
	v.HandleKey(keyRune("a"))
	v.HandleKey(keyRune("c"))
	v.HandleKey(keyKey(tcell.KeyLeft))
	v.HandleKey(keyRune("b"))

	if got, want := v.prompt.input(), "abc"; got != want {
		t.Fatalf("prompt input after inline edit = %q, want %q", got, want)
	}
	if got, want := v.prompt.cursor(), 2; got != want {
		t.Fatalf("prompt cursor after inline edit = %d, want %d", got, want)
	}
}

func TestPromptCtrlBAndCtrlFMoveCursor(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(20, 2)

	v.HandleKey(keyRune(":"))
	for _, s := range []string{"a", "b", "c"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyCtrlB))
	v.HandleKey(keyKey(tcell.KeyCtrlB))
	if got, want := v.prompt.cursor(), 1; got != want {
		t.Fatalf("cursor after Ctrl-B = %d, want %d", got, want)
	}
	v.HandleKey(keyKey(tcell.KeyCtrlF))
	if got, want := v.prompt.cursor(), 2; got != want {
		t.Fatalf("cursor after Ctrl-F = %d, want %d", got, want)
	}
}

func TestPromptEditorOverwriteModeReplacesAtCursor(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(20, 2)

	v.HandleKey(keyRune(":"))
	for _, s := range []string{"a", "b", "c"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyLeft))
	v.HandleKey(keyKey(tcell.KeyLeft))
	v.HandleKey(keyKey(tcell.KeyInsert))
	v.HandleKey(keyRune("Z"))

	if got, want := v.prompt.input(), "aZc"; got != want {
		t.Fatalf("prompt input after overwrite edit = %q, want %q", got, want)
	}
	if !v.prompt.editor.Overwrite() {
		t.Fatal("overwrite mode = false, want true after Insert")
	}
}

func TestPromptEditorTreatsCombiningSequenceAsSingleCharacter(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(20, 2)

	v.HandleKey(keyRune(":"))
	v.HandleKey(keyRune("e\u0301"))
	v.HandleKey(keyRune("x"))
	if got, want := v.prompt.cursor(), 2; got != want {
		t.Fatalf("cursor after grapheme insert = %d, want %d", got, want)
	}

	v.HandleKey(keyKey(tcell.KeyCtrlB))
	if got, want := v.prompt.cursor(), 1; got != want {
		t.Fatalf("cursor after Ctrl-B over grapheme = %d, want %d", got, want)
	}

	v.HandleKey(keyKey(tcell.KeyBackspace))
	if got, want := v.prompt.input(), "x"; got != want {
		t.Fatalf("prompt input after grapheme backspace = %q, want %q", got, want)
	}
	if got, want := v.prompt.cursor(), 0; got != want {
		t.Fatalf("cursor after grapheme backspace = %d, want %d", got, want)
	}
}

func TestPromptCtrlKDeletesToEndOfLine(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(20, 2)

	v.HandleKey(keyRune(":"))
	for _, s := range []string{"a", "b", "c", "d", "e"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyLeft))
	v.HandleKey(keyKey(tcell.KeyLeft))
	v.HandleKey(keyKey(tcell.KeyCtrlK))

	if got, want := v.prompt.input(), "abc"; got != want {
		t.Fatalf("prompt input after Ctrl-K = %q, want %q", got, want)
	}
	if got, want := v.prompt.cursor(), 3; got != want {
		t.Fatalf("cursor after Ctrl-K = %d, want %d", got, want)
	}
}

func TestPromptCtrlWDeletesPreviousWord(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(20, 2)

	v.HandleKey(keyRune(":"))
	for _, s := range []string{"a", "l", "p", "h", "a", " ", " ", "b", "e", "t", "a"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyCtrlW))

	if got, want := v.prompt.input(), "alpha  "; got != want {
		t.Fatalf("prompt input after first Ctrl-W = %q, want %q", got, want)
	}
	if got, want := v.prompt.cursor(), 7; got != want {
		t.Fatalf("cursor after first Ctrl-W = %d, want %d", got, want)
	}

	v.HandleKey(keyKey(tcell.KeyCtrlW))
	if got, want := v.prompt.input(), ""; got != want {
		t.Fatalf("prompt input after second Ctrl-W = %q, want empty", got)
	}
	if got, want := v.prompt.cursor(), 0; got != want {
		t.Fatalf("cursor after second Ctrl-W = %d, want %d", got, want)
	}
}

func TestPromptCtrlUDeletesToBeginningOfLine(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(20, 2)

	v.HandleKey(keyRune(":"))
	for _, s := range []string{"a", "b", "c", "d", "e"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyLeft))
	v.HandleKey(keyKey(tcell.KeyLeft))
	v.HandleKey(keyKey(tcell.KeyCtrlU))

	if got, want := v.prompt.input(), "de"; got != want {
		t.Fatalf("prompt input after Ctrl-U = %q, want %q", got, want)
	}
	if got, want := v.prompt.cursor(), 0; got != want {
		t.Fatalf("cursor after Ctrl-U = %d, want %d", got, want)
	}
}

func TestPromptHistoryRecallIsPromptKindAware(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\nbeta\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(20, 2)

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"a", "l", "p", "h", "a"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	v.HandleKey(keyRune(":"))
	v.HandleKey(keyRune("1"))
	v.HandleKey(keyKey(tcell.KeyEnter))

	v.HandleKey(keyRune("?"))
	for _, s := range []string{"b", "e", "t", "a"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	v.HandleKey(keyRune("/"))
	v.HandleKey(keyKey(tcell.KeyUp))
	if got, want := v.prompt.input(), "beta"; got != want {
		t.Fatalf("search history newest entry = %q, want %q", got, want)
	}
	v.HandleKey(keyKey(tcell.KeyUp))
	if got, want := v.prompt.input(), "alpha"; got != want {
		t.Fatalf("search history older entry = %q, want %q", got, want)
	}

	v.cancelPrompt()
	v.HandleKey(keyRune(":"))
	v.HandleKey(keyKey(tcell.KeyUp))
	if got, want := v.prompt.input(), "1"; got != want {
		t.Fatalf("command history entry = %q, want %q", got, want)
	}
}

func TestPromptHistoryDownRestoresDraft(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(20, 2)

	v.HandleKey(keyRune(":"))
	for _, s := range []string{"1", "2"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	v.HandleKey(keyRune(":"))
	for _, s := range []string{"n", "e"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyUp))
	if got, want := v.prompt.input(), "12"; got != want {
		t.Fatalf("recalled history entry = %q, want %q", got, want)
	}
	v.HandleKey(keyKey(tcell.KeyDown))
	if got, want := v.prompt.input(), "ne"; got != want {
		t.Fatalf("restored draft after history = %q, want %q", got, want)
	}
}

func TestSetSearchCaseCommand(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("Alpha\nbeta\nALPHA\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	for _, command := range []string{"set searchcase case", "set searchcase nocase", "set searchcase smart"} {
		v.HandleKey(keyRune(":"))
		for _, s := range command {
			v.HandleKey(keyRune(string(s)))
		}
		v.HandleKey(keyKey(tcell.KeyEnter))
	}

	if got, want := v.SearchCaseMode(), SearchSmartCase; got != want {
		t.Fatalf("SearchCaseMode after :set commands = %v, want %v", got, want)
	}
}

func TestSetSearchModeCommand(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alphabet alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	for _, command := range []string{"set searchmode word", "set searchmode regex", "set searchmode sub"} {
		v.HandleKey(keyRune(":"))
		for _, s := range command {
			v.HandleKey(keyRune(string(s)))
		}
		v.HandleKey(keyKey(tcell.KeyEnter))
	}

	if got, want := v.SearchMode(), SearchSubstring; got != want {
		t.Fatalf("SearchMode after :set commands = %v, want %v", got, want)
	}
}

func TestPromptSearchPreservesWhitespace(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("a b\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	v.HandleKey(keyRune("/"))
	v.HandleKey(keyRune(" "))
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.search.Query, " "; got != want {
		t.Fatalf("search query after space search = %q, want %q", got, want)
	}
	if got, want := len(v.search.Matches), 1; got != want {
		t.Fatalf("space search match count = %d, want %d", got, want)
	}
}

func TestPromptCommitRebuildsSearchAfterDocumentChange(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"b", "e", "t", "a"} {
		v.HandleKey(keyRune(s))
	}
	if v.prompt == nil || v.prompt.preview == nil || len(v.prompt.preview.Matches) != 0 {
		t.Fatal("expected zero cached preview matches before document change")
	}

	if err := doc.Append([]byte("beta\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	v.Refresh()
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := len(v.search.Matches), 1; got != want {
		t.Fatalf("committed match count after document change = %d, want %d", got, want)
	}
}

func TestWholeWordSearchSkipsSubstrings(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alphabet alpha alpha_beta\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true, SearchMode: SearchWholeWord})
	v.SetSize(20, 2)

	if !v.SearchForward("alpha") {
		t.Fatal("SearchForward(alpha) = false, want true")
	}
	if got, want := len(v.search.Matches), 1; got != want {
		t.Fatalf("whole-word match count = %d, want %d", got, want)
	}
	if got, want := v.search.Matches[0].StartRune, 9; got != want {
		t.Fatalf("whole-word match start rune = %d, want %d", got, want)
	}
}

func TestF3UpdatesPromptPreviewMatches(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alphabet\nalpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"a", "l", "p", "h", "a"} {
		v.HandleKey(keyRune(s))
	}
	if got, want := len(v.prompt.preview.Matches), 2; got != want {
		t.Fatalf("substring preview match count = %d, want %d", got, want)
	}

	v.HandleKey(keyKey(tcell.KeyF3))
	if got, want := len(v.prompt.preview.Matches), 1; got != want {
		t.Fatalf("whole-word preview match count = %d, want %d", got, want)
	}
}

func TestRegexSearchMatchesPattern(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("error 500\nwarning 404\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true, SearchMode: SearchRegex})
	v.SetSize(20, 2)

	if !v.SearchForward(`[45]0[04]`) {
		t.Fatal("SearchForward(regex) = false, want true")
	}
	if got, want := len(v.search.Matches), 2; got != want {
		t.Fatalf("regex match count = %d, want %d", got, want)
	}
}

func TestInvalidRegexPreviewKeepsLastValidMatches(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true, SearchMode: SearchRegex})
	v.SetSize(20, 2)

	v.HandleKey(keyRune("/"))
	v.HandleKey(keyRune("a"))
	v.HandleKey(keyRune("l"))
	if v.prompt.preview == nil || len(v.prompt.preview.Matches) != 1 {
		t.Fatal("expected valid regex preview for al")
	}

	v.HandleKey(keyRune("["))
	if v.prompt.errText == "" {
		t.Fatal("expected invalid regex error text")
	}
	if v.prompt.preview == nil || v.prompt.preview.Query != "al" {
		t.Fatalf("preview query after invalid regex = %#v, want last valid preview", v.prompt.preview)
	}
}

func TestInvalidRegexEnterKeepsPromptOpen(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true, SearchMode: SearchRegex})
	v.SetSize(20, 2)

	v.HandleKey(keyRune("/"))
	v.HandleKey(keyRune("["))
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.mode, modePrompt; got != want {
		t.Fatalf("viewer mode after invalid regex Enter = %v, want %v", got, want)
	}
	if v.prompt == nil || v.prompt.errText == "" {
		t.Fatal("expected prompt error to remain visible after invalid regex Enter")
	}
}

func TestCycleSearchCaseModePreservesRegexDiagnostic(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true, SearchMode: SearchRegex})
	v.SetSize(20, 2)

	if v.SearchForward("[") {
		t.Fatal("SearchForward([) = true, want false")
	}
	v.CycleSearchCaseMode()

	if got := v.message; !strings.HasPrefix(got, "regex:error ") {
		t.Fatalf("message after F2 on invalid regex = %q, want regex:error prefix", got)
	}
}

func TestPromptCaseToggleUpdatesCommittedSearch(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("Alpha\nalpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	if !v.SearchForward("alpha") {
		t.Fatal("SearchForward(alpha) = false, want true")
	}
	if got, want := len(v.search.Matches), 2; got != want {
		t.Fatalf("initial match count = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("/"))
	v.HandleKey(keyRune("x"))
	v.HandleKey(keyKey(tcell.KeyF2))
	v.HandleKey(keyKey(tcell.KeyEscape))

	if got, want := v.SearchCaseMode(), SearchCaseSensitive; got != want {
		t.Fatalf("SearchCaseMode after cancel = %v, want %v", got, want)
	}
	if got, want := len(v.search.Matches), 1; got != want {
		t.Fatalf("committed match count after cancel = %d, want %d", got, want)
	}
}

func TestPromptModeToggleUpdatesCommittedSearch(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alphabet alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	if !v.SearchForward("alpha") {
		t.Fatal("SearchForward(alpha) = false, want true")
	}
	if got, want := len(v.search.Matches), 2; got != want {
		t.Fatalf("initial match count = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("/"))
	v.HandleKey(keyRune("x"))
	v.HandleKey(keyKey(tcell.KeyF3))
	v.HandleKey(keyKey(tcell.KeyEscape))

	if got, want := v.SearchMode(), SearchWholeWord; got != want {
		t.Fatalf("SearchMode after cancel = %v, want %v", got, want)
	}
	if got, want := len(v.search.Matches), 1; got != want {
		t.Fatalf("committed whole-word match count after cancel = %d, want %d", got, want)
	}
}

func TestEmptySearchClearsExistingMatches(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\nbeta\nalpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"a", "l", "p", "h", "a"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	v.HandleKey(keyRune("/"))
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.search.Query, ""; got != want {
		t.Fatalf("search query after empty submit = %q, want %q", got, want)
	}
	if got, want := len(v.search.Matches), 0; got != want {
		t.Fatalf("match count after empty submit = %d, want %d", got, want)
	}
	if got, want := v.search.Current, 0; got != want {
		t.Fatalf("current match after empty submit = %d, want %d", got, want)
	}
}

func TestPromptCommandJumpsToLine(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\nfour\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	v.HandleKey(keyRune(":"))
	v.HandleKey(keyRune("3"))
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.firstVisibleAnchor().LineIndex, 2; got != want {
		t.Fatalf("visible line after :3 = %d, want %d", got, want)
	}
}

func TestPromptCommandJumpsToLogicalLineInWrap(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdef\nsecond\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.SoftWrap, ShowStatus: true})
	v.SetSize(4, 2)

	v.HandleKey(keyRune(":"))
	v.HandleKey(keyRune("2"))
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.Position().Row, 2; got != want {
		t.Fatalf("Position().Row after :2 = %d, want %d", got, want)
	}
	if got, want := v.firstVisibleAnchor(), (layout.Anchor{LineIndex: 1, GraphemeIndex: 0}); got != want {
		t.Fatalf("anchor after :2 = %+v, want %+v", got, want)
	}
}

func TestPromptCommandJumpsToSourceLineWhenSqueezed(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\n\n\nthree\nfour\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{
		TabWidth:          4,
		WrapMode:          layout.NoWrap,
		ShowStatus:        true,
		SqueezeBlankLines: true,
	})
	v.SetSize(20, 2)

	v.HandleKey(keyRune(":"))
	v.HandleKey(keyRune("4"))
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.firstVisibleAnchor().LineIndex, 2; got != want {
		t.Fatalf("visible squeezed line after :4 = %d, want %d", got, want)
	}
	if got, want := v.lines[2].Text, "three"; got != want {
		t.Fatalf("visible squeezed line text = %q, want %q", got, want)
	}
}

func TestSearchRevealKeepsVisibleHorizontalContext(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("0123456789 target suffix")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(10, 2)
	v.colOffset = 8
	v.relayout()

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"t", "a", "r", "g", "e", "t"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.colOffset, 8; got != want {
		t.Fatalf("col offset after visible search = %d, want %d", got, want)
	}
}

func TestSearchRevealOnlyScrollsEnoughToShowMatch(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("0123456789 target suffix")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(10, 2)
	v.colOffset = 0
	v.relayout()

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"t", "a", "r", "g", "e", "t"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.colOffset, 7; got != want {
		t.Fatalf("col offset after reveal = %d, want %d", got, want)
	}
}

func TestSearchRevealKeepsVisibleVerticalContext(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\nfour\nfive\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 4)
	v.rowOffset = 1
	v.relayout()

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"f", "o", "u", "r"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.rowOffset, 1; got != want {
		t.Fatalf("row offset after visible search = %d, want %d", got, want)
	}
}

func TestSearchRevealOnlyScrollsEnoughVertically(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\nfour\nfive\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 4)
	v.rowOffset = 0
	v.relayout()

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"f", "o", "u", "r"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.rowOffset, 1; got != want {
		t.Fatalf("row offset after reveal = %d, want %d", got, want)
	}
}

func TestApplyMatchCellStyleUsesUnderlineAccents(t *testing.T) {
	base := tcell.StyleDefault.Foreground(tcolor.White)

	inactive := applyMatchCellStyle(base, false)
	if got, want := inactive.GetUnderlineStyle(), tcell.UnderlineStyleNone; got != want {
		t.Fatalf("inactive underline style = %v, want %v", got, want)
	}
	if got, want := inactive.GetBackground(), inactiveMatchStyle.Bg; got != want {
		t.Fatalf("inactive background = %v, want %v", got, want)
	}
	if got, want := inactive.GetForeground(), inactiveMatchStyle.Fg; got != want {
		t.Fatalf("inactive foreground = %v, want %v", got, want)
	}
	if inactive.HasBold() {
		t.Fatalf("inactive match should not force bold")
	}

	current := applyMatchCellStyle(base, true)
	if got, want := current.GetUnderlineStyle(), currentMatchStyle.UnderlineStyle; got != want {
		t.Fatalf("current underline style = %v, want %v", got, want)
	}
	if got, want := current.GetUnderlineColor(), currentMatchStyle.UnderlineColor; got != want {
		t.Fatalf("current underline color = %v, want %v", got, want)
	}
	if got, want := current.GetBackground(), currentMatchStyle.Bg; got != want {
		t.Fatalf("current background = %v, want %v", got, want)
	}
	if got, want := current.GetForeground(), currentMatchStyle.Fg; got != want {
		t.Fatalf("current foreground = %v, want %v", got, want)
	}
	if !current.HasBold() {
		t.Fatalf("current match should be bold")
	}
}

func TestRefreshPicksUpAppendedDocumentContent(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 4)

	if err := doc.Append([]byte("hello\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	doc.Flush()
	v.Refresh()

	if got, want := len(v.lines), 1; got != want {
		t.Fatalf("line count after refresh = %d, want %d", got, want)
	}
	if got, want := v.lines[0].Text, "hello"; got != want {
		t.Fatalf("line text after refresh = %q, want %q", got, want)
	}
}

func TestRefreshInFollowModeStaysAtBottomAfterAppend(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.Follow()

	if err := doc.Append([]byte("three\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	doc.Flush()
	v.Refresh()

	if !v.Following() {
		t.Fatalf("follow mode = false, want true")
	}
	if got, want := v.rowOffset, v.maxRowOffset(); got != want {
		t.Fatalf("row offset after append = %d, want %d", got, want)
	}
}

func TestSetSizeInFollowModeRepinsToEOF(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\nfour\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 4)
	v.Follow()

	if !v.EOFVisible() {
		t.Fatal("EOF visible before resize = false, want true")
	}

	v.SetSize(20, 2)

	if !v.Following() {
		t.Fatalf("follow mode after resize = false, want true")
	}
	if !v.EOFVisible() {
		t.Fatal("EOF visible after resize = false, want true")
	}
	if got, want := v.rowOffset, v.maxRowOffset(); got != want {
		t.Fatalf("row offset after resize = %d, want %d", got, want)
	}
}

func TestScrollUpExitsFollowMode(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.Follow()
	v.HandleKey(keyRune("k"))

	if v.Following() {
		t.Fatalf("follow mode = true, want false")
	}
}

func TestRefreshWithoutFollowKeepsViewportWhenAppended(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.rowOffset = 0
	v.relayout()

	if err := doc.Append([]byte("four\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	doc.Flush()
	v.Refresh()

	if v.Following() {
		t.Fatalf("follow mode = true, want false")
	}
	if got, want := v.rowOffset, 0; got != want {
		t.Fatalf("row offset after append = %d, want %d", got, want)
	}
}

func TestScrollDownToBottomDoesNotEnableFollowMode(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.ScrollDown(10)

	if v.Following() {
		t.Fatalf("follow mode = true, want false")
	}
	if got, want := v.rowOffset, v.maxRowOffset(); got != want {
		t.Fatalf("row offset after ScrollDown = %d, want %d", got, want)
	}
}

func TestFollowKeyEnablesFollowMode(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.HandleKey(keyRune("F"))

	if !v.Following() {
		t.Fatalf("follow mode = false, want true")
	}
	if got, want := v.rowOffset, v.maxRowOffset(); got != want {
		t.Fatalf("row offset after F = %d, want %d", got, want)
	}
}

func TestCancelPromptPreservesFollowMode(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.Follow()
	v.HandleKey(keyRune("/"))
	v.HandleKey(keyKey(tcell.KeyEscape))

	if !v.Following() {
		t.Fatalf("follow mode = false, want true")
	}
}

func TestSearchRepeatWithoutActiveSearchPreservesFollowMode(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.Follow()
	v.HandleKey(keyRune("n"))

	if !v.Following() {
		t.Fatalf("follow mode = false, want true")
	}
}

func TestToggleHelpMode(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})

	if got, want := v.mode, modeNormal; got != want {
		t.Fatalf("initial mode = %v, want %v", got, want)
	}
	v.HandleKey(keyRune("H"))
	if got, want := v.mode, modeHelp; got != want {
		t.Fatalf("mode after H = %v, want %v", got, want)
	}
	v.HandleKey(keyKey(tcell.KeyEscape))
	if got, want := v.mode, modeNormal; got != want {
		t.Fatalf("mode after Esc = %v, want %v", got, want)
	}
}

func TestViewerUsesCustomTextBundle(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{
		TabWidth:   4,
		WrapMode:   layout.NoWrap,
		ShowStatus: true,
		Text: Text{
			HelpTitle: "Ayuda",
		},
	})

	if got, want := v.text.HelpTitle, "Ayuda"; got != want {
		t.Fatalf("help title = %q, want %q", got, want)
	}
}

func TestDrawHelpRespectsFrameInsets(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Chrome: Chrome{
			Frame: Frame{
				Horizontal:  "─",
				Vertical:    "│",
				TopLeft:     "╭",
				TopRight:    "╮",
				BottomLeft:  "╰",
				BottomRight: "╯",
			},
		},
	})
	v.SetSize(20, 6)
	v.toggleHelp()

	_, screen := newMockScreen(t, 20, 6)
	defer screen.Fini()

	v.Draw(screen)

	if got := cellRune(screen, 0, 0); got != '╭' {
		t.Fatalf("top-left border rune = %q, want %q", got, '╭')
	}
	if got := cellRune(screen, 0, 1); got != '│' {
		t.Fatalf("left border rune = %q, want %q", got, '│')
	}
	if got := cellRune(screen, 1, 1); got != 'G' {
		t.Fatalf("help body rune = %q, want %q", got, 'G')
	}
}

func TestDrawHelpAppliesANSIStyles(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Text: Text{
			HelpBody: "\x1b[1;4mSection\x1b[0m\nnormal \x1b[3mvalue\x1b[0m",
		},
	})
	v.SetSize(24, 6)
	v.toggleHelp()

	_, screen := newMockScreen(t, 24, 6)
	defer screen.Fini()

	v.Draw(screen)

	if got := strings.TrimRight(screenRowString(screen, 1, 24), " "); got != "Section" {
		t.Fatalf("styled heading row = %q, want %q", got, "Section")
	}
	if got := strings.TrimRight(screenRowString(screen, 2, 24), " "); got != "normal value" {
		t.Fatalf("styled body row = %q, want %q", got, "normal value")
	}

	_, headingStyle, _ := screen.Get(0, 1)
	if !headingStyle.HasBold() {
		t.Fatal("help heading lost bold ANSI styling")
	}
	if got, want := headingStyle.GetUnderlineStyle(), tcell.UnderlineStyleSolid; got != want {
		t.Fatalf("help heading underline = %v, want %v", got, want)
	}
	pos := strings.Index(screenRowString(screen, 2, 24), "value")
	if pos < 0 {
		t.Fatalf("styled body row = %q, want it to contain %q", screenRowString(screen, 2, 24), "value")
	}
	_, style, _ := screen.Get(pos, 2)
	if !style.HasItalic() {
		t.Fatal("help body lost italic ANSI styling")
	}
}

func TestStatusShowsRightOverflowIndicator(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdefghijklmnopqrstuvwxyz")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(10, 2)
	v.relayout()

	left, right := v.statusOverflow()
	if left {
		t.Fatalf("left overflow = true, want false")
	}
	if !right {
		t.Fatalf("right overflow = false, want true")
	}
	leftText, rightText := v.statusText()
	if strings.Contains(leftText, "▶") || strings.Contains(rightText, "▶") {
		t.Fatalf("status text (%q, %q) should not embed right overflow indicator", leftText, rightText)
	}
}

func TestStatusShowsBothOverflowIndicatorsWhenScrolledMidway(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdefghijklmnopqrstuvwxyz")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(10, 2)
	v.colOffset = 5
	v.relayout()

	left, right := v.statusOverflow()
	if !left || !right {
		t.Fatalf("overflow = (%v, %v), want (true, true)", left, right)
	}
}

func TestStatusTextPlacesPositionOnRight(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\nbeta\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.relayout()

	leftText, rightText := v.statusText()
	if got, want := leftText, "F1 Help"; got != want {
		t.Fatalf("left status text = %q, want %q", got, want)
	}
	if !strings.Contains(rightText, "row 1/2") || !strings.Contains(rightText, "col 1/5") || !strings.Contains(rightText, "⇆") {
		t.Fatalf("right status text = %q, want row+col indicator with no-wrap glyph", rightText)
	}
}

func TestStatusTextUsesLogicalCoordinatesInSoftWrap(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdefgh\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.SoftWrap, ShowStatus: true})
	v.SetSize(4, 2)
	v.ScrollDown(1)

	_, rightText := v.statusText()
	if !strings.Contains(rightText, "row 1/1") || !strings.Contains(rightText, "col 5/8") {
		t.Fatalf("right status text = %q, want logical row+col indicator", rightText)
	}
}

func TestStatusTextAndPositionAgreeWithFixedHeaders(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("h1\nh2\nbody1\nbody2\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true, HeaderLines: 2})
	v.SetSize(20, 2)
	v.relayout()

	pos := v.Position()
	if got, want := pos.Row, 3; got != want {
		t.Fatalf("Position().Row with fixed headers filling viewport = %d, want %d", got, want)
	}
	_, rightText := v.statusText()
	if !strings.Contains(rightText, "row 3/4") {
		t.Fatalf("right status text = %q, want row indicator for first visible body row", rightText)
	}
}

func TestStatusTextShowsEOFWhenVisible(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\nbeta\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 3)
	v.relayout()

	_, rightText := v.statusText()
	if !strings.Contains(rightText, "EOF") {
		t.Fatalf("right status text = %q, want EOF indicator", rightText)
	}
}

func TestEOFVisibleRequiresLineEndToBeVisibleInNoWrap(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdefghijklmnopqrstuvwxyz\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(10, 2)
	v.relayout()

	if v.EOFVisible() {
		t.Fatal("EOFVisible() = true, want false when line end is clipped")
	}
}

func TestStatusTextUsesActiveSearchOverride(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("Beta\nbeta\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.relayout()
	if !v.SearchForwardWithCase("BETA", SearchCaseInsensitive) {
		t.Fatal("SearchForwardWithCase(BETA, SearchCaseInsensitive) = false, want true")
	}

	leftText, rightText := v.statusText()
	if !strings.Contains(leftText, "/BETA 1/2") {
		t.Fatalf("left status text = %q, want active search info", leftText)
	}
	if strings.Contains(rightText, "F2:") || strings.Contains(rightText, "F3:") {
		t.Fatalf("right status text = %q, want no prompt-only search mode hint", rightText)
	}
}

func TestStatusTextCanReplaceHelpHint(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{
		TabWidth:   4,
		WrapMode:   layout.NoWrap,
		ShowStatus: true,
		Text: Text{
			StatusHelpHint: "hilfe",
		},
	})
	v.SetSize(20, 2)
	v.relayout()

	leftText, _ := v.statusText()
	if got, want := leftText, "hilfe"; got != want {
		t.Fatalf("left status text = %q, want %q", got, want)
	}
}

func TestStatusTextCanHideHelpHint(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{
		TabWidth:   4,
		WrapMode:   layout.NoWrap,
		ShowStatus: true,
		Text: Text{
			HideStatusHelpHint: true,
		},
	})
	v.SetSize(20, 2)
	v.relayout()

	leftText, _ := v.statusText()
	if got, want := leftText, ""; got != want {
		t.Fatalf("left status text = %q, want %q", got, want)
	}
}

func TestStatusHelpHintDoesNotShiftWhenLeftOverflowAppears(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(30, 2)
	v.relayout()

	_, screen := newMockScreen(t, 30, 2)
	defer screen.Fini()

	v.drawStatus(screen, 1)
	if got := cellRune(screen, 3, 1); got != 'F' {
		t.Fatalf("help hint start before scroll = %q, want %q", got, 'F')
	}

	v.colOffset = 1
	v.relayout()
	screen.Clear()
	v.drawStatus(screen, 1)
	if got := cellRune(screen, 1, 1); got != '◀' {
		t.Fatalf("left overflow indicator = %q, want %q", got, '◀')
	}
	if got := cellRune(screen, 3, 1); got != 'F' {
		t.Fatalf("help hint start after scroll = %q, want %q", got, 'F')
	}
}

func TestStatusTextSuppressesHelpHintWhenMessagePresent(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.relayout()
	v.message = "saved"

	leftText, _ := v.statusText()
	if got, want := leftText, "saved"; got != want {
		t.Fatalf("left status text = %q, want %q", got, want)
	}
}

func TestStatusTextDoesNotDuplicateModeMessage(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.relayout()
	v.CycleSearchMode()

	leftText, rightText := v.statusText()
	if got, want := leftText, "F1 Help"; got != want {
		t.Fatalf("left status text after F3 = %q, want %q", got, want)
	}
	if strings.Contains(rightText, "F2:") || strings.Contains(rightText, "F3:") {
		t.Fatalf("right status text after F3 = %q, want no prompt-only search mode hint", rightText)
	}
}

func TestStatusTextShowsWrapHintInSoftWrapMode(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.SoftWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.relayout()

	_, rightText := v.statusText()
	if !strings.Contains(rightText, "↪") {
		t.Fatalf("right status text = %q, want wrap hint", rightText)
	}
}

func TestStatusTextShowsColumnTotalsWhenScrolled(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdefghij\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(6, 2)
	v.colOffset = 3
	v.relayout()

	_, rightText := v.statusText()
	if !strings.Contains(rightText, "col 4/10") {
		t.Fatalf("right status text = %q, want column total", rightText)
	}
	if !strings.Contains(rightText, "⇆") {
		t.Fatalf("right status text = %q, want no-wrap glyph", rightText)
	}
}

func TestStatusBarUsesDisplayWidthForRightAlignedText(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{
		TabWidth:   4,
		WrapMode:   layout.NoWrap,
		ShowStatus: true,
		Text: Text{
			StatusPosition: func(current, total, column, columns int) string {
				return "界a"
			},
		},
	})
	v.SetSize(10, 2)
	v.relayout()

	_, screen := newMockScreen(t, 10, 2)
	defer screen.Fini()

	v.drawStatus(screen, 1)

	if got := cellRune(screen, 3, 1); got != '界' {
		t.Fatalf("status rune at expected start = %q, want %q", got, '界')
	}
}

func TestStatusLineFormatterOverridesBuiltInStatusText(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\nbeta\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{
		TabWidth:   4,
		WrapMode:   layout.NoWrap,
		ShowStatus: true,
		Text: Text{
			StatusLine: func(info StatusInfo) (left, right string) {
				if info.DefaultRight == "" {
					t.Fatalf("StatusLine got empty right default: %+v", info)
				}
				if info.EOFVisible {
					t.Fatalf("StatusLine EOFVisible = true, want false")
				}
				return "L:" + info.DefaultLeft, "R:" + info.DefaultRight
			},
		},
	})
	v.SetSize(20, 2)
	v.relayout()

	leftText, rightText := v.statusText()
	if got, want := leftText, "L:F1 Help"; got != want {
		t.Fatalf("left status text = %q, want %q", got, want)
	}
	if !strings.HasPrefix(rightText, "R:row 1/2  col 1/5") {
		t.Fatalf("right status text = %q, want prefix %q", rightText, "R:row 1/2  col 1/5")
	}
}

func TestStatusHelpHintKeyUsesConfiguredStyle(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	keyStyle := tcell.StyleDefault.Foreground(tcolor.PaletteColor(10)).Background(tcolor.PaletteColor(4)).Bold(true)
	statusStyle := tcell.StyleDefault.Foreground(tcolor.PaletteColor(15)).Background(tcolor.PaletteColor(2))
	v := New(doc, Config{
		TabWidth:   4,
		WrapMode:   layout.NoWrap,
		ShowStatus: true,
		Text: Text{
			StatusPosition: func(current, total, column, columns int) string {
				return ""
			},
		},
		Chrome: Chrome{
			StatusStyle:        statusStyle,
			StatusHelpKeyStyle: keyStyle,
		},
	})
	v.SetSize(20, 2)
	v.relayout()

	_, screen := newMockScreen(t, 20, 2)
	defer screen.Fini()
	v.drawStatus(screen, 1)

	_, gotKeyStyle, _ := screen.Get(3, 1)
	if got, want := gotKeyStyle.GetForeground(), keyStyle.GetForeground(); got != want {
		t.Fatalf("help key fg = %v, want %v", got, want)
	}
	if got, want := gotKeyStyle.GetBackground(), keyStyle.GetBackground(); got != want {
		t.Fatalf("help key bg = %v, want %v", got, want)
	}
	if !gotKeyStyle.HasBold() {
		t.Fatal("help key style lost bold attribute")
	}

	_, gotRestStyle, _ := screen.Get(6, 1)
	if got, want := gotRestStyle.GetForeground(), statusStyle.GetForeground(); got != want {
		t.Fatalf("help hint rest fg = %v, want %v", got, want)
	}
}

func TestPromptLineFormatterOverridesBuiltInPromptText(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Text: Text{
			PromptLine: func(info PromptInfo) string {
				if got, want := info.DefaultText, "/a"; got != want {
					t.Fatalf("PromptLine default text = %q, want %q", got, want)
				}
				if got, want := info.Kind, PromptKindSearchForward; got != want {
					t.Fatalf("PromptLine kind = %v, want %v", got, want)
				}
				return "find>" + info.Input
			},
		},
	})
	v.SetSize(20, 2)
	v.beginPrompt(promptSearchForward)
	v.prompt.editor.SetText("a")

	if got, want := v.promptText(), "find>a"; got != want {
		t.Fatalf("promptText() = %q, want %q", got, want)
	}
}

func TestDrawPromptShowsTailWhenInputOverflows(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(18, 2)
	v.beginPrompt(promptSearchForward)
	v.prompt.editor.SetText("abcdefghijkl")

	_, screen := newMockScreen(t, 18, 2)
	defer screen.Fini()
	v.drawPrompt(screen, 1)

	if got, want := screenRowString(screen, 1, 18), " /…kl F2:A? F3:ab "; got != want {
		t.Fatalf("prompt row = %q, want %q", got, want)
	}
}

func TestDrawPromptKeepsCursorVisibleWhenInputIsClipped(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(6, 2)
	v.beginPrompt(promptCommand)
	v.prompt.editor.SetText("abcdef")
	v.prompt.editor.MoveHome()
	v.prompt.editor.MoveRight()

	term, screen := newMockScreen(t, 6, 2)
	defer screen.Fini()
	v.Draw(screen)

	if got, want := screenRowString(screen, 1, 6), " :…bcd"; got != want {
		t.Fatalf("prompt row = %q, want %q", got, want)
	}
	if got, want := term.Pos(), (vt.Coord{X: vt.Col(3), Y: vt.Row(1)}); got != want {
		t.Fatalf("cursor position = %+v, want %+v", got, want)
	}
}

func TestDrawPromptUsesCursorStyleForInsertAndOverwrite(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(10, 2)
	v.beginPrompt(promptCommand)
	v.prompt.editor.SetText("alpha")

	term, screen := newMockScreen(t, 10, 2)
	defer screen.Fini()

	v.Draw(screen)
	if got, want := term.Backend().GetCursor(), vt.SteadyBar; got != want {
		t.Fatalf("insert cursor style = %v, want %v", got, want)
	}

	v.prompt.editor.ToggleOverwrite()
	v.Draw(screen)
	if got, want := term.Backend().GetCursor(), vt.SteadyBlock; got != want {
		t.Fatalf("overwrite cursor style = %v, want %v", got, want)
	}
}

func TestDisplayPromptTextReturnsEmptyForNonPositiveWidth(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.beginPrompt(promptSearchForward)
	v.prompt.editor.SetText("alpha")

	if got := v.displayPromptText(0); got != "" {
		t.Fatalf("displayPromptText(0) = %q, want empty", got)
	}
}

func TestBuiltInSearchPromptPlacesModeHintOnRight(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(20, 2)
	v.beginPrompt(promptSearchForward)
	v.prompt.editor.SetText("alpha")

	_, screen := newMockScreen(t, 20, 2)
	defer screen.Fini()
	v.drawPrompt(screen, 1)

	row := screenRowString(screen, 1, 20)
	if !strings.HasPrefix(row, " /alpha") {
		t.Fatalf("prompt row = %q, want left search input", row)
	}
	if !strings.HasSuffix(row, " F2:A? F3:ab ") {
		t.Fatalf("prompt row = %q, want right search mode hint", row)
	}
}

func TestSearchPromptHintKeysUseHighlightedStyle(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	keyStyle := tcell.StyleDefault.Foreground(tcolor.PaletteColor(10)).Background(tcolor.PaletteColor(4)).Bold(true)
	promptStyle := tcell.StyleDefault.Foreground(tcolor.PaletteColor(15)).Background(tcolor.PaletteColor(2))
	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Chrome: Chrome{
			PromptStyle:        promptStyle,
			StatusHelpKeyStyle: keyStyle,
		},
	})
	v.SetSize(24, 2)
	v.beginPrompt(promptSearchForward)
	v.prompt.editor.SetText("alpha")

	_, screen := newMockScreen(t, 24, 2)
	defer screen.Fini()
	v.drawPrompt(screen, 1)

	row := screenRowString(screen, 1, 24)
	idx := strings.Index(row, "F2:")
	if idx < 0 {
		t.Fatalf("prompt row = %q, want F2 hint", row)
	}
	_, gotKeyStyle, _ := screen.Get(idx, 1)
	if got, want := gotKeyStyle.GetForeground(), keyStyle.GetForeground(); got != want {
		t.Fatalf("prompt hint key fg = %v, want %v", got, want)
	}
	if got, want := gotKeyStyle.GetBackground(), keyStyle.GetBackground(); got != want {
		t.Fatalf("prompt hint key bg = %v, want %v", got, want)
	}
	if !gotKeyStyle.HasBold() {
		t.Fatal("prompt hint key style lost bold attribute")
	}
}

func TestPromptLineFormatterDoesNotDuplicatePromptError(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{
		TabWidth:   4,
		WrapMode:   layout.NoWrap,
		SearchMode: SearchRegex,
		Text: Text{
			PromptLine: func(info PromptInfo) string {
				return info.DefaultText + "  " + info.Error
			},
		},
	})
	v.SetSize(40, 2)
	v.beginPrompt(promptSearchForward)
	v.prompt.editor.SetText("(")
	v.updatePromptPreview()

	_, screen := newMockScreen(t, 40, 2)
	defer screen.Fini()

	v.drawPrompt(screen, 1)
	if got := strings.Count(screenRowString(screen, 1, 40), "regex:error"); got != 1 {
		t.Fatalf("prompt error count = %d, want 1", got)
	}
}

func TestPromptLineFormatterOwnsErrorRendering(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{
		TabWidth:   4,
		WrapMode:   layout.NoWrap,
		SearchMode: SearchRegex,
		Text: Text{
			PromptLine: func(info PromptInfo) string {
				return "prompt>" + info.Input
			},
		},
	})
	v.SetSize(40, 2)
	v.beginPrompt(promptSearchForward)
	v.prompt.editor.SetText("(")
	v.updatePromptPreview()

	_, screen := newMockScreen(t, 40, 2)
	defer screen.Fini()

	v.drawPrompt(screen, 1)
	if got := screenRowString(screen, 1, 40); strings.Contains(got, "regex:error") {
		t.Fatalf("prompt row = %q, want custom prompt renderer to own error text", got)
	}
}

func TestDrawStatusAndPromptUseConfiguredStyles(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	statusStyle := tcell.StyleDefault.Foreground(tcolor.PaletteColor(3)).Background(tcolor.PaletteColor(7))
	promptStyle := tcell.StyleDefault.Foreground(tcolor.PaletteColor(2)).Background(tcolor.PaletteColor(0)).Bold(true)
	promptErrorStyle := tcell.StyleDefault.Foreground(tcolor.PaletteColor(1)).Background(tcolor.PaletteColor(0))

	v := New(doc, Config{
		TabWidth:   4,
		WrapMode:   layout.NoWrap,
		ShowStatus: true,
		SearchMode: SearchRegex,
		Chrome: Chrome{
			StatusStyle:      statusStyle,
			PromptStyle:      promptStyle,
			PromptErrorStyle: promptErrorStyle,
		},
	})
	v.SetSize(20, 2)
	v.relayout()

	_, screen := newMockScreen(t, 20, 2)
	defer screen.Fini()

	v.drawStatus(screen, 1)
	_, gotStatusStyle, _ := screen.Get(0, 1)
	if got, want := gotStatusStyle.GetForeground(), statusStyle.GetForeground(); got != want {
		t.Fatalf("status fg = %v, want %v", got, want)
	}
	if got, want := gotStatusStyle.GetBackground(), statusStyle.GetBackground(); got != want {
		t.Fatalf("status bg = %v, want %v", got, want)
	}

	v.beginPrompt(promptSearchForward)
	v.prompt.editor.SetText("(")
	v.updatePromptPreview()
	screen.Clear()
	v.drawPrompt(screen, 1)

	_, gotPromptStyle, _ := screen.Get(1, 1)
	if got, want := gotPromptStyle.GetForeground(), promptStyle.GetForeground(); got != want {
		t.Fatalf("prompt fg = %v, want %v", got, want)
	}
	if got, want := gotPromptStyle.GetBackground(), promptStyle.GetBackground(); got != want {
		t.Fatalf("prompt bg = %v, want %v", got, want)
	}
	if !gotPromptStyle.HasBold() {
		t.Fatal("prompt style lost bold attribute")
	}

	errorStart := stringWidth(truncateToWidth(" "+v.promptText(), v.width))
	_, gotErrorStyle, _ := screen.Get(errorStart, 1)
	if got, want := gotErrorStyle.GetForeground(), promptErrorStyle.GetForeground(); got != want {
		t.Fatalf("prompt error fg = %v, want %v", got, want)
	}
	if got, want := gotErrorStyle.GetBackground(), promptErrorStyle.GetBackground(); got != want {
		t.Fatalf("prompt error bg = %v, want %v", got, want)
	}
}

func TestThemeRemapsDefaultsAndANSI16Only(t *testing.T) {
	v := New(model.NewDocument(4), Config{
		Theme: Theme{
			DefaultFG: tcolor.Red,
			DefaultBG: tcolor.Blue,
			ANSI: [16]tcolor.Color{
				1: tcolor.Aqua,
			},
		},
	})

	defaultStyle := v.toTCellStyle(ansi.DefaultStyle())
	if got, want := defaultStyle.GetForeground(), tcolor.Red; got != want {
		t.Fatalf("default fg = %v, want %v", got, want)
	}
	if got, want := defaultStyle.GetBackground(), tcolor.Blue; got != want {
		t.Fatalf("default bg = %v, want %v", got, want)
	}

	indexedStyle := v.toTCellStyle(ansi.Style{Fg: ansi.IndexedColor(1), Bg: ansi.IndexedColor(16)})
	if got, want := indexedStyle.GetForeground(), tcolor.Aqua; got != want {
		t.Fatalf("indexed fg = %v, want %v", got, want)
	}
	if got, want := indexedStyle.GetBackground(), tcolor.PaletteColor(16); got != want {
		t.Fatalf("indexed bg = %v, want %v", got, want)
	}

	rgbStyle := v.toTCellStyle(ansi.Style{Fg: ansi.RGBColor(1, 2, 3)})
	if got, want := rgbStyle.GetForeground(), tcolor.NewRGBColor(1, 2, 3); got != want {
		t.Fatalf("rgb fg = %v, want %v", got, want)
	}
}

func TestThemeAllowsExplicitResetMappings(t *testing.T) {
	v := New(model.NewDocument(4), Config{
		Theme: Theme{
			DefaultFG: tcolor.Reset,
			DefaultBG: tcolor.Reset,
			ANSI: [16]tcolor.Color{
				1: tcolor.Reset,
			},
		},
	})

	defaultStyle := v.toTCellStyle(ansi.DefaultStyle())
	if got, want := defaultStyle.GetForeground(), tcolor.Reset; got != want {
		t.Fatalf("default fg = %v, want %v", got, want)
	}
	if got, want := defaultStyle.GetBackground(), tcolor.Reset; got != want {
		t.Fatalf("default bg = %v, want %v", got, want)
	}

	indexedStyle := v.toTCellStyle(ansi.Style{Fg: ansi.IndexedColor(1)})
	if got, want := indexedStyle.GetForeground(), tcolor.Reset; got != want {
		t.Fatalf("indexed fg = %v, want %v", got, want)
	}
}

func TestDrawUsesThemedDefaultBackgroundForBlankContent(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("hi\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Theme: Theme{
			DefaultBG: tcolor.Blue,
		},
	})
	v.SetSize(8, 3)

	_, screen := newMockScreen(t, 8, 3)
	defer screen.Fini()

	v.Draw(screen)

	_, eolStyle, _ := screen.Get(4, 0)
	if got, want := eolStyle.GetBackground(), tcolor.Blue; got != want {
		t.Fatalf("blank cell after line bg = %v, want %v", got, want)
	}

	_, blankRowStyle, _ := screen.Get(0, 2)
	if got, want := blankRowStyle.GetBackground(), tcolor.Blue; got != want {
		t.Fatalf("blank row bg = %v, want %v", got, want)
	}
}

func TestTruncateToWidthUsesDisplayCellsAndPreservesUTF8(t *testing.T) {
	got := truncateToWidth("é界b", 2)
	if !utf8.ValidString(got) {
		t.Fatalf("truncateToWidth produced invalid UTF-8: %q", got)
	}
	if got != "é" {
		t.Fatalf("truncateToWidth = %q, want %q", got, "é")
	}
}

func TestDrawFrameInsetsContentAndRendersTitle(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("hi\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Chrome: Chrome{
			Title: "Doc",
			Frame: Frame{
				Horizontal:  "─",
				Vertical:    "│",
				TopLeft:     "╭",
				TopRight:    "╮",
				BottomLeft:  "╰",
				BottomRight: "╯",
			},
		},
	})
	v.SetSize(10, 4)

	_, screen := newMockScreen(t, 10, 4)
	defer screen.Fini()

	v.Draw(screen)

	if got := cellRune(screen, 0, 0); got != '╭' {
		t.Fatalf("top-left border rune = %q, want %q", got, '╭')
	}
	if got := cellRune(screen, 2, 0); got != 'D' {
		t.Fatalf("title rune = %q, want %q", got, 'D')
	}
	if got := cellRune(screen, 0, 1); got != '│' {
		t.Fatalf("left border rune = %q, want %q", got, '│')
	}
	if got := cellRune(screen, 1, 1); got != 'h' {
		t.Fatalf("content rune = %q, want %q", got, 'h')
	}
}

func TestDrawFrameUsesBorderAndTitleStyles(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("hi\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	borderStyle := tcell.StyleDefault.Foreground(tcolor.PaletteColor(4))
	titleStyle := tcell.StyleDefault.Foreground(tcolor.PaletteColor(15)).Bold(true)
	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Chrome: Chrome{
			Title:       "Doc",
			BorderStyle: borderStyle,
			TitleStyle:  titleStyle,
			Frame: Frame{
				Horizontal:  "─",
				Vertical:    "│",
				TopLeft:     "┌",
				TopRight:    "┐",
				BottomLeft:  "└",
				BottomRight: "┘",
			},
		},
	})
	v.SetSize(10, 4)

	_, screen := newMockScreen(t, 10, 4)
	defer screen.Fini()

	v.Draw(screen)

	_, borderCellStyle, _ := screen.Get(0, 0)
	if got, want := borderCellStyle.GetForeground(), borderStyle.GetForeground(); got != want {
		t.Fatalf("border foreground = %v, want %v", got, want)
	}

	_, titleCellStyle, _ := screen.Get(2, 0)
	if got, want := titleCellStyle.GetForeground(), titleStyle.GetForeground(); got != want {
		t.Fatalf("title foreground = %v, want %v", got, want)
	}
	if !titleCellStyle.HasBold() {
		t.Fatalf("title style should be bold")
	}
}

func TestDrawFrameTitleAlignment(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("hi\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	testCases := []struct {
		name  string
		align TitleAlign
		x     int
	}{
		{name: "left", align: TitleAlignLeft, x: 2},
		{name: "center", align: TitleAlignCenter, x: 3},
		{name: "right", align: TitleAlignRight, x: 5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v := New(doc, Config{
				TabWidth: 4,
				WrapMode: layout.NoWrap,
				Chrome: Chrome{
					Title:      "Doc",
					TitleAlign: tc.align,
					Frame: Frame{
						Horizontal:  "─",
						Vertical:    "│",
						TopLeft:     "┌",
						TopRight:    "┐",
						BottomLeft:  "└",
						BottomRight: "┘",
					},
				},
			})
			v.SetSize(10, 4)

			_, screen := newMockScreen(t, 10, 4)
			defer screen.Fini()

			v.Draw(screen)

			if got := cellRune(screen, tc.x, 0); got != 'D' {
				t.Fatalf("title rune at x=%d = %q, want %q", tc.x, got, 'D')
			}
		})
	}
}

func TestHelpFrameTitleUsesConfiguredAlignment(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("hi\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	title := "Help"
	if label, x := frameTitleLabel(title, 10, TitleAlignRight); label == "" || x != 3 {
		t.Fatalf("frameTitleLabel(Help, right) = (%q, %d), want non-empty label at x=3", label, x)
	}

	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Text: Text{
			HelpTitle: title,
		},
		Chrome: Chrome{
			TitleAlign: TitleAlignRight,
			Frame: Frame{
				Horizontal:  "─",
				Vertical:    "│",
				TopLeft:     "┌",
				TopRight:    "┐",
				BottomLeft:  "└",
				BottomRight: "┘",
			},
		},
	})
	v.SetSize(10, 4)
	v.mode = modeHelp

	_, screen := newMockScreen(t, 10, 4)
	defer screen.Fini()

	v.text.HelpClose = ""
	v.Draw(screen)

	if got := cellRune(screen, 4, 0); got != 'H' {
		t.Fatalf("help title rune at x=4 = %q, want %q", got, 'H')
	}
}

func TestHelpBodyUsesThemedDefaultBackground(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Theme: Theme{
			DefaultFG: tcolor.NewRGBColor(0x65, 0x7b, 0x83),
			DefaultBG: tcolor.NewRGBColor(0xfd, 0xf6, 0xe3),
		},
		Chrome: Chrome{
			Frame: Frame{
				Horizontal:  "─",
				Vertical:    "│",
				TopLeft:     "┌",
				TopRight:    "┐",
				BottomLeft:  "└",
				BottomRight: "┘",
			},
		},
	})
	v.SetSize(20, 6)
	v.toggleHelp()

	_, screen := newMockScreen(t, 20, 6)
	defer screen.Fini()

	v.Draw(screen)

	_, style, _ := screen.Get(2, 1)
	if got, want := style.GetBackground(), tcolor.NewRGBColor(0xfd, 0xf6, 0xe3); got != want {
		t.Fatalf("help body background = %v, want %v", got, want)
	}
}

func TestDrawLineNumbersUsesAdaptiveGutterWidth(t *testing.T) {
	doc := model.NewDocument(4)
	var lines strings.Builder
	for i := 0; i < 12; i++ {
		lines.WriteString("x\n")
	}
	if err := doc.Append([]byte(lines.String())); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, LineNumbers: true})
	v.SetSize(10, 4)

	_, screen := newMockScreen(t, 10, 4)
	defer screen.Fini()
	v.Draw(screen)

	if got := cellRune(screen, 1, 0); got != '1' {
		t.Fatalf("line number rune = %q, want %q", got, '1')
	}
	if got := cellRune(screen, 2, 0); got != ' ' {
		t.Fatalf("gutter separator rune = %q, want space", got)
	}
}

func TestDrawLineNumbersUseSourceNumberingWhenSqueezed(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\n\n\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{
		TabWidth:          4,
		WrapMode:          layout.NoWrap,
		LineNumbers:       true,
		SqueezeBlankLines: true,
	})
	v.SetSize(8, 4)

	_, screen := newMockScreen(t, 8, 4)
	defer screen.Fini()
	v.Draw(screen)

	if got := cellRune(screen, 0, 0); got != '1' {
		t.Fatalf("first line number rune = %q, want %q", got, '1')
	}
	if got := cellRune(screen, 0, 1); got != '2' {
		t.Fatalf("squeezed blank line number rune = %q, want %q", got, '2')
	}
	if got := cellRune(screen, 0, 2); got != '4' {
		t.Fatalf("third line number rune = %q, want %q", got, '4')
	}
}

func TestSetSqueezeBlankLinesPreservesVisibleSourceLine(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\n\n\nthree\nfour\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(20, 2)
	v.ScrollDown(3)

	if got, want := v.lines[v.firstVisibleAnchor().LineIndex].Text, "three"; got != want {
		t.Fatalf("visible line before squeeze = %q, want %q", got, want)
	}

	v.SetSqueezeBlankLines(true)

	if got, want := v.lines[v.firstVisibleAnchor().LineIndex].Text, "three"; got != want {
		t.Fatalf("visible line after squeeze = %q, want %q", got, want)
	}
}

func TestDrawLineNumbersBlankOnWrappedContinuationRows(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdefghij\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.SoftWrap, LineNumbers: true})
	v.SetSize(6, 4)

	_, screen := newMockScreen(t, 6, 4)
	defer screen.Fini()
	v.Draw(screen)

	if got := cellRune(screen, 0, 0); got != '1' {
		t.Fatalf("first wrapped row line number rune = %q, want %q", got, '1')
	}
	if got := cellRune(screen, 0, 1); got != ' ' {
		t.Fatalf("continuation row gutter rune = %q, want space", got)
	}
}

func TestDrawLineNumbersWithHeaderColumnsInSoftWrap(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdefghi\njklmnopqr\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.SoftWrap, LineNumbers: true, HeaderColumns: 2})
	v.SetSize(8, 4)

	_, screen := newMockScreen(t, 8, 4)
	defer screen.Fini()
	v.Draw(screen)

	if got := cellRune(screen, 0, 0); got != '1' {
		t.Fatalf("first logical row line number rune = %q, want %q", got, '1')
	}
	if got := cellRune(screen, 0, 1); got != ' ' {
		t.Fatalf("first continuation row gutter rune = %q, want space", got)
	}
	if got := cellRune(screen, 0, 2); got != '2' {
		t.Fatalf("second logical row line number rune = %q, want %q", got, '2')
	}
	if got := cellRune(screen, 0, 3); got != ' ' {
		t.Fatalf("second continuation row gutter rune = %q, want space", got)
	}
}

func TestLineNumberStyleAppliesToBlankGutterRows(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdefghij\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	gutterBG := tcolor.NewRGBColor(0x11, 0x22, 0x33)
	v := New(doc, Config{
		TabWidth:    4,
		WrapMode:    layout.SoftWrap,
		LineNumbers: true,
		Chrome: Chrome{
			LineNumberStyle: tcell.StyleDefault.Background(gutterBG),
		},
	})
	v.SetSize(6, 4)

	_, screen := newMockScreen(t, 6, 4)
	defer screen.Fini()
	v.Draw(screen)

	_, style, _ := screen.Get(0, 1)
	if got, want := style.GetBackground(), gutterBG; got != want {
		t.Fatalf("continuation gutter background = %v, want %v", got, want)
	}
}

func TestHeaderLinesStayFixedInNoWrap(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("header\none\ntwo\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, HeaderLines: 1})
	v.SetSize(12, 3)
	v.ScrollDown(1)

	_, screen := newMockScreen(t, 12, 3)
	defer screen.Fini()
	v.Draw(screen)

	if got := screenRowString(screen, 0, 12); !strings.Contains(got, "header") {
		t.Fatalf("top row = %q, want fixed header line", got)
	}
	if got := screenRowString(screen, 1, 12); !strings.Contains(got, "two") {
		t.Fatalf("second row = %q, want scrolled body line", got)
	}
}

func TestHeaderLinesStayFixedAcrossWrappedRows(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdef\none\ntwo\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.SoftWrap, HeaderLines: 1})
	v.SetSize(4, 4)
	v.ScrollDown(1)

	_, screen := newMockScreen(t, 4, 4)
	defer screen.Fini()
	v.Draw(screen)

	if got := screenRowString(screen, 0, 4); got != "abcd" {
		t.Fatalf("first wrapped header row = %q, want %q", got, "abcd")
	}
	if got := screenRowString(screen, 1, 4); got != "ef  " {
		t.Fatalf("second wrapped header row = %q, want %q", got, "ef  ")
	}
}

func TestHeaderColumnsStayFixedInNoWrap(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("ABCDE12345\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, HeaderColumns: 2})
	v.SetSize(6, 2)
	v.ScrollRight(2)

	_, screen := newMockScreen(t, 6, 2)
	defer screen.Fini()
	v.Draw(screen)

	if got := screenRowString(screen, 0, 6); got != "ABE123" {
		t.Fatalf("row with fixed header columns = %q, want %q", got, "ABE123")
	}
}

func TestHeaderColumnsBlankOnWrappedContinuation(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdefghi\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.SoftWrap, HeaderColumns: 2})
	v.SetSize(6, 3)

	_, screen := newMockScreen(t, 6, 3)
	defer screen.Fini()
	v.Draw(screen)

	if got := screenRowString(screen, 0, 6); got != "abcdef" {
		t.Fatalf("first wrapped row with header columns = %q, want %q", got, "abcdef")
	}
	if got := screenRowString(screen, 1, 6); got != "  ghi " {
		t.Fatalf("continuation row with blank header columns = %q, want %q", got, "  ghi ")
	}
}

func TestHeaderColumnsDoNotShowClipMarkerForLeadingTab(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("\tabcdef\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, HeaderColumns: 2})
	v.SetSize(6, 2)

	_, screen := newMockScreen(t, 6, 2)
	defer screen.Fini()
	v.Draw(screen)

	if got := screenRowString(screen, 0, 6); strings.Contains(got, ">") {
		t.Fatalf("row with leading tab in fixed columns = %q, should not contain clip marker", got)
	}
}

func TestHeaderStyleAppliesToFixedRows(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("header\nbody\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	headerBG := tcolor.NewRGBColor(0xaa, 0xbb, 0xcc)
	v := New(doc, Config{
		TabWidth:    4,
		WrapMode:    layout.NoWrap,
		HeaderLines: 1,
		Chrome: Chrome{
			HeaderStyle: tcell.StyleDefault.Background(headerBG).Bold(true),
		},
	})
	v.SetSize(10, 3)

	_, screen := newMockScreen(t, 10, 3)
	defer screen.Fini()
	v.Draw(screen)

	_, headerStyle, _ := screen.Get(0, 0)
	if got, want := headerStyle.GetBackground(), headerBG; got != want {
		t.Fatalf("header background = %v, want %v", got, want)
	}
	if !headerStyle.HasBold() {
		t.Fatal("header style should be bold")
	}
	_, bodyStyle, _ := screen.Get(0, 1)
	if got, want := bodyStyle.GetBackground(), tcolor.Default; got != want {
		t.Fatalf("body background = %v, want %v", got, want)
	}
}

func TestDefaultHeaderStyleIsBold(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("header\nbody\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{
		TabWidth:    4,
		WrapMode:    layout.NoWrap,
		HeaderLines: 1,
	})
	v.SetSize(10, 3)

	_, screen := newMockScreen(t, 10, 3)
	defer screen.Fini()
	v.Draw(screen)

	_, headerStyle, _ := screen.Get(0, 0)
	if !headerStyle.HasBold() {
		t.Fatal("default header style should be bold")
	}
}

func TestHeaderStyleAppliesToFixedColumns(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("header\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	headerBG := tcolor.NewRGBColor(0xaa, 0xbb, 0xcc)
	v := New(doc, Config{
		TabWidth:      4,
		WrapMode:      layout.NoWrap,
		HeaderColumns: 2,
		Chrome: Chrome{
			HeaderStyle: tcell.StyleDefault.Background(headerBG).Bold(true),
		},
	})
	v.SetSize(8, 2)

	_, screen := newMockScreen(t, 8, 2)
	defer screen.Fini()
	v.Draw(screen)

	_, headerStyle, _ := screen.Get(0, 0)
	if got, want := headerStyle.GetBackground(), headerBG; got != want {
		t.Fatalf("header column background = %v, want %v", got, want)
	}
	_, bodyStyle, _ := screen.Get(2, 0)
	if got, want := bodyStyle.GetBackground(), tcolor.Default; got != want {
		t.Fatalf("body background = %v, want %v", got, want)
	}
}

func keyRune(s string) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, s, tcell.ModNone)
}

func keyRuneMod(s string, mod tcell.ModMask) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, s, mod)
}

func keyCtrlRune(s string) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, s, tcell.ModCtrl)
}

func keyKey(k tcell.Key) *tcell.EventKey {
	return tcell.NewEventKey(k, "", tcell.ModNone)
}

func keyKeyMod(k tcell.Key, mod tcell.ModMask) *tcell.EventKey {
	return tcell.NewEventKey(k, "", mod)
}

func cellRune(screen tcell.Screen, x, y int) rune {
	str, _, _ := screen.Get(x, y)
	if str == "" {
		return 0
	}
	return []rune(str)[0]
}

func screenRowString(screen tcell.Screen, y, width int) string {
	var b strings.Builder
	for x := 0; x < width; x++ {
		str, _, _ := screen.Get(x, y)
		if str == "" {
			b.WriteRune(' ')
			continue
		}
		b.WriteString(str)
	}
	return b.String()
}

func newMockScreen(t *testing.T, width, height int) (vt.MockTerm, tcell.Screen) {
	t.Helper()
	if runtime.GOOS == "js" {
		t.Skip("not supported on webasm")
	}
	term := vt.NewMockTerm(vt.MockOptSize{X: vt.Col(width), Y: vt.Row(height)})
	screen, err := tcell.NewTerminfoScreenFromTty(term)
	if err != nil {
		t.Fatalf("failed to get mock screen: %v", err)
	}
	if err := screen.Init(); err != nil {
		t.Fatalf("failed to initialize mock screen: %v", err)
	}
	return term, screen
}
