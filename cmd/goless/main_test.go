// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/goless"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
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

	base := goless.PrettyPreset.Chrome
	base.StatusHelpKeyStyle = tcell.StyleDefault.Foreground(color.Red).Bold(true)
	chrome, err = programChrome("single", "Demo", base)
	if err != nil {
		t.Fatalf("programChrome(single, custom help style) failed: %v", err)
	}
	if got, want := chrome.StatusHelpKeyStyle, base.StatusHelpKeyStyle; got != want {
		t.Fatalf("programChrome(single).StatusHelpKeyStyle = %#v, want %#v", got, want)
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
		"pretty": "dark",
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
	if got, want := decision.Style.GetForeground(), color.Blue; got != want {
		t.Fatalf("demo hyperlink foreground = %v, want %v", got, want)
	}
	if got, want := decision.Style.GetBackground(), color.Default; got != want {
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

func TestHandleProgramQuitIfOneScreenQuitsAtLastFile(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	snapshot := programViewportSnapshot{eofVisible: true}
	quit, err := handleProgramQuitIfOneScreen(true, session, func() programViewportSnapshot { return snapshot }, true, func() (bool, error) { return true, nil })
	if err != nil {
		t.Fatalf("handleProgramQuitIfOneScreen returned error: %v", err)
	}
	if !quit {
		t.Fatal("handleProgramQuitIfOneScreen(...) = false, want true")
	}
}

func TestHandleProgramQuitIfOneScreenAdvancesPastShortFile(t *testing.T) {
	session := newProgramSession([]string{"one.txt", "two.txt"}, programStartup{})
	current := programViewportSnapshot{eofVisible: true}
	reloads := 0
	quit, err := handleProgramQuitIfOneScreen(true, session, func() programViewportSnapshot { return current }, true, func() (bool, error) {
		reloads++
		current = programViewportSnapshot{eofVisible: false}
		return true, nil
	})
	if err != nil {
		t.Fatalf("handleProgramQuitIfOneScreen returned error: %v", err)
	}
	if quit {
		t.Fatal("handleProgramQuitIfOneScreen(...) = true, want false")
	}
	if got, want := session.currentFile(), "two.txt"; got != want {
		t.Fatalf("currentFile() = %q, want %q", got, want)
	}
	if got, want := reloads, 1; got != want {
		t.Fatalf("reload count = %d, want %d", got, want)
	}
}

func TestHandleProgramQuitIfOneScreenDisabledWhenInputIncomplete(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	snapshot := programViewportSnapshot{eofVisible: true}
	quit, err := handleProgramQuitIfOneScreen(true, session, func() programViewportSnapshot { return snapshot }, false, func() (bool, error) { return true, nil })
	if err != nil {
		t.Fatalf("handleProgramQuitIfOneScreen returned error: %v", err)
	}
	if quit {
		t.Fatal("handleProgramQuitIfOneScreen(...) = true, want false")
	}
}

func TestHandleProgramQuitIfOneScreenIgnoresPostStartupViewport(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{line: 999})
	snapshot := programViewportSnapshot{eofVisible: false}

	quit, err := handleProgramQuitIfOneScreen(true, session, func() programViewportSnapshot { return snapshot }, true, func() (bool, error) { return true, nil })
	if err != nil {
		t.Fatalf("handleProgramQuitIfOneScreen returned error: %v", err)
	}
	if quit {
		t.Fatal("handleProgramQuitIfOneScreen(...) = true, want false when top-of-file viewport does not fit")
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
		func() (bool, error) { return true, nil },
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
		func() (bool, error) {
			reloads++
			return true, nil
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

	quit, err := handleProgramVisibleEOF(programQuitAtEOFWhenVisible, session, func() *goless.Pager { return pager }, true, func() (bool, error) { return true, nil })
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
	quit, err := handleProgramVisibleEOF(programQuitAtEOFWhenVisible, session, func() *goless.Pager { return pager }, true, func() (bool, error) {
		reloads++
		pager = goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
		pager.SetSize(20, 5)
		if err := pager.AppendString("one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"); err != nil {
			return false, err
		}
		pager.Flush()
		return true, nil
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

func TestHandleDemoVisibleEOFWaitsForExplicitStdin(t *testing.T) {
	session := newProgramSession([]string{"one.txt", "-"}, programStartup{})
	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 5)
	if err := pager.AppendString("one\ntwo\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	reloads := 0
	quit, err := handleProgramVisibleEOF(programQuitAtEOFWhenVisible, session, func() *goless.Pager { return pager }, true, func() (bool, error) {
		reloads++
		pager = goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
		pager.SetSize(20, 5)
		return false, nil
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
	if got, want := session.currentFile(), "-"; got != want {
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

	quit, err := handleProgramVisibleEOF(programQuitAtEOFWhenVisible, session, func() *goless.Pager { return pager }, true, func() (bool, error) { return true, nil })
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

	quit, err := handleProgramVisibleEOF(programQuitAtEOFWhenVisible, session, func() *goless.Pager { return pager }, true, func() (bool, error) { return true, nil })
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
				func() (bool, error) { return true, nil },
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
		func() (bool, error) { return true, nil },
	)
	if err != nil {
		t.Fatalf("handleProgramVisibleEOFAction returned error: %v", err)
	}
	if quit {
		t.Fatal("handleProgramVisibleEOFAction(...) = true, want false")
	}
}

func TestParseProgramFlagsCompatibilityAliases(t *testing.T) {
	var out bytes.Buffer
	opts, args, err := parseProgramFlags([]string{"-F", "-S", "-N", "-s", "-x", "4", "-I", "sample.txt"}, &out)
	if err != nil {
		t.Fatalf("parseProgramFlags(...) failed: %v", err)
	}
	if opts.showHelp {
		t.Fatal("parseProgramFlags(...) unexpectedly set showHelp")
	}
	if !opts.lineNumbers {
		t.Fatal("parseProgramFlags(...) did not enable line numbers for -N")
	}
	if !opts.quitIfOneScreen {
		t.Fatal("parseProgramFlags(...) did not enable quitIfOneScreen for -F")
	}
	if !opts.squeeze {
		t.Fatal("parseProgramFlags(...) did not enable squeeze for -s")
	}
	if got, want := opts.tabWidth, 4; got != want {
		t.Fatalf("tab width = %d, want %d", got, want)
	}
	if got, want := opts.searchCaseMode, goless.SearchCaseInsensitive; got != want {
		t.Fatalf("search case mode = %v, want %v", got, want)
	}
	if got, want := len(args), 1; got != want {
		t.Fatalf("len(args) = %d, want %d", got, want)
	}
	if got, want := args[0], "sample.txt"; got != want {
		t.Fatalf("args[0] = %q, want %q", got, want)
	}
}

func TestParseProgramFlagsRejectsConflictingSearchCaseFlags(t *testing.T) {
	var out bytes.Buffer

	if _, _, err := parseProgramFlags([]string{"-i", "-I"}, &out); err == nil {
		t.Fatal("parseProgramFlags(-i, -I) = nil error, want mutual exclusion error")
	}
}

func TestParseProgramFlagsRejectsNonPositiveTabWidth(t *testing.T) {
	var out bytes.Buffer

	if _, _, err := parseProgramFlags([]string{"-x", "0"}, &out); err == nil {
		t.Fatal("parseProgramFlags(-x 0) = nil error, want invalid tab width error")
	}
}

func TestParseProgramFlagsVersion(t *testing.T) {
	var out bytes.Buffer
	opts, args, err := parseProgramFlags([]string{"-version"}, &out)
	if err != nil {
		t.Fatalf("parseProgramFlags(-version) failed: %v", err)
	}
	if !opts.version {
		t.Fatal("parseProgramFlags(-version) did not set version")
	}
	if len(args) != 0 {
		t.Fatalf("len(args) after -version = %d, want 0", len(args))
	}
}

func TestParseProgramFlagsHelp(t *testing.T) {
	var out bytes.Buffer
	opts, args, err := parseProgramFlags([]string{"--help"}, &out)
	if err != nil {
		t.Fatalf("parseProgramFlags(--help) failed: %v", err)
	}
	if !opts.showHelp {
		t.Fatal("parseProgramFlags(--help) did not set showHelp")
	}
	if len(args) != 0 {
		t.Fatalf("len(args) after --help = %d, want 0", len(args))
	}
	if got := out.String(); !strings.Contains(got, "usage: goless") {
		t.Fatalf("help output = %q, want usage text", got)
	}

	out.Reset()
	opts, args, err = parseProgramFlags([]string{"-?"}, &out)
	if err != nil {
		t.Fatalf("parseProgramFlags(-?) failed: %v", err)
	}
	if !opts.showHelp {
		t.Fatal("parseProgramFlags(-?) did not set showHelp")
	}
	if len(args) != 0 {
		t.Fatalf("len(args) after -? = %d, want 0", len(args))
	}
	if got := out.String(); !strings.Contains(got, "usage: goless") {
		t.Fatalf("help output for -? = %q, want usage text", got)
	}

	out.Reset()
	opts, args, err = parseProgramFlags([]string{"-h"}, &out)
	if err != nil {
		t.Fatalf("parseProgramFlags(-h) failed: %v", err)
	}
	if !opts.showHelp {
		t.Fatal("parseProgramFlags(-h) did not set showHelp")
	}
	if len(args) != 0 {
		t.Fatalf("len(args) after -h = %d, want 0", len(args))
	}
	if got := out.String(); !strings.Contains(got, "usage: goless") {
		t.Fatalf("help output for -h = %q, want usage text", got)
	}
}

func TestParseProgramFlagsLicense(t *testing.T) {
	var out bytes.Buffer
	opts, args, err := parseProgramFlags([]string{"--license"}, &out)
	if err != nil {
		t.Fatalf("parseProgramFlags(--license) failed: %v", err)
	}
	if !opts.showLicense {
		t.Fatal("parseProgramFlags(--license) did not set showLicense")
	}
	if len(args) != 0 {
		t.Fatalf("len(args) after --license = %d, want 0", len(args))
	}
}

func TestParseProgramFlagsLicenseExclusive(t *testing.T) {
	var out bytes.Buffer
	_, _, err := parseProgramFlags([]string{"--license", "-N"}, &out)
	if err == nil {
		t.Fatal("parseProgramFlags(--license -N) = nil error, want error")
	}
	out.Reset()
	if _, _, err := parseProgramFlags([]string{"--license", "file.txt"}, &out); err == nil {
		t.Fatal("parseProgramFlags(--license file.txt) = nil error, want error")
	}
}

func TestProgramRequiresInput(t *testing.T) {
	tests := []struct {
		name  string
		opts  programOptions
		files []string
		want  bool
	}{
		{name: "license without files", opts: programOptions{showLicense: true}, want: false},
		{name: "license with stdin file", opts: programOptions{showLicense: true}, files: []string{"-"}, want: false},
		{name: "no files", opts: programOptions{}, want: true},
		{name: "stdin file", opts: programOptions{}, files: []string{"sample.txt", "-"}, want: true},
		{name: "real file", opts: programOptions{}, files: []string{"sample.txt"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := programRequiresInput(tt.opts, tt.files); got != tt.want {
				t.Fatalf("programRequiresInput(%+v, %v) = %v, want %v", tt.opts, tt.files, got, tt.want)
			}
		})
	}
}

func TestProgramShouldQuitAfterOverlayClose(t *testing.T) {
	tests := []struct {
		name          string
		withFiles     bool
		content       string
		result        goless.KeyResult
		overlay       bool
		remainOverlay bool
		want          bool
	}{
		{
			name:          "license only close exits",
			result:        goless.KeyResult{Handled: true, Context: goless.HelpKeyContext, Action: goless.KeyActionToggleHelp},
			overlay:       true,
			remainOverlay: false,
			want:          true,
		},
		{
			name:          "still showing overlay does not exit",
			result:        goless.KeyResult{Handled: true, Context: goless.HelpKeyContext, Action: goless.KeyActionToggleHelp},
			overlay:       true,
			remainOverlay: true,
			want:          false,
		},
		{
			name:      "open files do not exit",
			withFiles: true,
			result:    goless.KeyResult{Handled: true, Context: goless.HelpKeyContext, Action: goless.KeyActionToggleHelp},
			want:      false,
		},
		{
			name:    "document content does not exit",
			content: "alpha\n",
			result:  goless.KeyResult{Handled: true, Context: goless.HelpKeyContext, Action: goless.KeyActionToggleHelp},
			want:    false,
		},
		{
			name:   "non-toggle action does not exit",
			result: goless.KeyResult{Handled: true, Context: goless.HelpKeyContext, Action: goless.KeyActionRefresh},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var files []string
			if tt.withFiles {
				files = []string{"sample.txt"}
			}
			session := newProgramSession(files, programStartup{})
			pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
			pager.SetSize(20, 4)
			if tt.content != "" {
				if err := pager.AppendString(tt.content); err != nil {
					t.Fatalf("AppendString failed: %v", err)
				}
				pager.Flush()
			}
			if tt.overlay {
				pager.ShowInformation("License", "Apache License")
			}
			wasShowing := pager.ShowingInformation()
			if tt.overlay && !tt.remainOverlay {
				pager.HideInformation()
			}

			if got := programShouldQuitAfterOverlayClose(session, pager, wasShowing, tt.result); got != tt.want {
				t.Fatalf("programShouldQuitAfterOverlayClose(...) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewDemoPagerPropagatesConfig(t *testing.T) {
	pager := newProgramPager(
		goless.RenderHybrid,
		goless.Preset{},
		goless.Chrome{},
		false,
		false,
		true,
		goless.SearchCaseInsensitive,
		true,
		4,
		nil,
	)

	if !pager.SqueezeBlankLines() {
		t.Fatal("newProgramPager(..., squeeze=true, ...) did not enable squeeze mode")
	}
	if !pager.LineNumbers() {
		t.Fatal("newProgramPager(..., lineNumbers=true, ...) did not enable line numbers")
	}
	if got, want := pager.SearchCaseMode(), goless.SearchCaseInsensitive; got != want {
		t.Fatalf("SearchCaseMode() = %v, want %v", got, want)
	}
}

func TestDemoSessionCommandHandler(t *testing.T) {
	session := newProgramSession([]string{"one.txt", "two.txt"}, programStartup{})
	reloads := 0
	handler := session.commandHandler(func() error {
		reloads++
		return nil
	}, nil)

	if result := handler(goless.Command{Name: "quit"}); !result.Handled || !result.Quit {
		t.Fatalf("quit command result = %+v, want handled quit", result)
	}
	if result := handler(goless.Command{Name: "q"}); !result.Handled || !result.Quit {
		t.Fatalf("q command result = %+v, want handled quit", result)
	}
	if result := handler(goless.Command{Name: "Q"}); !result.Handled || !result.Quit {
		t.Fatalf("Q command result = %+v, want handled quit", result)
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
	}, nil)

	result := handler(goless.Command{Name: "file"})
	if !result.Handled || result.Quit {
		t.Fatalf("file command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "one.txt (1/3)"; got != want {
		t.Fatalf("file command message = %q, want %q", got, want)
	}

	result = handler(goless.Command{Name: "version"})
	if !result.Handled || result.Quit {
		t.Fatalf("version command result = %+v, want handled non-quit", result)
	}
	if result.Message == "" {
		t.Fatalf("version is missing")
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

	result = handler(goless.Command{Name: "x"})
	if !result.Handled || result.Quit {
		t.Fatalf("x command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "already at first file"; got != want {
		t.Fatalf("x command message = %q, want %q", got, want)
	}
	if got, want := reloads, 2; got != want {
		t.Fatalf("reload count after x = %d, want %d", got, want)
	}
}

func TestDemoSessionLicenseCommandShowsBundledLicense(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	var (
		title string
		body  string
	)
	handler := session.commandHandler(func() error {
		t.Fatal("reload should not be called for :license")
		return nil
	}, func(gotTitle, gotBody string) {
		title = gotTitle
		body = gotBody
	})

	result := handler(goless.Command{Name: "license"})
	if !result.Handled || result.Quit {
		t.Fatalf("license command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, ""; got != want {
		t.Fatalf("license command message = %q, want empty", got)
	}
	if got, want := title, "Apache License 2.0"; got != want {
		t.Fatalf("license title = %q, want %q", got, want)
	}
	if !strings.Contains(body, "Apache License") {
		t.Fatalf("license body missing Apache heading: %q", body)
	}
	if !strings.Contains(body, "Version 2.0, January 2004") {
		t.Fatalf("license body missing version heading: %q", body)
	}
}

func TestHandleProgramStatusKey(t *testing.T) {
	var seen []string
	pager := goless.New(goless.Config{
		TabWidth:   4,
		WrapMode:   goless.NoWrap,
		ShowStatus: true,
		CommandHandler: func(cmd goless.Command) goless.CommandResult {
			seen = append(seen, cmd.Name)
			if cmd.Name == "file" || cmd.Name == "f" {
				return goless.CommandResult{Handled: true, Message: "demo.txt (1/1)"}
			}
			return goless.CommandResult{}
		},
	})
	pager.SetSize(20, 2)
	if err := pager.AppendString("one\ntwo\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	for _, tt := range []struct {
		name string
		ev   *tcell.EventKey
	}{
		{name: "equals", ev: tcell.NewEventKey(tcell.KeyRune, "=", tcell.ModNone)},
		{name: "ctrl-g", ev: tcell.NewEventKey(tcell.KeyCtrlG, "", tcell.ModNone)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			seen = nil
			if !handleProgramStatusKey(pager, tt.ev) {
				t.Fatalf("handleProgramStatusKey(%s) = false, want true", tt.name)
			}
			if got, want := len(seen), 1; got != want {
				t.Fatalf("command count after %s = %d, want %d", tt.name, got, want)
			}
			if got, want := seen[0], "file"; got != want {
				t.Fatalf("command after %s = %q, want %q", tt.name, got, want)
			}
		})
	}

	if handleProgramStatusKey(pager, tcell.NewEventKey(tcell.KeyRune, "=", tcell.ModAlt)) {
		t.Fatal("handleProgramStatusKey(Alt-=) = true, want false")
	}
}

func TestShouldHandleProgramStatusKey(t *testing.T) {
	tests := []struct {
		name   string
		result goless.KeyResult
		want   bool
	}{
		{name: "normal-unhandled", result: goless.KeyResult{}, want: true},
		{name: "normal-handled", result: goless.KeyResult{Handled: true, Context: goless.NormalKeyContext}, want: false},
		{name: "prompt-unhandled", result: goless.KeyResult{Context: goless.PromptKeyContext}, want: false},
		{name: "help-unhandled", result: goless.KeyResult{Context: goless.HelpKeyContext}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldHandleProgramStatusKey(tt.result); got != tt.want {
				t.Fatalf("shouldHandleProgramStatusKey(%+v) = %v, want %v", tt.result, got, tt.want)
			}
		})
	}
}

func TestUpdateProgramQuitIfOneScreenArm(t *testing.T) {
	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 3)
	if err := pager.AppendString("one\ntwo\nthree\nfour\nfive\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if got := updateProgramQuitIfOneScreenArm(true, pager); !got {
		t.Fatal("updateProgramQuitIfOneScreenArm(true, origin pager) = false, want true")
	}

	pager.ScrollDown(1)
	if got := updateProgramQuitIfOneScreenArm(true, pager); got {
		t.Fatal("updateProgramQuitIfOneScreenArm(true, scrolled pager) = true, want false")
	}
	if got := updateProgramQuitIfOneScreenArm(false, pager); got {
		t.Fatal("updateProgramQuitIfOneScreenArm(false, scrolled pager) = true, want false")
	}

	pager = goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 3)
	if err := pager.AppendString("one\ntwo\nthree\nfour\nfive\n"); err != nil {
		t.Fatalf("AppendString(second pager) failed: %v", err)
	}
	pager.Flush()
	pager.Follow()
	if got := updateProgramQuitIfOneScreenArm(true, pager); got {
		t.Fatal("updateProgramQuitIfOneScreenArm(true, following pager) = true, want false")
	}
}

func TestDemoSessionLabelsExplicitStdin(t *testing.T) {
	session := newProgramSession([]string{"one.txt", "-", "three.txt"}, programStartup{})
	reloads := 0
	handler := session.commandHandler(func() error {
		reloads++
		return nil
	}, nil)

	result := handler(goless.Command{Name: "next"})
	if !result.Handled || result.Quit {
		t.Fatalf("next command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "stdin (2/3)"; got != want {
		t.Fatalf("next command message = %q, want %q", got, want)
	}
	if got, want := reloads, 1; got != want {
		t.Fatalf("reload count after next = %d, want %d", got, want)
	}
}

func TestDemoSessionChromeLabelsExplicitStdin(t *testing.T) {
	session := newProgramSession([]string{"-"}, programStartup{})
	chrome := session.chrome(goless.Chrome{Title: "Demo"})
	if got, want := chrome.Title, "Demo - stdin"; got != want {
		t.Fatalf("chrome.Title = %q, want %q", got, want)
	}
}

func TestDemoSessionCommandHandlerBlocksNavigationWhileStdinReading(t *testing.T) {
	session := newProgramSession([]string{"one.txt", "-"}, programStartup{})
	handler := session.commandHandler(func() error {
		return fmt.Errorf("stdin still reading")
	}, nil)

	result := handler(goless.Command{Name: "next"})
	if !result.Handled || result.Quit {
		t.Fatalf("next command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "stdin still reading"; got != want {
		t.Fatalf("next command message = %q, want %q", got, want)
	}
	if got, want := session.currentFile(), "one.txt"; got != want {
		t.Fatalf("current file after blocked reload = %q, want %q", got, want)
	}
}

func TestDemoSessionCommandHandlerReloadFailureRestoresIndex(t *testing.T) {
	session := newProgramSession([]string{"one.txt", "two.txt"}, programStartup{})
	handler := session.commandHandler(func() error {
		return fmt.Errorf("reload failed")
	}, nil)

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
	}, nil)

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
		{name: "explicit stdin only", args: []string{"-"}},
		{name: "explicit stdin among files", args: []string{"a.txt", "-", "b.txt"}, wantFiles: []string{"a.txt", "-", "b.txt"}},
		{name: "startup only", args: []string{"+42"}, wantLine: 42},
		{name: "startup and file", args: []string{"+42", "sample.txt"}, wantLine: 42, wantFiles: []string{"sample.txt"}},
		{name: "startup and explicit stdin", args: []string{"+42", "-"}, wantLine: 42},
		{name: "startup search and file", args: []string{"+/needle", "sample.txt"}, wantQuery: "needle", wantFiles: []string{"sample.txt"}},
		{name: "startup with separator", args: []string{"+7", "--", "sample.txt"}, wantLine: 7, wantFiles: []string{"sample.txt"}},
		{name: "separator only", args: []string{"--", "sample.txt"}, wantFiles: []string{"sample.txt"}},
		{name: "separator stdin", args: []string{"--", "-"}},
		{name: "duplicate explicit stdin", args: []string{"-", "-"}, wantErr: true},
		{name: "duplicate explicit stdin among files", args: []string{"a.txt", "-", "b.txt", "-"}, wantErr: true},
		{name: "duplicate explicit stdin with startup", args: []string{"+42", "--", "-", "sample.txt", "-"}, wantErr: true},
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

func TestProgramInputsUseStdin(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		want  bool
	}{
		{name: "none"},
		{name: "file only", files: []string{"sample.txt"}},
		{name: "stdin only", files: []string{"-"}, want: true},
		{name: "mixed", files: []string{"sample.txt", "-", "other.txt"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := programInputsUseStdin(tt.files); got != tt.want {
				t.Fatalf("programInputsUseStdin(%v) = %v, want %v", tt.files, got, tt.want)
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

func TestPassThroughProgramInputsFromMixedSources(t *testing.T) {
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
	if err := passThroughProgramInputs(&out, bytes.NewBufferString("stdin\n"), []string{first, "-", second}); err != nil {
		t.Fatalf("passThroughProgramInputs(mixed) failed: %v", err)
	}
	if got, want := out.String(), "first\nstdin\nsecond\n"; got != want {
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

func TestProgramInputLoaderCachesExplicitStdin(t *testing.T) {
	loader := newProgramInputLoader(bytes.NewBufferString("alpha\nbeta\n"))

	first, err := loader.open("-")
	if err != nil {
		t.Fatalf("loader.open(-) failed: %v", err)
	}
	firstData, err := io.ReadAll(first)
	if err != nil {
		t.Fatalf("ReadAll(first) failed: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close(first) failed: %v", err)
	}

	second, err := loader.open("-")
	if err != nil {
		t.Fatalf("loader.open(-) second time failed: %v", err)
	}
	secondData, err := io.ReadAll(second)
	if err != nil {
		t.Fatalf("ReadAll(second) failed: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("Close(second) failed: %v", err)
	}

	if got, want := string(firstData), "alpha\nbeta\n"; got != want {
		t.Fatalf("first stdin read = %q, want %q", got, want)
	}
	if got, want := string(secondData), "alpha\nbeta\n"; got != want {
		t.Fatalf("second stdin read = %q, want %q", got, want)
	}
}

func TestProgramInputLoaderRejectsUnavailableExplicitStdin(t *testing.T) {
	loader := newProgramInputLoader(nil)
	if loader.canStream("-") {
		t.Fatal("loader.canStream(-) = true with nil stdin, want false")
	}
	if _, err := loader.open("-"); err == nil {
		t.Fatal("loader.open(-) with nil stdin = nil error, want error")
	}
}

func TestProgramInputLoaderStreamsAndCachesExplicitStdin(t *testing.T) {
	loader := newProgramInputLoader(bytes.NewBufferString("alpha\nbeta\n"))
	if !loader.canStream("-") {
		t.Fatal("loader.canStream(-) = false, want true")
	}

	reader, finish, err := loader.startStdinStream()
	if err != nil {
		t.Fatalf("loader.startStdinStream() failed: %v", err)
	}
	if loader.canStream("-") {
		t.Fatal("loader.canStream(-) = true while stdin is active, want false")
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll(stream) failed: %v", err)
	}
	finish(nil, data)

	if got, want := string(data), "alpha\nbeta\n"; got != want {
		t.Fatalf("stream data = %q, want %q", got, want)
	}

	cached, err := loader.open("-")
	if err != nil {
		t.Fatalf("loader.open(-) after stream failed: %v", err)
	}
	cachedData, err := io.ReadAll(cached)
	if err != nil {
		t.Fatalf("ReadAll(cached) failed: %v", err)
	}
	if err := cached.Close(); err != nil {
		t.Fatalf("Close(cached) failed: %v", err)
	}
	if got, want := string(cachedData), "alpha\nbeta\n"; got != want {
		t.Fatalf("cached data = %q, want %q", got, want)
	}
}

func TestProgramInputLoaderStartStdinStreamErrors(t *testing.T) {
	if _, _, err := (*programInputLoader)(nil).startStdinStream(); err == nil {
		t.Fatal("nil loader startStdinStream = nil error, want error")
	}

	loader := newProgramInputLoader(bytes.NewBufferString("alpha\n"))
	reader, finish, err := loader.startStdinStream()
	if err != nil {
		t.Fatalf("loader.startStdinStream() failed: %v", err)
	}
	if _, _, err := loader.startStdinStream(); err == nil {
		t.Fatal("second startStdinStream while active = nil error, want error")
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll(stream) failed: %v", err)
	}
	finish(nil, data)
	if _, _, err := loader.startStdinStream(); err == nil {
		t.Fatal("startStdinStream after cache = nil error, want error")
	}
}

func TestReadIntoPagerWithAfterRunsHook(t *testing.T) {
	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 3)
	events := make(chan tcell.Event, 8)
	result := make(chan error, 1)
	done := make(chan error, 1)

	go readIntoPagerWithAfter(pager, bytes.NewBufferString("alpha\nbeta\n"), events, result, func(err error) {
		done <- err
	})

	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("readIntoPagerWithAfter result = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for read result")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("readIntoPagerWithAfter hook = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for after hook")
	}

	if got, want := pager.Position().Rows, 2; got != want {
		t.Fatalf("Position().Rows = %d, want %d", got, want)
	}
}

func TestStartProgramReadReturnsResultChannel(t *testing.T) {
	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 3)
	events := make(chan tcell.Event, 8)

	result := startProgramRead(pager, bytes.NewBufferString("alpha\nbeta\n"), events, nil)
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("startProgramRead result = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for startProgramRead result")
	}
}

func TestLoadProgramInputFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(sample) failed: %v", err)
	}

	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	snapshot, err := loadProgramInput(pager, nil, path, programStartup{line: 3})
	if err != nil {
		t.Fatalf("loadProgramInput(file) failed: %v", err)
	}
	if snapshot.eofVisible {
		t.Fatal("snapshot.eofVisible after loading tall file = true, want false before startup positioning")
	}
	if got, want := pager.Position().Row, 3; got != want {
		t.Fatalf("Position().Row = %d, want %d", got, want)
	}
}

func TestLoadProgramInputFromCachedStdin(t *testing.T) {
	loader := newProgramInputLoader(bytes.NewBufferString("alpha\nbeta\n"))
	reader, finish, err := loader.startStdinStream()
	if err != nil {
		t.Fatalf("loader.startStdinStream() failed: %v", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll(stream) failed: %v", err)
	}
	finish(nil, data)

	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	if _, err := loadProgramInput(pager, loader, "-", programStartup{query: "beta"}); err != nil {
		t.Fatalf("loadProgramInput(stdin) failed: %v", err)
	}
	state := pager.SearchState()
	if got, want := state.Query, "beta"; got != want {
		t.Fatalf("SearchState().Query = %q, want %q", got, want)
	}
}

func TestReloadProgramInputStreamsExplicitStdin(t *testing.T) {
	session := newProgramSession([]string{"-"}, programStartup{})
	loader := newProgramInputLoader(bytes.NewBufferString("alpha\nbeta\n"))
	events := make(chan tcell.Event, 8)
	var pager *goless.Pager
	var readResult chan error

	buildPager := func() *goless.Pager {
		next := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
		next.SetSize(20, 3)
		return next
	}

	loaded, _, err := reloadProgramInput(session, loader, &pager, buildPager, 20, 3, events, &readResult)
	if err != nil {
		t.Fatalf("reloadProgramInput(stream) failed: %v", err)
	}
	if loaded {
		t.Fatal("reloadProgramInput(stream) = loaded, want false")
	}
	if pager == nil {
		t.Fatal("reloadProgramInput(stream) left pager nil")
	}
	if readResult == nil {
		t.Fatal("reloadProgramInput(stream) left readResult nil")
	}
	select {
	case err := <-readResult:
		if err != nil {
			t.Fatalf("reloadProgramInput(stream) result = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for streamed stdin read")
	}

	if !loader.stdinLoaded {
		t.Fatal("loader.stdinLoaded = false after streamed read, want true")
	}
}

func TestReloadProgramInputBlocksWhileReading(t *testing.T) {
	session := newProgramSession([]string{"-"}, programStartup{})
	loader := newProgramInputLoader(bytes.NewBufferString("alpha\n"))
	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	readResult := make(chan error, 1)

	loaded, _, err := reloadProgramInput(session, loader, &pager, func() *goless.Pager {
		return goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	}, 20, 3, make(chan tcell.Event, 1), &readResult)
	if err == nil {
		t.Fatal("reloadProgramInput while readResult active = nil error, want error")
	}
	if loaded {
		t.Fatal("reloadProgramInput while readResult active = loaded, want false")
	}
	if got, want := err.Error(), "stdin still reading"; got != want {
		t.Fatalf("reloadProgramInput error = %q, want %q", got, want)
	}
}

func TestReloadProgramInputLoadsCachedStdinSynchronously(t *testing.T) {
	session := newProgramSession([]string{"-"}, programStartup{query: "beta"})
	loader := newProgramInputLoader(bytes.NewBufferString("alpha\nbeta\n"))
	reader, finish, err := loader.startStdinStream()
	if err != nil {
		t.Fatalf("loader.startStdinStream() failed: %v", err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll(stream) failed: %v", err)
	}
	finish(nil, data)

	var pager *goless.Pager
	var readResult chan error
	buildPager := func() *goless.Pager {
		return goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	}

	loaded, _, err := reloadProgramInput(session, loader, &pager, buildPager, 20, 3, make(chan tcell.Event, 1), &readResult)
	if err != nil {
		t.Fatalf("reloadProgramInput(cached) failed: %v", err)
	}
	if !loaded {
		t.Fatal("reloadProgramInput(cached) = not loaded, want true")
	}
	if pager == nil {
		t.Fatal("reloadProgramInput(cached) left pager nil")
	}
	if readResult != nil {
		t.Fatal("reloadProgramInput(cached) set readResult, want nil")
	}
	if got, want := pager.SearchState().Query, "beta"; got != want {
		t.Fatalf("SearchState().Query = %q, want %q", got, want)
	}
}
