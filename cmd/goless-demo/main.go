package main

import (
	"flag"
	"fmt"
	"io"
	"os"

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
	doc := model.NewDocument(32 * 1024)

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: goless-demo [file]\n")
	}
	flag.Parse()

	var input io.Reader = os.Stdin
	if flag.NArg() > 1 {
		return fmt.Errorf("usage: goless-demo [file]")
	}
	if flag.NArg() == 1 {
		file, err := os.Open(flag.Arg(0))
		if err != nil {
			return err
		}
		defer file.Close()
		input = file
	}

	if err := appendFromReader(doc, input); err != nil {
		return err
	}
	doc.Flush()

	screen, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := screen.Init(); err != nil {
		return err
	}
	defer screen.Fini()

	viewer := view.New(doc, view.Config{
		TabWidth:   8,
		WrapMode:   layout.NoWrap,
		ShowStatus: true,
	})

	width, height := screen.Size()
	viewer.SetSize(width, height)
	viewer.Draw(screen)

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
		}
		viewer.Draw(screen)
	}
}

func appendFromReader(doc *model.Document, r io.Reader) error {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if appendErr := doc.Append(buf[:n]); appendErr != nil {
				return appendErr
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
