// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"regexp"
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

// SearchMode controls whether search uses literal substring, whole-word, or regex matching.
type SearchMode int

const (
	// SearchSubstring matches the query anywhere in the visible text.
	SearchSubstring SearchMode = iota
	// SearchWholeWord restricts matches to whole-word boundaries.
	SearchWholeWord
	// SearchRegex treats the query as a regular expression.
	SearchRegex
)

type searchMatch struct {
	LineIndex  int
	StartByte  int
	EndByte    int
	StartRune  int
	EndRune    int
	StartGraph int
	EndGraph   int
}

type searchState struct {
	Query        string
	Forward      bool
	CaseMode     SearchCaseMode
	Mode         SearchMode
	Matches      []searchMatch
	Current      int
	CompileError string
}

// SearchSnapshot summarizes the current search state visible to embedders.
//
// CurrentMatch is 1-based. It is 0 when no match is selected.
type SearchSnapshot struct {
	// Query is the current committed or preview search text.
	Query string
	// Forward reports whether the search direction is forward rather than backward.
	Forward bool
	// CaseMode is the active search case mode for this state.
	CaseMode SearchCaseMode
	// Mode is the active search behavior for this state.
	Mode SearchMode
	// MatchCount is the number of matches currently found for Query.
	MatchCount int
	// CurrentMatch is the 1-based selected match index, or 0 when no match is selected.
	CurrentMatch int
	// CompileError reports the current regex compilation error, if any.
	CompileError string
	// Preview reports whether this state comes from an in-progress search prompt.
	Preview bool
}

func (v *Viewer) activeSearch() *searchState {
	if v.mode == modePrompt && v.prompt != nil && v.prompt.preview != nil {
		return v.prompt.preview
	}
	return &v.search
}

// SearchSnapshot reports the current committed or preview search state.
func (v *Viewer) SearchSnapshot() SearchSnapshot {
	if v.mode == modePrompt && v.prompt != nil {
		switch v.prompt.kind {
		case promptSearchForward, promptSearchBackward:
			snapshot := SearchSnapshot{
				Query:        v.prompt.input(),
				Forward:      v.prompt.kind == promptSearchForward,
				CaseMode:     normalizeSearchCaseMode(v.cfg.SearchCase),
				Mode:         normalizeSearchMode(v.cfg.SearchMode),
				CompileError: v.prompt.errText,
				Preview:      true,
			}
			if v.prompt.preview != nil && v.prompt.preview.Query == snapshot.Query && snapshot.CompileError == "" {
				snapshot.MatchCount = len(v.prompt.preview.Matches)
				if v.prompt.preview.Current >= 0 && v.prompt.preview.Current < snapshot.MatchCount {
					snapshot.CurrentMatch = v.prompt.preview.Current + 1
				}
			}
			return snapshot
		}
	}

	snapshot := SearchSnapshot{
		Query:   v.search.Query,
		Forward: v.search.Forward,
		Preview: false,
	}
	if snapshot.Query != "" {
		snapshot.CaseMode = normalizeSearchCaseMode(v.search.CaseMode)
		snapshot.Mode = normalizeSearchMode(v.search.Mode)
		snapshot.CompileError = v.search.CompileError
		snapshot.MatchCount = len(v.search.Matches)
		if v.search.Current >= 0 && v.search.Current < snapshot.MatchCount {
			snapshot.CurrentMatch = v.search.Current + 1
		}
		return snapshot
	}

	snapshot.Forward = true
	snapshot.CaseMode = normalizeSearchCaseMode(v.cfg.SearchCase)
	snapshot.Mode = normalizeSearchMode(v.cfg.SearchMode)
	return snapshot
}

func searchCaseLabel(mode SearchCaseMode) string {
	switch normalizeSearchCaseMode(mode) {
	case SearchCaseSensitive:
		return "AA"
	case SearchCaseInsensitive:
		return "Aa"
	default:
		return "A?"
	}
}

func searchBehaviorLabel(mode SearchMode) string {
	switch normalizeSearchMode(mode) {
	case SearchWholeWord:
		return "a␣"
	case SearchRegex:
		return ".*"
	default:
		return "ab"
	}
}

func searchModeLabel(caseMode SearchCaseMode, mode SearchMode) string {
	return searchCaseLabel(caseMode) + " " + searchBehaviorLabel(mode)
}

func searchModeHintText(caseMode SearchCaseMode, mode SearchMode) string {
	return "F2:" + searchCaseLabel(caseMode) + " F3:" + searchBehaviorLabel(mode)
}

func (v *Viewer) searchModeLabel() string {
	return searchModeLabel(v.cfg.SearchCase, v.cfg.SearchMode)
}

func (v *Viewer) searchModeHintText() string {
	return searchModeHintText(v.cfg.SearchCase, v.cfg.SearchMode)
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
	return next
}

func (v *Viewer) CycleSearchMode() SearchMode {
	next := SearchSubstring
	switch normalizeSearchMode(v.cfg.SearchMode) {
	case SearchSubstring:
		next = SearchWholeWord
	case SearchWholeWord:
		next = SearchRegex
	case SearchRegex:
		next = SearchSubstring
	}
	v.SetSearchMode(next)
	return next
}

func (v *Viewer) rebuildSearch() {
	v.rebuildSearchState(&v.search)
}

func (v *Viewer) rebuildSearchState(state *searchState) {
	if state == nil {
		return
	}
	if state.Query == "" {
		state.Matches = nil
		state.Current = -1
		state.CompileError = ""
		return
	}

	state.CompileError = ""
	var matches []searchMatch
	pattern := state.Query
	effectiveCase := resolveSearchCaseMode(state.CaseMode, pattern)
	switch normalizeSearchMode(state.Mode) {
	case SearchRegex:
		re, err := compileSearchRegexp(pattern, effectiveCase)
		if err != nil {
			state.Matches = nil
			state.Current = -1
			state.CompileError = "regex:error " + err.Error()
			return
		}
		for lineIndex, line := range v.lines {
			for _, match := range findRegexpMatches(line, re) {
				match.LineIndex = lineIndex
				matches = append(matches, match)
			}
		}
	default:
		patternRunes := utf8.RuneCountInString(pattern)
		for lineIndex, line := range v.lines {
			for _, match := range findStringMatches(line, pattern, patternRunes, effectiveCase, normalizeSearchMode(state.Mode)) {
				match.LineIndex = lineIndex
				matches = append(matches, match)
			}
		}
	}

	state.Matches = matches
	if len(matches) == 0 {
		state.Current = -1
		return
	}
	if state.Current < 0 || state.Current >= len(matches) {
		state.Current = 0
	}
}

func (v *Viewer) beginPrompt(kind promptKind) {
	v.mode = modePrompt
	v.prompt = &promptState{kind: kind}
	switch kind {
	case promptSearchForward, promptSearchBackward:
		if v.search.Query != "" {
			v.prompt.editor.SetText(v.search.Query)
			v.prompt.seeded = true
		}
	}
	historyKind := historyKindForPrompt(kind)
	v.prompt.history = promptHistoryState{
		kind:  historyKind,
		index: len(v.history[historyKind]),
		draft: v.prompt.input(),
	}
	v.updatePromptPrefix()
	v.updatePromptPreview()
}

func (v *Viewer) cancelPrompt() {
	if v.mode == modePrompt {
		v.mode = modeNormal
	}
	v.prompt = nil
}

func (v *Viewer) commitPrompt() KeyResult {
	if v.prompt == nil {
		return KeyResult{}
	}

	text := v.prompt.input()
	seeded := v.prompt.seeded
	commit := true
	quit := false
	switch v.prompt.kind {
	case promptSearchForward:
		commit = v.commitPromptSearch(text, true)
	case promptSearchBackward:
		commit = v.commitPromptSearch(text, false)
	case promptCommand:
		commit, quit = v.runCommand(text)
	}
	if commit {
		v.recordPromptHistory(v.prompt.kind, text, seeded)
		v.cancelPrompt()
	}
	if quit {
		return KeyResult{Handled: true, Quit: true, Context: KeyContextPrompt}
	}
	return KeyResult{Handled: true, Context: KeyContextPrompt}
}

func (v *Viewer) commitPromptSearch(text string, forward bool) bool {
	if text == "" || (v.prompt != nil && v.prompt.seeded) {
		v.clearSearch()
		return true
	}
	if v.prompt != nil && v.prompt.errText != "" {
		v.message = v.prompt.errText
		return false
	}

	v.follow = false
	v.ensureLayout()
	v.search = searchState{
		Query:    text,
		Forward:  forward,
		CaseMode: normalizeSearchCaseMode(v.cfg.SearchCase),
		Mode:     normalizeSearchMode(v.cfg.SearchMode),
		Current:  -1,
	}
	v.rebuildSearch()
	if v.search.CompileError != "" {
		v.message = v.search.CompileError
		return false
	}
	if len(v.search.Matches) == 0 {
		v.message = v.text.SearchNotFound(text)
		return true
	}
	if v.search.Current < 0 || v.search.Current >= len(v.search.Matches) {
		v.search.Current = v.pickInitialMatch(forward)
	}
	v.goToMatch(v.search.Current)
	v.message = v.text.SearchMatchCount(text, len(v.search.Matches))
	return true
}

func (v *Viewer) updatePromptPrefix() {
	if v.prompt == nil {
		return
	}
	switch v.prompt.kind {
	case promptSearchForward:
		v.prompt.prefix = "/"
	case promptSearchBackward:
		v.prompt.prefix = "?"
	case promptCommand:
		v.prompt.prefix = ":"
	}
}

func (v *Viewer) updatePromptPreview() {
	if v.prompt == nil {
		return
	}
	switch v.prompt.kind {
	case promptSearchForward, promptSearchBackward:
	default:
		return
	}

	query := v.prompt.input()
	if query == "" {
		v.prompt.preview = nil
		v.prompt.errText = ""
		return
	}

	preview := &searchState{
		Query:    query,
		Forward:  v.prompt.kind == promptSearchForward,
		CaseMode: normalizeSearchCaseMode(v.cfg.SearchCase),
		Mode:     normalizeSearchMode(v.cfg.SearchMode),
		Current:  -1,
	}
	v.ensureLayout()
	v.rebuildSearchState(preview)
	if preview.CompileError != "" {
		v.prompt.errText = preview.CompileError
		return
	}
	v.prompt.errText = ""
	if len(preview.Matches) > 0 {
		preview.Current = v.pickInitialPreviewMatch(preview)
	}
	v.prompt.preview = preview
}

func (v *Viewer) startSearch(query string, forward bool, mode SearchCaseMode) {
	if query == "" {
		v.clearSearch()
		return
	}

	v.follow = false
	v.ensureLayout()
	v.search.Query = query
	v.search.Forward = forward
	v.search.CaseMode = normalizeSearchCaseMode(mode)
	v.search.Mode = normalizeSearchMode(v.cfg.SearchMode)
	v.search.Current = -1
	v.rebuildSearch()
	if v.search.CompileError != "" {
		v.message = v.search.CompileError
		return
	}
	if len(v.search.Matches) == 0 {
		v.message = v.text.SearchNotFound(query)
		return
	}

	v.search.Current = v.pickInitialMatch(forward)
	v.goToMatch(v.search.Current)
	v.message = v.text.SearchMatchCount(query, len(v.search.Matches))
}

// SearchForward starts a forward search and reports whether any match exists.
func (v *Viewer) SearchForward(query string) bool {
	v.startSearch(query, true, v.cfg.SearchCase)
	return len(v.search.Matches) > 0
}

// SearchForwardWithCase starts a forward search with the supplied case mode.
func (v *Viewer) SearchForwardWithCase(query string, mode SearchCaseMode) bool {
	v.startSearch(query, true, mode)
	return len(v.search.Matches) > 0
}

// SearchBackward starts a backward search and reports whether any match exists.
func (v *Viewer) SearchBackward(query string) bool {
	v.startSearch(query, false, v.cfg.SearchCase)
	return len(v.search.Matches) > 0
}

// SearchBackwardWithCase starts a backward search with the supplied case mode.
func (v *Viewer) SearchBackwardWithCase(query string, mode SearchCaseMode) bool {
	v.startSearch(query, false, mode)
	return len(v.search.Matches) > 0
}

func (v *Viewer) clearSearch() {
	v.search = searchState{}
	v.message = ""
}

// ClearSearch removes any active search state.
func (v *Viewer) ClearSearch() {
	v.clearSearch()
}

func (v *Viewer) repeatSearch(forward bool) {
	if len(v.search.Matches) == 0 {
		if v.search.CompileError != "" {
			v.message = v.search.CompileError
			return
		}
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
	return v.pickMatchAtAnchor(v.search.Matches, forward)
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

	frozen := v.headerColumnWidth(v.rawContentWidth())
	if matchEnd <= frozen {
		return
	}
	width := max(v.bodyContentWidth(), 1)
	windowStart := frozen + v.colOffset
	windowEnd := windowStart + width
	if matchStart >= windowStart && matchEnd <= windowEnd {
		return
	}

	switch {
	case matchEnd-matchStart >= width:
		v.colOffset = max(matchStart-frozen, 0)
	case matchStart < windowStart:
		v.colOffset = max(matchStart-frozen, 0)
	case matchEnd > windowEnd:
		v.colOffset = max(matchEnd-frozen-width, 0)
	}

	v.relayout()
}

func (v *Viewer) runCommand(text string) (commit bool, quit bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return true, false
	}

	if v.runSetCommand(text) {
		return true, false
	}

	if percent, ok := parsePercentCommand(text); ok {
		v.goToPercent(percent)
		return true, false
	}

	lineNumber, err := strconv.Atoi(text)
	if err == nil {
		v.goToLine(lineNumber)
		return true, false
	}

	if v.cfg.CommandHandler != nil {
		result := v.cfg.CommandHandler(parseCommand(text))
		if result.Handled {
			if result.Message != "" {
				v.message = result.Message
			}
			return !result.KeepPrompt, result.Quit
		}
	}

	v.message = v.text.CommandUnknown(text)
	return true, false
}

func historyKindForPrompt(kind promptKind) promptHistoryKind {
	switch kind {
	case promptCommand:
		return promptHistoryCommand
	default:
		return promptHistorySearch
	}
}

func (v *Viewer) recordPromptHistory(kind promptKind, text string, seeded bool) {
	historyKind := historyKindForPrompt(kind)
	switch historyKind {
	case promptHistoryCommand:
		text = strings.TrimSpace(text)
	default:
		if seeded {
			return
		}
	}
	if text == "" {
		return
	}

	entries := v.history[historyKind]
	filtered := entries[:0]
	for _, entry := range entries {
		if entry != text {
			filtered = append(filtered, entry)
		}
	}
	filtered = append(filtered, text)
	const maxPromptHistory = 16
	if len(filtered) > maxPromptHistory {
		filtered = filtered[len(filtered)-maxPromptHistory:]
	}
	v.history[historyKind] = filtered
}

func (v *Viewer) recallPromptHistory(step int) {
	if v.prompt == nil || step == 0 {
		return
	}

	entries := v.history[v.prompt.history.kind]
	if len(entries) == 0 {
		return
	}

	switch {
	case step < 0:
		if v.prompt.history.index <= 0 {
			return
		}
		if v.prompt.history.index == len(entries) {
			v.prompt.history.draft = v.prompt.input()
		}
		v.prompt.history.index--
		v.prompt.editor.SetText(entries[v.prompt.history.index])
	case step > 0:
		if v.prompt.history.index >= len(entries) {
			return
		}
		v.prompt.history.index++
		if v.prompt.history.index == len(entries) {
			v.prompt.editor.SetText(v.prompt.history.draft)
		} else {
			v.prompt.editor.SetText(entries[v.prompt.history.index])
		}
	}

	v.prompt.seeded = false
	v.updatePromptPreview()
}

func parsePercentCommand(text string) (int, bool) {
	if !strings.HasSuffix(text, "%") {
		return 0, false
	}
	raw := strings.TrimSpace(strings.TrimSuffix(text, "%"))
	if raw == "" {
		return 0, false
	}
	percent, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return percent, true
}

func parseSetAssignment(text string) (name, value string, ok bool) {
	index := strings.Index(text, "=")
	if index < 0 {
		return "", "", false
	}
	name = strings.TrimSpace(text[:index])
	value = strings.TrimSpace(text[index+1:])
	if name == "" || value == "" || strings.ContainsAny(name, " \t\r\n") || strings.ContainsAny(value, " \t\r\n") {
		return "", "", false
	}
	return name, value, true
}

func visualizationEnabled(visual Visualization) bool {
	return visual.ShowTabs || visual.ShowNewlines || visual.ShowCarriageReturns || visual.ShowEOF
}

func setVisualizationEnabled(visual Visualization, enabled bool) Visualization {
	visual.ShowTabs = enabled
	visual.ShowNewlines = enabled
	visual.ShowCarriageReturns = enabled
	visual.ShowEOF = enabled
	return visual
}

func (v *Viewer) runSetCommand(text string) bool {
	fields := strings.Fields(text)
	if len(fields) == 0 || fields[0] != "set" {
		return false
	}

	setText := strings.TrimSpace(strings.TrimPrefix(text, "set"))
	if setText == "" {
		return false
	}

	if name, value, ok := parseSetAssignment(setText); ok {
		switch name {
		case "searchcase":
			var mode SearchCaseMode
			switch value {
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
			v.message = ""
			return true
		case "searchmode":
			var mode SearchMode
			switch value {
			case "sub", "substring":
				mode = SearchSubstring
			case "word", "wholeword":
				mode = SearchWholeWord
			case "regex":
				mode = SearchRegex
			default:
				v.message = v.text.CommandUnknown(text)
				return true
			}
			v.SetSearchMode(mode)
			v.message = ""
			return true
		case "tabstop":
			width, err := strconv.Atoi(value)
			if err != nil || width <= 0 {
				v.message = v.text.CommandUnknown(text)
				return true
			}
			v.SetTabWidth(width)
			v.message = ""
			return true
		case "pinlines":
			count, err := strconv.Atoi(value)
			if err != nil || count < 0 {
				v.message = v.text.CommandUnknown(text)
				return true
			}
			v.SetHeaderLines(count)
			v.message = ""
			return true
		case "pincols":
			count, err := strconv.Atoi(value)
			if err != nil || count < 0 {
				v.message = v.text.CommandUnknown(text)
				return true
			}
			v.SetHeaderColumns(count)
			v.message = ""
			return true
		default:
			v.message = v.text.CommandUnknown(text)
			return true
		}
	}

	if len(fields) != 2 {
		v.message = v.text.CommandUnknown(text)
		return true
	}

	switch fields[1] {
	case "number":
		v.SetLineNumbers(true)
	case "nonumber":
		v.SetLineNumbers(false)
	case "invnumber":
		v.ToggleLineNumbers()
	case "wrap":
		v.SetWrapMode(layout.SoftWrap)
	case "nowrap":
		v.SetWrapMode(layout.NoWrap)
	case "invwrap":
		v.ToggleWrap()
	case "list":
		v.SetVisualization(setVisualizationEnabled(v.cfg.Visualization, true))
	case "nolist":
		v.SetVisualization(setVisualizationEnabled(v.cfg.Visualization, false))
	case "invlist":
		v.SetVisualization(setVisualizationEnabled(v.cfg.Visualization, !visualizationEnabled(v.cfg.Visualization)))
	case "squeeze":
		v.SetSqueezeBlankLines(true)
	case "nosqueeze":
		v.SetSqueezeBlankLines(false)
	case "invsqueeze":
		v.SetSqueezeBlankLines(!v.SqueezeBlankLines())
	case "ignorecase":
		v.SetSearchCaseMode(SearchCaseInsensitive)
	case "noignorecase":
		v.SetSearchCaseMode(SearchCaseSensitive)
	case "smartcase":
		v.SetSearchCaseMode(SearchSmartCase)
	case "nosmartcase":
		if v.SearchCaseMode() == SearchSmartCase {
			v.SetSearchCaseMode(SearchCaseInsensitive)
		}
	default:
		return false
	}

	v.message = ""
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
	if lineIndex >= len(v.sourceLines) {
		v.message = v.text.CommandOutOfRange(lineNumber)
		return
	}
	if v.cfg.WrapMode == layout.NoWrap {
		v.colOffset = 0
		v.relayout()
	}
	v.restoreSourceAnchor(layout.Anchor{LineIndex: lineIndex, GraphemeIndex: 0})
	v.message = v.text.CommandLine(lineNumber)
}

func (v *Viewer) goToPercent(percent int) {
	if percent < 0 || percent > 100 {
		v.message = v.text.CommandUnknown(strconv.Itoa(percent) + "%")
		return
	}
	if !v.GoPercent(percent) {
		return
	}
	v.message = strconv.Itoa(percent) + "%"
}

// JumpToLine moves the viewport to the requested logical line number.
func (v *Viewer) JumpToLine(lineNumber int) bool {
	v.goToLine(lineNumber)
	return lineNumber > 0 && lineNumber <= len(v.sourceLines)
}

func (v *Viewer) graphemeMatched(line model.Line, lineIndex int, grapheme model.Grapheme) (bool, bool) {
	search := v.activeSearch()
	if len(search.Matches) == 0 {
		return false, false
	}
	matched := false
	for i, match := range search.Matches {
		if match.LineIndex != lineIndex {
			continue
		}
		if grapheme.RuneStart >= match.EndRune || grapheme.RuneEnd <= match.StartRune {
			continue
		}
		matched = true
		if i == search.Current {
			return true, true
		}
	}
	return matched, false
}

func (v *Viewer) pickInitialPreviewMatch(search *searchState) int {
	if search == nil {
		return -1
	}
	return v.pickMatchAtAnchor(search.Matches, search.Forward)
}

func (v *Viewer) pickMatchAtAnchor(matches []searchMatch, forward bool) int {
	if len(matches) == 0 {
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
		for i, match := range matches {
			if match.LineIndex > anchor.LineIndex || (match.LineIndex == anchor.LineIndex && match.StartRune >= anchorRune) {
				return i
			}
		}
		return 0
	}

	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		if match.LineIndex < anchor.LineIndex || (match.LineIndex == anchor.LineIndex && match.StartRune <= anchorRune) {
			return i
		}
	}
	return len(matches) - 1
}

func findStringMatches(line model.Line, pattern string, patternRunes int, caseMode SearchCaseMode, mode SearchMode) []searchMatch {
	if pattern == "" || len(pattern) > len(line.Text) {
		return nil
	}
	if caseMode != SearchCaseInsensitive {
		return findSensitiveMatches(line, pattern, patternRunes, mode)
	}

	patternFold := []rune(pattern)
	var matches []searchMatch
	text := line.Text
	for startByte, startRune := 0, 0; startByte < len(text); {
		endByte, endRune, ok := foldedPrefixRuneEnd(text[startByte:], patternFold, startRune)
		if ok {
			match := searchMatch{
				StartByte:  startByte,
				EndByte:    startByte + endByte,
				StartRune:  startRune,
				EndRune:    endRune,
				StartGraph: graphemeIndexForRune(line, startRune),
				EndGraph:   graphemeIndexForRuneEnd(line, endRune),
			}
			if matchAllowed(line.Text, match, mode) {
				matches = append(matches, match)
			}
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

func findSensitiveMatches(line model.Line, pattern string, patternRunes int, mode SearchMode) []searchMatch {
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
		match := searchMatch{
			StartByte:  startByte,
			EndByte:    startByte + len(pattern),
			StartRune:  startRune,
			EndRune:    endRune,
			StartGraph: graphemeIndexForRune(line, startRune),
			EndGraph:   graphemeIndexForRuneEnd(line, endRune),
		}
		if matchAllowed(line.Text, match, mode) {
			matches = append(matches, match)
		}

		_, width := utf8.DecodeRuneInString(text[startByte:])
		if width <= 0 {
			width = 1
		}
		searchByte = startByte + width
		searchRune = startRune + 1
	}

	return matches
}

func findRegexpMatches(line model.Line, re *regexp.Regexp) []searchMatch {
	if re == nil || line.Text == "" {
		return nil
	}
	var matches []searchMatch
	for _, loc := range re.FindAllStringIndex(line.Text, -1) {
		startByte, endByte := loc[0], loc[1]
		if startByte == endByte {
			continue
		}
		startRune := utf8.RuneCountInString(line.Text[:startByte])
		endRune := startRune + utf8.RuneCountInString(line.Text[startByte:endByte])
		matches = append(matches, searchMatch{
			StartByte:  startByte,
			EndByte:    endByte,
			StartRune:  startRune,
			EndRune:    endRune,
			StartGraph: graphemeIndexForRune(line, startRune),
			EndGraph:   graphemeIndexForRuneEnd(line, endRune),
		})
	}
	return matches
}

func foldedPrefixRuneEnd(text string, pattern []rune, startRune int) (int, int, bool) {
	byteOffset := 0
	runeIndex := startRune
	for _, want := range pattern {
		if byteOffset >= len(text) {
			return 0, 0, false
		}
		got, width := utf8.DecodeRuneInString(text[byteOffset:])
		if width <= 0 || !equalFoldRune(got, want) {
			return 0, 0, false
		}
		byteOffset += width
		runeIndex++
	}
	return byteOffset, runeIndex, true
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

func normalizeSearchMode(mode SearchMode) SearchMode {
	switch mode {
	case SearchWholeWord, SearchRegex:
		return mode
	default:
		return SearchSubstring
	}
}

func matchAllowed(text string, match searchMatch, mode SearchMode) bool {
	if normalizeSearchMode(mode) != SearchWholeWord {
		return true
	}
	return isWholeWordMatch(text, match.StartByte, match.EndByte)
}

func compileSearchRegexp(pattern string, caseMode SearchCaseMode) (*regexp.Regexp, error) {
	if resolveSearchCaseMode(caseMode, pattern) == SearchCaseInsensitive {
		pattern = "(?i)" + pattern
	}
	return regexp.Compile(pattern)
}

func isWholeWordMatch(text string, startByte, endByte int) bool {
	if startByte > 0 {
		prev, _ := utf8.DecodeLastRuneInString(text[:startByte])
		if isWordRune(prev) {
			return false
		}
	}
	if endByte < len(text) {
		next, _ := utf8.DecodeRuneInString(text[endByte:])
		if isWordRune(next) {
			return false
		}
	}
	return true
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) || r == '_'
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
