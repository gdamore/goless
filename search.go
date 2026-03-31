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
