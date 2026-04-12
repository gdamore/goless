// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package main

import (
	"fmt"
	"os"
)

func replaceProgramFile(tempPath, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return os.Rename(tempPath, path)
		}
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s exists and is not a regular file", path)
	}
	if err := os.Remove(path); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
