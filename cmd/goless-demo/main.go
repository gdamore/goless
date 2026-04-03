// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/gdamore/goless"
	"github.com/gdamore/tcell/v3"
	tcolor "github.com/gdamore/tcell/v3/color"
)

type demoQuitAtEOFPolicy int

const (
	demoQuitAtEOFNever demoQuitAtEOFPolicy = iota
	demoQuitAtEOFOnForwardEOF
	demoQuitAtEOFWhenVisible
)

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
		fmt.Fprintf(flag.CommandLine.Output(), "usage: goless-demo [-e|-E] [-preset none|dark|light|plain|pretty] [-chrome auto|none|single|rounded] [-hidden] [-live-links] [-render hybrid|literal|presentation] [-squeeze] [-title text] [+line|+/pattern] [file ...]\n")
	}
	flag.BoolVar(&quitAtEOF, "e", false, "quit on the first forward command at EOF")
	flag.BoolVar(&quitAtEOFFirst, "E", false, "quit when EOF is already visible on screen")
	flag.StringVar(&presetName, "preset", "none", "visual preset: none, dark, light, plain, pretty")
	flag.StringVar(&chromeName, "chrome", "auto", "chrome override: auto, none, single, rounded")
	flag.BoolVar(&hidden, "hidden", false, "show tabs, line endings, carriage returns, and EOF markers")
	flag.BoolVar(&liveLinks, "live-links", false, "enable live OSC 8 hyperlinks in the demo")
	flag.BoolVar(&quitAtEOF, "quit-at-eof", false, "long form of -e")
	flag.BoolVar(&quitAtEOFFirst, "QUIT-AT-EOF", false, "long form of -E")
	flag.StringVar(&renderName, "render", "hybrid", "render mode: hybrid, literal, presentation")
	flag.BoolVar(&squeeze, "squeeze", false, "collapse repeated blank lines in the current view")
	flag.StringVar(&title, "title", "", "frame title")
	flag.Parse()

	renderMode, err := demoRenderMode(renderName)
	if err != nil {
		return err
	}
	preset, err := demoPreset(presetName)
	if err != nil {
		return err
	}
	startup, files, err := demoInputs(flag.Args())
	if err != nil {
		return err
	}
	session := newDemoSession(files, startup)
	quitAtEOFPolicy := demoQuitAtEOFPolicyFromFlags(quitAtEOF, quitAtEOFFirst)
	chromeCfg, err := demoChrome(chromeName, title, preset.Chrome)
	if err != nil {
		return err
	}

	if !session.hasFiles() && stdinIsTerminal() {
		return fmt.Errorf("stdin is a terminal; specify a file or pipe input")
	}

	screen, err := newDemoScreen(quitAtEOFPolicy)
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
		pager          *goless.Pager
		readResult     chan error
		reloadCurrent  func() error
		buildDemoPager func() *goless.Pager
	)
	buildDemoPager = func() *goless.Pager {
		return newDemoPager(
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
				return reloadCurrent()
			}),
		)
	}

	if session.hasFiles() {
		reloadCurrent = func() error {
			nextPager := buildDemoPager()
			nextPager.SetSize(width, height)
			if err := loadDemoFile(nextPager, session.currentFile(), session.startup); err != nil {
				return err
			}
			pager = nextPager
			return nil
		}
		if err := reloadCurrent(); err != nil {
			return err
		}
	} else {
		pager = buildDemoPager()
		pager.SetSize(width, height)
		readResult = make(chan error, 1)
		go readIntoPager(pager, os.Stdin, screen.EventQ(), readResult)
	}
	pager.Draw(screen)
	quit, err := handleDemoVisibleEOF(quitAtEOFPolicy, session, func() *goless.Pager { return pager }, readResult == nil, reloadCurrent)
	if err != nil {
		return err
	}
	pager.Draw(screen)
	if quit {
		return demoExit(screen, quitAtEOFPolicy)
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
				presetName = nextDemoPresetName(presetName)
				preset, err = demoPreset(presetName)
				if err != nil {
					return err
				}
				chromeCfg, err = demoChrome(chromeName, title, preset.Chrome)
				if err != nil {
					return err
				}
				pager.SetTheme(preset.Theme)
				pager.SetChrome(session.chrome(chromeCfg))
				break
			}
			if event.Key() == tcell.KeyF5 {
				hidden = !hidden
				pager.SetVisualization(demoVisualization(hidden))
				break
			}
			before := pager.Position()
			beforeFollowing := pager.Following()
			result := pager.HandleKeyResult(event)
			if result.Quit {
				return demoExit(screen, quitAtEOFPolicy)
			}
			quit, err := handleDemoVisibleEOFAction(quitAtEOFPolicy, session, func() *goless.Pager { return pager }, result, readResult == nil, reloadCurrent)
			if err != nil {
				return err
			}
			pager.Draw(screen)
			if quit {
				return demoExit(screen, quitAtEOFPolicy)
			}
			if quit, err := applyDemoQuitAtEOF(
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
				return demoExit(screen, quitAtEOFPolicy)
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
					quit, err := handleDemoVisibleEOF(quitAtEOFPolicy, session, func() *goless.Pager { return pager }, true, reloadCurrent)
					if err != nil {
						return err
					}
					pager.Draw(screen)
					if quit {
						return demoExit(screen, quitAtEOFPolicy)
					}
				default:
				}
			}
		}
		pager.Draw(screen)
	}
}

func demoQuitAtEOFPolicyFromFlags(quitAtEOF, quitAtEOFFirst bool) demoQuitAtEOFPolicy {
	if quitAtEOFFirst {
		return demoQuitAtEOFWhenVisible
	}
	if quitAtEOF {
		return demoQuitAtEOFOnForwardEOF
	}
	return demoQuitAtEOFNever
}

func demoExit(screen tcell.Screen, policy demoQuitAtEOFPolicy) error {
	if policy == demoQuitAtEOFWhenVisible {
		clearDemoStatusLine(screen)
	}
	return nil
}

func clearDemoStatusLine(screen tcell.Screen) {
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

func newDemoScreen(policy demoQuitAtEOFPolicy) (tcell.Screen, error) {
	if policy == demoQuitAtEOFWhenVisible {
		return tcell.NewTerminfoScreen(tcell.OptAltScreen(false))
	}
	return tcell.NewScreen()
}

func applyDemoQuitAtEOF(
	policy demoQuitAtEOFPolicy,
	session *demoSession,
	result goless.KeyResult,
	inputComplete bool,
	following bool,
	before goless.Position,
	after goless.Position,
	reload func() error,
) (bool, error) {
	if policy != demoQuitAtEOFOnForwardEOF || !inputComplete {
		return false, nil
	}
	if result.Context != goless.NormalKeyContext || !result.Handled {
		return false, nil
	}
	if following {
		return false, nil
	}
	if !isDemoQuitAtEOFAction(result.Action) {
		return false, nil
	}
	if demoPositionChanged(before, after) {
		return false, nil
	}
	return advanceDemoSessionOrQuit(session, reload)
}

func isDemoQuitAtEOFAction(action goless.KeyAction) bool {
	switch action {
	case goless.KeyActionScrollDown, goless.KeyActionHalfPageDown, goless.KeyActionPageDown, goless.KeyActionGoBottom:
		return true
	default:
		return false
	}
}

func demoPositionChanged(before, after goless.Position) bool {
	return before.Row != after.Row || before.Rows != after.Rows || before.Column != after.Column || before.Columns != after.Columns
}

func handleDemoVisibleEOFAction(
	policy demoQuitAtEOFPolicy,
	session *demoSession,
	currentPager func() *goless.Pager,
	result goless.KeyResult,
	inputComplete bool,
	reload func() error,
) (bool, error) {
	if policy != demoQuitAtEOFWhenVisible || !inputComplete {
		return false, nil
	}
	if result.Context != goless.NormalKeyContext || !result.Handled || !isDemoQuitAtEOFAction(result.Action) {
		return false, nil
	}
	return handleDemoVisibleEOF(policy, session, currentPager, inputComplete, reload)
}

func handleDemoVisibleEOF(
	policy demoQuitAtEOFPolicy,
	session *demoSession,
	currentPager func() *goless.Pager,
	inputComplete bool,
	reload func() error,
) (bool, error) {
	pager := currentPager()
	if policy != demoQuitAtEOFWhenVisible || !inputComplete || pager == nil || pager.Following() {
		return false, nil
	}
	for pager.EOFVisible() {
		quit, err := advanceDemoSessionOrQuit(session, reload)
		if err != nil || quit {
			return quit, err
		}
		pager = currentPager()
		if pager == nil || pager.Following() {
			return false, nil
		}
	}
	return false, nil
}

func advanceDemoSessionOrQuit(session *demoSession, reload func() error) (bool, error) {
	if session != nil && session.canNext() {
		index := session.index
		session.next()
		if err := reload(); err != nil {
			session.index = index
			return false, err
		}
		return false, nil
	}
	return true, nil
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
	err := appendFromReader(pager, r, func() {
		eventQ <- tcell.NewEventInterrupt(nil)
	})
	pager.Flush()
	result <- err
	eventQ <- tcell.NewEventInterrupt(nil)
}

type demoStartup struct {
	line  int
	query string
}

type demoSession struct {
	files   []string
	index   int
	startup demoStartup
}

func newDemoSession(files []string, startup demoStartup) *demoSession {
	return &demoSession{
		files:   append([]string(nil), files...),
		startup: startup,
	}
}

func (s *demoSession) hasFiles() bool {
	return len(s.files) > 0
}

func (s *demoSession) currentFile() string {
	if len(s.files) == 0 {
		return ""
	}
	return s.files[s.index]
}

func (s *demoSession) currentFileLabel() string {
	if !s.hasFiles() {
		return "stdin"
	}
	return fmt.Sprintf("%s (%d/%d)", s.currentFile(), s.index+1, len(s.files))
}

func (s *demoSession) chrome(base goless.Chrome) goless.Chrome {
	chrome := base
	if s.hasFiles() {
		if chrome.Title == "" {
			chrome.Title = s.currentFile()
		} else {
			chrome.Title = chrome.Title + " - " + s.currentFile()
		}
	}
	return chrome
}

func (s *demoSession) canNext() bool {
	return s.index+1 < len(s.files)
}

func (s *demoSession) canPrev() bool {
	return s.index > 0
}

func (s *demoSession) next() bool {
	if !s.canNext() {
		return false
	}
	s.index++
	return true
}

func (s *demoSession) prev() bool {
	if !s.canPrev() {
		return false
	}
	s.index--
	return true
}

func (s *demoSession) first() bool {
	if !s.hasFiles() || s.index == 0 {
		return false
	}
	s.index = 0
	return true
}

func (s *demoSession) last() bool {
	if !s.hasFiles() || s.index == len(s.files)-1 {
		return false
	}
	s.index = len(s.files) - 1
	return true
}

func (s *demoSession) commandHandler(reload func() error) func(goless.Command) goless.CommandResult {
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

func demoInputs(args []string) (demoStartup, []string, error) {
	var startup demoStartup
	positional := args
	if len(positional) > 0 && positional[0] == "--" {
		positional = positional[1:]
	}
	if len(positional) > 0 && strings.HasPrefix(positional[0], "+") {
		parsedStartup, err := parseStartup(positional[0])
		if err != nil {
			return demoStartup{}, nil, err
		}
		startup = parsedStartup
		positional = positional[1:]
		if len(positional) > 0 && positional[0] == "--" {
			positional = positional[1:]
		}
	}
	return startup, append([]string(nil), positional...), nil
}

func parseStartup(arg string) (demoStartup, error) {
	if arg == "" || !strings.HasPrefix(arg, "+") {
		return demoStartup{}, fmt.Errorf("invalid startup directive %q", arg)
	}
	if query, ok := strings.CutPrefix(arg, "+/"); ok {
		if query == "" {
			return demoStartup{}, fmt.Errorf("invalid startup search %q", arg)
		}
		return demoStartup{query: query}, nil
	}
	line, err := strconv.Atoi(arg[1:])
	if err != nil || line <= 0 {
		return demoStartup{}, fmt.Errorf("invalid startup line %q", arg)
	}
	return demoStartup{line: line}, nil
}

func applyStartupCommand(pager *goless.Pager, startup demoStartup) {
	if startup.line > 0 {
		pager.JumpToLine(startup.line)
		return
	}
	if startup.query != "" {
		pager.SearchForward(startup.query)
	}
}

func loadDemoFile(pager *goless.Pager, path string, startup demoStartup) error {
	file, err := os.Open(path)
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

func demoPreset(name string) (goless.Preset, error) {
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

func nextDemoPresetName(current string) string {
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

func demoVisualization(enabled bool) goless.Visualization {
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

func demoHyperlinkHandler(live bool) goless.HyperlinkHandler {
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

func newDemoPager(
	renderMode goless.RenderMode,
	preset goless.Preset,
	chrome goless.Chrome,
	hidden bool,
	liveLinks bool,
	squeeze bool,
	commandHandler func(goless.Command) goless.CommandResult,
) *goless.Pager {
	return goless.New(goless.Config{
		TabWidth:          8,
		WrapMode:          goless.NoWrap,
		RenderMode:        renderMode,
		SqueezeBlankLines: squeeze,
		Theme:             preset.Theme,
		Visualization:     demoVisualization(hidden),
		HyperlinkHandler:  demoHyperlinkHandler(liveLinks),
		CommandHandler:    commandHandler,
		Chrome:            chrome,
		ShowStatus:        true,
	})
}

func demoChrome(name, title string, base goless.Chrome) (goless.Chrome, error) {
	chrome := goless.Chrome{
		TitleAlign:       base.TitleAlign,
		Title:            base.Title,
		BorderStyle:      base.BorderStyle,
		TitleStyle:       base.TitleStyle,
		StatusStyle:      base.StatusStyle,
		LineNumberStyle:  base.LineNumberStyle,
		HeaderStyle:      base.HeaderStyle,
		PromptStyle:      base.PromptStyle,
		PromptErrorStyle: base.PromptErrorStyle,
		Frame: goless.Frame{
			Horizontal:  base.Frame.Horizontal,
			Vertical:    base.Frame.Vertical,
			TopLeft:     base.Frame.TopLeft,
			TopRight:    base.Frame.TopRight,
			BottomLeft:  base.Frame.BottomLeft,
			BottomRight: base.Frame.BottomRight,
		},
	}
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

func demoRenderMode(name string) (goless.RenderMode, error) {
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
