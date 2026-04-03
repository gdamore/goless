// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"github.com/gdamore/goless/internal/ansi"
	"github.com/gdamore/goless/internal/model"
	"github.com/gdamore/tcell/v3"
)

const helpDocChunkSize = 256

func (v *Viewer) drawHelp(screen tcell.Screen) {
	bodyStyle := v.toTCellStyle(ansi.DefaultStyle())
	if !v.cfg.Chrome.Frame.enabled() && v.width > 0 {
		titleStyle := tcell.StyleDefault.Reverse(true)
		title := " " + v.text.HelpTitle + "  " + v.text.HelpClose
		screen.PutStrStyled(0, 0, padRightToWidth(title, v.width), titleStyle)
	}

	lines := v.helpLines()
	bodyX, bodyY, bodyWidth, bodyHeight := v.contentRect()
	for y := 0; y < bodyHeight; y++ {
		screen.PutStrStyled(bodyX, bodyY+y, padRightToWidth("", bodyWidth), bodyStyle)
		lineIndex := v.helpOffset + y
		if lineIndex >= len(lines) {
			continue
		}
		v.drawHelpDocumentLine(screen, bodyX, bodyY+y, bodyWidth, lines[lineIndex])
	}
}

func (v *Viewer) maxHelpOffset() int {
	_, _, _, bodyHeight := v.contentRect()
	return max(len(v.helpLines())-bodyHeight, 0)
}

func (v *Viewer) clampHelpOffset() {
	if v.helpOffset < 0 {
		v.helpOffset = 0
	}
	if maxOffset := v.maxHelpOffset(); v.helpOffset > maxOffset {
		v.helpOffset = maxOffset
	}
}

func (v *Viewer) helpLines() []model.Line {
	doc := model.NewDocumentWithMode(helpDocChunkSize, ansi.RenderPresentation)
	_ = doc.Append([]byte(v.text.HelpBody))
	doc.Flush()
	return doc.Lines()
}

func (v *Viewer) drawHelpDocumentLine(screen tcell.Screen, x, y, width int, line model.Line) {
	drawnWidth := 0
	for _, grapheme := range line.Graphemes {
		if grapheme.CellWidth <= 0 {
			continue
		}
		if drawnWidth+grapheme.CellWidth > width {
			break
		}
		style := v.toTCellStyle(styleForGrapheme(line, grapheme.RuneStart))
		screen.PutStrStyled(x+drawnWidth, y, grapheme.Text, style)
		drawnWidth += grapheme.CellWidth
	}
}
