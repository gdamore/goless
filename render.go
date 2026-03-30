// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

// RenderMode controls how escape and control sequences are presented.
type RenderMode int

const (
	// RenderHybrid applies supported styling and shows unsupported sequences visibly.
	RenderHybrid RenderMode = iota
	// RenderLiteral shows escape and control sequences literally and does not apply styling.
	RenderLiteral
	// RenderPresentation applies supported styling and hides unsupported sequences.
	RenderPresentation
)
