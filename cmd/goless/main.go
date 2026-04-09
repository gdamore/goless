// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/goless"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
)

type programQuitAtEOFPolicy int

const (
	programQuitAtEOFNever programQuitAtEOFPolicy = iota
	programQuitAtEOFOnForwardEOF
	programQuitAtEOFWhenVisible
)

const programStdinInput = "-"
const programFileFollowInterval = 250 * time.Millisecond

type programOptions struct {
	chromeName        string
	hidden            bool
	ignoredChopLines  bool
	ignoredRawControl bool
	lineNumbers       bool
	liveLinks         bool
	presetName        string
	quitAtEOF         bool
	quitAtEOFFirst    bool
	quitIfOneScreen   bool
	renderName        string
	searchCaseMode    goless.SearchCaseMode
	showHelp          bool
	showLicense       bool
	squeeze           bool
	tabWidth          int
	title             string
	version           bool
}

type programViewportSnapshot struct {
	eofVisible bool
	following  bool
}

type programInputResult struct {
	handled bool
	quit    bool
	action  goless.KeyAction
	context goless.KeyContext
}

type programFileFollower struct {
	path  string
	pager *goless.Pager
	info  os.FileInfo
	stop  chan struct{}
	done  chan struct{}
}

var (
	programScreenFactory        = newProgramScreen
	programStdinIsTerminalFunc  = stdinIsTerminal
	programStdoutIsTerminalFunc = stdoutIsTerminal
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	opts, args, err := parseProgramFlags(os.Args[1:], flag.CommandLine.Output())
	if err != nil {
		return err
	}
	if opts.showHelp {
		return nil
	}
	if opts.version {
		fmt.Printf("goless version %s\n", version())
		return nil
	}

	renderMode, err := programRenderMode(opts.renderName)
	if err != nil {
		return err
	}
	preset, err := programPreset(opts.presetName)
	if err != nil {
		return err
	}
	startup, files, err := programInputs(args)
	if err != nil {
		return err
	}
	session := newProgramSession(files, startup)
	inputLoader := newProgramInputLoader(os.Stdin)
	quitAtEOFPolicy := programQuitAtEOFPolicyFromFlags(opts.quitAtEOF, opts.quitAtEOFFirst)
	quitIfOneScreenArmed := opts.quitIfOneScreen
	chromeCfg, err := programChrome(opts.chromeName, opts.title, preset.Chrome)
	if err != nil {
		return err
	}
	if !programStdoutIsTerminalFunc() {
		if opts.showLicense {
			_, err := io.WriteString(os.Stdout, goless.LicenseText())
			return err
		}
		return passThroughProgramInputs(os.Stdout, os.Stdin, files)
	}

	if programStdinIsTerminalFunc() && programRequiresInput(opts, files) {
		return fmt.Errorf("stdin is a terminal; specify a file or pipe input")
	}

	screen, err := programScreenFactory(quitAtEOFPolicy)
	if err != nil {
		return err
	}
	if err := screen.Init(); err != nil {
		return err
	}
	defer screen.Fini()
	screen.EnableMouse()

	width, height := screen.Size()
	if width <= 0 || height <= 0 {
		screen.Sync()
		width, height = screen.Size()
	}

	var (
		pager             *goless.Pager
		pagerSnapshot     programViewportSnapshot
		readResult        chan error
		loadCurrent       func() (bool, error)
		reloadDisplayed   func() error
		buildProgramPager func() *goless.Pager
		fileFollower      *programFileFollower
		currentFileInfo   os.FileInfo
	)
	stopProgramFileFollower := func() {
		if fileFollower == nil {
			return
		}
		fileFollower.Stop()
		fileFollower = nil
	}
	updateProgramFileFollower := func() {
		path := ""
		if session != nil && session.hasFiles() {
			path = session.currentFile()
		}
		shouldFollow := pager != nil && readResult == nil && path != "" && !isProgramStdinInput(path) && pager.Following()
		if fileFollower != nil {
			if !shouldFollow || fileFollower.path != path || fileFollower.pager != pager {
				stopProgramFileFollower()
			}
		}
		if shouldFollow && fileFollower == nil {
			fileFollower = startProgramFileFollow(pager, path, currentFileInfo, screen.EventQ(), programFileFollowInterval)
		}
	}
	defer stopProgramFileFollower()
	buildProgramPager = func() *goless.Pager {
		return newProgramPager(
			renderMode,
			preset,
			session.chrome(chromeCfg),
			opts.hidden,
			opts.liveLinks,
			opts.lineNumbers,
			opts.searchCaseMode,
			opts.squeeze,
			opts.tabWidth,
			session.commandHandler(
				func() error {
					if loadCurrent == nil {
						return fmt.Errorf("file reload unavailable")
					}
					_, err := loadCurrent()
					return err
				},
				reloadDisplayed,
				func(title, body string) {
					if pager != nil {
						pager.ShowInformation(title, body)
					}
				},
			),
		)
	}
	if opts.showLicense {
		pager = buildProgramPager()
		pager.SetSize(width, height)
		pager.ShowInformation("Apache License 2.0", goless.LicenseText())
	} else if session.hasFiles() {
		loadCurrent = func() (bool, error) {
			stopProgramFileFollower()
			loaded, snapshot, info, err := reloadProgramInput(session, inputLoader, &pager, buildProgramPager, width, height, screen.EventQ(), &readResult)
			if err == nil {
				pagerSnapshot = snapshot
				currentFileInfo = info
				updateProgramFileFollower()
			}
			return loaded, err
		}
		reloadDisplayed = func() error {
			if pager == nil {
				return fmt.Errorf("file reload unavailable")
			}
			if readResult != nil {
				return fmt.Errorf("stdin still reading")
			}
			stopProgramFileFollower()
			info, err := reloadProgramInputInPlaceWithInfo(pager, inputLoader, session.currentFile())
			if err == nil {
				currentFileInfo = info
				updateProgramFileFollower()
			}
			return err
		}
		if _, err := loadCurrent(); err != nil {
			return err
		}
	} else {
		pager = buildProgramPager()
		pager.SetSize(width, height)
		readResult = startProgramRead(pager, os.Stdin, screen.EventQ(), nil)
	}
	updateProgramFileFollower()
	pager.Draw(screen)
	if !opts.showLicense {
		quit, err := handleProgramQuitIfOneScreen(quitIfOneScreenArmed, session, func() programViewportSnapshot { return pagerSnapshot }, readResult == nil, loadCurrent)
		if err != nil {
			return err
		}
		pager.Draw(screen)
		if quit {
			return programExit(screen, quitAtEOFPolicy)
		}
		quit, err = handleProgramVisibleEOF(quitAtEOFPolicy, session, func() *goless.Pager { return pager }, readResult == nil, loadCurrent)
		if err != nil {
			return err
		}
		pager.Draw(screen)
		if quit {
			return programExit(screen, quitAtEOFPolicy)
		}
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
				opts.presetName = nextProgramPresetName(opts.presetName)
				preset, err = programPreset(opts.presetName)
				if err != nil {
					return err
				}
				chromeCfg, err = programChrome(opts.chromeName, opts.title, preset.Chrome)
				if err != nil {
					return err
				}
				pager.SetTheme(preset.Theme)
				pager.SetChrome(session.chrome(chromeCfg))
				break
			}
			if event.Key() == tcell.KeyF5 {
				opts.hidden = !opts.hidden
				pager.SetVisualization(programVisualization(opts.hidden))
				break
			}
			before := pager.Position()
			beforeFollowing := pager.Following()
			wasShowingInformation := pager.ShowingInformation()
			keyResult := pager.HandleKeyResult(event)
			if shouldHandleProgramReloadKey(keyResult) && handleProgramReloadKey(pager, event) {
				pager.Draw(screen)
				break
			}
			if shouldHandleProgramStatusKey(keyResult) && handleProgramStatusKey(pager, event) {
				pager.Draw(screen)
				break
			}
			if keyResult.Quit {
				return programExit(screen, quitAtEOFPolicy)
			}
			if programShouldQuitAfterOverlayClose(session, pager, wasShowingInformation, keyResult) {
				return programExit(screen, quitAtEOFPolicy)
			}
			quit, err := handleProgramPostInput(
				quitAtEOFPolicy,
				opts.showLicense,
				session,
				pager,
				programInputResult{
					handled: keyResult.Handled,
					quit:    keyResult.Quit,
					action:  keyResult.Action,
					context: keyResult.Context,
				},
				readResult == nil,
				beforeFollowing,
				before,
				loadCurrent,
			)
			if err != nil {
				return err
			}
			if quit {
				return programExit(screen, quitAtEOFPolicy)
			}
			if readResult != nil {
				quitIfOneScreenArmed = updateProgramQuitIfOneScreenArm(quitIfOneScreenArmed, pager)
			}
		case *tcell.EventMouse:
			before := pager.Position()
			beforeFollowing := pager.Following()
			mouseResult := pager.HandleMouseResult(event)
			quit, err := handleProgramPostInput(
				quitAtEOFPolicy,
				opts.showLicense,
				session,
				pager,
				programInputResult{
					handled: mouseResult.Handled,
					action:  mouseResult.Action,
					context: mouseResult.Context,
				},
				readResult == nil,
				beforeFollowing,
				before,
				loadCurrent,
			)
			if err != nil {
				return err
			}
			if quit {
				return programExit(screen, quitAtEOFPolicy)
			}
			if readResult != nil {
				quitIfOneScreenArmed = updateProgramQuitIfOneScreenArm(quitIfOneScreenArmed, pager)
			}
		case *tcell.EventInterrupt:
			if readResult != nil {
				select {
				case err := <-readResult:
					if err != nil {
						return err
					}
					pagerSnapshot = snapshotProgramPager(pager)
					applyStartupCommand(pager, startup)
					readResult = nil
					quit, err := handleProgramQuitIfOneScreen(quitIfOneScreenArmed, session, func() programViewportSnapshot { return pagerSnapshot }, true, loadCurrent)
					if err != nil {
						return err
					}
					pager.Draw(screen)
					if quit {
						return programExit(screen, quitAtEOFPolicy)
					}
					quit, err = handleProgramVisibleEOF(quitAtEOFPolicy, session, func() *goless.Pager { return pager }, true, loadCurrent)
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
		updateProgramFileFollower()
		pager.Draw(screen)
	}
}

func parseProgramFlags(args []string, output io.Writer) (programOptions, []string, error) {
	opts := programOptions{
		presetName:     "pretty",
		chromeName:     "auto",
		renderName:     "hybrid",
		searchCaseMode: goless.SearchSmartCase,
		tabWidth:       8,
	}

	fs := flag.NewFlagSet("goless", flag.ContinueOnError)
	fs.SetOutput(output)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: goless [-?|-e|-E|-F|-N|-R|-S|-i|-I|-s] [-x n] [-preset dark|light|plain|pretty] [-chrome auto|none|single|rounded] [-hidden] [-live-links] [-render hybrid|literal|presentation] [-squeeze] [-title text] [-license] [+line|+/pattern] [-version] [path ...]\n")
	}

	fs.BoolVar(&opts.showHelp, "?", false, "show help")
	fs.BoolVar(&opts.showHelp, "help", false, "show help")
	fs.BoolVar(&opts.showHelp, "h", false, "show help")
	fs.BoolVar(&opts.showLicense, "license", false, "show the bundled Apache license")
	fs.BoolVar(&opts.quitAtEOF, "e", false, "quit on the first forward command at EOF")
	fs.BoolVar(&opts.quitAtEOFFirst, "E", false, "quit when EOF is already visible on screen")
	fs.BoolVar(&opts.quitIfOneScreen, "F", false, "quit if the entire input fits on one screen")
	fs.BoolVar(&opts.lineNumbers, "N", false, "show line numbers")
	fs.BoolVar(&opts.ignoredRawControl, "R", false, "accepted as a less compatibility no-op")
	fs.BoolVar(&opts.ignoredChopLines, "S", false, "accepted as a less compatibility no-op")
	fs.BoolVar(&opts.hidden, "hidden", false, "show tabs, line endings, carriage returns, and EOF markers")
	fs.BoolVar(&opts.liveLinks, "live-links", false, "enable live OSC 8 hyperlinks in the program")
	fs.BoolVar(&opts.quitAtEOF, "quit-at-eof", false, "long form of -e")
	fs.BoolVar(&opts.quitAtEOFFirst, "QUIT-AT-EOF", false, "long form of -E")
	fs.BoolVar(&opts.squeeze, "s", false, "collapse repeated blank lines in the current view")
	fs.BoolVar(&opts.squeeze, "squeeze", false, "collapse repeated blank lines in the current view")
	fs.StringVar(&opts.presetName, "preset", "pretty", "visual preset: none, dark, light, plain, pretty")
	fs.StringVar(&opts.chromeName, "chrome", "auto", "chrome override: auto, none, single, rounded")
	fs.StringVar(&opts.renderName, "render", "hybrid", "render mode: hybrid, literal, presentation")
	fs.StringVar(&opts.title, "title", "", "frame title")
	fs.IntVar(&opts.tabWidth, "x", 8, "tab width")
	fs.BoolVar(&opts.version, "version", false, "display program version and exit")

	var ignoreSmartCase bool
	var ignoreCase bool
	fs.BoolVar(&ignoreSmartCase, "i", false, "smart-case search")
	fs.BoolVar(&ignoreCase, "I", false, "case-insensitive search")

	if err := fs.Parse(args); err != nil {
		return programOptions{}, nil, err
	}
	if opts.showHelp {
		fs.Usage()
		return opts, nil, nil
	}
	if ignoreSmartCase && ignoreCase {
		return programOptions{}, nil, fmt.Errorf("-i and -I are mutually exclusive")
	}
	if ignoreCase {
		opts.searchCaseMode = goless.SearchCaseInsensitive
	} else if ignoreSmartCase {
		opts.searchCaseMode = goless.SearchSmartCase
	}
	if opts.tabWidth <= 0 {
		return programOptions{}, nil, fmt.Errorf("-x must be greater than 0")
	}
	if opts.showLicense {
		if fs.NFlag() > 1 {
			return programOptions{}, nil, fmt.Errorf("--license must be used alone")
		}
		if len(fs.Args()) > 0 {
			return programOptions{}, nil, fmt.Errorf("--license cannot be combined with files")
		}
	}
	return opts, fs.Args(), nil
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

func programRequiresInput(opts programOptions, files []string) bool {
	if opts.showLicense {
		return false
	}
	return len(files) == 0 || programInputsUseStdin(files)
}

func programShouldQuitAfterOverlayClose(session *programSession, pager *goless.Pager, wasShowingInformation bool, result goless.KeyResult) bool {
	if pager == nil {
		return false
	}
	if result.Context != goless.HelpKeyContext || result.Action != goless.KeyActionToggleHelp {
		return false
	}
	if !wasShowingInformation {
		return false
	}
	if pager.ShowingInformation() {
		return false
	}
	if session != nil && session.hasFiles() {
		return false
	}
	return pager.Len() == 0
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
	for x := range width {
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

func handleProgramPostInput(
	policy programQuitAtEOFPolicy,
	showLicense bool,
	session *programSession,
	pager *goless.Pager,
	result programInputResult,
	inputComplete bool,
	following bool,
	before goless.Position,
	reload func() (bool, error),
) (bool, error) {
	if pager == nil {
		return false, nil
	}
	keyResult := result.keyResult()
	if !showLicense {
		quit, err := handleProgramVisibleEOFAction(policy, session, func() *goless.Pager { return pager }, keyResult, inputComplete, reload)
		if err != nil || quit {
			return quit, err
		}
	}
	return applyProgramQuitAtEOF(policy, session, keyResult, inputComplete, following, before, pager.Position(), reload)
}

func (r programInputResult) keyResult() goless.KeyResult {
	return goless.KeyResult{
		Handled: r.handled,
		Quit:    r.quit,
		Action:  r.action,
		Context: r.context,
	}
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

func handleProgramQuitIfOneScreen(
	enabled bool,
	session *programSession,
	currentSnapshot func() programViewportSnapshot,
	inputComplete bool,
	reload func() (bool, error),
) (bool, error) {
	if !enabled || !inputComplete {
		return false, nil
	}
	snapshot := currentSnapshot()
	if snapshot.following {
		return false, nil
	}
	for snapshot.eofVisible {
		quit, loaded, err := advanceProgramSessionOrQuit(session, reload)
		if err != nil || quit {
			return quit, err
		}
		if !loaded {
			return false, nil
		}
		snapshot = currentSnapshot()
		if snapshot.following {
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

func startProgramFileFollow(pager *goless.Pager, path string, info os.FileInfo, eventQ chan tcell.Event, interval time.Duration) *programFileFollower {
	if interval <= 0 {
		interval = programFileFollowInterval
	}
	follower := &programFileFollower{
		path:  path,
		pager: pager,
		info:  info,
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer close(follower.done)
		for {
			select {
			case <-follower.stop:
				return
			case <-ticker.C:
				changed, err := syncProgramFileFollow(follower.pager, follower.path, &follower.info)
				if err != nil || !changed {
					continue
				}
				eventQ <- tcell.NewEventInterrupt(nil)
			}
		}
	}()
	return follower
}

func (f *programFileFollower) Stop() {
	if f == nil {
		return
	}
	close(f.stop)
	<-f.done
}

func syncProgramFileFollow(pager *goless.Pager, path string, previous *os.FileInfo) (bool, error) {
	if pager == nil || path == "" || isProgramStdinInput(path) {
		return false, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if previous != nil && *previous != nil && !os.SameFile(*previous, info) {
		if err := reloadProgramInputInPlace(pager, nil, path); err != nil {
			return false, err
		}
		*previous = info
		return true, nil
	}
	size := info.Size()
	current := pager.Len()
	switch {
	case size == current:
		if previous != nil {
			*previous = info
		}
		return false, nil
	case size < current:
		if err := reloadProgramInputInPlace(pager, nil, path); err != nil {
			return false, err
		}
		if previous != nil {
			*previous = info
		}
		return true, nil
	default:
		file, err := os.Open(path)
		if err != nil {
			return false, err
		}
		defer file.Close()
		if _, err := file.Seek(current, io.SeekStart); err != nil {
			return false, err
		}
		if _, err := pager.ReadFrom(file); err != nil {
			return false, err
		}
		if previous != nil {
			*previous = info
		}
		return true, nil
	}
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
) (bool, programViewportSnapshot, os.FileInfo, error) {
	if readResult != nil && *readResult != nil {
		return false, programViewportSnapshot{}, nil, fmt.Errorf("stdin still reading")
	}
	nextPager := buildPager()
	nextPager.SetSize(width, height)
	currentPath := session.currentFile()
	if loader != nil && loader.canStream(currentPath) {
		reader, finish, err := loader.startStdinStream()
		if err != nil {
			return false, programViewportSnapshot{}, nil, err
		}
		var cache bytes.Buffer
		*pager = nextPager
		if readResult != nil {
			*readResult = startProgramRead(*pager, io.TeeReader(reader, &cache), eventQ, func(err error) {
				finish(err, cache.Bytes())
			})
		}
		return false, programViewportSnapshot{}, nil, nil
	}
	snapshot, info, err := loadProgramInput(nextPager, loader, currentPath, session.startup)
	if err != nil {
		return false, programViewportSnapshot{}, nil, err
	}
	*pager = nextPager
	if readResult != nil {
		*readResult = nil
	}
	return true, snapshot, info, nil
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

func version() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return "unknown"
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

func (s *programSession) commandHandler(load func() error, reload func() error, showInformation func(title, body string)) func(goless.Command) goless.CommandResult {
	return func(cmd goless.Command) goless.CommandResult {
		switch cmd.Name {
		case "q", "Q", "quit":
			return goless.CommandResult{Handled: true, Quit: true}
		case "file", "f":
			return goless.CommandResult{Handled: true, Message: s.currentFileLabel()}
		case "license":
			if showInformation != nil {
				showInformation("Apache License 2.0", goless.LicenseText())
			}
			return goless.CommandResult{Handled: true}
		case "version":
			return goless.CommandResult{Handled: true, Message: version()}
		case "reload":
			if reload == nil {
				return goless.CommandResult{Handled: true, Message: "file reload unavailable"}
			}
			if err := reload(); err != nil {
				return goless.CommandResult{Handled: true, Message: err.Error()}
			}
			return goless.CommandResult{Handled: true, Message: s.currentFileLabel()}
		case "next", "n":
			if !s.canNext() {
				return goless.CommandResult{Handled: true, Message: "already at last file"}
			}
			if load == nil {
				return goless.CommandResult{Handled: true, Message: "file reload unavailable"}
			}
			index := s.index
			s.next()
			if err := load(); err != nil {
				s.index = index
				return goless.CommandResult{Handled: true, Message: err.Error()}
			}
			return goless.CommandResult{Handled: true, Message: s.currentFileLabel()}
		case "prev", "p", "previous":
			if !s.canPrev() {
				return goless.CommandResult{Handled: true, Message: "already at first file"}
			}
			if load == nil {
				return goless.CommandResult{Handled: true, Message: "file reload unavailable"}
			}
			index := s.index
			s.prev()
			if err := load(); err != nil {
				s.index = index
				return goless.CommandResult{Handled: true, Message: err.Error()}
			}
			return goless.CommandResult{Handled: true, Message: s.currentFileLabel()}
		case "first", "rewind", "x":
			if !s.hasFiles() || s.index == 0 {
				return goless.CommandResult{Handled: true, Message: "already at first file"}
			}
			if load == nil {
				return goless.CommandResult{Handled: true, Message: "file reload unavailable"}
			}
			index := s.index
			s.first()
			if err := load(); err != nil {
				s.index = index
				return goless.CommandResult{Handled: true, Message: err.Error()}
			}
			return goless.CommandResult{Handled: true, Message: s.currentFileLabel()}
		case "last":
			if !s.hasFiles() || s.index == len(s.files)-1 {
				return goless.CommandResult{Handled: true, Message: "already at last file"}
			}
			if load == nil {
				return goless.CommandResult{Handled: true, Message: "file reload unavailable"}
			}
			index := s.index
			s.last()
			if err := load(); err != nil {
				s.index = index
				return goless.CommandResult{Handled: true, Message: err.Error()}
			}
			return goless.CommandResult{Handled: true, Message: s.currentFileLabel()}
		default:
			return goless.CommandResult{}
		}
	}
}

func handleProgramStatusKey(pager *goless.Pager, ev *tcell.EventKey) bool {
	if pager == nil || ev == nil {
		return false
	}
	if ev.Key() != tcell.KeyCtrlG && !(ev.Key() == tcell.KeyRune && ev.Str() == "=" && ev.Modifiers() == tcell.ModNone) {
		return false
	}

	return runProgramCommand(pager, "file")
}

func shouldHandleProgramStatusKey(result goless.KeyResult) bool {
	return !result.Handled && result.Context == goless.NormalKeyContext
}

func handleProgramReloadKey(pager *goless.Pager, ev *tcell.EventKey) bool {
	if pager == nil || ev == nil {
		return false
	}
	if ev.Key() != tcell.KeyRune || ev.Str() != "R" || ev.Modifiers() != tcell.ModNone {
		return false
	}
	return runProgramCommand(pager, "reload")
}

func shouldHandleProgramReloadKey(result goless.KeyResult) bool {
	return !result.Handled && result.Context == goless.NormalKeyContext
}

func runProgramCommand(pager *goless.Pager, command string) bool {
	if pager == nil || command == "" {
		return false
	}
	pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, ":", tcell.ModNone))
	for _, r := range command {
		pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
	}
	pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	return true
}

func snapshotProgramPager(pager *goless.Pager) programViewportSnapshot {
	if pager == nil {
		return programViewportSnapshot{}
	}
	return programViewportSnapshot{
		eofVisible: pager.EOFVisible(),
		following:  pager.Following(),
	}
}

func programViewportAtOrigin(pager *goless.Pager) bool {
	if pager == nil {
		return true
	}
	if pager.Following() {
		return false
	}
	pos := pager.Position()
	return pos.Row <= 1 && pos.Column <= 1
}

func updateProgramQuitIfOneScreenArm(armed bool, pager *goless.Pager) bool {
	if !armed {
		return false
	}
	return programViewportAtOrigin(pager)
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
	return slices.ContainsFunc(files, isProgramStdinInput)
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

func loadProgramInput(pager *goless.Pager, loader *programInputLoader, path string, startup programStartup) (programViewportSnapshot, os.FileInfo, error) {
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
		return programViewportSnapshot{}, nil, err
	}
	defer file.Close()
	var info os.FileInfo
	if statter, ok := file.(interface{ Stat() (os.FileInfo, error) }); ok && !isProgramStdinInput(path) {
		info, err = statter.Stat()
		if err != nil {
			return programViewportSnapshot{}, nil, err
		}
	}
	if _, err := pager.ReadFrom(file); err != nil {
		return programViewportSnapshot{}, nil, err
	}
	pager.Flush()
	snapshot := snapshotProgramPager(pager)
	applyStartupCommand(pager, startup)
	return snapshot, info, nil
}

func reloadProgramInputInPlace(pager *goless.Pager, loader *programInputLoader, path string) error {
	_, err := reloadProgramInputInPlaceWithInfo(pager, loader, path)
	return err
}

func reloadProgramInputInPlaceWithInfo(pager *goless.Pager, loader *programInputLoader, path string) (os.FileInfo, error) {
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
		return nil, err
	}
	defer file.Close()
	var info os.FileInfo
	if statter, ok := file.(interface{ Stat() (os.FileInfo, error) }); ok && !isProgramStdinInput(path) {
		info, err = statter.Stat()
		if err != nil {
			return nil, err
		}
	}
	if _, err := pager.ReloadFrom(file); err != nil {
		return nil, err
	}
	pager.Flush()
	return info, nil
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
				Foreground(color.Blue).
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
	lineNumbers bool,
	searchCaseMode goless.SearchCaseMode,
	squeeze bool,
	tabWidth int,
	commandHandler func(goless.Command) goless.CommandResult,
) *goless.Pager {
	return goless.New(
		goless.WithTabWidth(tabWidth),
		goless.WithWrapMode(goless.NoWrap),
		goless.WithRenderMode(renderMode),
		goless.WithLineNumbers(lineNumbers),
		goless.WithSearchCaseMode(searchCaseMode),
		goless.WithSqueezeBlankLines(squeeze),
		goless.WithText(programText()),
		goless.WithTheme(preset.Theme),
		goless.WithVisualization(programVisualization(hidden)),
		goless.WithHyperlinkHandler(programHyperlinkHandler(liveLinks)),
		goless.WithCommandHandler(commandHandler),
		goless.WithChrome(chrome),
		goless.WithShowStatus(true),
	)
}

//go:embed  help.txt
var helpText string

func programText() goless.Text {
	text := goless.DefaultText()
	text.HelpBody = helpText
	return text
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
