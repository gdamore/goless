// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import "testing"

func TestDefaultTextHelpCloseUsesF1(t *testing.T) {
	if got, want := DefaultText().HelpClose, "Esc/q/F1 close"; got != want {
		t.Fatalf("DefaultText().HelpClose = %q, want %q", got, want)
	}
}
