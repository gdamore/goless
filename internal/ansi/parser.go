// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package ansi

import (
	"bytes"
	"strconv"
	"strings"
	"unicode/utf8"
)

type parserMode uint8

const (
	modeInit parserMode = iota
	modeEsc
	modeCSI
	modeOSC
	modeStr
	modeUTF
)

// RenderMode controls how escape and control sequences are presented.
type RenderMode uint8

const (
	// RenderHybrid applies supported styling and shows unsupported sequences visibly.
	RenderHybrid RenderMode = iota
	// RenderLiteral shows escape and control sequences literally and does not apply styling.
	RenderLiteral
	// RenderPresentation applies supported styling and hides unsupported sequences.
	RenderPresentation
)

// Receiver consumes visible output emitted by the parser.
type Receiver interface {
	Print(r rune, style Style, offset int64)
	Newline(style Style, offset int64)
}

// Parser incrementally parses text, UTF-8, and ANSI escape sequences.
type Parser struct {
	receiver        Receiver
	step            func(byte)
	mode            parserMode
	buf             bytes.Buffer
	utfLen          int
	offset          int64
	style           Style
	pendingCR       bool
	pendingCROffset int64
	renderMode      RenderMode
}

// NewParser constructs a parser that sends visible output to the receiver.
func NewParser(receiver Receiver) *Parser {
	return NewParserWithMode(receiver, RenderHybrid)
}

// NewParserWithMode constructs a parser with the supplied render mode.
func NewParserWithMode(receiver Receiver, mode RenderMode) *Parser {
	p := &Parser{
		receiver:   receiver,
		style:      DefaultStyle(),
		renderMode: mode,
	}
	p.setStep(modeInit, p.stateInit)
	return p
}

// Style returns the current parser style state.
func (p *Parser) Style() Style {
	return p.style
}

// Write feeds bytes into the parser.
func (p *Parser) Write(data []byte) (int, error) {
	for _, b := range data {
		p.offset++
		p.processByte(b)
	}
	return len(data), nil
}

// Flush emits any pending visible fallbacks for incomplete sequences.
func (p *Parser) Flush() {
	if p.pendingCR {
		p.emitRawControl('\r', p.pendingCROffset)
		p.pendingCR = false
	}

	switch p.mode {
	case modeUTF:
		p.emitInvalidBytes(p.buf.Bytes(), p.offset)
	case modeEsc, modeCSI, modeOSC, modeStr:
		if p.showsUnsupportedSequences() {
			p.emitEscapeVisible(nil)
		}
	}

	p.buf.Reset()
	p.utfLen = 0
	p.setStep(modeInit, p.stateInit)
}

func (p *Parser) processByte(b byte) {
	if p.pendingCR {
		if b != '\n' {
			p.emitRawControl('\r', p.pendingCROffset)
		}
		p.pendingCR = false
	}
	p.step(b)
}

func (p *Parser) setStep(mode parserMode, step func(byte)) {
	p.mode = mode
	p.step = step
}

func (p *Parser) stateInit(b byte) {
	p.buf.Reset()

	if b >= ' ' && b < 0x7f {
		p.emitRune(rune(b), p.offset)
		return
	}

	if p.renderMode != RenderLiteral {
		if esc, ok := c1EscapeByte(b); ok {
			p.stateEsc(esc)
			return
		}
	}

	switch b {
	case 0x1b:
		p.buf.Reset()
		p.setStep(modeEsc, p.stateEsc)
	case '\t':
		p.emitRune('\t', p.offset)
	case '\n':
		p.receiver.Newline(p.style, p.offset)
	case '\r':
		p.pendingCR = true
		p.pendingCROffset = p.offset
	default:
		switch {
		case b&0xE0 == 0xC0:
			p.utfLen = 2
			p.buf.WriteByte(b)
			p.setStep(modeUTF, p.stateUTF)
		case b&0xF0 == 0xE0:
			p.utfLen = 3
			p.buf.WriteByte(b)
			p.setStep(modeUTF, p.stateUTF)
		case b&0xF8 == 0xF0:
			p.utfLen = 4
			p.buf.WriteByte(b)
			p.setStep(modeUTF, p.stateUTF)
		case b >= 0x80:
			p.emitInvalidBytes([]byte{b}, p.offset)
		default:
			p.emitRawControl(b, p.offset)
		}
	}
}

func (p *Parser) stateEsc(b byte) {
	p.buf.WriteByte(b)

	switch b {
	case '[':
		p.setStep(modeCSI, p.stateCSI)
	case ']':
		p.setStep(modeOSC, p.stateOSC)
	case 'P', '^', '_', 'X':
		p.setStep(modeStr, p.stateStr)
	default:
		if p.showsUnsupportedSequences() {
			p.emitEscapeVisible([]byte{b})
			return
		}
		p.buf.Reset()
		p.setStep(modeInit, p.stateInit)
	}
}

func (p *Parser) stateCSI(b byte) {
	if b >= 0x30 && b <= 0x3f {
		p.buf.WriteByte(b)
		return
	}
	if b >= 0x20 && b <= 0x2f {
		p.buf.WriteByte(b)
		return
	}
	if b >= 0x40 && b <= 0x7e {
		body := p.buf.Bytes()
		if len(body) > 0 && body[0] == '[' && b == 'm' && p.renderMode != RenderLiteral {
			p.processSGR(string(body[1:]))
		} else if p.showsUnsupportedSequences() {
			p.emitEscapeVisible([]byte{b})
			return
		}
		p.buf.Reset()
		p.setStep(modeInit, p.stateInit)
		return
	}

	if p.showsUnsupportedSequences() {
		p.emitEscapeVisible([]byte{b})
		return
	}
	p.buf.Reset()
	p.setStep(modeInit, p.stateInit)
}

func (p *Parser) stateOSC(b byte) {
	switch b {
	case 0x07:
		handled := p.handlesOSC() && p.processOSC()
		if p.showsUnsupportedSequences() && !handled {
			p.emitEscapeVisible([]byte{0x07})
			return
		}
		p.buf.Reset()
		p.setStep(modeInit, p.stateInit)
	case 0x9c:
		handled := p.handlesOSC() && p.processOSC()
		if p.showsUnsupportedSequences() && !handled {
			p.emitEscapeVisible([]byte{0x1b, '\\'})
			return
		}
		p.buf.Reset()
		p.setStep(modeInit, p.stateInit)
	case '\\':
		buf := p.buf.Bytes()
		if len(buf) > 0 && buf[len(buf)-1] == 0x1b {
			p.buf.Truncate(p.buf.Len() - 1)
			handled := p.handlesOSC() && p.processOSC()
			if p.showsUnsupportedSequences() && !handled {
				p.emitEscapeVisible([]byte{0x1b, '\\'})
				return
			}
			p.buf.Reset()
			p.setStep(modeInit, p.stateInit)
			return
		}
		p.buf.WriteByte(b)
	default:
		p.buf.WriteByte(b)
	}
}

func (p *Parser) stateStr(b byte) {
	switch b {
	case 0x07, 0x9c:
		if p.showsUnsupportedSequences() {
			final := []byte{b}
			if b == 0x9c {
				final = []byte{0x1b, '\\'}
			}
			p.emitEscapeVisible(final)
			return
		}
		p.buf.Reset()
		p.setStep(modeInit, p.stateInit)
	case '\\':
		buf := p.buf.Bytes()
		if len(buf) > 0 && buf[len(buf)-1] == 0x1b {
			p.buf.Truncate(p.buf.Len() - 1)
			if p.showsUnsupportedSequences() {
				p.emitEscapeVisible([]byte{0x1b, '\\'})
				return
			}
			p.buf.Reset()
			p.setStep(modeInit, p.stateInit)
			return
		}
		p.buf.WriteByte(b)
	default:
		p.buf.WriteByte(b)
	}
}

func (p *Parser) stateUTF(b byte) {
	if b&0xC0 == 0x80 {
		p.buf.WriteByte(b)
		if p.buf.Len() == p.utfLen {
			r, _ := utf8.DecodeRune(p.buf.Bytes())
			if r == utf8.RuneError {
				p.emitInvalidBytes(p.buf.Bytes(), p.offset)
			} else {
				p.emitRune(r, p.offset)
			}
			p.buf.Reset()
			p.utfLen = 0
			p.setStep(modeInit, p.stateInit)
		}
		return
	}

	p.emitInvalidBytes(p.buf.Bytes(), p.offset-1)
	p.buf.Reset()
	p.utfLen = 0
	p.setStep(modeInit, p.stateInit)
	p.stateInit(b)
}

func (p *Parser) emitRune(r rune, offset int64) {
	p.receiver.Print(r, p.style, offset)
}

func (p *Parser) emitLiteralByte(b byte, offset int64) {
	switch b {
	case '\t':
		p.emitRune('\t', offset)
	case '\n':
		p.receiver.Newline(p.style, offset)
	default:
		if b >= ' ' && b < 0x7f {
			p.emitRune(rune(b), offset)
			return
		}
		if b < ' ' || b == 0x7f {
			p.emitRawControl(b, offset)
			return
		}
		p.emitInvalidBytes([]byte{b}, offset)
	}
}

func (p *Parser) emitRawControl(b byte, offset int64) {
	if p.renderMode != RenderLiteral {
		return
	}
	if r, ok := controlPicture(b); ok {
		p.emitRune(r, offset)
	}
}

func (p *Parser) emitInvalidBytes(data []byte, endOffset int64) {
	if len(data) == 0 {
		return
	}
	startOffset := endOffset - int64(len(data)) + 1
	for i, b := range data {
		offset := startOffset + int64(i)
		if p.renderMode == RenderLiteral {
			p.emitHexByte(b, offset)
			continue
		}
		p.emitRune(utf8.RuneError, offset)
	}
}

func (p *Parser) emitHexByte(b byte, offset int64) {
	const digits = "0123456789abcdef"
	p.emitRune('\\', offset)
	p.emitRune('x', offset)
	p.emitRune(rune(digits[b>>4]), offset)
	p.emitRune(rune(digits[b&0x0f]), offset)
}

func (p *Parser) emitEscapeVisible(final []byte) {
	p.emitVisibleByte(0x1b, p.offset)
	for _, b := range p.buf.Bytes() {
		p.emitVisibleByte(b, p.offset)
	}
	for _, b := range final {
		p.emitVisibleByte(b, p.offset)
	}
	p.buf.Reset()
	p.setStep(modeInit, p.stateInit)
}

func (p *Parser) emitVisibleByte(b byte, offset int64) {
	switch b {
	case '\t':
		p.emitRune('\t', offset)
	case '\n':
		p.receiver.Newline(p.style, offset)
	default:
		if b >= ' ' && b < 0x7f {
			p.emitRune(rune(b), offset)
			return
		}
		if b < ' ' || b == 0x7f {
			if r, ok := controlPicture(b); ok {
				p.emitRune(r, offset)
				return
			}
		}
		p.emitHexByte(b, offset)
	}
}

func (p *Parser) processOSC() bool {
	body := p.buf.String()
	if body == "" || body[0] != ']' {
		return false
	}
	body = body[1:]
	cmd, data, ok := strings.Cut(body, ";")
	if !ok {
		return false
	}
	if cmd != "8" {
		return false
	}
	p.processHyperlink(data)
	return true
}

func (p *Parser) processHyperlink(data string) {
	params, uri, ok := strings.Cut(data, ";")
	if !ok {
		return
	}
	if uri == "" {
		p.style.URL = ""
		p.style.URLID = ""
		return
	}

	id := ""
	for pair := range strings.SplitSeq(params, ":") {
		if val, ok := strings.CutPrefix(pair, "id="); ok {
			id = val
		}
	}
	p.style.URL = uri
	p.style.URLID = id
}

func (p *Parser) showsUnsupportedSequences() bool {
	return p.renderMode != RenderPresentation
}

func (p *Parser) handlesOSC() bool {
	return p.renderMode != RenderLiteral
}

func controlPicture(b byte) (rune, bool) {
	switch {
	case b <= 0x1f:
		return rune(0x2400) + rune(b), true
	case b == 0x7f:
		return 0x2421, true
	default:
		return 0, false
	}
}

func c1EscapeByte(b byte) (byte, bool) {
	switch b {
	case 0x90, 0x98, 0x9b, 0x9d, 0x9e, 0x9f:
		return b - 0x40, true
	default:
		return 0, false
	}
}

func (p *Parser) processSGR(body string) {
	if body == "" {
		url := p.style.URL
		urlID := p.style.URLID
		p.style = DefaultStyle()
		p.style.URL = url
		p.style.URLID = urlID
		return
	}

	parts := strings.Split(body, ";")
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if part == "" {
			part = "0"
		}
		code, err := strconv.Atoi(part)
		if err != nil {
			continue
		}

		switch {
		case code == 0:
			url := p.style.URL
			urlID := p.style.URLID
			p.style = DefaultStyle()
			p.style.URL = url
			p.style.URLID = urlID
		case code == 1:
			p.style.Bold = true
		case code == 2:
			p.style.Dim = true
		case code == 3:
			p.style.Italic = true
		case code == 4:
			p.style.Underline = true
		case code == 9:
			p.style.Strike = true
		case code == 5:
			p.style.Blink = true
		case code == 7:
			p.style.Reverse = true
		case code == 22:
			p.style.Bold = false
			p.style.Dim = false
		case code == 23:
			p.style.Italic = false
		case code == 24:
			p.style.Underline = false
		case code == 29:
			p.style.Strike = false
		case code == 25:
			p.style.Blink = false
		case code == 27:
			p.style.Reverse = false
		case code >= 30 && code <= 37:
			p.style.Fg = IndexedColor(uint8(code - 30))
		case code == 39:
			p.style.Fg = DefaultColor()
		case code >= 40 && code <= 47:
			p.style.Bg = IndexedColor(uint8(code - 40))
		case code == 49:
			p.style.Bg = DefaultColor()
		case code >= 90 && code <= 97:
			p.style.Fg = IndexedColor(uint8(code - 90 + 8))
		case code >= 100 && code <= 107:
			p.style.Bg = IndexedColor(uint8(code - 100 + 8))
		case code == 38 || code == 48:
			color, consumed, ok := parseExtendedColor(parts[i+1:])
			if !ok {
				continue
			}
			if code == 38 {
				p.style.Fg = color
			} else {
				p.style.Bg = color
			}
			i += consumed
		}
	}
}

func parseExtendedColor(parts []string) (Color, int, bool) {
	if len(parts) < 2 {
		return Color{}, 0, false
	}

	mode, err := strconv.Atoi(parts[0])
	if err != nil {
		return Color{}, 0, false
	}

	switch mode {
	case 5:
		index, err := strconv.Atoi(parts[1])
		if err != nil || index < 0 || index > 255 {
			return Color{}, 0, false
		}
		return IndexedColor(uint8(index)), 2, true
	case 2:
		if len(parts) < 4 {
			return Color{}, 0, false
		}
		r, errR := strconv.Atoi(parts[1])
		g, errG := strconv.Atoi(parts[2])
		b, errB := strconv.Atoi(parts[3])
		if errR != nil || errG != nil || errB != nil {
			return Color{}, 0, false
		}
		if r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255 {
			return Color{}, 0, false
		}
		return RGBColor(uint8(r), uint8(g), uint8(b)), 4, true
	default:
		return Color{}, 0, false
	}
}
