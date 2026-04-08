// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

// Package goless provides a secure, embeddable pager core for Go programs.
//
// A Pager consumes appendable content, normalizes escape sequences into a safe
// display model, and renders that model onto a tcell.Screen. It is intended for
// applications that want "less"-style viewing behavior without shell escape
// support or other subprocess features.
//
// The package exposes a small controller-oriented API:
//
//   - construct a Pager with New and Option values
//   - append content with Append, AppendString, or ReadFrom
//   - size it with SetSize
//   - render it with Draw
//   - enable mouse reporting on the host tcell.Screen when wheel input is desired
//   - feed input through HandleKey, HandleKeyResult, HandleMouse, HandleMouseResult, or direct navigation/search methods
//   - reconfigure runtime-safe behavior with Configure and RuntimeOption values
//   - inspect visible search state with SearchState when host chrome needs it
//
// OSC 8 hyperlinks are parsed in presentation-oriented render modes, but they
// remain inert unless the embedder opts in with HyperlinkHandler. That handler
// is expected to make the application's trust decision explicitly: keep the
// link inert, allow it to go live, rewrite the target, and/or restyle the
// linked span.
//
// The bundled program in cmd/goless shows the intended integration model for a
// full-screen tcell application.
package goless
