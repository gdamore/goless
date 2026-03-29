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
