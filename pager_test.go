// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import (
	"runtime"
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
	if got, want := result.Action, KeyActionNone; got != want {
		t.Fatalf("HandleKeyResult(x).Action = %v, want %v", got, want)
	}

	result = pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "q", tcell.ModNone))
	if !result.Handled {
		t.Fatal("HandleKeyResult(q).Handled = false, want true")
	}
	if !result.Quit {
		t.Fatal("HandleKeyResult(q).Quit = false, want true")
	}
	if got, want := result.Action, KeyActionQuit; got != want {
		t.Fatalf("HandleKeyResult(q).Action = %v, want %v", got, want)
	}
	if got, want := result.Context, NormalKeyContext; got != want {
		t.Fatalf("HandleKeyResult(q).Context = %v, want %v", got, want)
	}
}

func TestPagerHandleKeyResultPromptContext(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 4)
	if err := pager.AppendString("hello\nworld\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if got := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "/", tcell.ModNone)); !got.Handled {
		t.Fatal("HandleKeyResult(/).Handled = false, want true")
	}
	result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "f", tcell.ModNone))
	if !result.Handled {
		t.Fatal("HandleKeyResult(f).Handled = false, want true in prompt")
	}
	if result.Quit {
		t.Fatal("HandleKeyResult(f).Quit = true, want false in prompt")
	}
	if got, want := result.Action, KeyActionNone; got != want {
		t.Fatalf("HandleKeyResult(f).Action = %v, want %v in prompt", got, want)
	}
	if got, want := result.Context, PromptKeyContext; got != want {
		t.Fatalf("HandleKeyResult(f).Context = %v, want %v", got, want)
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
	pager.SetSize(30, 2)
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

func TestPagerConstructionOptionsSupportKeyCustomizationAndCapture(t *testing.T) {
	pager := New(
		WithKeyGroup(LessKeyGroup),
		WithUnboundKeys(KeyStroke{Context: NormalKeyContext, Key: tcell.KeyRune, Rune: "n"}),
		WithKeyBindings(
			KeyBinding{
				KeyStroke: KeyStroke{Context: NormalKeyContext, Key: tcell.KeyRune, Rune: "x"},
				Action:    KeyActionSearchNext,
			},
		),
		WithCaptureKey(func(ev *tcell.EventKey) bool {
			return ev.Key() == tcell.KeyRune && ev.Str() == "n"
		}),
		WithTabWidth(4),
		WithWrapMode(NoWrap),
		WithShowStatus(true),
	)
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
	if got, want := pager.SearchState().CurrentMatch, 1; got != want {
		t.Fatalf("SearchState().CurrentMatch after captured n = %d, want %d", got, want)
	}

	result = pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone))
	if !result.Handled {
		t.Fatal("HandleKeyResult(x).Handled = false, want true for custom binding")
	}
	if got, want := pager.SearchState().CurrentMatch, 2; got != want {
		t.Fatalf("SearchState().CurrentMatch after x = %d, want %d", got, want)
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

func TestPagerCommandHandlerHandlesUnknownCommand(t *testing.T) {
	handled := false
	pager := New(Config{
		TabWidth:   4,
		WrapMode:   NoWrap,
		ShowStatus: true,
		Text: Text{
			StatusLine: func(info StatusInfo) (left, right string) {
				return info.Message, ""
			},
		},
		CommandHandler: func(cmd Command) CommandResult {
			handled = true
			if got, want := cmd.Raw, "next file"; got != want {
				t.Fatalf("Command.Raw = %q, want %q", got, want)
			}
			if got, want := cmd.Name, "next"; got != want {
				t.Fatalf("Command.Name = %q, want %q", got, want)
			}
			if got, want := strings.Join(cmd.Args, ","), "file"; got != want {
				t.Fatalf("Command.Args = %q, want %q", got, want)
			}
			return CommandResult{
				Handled: true,
				Message: "advanced",
			}
		},
	})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\nbeta\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, ":", tcell.ModNone)); !result.Handled {
		t.Fatal("HandleKeyResult(:).Handled = false, want true")
	}
	for _, r := range "next file" {
		pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
	}
	result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if !result.Handled {
		t.Fatal("HandleKeyResult(Enter).Handled = false, want true")
	}
	if result.Quit {
		t.Fatal("HandleKeyResult(Enter).Quit = true, want false")
	}
	if !handled {
		t.Fatal("CommandHandler was not called")
	}

	screen := newPagerMockScreen(t, 30, 2)
	defer screen.Fini()
	pager.Draw(screen)
	if got := pagerRowString(screen, 1, 30); !strings.Contains(got, "advanced") {
		t.Fatalf("status line = %q, want message", got)
	}
}

func TestPagerOptionsSupportHandlersTextAndRenderMode(t *testing.T) {
	handled := false
	pager := New(
		WithTabWidth(4),
		WithWrapMode(NoWrap),
		WithRenderMode(RenderPresentation),
		WithShowStatus(true),
		WithText(Text{
			StatusLine: func(info StatusInfo) (left, right string) {
				return info.Message, ""
			},
		}),
		WithHyperlinkHandler(func(info HyperlinkInfo) HyperlinkDecision {
			return HyperlinkDecision{Live: true}
		}),
	)
	pager.Configure(WithCommandHandler(func(cmd Command) CommandResult {
		handled = true
		return CommandResult{Handled: true, Message: "configured"}
	}))
	pager.SetSize(20, 2)
	if err := pager.AppendString("x\x1b]8;id=demo;https://example.com\aY\x1b]8;;\aZ"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	screen := newPagerMockScreen(t, 20, 2)
	defer screen.Fini()
	pager.Draw(screen)
	if id, url := pagerCellStyle(screen, 1, 0).GetUrl(); id != "demo" || url != "https://example.com" {
		t.Fatalf("live link = (%q, %q), want (%q, %q)", id, url, "demo", "https://example.com")
	}

	pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, ":", tcell.ModNone))
	for _, r := range "noop" {
		pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
	}
	result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if !result.Handled || result.Quit {
		t.Fatalf("HandleKeyResult(Enter) = %+v, want handled non-quit", result)
	}
	if !handled {
		t.Fatal("configured CommandHandler was not called")
	}

	screen.Clear()
	pager.Draw(screen)
	if got := pagerRowString(screen, 1, 20); !strings.Contains(got, "configured") {
		t.Fatalf("status line = %q, want configured message", got)
	}
}

func TestPagerCommandHandlerCanKeepPromptOpen(t *testing.T) {
	pager := New(Config{
		TabWidth:   4,
		WrapMode:   NoWrap,
		ShowStatus: true,
		CommandHandler: func(cmd Command) CommandResult {
			return CommandResult{
				Handled:    true,
				Message:    "need more",
				KeepPrompt: true,
			}
		},
	})
	pager.SetSize(30, 2)
	if err := pager.AppendString("alpha\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, ":", tcell.ModNone))
	pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "n", tcell.ModNone))
	result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if !result.Handled {
		t.Fatal("HandleKeyResult(Enter).Handled = false, want true")
	}
	if result.Quit {
		t.Fatal("HandleKeyResult(Enter).Quit = true, want false")
	}

	screen := newPagerMockScreen(t, 30, 2)
	defer screen.Fini()
	pager.Draw(screen)
	if got := pagerRowString(screen, 1, 30); !strings.Contains(got, ":n") {
		t.Fatalf("prompt line = %q, want command prompt to remain open", got)
	}
}

func TestPagerCommandHandlerCanRequestQuit(t *testing.T) {
	pager := New(Config{
		TabWidth:   4,
		WrapMode:   NoWrap,
		ShowStatus: true,
		CommandHandler: func(cmd Command) CommandResult {
			return CommandResult{Handled: true, Quit: true}
		},
	})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, ":", tcell.ModNone))
	pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "q", tcell.ModNone))
	result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if !result.Handled {
		t.Fatal("HandleKeyResult(Enter).Handled = false, want true")
	}
	if !result.Quit {
		t.Fatal("HandleKeyResult(Enter).Quit = false, want true")
	}
}

func TestPagerBuiltInCommandWinsBeforeCommandHandler(t *testing.T) {
	handled := false
	pager := New(Config{
		TabWidth:   4,
		WrapMode:   NoWrap,
		ShowStatus: true,
		CommandHandler: func(cmd Command) CommandResult {
			handled = true
			return CommandResult{Handled: true, Message: "override"}
		},
	})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\nbeta\ngamma\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, ":", tcell.ModNone))
	pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "2", tcell.ModNone))
	result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if !result.Handled {
		t.Fatal("HandleKeyResult(Enter).Handled = false, want true")
	}
	if handled {
		t.Fatal("CommandHandler was called for built-in line jump")
	}
	if got, want := pager.Position().Row, 2; got != want {
		t.Fatalf("Position().Row = %d, want %d", got, want)
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
	if got, want := pos.Column, 4; got != want {
		t.Fatalf("Position().Column = %d, want %d", got, want)
	}
	if got, want := pos.Columns, 10; got != want {
		t.Fatalf("Position().Columns = %d, want %d", got, want)
	}
	if got, want := pager.WrapMode(), NoWrap; got != want {
		t.Fatalf("WrapMode() = %v, want %v", got, want)
	}

	pager.SetWrapMode(SoftWrap)
	if got, want := pager.WrapMode(), SoftWrap; got != want {
		t.Fatalf("WrapMode() after SetWrapMode = %v, want %v", got, want)
	}
	if got := pager.Position().Column; got != 1 {
		t.Fatalf("Position().Column after SoftWrap = %d, want 1", got)
	}
	if got, want := pager.Position().Columns, 10; got != want {
		t.Fatalf("Position().Columns after SoftWrap = %d, want %d", got, want)
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

func TestPagerHalfPageAndPercentNavigation(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 4)
	if err := pager.AppendString("one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	pager.HalfPageDown()
	if got, want := pager.Position().Row, 2; got != want {
		t.Fatalf("Position().Row after HalfPageDown = %d, want %d", got, want)
	}

	pager.HalfPageUp()
	if got, want := pager.Position().Row, 1; got != want {
		t.Fatalf("Position().Row after HalfPageUp = %d, want %d", got, want)
	}

	if !pager.GoPercent(50) {
		t.Fatal("GoPercent(50) = false, want true")
	}
	if got, want := pager.Position().Row, 4; got != want {
		t.Fatalf("Position().Row after GoPercent(50) = %d, want %d", got, want)
	}
}

func TestPagerLineNumberToggle(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	if pager.LineNumbers() {
		t.Fatal("LineNumbers() = true, want false by default")
	}

	pager.SetLineNumbers(true)
	if !pager.LineNumbers() {
		t.Fatal("LineNumbers() = false, want true after SetLineNumbers(true)")
	}

	pager.ToggleLineNumbers()
	if pager.LineNumbers() {
		t.Fatal("LineNumbers() = true, want false after ToggleLineNumbers()")
	}
}

func TestPagerNewWithOptions(t *testing.T) {
	pager := New(
		WithTabWidth(4),
		WithWrapMode(SoftWrap),
		WithLineNumbers(true),
		WithHeaderLines(2),
		WithHeaderColumns(3),
		WithShowStatus(true),
	)

	if got, want := pager.WrapMode(), SoftWrap; got != want {
		t.Fatalf("WrapMode() = %v, want %v", got, want)
	}
	if !pager.LineNumbers() {
		t.Fatal("LineNumbers() = false, want true")
	}
	if got, want := pager.HeaderLines(), 2; got != want {
		t.Fatalf("HeaderLines() = %d, want %d", got, want)
	}
	if got, want := pager.HeaderColumns(), 3; got != want {
		t.Fatalf("HeaderColumns() = %d, want %d", got, want)
	}
}

func TestPagerConfigureAppliesRuntimeOptions(t *testing.T) {
	pager := New(WithTabWidth(4), WithWrapMode(NoWrap), WithShowStatus(true))

	pager.Configure(
		WithWrapMode(SoftWrap),
		WithLineNumbers(true),
		WithSqueezeBlankLines(true),
		WithHeaderLines(2),
		WithHeaderColumns(3),
		WithSearchCaseMode(SearchCaseSensitive),
		WithSearchMode(SearchRegex),
	)

	if got, want := pager.WrapMode(), SoftWrap; got != want {
		t.Fatalf("WrapMode() after Configure = %v, want %v", got, want)
	}
	if !pager.LineNumbers() {
		t.Fatal("LineNumbers() after Configure = false, want true")
	}
	if !pager.SqueezeBlankLines() {
		t.Fatal("SqueezeBlankLines() after Configure = false, want true")
	}
	if got, want := pager.HeaderLines(), 2; got != want {
		t.Fatalf("HeaderLines() after Configure = %d, want %d", got, want)
	}
	if got, want := pager.HeaderColumns(), 3; got != want {
		t.Fatalf("HeaderColumns() after Configure = %d, want %d", got, want)
	}
	if got, want := pager.SearchCaseMode(), SearchCaseSensitive; got != want {
		t.Fatalf("SearchCaseMode() after Configure = %v, want %v", got, want)
	}
	if got, want := pager.SearchMode(), SearchRegex; got != want {
		t.Fatalf("SearchMode() after Configure = %v, want %v", got, want)
	}
}

func TestPagerSqueezeBlankLines(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	if pager.SqueezeBlankLines() {
		t.Fatal("SqueezeBlankLines() = true, want false by default")
	}

	pager.SetSqueezeBlankLines(true)
	if !pager.SqueezeBlankLines() {
		t.Fatal("SqueezeBlankLines() = false, want true after SetSqueezeBlankLines(true)")
	}
}

func TestPagerHeaderLines(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	if got, want := pager.HeaderLines(), 0; got != want {
		t.Fatalf("HeaderLines() = %d, want %d", got, want)
	}

	pager.SetHeaderLines(2)
	if got, want := pager.HeaderLines(), 2; got != want {
		t.Fatalf("HeaderLines() after SetHeaderLines(2) = %d, want %d", got, want)
	}

	pager.SetHeaderLines(-1)
	if got, want := pager.HeaderLines(), 0; got != want {
		t.Fatalf("HeaderLines() after SetHeaderLines(-1) = %d, want %d", got, want)
	}
}

func TestPagerHeaderColumns(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	if got, want := pager.HeaderColumns(), 0; got != want {
		t.Fatalf("HeaderColumns() = %d, want %d", got, want)
	}

	pager.SetHeaderColumns(3)
	if got, want := pager.HeaderColumns(), 3; got != want {
		t.Fatalf("HeaderColumns() after SetHeaderColumns(3) = %d, want %d", got, want)
	}

	pager.SetHeaderColumns(-1)
	if got, want := pager.HeaderColumns(), 0; got != want {
		t.Fatalf("HeaderColumns() after SetHeaderColumns(-1) = %d, want %d", got, want)
	}
}

func TestPagerJumpToLineUsesSqueezedView(t *testing.T) {
	pager := New(Config{
		TabWidth:          4,
		WrapMode:          NoWrap,
		ShowStatus:        true,
		SqueezeBlankLines: true,
	})
	pager.SetSize(20, 2)
	if err := pager.AppendString("one\n\n\nthree\nfour\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if !pager.JumpToLine(3) {
		t.Fatal("JumpToLine(3) = false, want true")
	}
	if got, want := pager.Position().Row, 3; got != want {
		t.Fatalf("Position().Row after squeezed JumpToLine = %d, want %d", got, want)
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
	statusStyle := tcell.StyleDefault.Foreground(tcolor.PaletteColor(3)).Background(tcolor.PaletteColor(7))
	promptStyle := tcell.StyleDefault.Foreground(tcolor.PaletteColor(2)).Background(tcolor.PaletteColor(0)).Bold(true)
	promptErrorStyle := tcell.StyleDefault.Foreground(tcolor.PaletteColor(1)).Background(tcolor.PaletteColor(0))
	pager := New(Config{
		TabWidth:   4,
		WrapMode:   NoWrap,
		ShowStatus: true,
		SearchMode: SearchRegex,
		Chrome:     Chrome{StatusStyle: statusStyle, PromptStyle: promptStyle, PromptErrorStyle: promptErrorStyle},
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
	if got := pagerCellRune(screen, 3, 1); got != 'L' {
		t.Fatalf("status text rune = %q, want %q", got, 'L')
	}
	if got, want := pagerCellStyle(screen, 0, 1).GetForeground(), statusStyle.GetForeground(); got != want {
		t.Fatalf("status fg = %v, want %v", got, want)
	}
	if got, want := pagerCellStyle(screen, 0, 1).GetBackground(), statusStyle.GetBackground(); got != want {
		t.Fatalf("status bg = %v, want %v", got, want)
	}

	if result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "/", tcell.ModNone)); !result.Handled {
		t.Fatal("HandleKeyResult(/).Handled = false, want true")
	}
	if result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "(", tcell.ModNone)); !result.Handled {
		t.Fatal("HandleKeyResult(()).Handled = false, want true")
	}
	screen.Clear()
	pager.Draw(screen)
	if !promptCalled {
		t.Fatal("PromptLine hook was not called")
	}
	if got := pagerCellRune(screen, 1, 1); got != 'f' {
		t.Fatalf("prompt text rune = %q, want %q", got, 'f')
	}
	promptCellStyle := pagerCellStyle(screen, 1, 1)
	if got, want := promptCellStyle.GetForeground(), promptStyle.GetForeground(); got != want {
		t.Fatalf("prompt fg = %v, want %v", got, want)
	}
	if got, want := promptCellStyle.GetBackground(), promptStyle.GetBackground(); got != want {
		t.Fatalf("prompt bg = %v, want %v", got, want)
	}
	if !promptCellStyle.HasBold() {
		t.Fatal("prompt style lost bold attribute")
	}
	if got := pagerRowString(screen, 1, 20); strings.Contains(got, "regex:error") {
		t.Fatalf("prompt row = %q, want custom PromptLine to own error rendering", got)
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

func TestPagerSetThemeAffectsSubsequentDraw(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap})
	pager.SetSize(20, 2)
	if err := pager.AppendString("plain\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()
	pager.SetTheme(Theme{
		DefaultFG: tcolor.Green,
		DefaultBG: tcolor.Navy,
	})

	screen := newPagerMockScreen(t, 20, 2)
	defer screen.Fini()

	pager.Draw(screen)
	_, style, _ := screen.Get(0, 0)
	if got, want := style.GetForeground(), tcolor.Green; got != want {
		t.Fatalf("plain fg after SetTheme = %v, want %v", got, want)
	}
	if got, want := style.GetBackground(), tcolor.Navy; got != want {
		t.Fatalf("plain bg after SetTheme = %v, want %v", got, want)
	}
}

func TestPagerSetChromeAffectsSubsequentDraw(t *testing.T) {
	borderStyle := tcell.StyleDefault.Foreground(tcolor.Blue)
	titleStyle := tcell.StyleDefault.Foreground(tcolor.Fuchsia).Bold(true)
	statusStyle := tcell.StyleDefault.Foreground(tcolor.White).Background(tcolor.Navy)
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 3)
	if err := pager.AppendString("plain\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()
	pager.SetChrome(Chrome{
		Title:       "Demo",
		TitleAlign:  TitleAlignCenter,
		Frame:       RoundedFrame(),
		BorderStyle: borderStyle,
		TitleStyle:  titleStyle,
		StatusStyle: statusStyle,
	})

	screen := newPagerMockScreen(t, 20, 3)
	defer screen.Fini()

	pager.Draw(screen)
	if got := pagerCellRune(screen, 0, 0); got != '╭' {
		t.Fatalf("top-left frame rune after SetChrome = %q, want %q", got, '╭')
	}
	if got, want := pagerCellStyle(screen, 0, 0).GetForeground(), borderStyle.GetForeground(); got != want {
		t.Fatalf("border fg after SetChrome = %v, want %v", got, want)
	}
	titleX := pagerFindRune(screen, 0, 'D', 20)
	if titleX < 0 {
		t.Fatal("title rune after SetChrome not found")
	}
	if got, want := pagerCellStyle(screen, titleX, 0).GetForeground(), titleStyle.GetForeground(); got != want {
		t.Fatalf("title fg after SetChrome = %v, want %v", got, want)
	}
	if got, want := pagerCellStyle(screen, 0, 2).GetBackground(), statusStyle.GetBackground(); got != want {
		t.Fatalf("status bg after SetChrome = %v, want %v", got, want)
	}
}

func TestPagerVisualizationMarkers(t *testing.T) {
	pager := New(Config{
		TabWidth: 4,
		WrapMode: NoWrap,
		Visualization: Visualization{
			ShowTabs:            true,
			ShowNewlines:        true,
			ShowCarriageReturns: true,
			ShowEOF:             true,
			TabGlyph:            ">",
			NewlineGlyph:        "N",
			CarriageReturnGlyph: "R",
			EOFGlyph:            "E",
		},
	})
	pager.SetSize(20, 1)
	if err := pager.AppendString("a\tb\r\nc\nlast"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	screen := newPagerMockScreen(t, 20, 1)
	defer screen.Fini()

	pager.Draw(screen)
	if got, want := pagerRowString(screen, 0, 20), "a>  bRN             "; got != want {
		t.Fatalf("row 0 = %q, want %q", got, want)
	}

	pager.ScrollDown(1)
	screen.Clear()
	pager.Draw(screen)
	if got, want := pagerRowString(screen, 0, 20), "cN                  "; got != want {
		t.Fatalf("row 1 = %q, want %q", got, want)
	}

	pager.ScrollDown(1)
	screen.Clear()
	pager.Draw(screen)
	if got, want := pagerRowString(screen, 0, 20), "lastE               "; got != want {
		t.Fatalf("row 2 = %q, want %q", got, want)
	}
}

func TestPagerSetVisualizationAffectsSubsequentDraw(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap})
	pager.SetSize(20, 1)
	if err := pager.AppendString("a\tb\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()
	pager.SetVisualization(Visualization{
		ShowTabs:     true,
		ShowNewlines: true,
		ShowEOF:      true,
		TabGlyph:     ">",
		NewlineGlyph: "N",
		EOFGlyph:     "E",
	})

	screen := newPagerMockScreen(t, 20, 1)
	defer screen.Fini()

	pager.Draw(screen)
	if got, want := pagerRowString(screen, 0, 20), "a>  bNE             "; got != want {
		t.Fatalf("row after SetVisualization = %q, want %q", got, want)
	}
}

func TestPagerVisualizationMarkersRemainReachableAtLineEnd(t *testing.T) {
	pager := New(Config{
		TabWidth: 4,
		WrapMode: NoWrap,
		Visualization: Visualization{
			ShowNewlines: true,
			NewlineGlyph: "N",
		},
	})
	pager.SetSize(4, 1)
	if err := pager.AppendString("abcd\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	screen := newPagerMockScreen(t, 4, 1)
	defer screen.Fini()

	pager.Draw(screen)
	if got, want := pagerRowString(screen, 0, 4), "abcd"; got != want {
		t.Fatalf("row before horizontal scroll = %q, want %q", got, want)
	}

	pager.ScrollRight(1)
	screen.Clear()
	pager.Draw(screen)
	if got, want := pagerRowString(screen, 0, 4), "bcdN"; got != want {
		t.Fatalf("row after horizontal scroll = %q, want %q", got, want)
	}
}

func TestPagerSetVisualizationReclampsHorizontalScroll(t *testing.T) {
	pager := New(Config{
		TabWidth: 4,
		WrapMode: NoWrap,
		Visualization: Visualization{
			ShowNewlines: true,
			NewlineGlyph: "N",
		},
	})
	pager.SetSize(4, 1)
	if err := pager.AppendString("abcd\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()
	pager.ScrollRight(1)

	screen := newPagerMockScreen(t, 4, 1)
	defer screen.Fini()

	pager.Draw(screen)
	if got, want := pagerRowString(screen, 0, 4), "bcdN"; got != want {
		t.Fatalf("row before disabling visualization = %q, want %q", got, want)
	}

	pager.SetVisualization(Visualization{})
	screen.Clear()
	pager.Draw(screen)
	if got, want := pagerRowString(screen, 0, 4), "abcd"; got != want {
		t.Fatalf("row after disabling visualization = %q, want %q", got, want)
	}
}

func TestPagerRendersStrikethroughStyle(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap})
	pager.SetSize(8, 1)
	if err := pager.AppendString("\x1b[9mX\x1b[29m\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	screen := newPagerMockScreen(t, 8, 1)
	defer screen.Fini()

	pager.Draw(screen)
	if !pagerCellStyle(screen, 0, 0).HasStrikeThrough() {
		t.Fatal("style at rendered strikethrough cell = not strike-through")
	}
}

func TestPagerDoesNotRenderOSC8HyperlinkWithoutHandler(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, RenderMode: RenderPresentation})
	pager.SetSize(8, 1)
	if err := pager.AppendString("x\x1b]8;id=demo;https://example.com\aY\x1b]8;;\aZ"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	screen := newPagerMockScreen(t, 8, 1)
	defer screen.Fini()

	pager.Draw(screen)

	if got, want := pagerRowString(screen, 0, 3), "xYZ"; got != want {
		t.Fatalf("rendered text = %q, want %q", got, want)
	}
	if id, url := pagerCellStyle(screen, 2, 0).GetUrl(); id != "" || url != "" {
		t.Fatalf("trailing cell hyperlink = (%q, %q), want empty", id, url)
	}
	if id, url := pagerCellStyle(screen, 1, 0).GetUrl(); id != "" || url != "" {
		t.Fatalf("hyperlink unexpectedly live = (%q, %q)", id, url)
	}
}

func TestPagerDoesNotRenderOSC8EscapeSequenceInHybridWithoutHandler(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, RenderMode: RenderHybrid})
	pager.SetSize(64, 1)
	if err := pager.AppendString("See \x1b]8;id=sample;https://github.com/gdamore/goless\agoless on GitHub\x1b]8;;\a for project updates."); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	screen := newPagerMockScreen(t, 64, 1)
	defer screen.Fini()

	pager.Draw(screen)

	if got, want := pagerRowString(screen, 0, 41), "See goless on GitHub for project updates."; got != want {
		t.Fatalf("rendered text = %q, want %q", got, want)
	}
	if strings.Contains(pagerRowString(screen, 0, 64), "␛]8") {
		t.Fatal("OSC 8 escape sequence rendered visibly in hybrid mode")
	}
	if id, url := pagerCellStyle(screen, 4, 0).GetUrl(); id != "" || url != "" {
		t.Fatalf("link unexpectedly live in default hybrid mode = (%q, %q)", id, url)
	}
}

func TestPagerRendersOSC8HyperlinkWithHandler(t *testing.T) {
	pager := New(Config{
		TabWidth:   4,
		WrapMode:   NoWrap,
		RenderMode: RenderHybrid,
		HyperlinkHandler: func(info HyperlinkInfo) HyperlinkDecision {
			if info.Target != "https://example.com" || info.Text != "Y" {
				t.Fatalf("handler info = %+v", info)
			}
			return HyperlinkDecision{
				Live:   true,
				Target: "https://safe.example.com/path",
			}
		},
	})
	pager.SetSize(8, 1)
	if err := pager.AppendString("x\x1b]8;id=demo;https://example.com\aY\x1b]8;;\aZ"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	screen := newPagerMockScreen(t, 8, 1)
	defer screen.Fini()

	pager.Draw(screen)

	if got, want := pagerRowString(screen, 0, 3), "xYZ"; got != want {
		t.Fatalf("rendered text = %q, want %q", got, want)
	}

	id, url := pagerCellStyle(screen, 1, 0).GetUrl()
	if got, want := url, "https://safe.example.com/path"; got != want {
		t.Fatalf("hyperlink url = %q, want %q", got, want)
	}
	if got, want := id, "demo"; got != want {
		t.Fatalf("hyperlink id = %q, want %q", got, want)
	}
}

func TestPagerRendersOSC8HyperlinkStyleOverride(t *testing.T) {
	pager := New(Config{
		TabWidth:   4,
		WrapMode:   NoWrap,
		RenderMode: RenderPresentation,
		HyperlinkHandler: func(info HyperlinkInfo) HyperlinkDecision {
			if info.Text != "demo" {
				t.Fatalf("handler text = %q, want %q", info.Text, "demo")
			}
			return HyperlinkDecision{
				Style: info.Style.
					Foreground(tcolor.Blue).
					Underline(tcell.UnderlineStyleSolid),
				StyleSet: true,
			}
		},
	})
	pager.SetSize(4, 1)
	if err := pager.AppendString("\x1b]8;;https://example.com\ademo\x1b]8;;\a"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	screen := newPagerMockScreen(t, 4, 1)
	defer screen.Fini()

	pager.Draw(screen)

	if got, want := pagerRowString(screen, 0, 4), "demo"; got != want {
		t.Fatalf("rendered text = %q, want %q", got, want)
	}
	if fg := pagerCellStyle(screen, 0, 0).GetForeground(); fg != tcolor.Blue {
		t.Fatalf("foreground = %v, want %v", fg, tcolor.Blue)
	}
	if !pagerCellStyle(screen, 0, 0).HasUnderline() {
		t.Fatal("style override did not enable underline")
	}
}

func TestPagerOSC8StyleOverrideReplacesBaseStyle(t *testing.T) {
	pager := New(Config{
		TabWidth:   4,
		WrapMode:   NoWrap,
		RenderMode: RenderPresentation,
		HyperlinkHandler: func(info HyperlinkInfo) HyperlinkDecision {
			return HyperlinkDecision{
				Style:    tcell.StyleDefault.Foreground(tcolor.Blue),
				StyleSet: true,
			}
		},
	})
	pager.SetSize(1, 1)
	if err := pager.AppendString("\x1b[7m\x1b]8;;https://example.com\ad\x1b]8;;\a"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	screen := newPagerMockScreen(t, 1, 1)
	defer screen.Fini()

	pager.Draw(screen)

	style := pagerCellStyle(screen, 0, 0)
	if fg := style.GetForeground(); fg != tcolor.Blue {
		t.Fatalf("foreground = %v, want %v", fg, tcolor.Blue)
	}
	if style.GetBackground() != tcolor.Default {
		t.Fatalf("background = %v, want default", style.GetBackground())
	}
	if style.GetAttributes() != 0 {
		t.Fatalf("attributes = %v, want 0", style.GetAttributes())
	}
	if style.HasUnderline() {
		t.Fatal("underline unexpectedly preserved from base style")
	}
}

func TestPagerRendersUnderlineVariantAndColor(t *testing.T) {
	pager := New(Config{
		TabWidth:   4,
		WrapMode:   NoWrap,
		RenderMode: RenderPresentation,
	})
	pager.SetSize(1, 1)
	if err := pager.AppendString("\x1b[4:3;58;2;1;2;3mA\x1b[0m"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	screen := newPagerMockScreen(t, 1, 1)
	defer screen.Fini()

	pager.Draw(screen)

	style := pagerCellStyle(screen, 0, 0)
	if got, want := style.GetUnderlineStyle(), tcell.UnderlineStyleCurly; got != want {
		t.Fatalf("underline style = %v, want %v", got, want)
	}
	if got, want := style.GetUnderlineColor(), tcolor.NewRGBColor(1, 2, 3); got != want {
		t.Fatalf("underline color = %v, want %v", got, want)
	}
}

func TestPagerVisualizationWideMarkerClipsAfterHorizontalScroll(t *testing.T) {
	pager := New(Config{
		TabWidth: 4,
		WrapMode: NoWrap,
		Visualization: Visualization{
			ShowNewlines: true,
			ShowEOF:      true,
			NewlineGlyph: "界",
			EOFGlyph:     "E",
		},
	})
	pager.SetSize(2, 1)
	if err := pager.AppendString("a\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()
	pager.ScrollRight(2)

	screen := newPagerMockScreen(t, 2, 1)
	defer screen.Fini()

	pager.Draw(screen)
	if got, want := pagerRowString(screen, 0, 2), " E"; got != want {
		t.Fatalf("row after horizontal scroll = %q, want %q", got, want)
	}
}

func TestPagerVisualizationWideMarkerBoundaryScrollPreservesNextMarker(t *testing.T) {
	pager := New(Config{
		TabWidth: 4,
		WrapMode: NoWrap,
		Visualization: Visualization{
			ShowNewlines: true,
			ShowEOF:      true,
			NewlineGlyph: "界",
			EOFGlyph:     "E",
		},
	})
	pager.SetSize(1, 1)
	if err := pager.AppendString("a\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()
	pager.ScrollRight(3)

	screen := newPagerMockScreen(t, 1, 1)
	defer screen.Fini()

	pager.Draw(screen)
	if got, want := pagerRowString(screen, 0, 1), "E"; got != want {
		t.Fatalf("row after boundary scroll = %q, want %q", got, want)
	}
}

func TestPagerVisualizationStyleDefaultCanBeExplicit(t *testing.T) {
	pager := New(Config{
		TabWidth: 4,
		WrapMode: NoWrap,
		Visualization: Visualization{
			ShowTabs: true,
			TabGlyph: ">",
			Style:    tcell.StyleDefault,
			StyleSet: true,
		},
	})
	pager.SetSize(8, 1)
	if err := pager.AppendString("a\t"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	screen := newPagerMockScreen(t, 8, 1)
	defer screen.Fini()

	pager.Draw(screen)

	style := pagerCellStyle(screen, 1, 0)
	if got, want := style.GetForeground(), tcolor.Default; got != want {
		t.Fatalf("marker fg = %v, want %v", got, want)
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

func TestPagerEmptySearchPromptClearsSearchQuietly(t *testing.T) {
	pager := New(Config{TabWidth: 4, WrapMode: NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\nbeta\nalpha\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if !pager.SearchForward("alpha") {
		t.Fatal("SearchForward(alpha) = false, want true")
	}
	if got, want := pager.SearchState().Query, "alpha"; got != want {
		t.Fatalf("SearchState().Query after search = %q, want %q", got, want)
	}

	if result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "/", tcell.ModNone)); !result.Handled {
		t.Fatal("HandleKeyResult(/).Handled = false, want true")
	}
	if result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)); !result.Handled {
		t.Fatal("HandleKeyResult(Enter).Handled = false, want true")
	}

	state := pager.SearchState()
	if got, want := state.Query, ""; got != want {
		t.Fatalf("SearchState().Query after empty prompt = %q, want empty", got)
	}
	if got, want := state.MatchCount, 0; got != want {
		t.Fatalf("SearchState().MatchCount after empty prompt = %d, want %d", got, want)
	}

	screen := newPagerMockScreen(t, 30, 2)
	defer screen.Fini()
	pager.Draw(screen)
	if got := pagerRowString(screen, 1, 30); strings.Contains(got, "empty search") {
		t.Fatalf("status line = %q, want quiet clear", got)
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

func pagerCellStyle(screen tcell.Screen, x, y int) tcell.Style {
	_, style, _ := screen.Get(x, y)
	return style
}

func pagerRowString(screen tcell.Screen, y, width int) string {
	var out strings.Builder
	for x := 0; x < width; x++ {
		str, _, _ := screen.Get(x, y)
		if str == "" {
			out.WriteRune(' ')
			continue
		}
		out.WriteString(str)
	}
	return out.String()
}

func pagerFindRune(screen tcell.Screen, y int, want rune, width int) int {
	for x := 0; x < width; x++ {
		if pagerCellRune(screen, x, y) == want {
			return x
		}
	}
	return -1
}

func newPagerMockScreen(t *testing.T, width, height int) tcell.Screen {
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
	return screen
}
