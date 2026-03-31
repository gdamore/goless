// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package ansi

import "testing"

func FuzzParserChunkedMatchesSingleWrite(f *testing.F) {
	seeds := [][]byte{
		[]byte("plain text"),
		[]byte("a\r\nb"),
		[]byte("a\x1b[31mB\x1b[0mC"),
		[]byte("x\x1b]8;;https://example.com\x1b\\y"),
		[]byte{0xf0, 0x9f, 0x98, 0x80},
		[]byte{0xf0, 0x9f},
		[]byte{0x1b, '[', '3', '8', ';', '2', ';', '2', '5', '5', ';', '0', ';', '7', 'm', 'x'},
		[]byte{0x1b, ']', '0', ';', 't', 'i', 't', 'l', 'e', 0x9c},
	}
	for _, seed := range seeds {
		f.Add(seed, uint8(1))
		f.Add(seed, uint8(2))
		f.Add(seed, uint8(7))
	}

	f.Fuzz(func(t *testing.T, data []byte, chunkHint uint8) {
		chunkSize := int(chunkHint%8) + 1
		modes := []RenderMode{RenderHybrid, RenderLiteral, RenderPresentation}

		for _, mode := range modes {
			wantEvents, wantStyle := parseAllAtOnce(data, mode)
			gotEvents, gotStyle := parseInChunks(data, mode, chunkSize)

			if len(gotEvents) != len(wantEvents) {
				t.Fatalf("mode %v chunkSize %d: event count = %d, want %d", mode, chunkSize, len(gotEvents), len(wantEvents))
			}
			for i := range wantEvents {
				if gotEvents[i] != wantEvents[i] {
					t.Fatalf("mode %v chunkSize %d: event %d = %+v, want %+v", mode, chunkSize, i, gotEvents[i], wantEvents[i])
				}
			}
			if gotStyle != wantStyle {
				t.Fatalf("mode %v chunkSize %d: style = %+v, want %+v", mode, chunkSize, gotStyle, wantStyle)
			}
		}
	})
}

func parseAllAtOnce(data []byte, mode RenderMode) ([]recordEvent, Style) {
	recv := &recordReceiver{}
	p := NewParserWithMode(recv, mode)
	if _, err := p.Write(data); err != nil {
		panic(err)
	}
	p.Flush()
	return recv.events, p.Style()
}

func parseInChunks(data []byte, mode RenderMode, chunkSize int) ([]recordEvent, Style) {
	recv := &recordReceiver{}
	p := NewParserWithMode(recv, mode)
	for start := 0; start < len(data); start += chunkSize {
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}
		if _, err := p.Write(data[start:end]); err != nil {
			panic(err)
		}
	}
	p.Flush()
	return recv.events, p.Style()
}
