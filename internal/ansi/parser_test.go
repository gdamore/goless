// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package ansi

import (
	"strings"
	"testing"
)

type recordEvent struct {
	kind   string
	r      rune
	style  Style
	offset int64
}

type recordReceiver struct {
	events []recordEvent
}

func (r *recordReceiver) Print(ch rune, style Style, offset int64) {
	r.events = append(r.events, recordEvent{kind: "print", r: ch, style: style, offset: offset})
}

func (r *recordReceiver) Newline(style Style, offset int64) {
	r.events = append(r.events, recordEvent{kind: "newline", style: style, offset: offset})
}

func TestParserCarriesUTF8AcrossWrites(t *testing.T) {
	recv := &recordReceiver{}
	p := NewParser(recv)

	if _, err := p.Write([]byte{0xf0, 0x9f}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if len(recv.events) != 0 {
		t.Fatalf("unexpected events before UTF-8 sequence completed: %+v", recv.events)
	}

	if _, err := p.Write([]byte{0x98, 0x80}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if got, want := len(recv.events), 1; got != want {
		t.Fatalf("event count = %d, want %d", got, want)
	}
	if got, want := recv.events[0].r, '😀'; got != want {
		t.Fatalf("rune = %q, want %q", got, want)
	}
}

func TestParserAppliesSGRAndShowsOSC(t *testing.T) {
	recv := &recordReceiver{}
	p := NewParser(recv)

	input := "a\x1b[31mB\x1b[0m\x1b]0;title\aC"
	if _, err := p.Write([]byte(input)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	var text strings.Builder
	for _, ev := range recv.events {
		if ev.kind == "print" {
			text.WriteRune(ev.r)
		}
	}
	if got, want := text.String(), "aB␛]0;title␇C"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}

	if got := recv.events[1].style.Fg; got != IndexedColor(1) {
		t.Fatalf("foreground = %+v, want %+v", got, IndexedColor(1))
	}
	if got := recv.events[len(recv.events)-1].style.Fg; got != DefaultColor() {
		t.Fatalf("reset foreground = %+v, want %+v", got, DefaultColor())
	}
}

func TestParserAppliesAndClearsStrikethrough(t *testing.T) {
	recv := &recordReceiver{}
	p := NewParser(recv)

	input := "a\x1b[9mB\x1b[29mC"
	if _, err := p.Write([]byte(input)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if got, want := len(recv.events), 3; got != want {
		t.Fatalf("event count = %d, want %d", got, want)
	}
	if !recv.events[1].style.Strike {
		t.Fatal("strikethrough for B = false, want true")
	}
	if recv.events[2].style.Strike {
		t.Fatal("strikethrough for C = true, want false")
	}
}

func TestParserLiteralShowsSGRWithoutStyling(t *testing.T) {
	recv := &recordReceiver{}
	p := NewParserWithMode(recv, RenderLiteral)

	input := "a\x1b[31mB\x1b[0mC"
	if _, err := p.Write([]byte(input)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	var text strings.Builder
	for _, ev := range recv.events {
		if ev.kind == "print" {
			text.WriteRune(ev.r)
		}
	}
	if got, want := text.String(), "a␛[31mB␛[0mC"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}

	foundB := false
	for _, ev := range recv.events {
		if ev.kind == "print" && ev.r == 'B' {
			foundB = true
			if got, want := ev.style, DefaultStyle(); got != want {
				t.Fatalf("style = %+v, want %+v", got, want)
			}
		}
	}
	if !foundB {
		t.Fatalf("did not observe print event for B")
	}
}

func TestParserPresentationHidesOSC(t *testing.T) {
	recv := &recordReceiver{}
	p := NewParserWithMode(recv, RenderPresentation)

	input := "a\x1b[31mB\x1b[0m\x1b]0;title\aC"
	if _, err := p.Write([]byte(input)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	var text strings.Builder
	for _, ev := range recv.events {
		if ev.kind == "print" {
			text.WriteRune(ev.r)
		}
	}
	if got, want := text.String(), "aBC"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}

	if got := recv.events[1].style.Fg; got != IndexedColor(1) {
		t.Fatalf("foreground = %+v, want %+v", got, IndexedColor(1))
	}
	if got := recv.events[len(recv.events)-1].style.Fg; got != DefaultColor() {
		t.Fatalf("reset foreground = %+v, want %+v", got, DefaultColor())
	}
}

func TestParserPresentationAppliesOSC8Hyperlink(t *testing.T) {
	recv := &recordReceiver{}
	p := NewParserWithMode(recv, RenderPresentation)

	input := "x\x1b]8;id=link-1;https://example.com\aY\x1b]8;;\aZ"
	if _, err := p.Write([]byte(input)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	var text strings.Builder
	for _, ev := range recv.events {
		if ev.kind == "print" {
			text.WriteRune(ev.r)
		}
	}
	if got, want := text.String(), "xYZ"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}

	if got, want := recv.events[1].style.URL, "https://example.com"; got != want {
		t.Fatalf("Y url = %q, want %q", got, want)
	}
	if got, want := recv.events[1].style.URLID, "link-1"; got != want {
		t.Fatalf("Y url id = %q, want %q", got, want)
	}
	if got := recv.events[2].style.URL; got != "" {
		t.Fatalf("Z url = %q, want empty", got)
	}
}

func TestParserHybridAppliesOSC8Hyperlink(t *testing.T) {
	recv := &recordReceiver{}
	p := NewParserWithMode(recv, RenderHybrid)

	input := "x\x1b]8;id=link-1;https://example.com\aY\x1b]8;;\aZ"
	if _, err := p.Write([]byte(input)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	var text strings.Builder
	for _, ev := range recv.events {
		if ev.kind == "print" {
			text.WriteRune(ev.r)
		}
	}
	if got, want := text.String(), "xYZ"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}

	if got, want := recv.events[1].style.URL, "https://example.com"; got != want {
		t.Fatalf("Y url = %q, want %q", got, want)
	}
	if got, want := recv.events[1].style.URLID, "link-1"; got != want {
		t.Fatalf("Y url id = %q, want %q", got, want)
	}
	if got := recv.events[2].style.URL; got != "" {
		t.Fatalf("Z url = %q, want empty", got)
	}
}

func TestParserShowsOSCTerminatedByST(t *testing.T) {
	recv := &recordReceiver{}
	p := NewParser(recv)

	input := "x\x1b]0;title\x1b\\y"
	if _, err := p.Write([]byte(input)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	var text strings.Builder
	for _, ev := range recv.events {
		if ev.kind == "print" {
			text.WriteRune(ev.r)
		}
	}
	if got, want := text.String(), "x␛]0;title␛\\y"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

func TestParserShowsOSCTerminatedByC1ST(t *testing.T) {
	recv := &recordReceiver{}
	p := NewParser(recv)

	input := []byte{'x', 0x1b, ']', '0', ';', 't', 'i', 't', 'l', 'e', 0x9c, 'y'}
	if _, err := p.Write(input); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	var text strings.Builder
	for _, ev := range recv.events {
		if ev.kind == "print" {
			text.WriteRune(ev.r)
		}
	}
	if got, want := text.String(), "x␛]0;title␛\\y"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

func TestParserFlushesIncompleteEscapeVisibly(t *testing.T) {
	recv := &recordReceiver{}
	p := NewParser(recv)

	if _, err := p.Write([]byte("x\x1b[31")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	p.Flush()

	var text strings.Builder
	for _, ev := range recv.events {
		if ev.kind == "print" {
			text.WriteRune(ev.r)
		}
	}

	if got, want := text.String(), "x␛[31"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

func TestParserNormalizesCRLF(t *testing.T) {
	recv := &recordReceiver{}
	p := NewParser(recv)

	if _, err := p.Write([]byte("a\r\nb")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if got, want := len(recv.events), 3; got != want {
		t.Fatalf("event count = %d, want %d", got, want)
	}
	if recv.events[1].kind != "newline" {
		t.Fatalf("second event = %q, want newline", recv.events[1].kind)
	}
}
