// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package ansi

// ColorKind describes how a color value should be interpreted.
type ColorKind uint8

const (
	// ColorDefault indicates the terminal default color.
	ColorDefault ColorKind = iota
	// ColorIndex indicates an indexed terminal color.
	ColorIndex
	// ColorRGB indicates a 24-bit RGB color.
	ColorRGB
)

// Color describes a terminal color in normalized form.
type Color struct {
	Kind  ColorKind
	Index uint8
	R     uint8
	G     uint8
	B     uint8
}

// DefaultColor returns the terminal default color.
func DefaultColor() Color {
	return Color{Kind: ColorDefault}
}

// IndexedColor returns an indexed terminal color.
func IndexedColor(index uint8) Color {
	return Color{Kind: ColorIndex, Index: index}
}

// RGBColor returns a 24-bit terminal color.
func RGBColor(r, g, b uint8) Color {
	return Color{Kind: ColorRGB, R: r, G: g, B: b}
}

// Style is the normalized style state produced by the ANSI parser.
type Style struct {
	Fg        Color
	Bg        Color
	Bold      bool
	Dim       bool
	Italic    bool
	Underline bool
	Strike    bool
	Blink     bool
	Reverse   bool
	URL       string
	URLID     string
}

// DefaultStyle returns the initial style state.
func DefaultStyle() Style {
	return Style{
		Fg: DefaultColor(),
		Bg: DefaultColor(),
	}
}
