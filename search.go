// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

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

// SearchState summarizes the current search state visible to an embedder.
//
// CurrentMatch is 1-based. It is 0 when no match is selected.
// Preview reports whether the state came from an in-progress / or ? prompt.
type SearchState struct {
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
