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
