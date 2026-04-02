// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import (
	"testing"

	tcolor "github.com/gdamore/tcell/v3/color"
)

func TestDarkPresetUsesSolarizedDarkDefaults(t *testing.T) {
	if got, want := DarkPreset.Theme.DefaultBG, rgb(0x00, 0x2b, 0x36); got != want {
		t.Fatalf("DarkPreset.Theme.DefaultBG = %v, want %v", got, want)
	}
	if got, want := DarkPreset.Theme.DefaultFG, rgb(0x83, 0x94, 0x96); got != want {
		t.Fatalf("DarkPreset.Theme.DefaultFG = %v, want %v", got, want)
	}
	if got, want := DarkPreset.Chrome.TitleAlign, TitleAlignCenter; got != want {
		t.Fatalf("DarkPreset.Chrome.TitleAlign = %v, want %v", got, want)
	}
	if got, want := DarkPreset.Chrome.BorderStyle.GetBackground(), DarkPreset.Theme.DefaultBG; got != want {
		t.Fatalf("DarkPreset.Chrome.BorderStyle background = %v, want %v", got, want)
	}
	if got, want := DarkPreset.Chrome.TitleStyle.GetBackground(), DarkPreset.Theme.DefaultBG; got != want {
		t.Fatalf("DarkPreset.Chrome.TitleStyle background = %v, want %v", got, want)
	}
	if got, want := DarkPreset.Chrome.StatusStyle.GetBackground(), DarkPreset.Theme.DefaultBG; got != want {
		t.Fatalf("DarkPreset.Chrome.StatusStyle background = %v, want %v", got, want)
	}
	if got, want := DarkPreset.Chrome.HeaderStyle.GetBackground(), rgb(0x58, 0x6e, 0x75); got != want {
		t.Fatalf("DarkPreset.Chrome.HeaderStyle background = %v, want %v", got, want)
	}
	if got, want := DarkPreset.Chrome.HeaderStyle.GetForeground(), rgb(0x00, 0x2b, 0x36); got != want {
		t.Fatalf("DarkPreset.Chrome.HeaderStyle foreground = %v, want %v", got, want)
	}
	if !DarkPreset.Chrome.HeaderStyle.HasBold() {
		t.Fatal("DarkPreset.Chrome.HeaderStyle should be bold")
	}
	if got, want := DarkPreset.Chrome.PromptStyle.GetBackground(), DarkPreset.Theme.DefaultBG; got != want {
		t.Fatalf("DarkPreset.Chrome.PromptStyle background = %v, want %v", got, want)
	}
	if got, want := DarkPreset.Chrome.PromptErrorStyle.GetBackground(), DarkPreset.Theme.DefaultBG; got != want {
		t.Fatalf("DarkPreset.Chrome.PromptErrorStyle background = %v, want %v", got, want)
	}
}

func TestLightPresetUsesSolarizedLightDefaults(t *testing.T) {
	if got, want := LightPreset.Theme.DefaultBG, rgb(0xfd, 0xf6, 0xe3); got != want {
		t.Fatalf("LightPreset.Theme.DefaultBG = %v, want %v", got, want)
	}
	if got, want := LightPreset.Theme.DefaultFG, rgb(0x65, 0x7b, 0x83); got != want {
		t.Fatalf("LightPreset.Theme.DefaultFG = %v, want %v", got, want)
	}
	if got, want := LightPreset.Chrome.BorderStyle.GetBackground(), rgb(0xfd, 0xf6, 0xe3); got != want {
		t.Fatalf("LightPreset.Chrome.BorderStyle background = %v, want %v", got, want)
	}
	if got, want := LightPreset.Chrome.TitleStyle.GetBackground(), rgb(0xfd, 0xf6, 0xe3); got != want {
		t.Fatalf("LightPreset.Chrome.TitleStyle background = %v, want %v", got, want)
	}
	if got, want := LightPreset.Chrome.HeaderStyle.GetBackground(), rgb(0x93, 0xa1, 0xa1); got != want {
		t.Fatalf("LightPreset.Chrome.HeaderStyle background = %v, want %v", got, want)
	}
	if got, want := LightPreset.Chrome.HeaderStyle.GetForeground(), rgb(0xfd, 0xf6, 0xe3); got != want {
		t.Fatalf("LightPreset.Chrome.HeaderStyle foreground = %v, want %v", got, want)
	}
	if !LightPreset.Chrome.HeaderStyle.HasBold() {
		t.Fatal("LightPreset.Chrome.HeaderStyle should be bold")
	}
}

func TestPlainPresetUsesMonochromePalette(t *testing.T) {
	if got, want := PlainPreset.Theme.ANSI[1], tcolor.Black; got != want {
		t.Fatalf("PlainPreset.Theme.ANSI[1] = %v, want %v", got, want)
	}
	if got, want := PlainPreset.Theme.ANSI[15], tcolor.White; got != want {
		t.Fatalf("PlainPreset.Theme.ANSI[15] = %v, want %v", got, want)
	}
	if got, want := PlainPreset.Chrome.TitleAlign, TitleAlignLeft; got != want {
		t.Fatalf("PlainPreset.Chrome.TitleAlign = %v, want %v", got, want)
	}
	if !PlainPreset.Chrome.HeaderStyle.HasReverse() {
		t.Fatal("PlainPreset.Chrome.HeaderStyle should be reverse")
	}
	if !PlainPreset.Chrome.HeaderStyle.HasDim() {
		t.Fatal("PlainPreset.Chrome.HeaderStyle should be dim")
	}
}

func TestPrettyPresetUsesRoundedDecorativeChrome(t *testing.T) {
	if got, want := PrettyPreset.Chrome.Frame, RoundedFrame(); got != want {
		t.Fatalf("PrettyPreset.Chrome.Frame = %#v, want %#v", got, want)
	}
	if got, want := PrettyPreset.Chrome.TitleAlign, TitleAlignCenter; got != want {
		t.Fatalf("PrettyPreset.Chrome.TitleAlign = %v, want %v", got, want)
	}
	if got, want := PrettyPreset.Chrome.TitleStyle.GetForeground(), rgb(0xd3, 0x36, 0x82); got != want {
		t.Fatalf("PrettyPreset.Chrome.TitleStyle foreground = %v, want %v", got, want)
	}
	if !PrettyPreset.Chrome.TitleStyle.HasBold() {
		t.Fatal("PrettyPreset.Chrome.TitleStyle should be bold")
	}
	if got, want := PrettyPreset.Chrome.HeaderStyle.GetForeground(), rgb(0x2a, 0xa1, 0x98); got != want {
		t.Fatalf("PrettyPreset.Chrome.HeaderStyle foreground = %v, want %v", got, want)
	}
	if got, want := PrettyPreset.Chrome.HeaderStyle.GetBackground(), tcolor.Default; got != want {
		t.Fatalf("PrettyPreset.Chrome.HeaderStyle background = %v, want %v", got, want)
	}
	if !PrettyPreset.Chrome.HeaderStyle.HasBold() {
		t.Fatal("PrettyPreset.Chrome.HeaderStyle should be bold")
	}
}
