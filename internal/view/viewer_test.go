// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"testing"

	"github.com/gdamore/goless/internal/layout"
	"github.com/gdamore/goless/internal/model"
	"github.com/gdamore/tcell/v3"
	tcolor "github.com/gdamore/tcell/v3/color"
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

func keyRune(s string) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, s, tcell.ModNone)
}

func keyKey(k tcell.Key) *tcell.EventKey {
	return tcell.NewEventKey(k, "", tcell.ModNone)
}
