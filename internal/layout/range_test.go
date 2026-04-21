// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package layout

import "testing"

func TestRangeMethods(t *testing.T) {
	tests := []struct {
		name       string
		r          Range
		empty      bool
		length     int
		normalized Range
	}{
		{
			name:       "empty",
			r:          Range{Start: 3, End: 3},
			empty:      true,
			length:     0,
			normalized: Range{Start: 3, End: 3},
		},
		{
			name:       "reversed and negative",
			r:          Range{Start: 5, End: -2},
			empty:      true,
			length:     0,
			normalized: Range{Start: 0, End: 5},
		},
		{
			name:       "negative start",
			r:          Range{Start: -3, End: 4},
			empty:      false,
			length:     7,
			normalized: Range{Start: 0, End: 4},
		},
		{
			name:       "all negative",
			r:          Range{Start: -5, End: -2},
			empty:      false,
			length:     3,
			normalized: Range{Start: 0, End: 0},
		},
		{
			name:       "normal",
			r:          Range{Start: 2, End: 7},
			empty:      false,
			length:     5,
			normalized: Range{Start: 2, End: 7},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.Empty(); got != tt.empty {
				t.Fatalf("Empty() = %v, want %v", got, tt.empty)
			}
			if got := tt.r.Length(); got != tt.length {
				t.Fatalf("Length() = %d, want %d", got, tt.length)
			}
			if got := tt.r.Normalize(); got != tt.normalized {
				t.Fatalf("Normalize() = %+v, want %+v", got, tt.normalized)
			}
		})
	}
}

func TestNormalizeRanges(t *testing.T) {
	if got := NormalizeRanges(nil); got != nil {
		t.Fatalf("NormalizeRanges(nil) = %+v, want nil", got)
	}

	if got := NormalizeRanges([]Range{{Start: 1, End: 1}, {Start: 2, End: 2}}); got != nil {
		t.Fatalf("NormalizeRanges(all empty) = %+v, want nil", got)
	}

	got := NormalizeRanges([]Range{
		{Start: 10, End: 12},
		{Start: 3, End: 5},
		{Start: 4, End: 7},
		{Start: 1, End: 3},
		{Start: 30, End: 40},
		{Start: 30, End: 35},
		{Start: 20, End: 22},
	})
	want := []Range{
		{Start: 1, End: 7},
		{Start: 10, End: 12},
		{Start: 20, End: 22},
		{Start: 30, End: 40},
	}
	if len(got) != len(want) {
		t.Fatalf("NormalizeRanges(...) len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeRanges(...)[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestSubtractRanges(t *testing.T) {
	if got := SubtractRanges(nil, []Range{{Start: 1, End: 2}}); got != nil {
		t.Fatalf("SubtractRanges(nil, ...) = %+v, want nil", got)
	}

	base := []Range{{Start: 1, End: 4}, {Start: 10, End: 14}}
	if got := SubtractRanges(base, nil); len(got) != len(base) || got[0] != base[0] || got[1] != base[1] {
		t.Fatalf("SubtractRanges(base, nil) = %+v, want %+v", got, base)
	}

	if got := SubtractRanges([]Range{{Start: 0, End: 5}}, []Range{{Start: 0, End: 10}}); got != nil {
		t.Fatalf("SubtractRanges(full removal) = %+v, want nil", got)
	}

	got := SubtractRanges(
		[]Range{{Start: 0, End: 10}, {Start: 20, End: 30}, {Start: 40, End: 45}},
		[]Range{
			{Start: 0, End: 1},
			{Start: 3, End: 4},
			{Start: 6, End: 7},
			{Start: 12, End: 13},
			{Start: 42, End: 50},
		},
	)
	want := []Range{
		{Start: 1, End: 3},
		{Start: 4, End: 6},
		{Start: 7, End: 10},
		{Start: 20, End: 30},
		{Start: 40, End: 42},
	}
	if len(got) != len(want) {
		t.Fatalf("SubtractRanges(...) len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SubtractRanges(...)[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
