// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/gdamore/tcell/v3"
)

type recordingSuspendScreen struct {
	tcell.Screen
	calls []string
}

func (s *recordingSuspendScreen) Suspend() error {
	s.calls = append(s.calls, "suspend")
	return nil
}

func (s *recordingSuspendScreen) Resume() error {
	s.calls = append(s.calls, "resume")
	return nil
}

func (s *recordingSuspendScreen) EnableMouse(flags ...tcell.MouseFlags) {
	s.calls = append(s.calls, "enable-mouse")
}

func (s *recordingSuspendScreen) DisableMouse() {
	s.calls = append(s.calls, "disable-mouse")
}

func (s *recordingSuspendScreen) Sync() {
	s.calls = append(s.calls, "sync")
}

func (s *recordingSuspendScreen) Size() (int, int) {
	s.calls = append(s.calls, "size")
	return 0, 0
}

func TestSuspendProgramScreenRestoresScreenSession(t *testing.T) {
	_, base := newMockProgramScreen(t, 80, 24)

	screen := &recordingSuspendScreen{Screen: base}
	var actionCalled bool
	if err := suspendProgramScreen(screen, nil, true, func() error {
		actionCalled = true
		screen.calls = append(screen.calls, "action")
		return nil
	}); err != nil {
		t.Fatalf("suspendProgramScreen(...) failed: %v", err)
	}
	if !actionCalled {
		t.Fatal("action was not called")
	}
	want := []string{"suspend", "action", "resume", "enable-mouse", "sync"}
	if got := screen.calls; len(got) != len(want) {
		t.Fatalf("call sequence = %v, want %v", got, want)
	}
	for i := range want {
		if screen.calls[i] != want[i] {
			t.Fatalf("call sequence = %v, want %v", screen.calls, want)
		}
	}
}

func TestSuspendProgramScreenUsesScreenMouseState(t *testing.T) {
	_, base := newMockProgramScreen(t, 80, 24)

	screen := &recordingSuspendScreen{Screen: base}
	if err := suspendProgramScreen(screen, nil, false, func() error {
		screen.calls = append(screen.calls, "action")
		return nil
	}); err != nil {
		t.Fatalf("suspendProgramScreen(...) failed: %v", err)
	}
	want := []string{"suspend", "action", "resume", "disable-mouse", "sync"}
	if got := screen.calls; len(got) != len(want) {
		t.Fatalf("call sequence = %v, want %v", got, want)
	}
	for i := range want {
		if screen.calls[i] != want[i] {
			t.Fatalf("call sequence = %v, want %v", screen.calls, want)
		}
	}
}
