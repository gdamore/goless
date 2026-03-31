// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package ansi

import (
	"strconv"
	"strings"
	"testing"
)

type benchmarkReceiver struct{}

func (benchmarkReceiver) Print(rune, Style, int64) {}

func (benchmarkReceiver) Newline(Style, int64) {}

func BenchmarkParserWrite(b *testing.B) {
	data := benchmarkANSIInput(4000)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		p := NewParserWithMode(benchmarkReceiver{}, RenderHybrid)
		if _, err := p.Write(data); err != nil {
			b.Fatalf("Write failed: %v", err)
		}
		p.Flush()
	}
}

func benchmarkANSIInput(lines int) []byte {
	var sb strings.Builder
	for i := 0; i < lines; i++ {
		sb.WriteString("line ")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(" \x1b[31merror\x1b[0m \x1b[38;2;1;2;3mcolor\x1b[0m ")
		sb.WriteString("\x1b]8;;https://example.com/item/")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString("\x1b\\link\x1b]8;;\x1b\\ ")
		sb.WriteString("payload with tabs\tand trailing text\n")
	}
	return []byte(sb.String())
}
