// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
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
