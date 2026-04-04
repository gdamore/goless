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
	for y := range bodyHeight {
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

func (v *Viewer) maxHelpColOffset() int {
	_, _, bodyWidth, _ := v.contentRect()
	if bodyWidth <= 0 {
		return 0
	}
	maxWidth := 0
	for _, line := range v.helpLines() {
		width := helpLineCellWidth(line)
		if width > maxWidth {
			maxWidth = width
		}
	}
	return max(maxWidth-bodyWidth, 0)
}

func (v *Viewer) helpLineMaxColOffset(lineIndex int) int {
	lines := v.helpLines()
	if lineIndex < 0 || lineIndex >= len(lines) {
		return 0
	}
	_, _, bodyWidth, _ := v.contentRect()
	if bodyWidth <= 0 {
		return 0
	}
	return max(helpLineCellWidth(lines[lineIndex])-bodyWidth, 0)
}

func (v *Viewer) clampHelpOffset() {
	if v.helpOffset < 0 {
		v.helpOffset = 0
	}
	if maxOffset := v.maxHelpOffset(); v.helpOffset > maxOffset {
		v.helpOffset = maxOffset
	}
	if v.helpColOffset < 0 {
		v.helpColOffset = 0
	}
	if maxOffset := v.maxHelpColOffset(); v.helpColOffset > maxOffset {
		v.helpColOffset = maxOffset
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
	skip := max(v.helpColOffset, 0)
	for _, grapheme := range line.Graphemes {
		if grapheme.CellWidth <= 0 {
			continue
		}
		if skip >= grapheme.CellWidth {
			skip -= grapheme.CellWidth
			continue
		}
		text := grapheme.Text
		cellWidth := grapheme.CellWidth
		if skip > 0 {
			text = trimLeftToWidth(text, skip)
			cellWidth -= skip
			skip = 0
		}
		if drawnWidth+cellWidth > width {
			remaining := width - drawnWidth
			if remaining <= 0 {
				break
			}
			text = truncateToWidth(text, remaining)
			cellWidth = min(cellWidth, remaining)
		}
		style := v.toTCellStyle(styleForGrapheme(line, grapheme.RuneStart))
		screen.PutStrStyled(x+drawnWidth, y, text, style)
		drawnWidth += cellWidth
		if drawnWidth >= width {
			break
		}
	}
}

func helpLineCellWidth(line model.Line) int {
	width := 0
	for _, grapheme := range line.Graphemes {
		if grapheme.CellWidth > 0 {
			width += grapheme.CellWidth
		}
	}
	return width
}
