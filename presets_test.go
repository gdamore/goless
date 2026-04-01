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
}

func TestLightPresetUsesSolarizedLightDefaults(t *testing.T) {
	if got, want := LightPreset.Theme.DefaultBG, rgb(0xfd, 0xf6, 0xe3); got != want {
		t.Fatalf("LightPreset.Theme.DefaultBG = %v, want %v", got, want)
	}
	if got, want := LightPreset.Theme.DefaultFG, rgb(0x65, 0x7b, 0x83); got != want {
		t.Fatalf("LightPreset.Theme.DefaultFG = %v, want %v", got, want)
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
}
