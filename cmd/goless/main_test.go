// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/goless"
	tcolor "github.com/gdamore/tcell/v3/color"
)

func TestDemoPreset(t *testing.T) {
	preset, err := programPreset("dark")
	if err != nil {
		t.Fatalf("programPreset(dark) failed: %v", err)
	}
	if got, want := preset.Theme.DefaultBG, goless.DarkPreset.Theme.DefaultBG; got != want {
		t.Fatalf("programPreset(dark).Theme.DefaultBG = %v, want %v", got, want)
	}

	if _, err := programPreset("bogus"); err == nil {
		t.Fatal("programPreset(bogus) = nil error, want error")
	}
}

func TestDemoChromeUsesPresetAndOverrides(t *testing.T) {
	chrome, err := programChrome("auto", "", goless.PrettyPreset.Chrome)
	if err != nil {
		t.Fatalf("programChrome(auto) failed: %v", err)
	}
	if got, want := chrome.Frame.TopLeft, "╭"; got != want {
		t.Fatalf("programChrome(auto).Frame.TopLeft = %q, want %q", got, want)
	}

	chrome, err = programChrome("single", "Demo", goless.PrettyPreset.Chrome)
	if err != nil {
		t.Fatalf("programChrome(single) failed: %v", err)
	}
	if got, want := chrome.Frame.TopLeft, "┌"; got != want {
		t.Fatalf("programChrome(single).Frame.TopLeft = %q, want %q", got, want)
	}
	if got, want := chrome.Title, "Demo"; got != want {
		t.Fatalf("programChrome(single).Title = %q, want %q", got, want)
	}

	chrome, err = programChrome("none", "", goless.PrettyPreset.Chrome)
	if err != nil {
		t.Fatalf("programChrome(none) failed: %v", err)
	}
	if got := chrome.Frame.TopLeft; got != "" {
		t.Fatalf("programChrome(none).Frame.TopLeft = %q, want empty", got)
	}
	if got := chrome.Frame.Horizontal; got != "" {
		t.Fatalf("programChrome(none).Frame.Horizontal = %q, want empty", got)
	}

	if _, err := programChrome("bogus", "", goless.PrettyPreset.Chrome); err == nil {
		t.Fatal("programChrome(bogus) = nil error, want error")
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
		if got := nextProgramPresetName(current); got != want {
			t.Fatalf("nextProgramPresetName(%q) = %q, want %q", current, got, want)
		}
	}
}

func TestDemoVisualization(t *testing.T) {
	disabled := programVisualization(false)
	if disabled.ShowTabs || disabled.ShowNewlines || disabled.ShowCarriageReturns || disabled.ShowEOF {
		t.Fatal("programVisualization(false) unexpectedly enabled markers")
	}

	enabled := programVisualization(true)
	if !enabled.ShowTabs || !enabled.ShowNewlines || !enabled.ShowCarriageReturns || !enabled.ShowEOF {
		t.Fatal("programVisualization(true) did not enable all markers")
	}
}

func TestDemoHyperlinkHandler(t *testing.T) {
	handler := programHyperlinkHandler(false)
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

	live := programHyperlinkHandler(true)(goless.HyperlinkInfo{
		Target: "http://example.com",
		Text:   "example",
	})
	if !live.Live {
		t.Fatal("demo live hyperlink handler left link inert")
	}
}

func TestDemoQuitAtEOFPolicyFromFlags(t *testing.T) {
	tests := []struct {
		name           string
		quitAtEOF      bool
		quitAtEOFFirst bool
		want           programQuitAtEOFPolicy
	}{
		{name: "disabled", want: programQuitAtEOFNever},
		{name: "forward eof", quitAtEOF: true, want: programQuitAtEOFOnForwardEOF},
		{name: "visible eof", quitAtEOFFirst: true, want: programQuitAtEOFWhenVisible},
		{name: "visible eof wins", quitAtEOF: true, quitAtEOFFirst: true, want: programQuitAtEOFWhenVisible},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := programQuitAtEOFPolicyFromFlags(tt.quitAtEOF, tt.quitAtEOFFirst)
			if got != tt.want {
				t.Fatalf("programQuitAtEOFPolicyFromFlags(%v, %v) = %v, want %v", tt.quitAtEOF, tt.quitAtEOFFirst, got, tt.want)
			}
		})
	}
}

func TestApplyDemoQuitAtEOFQuitsAtLastFile(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	quit, err := applyProgramQuitAtEOF(
		programQuitAtEOFOnForwardEOF,
		session,
		goless.KeyResult{Handled: true, Action: goless.KeyActionPageDown, Context: goless.NormalKeyContext},
		true,
		false,
		goless.Position{Row: 10, Rows: 10},
		goless.Position{Row: 10, Rows: 10},
		func() error { return nil },
	)
	if err != nil {
		t.Fatalf("applyProgramQuitAtEOF returned error: %v", err)
	}
	if !quit {
		t.Fatal("applyProgramQuitAtEOF(...) = false, want true")
	}
}

func TestApplyDemoQuitAtEOFAdvancesToNextFile(t *testing.T) {
	session := newProgramSession([]string{"one.txt", "two.txt"}, programStartup{})
	reloads := 0

	quit, err := applyProgramQuitAtEOF(
		programQuitAtEOFOnForwardEOF,
		session,
		goless.KeyResult{Handled: true, Action: goless.KeyActionPageDown, Context: goless.NormalKeyContext},
		true,
		false,
		goless.Position{Row: 10, Rows: 10},
		goless.Position{Row: 10, Rows: 10},
		func() error {
			reloads++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("applyProgramQuitAtEOF returned error: %v", err)
	}
	if quit {
		t.Fatal("applyProgramQuitAtEOF(...) = true, want false")
	}
	if got, want := session.currentFile(), "two.txt"; got != want {
		t.Fatalf("currentFile() = %q, want %q", got, want)
	}
	if got, want := reloads, 1; got != want {
		t.Fatalf("reload count = %d, want %d", got, want)
	}
}

func TestHandleDemoVisibleEOFQuitsAtLastFile(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 5)
	if err := pager.AppendString("one\ntwo\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	quit, err := handleProgramVisibleEOF(programQuitAtEOFWhenVisible, session, func() *goless.Pager { return pager }, true, func() error { return nil })
	if err != nil {
		t.Fatalf("handleProgramVisibleEOF returned error: %v", err)
	}
	if !quit {
		t.Fatal("handleProgramVisibleEOF(...) = false, want true")
	}
}

func TestHandleDemoVisibleEOFAdvancesPastShortFile(t *testing.T) {
	session := newProgramSession([]string{"one.txt", "two.txt"}, programStartup{})
	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 5)
	if err := pager.AppendString("one\ntwo\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	reloads := 0
	quit, err := handleProgramVisibleEOF(programQuitAtEOFWhenVisible, session, func() *goless.Pager { return pager }, true, func() error {
		reloads++
		pager = goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
		pager.SetSize(20, 5)
		if err := pager.AppendString("one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"); err != nil {
			return err
		}
		pager.Flush()
		return nil
	})
	if err != nil {
		t.Fatalf("handleProgramVisibleEOF returned error: %v", err)
	}
	if quit {
		t.Fatal("handleProgramVisibleEOF(...) = true, want false")
	}
	if got, want := reloads, 1; got != want {
		t.Fatalf("reload count = %d, want %d", got, want)
	}
	if got, want := session.currentFile(), "two.txt"; got != want {
		t.Fatalf("currentFile() = %q, want %q", got, want)
	}
}

func TestHandleDemoVisibleEOFIgnoredWhenNotVisible(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 3)
	if err := pager.AppendString("one\ntwo\nthree\nfour\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	quit, err := handleProgramVisibleEOF(programQuitAtEOFWhenVisible, session, func() *goless.Pager { return pager }, true, func() error { return nil })
	if err != nil {
		t.Fatalf("handleProgramVisibleEOF returned error: %v", err)
	}
	if quit {
		t.Fatal("handleProgramVisibleEOF(...) = true, want false")
	}
}

func TestHandleDemoVisibleEOFIgnoredInFollowMode(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 5)
	if err := pager.AppendString("one\ntwo\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()
	pager.Follow()

	quit, err := handleProgramVisibleEOF(programQuitAtEOFWhenVisible, session, func() *goless.Pager { return pager }, true, func() error { return nil })
	if err != nil {
		t.Fatalf("handleProgramVisibleEOF returned error: %v", err)
	}
	if quit {
		t.Fatal("handleProgramVisibleEOF(...) = true, want false")
	}
}

func TestApplyDemoQuitAtEOFIgnoredOutsideNormalCompletedNavigation(t *testing.T) {
	tests := []struct {
		name          string
		result        goless.KeyResult
		inputComplete bool
		following     bool
		before        goless.Position
		after         goless.Position
	}{
		{
			name:          "prompt input",
			result:        goless.KeyResult{Handled: true, Context: goless.PromptKeyContext},
			inputComplete: true,
			before:        goless.Position{Row: 10, Rows: 10},
			after:         goless.Position{Row: 10, Rows: 10},
		},
		{
			name:          "follow mode",
			result:        goless.KeyResult{Handled: true, Action: goless.KeyActionPageDown, Context: goless.NormalKeyContext},
			inputComplete: true,
			following:     true,
			before:        goless.Position{Row: 10, Rows: 10},
			after:         goless.Position{Row: 10, Rows: 10},
		},
		{
			name:   "stdin still reading",
			result: goless.KeyResult{Handled: true, Action: goless.KeyActionPageDown, Context: goless.NormalKeyContext},
			before: goless.Position{Row: 10, Rows: 10},
			after:  goless.Position{Row: 10, Rows: 10},
		},
		{
			name:          "position change clears arm",
			result:        goless.KeyResult{Handled: true, Action: goless.KeyActionPageDown, Context: goless.NormalKeyContext},
			inputComplete: true,
			before:        goless.Position{Row: 9, Rows: 10},
			after:         goless.Position{Row: 10, Rows: 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quit, err := applyProgramQuitAtEOF(
				programQuitAtEOFOnForwardEOF,
				newProgramSession([]string{"one.txt"}, programStartup{}),
				tt.result,
				tt.inputComplete,
				tt.following,
				tt.before,
				tt.after,
				func() error { return nil },
			)
			if err != nil {
				t.Fatalf("applyProgramQuitAtEOF returned error: %v", err)
			}
			if quit {
				t.Fatal("applyProgramQuitAtEOF(...) = true, want false")
			}
		})
	}
}

func TestHandleDemoVisibleEOFActionRequiresForwardNavigation(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 5)
	if err := pager.AppendString("one\ntwo\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	quit, err := handleProgramVisibleEOFAction(
		programQuitAtEOFWhenVisible,
		session,
		func() *goless.Pager { return pager },
		goless.KeyResult{Handled: true, Action: goless.KeyActionScrollUp, Context: goless.NormalKeyContext},
		true,
		func() error { return nil },
	)
	if err != nil {
		t.Fatalf("handleProgramVisibleEOFAction returned error: %v", err)
	}
	if quit {
		t.Fatal("handleProgramVisibleEOFAction(...) = true, want false")
	}
}

func TestNewDemoPagerPropagatesSqueezeMode(t *testing.T) {
	pager := newProgramPager(
		goless.RenderHybrid,
		goless.Preset{},
		goless.Chrome{},
		false,
		false,
		true,
		nil,
	)

	if !pager.SqueezeBlankLines() {
		t.Fatal("newProgramPager(..., squeeze=true, ...) did not enable squeeze mode")
	}
}

func TestDemoSessionCommandHandler(t *testing.T) {
	session := newProgramSession([]string{"one.txt", "two.txt"}, programStartup{})
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
	session := newProgramSession([]string{"one.txt", "two.txt", "three.txt"}, programStartup{})
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
	session := newProgramSession([]string{"one.txt", "two.txt"}, programStartup{})
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
	session := newProgramSession([]string{"one.txt", "two.txt", "three.txt"}, programStartup{})
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
			startup, files, err := programInputs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("programInputs(...) = nil error, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("programInputs(...) failed: %v", err)
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

	applyStartupCommand(pager, programStartup{line: 3})
	if got, want := pager.Position().Row, 3; got != want {
		t.Fatalf("Position().Row = %d, want %d", got, want)
	}

	pager = goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	if err := pager.AppendString("alpha\nbeta\ngamma\nbeta\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	applyStartupCommand(pager, programStartup{query: "beta"})
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

func TestPassThroughProgramInputsFromStdin(t *testing.T) {
	var out bytes.Buffer
	if err := passThroughProgramInputs(&out, bytes.NewBufferString("alpha\nbeta\n"), nil); err != nil {
		t.Fatalf("passThroughProgramInputs(stdin) failed: %v", err)
	}
	if got, want := out.String(), "alpha\nbeta\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestPassThroughProgramInputsFromFiles(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.txt")
	second := filepath.Join(dir, "second.txt")
	if err := os.WriteFile(first, []byte("first\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(first) failed: %v", err)
	}
	if err := os.WriteFile(second, []byte("second\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(second) failed: %v", err)
	}

	var out bytes.Buffer
	if err := passThroughProgramInputs(&out, bytes.NewBufferString("ignored\n"), []string{first, second}); err != nil {
		t.Fatalf("passThroughProgramInputs(files) failed: %v", err)
	}
	if got, want := out.String(), "first\nsecond\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestPassThroughProgramInputsReportsOpenError(t *testing.T) {
	var out bytes.Buffer
	err := passThroughProgramInputs(&out, bytes.NewBufferString("ignored\n"), []string{filepath.Join(t.TempDir(), "missing.txt")})
	if err == nil {
		t.Fatal("passThroughProgramInputs(...) = nil error, want error")
	}
}
