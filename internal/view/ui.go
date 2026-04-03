// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

// StatusInfo summarizes the built-in status bar state passed to Text.StatusLine.
type StatusInfo struct {
	Search       SearchSnapshot
	Following    bool
	EOFVisible   bool
	Message      string
	Position     Position
	DefaultLeft  string
	DefaultRight string
}

// PromptKind identifies the active built-in prompt type.
type PromptKind int

const (
	PromptKindSearchForward PromptKind = iota
	PromptKindSearchBackward
	PromptKindCommand
)

// PromptInfo summarizes the built-in prompt state passed to Text.PromptLine.
type PromptInfo struct {
	Kind        PromptKind
	Prefix      string
	Input       string
	Cursor      int
	Overwrite   bool
	Seeded      bool
	Error       string
	Search      SearchSnapshot
	DefaultText string
}
