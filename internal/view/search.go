// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"strconv"
	"strings"

	"github.com/gdamore/goless/internal/layout"
	"github.com/gdamore/goless/internal/model"
)

type searchMatch struct {
	LineIndex  int
	StartRune  int
	EndRune    int
	StartGraph int
	EndGraph   int
}

type searchState struct {
	Query   string
	Forward bool
	Matches []searchMatch
	Current int
}

func (v *Viewer) rebuildSearch() {
	if v.search.Query == "" {
		v.search.Matches = nil
		v.search.Current = -1
		return
	}

	var matches []searchMatch
	pattern := []rune(v.search.Query)
	for lineIndex, line := range v.lines {
		text := []rune(line.Text)
		for _, start := range findRuneMatches(text, pattern) {
			matches = append(matches, searchMatch{
				LineIndex:  lineIndex,
				StartRune:  start,
				EndRune:    start + len(pattern),
				StartGraph: graphemeIndexForRune(line, start),
				EndGraph:   graphemeIndexForRuneEnd(line, start+len(pattern)),
			})
		}
	}
	v.search.Matches = matches
	if len(matches) == 0 {
		v.search.Current = -1
		return
	}
	if v.search.Current < 0 || v.search.Current >= len(matches) {
		v.search.Current = 0
	}
}

func (v *Viewer) beginPrompt(kind promptKind) {
	v.mode = modePrompt
	v.prompt = &promptState{kind: kind}
	switch kind {
	case promptSearchForward:
		v.prompt.prefix = "/"
	case promptSearchBackward:
		v.prompt.prefix = "?"
	case promptCommand:
		v.prompt.prefix = ":"
	}
}

func (v *Viewer) cancelPrompt() {
	v.mode = modeNormal
	v.prompt = nil
}

func (v *Viewer) commitPrompt() {
	if v.prompt == nil {
		return
	}

	text := string(v.prompt.buffer)
	switch v.prompt.kind {
	case promptSearchForward:
		v.startSearch(text, true)
	case promptSearchBackward:
		v.startSearch(text, false)
	case promptCommand:
		v.runCommand(text)
	}
	v.cancelPrompt()
}

func (v *Viewer) startSearch(query string, forward bool) {
	query = strings.TrimSpace(query)
	if query == "" {
		v.clearSearch()
		return
	}

	v.ensureLayout()
	v.search.Query = query
	v.search.Forward = forward
	v.search.Current = -1
	v.rebuildSearch()
	if len(v.search.Matches) == 0 {
		v.setMessage(msgPromptNotFound, map[string]any{"Query": query})
		return
	}

	v.search.Current = v.pickInitialMatch(forward)
	v.goToMatch(v.search.Current)
	v.setMessage(msgPromptMatchCount, map[string]any{"Query": query, "Count": len(v.search.Matches)})
}

func (v *Viewer) clearSearch() {
	v.search = searchState{}
	v.setMessage(msgPromptEmptySearch, nil)
}

func (v *Viewer) repeatSearch(forward bool) {
	if len(v.search.Matches) == 0 {
		v.setMessage(msgPromptNoSearch, nil)
		return
	}
	step := 1
	if !forward {
		step = -1
	}
	v.search.Current = (v.search.Current + step + len(v.search.Matches)) % len(v.search.Matches)
	v.goToMatch(v.search.Current)
}

func (v *Viewer) pickInitialMatch(forward bool) int {
	if len(v.search.Matches) == 0 {
		return -1
	}

	anchor := v.firstVisibleAnchor()
	anchorRune := 0
	if anchor.LineIndex >= 0 && anchor.LineIndex < len(v.lines) {
		line := v.lines[anchor.LineIndex]
		if anchor.GraphemeIndex >= 0 && anchor.GraphemeIndex < len(line.Graphemes) {
			anchorRune = line.Graphemes[anchor.GraphemeIndex].RuneStart
		}
	}

	if forward {
		for i, match := range v.search.Matches {
			if match.LineIndex > anchor.LineIndex || (match.LineIndex == anchor.LineIndex && match.StartRune >= anchorRune) {
				return i
			}
		}
		return 0
	}

	for i := len(v.search.Matches) - 1; i >= 0; i-- {
		match := v.search.Matches[i]
		if match.LineIndex < anchor.LineIndex || (match.LineIndex == anchor.LineIndex && match.StartRune <= anchorRune) {
			return i
		}
	}
	return len(v.search.Matches) - 1
}

func (v *Viewer) goToMatch(index int) {
	if index < 0 || index >= len(v.search.Matches) {
		return
	}
	match := v.search.Matches[index]
	if v.cfg.WrapMode == layout.NoWrap {
		v.revealMatchHorizontally(match)
	}
	anchor := layout.Anchor{LineIndex: match.LineIndex, GraphemeIndex: match.StartGraph}
	v.revealAnchor(anchor)
}

func (v *Viewer) revealMatchHorizontally(match searchMatch) {
	if match.LineIndex < 0 || match.LineIndex >= len(v.layout.Lines) {
		return
	}
	info := v.layout.Lines[match.LineIndex]
	if match.StartGraph < 0 || match.StartGraph >= len(info.GraphemeCellStarts) {
		return
	}

	matchStart := info.GraphemeCellStarts[match.StartGraph]
	matchEnd := info.GraphemeCellEnds[match.StartGraph]
	if match.EndGraph >= 0 && match.EndGraph < len(info.GraphemeCellEnds) {
		matchEnd = info.GraphemeCellEnds[match.EndGraph]
	}

	width := max(v.width, 1)
	windowStart := v.colOffset
	windowEnd := windowStart + width
	if matchStart >= windowStart && matchEnd <= windowEnd {
		return
	}

	switch {
	case matchEnd-matchStart >= width:
		v.colOffset = matchStart
	case matchStart < windowStart:
		v.colOffset = matchStart
	case matchEnd > windowEnd:
		v.colOffset = matchEnd - width
	}

	v.relayout()
}

func (v *Viewer) runCommand(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	lineNumber, err := strconv.Atoi(text)
	if err != nil {
		v.setMessage(msgCommandUnknown, map[string]any{"Command": text})
		return
	}
	v.goToLine(lineNumber)
}

func (v *Viewer) goToLine(lineNumber int) {
	if lineNumber <= 0 {
		v.setMessage(msgCommandLineStart, nil)
		return
	}
	v.ensureLayout()
	lineIndex := lineNumber - 1
	if lineIndex >= len(v.lines) {
		v.setMessage(msgCommandOutOfRange, map[string]any{"Line": lineNumber})
		return
	}
	if v.cfg.WrapMode == layout.NoWrap {
		v.colOffset = 0
		v.relayout()
	}
	v.restoreAnchor(layout.Anchor{LineIndex: lineIndex, GraphemeIndex: 0})
	v.setMessage(msgCommandLine, map[string]any{"Line": lineNumber})
}

func (v *Viewer) graphemeMatched(line model.Line, lineIndex int, grapheme model.Grapheme) (bool, bool) {
	if len(v.search.Matches) == 0 {
		return false, false
	}
	for i, match := range v.search.Matches {
		if match.LineIndex != lineIndex {
			continue
		}
		if grapheme.RuneStart >= match.EndRune || grapheme.RuneEnd <= match.StartRune {
			continue
		}
		return true, i == v.search.Current
	}
	return false, false
}

func findRuneMatches(text []rune, pattern []rune) []int {
	if len(pattern) == 0 || len(pattern) > len(text) {
		return nil
	}
	var matches []int
	for i := 0; i+len(pattern) <= len(text); i++ {
		if runeSliceEqual(text[i:i+len(pattern)], pattern) {
			matches = append(matches, i)
		}
	}
	return matches
}

func runeSliceEqual(a []rune, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func graphemeIndexForRune(line model.Line, runeIndex int) int {
	for i, grapheme := range line.Graphemes {
		if runeIndex >= grapheme.RuneStart && runeIndex < grapheme.RuneEnd {
			return i
		}
	}
	if len(line.Graphemes) == 0 {
		return 0
	}
	return len(line.Graphemes) - 1
}

func graphemeIndexForRuneEnd(line model.Line, runeIndex int) int {
	if len(line.Graphemes) == 0 {
		return 0
	}
	if runeIndex <= 0 {
		return 0
	}
	for i, grapheme := range line.Graphemes {
		if runeIndex <= grapheme.RuneEnd {
			return i
		}
	}
	return len(line.Graphemes) - 1
}
