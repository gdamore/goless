// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gdamore/goless"
	"github.com/gdamore/tcell/v3"
	tcolor "github.com/gdamore/tcell/v3/color"
)

type programQuitAtEOFPolicy int

const (
	programQuitAtEOFNever programQuitAtEOFPolicy = iota
	programQuitAtEOFOnForwardEOF
	programQuitAtEOFWhenVisible
)

const programStdinInput = "-"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var chromeName string
	var hidden bool
	var liveLinks bool
	var presetName string
	var quitAtEOF bool
	var quitAtEOFFirst bool
	var renderName string
	var squeeze bool
	var title string

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: goless [-e|-E] [-preset none|dark|light|plain|pretty] [-chrome auto|none|single|rounded] [-hidden] [-live-links] [-render hybrid|literal|presentation] [-squeeze] [-title text] [+line|+/pattern] [path ...]\n")
	}
	flag.BoolVar(&quitAtEOF, "e", false, "quit on the first forward command at EOF")
	flag.BoolVar(&quitAtEOFFirst, "E", false, "quit when EOF is already visible on screen")
	flag.StringVar(&presetName, "preset", "none", "visual preset: none, dark, light, plain, pretty")
	flag.StringVar(&chromeName, "chrome", "auto", "chrome override: auto, none, single, rounded")
	flag.BoolVar(&hidden, "hidden", false, "show tabs, line endings, carriage returns, and EOF markers")
	flag.BoolVar(&liveLinks, "live-links", false, "enable live OSC 8 hyperlinks in the program")
	flag.BoolVar(&quitAtEOF, "quit-at-eof", false, "long form of -e")
	flag.BoolVar(&quitAtEOFFirst, "QUIT-AT-EOF", false, "long form of -E")
	flag.StringVar(&renderName, "render", "hybrid", "render mode: hybrid, literal, presentation")
	flag.BoolVar(&squeeze, "squeeze", false, "collapse repeated blank lines in the current view")
	flag.StringVar(&title, "title", "", "frame title")
	flag.Parse()

	renderMode, err := programRenderMode(renderName)
	if err != nil {
		return err
	}
	preset, err := programPreset(presetName)
	if err != nil {
		return err
	}
	startup, files, err := programInputs(flag.Args())
	if err != nil {
		return err
	}
	session := newProgramSession(files, startup)
	inputLoader := newProgramInputLoader(os.Stdin)
	quitAtEOFPolicy := programQuitAtEOFPolicyFromFlags(quitAtEOF, quitAtEOFFirst)
	chromeCfg, err := programChrome(chromeName, title, preset.Chrome)
	if err != nil {
		return err
	}
	if !stdoutIsTerminal() {
		return passThroughProgramInputs(os.Stdout, os.Stdin, files)
	}

	if stdinIsTerminal() && (!session.hasFiles() || programInputsUseStdin(files)) {
		return fmt.Errorf("stdin is a terminal; specify a file or pipe input")
	}

	screen, err := newProgramScreen(quitAtEOFPolicy)
	if err != nil {
		return err
	}
	if err := screen.Init(); err != nil {
		return err
	}
	defer screen.Fini()

	width, height := screen.Size()
	if width <= 0 || height <= 0 {
		screen.Sync()
		width, height = screen.Size()
	}

	var (
		pager             *goless.Pager
		readResult        chan error
		reloadCurrent     func() (bool, error)
		buildProgramPager func() *goless.Pager
	)
	buildProgramPager = func() *goless.Pager {
		return newProgramPager(
			renderMode,
			preset,
			session.chrome(chromeCfg),
			hidden,
			liveLinks,
			squeeze,
			session.commandHandler(func() error {
				if reloadCurrent == nil {
					return fmt.Errorf("file reload unavailable")
				}
				_, err := reloadCurrent()
				return err
			}),
		)
	}
	if session.hasFiles() {
		reloadCurrent = func() (bool, error) {
			return reloadProgramInput(session, inputLoader, &pager, buildProgramPager, width, height, screen.EventQ(), &readResult)
		}
		if _, err := reloadCurrent(); err != nil {
			return err
		}
	} else {
		pager = buildProgramPager()
		pager.SetSize(width, height)
		readResult = startProgramRead(pager, os.Stdin, screen.EventQ(), nil)
	}
	pager.Draw(screen)
	quit, err := handleProgramVisibleEOF(quitAtEOFPolicy, session, func() *goless.Pager { return pager }, readResult == nil, reloadCurrent)
	if err != nil {
		return err
	}
	pager.Draw(screen)
	if quit {
		return programExit(screen, quitAtEOFPolicy)
	}

	for {
		ev := <-screen.EventQ()
		switch event := ev.(type) {
		case *tcell.EventResize:
			width, height = event.Size()
			pager.SetSize(width, height)
			screen.Sync()
		case *tcell.EventKey:
			if event.Key() == tcell.KeyF4 {
				presetName = nextProgramPresetName(presetName)
				preset, err = programPreset(presetName)
				if err != nil {
					return err
				}
				chromeCfg, err = programChrome(chromeName, title, preset.Chrome)
				if err != nil {
					return err
				}
				pager.SetTheme(preset.Theme)
				pager.SetChrome(session.chrome(chromeCfg))
				break
			}
			if event.Key() == tcell.KeyF5 {
				hidden = !hidden
				pager.SetVisualization(programVisualization(hidden))
				break
			}
			before := pager.Position()
			beforeFollowing := pager.Following()
			result := pager.HandleKeyResult(event)
			if result.Quit {
				return programExit(screen, quitAtEOFPolicy)
			}
			quit, err := handleProgramVisibleEOFAction(quitAtEOFPolicy, session, func() *goless.Pager { return pager }, result, readResult == nil, reloadCurrent)
			if err != nil {
				return err
			}
			pager.Draw(screen)
			if quit {
				return programExit(screen, quitAtEOFPolicy)
			}
			if quit, err := applyProgramQuitAtEOF(
				quitAtEOFPolicy,
				session,
				result,
				readResult == nil,
				beforeFollowing,
				before,
				pager.Position(),
				reloadCurrent,
			); err != nil {
				return err
			} else if quit {
				return programExit(screen, quitAtEOFPolicy)
			}
		case *tcell.EventInterrupt:
			if readResult != nil {
				select {
				case err := <-readResult:
					if err != nil {
						return err
					}
					applyStartupCommand(pager, startup)
					readResult = nil
					quit, err := handleProgramVisibleEOF(quitAtEOFPolicy, session, func() *goless.Pager { return pager }, true, reloadCurrent)
					if err != nil {
						return err
					}
					pager.Draw(screen)
					if quit {
						return programExit(screen, quitAtEOFPolicy)
					}
				default:
				}
			}
		}
		pager.Draw(screen)
	}
}

func programQuitAtEOFPolicyFromFlags(quitAtEOF, quitAtEOFFirst bool) programQuitAtEOFPolicy {
	if quitAtEOFFirst {
		return programQuitAtEOFWhenVisible
	}
	if quitAtEOF {
		return programQuitAtEOFOnForwardEOF
	}
	return programQuitAtEOFNever
}

func programExit(screen tcell.Screen, policy programQuitAtEOFPolicy) error {
	if policy == programQuitAtEOFWhenVisible {
		clearProgramStatusLine(screen)
	}
	return nil
}

func clearProgramStatusLine(screen tcell.Screen) {
	if screen == nil {
		return
	}
	width, height := screen.Size()
	if width <= 0 || height <= 0 {
		return
	}
	y := height - 1
	for x := 0; x < width; x++ {
		screen.SetContent(x, y, ' ', nil, tcell.StyleDefault)
	}
	screen.Show()
}

func newProgramScreen(policy programQuitAtEOFPolicy) (tcell.Screen, error) {
	if policy == programQuitAtEOFWhenVisible {
		return tcell.NewTerminfoScreen(tcell.OptAltScreen(false))
	}
	return tcell.NewScreen()
}

func applyProgramQuitAtEOF(
	policy programQuitAtEOFPolicy,
	session *programSession,
	result goless.KeyResult,
	inputComplete bool,
	following bool,
	before goless.Position,
	after goless.Position,
	reload func() (bool, error),
) (bool, error) {
	if policy != programQuitAtEOFOnForwardEOF || !inputComplete {
		return false, nil
	}
	if result.Context != goless.NormalKeyContext || !result.Handled {
		return false, nil
	}
	if following {
		return false, nil
	}
	if !isProgramQuitAtEOFAction(result.Action) {
		return false, nil
	}
	if programPositionChanged(before, after) {
		return false, nil
	}
	quit, _, err := advanceProgramSessionOrQuit(session, reload)
	return quit, err
}

func isProgramQuitAtEOFAction(action goless.KeyAction) bool {
	switch action {
	case goless.KeyActionScrollDown, goless.KeyActionHalfPageDown, goless.KeyActionPageDown, goless.KeyActionGoBottom:
		return true
	default:
		return false
	}
}

func programPositionChanged(before, after goless.Position) bool {
	return before.Row != after.Row || before.Rows != after.Rows || before.Column != after.Column || before.Columns != after.Columns
}

func handleProgramVisibleEOFAction(
	policy programQuitAtEOFPolicy,
	session *programSession,
	currentPager func() *goless.Pager,
	result goless.KeyResult,
	inputComplete bool,
	reload func() (bool, error),
) (bool, error) {
	if policy != programQuitAtEOFWhenVisible || !inputComplete {
		return false, nil
	}
	if result.Context != goless.NormalKeyContext || !result.Handled || !isProgramQuitAtEOFAction(result.Action) {
		return false, nil
	}
	return handleProgramVisibleEOF(policy, session, currentPager, inputComplete, reload)
}

func handleProgramVisibleEOF(
	policy programQuitAtEOFPolicy,
	session *programSession,
	currentPager func() *goless.Pager,
	inputComplete bool,
	reload func() (bool, error),
) (bool, error) {
	pager := currentPager()
	if policy != programQuitAtEOFWhenVisible || !inputComplete || pager == nil || pager.Following() {
		return false, nil
	}
	for pager.EOFVisible() {
		quit, loaded, err := advanceProgramSessionOrQuit(session, reload)
		if err != nil || quit {
			return quit, err
		}
		if !loaded {
			return false, nil
		}
		pager = currentPager()
		if pager == nil || pager.Following() {
			return false, nil
		}
	}
	return false, nil
}

func advanceProgramSessionOrQuit(session *programSession, reload func() (bool, error)) (bool, bool, error) {
	if session != nil && session.canNext() {
		index := session.index
		session.next()
		loaded, err := reload()
		if err != nil {
			session.index = index
			return false, false, err
		}
		return false, loaded, nil
	}
	return true, true, nil
}

func appendFromReader(pager *goless.Pager, r io.Reader, onChunk func()) error {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if appendErr := pager.Append(buf[:n]); appendErr != nil {
				return appendErr
			}
			if onChunk != nil {
				onChunk()
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func readIntoPager(pager *goless.Pager, r io.Reader, eventQ chan tcell.Event, result chan<- error) {
	readIntoPagerWithAfter(pager, r, eventQ, result, nil)
}

func readIntoPagerWithAfter(pager *goless.Pager, r io.Reader, eventQ chan tcell.Event, result chan<- error, after func(error)) {
	err := appendFromReader(pager, r, func() {
		eventQ <- tcell.NewEventInterrupt(nil)
	})
	pager.Flush()
	if after != nil {
		after(err)
	}
	result <- err
	eventQ <- tcell.NewEventInterrupt(nil)
}

func startProgramRead(target *goless.Pager, reader io.Reader, eventQ chan tcell.Event, after func(error)) chan error {
	result := make(chan error, 1)
	go readIntoPagerWithAfter(target, reader, eventQ, result, after)
	return result
}

func passThroughProgramInputs(dst io.Writer, stdin io.Reader, files []string) error {
	if len(files) == 0 {
		return copyProgramInput(dst, stdin)
	}
	for _, path := range files {
		if isProgramStdinInput(path) {
			if err := copyProgramInput(dst, stdin); err != nil {
				return err
			}
			continue
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := copyProgramInput(dst, file); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

func copyProgramInput(dst io.Writer, src io.Reader) error {
	_, err := io.Copy(dst, src)
	return err
}

type programInputLoader struct {
	stdin       io.Reader
	stdinLoaded bool
	stdinActive bool
	stdinData   []byte
	mu          sync.Mutex
}

func newProgramInputLoader(stdin io.Reader) *programInputLoader {
	return &programInputLoader{stdin: stdin}
}

func (l *programInputLoader) open(path string) (io.ReadCloser, error) {
	if !isProgramStdinInput(path) {
		return os.Open(path)
	}
	if l == nil || l.stdin == nil {
		return nil, fmt.Errorf("stdin unavailable")
	}
	if !l.stdinLoaded {
		data, err := io.ReadAll(l.stdin)
		if err != nil {
			return nil, err
		}
		l.stdinData = data
		l.stdinLoaded = true
	}
	return io.NopCloser(bytes.NewReader(l.stdinData)), nil
}

func (l *programInputLoader) canStream(path string) bool {
	if !isProgramStdinInput(path) || l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stdin != nil && !l.stdinLoaded && !l.stdinActive
}

func (l *programInputLoader) startStdinStream() (io.Reader, func(error, []byte), error) {
	if l == nil || l.stdin == nil {
		return nil, nil, fmt.Errorf("stdin unavailable")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stdinLoaded {
		return nil, nil, fmt.Errorf("stdin already cached")
	}
	if l.stdinActive {
		return nil, nil, fmt.Errorf("stdin already reading")
	}
	l.stdinActive = true
	return l.stdin, func(err error, data []byte) {
		l.mu.Lock()
		defer l.mu.Unlock()
		if err == nil {
			l.stdinData = bytes.Clone(data)
			l.stdinLoaded = true
		}
		l.stdinActive = false
	}, nil
}

func reloadProgramInput(
	session *programSession,
	loader *programInputLoader,
	pager **goless.Pager,
	buildPager func() *goless.Pager,
	width, height int,
	eventQ chan tcell.Event,
	readResult *chan error,
) (bool, error) {
	if readResult != nil && *readResult != nil {
		return false, fmt.Errorf("stdin still reading")
	}
	nextPager := buildPager()
	nextPager.SetSize(width, height)
	currentPath := session.currentFile()
	if loader != nil && loader.canStream(currentPath) {
		reader, finish, err := loader.startStdinStream()
		if err != nil {
			return false, err
		}
		var cache bytes.Buffer
		*pager = nextPager
		if readResult != nil {
			*readResult = startProgramRead(*pager, io.TeeReader(reader, &cache), eventQ, func(err error) {
				finish(err, cache.Bytes())
			})
		}
		return false, nil
	}
	if err := loadProgramInput(nextPager, loader, currentPath, session.startup); err != nil {
		return false, err
	}
	*pager = nextPager
	if readResult != nil {
		*readResult = nil
	}
	return true, nil
}

type programStartup struct {
	line  int
	query string
}

type programSession struct {
	files   []string
	index   int
	startup programStartup
}

func newProgramSession(files []string, startup programStartup) *programSession {
	return &programSession{
		files:   append([]string(nil), files...),
		startup: startup,
	}
}

func (s *programSession) hasFiles() bool {
	return len(s.files) > 0
}

func (s *programSession) currentFile() string {
	if len(s.files) == 0 {
		return ""
	}
	return s.files[s.index]
}

func (s *programSession) currentFileLabel() string {
	if !s.hasFiles() {
		return "stdin"
	}
	return fmt.Sprintf("%s (%d/%d)", programInputLabel(s.currentFile()), s.index+1, len(s.files))
}

func (s *programSession) chrome(base goless.Chrome) goless.Chrome {
	chrome := base
	if s.hasFiles() {
		if chrome.Title == "" {
			chrome.Title = programInputLabel(s.currentFile())
		} else {
			chrome.Title = chrome.Title + " - " + programInputLabel(s.currentFile())
		}
	}
	return chrome
}

func (s *programSession) canNext() bool {
	return s.index+1 < len(s.files)
}

func (s *programSession) canPrev() bool {
	return s.index > 0
}

func (s *programSession) next() bool {
	if !s.canNext() {
		return false
	}
	s.index++
	return true
}

func (s *programSession) prev() bool {
	if !s.canPrev() {
		return false
	}
	s.index--
	return true
}

func (s *programSession) first() bool {
	if !s.hasFiles() || s.index == 0 {
		return false
	}
	s.index = 0
	return true
}

func (s *programSession) last() bool {
	if !s.hasFiles() || s.index == len(s.files)-1 {
		return false
	}
	s.index = len(s.files) - 1
	return true
}

func (s *programSession) commandHandler(reload func() error) func(goless.Command) goless.CommandResult {
	return func(cmd goless.Command) goless.CommandResult {
		switch cmd.Name {
		case "q", "quit":
			return goless.CommandResult{Handled: true, Quit: true}
		case "file", "f":
			return goless.CommandResult{Handled: true, Message: s.currentFileLabel()}
		case "next", "n":
			if !s.canNext() {
				return goless.CommandResult{Handled: true, Message: "already at last file"}
			}
			index := s.index
			s.next()
			if err := reload(); err != nil {
				s.index = index
				return goless.CommandResult{Handled: true, Message: err.Error()}
			}
			return goless.CommandResult{Handled: true, Message: s.currentFileLabel()}
		case "prev", "p", "previous":
			if !s.canPrev() {
				return goless.CommandResult{Handled: true, Message: "already at first file"}
			}
			index := s.index
			s.prev()
			if err := reload(); err != nil {
				s.index = index
				return goless.CommandResult{Handled: true, Message: err.Error()}
			}
			return goless.CommandResult{Handled: true, Message: s.currentFileLabel()}
		case "first", "rewind":
			if !s.hasFiles() || s.index == 0 {
				return goless.CommandResult{Handled: true, Message: "already at first file"}
			}
			index := s.index
			s.first()
			if err := reload(); err != nil {
				s.index = index
				return goless.CommandResult{Handled: true, Message: err.Error()}
			}
			return goless.CommandResult{Handled: true, Message: s.currentFileLabel()}
		case "last":
			if !s.hasFiles() || s.index == len(s.files)-1 {
				return goless.CommandResult{Handled: true, Message: "already at last file"}
			}
			index := s.index
			s.last()
			if err := reload(); err != nil {
				s.index = index
				return goless.CommandResult{Handled: true, Message: err.Error()}
			}
			return goless.CommandResult{Handled: true, Message: s.currentFileLabel()}
		default:
			return goless.CommandResult{}
		}
	}
}

func programInputs(args []string) (programStartup, []string, error) {
	var startup programStartup
	positional := args
	if len(positional) > 0 && positional[0] == "--" {
		positional = positional[1:]
	}
	if len(positional) > 0 && strings.HasPrefix(positional[0], "+") {
		parsedStartup, err := parseStartup(positional[0])
		if err != nil {
			return programStartup{}, nil, err
		}
		startup = parsedStartup
		positional = positional[1:]
		if len(positional) > 0 && positional[0] == "--" {
			positional = positional[1:]
		}
	}
	files := append([]string(nil), positional...)
	stdinCount := 0
	for _, path := range files {
		if isProgramStdinInput(path) {
			stdinCount++
		}
	}
	if stdinCount > 1 {
		return programStartup{}, nil, fmt.Errorf("stdin may be specified at most once")
	}
	if len(files) == 1 && isProgramStdinInput(files[0]) {
		files = nil
	}
	return startup, files, nil
}

func programInputsUseStdin(files []string) bool {
	for _, path := range files {
		if isProgramStdinInput(path) {
			return true
		}
	}
	return false
}

func parseStartup(arg string) (programStartup, error) {
	if arg == "" || !strings.HasPrefix(arg, "+") {
		return programStartup{}, fmt.Errorf("invalid startup directive %q", arg)
	}
	if query, ok := strings.CutPrefix(arg, "+/"); ok {
		if query == "" {
			return programStartup{}, fmt.Errorf("invalid startup search %q", arg)
		}
		return programStartup{query: query}, nil
	}
	line, err := strconv.Atoi(arg[1:])
	if err != nil || line <= 0 {
		return programStartup{}, fmt.Errorf("invalid startup line %q", arg)
	}
	return programStartup{line: line}, nil
}

func applyStartupCommand(pager *goless.Pager, startup programStartup) {
	if startup.line > 0 {
		pager.JumpToLine(startup.line)
		return
	}
	if startup.query != "" {
		pager.SearchForward(startup.query)
	}
}

func loadProgramInput(pager *goless.Pager, loader *programInputLoader, path string, startup programStartup) error {
	var (
		file io.ReadCloser
		err  error
	)
	if loader != nil {
		file, err = loader.open(path)
	} else if isProgramStdinInput(path) {
		file = io.NopCloser(os.Stdin)
	} else {
		file, err = os.Open(path)
	}
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := pager.ReadFrom(file); err != nil {
		return err
	}
	pager.Flush()
	applyStartupCommand(pager, startup)
	return nil
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func isProgramStdinInput(path string) bool {
	return path == programStdinInput
}

func programInputLabel(path string) string {
	if isProgramStdinInput(path) {
		return "stdin"
	}
	return path
}

func stdoutIsTerminal() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func programPreset(name string) (goless.Preset, error) {
	switch name {
	case "dark":
		return goless.DarkPreset, nil
	case "light":
		return goless.LightPreset, nil
	case "plain":
		return goless.PlainPreset, nil
	case "pretty":
		return goless.PrettyPreset, nil
	case "none", "":
		return goless.Preset{}, nil
	default:
		return goless.Preset{}, fmt.Errorf("unknown preset %q; expected none, dark, light, plain, or pretty", name)
	}
}

func nextProgramPresetName(current string) string {
	switch current {
	case "dark":
		return "light"
	case "light":
		return "plain"
	case "plain":
		return "pretty"
	case "pretty":
		return "none"
	default:
		return "dark"
	}
}

func programVisualization(enabled bool) goless.Visualization {
	if !enabled {
		return goless.Visualization{}
	}
	return goless.Visualization{
		ShowTabs:            true,
		ShowNewlines:        true,
		ShowCarriageReturns: true,
		ShowEOF:             true,
	}
}

func programHyperlinkHandler(live bool) goless.HyperlinkHandler {
	return func(info goless.HyperlinkInfo) goless.HyperlinkDecision {
		return goless.HyperlinkDecision{
			Style: info.Style.
				Foreground(tcolor.Blue).
				Underline(tcell.UnderlineStyleSolid),
			Live:     live,
			StyleSet: true,
		}
	}
}

func newProgramPager(
	renderMode goless.RenderMode,
	preset goless.Preset,
	chrome goless.Chrome,
	hidden bool,
	liveLinks bool,
	squeeze bool,
	commandHandler func(goless.Command) goless.CommandResult,
) *goless.Pager {
	return goless.New(
		goless.WithTabWidth(8),
		goless.WithWrapMode(goless.NoWrap),
		goless.WithRenderMode(renderMode),
		goless.WithSqueezeBlankLines(squeeze),
		goless.WithTheme(preset.Theme),
		goless.WithVisualization(programVisualization(hidden)),
		goless.WithHyperlinkHandler(programHyperlinkHandler(liveLinks)),
		goless.WithCommandHandler(commandHandler),
		goless.WithChrome(chrome),
		goless.WithShowStatus(true),
	)
}

func programChrome(name, title string, base goless.Chrome) (goless.Chrome, error) {
	chrome := base
	if title != "" {
		chrome.Title = title
	}

	var frame goless.Frame
	switch name {
	case "auto", "":
		return chrome, nil
	case "single":
		frame = goless.SingleFrame()
	case "rounded":
		frame = goless.RoundedFrame()
	case "none":
		chrome.Frame = goless.Frame{}
		return chrome, nil
	default:
		return goless.Chrome{}, fmt.Errorf("unknown chrome %q; expected auto, none, single, or rounded", name)
	}

	chrome.Frame = goless.Frame{
		Horizontal:  frame.Horizontal,
		Vertical:    frame.Vertical,
		TopLeft:     frame.TopLeft,
		TopRight:    frame.TopRight,
		BottomLeft:  frame.BottomLeft,
		BottomRight: frame.BottomRight,
	}
	return chrome, nil
}

func programRenderMode(name string) (goless.RenderMode, error) {
	switch name {
	case "literal":
		return goless.RenderLiteral, nil
	case "presentation":
		return goless.RenderPresentation, nil
	case "hybrid", "":
		return goless.RenderHybrid, nil
	default:
		return goless.RenderHybrid, fmt.Errorf("unknown render mode %q; expected hybrid, literal, or presentation", name)
	}
}
