// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package main

import "os"

func replaceProgramFile(tempPath, path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tempPath, path)
}
