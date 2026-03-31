// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless_test

import (
	"strings"

	"github.com/gdamore/goless"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
)

func ExampleNew() {
	pager := goless.New(goless.Config{
		WrapMode:   goless.NoWrap,
		KeyGroup:   goless.LessKeyGroup,
		RenderMode: goless.RenderHybrid,
		ShowStatus: true,
		Chrome: goless.Chrome{
			Title: "Example",
			Frame: goless.RoundedFrame(),
		},
	})

	_, _ = pager.ReadFrom(strings.NewReader("hello\nworld\n"))
	pager.Flush()
	pager.SetSize(80, 24)

	term := vt.NewMockTerm(vt.MockOptSize{X: 80, Y: 24})
	screen, err := tcell.NewTerminfoScreenFromTty(term)
	if err != nil {
		return
	}
	if err := screen.Init(); err != nil {
		return
	}
	defer screen.Fini()

	pager.Draw(screen)
	if result := pager.HandleKeyResult(tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone)); result.Handled {
		return
	}
	_ = pager.SearchForward("world")
	pager.HandleKey(tcell.NewEventKey(tcell.KeyRune, "G", tcell.ModNone))
	pager.Draw(screen)
}
