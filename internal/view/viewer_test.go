// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gdamore/goless/internal/layout"
	"github.com/gdamore/goless/internal/model"
	"github.com/gdamore/tcell/v3"
	tcolor "github.com/gdamore/tcell/v3/color"
	"github.com/gdamore/tcell/v3/vt"
)

func TestToggleWrapPreservesAnchor(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("ab界cd")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(3, 2)
	v.colOffset = 2
	v.relayout()

	before := v.firstVisibleAnchor()
	if got, want := before.GraphemeIndex, 2; got != want {
		t.Fatalf("anchor grapheme = %d, want %d", got, want)
	}

	v.ToggleWrap()
	after := v.firstVisibleAnchor()
	if got, want := after, before; got != want {
		t.Fatalf("anchor after toggle = %+v, want %+v", got, want)
	}
	if got, want := v.cfg.WrapMode, layout.SoftWrap; got != want {
		t.Fatalf("wrap mode = %v, want %v", got, want)
	}
}

func TestToggleWrapToNoWrapRestoresHorizontalAnchor(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("ab界cd")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.SoftWrap})
	v.SetSize(3, 2)
	v.rowOffset = 1
	v.relayout()

	before := v.firstVisibleAnchor()
	if got, want := before.GraphemeIndex, 2; got != want {
		t.Fatalf("anchor grapheme = %d, want %d", got, want)
	}

	v.ToggleWrap()

	if got, want := v.cfg.WrapMode, layout.NoWrap; got != want {
		t.Fatalf("wrap mode = %v, want %v", got, want)
	}
	if got, want := v.colOffset, 2; got != want {
		t.Fatalf("col offset = %d, want %d", got, want)
	}
	after := v.firstVisibleAnchor()
	if got, want := after, before; got != want {
		t.Fatalf("anchor after toggle = %+v, want %+v", got, want)
	}
}

func TestScrollRightClampsToContent(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdef")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap})
	v.SetSize(3, 4)
	v.ScrollRight(100)

	if got, want := v.colOffset, 3; got != want {
		t.Fatalf("col offset = %d, want %d", got, want)
	}
}

func TestPromptSearchFindsAndRepeatsForward(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\nbeta\nalpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"a", "l", "p", "h", "a"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.search.Query, "alpha"; got != want {
		t.Fatalf("search query = %q, want %q", got, want)
	}
	if got, want := len(v.search.Matches), 2; got != want {
		t.Fatalf("match count = %d, want %d", got, want)
	}
	if got, want := v.search.Current, 0; got != want {
		t.Fatalf("current match = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("n"))
	if got, want := v.search.Current, 1; got != want {
		t.Fatalf("current match after n = %d, want %d", got, want)
	}
	if got, want := v.firstVisibleAnchor().LineIndex, 2; got != want {
		t.Fatalf("visible line after n = %d, want %d", got, want)
	}

	v.HandleKey(keyRune("N"))
	if got, want := v.search.Current, 0; got != want {
		t.Fatalf("current match after N = %d, want %d", got, want)
	}
}

func TestEmptySearchClearsExistingMatches(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\nbeta\nalpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"a", "l", "p", "h", "a"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	v.HandleKey(keyRune("/"))
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.search.Query, ""; got != want {
		t.Fatalf("search query after empty submit = %q, want %q", got, want)
	}
	if got, want := len(v.search.Matches), 0; got != want {
		t.Fatalf("match count after empty submit = %d, want %d", got, want)
	}
	if got, want := v.search.Current, 0; got != want {
		t.Fatalf("current match after empty submit = %d, want %d", got, want)
	}
}

func TestPromptCommandJumpsToLine(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\nfour\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)

	v.HandleKey(keyRune(":"))
	v.HandleKey(keyRune("3"))
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.firstVisibleAnchor().LineIndex, 2; got != want {
		t.Fatalf("visible line after :3 = %d, want %d", got, want)
	}
}

func TestSearchRevealKeepsVisibleHorizontalContext(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("0123456789 target suffix")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(10, 2)
	v.colOffset = 8
	v.relayout()

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"t", "a", "r", "g", "e", "t"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.colOffset, 8; got != want {
		t.Fatalf("col offset after visible search = %d, want %d", got, want)
	}
}

func TestSearchRevealOnlyScrollsEnoughToShowMatch(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("0123456789 target suffix")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(10, 2)
	v.colOffset = 0
	v.relayout()

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"t", "a", "r", "g", "e", "t"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.colOffset, 7; got != want {
		t.Fatalf("col offset after reveal = %d, want %d", got, want)
	}
}

func TestSearchRevealKeepsVisibleVerticalContext(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\nfour\nfive\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 4)
	v.rowOffset = 1
	v.relayout()

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"f", "o", "u", "r"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.rowOffset, 1; got != want {
		t.Fatalf("row offset after visible search = %d, want %d", got, want)
	}
}

func TestSearchRevealOnlyScrollsEnoughVertically(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\nfour\nfive\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 4)
	v.rowOffset = 0
	v.relayout()

	v.HandleKey(keyRune("/"))
	for _, s := range []string{"f", "o", "u", "r"} {
		v.HandleKey(keyRune(s))
	}
	v.HandleKey(keyKey(tcell.KeyEnter))

	if got, want := v.rowOffset, 1; got != want {
		t.Fatalf("row offset after reveal = %d, want %d", got, want)
	}
}

func TestApplyMatchCellStyleUsesUnderlineAccents(t *testing.T) {
	base := tcell.StyleDefault.Foreground(tcolor.White)

	inactive := applyMatchCellStyle(base, false)
	if got, want := inactive.GetUnderlineStyle(), tcell.UnderlineStyleNone; got != want {
		t.Fatalf("inactive underline style = %v, want %v", got, want)
	}
	if got, want := inactive.GetBackground(), inactiveMatchStyle.Bg; got != want {
		t.Fatalf("inactive background = %v, want %v", got, want)
	}
	if got, want := inactive.GetForeground(), inactiveMatchStyle.Fg; got != want {
		t.Fatalf("inactive foreground = %v, want %v", got, want)
	}
	if inactive.HasBold() {
		t.Fatalf("inactive match should not force bold")
	}

	current := applyMatchCellStyle(base, true)
	if got, want := current.GetUnderlineStyle(), currentMatchStyle.UnderlineStyle; got != want {
		t.Fatalf("current underline style = %v, want %v", got, want)
	}
	if got, want := current.GetUnderlineColor(), currentMatchStyle.UnderlineColor; got != want {
		t.Fatalf("current underline color = %v, want %v", got, want)
	}
	if got, want := current.GetBackground(), currentMatchStyle.Bg; got != want {
		t.Fatalf("current background = %v, want %v", got, want)
	}
	if got, want := current.GetForeground(), currentMatchStyle.Fg; got != want {
		t.Fatalf("current foreground = %v, want %v", got, want)
	}
	if !current.HasBold() {
		t.Fatalf("current match should be bold")
	}
}

func TestRefreshPicksUpAppendedDocumentContent(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 4)

	if err := doc.Append([]byte("hello\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	doc.Flush()
	v.Refresh()

	if got, want := len(v.lines), 1; got != want {
		t.Fatalf("line count after refresh = %d, want %d", got, want)
	}
	if got, want := v.lines[0].Text, "hello"; got != want {
		t.Fatalf("line text after refresh = %q, want %q", got, want)
	}
}

func TestRefreshInFollowModeStaysAtBottomAfterAppend(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.Follow()

	if err := doc.Append([]byte("three\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	doc.Flush()
	v.Refresh()

	if !v.Following() {
		t.Fatalf("follow mode = false, want true")
	}
	if got, want := v.rowOffset, v.maxRowOffset(); got != want {
		t.Fatalf("row offset after append = %d, want %d", got, want)
	}
}

func TestScrollUpExitsFollowMode(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.Follow()
	v.HandleKey(keyRune("k"))

	if v.Following() {
		t.Fatalf("follow mode = true, want false")
	}
}

func TestRefreshWithoutFollowKeepsViewportWhenAppended(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.rowOffset = 0
	v.relayout()

	if err := doc.Append([]byte("four\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	doc.Flush()
	v.Refresh()

	if v.Following() {
		t.Fatalf("follow mode = true, want false")
	}
	if got, want := v.rowOffset, 0; got != want {
		t.Fatalf("row offset after append = %d, want %d", got, want)
	}
}

func TestFollowKeyEnablesFollowMode(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.HandleKey(keyRune("F"))

	if !v.Following() {
		t.Fatalf("follow mode = false, want true")
	}
	if got, want := v.rowOffset, v.maxRowOffset(); got != want {
		t.Fatalf("row offset after F = %d, want %d", got, want)
	}
}

func TestCancelPromptPreservesFollowMode(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.Follow()
	v.HandleKey(keyRune("/"))
	v.HandleKey(keyKey(tcell.KeyEscape))

	if !v.Following() {
		t.Fatalf("follow mode = false, want true")
	}
}

func TestSearchRepeatWithoutActiveSearchPreservesFollowMode(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("one\ntwo\nthree\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.Follow()
	v.HandleKey(keyRune("n"))

	if !v.Following() {
		t.Fatalf("follow mode = false, want true")
	}
}

func TestToggleHelpMode(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})

	if got, want := v.mode, modeNormal; got != want {
		t.Fatalf("initial mode = %v, want %v", got, want)
	}
	v.HandleKey(keyRune("H"))
	if got, want := v.mode, modeHelp; got != want {
		t.Fatalf("mode after H = %v, want %v", got, want)
	}
	v.HandleKey(keyKey(tcell.KeyEscape))
	if got, want := v.mode, modeNormal; got != want {
		t.Fatalf("mode after Esc = %v, want %v", got, want)
	}
}

func TestViewerUsesCustomTextBundle(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{
		TabWidth:   4,
		WrapMode:   layout.NoWrap,
		ShowStatus: true,
		Text: Text{
			HelpTitle: "Ayuda",
		},
	})

	if got, want := v.text.HelpTitle, "Ayuda"; got != want {
		t.Fatalf("help title = %q, want %q", got, want)
	}
}

func TestDrawHelpRespectsFrameInsets(t *testing.T) {
	doc := model.NewDocument(4)
	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Chrome: Chrome{
			Frame: Frame{
				Horizontal:  "─",
				Vertical:    "│",
				TopLeft:     "╭",
				TopRight:    "╮",
				BottomLeft:  "╰",
				BottomRight: "╯",
			},
		},
	})
	v.SetSize(20, 6)
	v.toggleHelp()

	_, screen := newMockScreen(t, 20, 6)
	defer screen.Fini()

	v.Draw(screen)

	if got := cellRune(screen, 0, 0); got != '╭' {
		t.Fatalf("top-left border rune = %q, want %q", got, '╭')
	}
	if got := cellRune(screen, 0, 1); got != '│' {
		t.Fatalf("left border rune = %q, want %q", got, '│')
	}
	if got := cellRune(screen, 1, 1); got != 'N' {
		t.Fatalf("help body rune = %q, want %q", got, 'N')
	}
}

func TestStatusShowsRightOverflowIndicator(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdefghijklmnopqrstuvwxyz")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(10, 2)
	v.relayout()

	left, right := v.statusOverflow()
	if left {
		t.Fatalf("left overflow = true, want false")
	}
	if !right {
		t.Fatalf("right overflow = false, want true")
	}
	leftText, rightText := v.statusText()
	if strings.Contains(leftText, "▶") || strings.Contains(rightText, "▶") {
		t.Fatalf("status text (%q, %q) should not embed right overflow indicator", leftText, rightText)
	}
}

func TestStatusShowsBothOverflowIndicatorsWhenScrolledMidway(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("abcdefghijklmnopqrstuvwxyz")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(10, 2)
	v.colOffset = 5
	v.relayout()

	left, right := v.statusOverflow()
	if !left || !right {
		t.Fatalf("overflow = (%v, %v), want (true, true)", left, right)
	}
}

func TestStatusTextPlacesPositionOnRight(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\nbeta\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{TabWidth: 4, WrapMode: layout.NoWrap, ShowStatus: true})
	v.SetSize(20, 2)
	v.relayout()

	leftText, rightText := v.statusText()
	if leftText != "" {
		t.Fatalf("left status text = %q, want empty", leftText)
	}
	if !strings.Contains(rightText, "row 1/2") {
		t.Fatalf("right status text = %q, want row indicator", rightText)
	}
}

func TestStatusBarUsesDisplayWidthForRightAlignedText(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("alpha\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{
		TabWidth:   4,
		WrapMode:   layout.NoWrap,
		ShowStatus: true,
		Text: Text{
			StatusPosition: func(current, total, column int) string {
				return "界a"
			},
		},
	})
	v.SetSize(10, 2)
	v.relayout()

	_, screen := newMockScreen(t, 10, 2)
	defer screen.Fini()

	v.drawStatus(screen, 1)

	if got := cellRune(screen, 6, 1); got != '界' {
		t.Fatalf("status rune at expected start = %q, want %q", got, '界')
	}
}

func TestTruncateToWidthUsesDisplayCellsAndPreservesUTF8(t *testing.T) {
	got := truncateToWidth("é界b", 2)
	if !utf8.ValidString(got) {
		t.Fatalf("truncateToWidth produced invalid UTF-8: %q", got)
	}
	if got != "é" {
		t.Fatalf("truncateToWidth = %q, want %q", got, "é")
	}
}

func TestDrawFrameInsetsContentAndRendersTitle(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("hi\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Chrome: Chrome{
			Title: "Doc",
			Frame: Frame{
				Horizontal:  "─",
				Vertical:    "│",
				TopLeft:     "╭",
				TopRight:    "╮",
				BottomLeft:  "╰",
				BottomRight: "╯",
			},
		},
	})
	v.SetSize(10, 4)

	_, screen := newMockScreen(t, 10, 4)
	defer screen.Fini()

	v.Draw(screen)

	if got := cellRune(screen, 0, 0); got != '╭' {
		t.Fatalf("top-left border rune = %q, want %q", got, '╭')
	}
	if got := cellRune(screen, 2, 0); got != 'D' {
		t.Fatalf("title rune = %q, want %q", got, 'D')
	}
	if got := cellRune(screen, 0, 1); got != '│' {
		t.Fatalf("left border rune = %q, want %q", got, '│')
	}
	if got := cellRune(screen, 1, 1); got != 'h' {
		t.Fatalf("content rune = %q, want %q", got, 'h')
	}
}

func TestDrawFrameUsesBorderAndTitleStyles(t *testing.T) {
	doc := model.NewDocument(4)
	if err := doc.Append([]byte("hi\n")); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	borderStyle := tcell.StyleDefault.Foreground(tcolor.PaletteColor(4))
	titleStyle := tcell.StyleDefault.Foreground(tcolor.PaletteColor(15)).Bold(true)
	v := New(doc, Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		Chrome: Chrome{
			Title:       "Doc",
			BorderStyle: borderStyle,
			TitleStyle:  titleStyle,
			Frame: Frame{
				Horizontal:  "─",
				Vertical:    "│",
				TopLeft:     "┌",
				TopRight:    "┐",
				BottomLeft:  "└",
				BottomRight: "┘",
			},
		},
	})
	v.SetSize(10, 4)

	_, screen := newMockScreen(t, 10, 4)
	defer screen.Fini()

	v.Draw(screen)

	_, borderCellStyle, _ := screen.Get(0, 0)
	if got, want := borderCellStyle.GetForeground(), borderStyle.GetForeground(); got != want {
		t.Fatalf("border foreground = %v, want %v", got, want)
	}

	_, titleCellStyle, _ := screen.Get(2, 0)
	if got, want := titleCellStyle.GetForeground(), titleStyle.GetForeground(); got != want {
		t.Fatalf("title foreground = %v, want %v", got, want)
	}
	if !titleCellStyle.HasBold() {
		t.Fatalf("title style should be bold")
	}
}

func keyRune(s string) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, s, tcell.ModNone)
}

func keyKey(k tcell.Key) *tcell.EventKey {
	return tcell.NewEventKey(k, "", tcell.ModNone)
}

func cellRune(screen tcell.Screen, x, y int) rune {
	str, _, _ := screen.Get(x, y)
	if str == "" {
		return 0
	}
	return []rune(str)[0]
}

func newMockScreen(t *testing.T, width, height int) (vt.MockTerm, tcell.Screen) {
	t.Helper()
	if runtime.GOOS == "js" {
		t.Skip("not supported on webasm")
	}
	term := vt.NewMockTerm(vt.MockOptSize{X: vt.Col(width), Y: vt.Row(height)})
	screen, err := tcell.NewTerminfoScreenFromTty(term)
	if err != nil {
		t.Fatalf("failed to get mock screen: %v", err)
	}
	if err := screen.Init(); err != nil {
		t.Fatalf("failed to initialize mock screen: %v", err)
	}
	return term, screen
}
