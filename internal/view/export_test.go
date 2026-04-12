// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/gdamore/goless/internal/ansi"
	"github.com/gdamore/goless/internal/layout"
	"github.com/gdamore/goless/internal/model"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

func stripTestANSI(text string) string {
	return ansiEscapePattern.ReplaceAllString(text, "")
}

func newExportTestViewer(t *testing.T, text string, cfg Config, width, height int) *Viewer {
	t.Helper()

	doc := model.NewDocument(4)
	if err := doc.Append([]byte(text)); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, cfg)
	v.SetSize(width, height)
	return v
}

func TestViewerExportCurrentContentPreservesLineEndingsAndANSI(t *testing.T) {
	v := newExportTestViewer(t, "\x1b[1;31mred\x1b[0m\r\nplain", Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
	}, 20, 3)

	plain, err := v.Export(ExportOptions{
		Scope:  ExportScopeContent,
		Format: ExportFormatPlain,
	})
	if err != nil {
		t.Fatalf("Export(plain) failed: %v", err)
	}
	if got, want := string(plain), "red\r\nplain"; got != want {
		t.Fatalf("Export(plain) = %q, want %q", got, want)
	}

	colored, err := v.Export(ExportOptions{
		Scope:  ExportScopeContent,
		Format: ExportFormatANSI,
	})
	if err != nil {
		t.Fatalf("Export(ansi) failed: %v", err)
	}
	if !strings.Contains(string(colored), "\x1b[") {
		t.Fatalf("Export(ansi) = %q, want ANSI escapes", string(colored))
	}
	if got, want := stripTestANSI(string(colored)), "red\r\nplain"; got != want {
		t.Fatalf("stripANSI(Export(ansi)) = %q, want %q", got, want)
	}
}

func TestViewerExportViewportSupportsPlainAndANSI(t *testing.T) {
	v := newExportTestViewer(t, "\x1b[31mABCDE12345\x1b[0m\n", Config{
		TabWidth:      4,
		WrapMode:      layout.NoWrap,
		HeaderColumns: 2,
	}, 6, 2)
	v.ScrollRight(2)

	plain, err := v.Export(ExportOptions{
		Scope:  ExportScopeViewport,
		Format: ExportFormatPlain,
	})
	if err != nil {
		t.Fatalf("Export(viewport plain) failed: %v", err)
	}
	if got, want := string(plain), "ABE123"; got != want {
		t.Fatalf("Export(viewport plain) = %q, want %q", got, want)
	}

	colored, err := v.Export(ExportOptions{
		Scope:  ExportScopeViewport,
		Format: ExportFormatANSI,
	})
	if err != nil {
		t.Fatalf("Export(viewport ansi) failed: %v", err)
	}
	if !strings.Contains(string(colored), "\x1b[") {
		t.Fatalf("Export(viewport ansi) = %q, want ANSI escapes", string(colored))
	}
	if got, want := stripTestANSI(string(colored)), "ABE123"; got != want {
		t.Fatalf("stripANSI(Export(viewport ansi)) = %q, want %q", got, want)
	}
}

func TestExportStyleHelpers(t *testing.T) {
	style := ansi.Style{
		Bold:           true,
		Italic:         true,
		Underline:      ansi.UnderlineStyleDouble,
		Strike:         true,
		Fg:             ansi.Color{Kind: ansi.ColorIndex, Index: 9},
		Bg:             ansi.Color{Kind: ansi.ColorRGB, R: 1, G: 2, B: 3},
		UnderlineColor: ansi.Color{Kind: ansi.ColorRGB, R: 4, G: 5, B: 6},
		URL:            "https://example.com",
		URLID:          "link",
	}

	if got, want := ansiStyleSequence(style), "\x1b[0;1;3;4:2;9;91;48;2;1;2;3;58;2;4;5;6m"; got != want {
		t.Fatalf("ansiStyleSequence(style) = %q, want %q", got, want)
	}
	if got := ansiStyleSequence(ansi.DefaultStyle()); got != "\x1b[0m" {
		t.Fatalf("ansiStyleSequence(default) = %q, want reset", got)
	}

	sanitized := sanitizeExportStyle(style)
	if sanitized.URL != "" || sanitized.URLID != "" {
		t.Fatalf("sanitizeExportStyle(...) = %+v, want empty URL fields", sanitized)
	}
	if exportStyleIsDefault(style) {
		t.Fatal("exportStyleIsDefault(style) = true, want false")
	}
	if !exportStyleIsDefault(ansi.DefaultStyle()) {
		t.Fatal("exportStyleIsDefault(default) = false, want true")
	}
	if !exportStylesEqual(style, sanitized) {
		t.Fatal("exportStylesEqual(style, sanitized) = false, want true")
	}
	if exportStylesEqual(style, ansi.DefaultStyle()) {
		t.Fatal("exportStylesEqual(style, default) = true, want false")
	}

	if got, want := ansiColorCodes(ansi.Color{Kind: ansi.ColorIndex, Index: 3}, false), []string{"33"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ansiColorCodes(index<8) = %v, want %v", got, want)
	}
	if got, want := ansiColorCodes(ansi.Color{Kind: ansi.ColorIndex, Index: 10}, true), []string{"102"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ansiColorCodes(index<16 background) = %v, want %v", got, want)
	}
	if got, want := ansiColorCodes(ansi.Color{Kind: ansi.ColorRGB, R: 7, G: 8, B: 9}, false), []string{"38", "2", "7", "8", "9"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ansiColorCodes(rgb) = %v, want %v", got, want)
	}
	if got, want := ansiUnderlineColorCodes(ansi.Color{Kind: ansi.ColorIndex, Index: 12}), []string{"58", "5", "12"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ansiUnderlineColorCodes(index) = %v, want %v", got, want)
	}
	if got := ansiUnderlineColorCodes(ansi.Color{}); got != nil {
		t.Fatalf("ansiUnderlineColorCodes(default) = %v, want nil", got)
	}
}

func TestExportHelperFunctions(t *testing.T) {
	if got, want := runeByteOffsets("a界b"), []int{0, 1, 4, 5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("runeByteOffsets = %v, want %v", got, want)
	}
	if got, want := lineEndingString(model.LineEndingCRLF), "\r\n"; got != want {
		t.Fatalf("lineEndingString(CRLF) = %q, want %q", got, want)
	}
	if got, want := lineEndingString(model.LineEndingVT), "\v"; got != want {
		t.Fatalf("lineEndingString(VT) = %q, want %q", got, want)
	}
	if got, want := lineEndingString(model.LineEndingFF), "\f"; got != want {
		t.Fatalf("lineEndingString(FF) = %q, want %q", got, want)
	}
	if got, want := lineEndingString(model.LineEndingLF), "\n"; got != want {
		t.Fatalf("lineEndingString(LF) = %q, want %q", got, want)
	}
	if got := lineEndingString(model.LineEndingNone); got != "" {
		t.Fatalf("lineEndingString(None) = %q, want empty", got)
	}
	if got, want := clamp(-1, 0, 2), 0; got != want {
		t.Fatalf("clamp(low) = %d, want %d", got, want)
	}
	if got, want := clamp(3, 0, 2), 2; got != want {
		t.Fatalf("clamp(high) = %d, want %d", got, want)
	}
	if got, want := clamp(1, 0, 2), 1; got != want {
		t.Fatalf("clamp(mid) = %d, want %d", got, want)
	}
	if got, want := itoa(42), "42"; got != want {
		t.Fatalf("itoa(42) = %q, want %q", got, want)
	}
}
