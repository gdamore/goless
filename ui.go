// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

// StatusInfo summarizes the built-in status bar state passed to Text.StatusLine.
type StatusInfo struct {
	// Search is the current committed or preview search state.
	Search SearchState
	// Following reports whether follow mode is active.
	Following bool
	// Message is the current status message appended by pager actions.
	Message string
	// Position is the current viewport position.
	Position Position
	// DefaultLeft is the built-in left status text.
	DefaultLeft string
	// DefaultRight is the built-in right status text.
	DefaultRight string
}

// PromptKind identifies the active built-in prompt type.
type PromptKind int

const (
	// PromptSearchForward is the "/" search prompt.
	PromptSearchForward PromptKind = iota
	// PromptSearchBackward is the "?" search prompt.
	PromptSearchBackward
	// PromptCommand is the ":" command prompt.
	PromptCommand
)

// PromptInfo summarizes the built-in prompt state passed to Text.PromptLine.
type PromptInfo struct {
	// Kind identifies which built-in prompt is active.
	Kind PromptKind
	// Prefix is the built-in prompt prefix, such as "/[smart,sub] ".
	Prefix string
	// Input is the current editable prompt buffer.
	Input string
	// Error is the current prompt-side error indicator, such as an invalid regex.
	Error string
	// Search is the current committed or preview search state.
	Search SearchState
	// DefaultText is the built-in prompt text before any custom formatting.
	DefaultText string
}
