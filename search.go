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
