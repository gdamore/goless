// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/gdamore/goless"
	tcolor "github.com/gdamore/tcell/v3/color"
)

func TestDemoPreset(t *testing.T) {
	preset, err := demoPreset("dark")
	if err != nil {
		t.Fatalf("demoPreset(dark) failed: %v", err)
	}
	if got, want := preset.Theme.DefaultBG, goless.DarkPreset.Theme.DefaultBG; got != want {
		t.Fatalf("demoPreset(dark).Theme.DefaultBG = %v, want %v", got, want)
	}

	if _, err := demoPreset("bogus"); err == nil {
		t.Fatal("demoPreset(bogus) = nil error, want error")
	}
}

func TestDemoChromeUsesPresetAndOverrides(t *testing.T) {
	chrome := demoChrome("auto", "", goless.PrettyPreset.Chrome)
	if got, want := chrome.Frame.TopLeft, "╭"; got != want {
		t.Fatalf("demoChrome(auto).Frame.TopLeft = %q, want %q", got, want)
	}

	chrome = demoChrome("single", "Demo", goless.PrettyPreset.Chrome)
	if got, want := chrome.Frame.TopLeft, "┌"; got != want {
		t.Fatalf("demoChrome(single).Frame.TopLeft = %q, want %q", got, want)
	}
	if got, want := chrome.Title, "Demo"; got != want {
		t.Fatalf("demoChrome(single).Title = %q, want %q", got, want)
	}

	chrome = demoChrome("none", "", goless.PrettyPreset.Chrome)
	if got := chrome.Frame.TopLeft; got != "" {
		t.Fatalf("demoChrome(none).Frame.TopLeft = %q, want empty", got)
	}
	if got := chrome.Frame.Horizontal; got != "" {
		t.Fatalf("demoChrome(none).Frame.Horizontal = %q, want empty", got)
	}
}

func TestDemoThemeCopiesPresetTheme(t *testing.T) {
	theme := demoTheme(goless.Theme{
		DefaultFG: tcolor.Red,
		DefaultBG: tcolor.Blue,
		ANSI:      [16]tcolor.Color{1: tcolor.Aqua},
	})
	if got, want := theme.DefaultFG, tcolor.Red; got != want {
		t.Fatalf("demoTheme().DefaultFG = %v, want %v", got, want)
	}
	if got, want := theme.ANSI[1], tcolor.Aqua; got != want {
		t.Fatalf("demoTheme().ANSI[1] = %v, want %v", got, want)
	}
}
