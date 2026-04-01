// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
	tcolor "github.com/gdamore/tcell/v3/color"
	"github.com/gdamore/tcell/v3/vt"
)

func TestPagerAppendAndLen(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 4)

	if err := pager.AppendString("hello\nworld\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if got, want := pager.Len(), int64(len("hello\nworld\n")); got != want {
		t.Fatalf("Len = %d, want %d", got, want)
	}
}

func TestPagerReadFrom(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 4)

	n, err := pager.ReadFrom(strings.NewReader("alpha\nbeta\n"))
	if err != nil {
		t.Fatalf("ReadFrom failed: %v", err)
	}
	pager.Flush()

	if got, want := n, int64(len("alpha\nbeta\n")); got != want {
		t.Fatalf("ReadFrom count = %d, want %d", got, want)
	}
	if got, want := pager.Len(), int64(len("alpha\nbeta\n")); got != want {
		t.Fatalf("Len = %d, want %d", got, want)
	}
}

func TestPagerHandleKey(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 4)
	if err := pager.AppendString("hello\nworld\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if !pager.HandleKey(tcell.NewEventKey(tcell.KeyRune, "q", tcell.ModNone)) {
		t.Fatalf("HandleKey(q) = false, want true")
	}
}

func TestPagerHandleKeyResult(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 4)
	if err := pager.AppendString("hello\nworld\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone))
	if result.Handled {
		t.Fatal("HandleKeyResult(x).Handled = true, want false")
	}
	if result.Quit {
		t.Fatal("HandleKeyResult(x).Quit = true, want false")
	}

	result = pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "q", tcell.ModNone))
	if !result.Handled {
		t.Fatal("HandleKeyResult(q).Handled = false, want true")
	}
	if !result.Quit {
		t.Fatal("HandleKeyResult(q).Quit = false, want true")
	}
}

func TestPagerCaptureKeyReservesBinding(t *testing.T) {
	pager := New(Config{
		TabWidth:   4,
		WrapMode:   NoWrap,
		ShowStatus: true,
		CaptureKey: func(ev *tcell.EventKey) bool {
			return ev.Key() == tcell.KeyRune && ev.Str() == "n"
		},
	})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\nbeta\nalpha\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()
	if !pager.SearchForward("alpha") {
		t.Fatal("SearchForward(alpha) = false, want true")
	}

	result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "n", tcell.ModNone))
	if result.Handled {
		t.Fatal("HandleKeyResult(n).Handled = true, want false when captured")
	}
	if result.Quit {
		t.Fatal("HandleKeyResult(n).Quit = true, want false")
	}
	if got, want := pager.Position().Row, 1; got != want {
		t.Fatalf("Position().Row after captured n = %d, want %d", got, want)
	}
}

func TestPagerEmptyKeyGroupDisablesBundledBindings(t *testing.T) {
	pager := New(Config{
		KeyGroup:   EmptyKeyGroup,
		TabWidth:   4,
		WrapMode:   NoWrap,
		ShowStatus: true,
	})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\nbeta\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "q", tcell.ModNone))
	if result.Handled {
		t.Fatal("HandleKeyResult(q).Handled = true, want false with EmptyKeyGroup")
	}
	if result.Quit {
		t.Fatal("HandleKeyResult(q).Quit = true, want false with EmptyKeyGroup")
	}
}

func TestPagerUnbindKeyRemovesSpecificDefaultBinding(t *testing.T) {
	pager := New(Config{
		KeyGroup: LessKeyGroup,
		UnbindKeys: []KeyStroke{
			{Context: NormalKeyContext, Key: tcell.KeyRune, Rune: "n"},
		},
		TabWidth:   4,
		WrapMode:   NoWrap,
		ShowStatus: true,
	})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\nbeta\nalpha\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()
	if !pager.SearchForward("alpha") {
		t.Fatal("SearchForward(alpha) = false, want true")
	}

	result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "n", tcell.ModNone))
	if result.Handled {
		t.Fatal("HandleKeyResult(n).Handled = true, want false after unbind")
	}
	if got, want := pager.SearchState().CurrentMatch, 1; got != want {
		t.Fatalf("SearchState().CurrentMatch after n = %d, want %d", got, want)
	}

	result = pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "N", tcell.ModNone))
	if !result.Handled {
		t.Fatal("HandleKeyResult(N).Handled = false, want true")
	}
	if got, want := pager.SearchState().CurrentMatch, 2; got != want {
		t.Fatalf("SearchState().CurrentMatch after N = %d, want %d", got, want)
	}
}

func TestPagerKeyBindingsOverrideBundledBindings(t *testing.T) {
	pager := New(Config{
		KeyGroup: LessKeyGroup,
		KeyBindings: []KeyBinding{
			{
				KeyStroke: KeyStroke{Context: NormalKeyContext, Key: tcell.KeyRune, Rune: "x"},
				Action:    KeyActionSearchNext,
			},
			{
				KeyStroke: KeyStroke{Context: PromptKeyContext, Key: tcell.KeyF4},
				Action:    KeyActionCycleSearchMode,
			},
		},
		UnbindKeys: []KeyStroke{
			{Context: PromptKeyContext, Key: tcell.KeyF3},
		},
		TabWidth:   4,
		WrapMode:   NoWrap,
		ShowStatus: true,
	})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\nbeta\nalpha\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()
	if !pager.SearchForward("alpha") {
		t.Fatal("SearchForward(alpha) = false, want true")
	}

	result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone))
	if !result.Handled {
		t.Fatal("HandleKeyResult(x).Handled = false, want true for custom binding")
	}
	if got, want := pager.SearchState().CurrentMatch, 2; got != want {
		t.Fatalf("SearchState().CurrentMatch after x = %d, want %d", got, want)
	}

	result = pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "/", tcell.ModNone))
	if !result.Handled {
		t.Fatal("HandleKeyResult(/).Handled = false, want true")
	}
	result = pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyF3, "", tcell.ModNone))
	if result.Handled {
		t.Fatal("HandleKeyResult(F3).Handled = true, want false after prompt unbind")
	}
	if got, want := pager.SearchMode(), SearchSubstring; got != want {
		t.Fatalf("SearchMode() after F3 = %v, want %v", got, want)
	}

	result = pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyF4, "", tcell.ModNone))
	if !result.Handled {
		t.Fatal("HandleKeyResult(F4).Handled = false, want true for prompt binding")
	}
	if got, want := pager.SearchMode(), SearchWholeWord; got != want {
		t.Fatalf("SearchMode() after F4 = %v, want %v", got, want)
	}
}

func TestPagerPromptKeyBindingOverridesBuiltins(t *testing.T) {
	pager := New(Config{
		KeyGroup: LessKeyGroup,
		KeyBindings: []KeyBinding{
			{
				KeyStroke: KeyStroke{Context: PromptKeyContext, Key: tcell.KeyEscape},
				Action:    KeyActionQuit,
			},
		},
		TabWidth:   4,
		WrapMode:   NoWrap,
		ShowStatus: true,
	})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\nbeta\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "/", tcell.ModNone))
	if !result.Handled {
		t.Fatal("HandleKeyResult(/).Handled = false, want true")
	}

	result = pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyEscape, "", tcell.ModNone))
	if !result.Handled {
		t.Fatal("HandleKeyResult(Escape).Handled = false, want true")
	}
	if !result.Quit {
		t.Fatal("HandleKeyResult(Escape).Quit = false, want true for prompt override")
	}
}

func TestPagerSearchMethods(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\nbeta\nalpha\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if !pager.SearchForward("alpha") {
		t.Fatal("SearchForward(alpha) = false, want true")
	}
	if got, want := pager.Position().Row, 1; got != want {
		t.Fatalf("Position().Row after SearchForward = %d, want %d", got, want)
	}
	if !pager.SearchNext() {
		t.Fatal("SearchNext() = false, want true")
	}
	if got, want := pager.Position().Row, 3; got != want {
		t.Fatalf("Position().Row after SearchNext = %d, want %d", got, want)
	}
	if !pager.SearchPrev() {
		t.Fatal("SearchPrev() = false, want true")
	}
	if got, want := pager.Position().Row, 1; got != want {
		t.Fatalf("Position().Row after SearchPrev = %d, want %d", got, want)
	}
	if pager.SearchForward("missing") {
		t.Fatal("SearchForward(missing) = true, want false")
	}
}

func TestPagerWrapModeAndPosition(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(5, 4)
	if err := pager.AppendString("abcdefghij\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()
	pager.ScrollRight(3)

	pos := pager.Position()
	if got, want := pos.Column, 3; got != want {
		t.Fatalf("Position().Column = %d, want %d", got, want)
	}
	if got, want := pager.WrapMode(), NoWrap; got != want {
		t.Fatalf("WrapMode() = %v, want %v", got, want)
	}

	pager.SetWrapMode(SoftWrap)
	if got, want := pager.WrapMode(), SoftWrap; got != want {
		t.Fatalf("WrapMode() after SetWrapMode = %v, want %v", got, want)
	}
	if got := pager.Position().Column; got != 0 {
		t.Fatalf("Position().Column after SoftWrap = %d, want 0", got)
	}
}

func TestPagerJumpToLine(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	if err := pager.AppendString("one\ntwo\nthree\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if !pager.JumpToLine(3) {
		t.Fatal("JumpToLine(3) = false, want true")
	}
	if got, want := pager.Position().Row, 3; got != want {
		t.Fatalf("Position().Row after JumpToLine = %d, want %d", got, want)
	}
	if pager.JumpToLine(4) {
		t.Fatal("JumpToLine(4) = true, want false")
	}
}

func TestPagerSearchForwardSmartCase(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	if err := pager.AppendString("Alpha\nbeta\nALPHA\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if !pager.SearchForward("alpha") {
		t.Fatal("SearchForward(alpha) = false, want true under smart-case")
	}
	if !pager.SearchNext() {
		t.Fatal("SearchNext() = false, want true")
	}
	if got, want := pager.Position().Row, 3; got != want {
		t.Fatalf("Position().Row after smart-case SearchNext = %d, want %d", got, want)
	}
}

func TestPagerSearchForwardWithCaseInsensitive(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\nBeta\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if !pager.SearchForwardWithCase("BETA", SearchCaseInsensitive) {
		t.Fatal("SearchForwardWithCase(BETA, SearchCaseInsensitive) = false, want true")
	}
	if got, want := pager.Position().Row, 2; got != want {
		t.Fatalf("Position().Row after case-insensitive search = %d, want %d", got, want)
	}
}

func TestPagerSetSearchCaseMode(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\nBeta\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()
	pager.SetSearchCaseMode(SearchCaseSensitive)

	if got, want := pager.SearchCaseMode(), SearchCaseSensitive; got != want {
		t.Fatalf("SearchCaseMode() = %v, want %v", got, want)
	}
	if pager.SearchForward("BETA") {
		t.Fatal("SearchForward(BETA) = true, want false under case-sensitive mode")
	}
}

func TestPagerCycleSearchCaseMode(t *testing.T) {
	pager := New(Config{})

	if got, want := pager.CycleSearchCaseMode(), SearchCaseSensitive; got != want {
		t.Fatalf("CycleSearchCaseMode() = %v, want %v", got, want)
	}
	if got, want := pager.CycleSearchCaseMode(), SearchCaseInsensitive; got != want {
		t.Fatalf("CycleSearchCaseMode() second = %v, want %v", got, want)
	}
	if got, want := pager.CycleSearchCaseMode(), SearchSmartCase; got != want {
		t.Fatalf("CycleSearchCaseMode() third = %v, want %v", got, want)
	}
}

func TestPagerSearchStateDefaults(t *testing.T) {
	pager := New(Config{})
	pager.SetSearchCaseMode(SearchCaseSensitive)
	pager.SetSearchMode(SearchRegex)

	state := pager.SearchState()
	if state.Query != "" {
		t.Fatalf("SearchState().Query = %q, want empty", state.Query)
	}
	if !state.Forward {
		t.Fatal("SearchState().Forward = false, want true default")
	}
	if got, want := state.CaseMode, SearchCaseSensitive; got != want {
		t.Fatalf("SearchState().CaseMode = %v, want %v", got, want)
	}
	if got, want := state.Mode, SearchRegex; got != want {
		t.Fatalf("SearchState().Mode = %v, want %v", got, want)
	}
	if state.MatchCount != 0 || state.CurrentMatch != 0 {
		t.Fatalf("SearchState() matches = %d/%d, want 0/0", state.CurrentMatch, state.MatchCount)
	}
	if state.CompileError != "" {
		t.Fatalf("SearchState().CompileError = %q, want empty", state.CompileError)
	}
	if state.Preview {
		t.Fatal("SearchState().Preview = true, want false")
	}
}

func TestPagerSearchStateCommittedSearch(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\nbeta\nalpha\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if !pager.SearchForward("alpha") {
		t.Fatal("SearchForward(alpha) = false, want true")
	}

	state := pager.SearchState()
	if got, want := state.Query, "alpha"; got != want {
		t.Fatalf("SearchState().Query = %q, want %q", got, want)
	}
	if !state.Forward {
		t.Fatal("SearchState().Forward = false, want true")
	}
	if got, want := state.CaseMode, SearchSmartCase; got != want {
		t.Fatalf("SearchState().CaseMode = %v, want %v", got, want)
	}
	if got, want := state.Mode, SearchSubstring; got != want {
		t.Fatalf("SearchState().Mode = %v, want %v", got, want)
	}
	if got, want := state.MatchCount, 2; got != want {
		t.Fatalf("SearchState().MatchCount = %d, want %d", got, want)
	}
	if got, want := state.CurrentMatch, 1; got != want {
		t.Fatalf("SearchState().CurrentMatch = %d, want %d", got, want)
	}
	if state.CompileError != "" {
		t.Fatalf("SearchState().CompileError = %q, want empty", state.CompileError)
	}
	if state.Preview {
		t.Fatal("SearchState().Preview = true, want false")
	}
}

func TestPagerSearchStateShowsPreview(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\nalpine\nbeta\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if got := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "/", tcell.ModNone)); !got.Handled {
		t.Fatal("HandleKeyResult(/).Handled = false, want true")
	}
	if got := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "a", tcell.ModNone)); !got.Handled {
		t.Fatal("HandleKeyResult(a).Handled = false, want true")
	}

	state := pager.SearchState()
	if got, want := state.Query, "a"; got != want {
		t.Fatalf("SearchState().Query = %q, want %q", got, want)
	}
	if !state.Forward {
		t.Fatal("SearchState().Forward = false, want true")
	}
	if got, want := state.MatchCount, 4; got != want {
		t.Fatalf("SearchState().MatchCount = %d, want %d", got, want)
	}
	if got, want := state.CurrentMatch, 1; got != want {
		t.Fatalf("SearchState().CurrentMatch = %d, want %d", got, want)
	}
	if state.CompileError != "" {
		t.Fatalf("SearchState().CompileError = %q, want empty", state.CompileError)
	}
	if !state.Preview {
		t.Fatal("SearchState().Preview = false, want true")
	}
}

func TestPagerSearchStateShowsInvalidRegexPreview(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	pager.SetSearchMode(SearchRegex)
	if err := pager.AppendString("alpha\nalpine\nbeta\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "/", tcell.ModNone))
	pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "a", tcell.ModNone))
	pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "(", tcell.ModNone))

	state := pager.SearchState()
	if got, want := state.Query, "a("; got != want {
		t.Fatalf("SearchState().Query = %q, want %q", got, want)
	}
	if got, want := state.Mode, SearchRegex; got != want {
		t.Fatalf("SearchState().Mode = %v, want %v", got, want)
	}
	if state.CompileError == "" {
		t.Fatal("SearchState().CompileError = empty, want regex error")
	}
	if state.MatchCount != 0 || state.CurrentMatch != 0 {
		t.Fatalf("SearchState() matches = %d/%d, want 0/0 for invalid regex", state.CurrentMatch, state.MatchCount)
	}
	if !state.Preview {
		t.Fatal("SearchState().Preview = false, want true")
	}
}

func TestPagerTextHooksFormatStatusAndPrompt(t *testing.T) {
	statusCalled := false
	promptCalled := false
	pager := New(Config{
		TabWidth:   4,
		WrapMode:   NoWrap,
		ShowStatus: true,
		Text: Text{
			StatusLine: func(info StatusInfo) (left, right string) {
				statusCalled = true
				if got, want := info.Position.Row, 1; got != want {
					t.Fatalf("StatusLine position row = %d, want %d", got, want)
				}
				return "LEFT", "RIGHT"
			},
			PromptLine: func(info PromptInfo) string {
				promptCalled = true
				if got, want := info.Kind, PromptSearchForward; got != want {
					t.Fatalf("PromptLine kind = %v, want %v", got, want)
				}
				return "find>" + info.Input
			},
		},
	})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	screen := newPagerMockScreen(t, 20, 2)
	defer screen.Fini()

	pager.Draw(screen)
	if !statusCalled {
		t.Fatal("StatusLine hook was not called")
	}
	if got := pagerCellRune(screen, 2, 1); got != 'L' {
		t.Fatalf("status text rune = %q, want %q", got, 'L')
	}

	if result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "/", tcell.ModNone)); !result.Handled {
		t.Fatal("HandleKeyResult(/).Handled = false, want true")
	}
	if result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "a", tcell.ModNone)); !result.Handled {
		t.Fatal("HandleKeyResult(a).Handled = false, want true")
	}
	screen.Clear()
	pager.Draw(screen)
	if !promptCalled {
		t.Fatal("PromptLine hook was not called")
	}
	if got := pagerCellRune(screen, 1, 1); got != 'f' {
		t.Fatalf("prompt text rune = %q, want %q", got, 'f')
	}
}

func TestPagerThemeAffectsContentColorsOnly(t *testing.T) {
	pager := New(Config{
		TabWidth:   4,
		WrapMode:   NoWrap,
		ShowStatus: true,
		Theme: Theme{
			DefaultFG: tcolor.Red,
			DefaultBG: tcolor.Blue,
			ANSI: [16]tcolor.Color{
				1: tcolor.Aqua,
			},
		},
	})
	pager.SetSize(20, 2)
	if err := pager.AppendString("plain\n\x1b[31mansi\x1b[0m\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	screen := newPagerMockScreen(t, 20, 2)
	defer screen.Fini()

	pager.Draw(screen)
	_, plainStyle, _ := screen.Get(0, 0)
	if got, want := plainStyle.GetForeground(), tcolor.Red; got != want {
		t.Fatalf("plain fg = %v, want %v", got, want)
	}
	if got, want := plainStyle.GetBackground(), tcolor.Blue; got != want {
		t.Fatalf("plain bg = %v, want %v", got, want)
	}

	pager.ScrollDown(1)
	screen.Clear()
	pager.Draw(screen)
	_, ansiStyle, _ := screen.Get(0, 0)
	if got, want := ansiStyle.GetForeground(), tcolor.Aqua; got != want {
		t.Fatalf("ansi fg = %v, want %v", got, want)
	}
	if got, want := ansiStyle.GetBackground(), tcolor.Blue; got != want {
		t.Fatalf("ansi bg = %v, want %v", got, want)
	}
}

func TestPagerSetSearchMode(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alphabet alpha\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()
	pager.SetSearchMode(SearchWholeWord)

	if got, want := pager.SearchMode(), SearchWholeWord; got != want {
		t.Fatalf("SearchMode() = %v, want %v", got, want)
	}
	if !pager.SearchForward("alpha") {
		t.Fatal("SearchForward(alpha) = false, want true under whole-word mode")
	}
}

func TestPagerCycleSearchMode(t *testing.T) {
	pager := New(Config{})

	if got, want := pager.CycleSearchMode(), SearchWholeWord; got != want {
		t.Fatalf("CycleSearchMode() = %v, want %v", got, want)
	}
	if got, want := pager.CycleSearchMode(), SearchRegex; got != want {
		t.Fatalf("CycleSearchMode() second = %v, want %v", got, want)
	}
	if got, want := pager.CycleSearchMode(), SearchSubstring; got != want {
		t.Fatalf("CycleSearchMode() third = %v, want %v", got, want)
	}
}

func TestPagerRegexSearch(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true, SearchMode: SearchRegex})
	pager.SetSize(20, 2)
	if err := pager.AppendString("error 500\nwarning 404\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if !pager.SearchForward(`[45]0[04]`) {
		t.Fatal("SearchForward(regex) = false, want true")
	}
	if !pager.SearchNext() {
		t.Fatal("SearchNext() = false, want true")
	}
	if got, want := pager.Position().Row, 2; got != want {
		t.Fatalf("Position().Row after regex SearchNext = %d, want %d", got, want)
	}
}

func TestPagerSearchPreservesWhitespace(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	if err := pager.AppendString("a b\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if !pager.SearchForward(" ") {
		t.Fatal("SearchForward(space) = false, want true")
	}
}

func pagerCellRune(screen tcell.Screen, x, y int) rune {
	str, _, _ := screen.Get(x, y)
	if str == "" {
		return 0
	}
	return []rune(str)[0]
}

func newPagerMockScreen(t *testing.T, width, height int) tcell.Screen {
	t.Helper()
	term := vt.NewMockTerm(vt.MockOptSize{X: vt.Col(width), Y: vt.Row(height)})
	screen, err := tcell.NewTerminfoScreenFromTty(term)
	if err != nil {
		t.Fatalf("failed to get mock screen: %v", err)
	}
	if err := screen.Init(); err != nil {
		t.Fatalf("failed to initialize mock screen: %v", err)
	}
	return screen
}
