// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/goless/internal/layout"
	"github.com/gdamore/goless/internal/model"
)

// SearchCaseMode controls literal search case matching.
type SearchCaseMode int

const (
	// SearchSmartCase uses case-insensitive matching unless the query contains an uppercase rune.
	SearchSmartCase SearchCaseMode = iota
	// SearchCaseSensitive requires exact rune case matches.
	SearchCaseSensitive
	// SearchCaseInsensitive matches runes case-insensitively.
	SearchCaseInsensitive
)

type searchMatch struct {
	LineIndex  int
	StartRune  int
	EndRune    int
	StartGraph int
	EndGraph   int
}

type searchState struct {
	Query    string
	Forward  bool
	CaseMode SearchCaseMode
	Matches  []searchMatch
	Current  int
}

func (v *Viewer) searchCaseLabel() string {
	switch normalizeSearchCaseMode(v.cfg.SearchCase) {
	case SearchCaseSensitive:
		return "case"
	case SearchCaseInsensitive:
		return "nocase"
	default:
		return "smart"
	}
}

func (v *Viewer) CycleSearchCaseMode() SearchCaseMode {
	next := SearchSmartCase
	switch normalizeSearchCaseMode(v.cfg.SearchCase) {
	case SearchSmartCase:
		next = SearchCaseSensitive
	case SearchCaseSensitive:
		next = SearchCaseInsensitive
	case SearchCaseInsensitive:
		next = SearchSmartCase
	}
	v.SetSearchCaseMode(next)
	v.message = "search:" + v.searchCaseLabel()
	return next
}

func (v *Viewer) rebuildSearch() {
	if v.search.Query == "" {
		v.search.Matches = nil
		v.search.Current = -1
		return
	}

	var matches []searchMatch
	pattern := v.search.Query
	patternRunes := utf8.RuneCountInString(pattern)
	effectiveCase := resolveSearchCaseMode(v.search.CaseMode, pattern)
	for lineIndex, line := range v.lines {
		for _, match := range findStringMatches(line, pattern, patternRunes, effectiveCase) {
			match.LineIndex = lineIndex
			matches = append(matches, searchMatch{
				LineIndex:  match.LineIndex,
				StartRune:  match.StartRune,
				EndRune:    match.EndRune,
				StartGraph: match.StartGraph,
				EndGraph:   match.EndGraph,
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
	v.updatePromptPrefix()
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
		v.startSearch(text, true, v.cfg.SearchCase)
	case promptSearchBackward:
		v.startSearch(text, false, v.cfg.SearchCase)
	case promptCommand:
		v.runCommand(text)
	}
	v.cancelPrompt()
}

func (v *Viewer) updatePromptPrefix() {
	if v.prompt == nil {
		return
	}
	switch v.prompt.kind {
	case promptSearchForward:
		v.prompt.prefix = "/[" + v.searchCaseLabel() + "] "
	case promptSearchBackward:
		v.prompt.prefix = "?[" + v.searchCaseLabel() + "] "
	case promptCommand:
		v.prompt.prefix = ":"
	}
}

func (v *Viewer) startSearch(query string, forward bool, mode SearchCaseMode) {
	query = strings.TrimSpace(query)
	if query == "" {
		v.clearSearch()
		return
	}

	v.follow = false
	v.ensureLayout()
	v.search.Query = query
	v.search.Forward = forward
	v.search.CaseMode = normalizeSearchCaseMode(mode)
	v.search.Current = -1
	v.rebuildSearch()
	if len(v.search.Matches) == 0 {
		v.message = v.text.SearchNotFound(query)
		return
	}

	v.search.Current = v.pickInitialMatch(forward)
	v.goToMatch(v.search.Current)
	v.message = v.text.SearchMatchCount(query, len(v.search.Matches))
}

// SearchForward starts a forward literal search and reports whether any match exists.
func (v *Viewer) SearchForward(query string) bool {
	v.startSearch(query, true, v.cfg.SearchCase)
	return len(v.search.Matches) > 0
}

// SearchForwardWithCase starts a forward literal search with the supplied case mode.
func (v *Viewer) SearchForwardWithCase(query string, mode SearchCaseMode) bool {
	v.startSearch(query, true, mode)
	return len(v.search.Matches) > 0
}

// SearchBackward starts a backward literal search and reports whether any match exists.
func (v *Viewer) SearchBackward(query string) bool {
	v.startSearch(query, false, v.cfg.SearchCase)
	return len(v.search.Matches) > 0
}

// SearchBackwardWithCase starts a backward literal search with the supplied case mode.
func (v *Viewer) SearchBackwardWithCase(query string, mode SearchCaseMode) bool {
	v.startSearch(query, false, mode)
	return len(v.search.Matches) > 0
}

func (v *Viewer) clearSearch() {
	v.search = searchState{}
	v.message = v.text.SearchEmpty
}

// ClearSearch removes any active search state.
func (v *Viewer) ClearSearch() {
	v.clearSearch()
}

func (v *Viewer) repeatSearch(forward bool) {
	if len(v.search.Matches) == 0 {
		v.message = v.text.SearchNone
		return
	}
	v.follow = false
	step := 1
	if !forward {
		step = -1
	}
	v.search.Current = (v.search.Current + step + len(v.search.Matches)) % len(v.search.Matches)
	v.goToMatch(v.search.Current)
}

// SearchNext advances to the next match in the active search direction.
func (v *Viewer) SearchNext() bool {
	if len(v.search.Matches) == 0 {
		v.repeatSearch(v.search.Forward)
		return false
	}
	v.follow = false
	v.repeatSearch(v.search.Forward)
	return true
}

// SearchPrev advances to the previous match relative to the active search direction.
func (v *Viewer) SearchPrev() bool {
	if len(v.search.Matches) == 0 {
		v.repeatSearch(!v.search.Forward)
		return false
	}
	v.follow = false
	v.repeatSearch(!v.search.Forward)
	return true
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

	if v.runSetCommand(text) {
		return
	}

	lineNumber, err := strconv.Atoi(text)
	if err != nil {
		v.message = v.text.CommandUnknown(text)
		return
	}
	v.goToLine(lineNumber)
}

func (v *Viewer) runSetCommand(text string) bool {
	fields := strings.Fields(text)
	if len(fields) != 3 || fields[0] != "set" || fields[1] != "searchcase" {
		return false
	}

	var mode SearchCaseMode
	switch fields[2] {
	case "smart":
		mode = SearchSmartCase
	case "case", "sensitive":
		mode = SearchCaseSensitive
	case "nocase", "insensitive":
		mode = SearchCaseInsensitive
	default:
		v.message = v.text.CommandUnknown(text)
		return true
	}

	v.SetSearchCaseMode(mode)
	v.message = "search:" + v.searchCaseLabel()
	return true
}

func (v *Viewer) goToLine(lineNumber int) {
	if lineNumber <= 0 {
		v.message = v.text.CommandLineStart
		return
	}
	v.follow = false
	v.ensureLayout()
	lineIndex := lineNumber - 1
	if lineIndex >= len(v.lines) {
		v.message = v.text.CommandOutOfRange(lineNumber)
		return
	}
	if v.cfg.WrapMode == layout.NoWrap {
		v.colOffset = 0
		v.relayout()
	}
	v.restoreAnchor(layout.Anchor{LineIndex: lineIndex, GraphemeIndex: 0})
	v.message = v.text.CommandLine(lineNumber)
}

// JumpToLine moves the viewport to the requested logical line.
func (v *Viewer) JumpToLine(lineNumber int) bool {
	v.goToLine(lineNumber)
	return lineNumber > 0 && lineNumber <= len(v.lines)
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

func findStringMatches(line model.Line, pattern string, patternRunes int, mode SearchCaseMode) []searchMatch {
	if pattern == "" || len(pattern) > len(line.Text) {
		return nil
	}
	if mode != SearchCaseInsensitive {
		return findSensitiveMatches(line, pattern, patternRunes)
	}

	patternFold := []rune(pattern)
	var matches []searchMatch
	text := line.Text
	for startByte, startRune := 0, 0; startByte < len(text); {
		endRune, ok := foldedPrefixRuneEnd(text[startByte:], patternFold, startRune)
		if ok {
			matches = append(matches, searchMatch{
				StartRune:  startRune,
				EndRune:    endRune,
				StartGraph: graphemeIndexForRune(line, startRune),
				EndGraph:   graphemeIndexForRuneEnd(line, endRune),
			})
		}

		_, width := utf8.DecodeRuneInString(text[startByte:])
		if width <= 0 {
			width = 1
		}
		startByte += width
		startRune++
	}

	return matches
}

func findSensitiveMatches(line model.Line, pattern string, patternRunes int) []searchMatch {
	var matches []searchMatch
	searchByte := 0
	searchRune := 0
	text := line.Text

	for searchByte <= len(text)-len(pattern) {
		idx := strings.Index(text[searchByte:], pattern)
		if idx < 0 {
			break
		}

		startByte := searchByte + idx
		startRune := searchRune + utf8.RuneCountInString(text[searchByte:startByte])
		endRune := startRune + patternRunes
		matches = append(matches, searchMatch{
			StartRune:  startRune,
			EndRune:    endRune,
			StartGraph: graphemeIndexForRune(line, startRune),
			EndGraph:   graphemeIndexForRuneEnd(line, endRune),
		})

		_, width := utf8.DecodeRuneInString(text[startByte:])
		if width <= 0 {
			width = 1
		}
		searchByte = startByte + width
		searchRune = startRune + 1
	}

	return matches
}

func foldedPrefixRuneEnd(text string, pattern []rune, startRune int) (int, bool) {
	byteOffset := 0
	runeIndex := startRune
	for _, want := range pattern {
		if byteOffset >= len(text) {
			return 0, false
		}
		got, width := utf8.DecodeRuneInString(text[byteOffset:])
		if width <= 0 || !equalFoldRune(got, want) {
			return 0, false
		}
		byteOffset += width
		runeIndex++
	}
	return runeIndex, true
}

func equalFoldRune(a, b rune) bool {
	if a == b {
		return true
	}
	for folded := unicode.SimpleFold(a); folded != a; folded = unicode.SimpleFold(folded) {
		if folded == b {
			return true
		}
	}
	return false
}

func resolveSearchCaseMode(mode SearchCaseMode, query string) SearchCaseMode {
	mode = normalizeSearchCaseMode(mode)
	if mode != SearchSmartCase {
		return mode
	}
	if queryHasUppercase(query) {
		return SearchCaseSensitive
	}
	return SearchCaseInsensitive
}

func normalizeSearchCaseMode(mode SearchCaseMode) SearchCaseMode {
	switch mode {
	case SearchCaseSensitive, SearchCaseInsensitive:
		return mode
	default:
		return SearchSmartCase
	}
}

func queryHasUppercase(query string) bool {
	for _, r := range query {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
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
