// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"flag"
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
	"github.com/gdamore/tcell/v3/vt"
)

const (
	testInputAlphaBeta       = "alpha\nbeta\n"
	testInputOneTwo          = "one\ntwo\n"
	testInputOneTwoThree     = "one\ntwo\nthree\n"
	testInputOneTwoThreeFour = "one\ntwo\nthree\nfour\n"
	testInputOneToFive       = "one\ntwo\nthree\nfour\nfive\n"
	testInputTallTenLines    = "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"
	testSampleFileName       = "sample.txt"
	testStdinFixtureFileName = "stdin.txt"
)

func runProgramForTest(t *testing.T, args []string, stdin string) (string, error) {
	t.Helper()
	return runProgramForTestWithOptions(t, args, programRunTestOptions{stdin: stdin})
}

func runProgramForTestWithStreams(t *testing.T, args []string, stdin string) (string, string, error) {
	t.Helper()

	stdinPath := writeTempFile(t, t.TempDir(), testStdinFixtureFileName, stdin)
	stdinFile, err := os.Open(stdinPath)
	if err != nil {
		t.Fatalf("Open(stdin) failed: %v", err)
	}
	defer stdinFile.Close()

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe(stdout) failed: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe(stderr) failed: %v", err)
	}
	type streamResult struct {
		data []byte
		err  error
	}
	stdoutDone := make(chan streamResult, 1)
	stderrDone := make(chan streamResult, 1)
	go func() {
		data, err := io.ReadAll(stdoutReader)
		stdoutDone <- streamResult{data: data, err: err}
	}()
	go func() {
		data, err := io.ReadAll(stderrReader)
		stderrDone <- streamResult{data: data, err: err}
	}()

	oldArgs := os.Args
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	oldFlagOutput := flag.CommandLine.Output()
	os.Args = append([]string{"goless"}, args...)
	os.Stdin = stdinFile
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	flag.CommandLine.SetOutput(stderrWriter)

	runErr := run()

	flag.CommandLine.SetOutput(oldFlagOutput)
	os.Args = oldArgs
	os.Stdin = oldStdin
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("Close(stdout writer) failed: %v", err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("Close(stderr writer) failed: %v", err)
	}
	stdoutResult := <-stdoutDone
	if stdoutResult.err != nil {
		t.Fatalf("ReadAll(stdout) failed: %v", stdoutResult.err)
	}
	stderrResult := <-stderrDone
	if stderrResult.err != nil {
		t.Fatalf("ReadAll(stderr) failed: %v", stderrResult.err)
	}
	if err := stdoutReader.Close(); err != nil {
		t.Fatalf("Close(stdout reader) failed: %v", err)
	}
	if err := stderrReader.Close(); err != nil {
		t.Fatalf("Close(stderr reader) failed: %v", err)
	}
	return string(stdoutResult.data), string(stderrResult.data), runErr
}

type programRunTestOptions struct {
	stdin          string
	stdinTerminal  *bool
	stdoutTerminal *bool
	screenFactory  func(programQuitAtEOFPolicy) (tcell.Screen, error)
}

func runProgramForTestWithOptions(t *testing.T, args []string, opts programRunTestOptions) (string, error) {
	t.Helper()

	stdinPath := writeTempFile(t, t.TempDir(), testStdinFixtureFileName, opts.stdin)
	stdinFile, err := os.Open(stdinPath)
	if err != nil {
		t.Fatalf("Open(stdin) failed: %v", err)
	}
	defer stdinFile.Close()

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe(stdout) failed: %v", err)
	}
	type stdoutResult struct {
		data []byte
		err  error
	}
	stdoutDone := make(chan stdoutResult, 1)
	go func() {
		data, err := io.ReadAll(stdoutReader)
		stdoutDone <- stdoutResult{data: data, err: err}
	}()

	oldArgs := os.Args
	oldStdin := os.Stdin
	oldStdout := os.Stdout
	oldFlagOutput := flag.CommandLine.Output()
	oldScreenFactory := programScreenFactory
	oldStdinIsTerminal := programStdinIsTerminalFunc
	oldStdoutIsTerminal := programStdoutIsTerminalFunc
	os.Args = append([]string{"goless"}, args...)
	os.Stdin = stdinFile
	os.Stdout = stdoutWriter
	flag.CommandLine.SetOutput(stdoutWriter)
	if opts.screenFactory != nil {
		programScreenFactory = opts.screenFactory
	}
	if opts.stdinTerminal != nil {
		value := *opts.stdinTerminal
		programStdinIsTerminalFunc = func() bool { return value }
	}
	if opts.stdoutTerminal != nil {
		value := *opts.stdoutTerminal
		programStdoutIsTerminalFunc = func() bool { return value }
	}

	runErr := run()

	flag.CommandLine.SetOutput(oldFlagOutput)
	os.Args = oldArgs
	os.Stdin = oldStdin
	os.Stdout = oldStdout
	programScreenFactory = oldScreenFactory
	programStdinIsTerminalFunc = oldStdinIsTerminal
	programStdoutIsTerminalFunc = oldStdoutIsTerminal

	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("Close(stdout writer) failed: %v", err)
	}
	readResult := <-stdoutDone
	if readResult.err != nil {
		t.Fatalf("ReadAll(stdout) failed: %v", readResult.err)
	}
	if err := stdoutReader.Close(); err != nil {
		t.Fatalf("Close(stdout reader) failed: %v", err)
	}
	return string(readResult.data), runErr
}

func newMockProgramScreen(t *testing.T, width, height int) (vt.MockTerm, tcell.Screen) {
	t.Helper()

	term := vt.NewMockTerm(vt.MockOptSize{X: vt.Col(width), Y: vt.Row(height)})
	screen, err := tcell.NewTerminfoScreenFromTty(term)
	if err != nil {
		t.Fatalf("NewTerminfoScreenFromTty failed: %v", err)
	}
	return term, screen
}

func newTestPager(t *testing.T, width, height int, content string) *goless.Pager {
	t.Helper()

	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(width, height)
	if content != "" {
		if err := pager.AppendString(content); err != nil {
			t.Fatalf("AppendString failed: %v", err)
		}
		pager.Flush()
	}
	return pager
}

func writeTempFile(t *testing.T, dir, name, body string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) failed: %v", name, err)
	}
	return path
}

func boolPtr(v bool) *bool {
	return &v
}

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

func TestProgramPresetDefaultPretty(t *testing.T) {
	preset, err := programPreset("pretty")
	if err != nil {
		t.Fatalf("programPreset(pretty) failed: %v", err)
	}
	if got, want := preset.Theme.DefaultBG, goless.PrettyPreset.Theme.DefaultBG; got != want {
		t.Fatalf("programPreset(pretty).Theme.DefaultBG = %v, want %v", got, want)
	}
}

func TestProgramRenderMode(t *testing.T) {
	tests := []struct {
		name    string
		want    goless.RenderMode
		wantErr bool
	}{
		{name: "literal", want: goless.RenderLiteral},
		{name: "presentation", want: goless.RenderPresentation},
		{name: "hybrid", want: goless.RenderHybrid},
		{name: "", want: goless.RenderHybrid},
		{name: "bogus", want: goless.RenderHybrid, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := programRenderMode(tt.name)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("programRenderMode(%q) = nil error, want error", tt.name)
				}
			} else if err != nil {
				t.Fatalf("programRenderMode(%q) failed: %v", tt.name, err)
			}
			if got != tt.want {
				t.Fatalf("programRenderMode(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
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

func TestProgramExitWithoutScreen(t *testing.T) {
	if err := programExit(nil, programQuitAtEOFNever); err != nil {
		t.Fatalf("programExit(nil, never) failed: %v", err)
	}
	if err := programExit(nil, programQuitAtEOFWhenVisible); err != nil {
		t.Fatalf("programExit(nil, visible) failed: %v", err)
	}
}

func TestProgramExitClearsStatusLine(t *testing.T) {
	_, screen := newMockProgramScreen(t, 8, 3)
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init() failed: %v", err)
	}
	defer screen.Fini()

	for x, r := range "content!" {
		screen.SetContent(x, 1, r, nil, tcell.StyleDefault)
		screen.SetContent(x, 2, r, nil, tcell.StyleDefault)
	}
	screen.Show()

	if err := programExit(screen, programQuitAtEOFWhenVisible); err != nil {
		t.Fatalf("programExit(screen, visible) failed: %v", err)
	}

	for x := 0; x < 8; x++ {
		got, _, _ := screen.Get(x, 2)
		if got != " " {
			t.Fatalf("bottom row cell %d = %q, want blank", x, got)
		}
	}
	if got, _, _ := screen.Get(0, 1); got != "c" {
		t.Fatalf("row above status line = %q, want preserved content", got)
	}
}

func TestNewProgramScreen(t *testing.T) {
	oldDefaultFactory := programDefaultScreenFactory
	oldTerminfoFactory := programTerminfoScreenFactory
	defer func() {
		programDefaultScreenFactory = oldDefaultFactory
		programTerminfoScreenFactory = oldTerminfoFactory
	}()

	t.Run("visible eof uses terminfo without alt screen", func(t *testing.T) {
		programDefaultScreenFactory = func() (tcell.Screen, error) {
			t.Fatal("default screen factory should not be called")
			return nil, nil
		}
		programTerminfoScreenFactory = func(opts ...tcell.TerminfoScreenOption) (tcell.Screen, error) {
			if got, want := len(opts), 1; got != want {
				t.Fatalf("len(opts) = %d, want %d", got, want)
			}
			opt, ok := opts[0].(tcell.OptAltScreen)
			if !ok {
				t.Fatalf("opts[0] type = %T, want tcell.OptAltScreen", opts[0])
			}
			if bool(opt) {
				t.Fatal("OptAltScreen = true, want false")
			}
			return nil, nil
		}

		if _, err := newProgramScreen(programQuitAtEOFWhenVisible); err != nil {
			t.Fatalf("newProgramScreen(visible) failed: %v", err)
		}
	})

	t.Run("other policies use default screen", func(t *testing.T) {
		programTerminfoScreenFactory = func(opts ...tcell.TerminfoScreenOption) (tcell.Screen, error) {
			t.Fatal("terminfo screen factory should not be called")
			return nil, nil
		}
		programDefaultScreenFactory = func() (tcell.Screen, error) {
			return nil, nil
		}

		if _, err := newProgramScreen(programQuitAtEOFNever); err != nil {
			t.Fatalf("newProgramScreen(never) failed: %v", err)
		}
	})
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
	pager := newTestPager(t, 20, 5, testInputOneTwo)

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
	pager := newTestPager(t, 20, 5, testInputOneTwo)

	reloads := 0
	quit, err := handleProgramVisibleEOF(programQuitAtEOFWhenVisible, session, func() *goless.Pager { return pager }, true, func() (bool, error) {
		reloads++
		pager = newTestPager(t, 20, 5, testInputTallTenLines)
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
	pager := newTestPager(t, 20, 5, testInputOneTwo)

	reloads := 0
	quit, err := handleProgramVisibleEOF(programQuitAtEOFWhenVisible, session, func() *goless.Pager { return pager }, true, func() (bool, error) {
		reloads++
		pager = newTestPager(t, 20, 5, "")
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
	pager := newTestPager(t, 20, 3, testInputOneTwoThreeFour)

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
	pager := newTestPager(t, 20, 5, testInputOneTwo)
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
	pager := newTestPager(t, 20, 5, testInputOneTwo)

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

func TestHandleProgramPostInputVisibleEOFFromMouseScroll(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	pager := newTestPager(t, 20, 5, testInputOneTwo)

	quit, err := handleProgramPostInput(
		programQuitAtEOFWhenVisible,
		false,
		session,
		pager,
		programInputResult{
			handled: true,
			action:  goless.KeyActionScrollDown,
			context: goless.NormalKeyContext,
		},
		true,
		false,
		pager.Position(),
		func() (bool, error) { return true, nil },
	)
	if err != nil {
		t.Fatalf("handleProgramPostInput returned error: %v", err)
	}
	if !quit {
		t.Fatal("handleProgramPostInput(...) = false, want true")
	}
}

func TestHandleProgramPostInputQuitAtEOFUsesStationaryForwardAction(t *testing.T) {
	session := newProgramSession([]string{"one.txt", "two.txt"}, programStartup{})
	pager := newTestPager(t, 20, 5, testInputOneTwo)

	reloads := 0
	quit, err := handleProgramPostInput(
		programQuitAtEOFOnForwardEOF,
		false,
		session,
		pager,
		programInputResult{
			handled: true,
			action:  goless.KeyActionScrollDown,
			context: goless.NormalKeyContext,
		},
		true,
		false,
		pager.Position(),
		func() (bool, error) {
			reloads++
			return true, nil
		},
	)
	if err != nil {
		t.Fatalf("handleProgramPostInput returned error: %v", err)
	}
	if quit {
		t.Fatal("handleProgramPostInput(...) = true, want false")
	}
	if got, want := reloads, 1; got != want {
		t.Fatalf("reload count = %d, want %d", got, want)
	}
	if got, want := session.currentFile(), "two.txt"; got != want {
		t.Fatalf("currentFile() = %q, want %q", got, want)
	}
}

func TestHandleProgramPostInputIgnoresNonNormalContexts(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	pager := newTestPager(t, 20, 5, testInputOneTwo)

	for _, tt := range []struct {
		name    string
		context goless.KeyContext
	}{
		{name: "help", context: goless.HelpKeyContext},
		{name: "prompt", context: goless.PromptKeyContext},
	} {
		t.Run(tt.name, func(t *testing.T) {
			quit, err := handleProgramPostInput(
				programQuitAtEOFWhenVisible,
				false,
				session,
				pager,
				programInputResult{
					handled: true,
					action:  goless.KeyActionScrollDown,
					context: tt.context,
				},
				true,
				false,
				pager.Position(),
				func() (bool, error) {
					t.Fatal("reload should not be called outside normal context")
					return false, nil
				},
			)
			if err != nil {
				t.Fatalf("handleProgramPostInput returned error: %v", err)
			}
			if quit {
				t.Fatal("handleProgramPostInput(...) = true, want false")
			}
		})
	}
}

func TestHandleProgramPostInputSkipsEOFPoliciesForLicenseView(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	pager := newTestPager(t, 20, 5, testInputOneTwo)

	quit, err := handleProgramPostInput(
		programQuitAtEOFWhenVisible,
		true,
		session,
		pager,
		programInputResult{
			handled: true,
			action:  goless.KeyActionScrollDown,
			context: goless.NormalKeyContext,
		},
		true,
		false,
		pager.Position(),
		func() (bool, error) {
			t.Fatal("reload should not be called for license view")
			return false, nil
		},
	)
	if err != nil {
		t.Fatalf("handleProgramPostInput returned error: %v", err)
	}
	if quit {
		t.Fatal("handleProgramPostInput(...) = true, want false")
	}
}

func TestParseProgramFlagsCompatibilityAliases(t *testing.T) {
	opts, args, err := parseProgramFlags([]string{"-F", "-S", "-N", "-s", "-x", "4", "-I", "sample.txt"})
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

func TestParseProgramFlagsSecure(t *testing.T) {
	opts, args, err := parseProgramFlags([]string{"-secure", "sample.txt"})
	if err != nil {
		t.Fatalf("parseProgramFlags(-secure) failed: %v", err)
	}
	if !opts.secure {
		t.Fatal("parseProgramFlags(-secure) did not enable secure mode")
	}
	if got, want := len(args), 1; got != want {
		t.Fatalf("len(args) = %d, want %d", got, want)
	}
	if got, want := args[0], "sample.txt"; got != want {
		t.Fatalf("args[0] = %q, want %q", got, want)
	}
}

func TestParseProgramFlagsRejectsConflictingSearchCaseFlags(t *testing.T) {
	if _, _, err := parseProgramFlags([]string{"-i", "-I"}); err == nil {
		t.Fatal("parseProgramFlags(-i, -I) = nil error, want mutual exclusion error")
	}
}

func TestParseProgramFlagsRejectsNonPositiveTabWidth(t *testing.T) {
	if _, _, err := parseProgramFlags([]string{"-x", "0"}); err == nil {
		t.Fatal("parseProgramFlags(-x 0) = nil error, want invalid tab width error")
	}
}

func TestParseProgramFlagsVersion(t *testing.T) {
	opts, args, err := parseProgramFlags([]string{"-version"})
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

func TestParseProgramFlagsDefaultConfig(t *testing.T) {
	opts, args, err := parseProgramFlags([]string{"--default-config"})
	if err != nil {
		t.Fatalf("parseProgramFlags(--default-config) failed: %v", err)
	}
	if !opts.showDefaultConfig {
		t.Fatal("parseProgramFlags(--default-config) did not set showDefaultConfig")
	}
	if len(args) != 0 {
		t.Fatalf("len(args) after --default-config = %d, want 0", len(args))
	}
}

func TestRunDefaultConfig(t *testing.T) {
	output, err := runProgramForTest(t, []string{"--default-config"}, "")
	if err != nil {
		t.Fatalf("run(--default-config) failed: %v", err)
	}
	if got, want := output, "{\n  \"theme\": \"pretty\",\n  \"hidden\": false,\n  \"line-numbers\": false,\n  \"live-links\": false,\n  \"secure\": false\n}\n"; got != want {
		t.Fatalf("run(--default-config) output = %q, want %q", got, want)
	}
}

func TestRunVersion(t *testing.T) {
	output, err := runProgramForTest(t, []string{"--version"}, "")
	if err != nil {
		t.Fatalf("run(--version) failed: %v", err)
	}
	if !strings.HasPrefix(output, "goless version ") {
		t.Fatalf("run(--version) output = %q, want version prefix", output)
	}
}

func TestParseProgramFlagsHelp(t *testing.T) {
	opts, args, err := parseProgramFlags([]string{"--help"})
	if err != nil {
		t.Fatalf("parseProgramFlags(--help) failed: %v", err)
	}
	if !opts.showHelp {
		t.Fatal("parseProgramFlags(--help) did not set showHelp")
	}
	if len(args) != 0 {
		t.Fatalf("len(args) after --help = %d, want 0", len(args))
	}

	opts, args, err = parseProgramFlags([]string{"-?"})
	if err != nil {
		t.Fatalf("parseProgramFlags(-?) failed: %v", err)
	}
	if !opts.showHelp {
		t.Fatal("parseProgramFlags(-?) did not set showHelp")
	}
	if len(args) != 0 {
		t.Fatalf("len(args) after -? = %d, want 0", len(args))
	}

	opts, args, err = parseProgramFlags([]string{"-h"})
	if err != nil {
		t.Fatalf("parseProgramFlags(-h) failed: %v", err)
	}
	if !opts.showHelp {
		t.Fatal("parseProgramFlags(-h) did not set showHelp")
	}
	if len(args) != 0 {
		t.Fatalf("len(args) after -h = %d, want 0", len(args))
	}
}

func TestRunHelp(t *testing.T) {
	output, err := runProgramForTest(t, []string{"--help"}, "")
	if err != nil {
		t.Fatalf("run(--help) failed: %v", err)
	}
	if !strings.Contains(output, "Options:") {
		t.Fatalf("run(--help) output = %q, want options table", output)
	}
	if !strings.Contains(output, "Show help and exit.") {
		t.Fatalf("run(--help) output = %q, want descriptive text", output)
	}
}

func TestRunHelpWritesToStdout(t *testing.T) {
	stdout, stderr, err := runProgramForTestWithStreams(t, []string{"--help"}, "")
	if err != nil {
		t.Fatalf("run(--help) failed: %v", err)
	}
	if !strings.Contains(stdout, "Options:") {
		t.Fatalf("stdout = %q, want options table", stdout)
	}
	if !strings.Contains(stdout, "Show help and exit.") {
		t.Fatalf("stdout = %q, want descriptive text", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty output", stderr)
	}
}

func TestRunUnknownFlagShowsHelpHint(t *testing.T) {
	output, err := runProgramForTest(t, []string{"--hep"}, "")
	if err == nil {
		t.Fatal("run(--hep) = nil error, want error")
	}
	if got := output; !strings.Contains(got, "unknown option --hep; did you mean --help?") {
		t.Fatalf("run(--hep) output = %q, want suggestion", got)
	}
	if got := output; !strings.Contains(got, "See goless --help for assistance.") {
		t.Fatalf("run(--hep) output = %q, want help hint", got)
	}
	if got := output; strings.Contains(got, "Options:") {
		t.Fatalf("run(--hep) output = %q, do not want usage table", got)
	}
}

func TestRunUnknownFlagWritesToStderr(t *testing.T) {
	stdout, stderr, err := runProgramForTestWithStreams(t, []string{"--hep"}, "")
	if err == nil {
		t.Fatal("run(--hep) = nil error, want error")
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty output", stdout)
	}
	if !strings.Contains(stderr, "unknown option --hep; did you mean --help?") {
		t.Fatalf("stderr = %q, want suggestion", stderr)
	}
	if !strings.Contains(stderr, "See goless --help for assistance.") {
		t.Fatalf("stderr = %q, want help hint", stderr)
	}
	if strings.Contains(stderr, "Options:") {
		t.Fatalf("stderr = %q, do not want usage table", stderr)
	}
}

func TestParseProgramFlagsLicense(t *testing.T) {
	opts, args, err := parseProgramFlags([]string{"--license"})
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

func TestRunLicenseToPipe(t *testing.T) {
	output, err := runProgramForTest(t, []string{"--license"}, "")
	if err != nil {
		t.Fatalf("run(--license) failed: %v", err)
	}
	if !strings.Contains(output, "Apache License") {
		t.Fatalf("run(--license) output missing license heading: %q", output)
	}
}

func TestRunPassesThroughStdinToPipe(t *testing.T) {
	output, err := runProgramForTest(t, nil, "alpha\nbeta\n")
	if err != nil {
		t.Fatalf("run(stdin passthrough) failed: %v", err)
	}
	if got, want := output, "alpha\nbeta\n"; got != want {
		t.Fatalf("run(stdin passthrough) output = %q, want %q", got, want)
	}
}

func TestRunRejectsUnknownRenderMode(t *testing.T) {
	_, err := runProgramForTest(t, []string{"--render", "bogus"}, "")
	if err == nil {
		t.Fatal("run(--render bogus) = nil error, want error")
	}
	if got := err.Error(); !strings.Contains(got, "unknown render mode") {
		t.Fatalf("run(--render bogus) error = %q, want unknown render mode", got)
	}
}

func TestRunRejectsUnknownTheme(t *testing.T) {
	_, err := runProgramForTest(t, []string{"--theme", "bogus"}, "")
	if err == nil {
		t.Fatal("run(--theme bogus) = nil error, want error")
	}
	if got := err.Error(); !strings.Contains(got, "unknown theme") {
		t.Fatalf("run(--theme bogus) error = %q, want unknown theme", got)
	}
}

func TestRunPassesThroughFilesToPipe(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, testSampleFileName, testInputOneTwo)

	output, err := runProgramForTest(t, []string{path}, "")
	if err != nil {
		t.Fatalf("run(file passthrough) failed: %v", err)
	}
	if got, want := output, "one\ntwo\n"; got != want {
		t.Fatalf("run(file passthrough) output = %q, want %q", got, want)
	}
}

func TestRunRejectsTerminalStdinWithoutInput(t *testing.T) {
	_, err := runProgramForTestWithOptions(t, nil, programRunTestOptions{
		stdinTerminal:  boolPtr(true),
		stdoutTerminal: boolPtr(true),
	})
	if err == nil {
		t.Fatal("run(no args, terminal stdin) = nil error, want error")
	}
	if got, want := err.Error(), "stdin is a terminal; specify a file or pipe input"; got != want {
		t.Fatalf("run(no args, terminal stdin) error = %q, want %q", got, want)
	}
}

func TestRunInteractiveFileQuitsOnQ(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, testSampleFileName, testInputOneTwoThree)

	term, screen := newMockProgramScreen(t, 80, 24)
	output, err := runProgramForTestWithOptions(t, []string{path}, programRunTestOptions{
		stdoutTerminal: boolPtr(true),
		stdinTerminal:  boolPtr(false),
		screenFactory: func(programQuitAtEOFPolicy) (tcell.Screen, error) {
			go func() {
				deadline := time.Now().Add(2 * time.Second)
				for time.Now().Before(deadline) {
					if w, h := screen.Size(); w > 0 && h > 0 {
						term.KeyTap(vt.KeyQ)
						return
					}
					time.Sleep(5 * time.Millisecond)
				}
			}()
			return screen, nil
		},
	})
	if err != nil {
		t.Fatalf("run(interactive file) failed: %v", err)
	}
	if output != "" {
		t.Fatalf("run(interactive file) output = %q, want empty", output)
	}
}

func TestRunPassThroughReportsOpenError(t *testing.T) {
	_, err := runProgramForTest(t, []string{filepath.Join(t.TempDir(), "missing.txt")}, "")
	if err == nil {
		t.Fatal("run(missing file) = nil error, want error")
	}
}

func TestParseProgramFlagsLicenseExclusive(t *testing.T) {
	_, _, err := parseProgramFlags([]string{"--license", "-N"})
	if err == nil {
		t.Fatal("parseProgramFlags(--license -N) = nil error, want error")
	}
	if _, _, err := parseProgramFlags([]string{"--license", "file.txt"}); err == nil {
		t.Fatal("parseProgramFlags(--license file.txt) = nil error, want error")
	}
}

func TestParseProgramFlagsDefaultConfigExclusive(t *testing.T) {
	_, _, err := parseProgramFlags([]string{"--default-config", "-N"})
	if err == nil {
		t.Fatal("parseProgramFlags(--default-config -N) = nil error, want error")
	}
	if _, _, err := parseProgramFlags([]string{"--default-config", "file.txt"}); err == nil {
		t.Fatal("parseProgramFlags(--default-config file.txt) = nil error, want error")
	}
}

func TestProgramStandardStreamsUseNonTerminalFiles(t *testing.T) {
	stdinPath := writeTempFile(t, t.TempDir(), testStdinFixtureFileName, "alpha\n")
	stdinFile, err := os.Open(stdinPath)
	if err != nil {
		t.Fatalf("Open(stdin) failed: %v", err)
	}
	defer stdinFile.Close()

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe(stdout) failed: %v", err)
	}
	defer stdoutReader.Close()
	defer stdoutWriter.Close()

	oldStdin := os.Stdin
	oldStdout := os.Stdout
	defer func() {
		os.Stdin = oldStdin
		os.Stdout = oldStdout
	}()

	os.Stdin = stdinFile
	os.Stdout = stdoutWriter
	if stdinIsTerminal() {
		t.Fatal("stdinIsTerminal() = true for regular file, want false")
	}
	if stdoutIsTerminal() {
		t.Fatal("stdoutIsTerminal() = true for pipe, want false")
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
	loads := 0
	reloads := 0
	handler := session.commandHandler(func() error {
		loads++
		return nil
	}, func() error {
		reloads++
		return nil
	}, nil, nil)

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
	if got, want := loads, 1; got != want {
		t.Fatalf("load count after next = %d, want %d", got, want)
	}

	result = handler(goless.Command{Name: "next"})
	if !result.Handled || result.Message == "" {
		t.Fatalf("boundary next command result = %+v, want handled message", result)
	}
	if got, want := loads, 1; got != want {
		t.Fatalf("load count after boundary next = %d, want %d", got, want)
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
	if got, want := loads, 2; got != want {
		t.Fatalf("load count after prev = %d, want %d", got, want)
	}

	result = handler(goless.Command{Name: "reload"})
	if !result.Handled || result.Quit {
		t.Fatalf("reload command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "one.txt (1/2)"; got != want {
		t.Fatalf("reload command message = %q, want %q", got, want)
	}
	if got, want := reloads, 1; got != want {
		t.Fatalf("reload count after reload = %d, want %d", got, want)
	}

	result = handler(goless.Command{Name: "bogus"})
	if result.Handled || result.Quit {
		t.Fatalf("bogus command result = %+v, want unhandled", result)
	}
}

func TestDemoSessionAdditionalCommands(t *testing.T) {
	session := newProgramSession([]string{"one.txt", "two.txt", "three.txt"}, programStartup{})
	loads := 0
	handler := session.commandHandler(func() error {
		loads++
		return nil
	}, func() error {
		return nil
	}, nil, nil)

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
	if got, want := loads, 1; got != want {
		t.Fatalf("load count after last = %d, want %d", got, want)
	}

	result = handler(goless.Command{Name: "rewind"})
	if !result.Handled || result.Quit {
		t.Fatalf("rewind command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "one.txt (1/3)"; got != want {
		t.Fatalf("rewind command message = %q, want %q", got, want)
	}
	if got, want := loads, 2; got != want {
		t.Fatalf("load count after rewind = %d, want %d", got, want)
	}

	result = handler(goless.Command{Name: "x"})
	if !result.Handled || result.Quit {
		t.Fatalf("x command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "already at first file"; got != want {
		t.Fatalf("x command message = %q, want %q", got, want)
	}
	if got, want := loads, 2; got != want {
		t.Fatalf("load count after x = %d, want %d", got, want)
	}
}

func TestDemoSessionLicenseCommandShowsBundledLicense(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	var (
		title string
		body  string
	)
	handler := session.commandHandler(func() error {
		t.Fatal("load should not be called for :license")
		return nil
	}, func() error {
		t.Fatal("reload should not be called for :license")
		return nil
	}, func(gotTitle, gotBody string) {
		title = gotTitle
		body = gotBody
	}, nil)

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

func TestDemoSessionEditCommand(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	edits := 0
	handler := session.commandHandler(nil, nil, nil, func() error {
		edits++
		return nil
	})

	result := handler(goless.Command{Name: "v"})
	if !result.Handled || result.Quit {
		t.Fatalf("edit command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, ""; got != want {
		t.Fatalf("edit command message = %q, want empty", got)
	}
	if got, want := edits, 1; got != want {
		t.Fatalf("edit count = %d, want %d", got, want)
	}
}

func TestDemoSessionEditCommandReturnsError(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	handler := session.commandHandler(nil, nil, nil, func() error {
		return fmt.Errorf("EDITOR is not set")
	})

	result := handler(goless.Command{Name: "edit"})
	if !result.Handled || result.Quit {
		t.Fatalf("edit error result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "EDITOR is not set"; got != want {
		t.Fatalf("edit error message = %q, want %q", got, want)
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
	if err := pager.AppendString(testInputOneTwo); err != nil {
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

func TestHandleProgramReloadKey(t *testing.T) {
	var seen []string
	pager := goless.New(goless.Config{
		TabWidth:   4,
		WrapMode:   goless.NoWrap,
		ShowStatus: true,
		CommandHandler: func(cmd goless.Command) goless.CommandResult {
			seen = append(seen, cmd.Name)
			if cmd.Name == "reload" {
				return goless.CommandResult{Handled: true, Message: "reloaded"}
			}
			return goless.CommandResult{}
		},
	})
	pager.SetSize(20, 2)
	if err := pager.AppendString(testInputOneTwo); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if !handleProgramReloadKey(pager, tcell.NewEventKey(tcell.KeyRune, "R", tcell.ModNone)) {
		t.Fatal("handleProgramReloadKey(R) = false, want true")
	}
	if got, want := len(seen), 1; got != want {
		t.Fatalf("command count after R = %d, want %d", got, want)
	}
	if got, want := seen[0], "reload"; got != want {
		t.Fatalf("command after R = %q, want %q", got, want)
	}
	if handleProgramReloadKey(pager, tcell.NewEventKey(tcell.KeyRune, "R", tcell.ModAlt)) {
		t.Fatal("handleProgramReloadKey(Alt-R) = true, want false")
	}
	if handleProgramReloadKey(nil, tcell.NewEventKey(tcell.KeyRune, "R", tcell.ModNone)) {
		t.Fatal("handleProgramReloadKey(nil pager) = true, want false")
	}
	if handleProgramReloadKey(pager, tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone)) {
		t.Fatal("handleProgramReloadKey(x) = true, want false")
	}
}

func TestHandleProgramEditKey(t *testing.T) {
	var seen []string
	pager := goless.New(goless.Config{
		TabWidth:   4,
		WrapMode:   goless.NoWrap,
		ShowStatus: true,
		CommandHandler: func(cmd goless.Command) goless.CommandResult {
			seen = append(seen, cmd.Name)
			if cmd.Name == "v" {
				return goless.CommandResult{Handled: true}
			}
			return goless.CommandResult{}
		},
	})
	pager.SetSize(20, 2)
	if err := pager.AppendString("one\ntwo\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if !handleProgramEditKey(pager, tcell.NewEventKey(tcell.KeyRune, "v", tcell.ModNone)) {
		t.Fatal("handleProgramEditKey(v) = false, want true")
	}
	if got, want := len(seen), 1; got != want {
		t.Fatalf("command count after v = %d, want %d", got, want)
	}
	if got, want := seen[0], "v"; got != want {
		t.Fatalf("command after v = %q, want %q", got, want)
	}
	if handleProgramEditKey(pager, tcell.NewEventKey(tcell.KeyRune, "v", tcell.ModAlt)) {
		t.Fatal("handleProgramEditKey(Alt-v) = true, want false")
	}
}

func TestHandleProgramSaveKeyOpensSavePrompt(t *testing.T) {
	var seen []goless.Command
	pager := goless.New(goless.Config{
		TabWidth: 4,
		WrapMode: goless.NoWrap,
		CommandHandler: func(cmd goless.Command) goless.CommandResult {
			seen = append(seen, cmd)
			return goless.CommandResult{Handled: true}
		},
	})
	pager.SetSize(24, 2)
	if err := pager.AppendString("one\ntwo\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	if !handleProgramSaveKey(pager, tcell.NewEventKey(tcell.KeyRune, "s", tcell.ModNone), false) {
		t.Fatal("handleProgramSaveKey(s, secure=false) = false, want true")
	}
	for _, r := range "out.txt" {
		pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
	}
	pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))

	if got, want := len(seen), 1; got != want {
		t.Fatalf("save command count = %d, want %d", got, want)
	}
	if got, want := strings.Join(seen[0].Args, " "), "--file --ansi out.txt"; got != want {
		t.Fatalf("save command args = %q, want %q", got, want)
	}
}

func TestHandleProgramSaveKeyFallsBackToCommandInSecureMode(t *testing.T) {
	var seen []goless.Command
	pager := goless.New(goless.Config{
		TabWidth: 4,
		WrapMode: goless.NoWrap,
		CommandHandler: func(cmd goless.Command) goless.CommandResult {
			seen = append(seen, cmd)
			return goless.CommandResult{Handled: true, Message: "save disabled in secure mode"}
		},
	})
	pager.SetSize(20, 2)

	if !handleProgramSaveKey(pager, tcell.NewEventKey(tcell.KeyRune, "s", tcell.ModNone), true) {
		t.Fatal("handleProgramSaveKey(s, secure=true) = false, want true")
	}
	if got, want := len(seen), 1; got != want {
		t.Fatalf("secure save command count = %d, want %d", got, want)
	}
	if got, want := seen[0].Name, "save"; got != want {
		t.Fatalf("secure save command = %q, want %q", got, want)
	}
}

func TestShouldHandleProgramReloadKey(t *testing.T) {
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
			if got := shouldHandleProgramReloadKey(tt.result); got != tt.want {
				t.Fatalf("shouldHandleProgramReloadKey(%+v) = %v, want %v", tt.result, got, tt.want)
			}
		})
	}
}

func TestParseProgramSaveCommand(t *testing.T) {
	req, err := parseProgramSaveCommand(goless.Command{
		Name: "save",
		Args: []string{"--view", "--mono", "tmp", "out.txt"},
	})
	if err != nil {
		t.Fatalf("parseProgramSaveCommand(...) failed: %v", err)
	}
	if got, want := req.path, "tmp out.txt"; got != want {
		t.Fatalf("save path = %q, want %q", got, want)
	}
	if got, want := req.export.Scope, goless.ExportViewport; got != want {
		t.Fatalf("save scope = %v, want %v", got, want)
	}
	if got, want := req.export.Format, goless.ExportPlain; got != want {
		t.Fatalf("save format = %v, want %v", got, want)
	}
}

func TestParseProgramSaveCommandDefaults(t *testing.T) {
	req, err := parseProgramSaveCommand(goless.Command{
		Name: "save",
		Args: []string{"out.txt"},
	})
	if err != nil {
		t.Fatalf("parseProgramSaveCommand(defaults) failed: %v", err)
	}
	if got, want := req.path, "out.txt"; got != want {
		t.Fatalf("save path = %q, want %q", got, want)
	}
	if got, want := req.export.Scope, goless.ExportCurrentContent; got != want {
		t.Fatalf("default save scope = %v, want %v", got, want)
	}
	if got, want := req.export.Format, goless.ExportANSI; got != want {
		t.Fatalf("default save format = %v, want %v", got, want)
	}
}

func TestParseProgramSaveCommandRequiresPath(t *testing.T) {
	_, err := parseProgramSaveCommand(goless.Command{Name: "save"})
	if !errors.Is(err, errProgramSavePath) {
		t.Fatalf("parseProgramSaveCommand(no path) error = %v, want %v", err, errProgramSavePath)
	}
}

func TestSaveProgramContentWritesRequestedExport(t *testing.T) {
	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap})
	pager.SetSize(20, 2)
	if err := pager.AppendString("\x1b[31mred\x1b[0m\nplain\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	path := filepath.Join(t.TempDir(), "saved.txt")
	gotPath, err := saveProgramContent(pager, goless.Command{
		Name: "save",
		Args: []string{"--mono", path},
	}, false)
	if err != nil {
		t.Fatalf("saveProgramContent(...) failed: %v", err)
	}
	if got, want := gotPath, path; got != want {
		t.Fatalf("saveProgramContent path = %q, want %q", got, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(saved.txt) failed: %v", err)
	}
	if got, want := string(data), "red\nplain\n"; got != want {
		t.Fatalf("saved file = %q, want %q", got, want)
	}
}

func TestSaveProgramContentSecureMode(t *testing.T) {
	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap})
	_, err := saveProgramContent(pager, goless.Command{
		Name: "save",
		Args: []string{"out.txt"},
	}, true)
	if !errors.Is(err, errProgramSaveDisabled) {
		t.Fatalf("saveProgramContent(..., secure=true) error = %v, want %v", err, errProgramSaveDisabled)
	}
}

func TestSaveProgramContentRequiresConfirmationBeforeOverwrite(t *testing.T) {
	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap})
	pager.SetSize(20, 2)
	if err := pager.AppendString("new\ncontent\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	path := filepath.Join(t.TempDir(), "saved.txt")
	if err := os.WriteFile(path, []byte("old\ncontent\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(existing) failed: %v", err)
	}
	cmd := goless.Command{Name: "save", Args: []string{path}}

	_, err := saveProgramContent(pager, cmd, false)
	if !errors.Is(err, errProgramSaveOverwrite) {
		t.Fatalf("first overwrite save error = %v, want %v", err, errProgramSaveOverwrite)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(existing) failed: %v", err)
	}
	if got, want := string(data), "old\ncontent\n"; got != want {
		t.Fatalf("file after unconfirmed overwrite = %q, want %q", got, want)
	}

	cmd.Confirmed = true
	gotPath, err := saveProgramContent(pager, cmd, false)
	if err != nil {
		t.Fatalf("confirmed overwrite save failed: %v", err)
	}
	if got, want := gotPath, path; got != want {
		t.Fatalf("confirmed overwrite path = %q, want %q", got, want)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(overwritten) failed: %v", err)
	}
	if got, want := string(data), "new\ncontent\n"; got != want {
		t.Fatalf("file after confirmed overwrite = %q, want %q", got, want)
	}
}

func TestSaveProgramContentRemovesTempFileOnReplaceError(t *testing.T) {
	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap})
	pager.SetSize(20, 2)
	if err := pager.AppendString("new\ncontent\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	root := t.TempDir()
	path := filepath.Join(root, "existing-dir")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("Mkdir(existing-dir) failed: %v", err)
	}

	_, err := saveProgramContent(pager, goless.Command{
		Name: "save",
		Args: []string{path},
	}, false)
	if err == nil {
		t.Fatal("saveProgramContent(directory path) = nil error, want failure")
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir(root) failed: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".goless-save-") {
			t.Fatalf("leftover temp file %q, want cleanup on save failure", entry.Name())
		}
	}
}

func TestProgramSaveHandlerNilReceiver(t *testing.T) {
	var handler *programSaveHandler
	_, err := handler.save(goless.Command{Name: "save", Args: []string{"out.txt"}})
	if !errors.Is(err, errProgramSaveUnavailable) {
		t.Fatalf("nil programSaveHandler save error = %v, want %v", err, errProgramSaveUnavailable)
	}
}

func TestShouldKeepProgramSavePrompt(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "disabled", err: errProgramSaveDisabled, want: false},
		{name: "unavailable", err: errProgramSaveUnavailable, want: false},
		{name: "overwrite", err: errProgramSaveOverwrite, want: false},
		{name: "other", err: errors.New("permission denied"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldKeepProgramSavePrompt(tt.err); got != tt.want {
				t.Fatalf("shouldKeepProgramSavePrompt(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestDemoSessionSaveCommandRequestsConfirmationPrompt(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	handler := session.commandHandlerWithSave(nil, nil, nil, nil, func(cmd goless.Command) (string, error) {
		return "", errProgramSaveOverwrite
	})

	result := handler(goless.Command{Name: "save", Args: []string{"out.txt"}})
	if !result.Handled || !result.KeepPrompt {
		t.Fatalf("save overwrite result = %+v, want handled keep-prompt result", result)
	}
	if got, want := result.PromptText, "File already exists. Overwrite? (yes/no) "; got != want {
		t.Fatalf("overwrite prompt text = %q, want %q", got, want)
	}
}

func TestShouldHandleProgramEditKey(t *testing.T) {
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
			if got := shouldHandleProgramEditKey(tt.result); got != tt.want {
				t.Fatalf("shouldHandleProgramEditKey(%+v) = %v, want %v", tt.result, got, tt.want)
			}
		})
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
	pager := newTestPager(t, 20, 3, testInputOneToFive)

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

	pager = newTestPager(t, 20, 3, testInputOneToFive)
	pager.Follow()
	if got := updateProgramQuitIfOneScreenArm(true, pager); got {
		t.Fatal("updateProgramQuitIfOneScreenArm(true, following pager) = true, want false")
	}
}

func TestDemoSessionLabelsExplicitStdin(t *testing.T) {
	session := newProgramSession([]string{"one.txt", "-", "three.txt"}, programStartup{})
	loads := 0
	handler := session.commandHandler(func() error {
		loads++
		return nil
	}, func() error {
		return nil
	}, nil, nil)

	result := handler(goless.Command{Name: "next"})
	if !result.Handled || result.Quit {
		t.Fatalf("next command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "stdin (2/3)"; got != want {
		t.Fatalf("next command message = %q, want %q", got, want)
	}
	if got, want := loads, 1; got != want {
		t.Fatalf("load count after next = %d, want %d", got, want)
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
	}, func() error {
		return fmt.Errorf("stdin still reading")
	}, nil, nil)

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
	}, func() error {
		return fmt.Errorf("reload failed")
	}, nil, nil)

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
	}, func() error {
		return fmt.Errorf("reload failed")
	}, nil, nil)

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

func TestDemoSessionCommandHandlerReloadUsesReloadHook(t *testing.T) {
	session := newProgramSession([]string{"one.txt", "two.txt"}, programStartup{})
	loads := 0
	reloads := 0
	handler := session.commandHandler(func() error {
		loads++
		return nil
	}, func() error {
		reloads++
		return nil
	}, nil, nil)

	result := handler(goless.Command{Name: "reload"})
	if !result.Handled || result.Quit {
		t.Fatalf("reload command result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "one.txt (1/2)"; got != want {
		t.Fatalf("reload command message = %q, want %q", got, want)
	}
	if got, want := loads, 0; got != want {
		t.Fatalf("load count after reload = %d, want %d", got, want)
	}
	if got, want := reloads, 1; got != want {
		t.Fatalf("reload count after reload = %d, want %d", got, want)
	}
	if got, want := session.currentFile(), "one.txt"; got != want {
		t.Fatalf("current file after reload = %q, want %q", got, want)
	}
}

func TestDemoSessionCommandHandlerReloadUnavailable(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	handler := session.commandHandler(func() error {
		t.Fatal("load should not be called when reload is unavailable")
		return nil
	}, nil, nil, nil)

	result := handler(goless.Command{Name: "reload"})
	if !result.Handled || result.Quit {
		t.Fatalf("reload unavailable result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "file reload unavailable"; got != want {
		t.Fatalf("reload unavailable message = %q, want %q", got, want)
	}
}

func TestDemoSessionCommandHandlerNextUnavailable(t *testing.T) {
	session := newProgramSession([]string{"one.txt", "two.txt"}, programStartup{})
	handler := session.commandHandler(nil, func() error {
		t.Fatal("reload should not be called when load is unavailable")
		return nil
	}, nil, nil)

	result := handler(goless.Command{Name: "next"})
	if !result.Handled || result.Quit {
		t.Fatalf("next unavailable result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "file reload unavailable"; got != want {
		t.Fatalf("next unavailable message = %q, want %q", got, want)
	}
	if got, want := session.currentFile(), "one.txt"; got != want {
		t.Fatalf("current file after unavailable next = %q, want %q", got, want)
	}
}

func TestDemoSessionCommandHandlerNavigationUnavailable(t *testing.T) {
	tests := []struct {
		name     string
		prepare  func(*programSession)
		command  string
		wantFile string
	}{
		{
			name: "prev unavailable",
			prepare: func(session *programSession) {
				session.last()
			},
			command:  "prev",
			wantFile: "two.txt",
		},
		{
			name: "first unavailable",
			prepare: func(session *programSession) {
				session.last()
			},
			command:  "first",
			wantFile: "two.txt",
		},
		{
			name:     "last unavailable",
			prepare:  func(session *programSession) {},
			command:  "last",
			wantFile: "one.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := newProgramSession([]string{"one.txt", "two.txt"}, programStartup{})
			tt.prepare(session)
			handler := session.commandHandler(nil, func() error {
				t.Fatal("reload should not be called when load is unavailable")
				return nil
			}, nil, nil)

			result := handler(goless.Command{Name: tt.command})
			if !result.Handled || result.Quit {
				t.Fatalf("%s result = %+v, want handled non-quit", tt.command, result)
			}
			if got, want := result.Message, "file reload unavailable"; got != want {
				t.Fatalf("%s unavailable message = %q, want %q", tt.command, got, want)
			}
			if got, want := session.currentFile(), tt.wantFile; got != want {
				t.Fatalf("current file after unavailable %s = %q, want %q", tt.command, got, want)
			}
		})
	}
}

func TestDemoSessionCommandHandlerReloadFailureReturnsMessage(t *testing.T) {
	session := newProgramSession([]string{"one.txt"}, programStartup{})
	handler := session.commandHandler(func() error {
		t.Fatal("load should not be called for reload failure")
		return nil
	}, func() error {
		return fmt.Errorf("reload failed")
	}, nil, nil)

	result := handler(goless.Command{Name: "reload"})
	if !result.Handled || result.Quit {
		t.Fatalf("reload failure result = %+v, want handled non-quit", result)
	}
	if got, want := result.Message, "reload failed"; got != want {
		t.Fatalf("reload failure message = %q, want %q", got, want)
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
	if err := pager.AppendString(testInputOneTwoThreeFour); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()

	applyStartupCommand(pager, programStartup{line: 3})
	if got, want := pager.Position().Row, 3; got != want {
		t.Fatalf("Position().Row = %d, want %d", got, want)
	}

	pager = newTestPager(t, 20, 2, "alpha\nbeta\ngamma\nbeta\n")

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

func TestEditProgramCurrentFileUsesCurrentLineAndReloads(t *testing.T) {
	previous := programEditorLauncher
	t.Cleanup(func() {
		programEditorLauncher = previous
	})

	var (
		gotLine int
		gotPath string
	)
	programEditorLauncher = func(line int, path string) error {
		gotLine = line
		gotPath = path
		return nil
	}

	term := vt.NewMockTerm(vt.MockOptSize{X: 80, Y: 24})
	screen, err := tcell.NewTerminfoScreenFromTty(term)
	if err != nil {
		t.Fatalf("NewTerminfoScreenFromTty failed: %v", err)
	}
	if err := screen.Init(); err != nil {
		t.Fatalf("screen.Init failed: %v", err)
	}
	defer screen.Fini()

	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 3)
	if err := pager.AppendString("one\ntwo\nthree\nfour\n"); err != nil {
		t.Fatalf("AppendString failed: %v", err)
	}
	pager.Flush()
	pager.JumpToLine(3)

	path := filepath.Join(t.TempDir(), "sample.txt")
	session := newProgramSession([]string{path}, programStartup{})
	reloads := 0
	if err := editProgramCurrentFile(screen, pager, session, false, true, func() error {
		reloads++
		return nil
	}); err != nil {
		t.Fatalf("editProgramCurrentFile(...) failed: %v", err)
	}
	if got, want := gotLine, 3; got != want {
		t.Fatalf("editor line = %d, want %d", got, want)
	}
	if got, want := gotPath, path; got != want {
		t.Fatalf("editor path = %q, want %q", got, want)
	}
	if got, want := reloads, 1; got != want {
		t.Fatalf("reload count = %d, want %d", got, want)
	}
}

func TestEditProgramCurrentFileSecureMode(t *testing.T) {
	session := newProgramSession([]string{"sample.txt"}, programStartup{})
	err := editProgramCurrentFile(nil, nil, session, true, true, nil)
	if err == nil {
		t.Fatal("editProgramCurrentFile(..., secure=true, ...) = nil error, want error")
	}
	if got, want := err.Error(), "editor disabled in secure mode"; got != want {
		t.Fatalf("secure mode error = %q, want %q", got, want)
	}
}

func TestEditProgramCurrentFileRejectsCurrentInputWithoutFile(t *testing.T) {
	for _, tt := range []struct {
		name    string
		session *programSession
	}{
		{name: "nil session"},
		{name: "stdin session", session: newProgramSession([]string{"-"}, programStartup{})},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := editProgramCurrentFile(nil, nil, tt.session, false, true, nil)
			if err == nil {
				t.Fatal("editProgramCurrentFile(...) = nil error, want error")
			}
			if got, want := err.Error(), "editor unavailable for current input"; got != want {
				t.Fatalf("error = %q, want %q", got, want)
			}
		})
	}
}

func TestSplitProgramEditor(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		want    []string
		wantErr bool
	}{
		{name: "simple", spec: "vim", want: []string{"vim"}},
		{name: "with args", spec: "vim -u NONE", want: []string{"vim", "-u", "NONE"}},
		{name: "quoted path", spec: "\"/Applications/Vim MacVim.app/Contents/bin/mvim\" -f", want: []string{"/Applications/Vim MacVim.app/Contents/bin/mvim", "-f"}},
		{name: "quoted windows path", spec: "\"C:\\Program Files\\Vim\\vim.exe\" -f", want: []string{"C:\\Program Files\\Vim\\vim.exe", "-f"}},
		{name: "single quotes", spec: "'/opt/editor bin/vim' -u NONE", want: []string{"/opt/editor bin/vim", "-u", "NONE"}},
		{name: "unterminated quote", spec: "\"vim", wantErr: true},
		{name: "literal trailing backslash", spec: "vim\\", want: []string{"vim\\"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := splitProgramEditor(tt.spec)
			if tt.wantErr {
				if err == nil {
					t.Fatal("splitProgramEditor(...) = nil error, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("splitProgramEditor(...) failed: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("len(args) = %d, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("args[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestLaunchProgramEditorDefaultsToVi(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "vi.args")
	script := filepath.Join(dir, "vi")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$GOLESS_TEST_VI_ARGS\"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(vi) failed: %v", err)
	}
	batch := filepath.Join(dir, "vi.bat")
	if err := os.WriteFile(batch, []byte("@echo off\r\n> \"%GOLESS_TEST_VI_ARGS%\" (\r\nfor %%A in (%*) do echo %%~A\r\n)\r\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(vi.bat) failed: %v", err)
	}

	previous := os.Getenv("EDITOR")
	if err := os.Unsetenv("EDITOR"); err != nil {
		t.Fatalf("Unsetenv(EDITOR) failed: %v", err)
	}
	previousPath := os.Getenv("PATH")
	previousArgsPath := os.Getenv("GOLESS_TEST_VI_ARGS")
	if err := os.Setenv("GOLESS_TEST_VI_ARGS", argsFile); err != nil {
		t.Fatalf("Setenv(GOLESS_TEST_VI_ARGS) failed: %v", err)
	}
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+previousPath); err != nil {
		t.Fatalf("Setenv(PATH) failed: %v", err)
	}
	t.Cleanup(func() {
		if previous == "" {
			_ = os.Unsetenv("EDITOR")
		} else {
			_ = os.Setenv("EDITOR", previous)
		}
		if previousArgsPath == "" {
			_ = os.Unsetenv("GOLESS_TEST_VI_ARGS")
		} else {
			_ = os.Setenv("GOLESS_TEST_VI_ARGS", previousArgsPath)
		}
		_ = os.Setenv("PATH", previousPath)
	})

	if err := launchProgramEditor(3, "sample.txt"); err != nil {
		t.Fatalf("launchProgramEditor(...) failed: %v", err)
	}
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("ReadFile(vi.args) failed: %v", err)
	}
	if got, want := strings.ReplaceAll(string(data), "\r\n", "\n"), "+3\nsample.txt\n"; got != want {
		t.Fatalf("vi args = %q, want %q", got, want)
	}
}

func TestPassThroughProgramInputsFromStdin(t *testing.T) {
	var out bytes.Buffer
	if err := passThroughProgramInputs(&out, bytes.NewBufferString(testInputAlphaBeta), nil); err != nil {
		t.Fatalf("passThroughProgramInputs(stdin) failed: %v", err)
	}
	if got, want := out.String(), testInputAlphaBeta; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestPassThroughProgramInputsFromFiles(t *testing.T) {
	dir := t.TempDir()
	first := writeTempFile(t, dir, "first.txt", "first\n")
	second := writeTempFile(t, dir, "second.txt", "second\n")

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
	first := writeTempFile(t, dir, "first.txt", "first\n")
	second := writeTempFile(t, dir, "second.txt", "second\n")

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
	loader := newProgramInputLoader(bytes.NewBufferString(testInputAlphaBeta))

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

	if got, want := string(firstData), testInputAlphaBeta; got != want {
		t.Fatalf("first stdin read = %q, want %q", got, want)
	}
	if got, want := string(secondData), testInputAlphaBeta; got != want {
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
	loader := newProgramInputLoader(bytes.NewBufferString(testInputAlphaBeta))
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

	if got, want := string(data), testInputAlphaBeta; got != want {
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
	if got, want := string(cachedData), testInputAlphaBeta; got != want {
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

	go readIntoPagerWithAfter(pager, bytes.NewBufferString(testInputAlphaBeta), events, result, func(err error) {
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

	result := startProgramRead(pager, bytes.NewBufferString(testInputAlphaBeta), events, nil)
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
	path := writeTempFile(t, dir, testSampleFileName, testInputOneTwoThree)

	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	snapshot, _, err := loadProgramInput(pager, nil, path, programStartup{line: 3})
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
	loader := newProgramInputLoader(bytes.NewBufferString(testInputAlphaBeta))
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
	if _, _, err := loadProgramInput(pager, loader, "-", programStartup{query: "beta"}); err != nil {
		t.Fatalf("loadProgramInput(stdin) failed: %v", err)
	}
	state := pager.SearchState()
	if got, want := state.Query, "beta"; got != want {
		t.Fatalf("SearchState().Query = %q, want %q", got, want)
	}
}

func TestReloadProgramInputInPlacePreservesViewport(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, testSampleFileName, "one\ntwo\nthree\nfour\n0123456789abcdef\n")

	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(8, 4)
	if _, _, err := loadProgramInput(pager, nil, path, programStartup{}); err != nil {
		t.Fatalf("loadProgramInput(file) failed: %v", err)
	}
	pager.ScrollDown(2)
	pager.ScrollRight(4)
	before := pager.Position()

	if err := os.WriteFile(path, []byte("uno\ndos\ntres\ncuatro\nABCDEFGHIJKLMNOP\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(reload) failed: %v", err)
	}
	if err := reloadProgramInputInPlace(pager, nil, path); err != nil {
		t.Fatalf("reloadProgramInputInPlace failed: %v", err)
	}

	after := pager.Position()
	if got, want := after.Row, before.Row; got != want {
		t.Fatalf("Position().Row after in-place reload = %d, want %d", got, want)
	}
	if got, want := after.Column, before.Column; got != want {
		t.Fatalf("Position().Column after in-place reload = %d, want %d", got, want)
	}
}

func TestReloadProgramInputInPlaceUsesCachedStdin(t *testing.T) {
	loader := newProgramInputLoader(bytes.NewBufferString(testInputAlphaBeta))
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
	if _, _, err := loadProgramInput(pager, loader, "-", programStartup{}); err != nil {
		t.Fatalf("loadProgramInput(stdin) failed: %v", err)
	}
	pager.ScrollDown(1)

	if err := reloadProgramInputInPlace(pager, loader, "-"); err != nil {
		t.Fatalf("reloadProgramInputInPlace(stdin) failed: %v", err)
	}
	if got, want := pager.Len(), int64(len(testInputAlphaBeta)); got != want {
		t.Fatalf("Len after stdin in-place reload = %d, want %d", got, want)
	}
}

func TestReloadProgramInputInPlaceErrorLeavesPagerUnchanged(t *testing.T) {
	pager := newTestPager(t, 20, 2, testInputAlphaBeta)

	err := reloadProgramInputInPlace(pager, nil, filepath.Join(t.TempDir(), "missing.txt"))
	if err == nil {
		t.Fatal("reloadProgramInputInPlace(missing file) = nil error, want error")
	}
	if got, want := pager.Len(), int64(len(testInputAlphaBeta)); got != want {
		t.Fatalf("Len after failed in-place reload = %d, want %d", got, want)
	}
}

func TestReloadProgramInputInPlaceUnavailableLoaderLeavesPagerUnchanged(t *testing.T) {
	pager := newTestPager(t, 20, 2, testInputAlphaBeta)

	err := reloadProgramInputInPlace(pager, newProgramInputLoader(nil), "-")
	if err == nil {
		t.Fatal("reloadProgramInputInPlace(unavailable stdin) = nil error, want error")
	}
	if got, want := pager.Len(), int64(len(testInputAlphaBeta)); got != want {
		t.Fatalf("Len after unavailable stdin reload = %d, want %d", got, want)
	}
}

func TestRunProgramCommandRejectsNilPagerAndEmptyCommand(t *testing.T) {
	if runProgramCommand(nil, "reload") {
		t.Fatal("runProgramCommand(nil, reload) = true, want false")
	}

	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 2)
	if runProgramCommand(pager, "") {
		t.Fatal("runProgramCommand(pager, empty) = true, want false")
	}
}

func TestReloadProgramInputStreamsExplicitStdin(t *testing.T) {
	session := newProgramSession([]string{"-"}, programStartup{})
	loader := newProgramInputLoader(bytes.NewBufferString(testInputAlphaBeta))
	events := make(chan tcell.Event, 8)
	var pager *goless.Pager
	var readResult chan error

	buildPager := func() *goless.Pager {
		next := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
		next.SetSize(20, 3)
		return next
	}

	loaded, _, _, err := reloadProgramInput(session, loader, &pager, buildPager, 20, 3, events, &readResult)
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

	loaded, _, _, err := reloadProgramInput(session, loader, &pager, func() *goless.Pager {
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
	loader := newProgramInputLoader(bytes.NewBufferString(testInputAlphaBeta))
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

	loaded, _, _, err := reloadProgramInput(session, loader, &pager, buildPager, 20, 3, make(chan tcell.Event, 1), &readResult)
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

func TestSyncProgramFileFollowAppendsNewBytes(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, testSampleFileName, testInputOneTwo)

	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 3)
	if _, _, err := loadProgramInput(pager, nil, path, programStartup{}); err != nil {
		t.Fatalf("loadProgramInput(file) failed: %v", err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile(sample) failed: %v", err)
	}
	if _, err := file.WriteString("three\n"); err != nil {
		file.Close()
		t.Fatalf("WriteString(append) failed: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(append file) failed: %v", err)
	}

	var previous os.FileInfo
	changed, err := syncProgramFileFollow(pager, path, &previous)
	if err != nil {
		t.Fatalf("syncProgramFileFollow failed: %v", err)
	}
	if !changed {
		t.Fatal("syncProgramFileFollow(...) = unchanged, want changed")
	}
	if got, want := pager.Len(), int64(len(testInputOneTwoThree)); got != want {
		t.Fatalf("Len after append = %d, want %d", got, want)
	}
	if !pager.SearchForward("three") {
		t.Fatal("SearchForward(three) = false, want true after append")
	}
	if got, want := pager.Position().Rows, 3; got != want {
		t.Fatalf("Position().Rows after append = %d, want %d", got, want)
	}
}

func TestSyncProgramFileFollowReloadsTruncatedFile(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, testSampleFileName, testInputOneTwoThree)

	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 3)
	if _, _, err := loadProgramInput(pager, nil, path, programStartup{}); err != nil {
		t.Fatalf("loadProgramInput(file) failed: %v", err)
	}

	if err := os.WriteFile(path, []byte("fresh\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(truncated) failed: %v", err)
	}

	var previous os.FileInfo
	changed, err := syncProgramFileFollow(pager, path, &previous)
	if err != nil {
		t.Fatalf("syncProgramFileFollow failed: %v", err)
	}
	if !changed {
		t.Fatal("syncProgramFileFollow(...) = unchanged, want changed")
	}
	if got, want := pager.Len(), int64(len("fresh\n")); got != want {
		t.Fatalf("Len after truncate = %d, want %d", got, want)
	}
	if !pager.SearchForward("fresh") {
		t.Fatal("SearchForward(fresh) = false, want true after truncate reload")
	}
	if pager.SearchForward("three") {
		t.Fatal("SearchForward(three) = true, want false after truncate reload")
	}
}

func TestSyncProgramFileFollowReloadsReplacedFileWithSameSize(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, testSampleFileName, testInputOneTwo)

	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 3)
	_, initialInfo, err := loadProgramInput(pager, nil, path, programStartup{})
	if err != nil {
		t.Fatalf("loadProgramInput(file) failed: %v", err)
	}
	var previous os.FileInfo = initialInfo

	replacement := filepath.Join(dir, "replacement.txt")
	if err := os.WriteFile(replacement, []byte("uno\ndos\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(replacement) failed: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove(sample) failed: %v", err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatalf("Rename(replacement) failed: %v", err)
	}

	changed, err := syncProgramFileFollow(pager, path, &previous)
	if err != nil {
		t.Fatalf("syncProgramFileFollow failed: %v", err)
	}
	if !changed {
		t.Fatal("syncProgramFileFollow(...) = unchanged, want changed after replacement")
	}
	if !pager.SearchForward("uno") {
		t.Fatal("SearchForward(uno) = false, want true after replacement reload")
	}
	if pager.SearchForward("one") {
		t.Fatal("SearchForward(one) = true, want false after replacement reload")
	}
}

func TestSyncProgramFileFollowReloadsReplacedFileWithLargerSize(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, testSampleFileName, testInputOneTwo)

	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 3)
	_, initialInfo, err := loadProgramInput(pager, nil, path, programStartup{})
	if err != nil {
		t.Fatalf("loadProgramInput(file) failed: %v", err)
	}
	var previous os.FileInfo = initialInfo

	replacement := filepath.Join(dir, "replacement.txt")
	if err := os.WriteFile(replacement, []byte("alpha\nbeta\nthree\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(replacement) failed: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove(sample) failed: %v", err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatalf("Rename(replacement) failed: %v", err)
	}

	changed, err := syncProgramFileFollow(pager, path, &previous)
	if err != nil {
		t.Fatalf("syncProgramFileFollow failed: %v", err)
	}
	if !changed {
		t.Fatal("syncProgramFileFollow(...) = unchanged, want changed after larger replacement")
	}
	if !pager.SearchForward("alpha") {
		t.Fatal("SearchForward(alpha) = false, want true after replacement reload")
	}
	if !pager.SearchForward("three") {
		t.Fatal("SearchForward(three) = false, want true after replacement reload")
	}
	if pager.SearchForward("one") {
		t.Fatal("SearchForward(one) = true, want false after replacement reload")
	}
}

func TestStartProgramFileFollowPollsForAppends(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, testSampleFileName, testInputOneTwo)

	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 3)
	if _, _, err := loadProgramInput(pager, nil, path, programStartup{}); err != nil {
		t.Fatalf("loadProgramInput(file) failed: %v", err)
	}

	events := make(chan tcell.Event, 8)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(sample) failed: %v", err)
	}
	follower := startProgramFileFollow(pager, path, info, events, 10*time.Millisecond)
	t.Cleanup(follower.Stop)

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile(sample) failed: %v", err)
	}
	if _, err := file.WriteString("three\n"); err != nil {
		file.Close()
		t.Fatalf("WriteString(append) failed: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(append file) failed: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for file follow update")
		case <-events:
			if pager.SearchForward("three") {
				return
			}
		}
	}
}

func TestStartProgramFileFollowStopDoesNotBlockOnFullEventQueue(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, testSampleFileName, testInputOneTwo)

	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 3)
	if _, _, err := loadProgramInput(pager, nil, path, programStartup{}); err != nil {
		t.Fatalf("loadProgramInput(file) failed: %v", err)
	}

	events := make(chan tcell.Event, 1)
	events <- tcell.NewEventInterrupt(nil)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(sample) failed: %v", err)
	}
	follower := startProgramFileFollow(pager, path, info, events, 10*time.Millisecond)

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile(sample) failed: %v", err)
	}
	if _, err := file.WriteString("three\n"); err != nil {
		file.Close()
		t.Fatalf("WriteString(append) failed: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close(append file) failed: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for !pager.SearchForward("three") {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for file follow append with full event queue")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	stopped := make(chan struct{})
	go func() {
		follower.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for file follower Stop with full event queue")
	}
}

func TestStartProgramFileFollowReloadsImmediateReplacement(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, testSampleFileName, testInputOneTwo)

	pager := goless.New(goless.Config{TabWidth: 4, WrapMode: goless.NoWrap, ShowStatus: true})
	pager.SetSize(20, 3)
	_, info, err := loadProgramInput(pager, nil, path, programStartup{})
	if err != nil {
		t.Fatalf("loadProgramInput(file) failed: %v", err)
	}

	events := make(chan tcell.Event, 8)
	follower := startProgramFileFollow(pager, path, info, events, 10*time.Millisecond)
	t.Cleanup(follower.Stop)

	replacement := filepath.Join(dir, "replacement.txt")
	if err := os.WriteFile(replacement, []byte("uno\ndos\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(replacement) failed: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove(sample) failed: %v", err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatalf("Rename(replacement) failed: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for file follow replacement update")
		case <-events:
			if pager.SearchForward("uno") {
				if pager.SearchForward("one") {
					t.Fatal("SearchForward(one) = true, want false after replacement reload")
				}
				return
			}
		}
	}
}
