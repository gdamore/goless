// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/gdamore/goless/internal/model"
)

func TestSoftWrapBuildsVisualRows(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdef")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	result := Build(doc.Lines(), Config{Width: 3, TabWidth: 4, WrapMode: SoftWrap})
	if got, want := len(result.Rows), 2; got != want {
		t.Fatalf("row count = %d, want %d", got, want)
	}

	row0 := result.Rows[0]
	if got, want := row0.FirstLogicalGrapheme(), 0; got != want {
		t.Fatalf("row0 grapheme start = %d, want %d", got, want)
	}
	if got, want := row0.LastLogicalGrapheme(), 3; got != want {
		t.Fatalf("row0 grapheme end = %d, want %d", got, want)
	}
	if got, want := len(row0.Segments), 3; got != want {
		t.Fatalf("row0 segment count = %d, want %d", got, want)
	}

	row1 := result.Rows[1]
	if got, want := row1.FirstLogicalGrapheme(), 3; got != want {
		t.Fatalf("row1 grapheme start = %d, want %d", got, want)
	}
	if got, want := row1.LastLogicalGrapheme(), 6; got != want {
		t.Fatalf("row1 grapheme end = %d, want %d", got, want)
	}
}

func TestSoftWrapUsesTabExpansion(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("a\tb")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	result := Build(doc.Lines(), Config{Width: 4, TabWidth: 4, WrapMode: SoftWrap})
	if got, want := len(result.Rows), 2; got != want {
		t.Fatalf("row count = %d, want %d", got, want)
	}

	row0 := result.Rows[0]
	if got, want := row0.LastLogicalGrapheme(), 2; got != want {
		t.Fatalf("row0 grapheme end = %d, want %d", got, want)
	}
	if got, want := row0.RenderedCellWidth, 4; got != want {
		t.Fatalf("row0 rendered width = %d, want %d", got, want)
	}

	row1 := result.Rows[1]
	if got, want := row1.FirstLogicalGrapheme(), 2; got != want {
		t.Fatalf("row1 grapheme start = %d, want %d", got, want)
	}
}

func TestNoWrapSnapsHorizontalOffsetToGraphemeBoundary(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("界ab")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	result := Build(doc.Lines(), Config{Width: 3, TabWidth: 4, WrapMode: NoWrap, HorizontalOffset: 1})
	if got, want := len(result.Rows), 1; got != want {
		t.Fatalf("row count = %d, want %d", got, want)
	}

	row := result.Rows[0]
	if got, want := row.SourceCellStart, 1; got != want {
		t.Fatalf("source cell start = %d, want %d", got, want)
	}
	if got, want := row.FirstLogicalGrapheme(), 0; got != want {
		t.Fatalf("grapheme start = %d, want %d", got, want)
	}
	if got, want := len(row.Segments), 3; got != want {
		t.Fatalf("segment count = %d, want %d", got, want)
	}
	if got, want := row.Segments[0].Display, "<"; got != want {
		t.Fatalf("left clip marker = %q, want %q", got, want)
	}
	if got, want := row.LastLogicalGrapheme(), 3; got != want {
		t.Fatalf("logical end = %d, want %d", got, want)
	}
}

func TestWideGraphemeWrapsAsSingleUnit(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("界ab")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	result := Build(doc.Lines(), Config{Width: 2, TabWidth: 4, WrapMode: SoftWrap})
	if got, want := len(result.Rows), 2; got != want {
		t.Fatalf("row count = %d, want %d", got, want)
	}
	if got, want := result.Rows[0].LastLogicalGrapheme(), 1; got != want {
		t.Fatalf("first row grapheme end = %d, want %d", got, want)
	}
	if got, want := result.Rows[0].RenderedCellWidth, 2; got != want {
		t.Fatalf("first row width = %d, want %d", got, want)
	}
}

func TestAnchorMapsAcrossWrapToggle(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("ab界cd")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	scroll := Build(doc.Lines(), Config{
		Width:            3,
		TabWidth:         4,
		WrapMode:         NoWrap,
		HorizontalOffset: 2,
	})
	anchor := scroll.AnchorForRow(0)
	if got, want := anchor.GraphemeIndex, 2; got != want {
		t.Fatalf("anchor grapheme = %d, want %d", got, want)
	}

	wrap := Build(doc.Lines(), Config{Width: 3, TabWidth: 4, WrapMode: SoftWrap})
	if got, want := wrap.RowIndexForAnchor(anchor), 1; got != want {
		t.Fatalf("wrapped row index = %d, want %d", got, want)
	}

	wrappedAnchor := wrap.AnchorForRow(1)
	if got, want := scroll.RowIndexForAnchor(wrappedAnchor), 0; got != want {
		t.Fatalf("scrolled row index = %d, want %d", got, want)
	}
}

func TestEmptyLineProducesRenderableRow(t *testing.T) {
	doc := model.NewDocument(4)

	result := Build(doc.Lines(), Config{Width: 4, TabWidth: 4, WrapMode: SoftWrap})
	if got, want := len(result.Rows), 1; got != want {
		t.Fatalf("row count = %d, want %d", got, want)
	}
	for i, row := range result.Rows {
		if got, want := row.FirstLogicalGrapheme(), 0; got != want {
			t.Fatalf("row %d grapheme start = %d, want %d", i, got, want)
		}
		if got, want := row.LastLogicalGrapheme(), 0; got != want {
			t.Fatalf("row %d grapheme end = %d, want %d", i, got, want)
		}
	}
}

func TestVisualRowStoresSegmentsInVisualOrder(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abc")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	result := Build(doc.Lines(), Config{Width: 3, TabWidth: 4, WrapMode: SoftWrap})
	row := result.Rows[0]
	if got, want := len(row.Segments), 3; got != want {
		t.Fatalf("segment count = %d, want %d", got, want)
	}
	for i, segment := range row.Segments {
		if got, want := segment.LogicalGraphemeIndex, i; got != want {
			t.Fatalf("segment %d logical index = %d, want %d", i, got, want)
		}
	}
}

func TestNoWrapUsesRightClipMarkerForPartialWideGrapheme(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("a界b")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	result := Build(doc.Lines(), Config{Width: 2, TabWidth: 4, WrapMode: NoWrap})
	if got, want := len(result.Rows), 1; got != want {
		t.Fatalf("row count = %d, want %d", got, want)
	}

	row := result.Rows[0]
	if got, want := len(row.Segments), 2; got != want {
		t.Fatalf("segment count = %d, want %d", got, want)
	}
	if got, want := row.Segments[1].Display, ">"; got != want {
		t.Fatalf("right clip marker = %q, want %q", got, want)
	}
	if got, want := row.Segments[1].LogicalGraphemeIndex, 1; got != want {
		t.Fatalf("marker grapheme index = %d, want %d", got, want)
	}
}
