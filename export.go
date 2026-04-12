// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import iview "github.com/gdamore/goless/internal/view"

// ExportScope selects which portion of the current pager content is exported.
type ExportScope int

const (
	// ExportCurrentContent writes the full current content set after view-time
	// transforms such as squeeze, and later any filtering, have been applied.
	ExportCurrentContent ExportScope = iota
	// ExportViewport writes only the content currently visible in the viewport.
	ExportViewport
)

// ExportFormat selects whether exported output contains ANSI styling.
type ExportFormat int

const (
	// ExportPlain writes plain text without ANSI styling.
	ExportPlain ExportFormat = iota
	// ExportANSI writes ANSI-styled output reconstructed from the current content.
	ExportANSI
)

// ExportOptions control pager export behavior.
type ExportOptions struct {
	Scope  ExportScope
	Format ExportFormat
}

func toInternalExportOptions(opts ExportOptions) iview.ExportOptions {
	return iview.ExportOptions{
		Scope:  iview.ExportScope(opts.Scope),
		Format: iview.ExportFormat(opts.Format),
	}
}
