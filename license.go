// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package goless

import _ "embed"

//go:embed LICENSE
var licenseText string

// LicenseText returns the project's bundled Apache License text.
func LicenseText() string {
	return licenseText
}
