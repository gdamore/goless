// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/goless/internal/ansi"
	"github.com/gdamore/goless/internal/layout"
	"github.com/gdamore/goless/internal/model"
)

// ExportScope selects which portion of the current viewer content is exported.
type ExportScope int

const (
	ExportScopeContent ExportScope = iota
	ExportScopeViewport
)

// ExportFormat selects whether export retains ANSI styling.
type ExportFormat int

const (
	ExportFormatPlain ExportFormat = iota
	ExportFormatANSI
)

// ExportOptions control viewer export behavior.
type ExportOptions struct {
	Scope  ExportScope
	Format ExportFormat
}

func normalizeExportOptions(opts ExportOptions) ExportOptions {
	switch opts.Scope {
	case ExportScopeViewport:
	default:
		opts.Scope = ExportScopeContent
	}
	switch opts.Format {
	case ExportFormatANSI:
	default:
		opts.Format = ExportFormatPlain
	}
	return opts
}

// Export returns the current content or viewport in plain or ANSI-styled form.
func (v *Viewer) Export(opts ExportOptions) ([]byte, error) {
	v.ensureLayout()
	opts = normalizeExportOptions(opts)
	switch opts.Scope {
	case ExportScopeViewport:
		return []byte(v.exportViewport(opts.Format)), nil
	default:
		return []byte(v.exportContent(opts.Format)), nil
	}
}

func (v *Viewer) exportContent(format ExportFormat) string {
	var out strings.Builder
	for i, line := range v.lines {
		if format == ExportFormatANSI {
			writeANSIStyledText(&out, line.Text, line.Styles)
		} else {
			out.WriteString(line.Text)
		}
		out.WriteString(lineEndingString(line.Ending))
		if format == ExportFormatANSI && i == len(v.lines)-1 && line.Ending == model.LineEndingNone {
			resetANSIStyle(&out)
		}
	}
	return out.String()
}

func (v *Viewer) exportViewport(format ExportFormat) string {
	_, _, contentWidth, _ := v.contentRect()
	_, _, headerWidth, _ := v.headerColumnRect()
	totalWidth := headerWidth + contentWidth
	if totalWidth < 0 {
		totalWidth = 0
	}

	var rows []string
	bodyHeight := max(v.bodyHeight(), 0)
	for y := range bodyHeight {
		rowIndex, _, ok := v.visibleLayoutRowAt(y)
		if !ok {
			break
		}
		row := v.layout.Rows[rowIndex]
		rows = append(rows, v.exportViewportRow(row, headerWidth, contentWidth, totalWidth, format))
	}
	return strings.Join(rows, "\n")
}

func (v *Viewer) exportViewportRow(row layout.VisualRow, headerWidth, contentWidth, totalWidth int, format ExportFormat) string {
	var out strings.Builder
	currentWidth := 0
	writeGap := func(target int) {
		for currentWidth < target {
			out.WriteByte(' ')
			currentWidth++
		}
	}

	line := model.Line{}
	if row.LineIndex >= 0 && row.LineIndex < len(v.lines) {
		line = v.lines[row.LineIndex]
	}

	appendSegment := func(target int, text string, style ansi.Style) {
		if target < currentWidth {
			target = currentWidth
		}
		writeGap(target)
		if format == ExportFormatANSI {
			state := styleState{current: ansi.DefaultStyle()}
			writeANSIStyledChunk(&out, &state, style, text)
			if !exportStyleIsDefault(state.current) {
				resetANSIStyle(&out)
			}
		} else {
			out.WriteString(text)
		}
		currentWidth = target + stringWidth(text)
	}

	if headerWidth > 0 {
		headerSegments := v.headerColumnSegments(row, headerWidth)
		for _, segment := range headerSegments {
			text, style, ok := v.exportSegmentText(row, line, segment)
			if !ok {
				continue
			}
			appendSegment(segment.RenderedCellFrom, text, style)
		}
		if len(headerSegments) == 0 {
			writeGap(headerWidth)
		}
	}

	base := headerWidth
	for _, segment := range row.Segments {
		text, style, ok := v.exportSegmentText(row, line, segment)
		if !ok {
			continue
		}
		appendSegment(base+segment.RenderedCellFrom, text, style)
	}

	if markers := v.trailingMarkers(row, line); markers != "" && row.LineIndex >= 0 && row.LineIndex < len(v.layout.Lines) {
		start := headerWidth + (v.layout.Lines[row.LineIndex].TotalCells - row.SourceCellStart)
		if start >= 0 && start < totalWidth {
			appendSegment(start, truncateToWidth(markers, totalWidth-start), ansi.DefaultStyle())
		}
	}

	return strings.TrimRight(out.String(), " ")
}

type styleState struct {
	current ansi.Style
}

func writeANSIStyledText(out *strings.Builder, text string, runs []model.StyleRun) {
	state := styleState{current: ansi.DefaultStyle()}
	if text == "" {
		return
	}
	offsets := runeByteOffsets(text)
	runes := len(offsets) - 1
	pos := 0
	for _, run := range runs {
		start := clamp(run.Start, 0, runes)
		end := clamp(run.End, start, runes)
		if start > pos {
			writeANSIStyledChunk(out, &state, ansi.DefaultStyle(), text[offsets[pos]:offsets[start]])
		}
		if end > start {
			writeANSIStyledChunk(out, &state, sanitizeExportStyle(run.Style), text[offsets[start]:offsets[end]])
		}
		pos = end
	}
	if pos < runes {
		writeANSIStyledChunk(out, &state, ansi.DefaultStyle(), text[offsets[pos]:offsets[runes]])
	}
	if !exportStyleIsDefault(state.current) {
		resetANSIStyle(out)
	}
}

func writeANSIStyledChunk(out *strings.Builder, state *styleState, style ansi.Style, text string) {
	if text == "" {
		return
	}
	style = sanitizeExportStyle(style)
	if state == nil {
		local := styleState{current: ansi.DefaultStyle()}
		state = &local
	}
	if !exportStylesEqual(state.current, style) {
		out.WriteString(ansiStyleSequence(style))
		state.current = style
	}
	out.WriteString(text)
}

func resetANSIStyle(out *strings.Builder) {
	out.WriteString("\x1b[0m")
}

func ansiStyleSequence(style ansi.Style) string {
	style = sanitizeExportStyle(style)
	if exportStyleIsDefault(style) {
		return "\x1b[0m"
	}

	codes := []string{"0"}
	if style.Bold {
		codes = append(codes, "1")
	}
	if style.Dim {
		codes = append(codes, "2")
	}
	if style.Italic {
		codes = append(codes, "3")
	}
	switch style.Underline {
	case ansi.UnderlineStyleSolid:
		codes = append(codes, "4")
	case ansi.UnderlineStyleDouble:
		codes = append(codes, "4:2")
	case ansi.UnderlineStyleCurly:
		codes = append(codes, "4:3")
	case ansi.UnderlineStyleDotted:
		codes = append(codes, "4:4")
	case ansi.UnderlineStyleDashed:
		codes = append(codes, "4:5")
	}
	if style.Blink {
		codes = append(codes, "5")
	}
	if style.Reverse {
		codes = append(codes, "7")
	}
	if style.Strike {
		codes = append(codes, "9")
	}
	codes = append(codes, ansiColorCodes(style.Fg, false)...)
	codes = append(codes, ansiColorCodes(style.Bg, true)...)
	codes = append(codes, ansiUnderlineColorCodes(style.UnderlineColor)...)
	return "\x1b[" + strings.Join(codes, ";") + "m"
}

func ansiColorCodes(c ansi.Color, background bool) []string {
	switch c.Kind {
	case ansi.ColorIndex:
		if c.Index < 8 {
			if background {
				return []string{itoa(int(40 + c.Index))}
			}
			return []string{itoa(int(30 + c.Index))}
		}
		if c.Index < 16 {
			if background {
				return []string{itoa(int(100 + c.Index - 8))}
			}
			return []string{itoa(int(90 + c.Index - 8))}
		}
		if background {
			return []string{"48", "5", itoa(int(c.Index))}
		}
		return []string{"38", "5", itoa(int(c.Index))}
	case ansi.ColorRGB:
		if background {
			return []string{"48", "2", itoa(int(c.R)), itoa(int(c.G)), itoa(int(c.B))}
		}
		return []string{"38", "2", itoa(int(c.R)), itoa(int(c.G)), itoa(int(c.B))}
	default:
		return nil
	}
}

func ansiUnderlineColorCodes(c ansi.Color) []string {
	switch c.Kind {
	case ansi.ColorIndex:
		return []string{"58", "5", itoa(int(c.Index))}
	case ansi.ColorRGB:
		return []string{"58", "2", itoa(int(c.R)), itoa(int(c.G)), itoa(int(c.B))}
	default:
		return nil
	}
}

func sanitizeExportStyle(style ansi.Style) ansi.Style {
	style.URL = ""
	style.URLID = ""
	return style
}

func exportStyleIsDefault(style ansi.Style) bool {
	return exportStylesEqual(style, ansi.DefaultStyle())
}

func exportStylesEqual(a, b ansi.Style) bool {
	return sanitizeExportStyle(a) == sanitizeExportStyle(b)
}

func runeByteOffsets(text string) []int {
	offsets := make([]int, 0, utf8.RuneCountInString(text)+1)
	for i := range text {
		offsets = append(offsets, i)
	}
	offsets = append(offsets, len(text))
	return offsets
}

func lineEndingString(ending model.LineEnding) string {
	switch ending {
	case model.LineEndingCRLF:
		return "\r\n"
	case model.LineEndingVT:
		return "\v"
	case model.LineEndingFF:
		return "\f"
	case model.LineEndingLF:
		return "\n"
	default:
		return ""
	}
}

func (v *Viewer) exportSegmentText(row layout.VisualRow, line model.Line, segment layout.VisualSegment) (string, ansi.Style, bool) {
	if segment.LogicalGraphemeIndex < 0 || segment.LogicalGraphemeIndex >= len(line.Graphemes) {
		return "", ansi.DefaultStyle(), false
	}
	grapheme := line.Graphemes[segment.LogicalGraphemeIndex]
	style := sanitizeExportStyle(styleForGrapheme(line, grapheme.RuneStart))
	width := max(segment.RenderedCellTo-segment.RenderedCellFrom, 0)

	if display, _, ok := v.visualizedSegment(row, line, segment, grapheme, v.toTCellStyle(style)); ok {
		return truncateToWidth(display, width), style, true
	}
	if segment.Display != "" {
		return truncateToWidth(segment.Display, width), style, true
	}
	if grapheme.Text == "\t" {
		return strings.Repeat(" ", width), style, true
	}
	return truncateToWidth(grapheme.Text, width), style, true
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func itoa(v int) string {
	return strconv.Itoa(v)
}
