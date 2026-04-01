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
	chrome, err := demoChrome("auto", "", goless.PrettyPreset.Chrome)
	if err != nil {
		t.Fatalf("demoChrome(auto) failed: %v", err)
	}
	if got, want := chrome.Frame.TopLeft, "╭"; got != want {
		t.Fatalf("demoChrome(auto).Frame.TopLeft = %q, want %q", got, want)
	}

	chrome, err = demoChrome("single", "Demo", goless.PrettyPreset.Chrome)
	if err != nil {
		t.Fatalf("demoChrome(single) failed: %v", err)
	}
	if got, want := chrome.Frame.TopLeft, "┌"; got != want {
		t.Fatalf("demoChrome(single).Frame.TopLeft = %q, want %q", got, want)
	}
	if got, want := chrome.Title, "Demo"; got != want {
		t.Fatalf("demoChrome(single).Title = %q, want %q", got, want)
	}

	chrome, err = demoChrome("none", "", goless.PrettyPreset.Chrome)
	if err != nil {
		t.Fatalf("demoChrome(none) failed: %v", err)
	}
	if got := chrome.Frame.TopLeft; got != "" {
		t.Fatalf("demoChrome(none).Frame.TopLeft = %q, want empty", got)
	}
	if got := chrome.Frame.Horizontal; got != "" {
		t.Fatalf("demoChrome(none).Frame.Horizontal = %q, want empty", got)
	}

	if _, err := demoChrome("bogus", "", goless.PrettyPreset.Chrome); err == nil {
		t.Fatal("demoChrome(bogus) = nil error, want error")
	}
}

func TestNextDemoPresetName(t *testing.T) {
	cases := map[string]string{
		"":       "dark",
		"dark":   "light",
		"light":  "plain",
		"plain":  "pretty",
		"pretty": "none",
		"none":   "dark",
		"bogus":  "dark",
	}
	for current, want := range cases {
		if got := nextDemoPresetName(current); got != want {
			t.Fatalf("nextDemoPresetName(%q) = %q, want %q", current, got, want)
		}
	}
}

func TestDemoVisualization(t *testing.T) {
	disabled := demoVisualization(false)
	if disabled.ShowTabs || disabled.ShowNewlines || disabled.ShowCarriageReturns || disabled.ShowEOF {
		t.Fatal("demoVisualization(false) unexpectedly enabled markers")
	}

	enabled := demoVisualization(true)
	if !enabled.ShowTabs || !enabled.ShowNewlines || !enabled.ShowCarriageReturns || !enabled.ShowEOF {
		t.Fatal("demoVisualization(true) did not enable all markers")
	}
}

func TestDemoHyperlinkHandler(t *testing.T) {
	handler := demoHyperlinkHandler(false)
	decision := handler(goless.HyperlinkInfo{
		Target: "https://example.com",
		Text:   "example",
	})
	if decision.Live {
		t.Fatal("demo hyperlink handler unexpectedly enabled live links")
	}
	if !decision.StyleSet {
		t.Fatal("demo hyperlink handler did not set style")
	}
	if got, want := decision.Style.GetForeground(), tcolor.Blue; got != want {
		t.Fatalf("demo hyperlink foreground = %v, want %v", got, want)
	}

	live := demoHyperlinkHandler(true)(goless.HyperlinkInfo{
		Target: "http://example.com",
		Text:   "example",
	})
	if !live.Live {
		t.Fatal("demo live hyperlink handler left link inert")
	}
}
