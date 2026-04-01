// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import "github.com/gdamore/tcell/v3"

// HyperlinkInfo describes a source-provided OSC 8 hyperlink span.
type HyperlinkInfo struct {
	// Target is the original hyperlink destination from the OSC 8 sequence.
	Target string
	// ID is the optional OSC 8 hyperlink id parameter.
	ID string
	// Text is the full linked display text for the logical line span.
	Text string
	// Style is the pager's base rendered style for the linked text.
	Style tcell.Style
}

// HyperlinkDecision controls how an OSC 8 hyperlink span is rendered.
type HyperlinkDecision struct {
	// Live reports whether the rendered span should carry hyperlink metadata.
	Live bool
	// Target replaces the source-provided hyperlink destination when non-empty.
	// When Live is true and Target is empty, the original HyperlinkInfo.Target is used.
	Target string
	// Style overrides the base hyperlink style when StyleSet is true.
	Style tcell.Style
	// StyleSet reports whether Style should replace the base style.
	StyleSet bool
}

// HyperlinkHandler decides how a parsed OSC 8 hyperlink span should be rendered.
//
// The zero behavior is intentionally conservative: parsed links stay inert
// unless the handler explicitly sets HyperlinkDecision.Live. This is meant to
// help embedders make a conscious trust decision for untrusted content instead
// of silently turning source-provided links into live targets.
type HyperlinkHandler func(HyperlinkInfo) HyperlinkDecision
