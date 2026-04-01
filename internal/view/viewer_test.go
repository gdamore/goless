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
	if got, want := v.prompt.String(), "/[smart,sub] "; got != want {
		t.Fatalf("prompt prefix = %q, want %q", got, want)
	}

	v.HandleKey(keyKey(tcell.KeyF2))
	if got, want := v.prompt.String(), "/[case,sub] "; got != want {
		t.Fatalf("prompt prefix after F2 = %q, want %q", got, want)
	}

	v.HandleKey(keyKey(tcell.KeyF3))
	if got, want := v.prompt.String(), "/[case,word] "; got != want {
		t.Fatalf("prompt prefix after F3 = %q, want %q", got, want)
	}

	v.HandleKey(keyKey(tcell.KeyF3))
	if got, want := v.prompt.String(), "/[case,regex] "; got != want {
		t.Fatalf("prompt prefix after second F3 = %q, want %q", got, want)
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
	if got := cellRune(screen, 1, 1); got != 'N' {
		t.Fatalf("help body rune = %q, want %q", got, 'N')
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
	if got, want := leftText, "search:smart,sub"; got != want {
		t.Fatalf("left status text = %q, want %q", got, want)
	}
	if !strings.Contains(rightText, "row 1/2") {
		t.Fatalf("right status text = %q, want row indicator", rightText)
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

	leftText, _ := v.statusText()
	if !strings.Contains(leftText, "search:nocase,sub") {
		t.Fatalf("left status text = %q, want active override label", leftText)
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

	leftText, _ := v.statusText()
	if got, want := leftText, "search:smart,word"; got != want {
		t.Fatalf("left status text after F3 = %q, want %q", got, want)
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
			StatusPosition: func(current, total, column int) string {
				return "界a"
			},
		},
	})
	v.SetSize(10, 2)
	v.relayout()

	_, screen := newMockScreen(t, 10, 2)
	defer screen.Fini()

	v.drawStatus(screen, 1)

	if got := cellRune(screen, 6, 1); got != '界' {
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
				if info.DefaultLeft == "" || info.DefaultRight == "" {
					t.Fatalf("StatusLine got empty defaults: %+v", info)
				}
				return "L:" + info.DefaultLeft, "R:" + info.DefaultRight
			},
		},
	})
	v.SetSize(20, 2)
	v.relayout()

	leftText, rightText := v.statusText()
	if got, want := leftText, "L:search:smart,sub"; got != want {
		t.Fatalf("left status text = %q, want %q", got, want)
	}
	if !strings.HasPrefix(rightText, "R:row 1/2  col 0") {
		t.Fatalf("right status text = %q, want prefix %q", rightText, "R:row 1/2  col 0")
	}
}

func TestPromptLineFormatterOverridesBuiltInPromptText(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Text: Text{
			PromptLine: func(info PromptInfo) string {
				if got, want := info.DefaultText, "/[smart,sub] a"; got != want {
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
	v.prompt.buffer = []rune("a")

	if got, want := v.promptText(), "find>a"; got != want {
		t.Fatalf("promptText() = %q, want %q", got, want)
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
	v.prompt.buffer = []rune("(")
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

func keyRune(s string) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, s, tcell.ModNone)
}

func keyKey(k tcell.Key) *tcell.EventKey {
	return tcell.NewEventKey(k, "", tcell.ModNone)
}

func cellRune(screen tcell.Screen, x, y int) rune {
	str, _, _ := screen.Get(x, y)
	if str == "" {
		return 0
	}
	return []rune(str)[0]
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
