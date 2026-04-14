// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"os"
	"syscall"
)

func programSuspendSelf() error {
	return syscall.Kill(os.Getpid(), syscall.SIGTSTP)
}
