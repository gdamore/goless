// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"strings"
	"unicode"

	"github.com/clipperhouse/displaywidth"
)

type lineEditor struct {
	buffer    []string
	cursor    int
	overwrite bool
}

func (e *lineEditor) String() string {
	if e == nil {
		return ""
	}
	return strings.Join(e.buffer, "")
}

func (e *lineEditor) Cursor() int {
	if e == nil {
		return 0
	}
	e.clampCursor()
	return e.cursor
}

func (e *lineEditor) Overwrite() bool {
	if e == nil {
		return false
	}
	return e.overwrite
}

func (e *lineEditor) SetText(text string) {
	if e == nil {
		return
	}
	e.buffer = splitGraphemes(text)
	e.cursor = len(e.buffer)
}

func (e *lineEditor) ToggleOverwrite() {
	if e == nil {
		return
	}
	e.overwrite = !e.overwrite
}

func (e *lineEditor) MoveLeft() bool {
	if e == nil {
		return false
	}
	if e.cursor <= 0 {
		return false
	}
	e.cursor--
	return true
}

func (e *lineEditor) MoveRight() bool {
	if e == nil {
		return false
	}
	if e.cursor >= len(e.buffer) {
		return false
	}
	e.cursor++
	return true
}

func (e *lineEditor) MoveHome() bool {
	if e == nil {
		return false
	}
	if e.cursor == 0 {
		return false
	}
	e.cursor = 0
	return true
}

func (e *lineEditor) MoveEnd() bool {
	if e == nil {
		return false
	}
	if e.cursor == len(e.buffer) {
		return false
	}
	e.cursor = len(e.buffer)
	return true
}

func (e *lineEditor) Clear() bool {
	if e == nil {
		return false
	}
	changed := len(e.buffer) > 0 || e.cursor > 0
	e.buffer = e.buffer[:0]
	e.cursor = 0
	return changed
}

func (e *lineEditor) Backspace() bool {
	if e == nil {
		return false
	}
	if e.cursor <= 0 || len(e.buffer) == 0 {
		return false
	}
	copy(e.buffer[e.cursor-1:], e.buffer[e.cursor:])
	e.buffer = e.buffer[:len(e.buffer)-1]
	e.cursor--
	return true
}

func (e *lineEditor) Delete() bool {
	if e == nil {
		return false
	}
	if e.cursor >= len(e.buffer) || len(e.buffer) == 0 {
		return false
	}
	copy(e.buffer[e.cursor:], e.buffer[e.cursor+1:])
	e.buffer = e.buffer[:len(e.buffer)-1]
	return true
}

func (e *lineEditor) DeleteToStart() bool {
	if e == nil {
		return false
	}
	if e.cursor <= 0 {
		return false
	}
	copy(e.buffer, e.buffer[e.cursor:])
	e.buffer = e.buffer[:len(e.buffer)-e.cursor]
	e.cursor = 0
	return true
}

func (e *lineEditor) DeleteToEnd() bool {
	if e == nil {
		return false
	}
	if e.cursor >= len(e.buffer) {
		return false
	}
	e.buffer = e.buffer[:e.cursor]
	return true
}

func (e *lineEditor) DeleteWordBackward() bool {
	if e == nil {
		return false
	}
	if e.cursor <= 0 {
		return false
	}

	start := e.cursor
	for start > 0 && clusterIsSpace(e.buffer[start-1]) {
		start--
	}
	for start > 0 && !clusterIsSpace(e.buffer[start-1]) {
		start--
	}
	if start == e.cursor {
		return false
	}

	copy(e.buffer[start:], e.buffer[e.cursor:])
	e.buffer = e.buffer[:len(e.buffer)-(e.cursor-start)]
	e.cursor = start
	return true
}

func (e *lineEditor) Insert(text string) bool {
	if e == nil {
		return false
	}
	insert := splitGraphemes(text)
	if len(insert) == 0 {
		return false
	}
	e.clampCursor()
	if e.overwrite {
		before := append([]string(nil), e.buffer[:e.cursor]...)
		afterStart := min(e.cursor+len(insert), len(e.buffer))
		after := append([]string(nil), e.buffer[afterStart:]...)
		e.buffer = append(before, insert...)
		e.buffer = append(e.buffer, after...)
		e.cursor += len(insert)
		return true
	}

	e.buffer = append(e.buffer[:e.cursor], append(insert, e.buffer[e.cursor:]...)...)
	e.cursor += len(insert)
	return true
}

func (e *lineEditor) clampCursor() {
	if e.cursor < 0 {
		e.cursor = 0
	}
	if e.cursor > len(e.buffer) {
		e.cursor = len(e.buffer)
	}
}

func splitGraphemes(text string) []string {
	if text == "" {
		return nil
	}

	gr := displaywidth.StringGraphemes(text)
	clusters := make([]string, 0, len(text))
	for gr.Next() {
		clusters = append(clusters, gr.Value())
	}
	return clusters
}

func clusterIsSpace(cluster string) bool {
	if cluster == "" {
		return false
	}
	for _, r := range cluster {
		if unicode.IsMark(r) {
			continue
		}
		return unicode.IsSpace(r)
	}
	return false
}
