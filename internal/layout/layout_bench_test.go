// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strconv"
	"strings"
	"testing"

	"github.com/gdamore/goless/internal/model"
)

const benchmarkChunkSize = 32 * 1024

var benchmarkLayoutResult Result

func BenchmarkBuild(b *testing.B) {
	lines := benchmarkLayoutLines(b)

	b.Run("NoWrap", func(b *testing.B) {
		cfg := Config{
			Width:            100,
			TabWidth:         8,
			WrapMode:         NoWrap,
			HorizontalOffset: 12,
		}
		b.ReportAllocs()
		for b.Loop() {
			benchmarkLayoutResult = Build(lines, cfg)
		}
	})

	b.Run("SoftWrap", func(b *testing.B) {
		cfg := Config{
			Width:    100,
			TabWidth: 8,
			WrapMode: SoftWrap,
		}
		b.ReportAllocs()
		for b.Loop() {
			benchmarkLayoutResult = Build(lines, cfg)
		}
	})
}

func benchmarkLayoutLines(b *testing.B) []model.Line {
	b.Helper()

	doc := model.NewDocument(benchmarkChunkSize)
	if err := doc.Append([]byte(benchmarkLayoutInput(3000))); err != nil {
		b.Fatalf("Append failed: %v", err)
	}
	doc.Flush()
	return doc.Lines()
}

func benchmarkLayoutInput(lines int) string {
	var sb strings.Builder
	for i := range lines {
		sb.WriteString("layout line ")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(" with segments, tabs\tand enough trailing text to force wrapping across multiple rows when width is constrained\n")
	}
	return sb.String()
}
