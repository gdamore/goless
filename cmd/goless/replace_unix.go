// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import "os"

func replaceProgramFile(tempPath, path string) error {
	return os.Rename(tempPath, path)
}
