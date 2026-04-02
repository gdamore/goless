// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import (
	"github.com/gdamore/tcell/v3"
	tcolor "github.com/gdamore/tcell/v3/color"
)

// Preset bundles a Theme and Chrome configuration that work well together.
//
// These presets are ordinary exported variables so embedders can copy and tweak
// them directly before passing the fields into Config. Copy a preset into a
// local variable before modifying it:
//
//	preset := DarkPreset
//	preset.Chrome.Title = "My App"
//	pager := New(Config{Theme: preset.Theme, Chrome: preset.Chrome})
type Preset struct {
	Theme  Theme
	Chrome Chrome
}

var (
	solarizedBase03  = rgb(0x00, 0x2b, 0x36)
	solarizedBase02  = rgb(0x07, 0x36, 0x42)
	solarizedBase01  = rgb(0x58, 0x6e, 0x75)
	solarizedBase00  = rgb(0x65, 0x7b, 0x83)
	solarizedBase0   = rgb(0x83, 0x94, 0x96)
	solarizedBase1   = rgb(0x93, 0xa1, 0xa1)
	solarizedBase2   = rgb(0xee, 0xe8, 0xd5)
	solarizedBase3   = rgb(0xfd, 0xf6, 0xe3)
	solarizedYellow  = rgb(0xb5, 0x89, 0x00)
	solarizedOrange  = rgb(0xcb, 0x4b, 0x16)
	solarizedRed     = rgb(0xdc, 0x32, 0x2f)
	solarizedMagenta = rgb(0xd3, 0x36, 0x82)
	solarizedViolet  = rgb(0x6c, 0x71, 0xc4)
	solarizedBlue    = rgb(0x26, 0x8b, 0xd2)
	solarizedCyan    = rgb(0x2a, 0xa1, 0x98)
	solarizedGreen   = rgb(0x85, 0x99, 0x00)
	prettyBlue       = rgb(0x3f, 0x63, 0x99)
)

// DarkPreset uses a solarized-dark-like content palette and matching chrome.
var DarkPreset = Preset{
	Theme: Theme{
		DefaultFG: solarizedBase0,
		DefaultBG: solarizedBase03,
		ANSI: [16]tcolor.Color{
			solarizedBase02,
			solarizedRed,
			solarizedGreen,
			solarizedYellow,
			solarizedBlue,
			solarizedMagenta,
			solarizedCyan,
			solarizedBase2,
			solarizedBase03,
			solarizedOrange,
			solarizedBase01,
			solarizedBase00,
			solarizedBase0,
			solarizedViolet,
			solarizedBase1,
			solarizedBase3,
		},
	},
	Chrome: Chrome{
		Frame:            SingleFrame(),
		TitleAlign:       TitleAlignCenter,
		BorderStyle:      tcell.StyleDefault.Foreground(solarizedBase01).Background(solarizedBase03),
		TitleStyle:       tcell.StyleDefault.Foreground(solarizedBlue).Background(solarizedBase03).Bold(true),
		StatusStyle:      tcell.StyleDefault.Foreground(solarizedBase3).Background(solarizedBase03),
		LineNumberStyle:  tcell.StyleDefault.Foreground(solarizedBase01).Dim(true),
		HeaderStyle:      tcell.StyleDefault.Background(solarizedBase02).Bold(true),
		PromptStyle:      tcell.StyleDefault.Foreground(solarizedBase3).Background(solarizedBase03),
		PromptErrorStyle: tcell.StyleDefault.Foreground(solarizedRed).Background(solarizedBase03).Bold(true),
	},
}

// LightPreset uses a solarized-light-like content palette and matching chrome.
var LightPreset = Preset{
	Theme: Theme{
		DefaultFG: solarizedBase00,
		DefaultBG: solarizedBase3,
		ANSI: [16]tcolor.Color{
			solarizedBase02,
			solarizedRed,
			solarizedGreen,
			solarizedYellow,
			solarizedBlue,
			solarizedMagenta,
			solarizedCyan,
			solarizedBase2,
			solarizedBase03,
			solarizedOrange,
			solarizedBase01,
			solarizedBase00,
			solarizedBase0,
			solarizedViolet,
			solarizedBase1,
			solarizedBase3,
		},
	},
	Chrome: Chrome{
		Frame:            SingleFrame(),
		TitleAlign:       TitleAlignCenter,
		BorderStyle:      tcell.StyleDefault.Foreground(solarizedBase1).Background(solarizedBase3),
		TitleStyle:       tcell.StyleDefault.Foreground(solarizedBlue).Background(solarizedBase3).Bold(true),
		StatusStyle:      tcell.StyleDefault.Foreground(solarizedBase02).Background(solarizedBase2),
		LineNumberStyle:  tcell.StyleDefault.Foreground(solarizedBase1).Dim(true),
		HeaderStyle:      tcell.StyleDefault.Background(solarizedBase2).Bold(true),
		PromptStyle:      tcell.StyleDefault.Foreground(solarizedBase02).Background(solarizedBase2),
		PromptErrorStyle: tcell.StyleDefault.Foreground(solarizedRed).Background(solarizedBase2).Bold(true),
	},
}

// PlainPreset keeps the pager monochrome and understated.
var PlainPreset = Preset{
	Theme: Theme{
		ANSI: [16]tcolor.Color{
			tcolor.Black,
			tcolor.Black,
			tcolor.Black,
			tcolor.Black,
			tcolor.Black,
			tcolor.Black,
			tcolor.Black,
			tcolor.Silver,
			tcolor.Gray,
			tcolor.Gray,
			tcolor.Gray,
			tcolor.Gray,
			tcolor.Gray,
			tcolor.Gray,
			tcolor.Gray,
			tcolor.White,
		},
	},
	Chrome: Chrome{
		TitleAlign:       TitleAlignLeft,
		StatusStyle:      tcell.StyleDefault.Reverse(true),
		LineNumberStyle:  tcell.StyleDefault.Dim(true),
		HeaderStyle:      tcell.StyleDefault.Reverse(true).Bold(true),
		PromptStyle:      tcell.StyleDefault.Reverse(true),
		PromptErrorStyle: tcell.StyleDefault.Reverse(true).Bold(true),
	},
}

// PrettyPreset leaves content colors alone but adds decorative rounded chrome.
var PrettyPreset = Preset{
	Theme: Theme{},
	Chrome: Chrome{
		Frame:            RoundedFrame(),
		TitleAlign:       TitleAlignCenter,
		BorderStyle:      tcell.StyleDefault.Foreground(prettyBlue),
		TitleStyle:       tcell.StyleDefault.Foreground(solarizedMagenta).Bold(true),
		StatusStyle:      tcell.StyleDefault.Foreground(solarizedBase3).Background(prettyBlue),
		LineNumberStyle:  tcell.StyleDefault.Foreground(prettyBlue).Dim(true),
		HeaderStyle:      tcell.StyleDefault.Background(prettyBlue).Foreground(solarizedBase3).Bold(true),
		PromptStyle:      tcell.StyleDefault.Foreground(solarizedBase3).Background(prettyBlue),
		PromptErrorStyle: tcell.StyleDefault.Foreground(solarizedRed).Background(prettyBlue).Bold(true),
	},
}

func rgb(r, g, b uint8) tcolor.Color {
	return tcolor.NewRGBColor(int32(r), int32(g), int32(b))
}
