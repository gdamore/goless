// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"testing"

	"github.com/gdamore/tcell/v3"
)

type recordingSuspendScreen struct {
	tcell.Screen
	calls      []string
	suspendErr error
	resumeErr  error
}

func (s *recordingSuspendScreen) Suspend() error {
	s.calls = append(s.calls, "suspend")
	return s.suspendErr
}

func (s *recordingSuspendScreen) Resume() error {
	s.calls = append(s.calls, "resume")
	return s.resumeErr
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

func TestSuspendProgramScreenSuspendFailure(t *testing.T) {
	_, base := newMockProgramScreen(t, 80, 24)

	wantErr := errors.New("suspend failed")
	screen := &recordingSuspendScreen{Screen: base, suspendErr: wantErr}
	actionCalled := false
	err := suspendProgramScreen(screen, nil, true, func() error {
		actionCalled = true
		screen.calls = append(screen.calls, "action")
		return nil
	})
	if err != wantErr {
		t.Fatalf("suspendProgramScreen(...) error = %v, want %v", err, wantErr)
	}
	if actionCalled {
		t.Fatal("action was called after suspend failure")
	}
	if got, want := screen.calls, []string{"suspend"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("call sequence = %v, want %v", got, want)
	}
}

func TestSuspendProgramScreenActionFailure(t *testing.T) {
	_, base := newMockProgramScreen(t, 80, 24)

	wantErr := errors.New("action failed")
	screen := &recordingSuspendScreen{Screen: base}
	err := suspendProgramScreen(screen, nil, true, func() error {
		screen.calls = append(screen.calls, "action")
		return wantErr
	})
	if err != wantErr {
		t.Fatalf("suspendProgramScreen(...) error = %v, want %v", err, wantErr)
	}
	wantCalls := []string{"suspend", "action", "resume", "enable-mouse", "sync"}
	if got := screen.calls; len(got) != len(wantCalls) {
		t.Fatalf("call sequence = %v, want %v", got, wantCalls)
	}
	for i := range wantCalls {
		if screen.calls[i] != wantCalls[i] {
			t.Fatalf("call sequence = %v, want %v", screen.calls, wantCalls)
		}
	}
}

func TestSuspendProgramScreenResumeFailure(t *testing.T) {
	_, base := newMockProgramScreen(t, 80, 24)

	wantErr := errors.New("resume failed")
	screen := &recordingSuspendScreen{Screen: base, resumeErr: wantErr}
	err := suspendProgramScreen(screen, nil, false, func() error {
		screen.calls = append(screen.calls, "action")
		return nil
	})
	if err != wantErr {
		t.Fatalf("suspendProgramScreen(...) error = %v, want %v", err, wantErr)
	}
	wantCalls := []string{"suspend", "action", "resume"}
	if got := screen.calls; len(got) != len(wantCalls) {
		t.Fatalf("call sequence = %v, want %v", got, wantCalls)
	}
	for i := range wantCalls {
		if screen.calls[i] != wantCalls[i] {
			t.Fatalf("call sequence = %v, want %v", screen.calls, wantCalls)
		}
	}
}
