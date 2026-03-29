// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"

	"github.com/gdamore/goless/internal/model"
)

// WrapMode controls whether logical lines wrap into visual rows.
type WrapMode int

const (
	NoWrap WrapMode = iota
	SoftWrap
)

// Config controls visual row derivation.
type Config struct {
	Width            int
	TabWidth         int
	WrapMode         WrapMode
	HorizontalOffset int
}

// Anchor identifies an approximate visible position that can be preserved across relayout.
type Anchor struct {
	LineIndex     int
	GraphemeIndex int
}

// VisualSegment maps one logical grapheme cluster into row cell coordinates.
// Segments are ordered by visual presentation, not by logical grapheme index.
type VisualSegment struct {
	LogicalGraphemeIndex int
	SourceCellStart      int
	SourceCellEnd        int
	RenderedCellFrom     int
	RenderedCellTo       int
	Display              string
}

// VisualRow is a derived row suitable for rendering.
type VisualRow struct {
	LineIndex         int
	SourceCellStart   int
	SourceCellEnd     int
	RenderedCellWidth int
	Segments          []VisualSegment
}

// LineLayout stores per-line cell metrics.
type LineLayout struct {
	TotalCells         int
	GraphemeCellStarts []int
	GraphemeCellEnds   []int
}

// Result is the derived layout for a set of lines under a specific configuration.
type Result struct {
	Config Config
	Lines  []LineLayout
	Rows   []VisualRow
}

// Build derives visual rows from logical lines.
func Build(lines []model.Line, cfg Config) Result {
	cfg = normalizeConfig(cfg)
	result := Result{
		Config: cfg,
		Lines:  make([]LineLayout, len(lines)),
	}

	for i, line := range lines {
		lineLayout := measureLine(line, cfg.TabWidth)
		result.Lines[i] = lineLayout
		if cfg.WrapMode == SoftWrap {
			result.Rows = append(result.Rows, buildWrappedRows(i, line, lineLayout, cfg.Width)...)
			continue
		}
		result.Rows = append(result.Rows, buildScrolledRow(i, line, lineLayout, cfg.Width, cfg.HorizontalOffset))
	}

	return result
}

// AnchorForRow returns the grapheme anchor associated with the row.
func (r Result) AnchorForRow(rowIndex int) Anchor {
	if rowIndex < 0 || rowIndex >= len(r.Rows) {
		return Anchor{}
	}
	return r.Rows[rowIndex].Anchor()
}

// RowIndexForAnchor returns the first row that contains the given anchor.
func (r Result) RowIndexForAnchor(anchor Anchor) int {
	for i, row := range r.Rows {
		if row.LineIndex != anchor.LineIndex {
			continue
		}
		if row.ContainsLogicalGrapheme(anchor.GraphemeIndex) {
			return i
		}
	}
	return -1
}

// Anchor returns the leading logical grapheme represented by the row.
func (r VisualRow) Anchor() Anchor {
	leading := 0
	if len(r.Segments) > 0 {
		leading = r.Segments[0].LogicalGraphemeIndex
	}
	return Anchor{
		LineIndex:     r.LineIndex,
		GraphemeIndex: leading,
	}
}

// FirstLogicalGrapheme returns the logical grapheme index anchoring this row.
func (r VisualRow) FirstLogicalGrapheme() int {
	if len(r.Segments) == 0 {
		return 0
	}
	first := r.Segments[0].LogicalGraphemeIndex
	for _, segment := range r.Segments[1:] {
		if segment.LogicalGraphemeIndex < first {
			first = segment.LogicalGraphemeIndex
		}
	}
	return first
}

// LastLogicalGrapheme returns the exclusive logical grapheme end for the row.
func (r VisualRow) LastLogicalGrapheme() int {
	if len(r.Segments) == 0 {
		return 0
	}
	last := r.Segments[0].LogicalGraphemeIndex + 1
	for _, segment := range r.Segments[1:] {
		if end := segment.LogicalGraphemeIndex + 1; end > last {
			last = end
		}
	}
	return last
}

// ContainsLogicalGrapheme reports whether the row contains the given logical grapheme index.
func (r VisualRow) ContainsLogicalGrapheme(index int) bool {
	if len(r.Segments) == 0 {
		return index == 0
	}
	for _, segment := range r.Segments {
		if segment.LogicalGraphemeIndex == index {
			return true
		}
	}
	return false
}

func normalizeConfig(cfg Config) Config {
	if cfg.Width <= 0 {
		cfg.Width = 1
	}
	if cfg.TabWidth <= 0 {
		cfg.TabWidth = 8
	}
	if cfg.HorizontalOffset < 0 {
		cfg.HorizontalOffset = 0
	}
	return cfg
}

func measureLine(line model.Line, tabWidth int) LineLayout {
	info := LineLayout{
		GraphemeCellStarts: make([]int, len(line.Graphemes)),
		GraphemeCellEnds:   make([]int, len(line.Graphemes)),
	}

	cell := 0
	for i, grapheme := range line.Graphemes {
		info.GraphemeCellStarts[i] = cell
		width := graphemeWidth(grapheme, cell, tabWidth)
		cell += width
		info.GraphemeCellEnds[i] = cell
	}
	info.TotalCells = cell
	return info
}

func buildWrappedRows(lineIndex int, line model.Line, info LineLayout, width int) []VisualRow {
	if len(line.Graphemes) == 0 {
		return []VisualRow{{LineIndex: lineIndex}}
	}

	var rows []VisualRow
	rowStart := 0
	rowCellStart := 0
	rowWidth := 0

	for i := range line.Graphemes {
		gw := info.GraphemeCellEnds[i] - info.GraphemeCellStarts[i]
		if rowStart < i && rowWidth+gw > width {
			rows = append(rows, makeRow(lineIndex, info, rowStart, i, rowCellStart))
			rowStart = i
			rowCellStart = info.GraphemeCellStarts[i]
			rowWidth = 0
		}
		if rowStart == i && gw > width {
			rows = append(rows, makeRow(lineIndex, info, i, i+1, info.GraphemeCellStarts[i]))
			rowStart = i + 1
			rowCellStart = info.GraphemeCellEnds[i]
			rowWidth = 0
			continue
		}
		rowWidth += gw
	}

	if rowStart < len(line.Graphemes) {
		rows = append(rows, makeRow(lineIndex, info, rowStart, len(line.Graphemes), rowCellStart))
	}
	if len(rows) == 0 {
		rows = append(rows, VisualRow{LineIndex: lineIndex})
	}
	return rows
}

func buildScrolledRow(lineIndex int, line model.Line, info LineLayout, width int, horizontalOffset int) VisualRow {
	if len(line.Graphemes) == 0 {
		return VisualRow{LineIndex: lineIndex}
	}

	row := VisualRow{
		LineIndex:       lineIndex,
		SourceCellStart: horizontalOffset,
		SourceCellEnd:   horizontalOffset,
	}
	if width <= 0 {
		return row
	}

	windowStart := horizontalOffset
	windowEnd := horizontalOffset + width
	renderX := 0

	for i := range line.Graphemes {
		grapheme := line.Graphemes[i]
		sourceStart := info.GraphemeCellStarts[i]
		sourceEnd := info.GraphemeCellEnds[i]
		if sourceEnd <= windowStart || sourceStart >= windowEnd {
			continue
		}

		leftClipped := sourceStart < windowStart
		rightClipped := sourceEnd > windowEnd

		if grapheme.Text == "\t" && (leftClipped || rightClipped) {
			visibleStart := max(sourceStart, windowStart)
			visibleEnd := min(sourceEnd, windowEnd)
			visibleWidth := max(visibleEnd-visibleStart, 0)
			if visibleWidth > 0 {
				row.Segments = append(row.Segments, VisualSegment{
					LogicalGraphemeIndex: i,
					SourceCellStart:      sourceStart,
					SourceCellEnd:        sourceEnd,
					RenderedCellFrom:     renderX,
					RenderedCellTo:       renderX + visibleWidth,
					Display:              spaces(visibleWidth),
				})
				renderX += visibleWidth
				row.SourceCellEnd = visibleEnd
			}
			if renderX >= width {
				return finalizeScrolledRow(row, renderX)
			}
			continue
		}

		switch {
		case leftClipped && rightClipped:
			if renderX < width {
				row.Segments = append(row.Segments, markerSegment(i, sourceStart, sourceEnd, renderX, leftClipMarker))
				renderX++
			}
			if renderX < width {
				row.Segments = append(row.Segments, markerSegment(i, sourceStart, sourceEnd, renderX, rightClipMarker))
				renderX++
			}
			row.SourceCellEnd = min(sourceEnd, windowEnd)
			return finalizeScrolledRow(row, renderX)
		case leftClipped:
			row.Segments = append(row.Segments, markerSegment(i, sourceStart, sourceEnd, renderX, leftClipMarker))
			renderX++
			row.SourceCellEnd = min(sourceEnd, windowEnd)
			if renderX >= width {
				return finalizeScrolledRow(row, renderX)
			}
			continue
		case rightClipped:
			row.Segments = append(row.Segments, markerSegment(i, sourceStart, sourceEnd, renderX, rightClipMarker))
			renderX++
			row.SourceCellEnd = windowEnd
			return finalizeScrolledRow(row, renderX)
		default:
			segWidth := sourceEnd - sourceStart
			row.Segments = append(row.Segments, VisualSegment{
				LogicalGraphemeIndex: i,
				SourceCellStart:      sourceStart,
				SourceCellEnd:        sourceEnd,
				RenderedCellFrom:     renderX,
				RenderedCellTo:       renderX + segWidth,
			})
			renderX += segWidth
			row.SourceCellEnd = sourceEnd
			if renderX >= width {
				return finalizeScrolledRow(row, renderX)
			}
		}
	}

	return finalizeScrolledRow(row, renderX)
}

func makeRow(lineIndex int, info LineLayout, start, end, sourceCellStart int) VisualRow {
	row := VisualRow{
		LineIndex:       lineIndex,
		SourceCellStart: sourceCellStart,
		SourceCellEnd:   sourceCellStart,
		Segments:        make([]VisualSegment, 0, max(end-start, 0)),
	}

	rowCell := 0
	for i := start; i < end; i++ {
		sourceStart := info.GraphemeCellStarts[i]
		sourceEnd := info.GraphemeCellEnds[i]
		row.Segments = append(row.Segments, VisualSegment{
			LogicalGraphemeIndex: i,
			SourceCellStart:      sourceStart,
			SourceCellEnd:        sourceEnd,
			RenderedCellFrom:     rowCell,
			RenderedCellTo:       rowCell + (sourceEnd - sourceStart),
		})
		rowCell += sourceEnd - sourceStart
		row.SourceCellEnd = sourceEnd
	}
	row.RenderedCellWidth = rowCell
	return row
}

func graphemeWidth(grapheme model.Grapheme, currentCell, tabWidth int) int {
	if grapheme.Text == "\t" {
		next := ((currentCell / tabWidth) + 1) * tabWidth
		return next - currentCell
	}
	if grapheme.CellWidth > 0 {
		return grapheme.CellWidth
	}
	return 0
}

func markerSegment(index, sourceStart, sourceEnd, renderX int, marker string) VisualSegment {
	return VisualSegment{
		LogicalGraphemeIndex: index,
		SourceCellStart:      sourceStart,
		SourceCellEnd:        sourceEnd,
		RenderedCellFrom:     renderX,
		RenderedCellTo:       renderX + 1,
		Display:              marker,
	}
}

func finalizeScrolledRow(row VisualRow, renderWidth int) VisualRow {
	row.RenderedCellWidth = renderWidth
	if row.SourceCellEnd < row.SourceCellStart {
		row.SourceCellEnd = row.SourceCellStart
	}
	return row
}

const (
	leftClipMarker  = "<"
	rightClipMarker = ">"
)

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}
