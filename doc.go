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
//   - construct a Pager with New
//   - append content with Append, AppendString, or ReadFrom
//   - size it with SetSize
//   - render it with Draw
//   - feed input through HandleKey or direct navigation/search methods
//
// The bundled demo in cmd/goless-demo shows the intended integration model for
// a full-screen tcell application.
package goless
