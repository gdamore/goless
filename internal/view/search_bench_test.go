// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"strconv"
	"strings"
	"testing"

	"github.com/gdamore/goless/internal/layout"
	"github.com/gdamore/goless/internal/model"
)

const benchmarkChunkSize = 32 * 1024

var benchmarkSearchFound bool

func BenchmarkViewerSearchForward(b *testing.B) {
	v := benchmarkViewerForSearch(b)
	b.ReportAllocs()

	for b.Loop() {
		benchmarkSearchFound = v.SearchForward("needle")
	}
}

func benchmarkViewerForSearch(b *testing.B) *Viewer {
	b.Helper()

	doc := model.NewDocument(benchmarkChunkSize)
	if err := doc.Append([]byte(benchmarkSearchInput(6000))); err != nil {
		b.Fatalf("Append failed: %v", err)
	}
	doc.Flush()

	v := New(doc, Config{
		TabWidth:   8,
		WrapMode:   layout.NoWrap,
		ShowStatus: true,
	})
	v.SetSize(100, 25)
	v.Refresh()
	return v
}

func benchmarkSearchInput(lines int) string {
	var sb strings.Builder
	for i := range lines {
		sb.WriteString("search line ")
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(" contains haystack text ")
		if i%40 == 0 {
			sb.WriteString("needle ")
		}
		sb.WriteString("and more trailing content for scanning\n")
	}
	return sb.String()
}
