package layout

import "github.com/gdamore/goless/internal/model"

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

// GraphemeCell maps one grapheme cluster into source and row cell coordinates.
type GraphemeCell struct {
	GraphemeIndex    int
	SourceCellStart  int
	SourceCellEnd    int
	RenderedCellFrom int
	RenderedCellTo   int
}

// VisualRow is a derived row suitable for rendering.
type VisualRow struct {
	LineIndex         int
	GraphemeStart     int
	GraphemeEnd       int
	SourceCellStart   int
	SourceCellEnd     int
	RenderedCellWidth int
	Cells             []GraphemeCell
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
	row := r.Rows[rowIndex]
	return Anchor{
		LineIndex:     row.LineIndex,
		GraphemeIndex: row.GraphemeStart,
	}
}

// RowIndexForAnchor returns the first row that contains the given anchor.
func (r Result) RowIndexForAnchor(anchor Anchor) int {
	for i, row := range r.Rows {
		if row.LineIndex != anchor.LineIndex {
			continue
		}
		if row.GraphemeStart == row.GraphemeEnd && anchor.GraphemeIndex == 0 {
			return i
		}
		if anchor.GraphemeIndex >= row.GraphemeStart && anchor.GraphemeIndex < row.GraphemeEnd {
			return i
		}
	}
	return -1
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

	startCell, startIndex := snappedStart(info, horizontalOffset)
	endLimit := startCell + width
	endIndex := startIndex
	for endIndex < len(line.Graphemes) && info.GraphemeCellStarts[endIndex] < endLimit {
		endIndex++
	}
	if endIndex == startIndex {
		endIndex = startIndex + 1
	}

	return makeRow(lineIndex, info, startIndex, endIndex, startCell)
}

func snappedStart(info LineLayout, horizontalOffset int) (int, int) {
	if horizontalOffset <= 0 || len(info.GraphemeCellStarts) == 0 {
		return 0, 0
	}

	for i := len(info.GraphemeCellStarts) - 1; i >= 0; i-- {
		if info.GraphemeCellStarts[i] <= horizontalOffset {
			return info.GraphemeCellStarts[i], i
		}
	}
	return 0, 0
}

func makeRow(lineIndex int, info LineLayout, start, end, sourceCellStart int) VisualRow {
	row := VisualRow{
		LineIndex:       lineIndex,
		GraphemeStart:   start,
		GraphemeEnd:     end,
		SourceCellStart: sourceCellStart,
		SourceCellEnd:   sourceCellStart,
		Cells:           make([]GraphemeCell, 0, max(end-start, 0)),
	}

	rowCell := 0
	for i := start; i < end; i++ {
		sourceStart := info.GraphemeCellStarts[i]
		sourceEnd := info.GraphemeCellEnds[i]
		row.Cells = append(row.Cells, GraphemeCell{
			GraphemeIndex:    i,
			SourceCellStart:  sourceStart,
			SourceCellEnd:    sourceEnd,
			RenderedCellFrom: rowCell,
			RenderedCellTo:   rowCell + (sourceEnd - sourceStart),
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
