// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import "github.com/gdamore/tcell/v3"

func (v *Viewer) drawHelp(screen tcell.Screen) {
	bodyStyle := tcell.StyleDefault
	if !v.cfg.Chrome.Frame.enabled() && v.width > 0 {
		titleStyle := tcell.StyleDefault.Reverse(true)
		title := " " + v.text.HelpTitle + "  " + v.text.HelpClose
		screen.PutStrStyled(0, 0, padRightToWidth(title, v.width), titleStyle)
	}

	lines := v.text.helpLines()
	bodyX, bodyY, bodyWidth, bodyHeight := v.contentRect()
	for y := 0; y < bodyHeight; y++ {
		lineIndex := v.helpOffset + y
		if lineIndex >= len(lines) {
			break
		}
		screen.PutStrStyled(bodyX, bodyY+y, truncateToWidth(lines[lineIndex], bodyWidth), bodyStyle)
	}
}

func (v *Viewer) maxHelpOffset() int {
	_, _, _, bodyHeight := v.contentRect()
	return max(len(v.text.helpLines())-bodyHeight, 0)
}

func (v *Viewer) clampHelpOffset() {
	if v.helpOffset < 0 {
		v.helpOffset = 0
	}
	if maxOffset := v.maxHelpOffset(); v.helpOffset > maxOffset {
		v.helpOffset = maxOffset
	}
}
