// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"strings"

	"github.com/gdamore/tcell/v3"
)

func (v *Viewer) drawHelp(screen tcell.Screen) {
	titleStyle := tcell.StyleDefault.Reverse(true)
	bodyStyle := tcell.StyleDefault
	if v.width > 0 {
		title := " " + v.text.HelpTitle + "  " + v.text.HelpClose
		if len(title) < v.width {
			title += strings.Repeat(" ", v.width-len(title))
		}
		screen.PutStrStyled(0, 0, truncateToWidth(title, v.width), titleStyle)
	}

	lines := v.text.helpLines()
	bodyTop := 1
	bodyHeight := max(v.height-bodyTop, 0)
	for y := 0; y < bodyHeight; y++ {
		lineIndex := v.helpOffset + y
		if lineIndex >= len(lines) {
			break
		}
		screen.PutStrStyled(0, bodyTop+y, truncateToWidth(lines[lineIndex], v.width), bodyStyle)
	}
}

func (v *Viewer) maxHelpOffset() int {
	return max(len(v.text.helpLines())-max(v.height-1, 0), 0)
}

func (v *Viewer) clampHelpOffset() {
	if v.helpOffset < 0 {
		v.helpOffset = 0
	}
	if maxOffset := v.maxHelpOffset(); v.helpOffset > maxOffset {
		v.helpOffset = maxOffset
	}
}
