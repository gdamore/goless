// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/gdamore/goless"
	"github.com/gdamore/goless/internal/ansi"
	"github.com/gdamore/goless/internal/layout"
	"github.com/gdamore/goless/internal/model"
	"github.com/gdamore/goless/internal/view"
	"github.com/gdamore/tcell/v3"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var chromeName string
	var presetName string
	var renderName string
	var title string

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: goless-demo [-preset none|dark|light|plain|pretty] [-chrome auto|none|single|rounded] [-render hybrid|literal|presentation] [-title text] [file]\n")
	}
	flag.StringVar(&presetName, "preset", "none", "visual preset: none, dark, light, plain, pretty")
	flag.StringVar(&chromeName, "chrome", "auto", "chrome override: auto, none, single, rounded")
	flag.StringVar(&renderName, "render", "hybrid", "render mode: hybrid, literal, presentation")
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
	doc := model.NewDocumentWithMode(32*1024, renderMode)

	var (
		input io.Reader = os.Stdin
		file  *os.File
	)
	if flag.NArg() > 1 {
		return fmt.Errorf("usage: goless-demo [file]")
	}
	if flag.NArg() == 1 {
		var err error
		file, err = os.Open(flag.Arg(0))
		if err != nil {
			return err
		}
		defer file.Close()
		input = file
	}
	if flag.NArg() == 0 && stdinIsTerminal() {
		return fmt.Errorf("stdin is a terminal; specify a file or pipe input")
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := screen.Init(); err != nil {
		return err
	}
	defer screen.Fini()

	chromeCfg, err := demoChrome(chromeName, title, preset.Chrome)
	if err != nil {
		return err
	}

	viewer := view.New(doc, view.Config{
		TabWidth:   8,
		WrapMode:   layout.NoWrap,
		Theme:      demoTheme(preset.Theme),
		Chrome:     chromeCfg,
		ShowStatus: true,
	})

	width, height := screen.Size()
	if width <= 0 || height <= 0 {
		screen.Sync()
		width, height = screen.Size()
	}
	viewer.SetSize(width, height)
	viewer.Draw(screen)

	readResult := make(chan error, 1)
	go readIntoDocument(doc, input, screen.EventQ(), readResult)

	for {
		ev := <-screen.EventQ()
		switch event := ev.(type) {
		case *tcell.EventResize:
			width, height = event.Size()
			viewer.SetSize(width, height)
			screen.Sync()
		case *tcell.EventKey:
			if viewer.HandleKey(event) {
				return nil
			}
		case *tcell.EventInterrupt:
			viewer.Refresh()
			select {
			case err := <-readResult:
				if err != nil {
					return err
				}
			default:
			}
		}
		viewer.Draw(screen)
	}
}

func appendFromReader(doc *model.Document, r io.Reader, onChunk func()) error {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if appendErr := doc.Append(buf[:n]); appendErr != nil {
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

func readIntoDocument(doc *model.Document, r io.Reader, eventQ chan tcell.Event, result chan<- error) {
	err := appendFromReader(doc, r, func() {
		eventQ <- tcell.NewEventInterrupt(nil)
	})
	doc.Flush()
	result <- err
	eventQ <- tcell.NewEventInterrupt(nil)
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

func demoTheme(theme goless.Theme) view.Theme {
	return view.Theme{
		DefaultFG: theme.DefaultFG,
		DefaultBG: theme.DefaultBG,
		ANSI:      theme.ANSI,
	}
}

func demoChrome(name, title string, base goless.Chrome) (view.Chrome, error) {
	chrome := view.Chrome{
		TitleAlign:       view.TitleAlign(base.TitleAlign),
		Title:            base.Title,
		BorderStyle:      base.BorderStyle,
		TitleStyle:       base.TitleStyle,
		StatusStyle:      base.StatusStyle,
		PromptStyle:      base.PromptStyle,
		PromptErrorStyle: base.PromptErrorStyle,
		Frame: view.Frame{
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
		chrome.Frame = view.Frame{}
		return chrome, nil
	default:
		return view.Chrome{}, fmt.Errorf("unknown chrome %q; expected auto, none, single, or rounded", name)
	}

	chrome.Frame = view.Frame{
		Horizontal:  frame.Horizontal,
		Vertical:    frame.Vertical,
		TopLeft:     frame.TopLeft,
		TopRight:    frame.TopRight,
		BottomLeft:  frame.BottomLeft,
		BottomRight: frame.BottomRight,
	}
	return chrome, nil
}

func demoRenderMode(name string) (ansi.RenderMode, error) {
	switch name {
	case "literal":
		return ansi.RenderLiteral, nil
	case "presentation":
		return ansi.RenderPresentation, nil
	case "hybrid", "":
		return ansi.RenderHybrid, nil
	default:
		return ansi.RenderHybrid, fmt.Errorf("unknown render mode %q; expected hybrid, literal, or presentation", name)
	}
}
