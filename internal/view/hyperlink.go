// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import "github.com/gdamore/tcell/v3"

type HyperlinkInfo struct {
	Target string
	ID     string
	Text   string
	Style  tcell.Style
}

type HyperlinkDecision struct {
	Live     bool
	Target   string
	Style    tcell.Style
	StyleSet bool
}

type HyperlinkHandler = func(info HyperlinkInfo) HyperlinkDecision
