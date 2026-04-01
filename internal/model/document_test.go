// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"testing"

	"github.com/gdamore/goless/internal/ansi"
)

func TestDocumentBuildsLinesAcrossAppends(t *testing.T) {
	doc := NewDocument(4)

	if err := doc.Append([]byte("hello\nwo")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	if err := doc.Append([]byte("rld")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	lines := doc.Lines()
	if got, want := len(lines), 2; got != want {
		t.Fatalf("line count = %d, want %d", got, want)
	}
	if got, want := lines[0].Text, "hello"; got != want {
		t.Fatalf("line 0 = %q, want %q", got, want)
	}
	if got, want := lines[1].Text, "world"; got != want {
		t.Fatalf("line 1 = %q, want %q", got, want)
	}
}

func TestDocumentTracksStyleRuns(t *testing.T) {
	doc := NewDocument(4)

	if err := doc.Append([]byte("a\x1b[31mb")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	lines := doc.Lines()
	if got, want := len(lines), 1; got != want {
		t.Fatalf("line count = %d, want %d", got, want)
	}
	if got, want := len(lines[0].Styles), 2; got != want {
		t.Fatalf("style run count = %d, want %d", got, want)
	}
	if got, want := lines[0].Styles[0].Style, ansi.DefaultStyle(); got != want {
		t.Fatalf("style 0 = %+v, want %+v", got, want)
	}
	if got, want := lines[0].Styles[1].Style.Fg, ansi.IndexedColor(1); got != want {
		t.Fatalf("style 1 fg = %+v, want %+v", got, want)
	}
}

func TestDocumentIndexesCombiningGraphemes(t *testing.T) {
	doc := NewDocument(4)

	if err := doc.Append([]byte("e\u0301x")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	lines := doc.Lines()
	if got, want := len(lines), 1; got != want {
		t.Fatalf("line count = %d, want %d", got, want)
	}
	if got, want := len(lines[0].Graphemes), 2; got != want {
		t.Fatalf("grapheme count = %d, want %d", got, want)
	}

	first := lines[0].Graphemes[0]
	if got, want := first.Text, "e\u0301"; got != want {
		t.Fatalf("first grapheme = %q, want %q", got, want)
	}
	if got, want := first.RuneStart, 0; got != want {
		t.Fatalf("first rune start = %d, want %d", got, want)
	}
	if got, want := first.RuneEnd, 2; got != want {
		t.Fatalf("first rune end = %d, want %d", got, want)
	}
	if got, want := first.CellWidth, 1; got != want {
		t.Fatalf("first width = %d, want %d", got, want)
	}
}

func TestDocumentIndexesEmojiZWJGraphemes(t *testing.T) {
	doc := NewDocument(4)

	if err := doc.Append([]byte("👨‍👩‍👧‍👦!")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	lines := doc.Lines()
	if got, want := len(lines), 1; got != want {
		t.Fatalf("line count = %d, want %d", got, want)
	}
	if got, want := len(lines[0].Graphemes), 2; got != want {
		t.Fatalf("grapheme count = %d, want %d", got, want)
	}

	first := lines[0].Graphemes[0]
	if got, want := first.Text, "👨‍👩‍👧‍👦"; got != want {
		t.Fatalf("first grapheme = %q, want %q", got, want)
	}
	if got, want := first.CellWidth, 2; got != want {
		t.Fatalf("first width = %d, want %d", got, want)
	}

	second := lines[0].Graphemes[1]
	if got, want := second.Text, "!"; got != want {
		t.Fatalf("second grapheme = %q, want %q", got, want)
	}
}

func TestDocumentSuppressesVTAndFFInDefaultRenderMode(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "vt", text: "\vfoo", want: "foo"},
		{name: "ff", text: "\ffoo", want: "foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := NewDocument(4)
			if err := doc.Append([]byte(tt.text)); err != nil {
				t.Fatalf("Append failed: %v", err)
			}

			lines := doc.Lines()
			if got, want := len(lines), 1; got != want {
				t.Fatalf("line count = %d, want %d", got, want)
			}
			if got := lines[0].Text; got != tt.want {
				t.Fatalf("line text = %q, want %q", got, tt.want)
			}
		})
	}
}
