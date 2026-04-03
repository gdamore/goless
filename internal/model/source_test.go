// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"os"
	"strings"
	"testing"
)

func readStore(t *testing.T, store ByteStore) string {
	t.Helper()

	buf := make([]byte, store.Len())
	if _, err := store.ReadAt(buf, 0); err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	return string(buf)
}

func TestBytesSourceFill(t *testing.T) {
	source := NewBytesSource([]byte("hello"))
	store := NewChunkStore(4)

	if err := source.Fill(store); err != nil {
		t.Fatalf("Fill failed: %v", err)
	}
	if got, want := store.Len(), int64(5); got != want {
		t.Fatalf("Len = %d, want %d", got, want)
	}
	if got, ok := source.Size(); !ok || got != 5 {
		t.Fatalf("Size = (%d, %v), want (5, true)", got, ok)
	}
}

func TestReaderSourceFill(t *testing.T) {
	source := NewReaderSource(strings.NewReader("stream"))
	store := NewChunkStore(4)

	if err := source.Fill(store); err != nil {
		t.Fatalf("Fill failed: %v", err)
	}
	if got, want := store.Len(), int64(6); got != want {
		t.Fatalf("Len = %d, want %d", got, want)
	}
	if _, ok := source.Size(); ok {
		t.Fatal("Size unexpectedly known for reader source")
	}
}

func TestFileSourceSize(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "goless-*")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })

	if _, err := file.WriteString("abcdef"); err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatalf("Seek failed: %v", err)
	}

	source := NewFileSource(file)
	if got, ok := source.Size(); !ok || got != 6 {
		t.Fatalf("Size = (%d, %v), want (6, true)", got, ok)
	}
}

func TestFileSourceSizeFromSeekedOffset(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "goless-*")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })

	if _, err := file.WriteString("abcdef"); err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}
	if _, err := file.Seek(2, 0); err != nil {
		t.Fatalf("Seek failed: %v", err)
	}

	source := NewFileSource(file)
	if got, ok := source.Size(); !ok || got != 4 {
		t.Fatalf("Size = (%d, %v), want (4, true)", got, ok)
	}

	store := NewChunkStore(4)
	if err := source.Fill(store); err != nil {
		t.Fatalf("Fill failed: %v", err)
	}
	if got := readStore(t, store); got != "cdef" {
		t.Fatalf("Fill data = %q, want %q", got, "cdef")
	}
}

func TestFileSourceFillUsesCapturedOffset(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "goless-*")
	if err != nil {
		t.Fatalf("CreateTemp failed: %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })

	if _, err := file.WriteString("abcdef"); err != nil {
		t.Fatalf("WriteString failed: %v", err)
	}
	if _, err := file.Seek(3, 0); err != nil {
		t.Fatalf("Seek failed: %v", err)
	}

	source := NewFileSource(file)

	if _, err := file.Seek(0, 0); err != nil {
		t.Fatalf("Seek failed: %v", err)
	}

	store := NewChunkStore(4)
	if err := source.Fill(store); err != nil {
		t.Fatalf("Fill failed: %v", err)
	}
	if got := readStore(t, store); got != "def" {
		t.Fatalf("Fill data = %q, want %q", got, "def")
	}
}
