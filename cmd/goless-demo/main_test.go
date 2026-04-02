// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
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
	if got, want := decision.Style.GetBackground(), tcolor.Default; got != want {
		t.Fatalf("demo hyperlink background = %v, want %v", got, want)
	}

	live := demoHyperlinkHandler(true)(goless.HyperlinkInfo{
		Target: "http://example.com",
		Text:   "example",
	})
	if !live.Live {
		t.Fatal("demo live hyperlink handler left link inert")
	}
}

func TestDemoSessionCommandHandler(t *testing.T) {
	session := newDemoSession([]string{"one.txt", "two.txt"}, demoStartup{})
	reloads := 0
	handler := session.commandHandler(func() error {
		reloads++
		return nil
	})

	if result := handler(goless.Command{Name: "quit"}); !result.Handled || !result.Quit {
		t.Fatalf("quit command result = %+v, want handled quit", result)
	}
	if result := handler(goless.Command{Name: "q"}); !result.Handled || !result.Quit {
		t.Fatalf("q command result = %+v, want handled quit", result)
	}

	result := handler(goless.Command{Name: "next"})
	if !result.Handled || result.Quit {
		t.Fatalf("next command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "two.txt (2/2)"; got != want {
		t.Fatalf("next command message = %q, want %q", got, want)
	}
	if got, want := session.currentFile(), "two.txt"; got != want {
		t.Fatalf("current file after next = %q, want %q", got, want)
	}
	if got, want := reloads, 1; got != want {
		t.Fatalf("reload count after next = %d, want %d", got, want)
	}

	result = handler(goless.Command{Name: "next"})
	if !result.Handled || result.Message == "" {
		t.Fatalf("boundary next command result = %+v, want handled message", result)
	}
	if got, want := reloads, 1; got != want {
		t.Fatalf("reload count after boundary next = %d, want %d", got, want)
	}

	result = handler(goless.Command{Name: "prev"})
	if !result.Handled || result.Quit {
		t.Fatalf("prev command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "one.txt (1/2)"; got != want {
		t.Fatalf("prev command message = %q, want %q", got, want)
	}
	if got, want := session.currentFile(), "one.txt"; got != want {
		t.Fatalf("current file after prev = %q, want %q", got, want)
	}
	if got, want := reloads, 2; got != want {
		t.Fatalf("reload count after prev = %d, want %d", got, want)
	}

	result = handler(goless.Command{Name: "bogus"})
	if result.Handled || result.Quit {
		t.Fatalf("bogus command result = %+v, want unhandled", result)
	}
}

func TestDemoSessionAdditionalCommands(t *testing.T) {
	session := newDemoSession([]string{"one.txt", "two.txt", "three.txt"}, demoStartup{})
	reloads := 0
	handler := session.commandHandler(func() error {
		reloads++
		return nil
	})

	result := handler(goless.Command{Name: "file"})
	if !result.Handled || result.Quit {
		t.Fatalf("file command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "one.txt (1/3)"; got != want {
		t.Fatalf("file command message = %q, want %q", got, want)
	}

	result = handler(goless.Command{Name: "last"})
	if !result.Handled || result.Quit {
		t.Fatalf("last command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "three.txt (3/3)"; got != want {
		t.Fatalf("last command message = %q, want %q", got, want)
	}
	if got, want := reloads, 1; got != want {
		t.Fatalf("reload count after last = %d, want %d", got, want)
	}

	result = handler(goless.Command{Name: "rewind"})
	if !result.Handled || result.Quit {
		t.Fatalf("rewind command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "one.txt (1/3)"; got != want {
		t.Fatalf("rewind command message = %q, want %q", got, want)
	}
	if got, want := reloads, 2; got != want {
		t.Fatalf("reload count after rewind = %d, want %d", got, want)
	}
}

func TestDemoSessionCommandHandlerReloadFailureRestoresIndex(t *testing.T) {
	session := newDemoSession([]string{"one.txt", "two.txt"}, demoStartup{})
	handler := session.commandHandler(func() error {
		return fmt.Errorf("reload failed")
	})

	result := handler(goless.Command{Name: "next"})
	if !result.Handled || result.Quit {
		t.Fatalf("next command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "reload failed"; got != want {
		t.Fatalf("next command message = %q, want %q", got, want)
	}
	if got, want := session.currentFile(), "one.txt"; got != want {
		t.Fatalf("current file after failed reload = %q, want %q", got, want)
	}
}

func TestDemoSessionCommandHandlerReloadFailureRestoresIndexForLast(t *testing.T) {
	session := newDemoSession([]string{"one.txt", "two.txt", "three.txt"}, demoStartup{})
	handler := session.commandHandler(func() error {
		return fmt.Errorf("reload failed")
	})

	result := handler(goless.Command{Name: "last"})
	if !result.Handled || result.Quit {
		t.Fatalf("last command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "reload failed"; got != want {
		t.Fatalf("last command message = %q, want %q", got, want)
	}
	if got, want := session.currentFile(), "one.txt"; got != want {
		t.Fatalf("current file after failed last reload = %q, want %q", got, want)
	}
}

func TestDemoInputs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantLine  int
		wantQuery string
		wantFiles []string
		wantErr   bool
	}{
		{name: "none", args: nil},
		{name: "file only", args: []string{"sample.txt"}, wantFiles: []string{"sample.txt"}},
		{name: "multiple files", args: []string{"a.txt", "b.txt"}, wantFiles: []string{"a.txt", "b.txt"}},
		{name: "startup only", args: []string{"+42"}, wantLine: 42},
		{name: "startup and file", args: []string{"+42", "sample.txt"}, wantLine: 42, wantFiles: []string{"sample.txt"}},
		{name: "startup search and file", args: []string{"+/needle", "sample.txt"}, wantQuery: "needle", wantFiles: []string{"sample.txt"}},
		{name: "startup with separator", args: []string{"+7", "--", "sample.txt"}, wantLine: 7, wantFiles: []string{"sample.txt"}},
		{name: "separator only", args: []string{"--", "sample.txt"}, wantFiles: []string{"sample.txt"}},
		{name: "bad startup", args: []string{"+bogus"}, wantErr: true},
		{name: "bad startup search", args: []string{"+/"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startup, files, err := demoInputs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("demoInputs(...) = nil error, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("demoInputs(...) failed: %v", err)
			}
			if got, want := startup.line, tt.wantLine; got != want {
				t.Fatalf("startup.line = %d, want %d", got, want)
			}
			if got, want := startup.query, tt.wantQuery; got != want {
				t.Fatalf("startup.query = %q, want %q", got, want)
			}
			if len(files) != len(tt.wantFiles) {
				t.Fatalf("len(files) = %d, want %d", len(files), len(tt.wantFiles))
			}
			for i := range tt.wantFiles {
				if got, want := files[i], tt.wantFiles[i]; got != want {
					t.Fatalf("files[%d] = %q, want %q", i, got, want)
				}
			}
		})
	}
}

func TestApplyStartupCommand(t *testing.T) {
	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	if err := pager.AppendString("one\ntwo\nthree\nfour\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	applyStartupCommand(pager, demoStartup{line: 3})
	if got, want := pager.Position().Row, 3; got != want {
		t.Fatalf("Position().Row = %d, want %d", got, want)
	}

	pager = goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\nbeta\ngamma\nbeta\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	applyStartupCommand(pager, demoStartup{query: "beta"})
	state := pager.SearchState()
	if got, want := state.Query, "beta"; got != want {
		t.Fatalf("SearchState().Query = %q, want %q", got, want)
	}
	if got, want := state.CurrentMatch, 1; got != want {
		t.Fatalf("SearchState().CurrentMatch = %d, want %d", got, want)
	}
	if got, want := pager.Position().Row, 2; got != want {
		t.Fatalf("Position().Row after startup search = %d, want %d", got, want)
	}
}
