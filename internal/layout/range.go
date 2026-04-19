// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package layout

import "sort"

// Range identifies a half-open interval [Start, End).
//
// It is used for pinned row and column spans, where Start and End are
// zero-based coordinates in the relevant axis.
type Range struct {
	Start int
	End   int
}

// Empty reports whether the range covers no cells.
func (r Range) Empty() bool {
	return r.End <= r.Start
}

// Length reports the number of cells covered by the range.
func (r Range) Length() int {
	if r.End <= r.Start {
		return 0
	}
	return r.End - r.Start
}

// Normalize returns a canonical non-negative range with Start <= End.
func (r Range) Normalize() Range {
	if r.End < r.Start {
		r.Start, r.End = r.End, r.Start
	}
	if r.Start < 0 {
		r.Start = 0
	}
	if r.End < 0 {
		r.End = 0
	}
	return r
}

// NormalizeRanges sorts and merges overlapping or touching ranges.
func NormalizeRanges(ranges []Range) []Range {
	if len(ranges) == 0 {
		return nil
	}

	dup := make([]Range, 0, len(ranges))
	for _, r := range ranges {
		r = r.Normalize()
		if r.Empty() {
			continue
		}
		dup = append(dup, r)
	}
	if len(dup) == 0 {
		return nil
	}

	sort.Slice(dup, func(i, j int) bool {
		if dup[i].Start == dup[j].Start {
			return dup[i].End < dup[j].End
		}
		return dup[i].Start < dup[j].Start
	})

	merged := dup[:1]
	for _, r := range dup[1:] {
		last := &merged[len(merged)-1]
		if r.Start <= last.End {
			if r.End > last.End {
				last.End = r.End
			}
			continue
		}
		merged = append(merged, r)
	}
	return merged
}

// SubtractRanges removes the given ranges from the base set and returns the
// remaining non-overlapping spans.
func SubtractRanges(base []Range, remove []Range) []Range {
	base = NormalizeRanges(base)
	remove = NormalizeRanges(remove)
	if len(base) == 0 {
		return nil
	}
	if len(remove) == 0 {
		return base
	}

	result := make([]Range, 0, len(base))
	for _, b := range base {
		curStart := b.Start
		curEnd := b.End
		for _, r := range remove {
			if r.End <= curStart {
				continue
			}
			if r.Start >= curEnd {
				break
			}
			if r.Start > curStart {
				result = append(result, Range{Start: curStart, End: r.Start})
			}
			if r.End >= curEnd {
				curStart = curEnd
				break
			}
			curStart = r.End
		}
		if curStart < curEnd {
			result = append(result, Range{Start: curStart, End: curEnd})
		}
	}
	return result
}
