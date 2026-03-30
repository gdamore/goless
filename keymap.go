// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

// KeyGroup selects a bundled set of pager key bindings.
type KeyGroup int

const (
	// DefaultKeyGroup selects the pager's default bundled key bindings.
	DefaultKeyGroup KeyGroup = iota
	// LessKeyGroup selects the pager's less-like bundled key bindings.
	LessKeyGroup
)
