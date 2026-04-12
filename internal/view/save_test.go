// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package view

import (
	"testing"

	"github.com/gdamore/goless/internal/layout"
	"github.com/gdamore/goless/internal/model"
)

func TestBeginSavePromptRequiresCommandHandler(t *testing.T) {
	v := New(model.NewDocument(4), Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
	})
	if v.BeginSavePrompt() {
		t.Fatal("BeginSavePrompt() = true, want false without command handler")
	}
}

func TestSaveModeCyclersIgnoreMissingPromptAndConfirmationPrompt(t *testing.T) {
	v := New(model.NewDocument(4), Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
	})

	if got, want := v.cycleSaveScope(), ExportScopeContent; got != want {
		t.Fatalf("cycleSaveScope() without prompt = %v, want %v", got, want)
	}
	if got, want := v.cycleSaveFormat(), ExportFormatANSI; got != want {
		t.Fatalf("cycleSaveFormat() without prompt = %v, want %v", got, want)
	}

	v.prompt = &promptState{
		export: ExportOptions{
			Scope:  ExportScopeViewport,
			Format: ExportFormatPlain,
		},
		saveConfirm: &saveConfirmState{path: "out.txt"},
	}
	if got, want := v.cycleSaveScope(), ExportScopeViewport; got != want {
		t.Fatalf("cycleSaveScope() during confirm = %v, want %v", got, want)
	}
	if got, want := v.cycleSaveFormat(), ExportFormatPlain; got != want {
		t.Fatalf("cycleSaveFormat() during confirm = %v, want %v", got, want)
	}
}

func TestRunSavePromptShowsUnknownCommandWithoutHandler(t *testing.T) {
	v := New(model.NewDocument(4), Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
	})
	v.beginPrompt(promptSave)

	commit, quit := v.runSavePrompt("out.txt")
	if !commit || quit {
		t.Fatalf("runSavePrompt(no handler) = commit %v quit %v, want commit true quit false", commit, quit)
	}
	if got, want := v.message, "unknown command: save"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestRunSavePromptShowsUnknownCommandWhenUnhandled(t *testing.T) {
	v := New(model.NewDocument(4), Config{
		TabWidth: 4,
		WrapMode: layout.NoWrap,
		CommandHandler: func(Command) CommandResult {
			return CommandResult{}
		},
	})
	v.beginPrompt(promptSave)

	commit, quit := v.runSavePrompt("out.txt")
	if !commit || quit {
		t.Fatalf("runSavePrompt(unhandled) = commit %v quit %v, want commit true quit false", commit, quit)
	}
	if got, want := v.message, "unknown command: save"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}
