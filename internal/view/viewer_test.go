package view

import (
	"testing"

	"github.com/gdamore/goless/internal/layout"
	"github.com/gdamore/goless/internal/model"
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
