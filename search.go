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

// SearchWordMode controls whether literal search matches substrings or whole words.
type SearchWordMode int

const (
	// SearchSubstring matches the query anywhere in the visible text.
	SearchSubstring SearchWordMode = iota
	// SearchWholeWord restricts matches to whole-word boundaries.
	SearchWholeWord
)
