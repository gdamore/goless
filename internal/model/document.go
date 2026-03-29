package model

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/goless/internal/ansi"
	"github.com/rivo/uniseg"
)

// StyleRun applies a style to a half-open rune range within a line.
type StyleRun struct {
	Start int
	End   int
	Style ansi.Style
}

// Line is a logical line in the parsed document.
type Line struct {
	ByteStart int64
	ByteEnd   int64
	Text      string
	Styles    []StyleRun
	Graphemes []Grapheme
}

// Document incrementally builds logical lines from appended bytes.
type Document struct {
	store     *ChunkStore
	parser    *ansi.Parser
	lines     []Line
	lineStart int64
	current   lineBuilder
}

// Grapheme describes a single visible grapheme cluster within a line.
type Grapheme struct {
	Text      string
	ByteStart int
	ByteEnd   int
	RuneStart int
	RuneEnd   int
	CellWidth int
}

type lineBuilder struct {
	text    strings.Builder
	runes   int
	byteEnd int64
	styles  []StyleRun
}

// NewDocument constructs a new logical document.
func NewDocument(chunkSize int) *Document {
	d := &Document{
		store: NewChunkStore(chunkSize),
	}
	d.parser = ansi.NewParser(d)
	return d
}

// Append appends raw bytes to the document and updates parsed state incrementally.
func (d *Document) Append(data []byte) error {
	d.store.Append(data)
	_, err := d.parser.Write(data)
	return err
}

// Flush finalizes any incomplete parser state.
func (d *Document) Flush() {
	d.parser.Flush()
}

// Len returns the number of raw bytes appended to the document.
func (d *Document) Len() int64 {
	return d.store.Len()
}

// Lines returns the logical lines known so far.
func (d *Document) Lines() []Line {
	lines := append([]Line(nil), d.lines...)
	if d.current.runes > 0 || len(d.lines) == 0 {
		lines = append(lines, d.currentLine())
	}
	return lines
}

// Print implements ansi.Receiver.
func (d *Document) Print(r rune, style ansi.Style, offset int64) {
	d.current.text.WriteRune(r)
	start := d.current.runes
	d.current.runes++
	d.current.byteEnd = offset

	if len(d.current.styles) > 0 {
		last := &d.current.styles[len(d.current.styles)-1]
		if last.Style == style && last.End == start {
			last.End++
			return
		}
	}
	d.current.styles = append(d.current.styles, StyleRun{
		Start: start,
		End:   start + 1,
		Style: style,
	})
}

// Newline implements ansi.Receiver.
func (d *Document) Newline(_ ansi.Style, offset int64) {
	line := d.currentLine()
	line.ByteEnd = offset
	d.lines = append(d.lines, line)

	d.lineStart = offset
	d.current = lineBuilder{}
}

func (d *Document) currentLine() Line {
	styles := append([]StyleRun(nil), d.current.styles...)
	text := d.current.text.String()
	return Line{
		ByteStart: d.lineStart,
		ByteEnd:   d.current.byteEnd,
		Text:      text,
		Styles:    styles,
		Graphemes: segmentGraphemes(text),
	}
}

func segmentGraphemes(text string) []Grapheme {
	if text == "" {
		return nil
	}

	var graphemes []Grapheme
	byteStart := 0
	runeStart := 0
	state := -1
	rest := text

	for rest != "" {
		cluster, next, width, newState := uniseg.FirstGraphemeClusterInString(rest, state)
		runes := utf8.RuneCountInString(cluster)
		graphemes = append(graphemes, Grapheme{
			Text:      cluster,
			ByteStart: byteStart,
			ByteEnd:   byteStart + len(cluster),
			RuneStart: runeStart,
			RuneEnd:   runeStart + runes,
			CellWidth: width,
		})
		byteStart += len(cluster)
		runeStart += runes
		rest = next
		state = newState
	}

	return graphemes
}
