// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"reflect"
	"testing"

	"github.com/gdamore/goless/internal/ansi"
)

func FuzzDocumentChunkedMatchesSingleAppend(f *testing.F) {
	seeds := [][]byte{
		[]byte("plain text"),
		[]byte("hello\nworld\n"),
		[]byte("a\x1b[31mB\x1b[0mC"),
		[]byte("e\u0301x"),
		[]byte("👨‍👩‍👧‍👦!\n"),
		[]byte("x\x1b]8;;https://example.com\x1b\\y"),
		[]byte{0xf0, 0x9f, 0x98, 0x80},
		[]byte{0xf0, 0x9f},
		[]byte("x\r\ny"),
	}
	for _, seed := range seeds {
		f.Add(seed, uint8(1), uint8(1))
		f.Add(seed, uint8(2), uint8(4))
		f.Add(seed, uint8(7), uint8(9))
	}

	f.Fuzz(func(t *testing.T, data []byte, appendHint uint8, storeHint uint8) {
		appendChunkSize := int(appendHint%8) + 1
		storeChunkSize := int(storeHint%16) + 1
		modes := []ansi.RenderMode{ansi.RenderHybrid, ansi.RenderLiteral, ansi.RenderPresentation}

		for _, mode := range modes {
			wantLen, wantLines := buildDocumentAllAtOnce(data, mode, storeChunkSize)
			gotLen, gotLines := buildDocumentInChunks(data, mode, storeChunkSize, appendChunkSize)

			if gotLen != wantLen {
				t.Fatalf("mode %v storeChunkSize %d appendChunkSize %d: len = %d, want %d", mode, storeChunkSize, appendChunkSize, gotLen, wantLen)
			}
			if !reflect.DeepEqual(gotLines, wantLines) {
				t.Fatalf("mode %v storeChunkSize %d appendChunkSize %d: lines = %#v, want %#v", mode, storeChunkSize, appendChunkSize, gotLines, wantLines)
			}
		}
	})
}

func buildDocumentAllAtOnce(data []byte, mode ansi.RenderMode, storeChunkSize int) (int64, []Line) {
	doc := NewDocumentWithMode(storeChunkSize, mode)
	if err := doc.Append(data); err != nil {
		panic(err)
	}
	doc.Flush()
	return doc.Len(), doc.Lines()
}

func buildDocumentInChunks(data []byte, mode ansi.RenderMode, storeChunkSize int, appendChunkSize int) (int64, []Line) {
	doc := NewDocumentWithMode(storeChunkSize, mode)
	for start := 0; start < len(data); start += appendChunkSize {
		end := start + appendChunkSize
		if end > len(data) {
			end = len(data)
		}
		if err := doc.Append(data[start:end]); err != nil {
			panic(err)
		}
	}
	doc.Flush()
	return doc.Len(), doc.Lines()
}
