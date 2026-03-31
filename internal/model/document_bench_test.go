// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"strconv"
	"strings"
	"testing"
)

var benchmarkDocumentLineCount int

func BenchmarkDocumentAppend(b *testing.B) {
	data := benchmarkDocumentInput(4000)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		doc := NewDocument(defaultChunkSize)
		if err := doc.Append(data); err != nil {
			b.Fatalf("Append failed: %v", err)
		}
		doc.Flush()
		benchmarkDocumentLineCount = len(doc.Lines())
	}
}

func benchmarkDocumentInput(lines int) []byte {
	var sb strings.Builder
	for i := 0; i < lines; i++ {
		sb.WriteString("row ")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(" \x1b[32mvalue\x1b[0m cafe\u0301 family ")
		sb.WriteString("\U0001F468\u200D\U0001F469\u200D\U0001F467\u200D\U0001F466")
		sb.WriteString("\n")
	}
	return []byte(sb.String())
}
