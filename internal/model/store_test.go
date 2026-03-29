package model

import (
	"io"
	"testing"
)

func TestChunkStoreAppendAndReadAcrossChunks(t *testing.T) {
	store := NewChunkStore(4)
	store.Append([]byte("hello"))
	store.Append([]byte(" world"))

	buf := make([]byte, 11)
	n, err := store.ReadAt(buf, 0)
	if err != nil {
		t.Fatalf("ReadAt failed: %v", err)
	}
	if got, want := n, len(buf); got != want {
		t.Fatalf("read = %d, want %d", got, want)
	}
	if got, want := string(buf), "hello world"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestChunkStoreReadAtEOF(t *testing.T) {
	store := NewChunkStore(4)
	store.Append([]byte("hello"))

	buf := make([]byte, 10)
	n, err := store.ReadAt(buf, 2)
	if err != io.EOF {
		t.Fatalf("ReadAt error = %v, want EOF", err)
	}
	if got, want := n, 3; got != want {
		t.Fatalf("read = %d, want %d", got, want)
	}
	if got, want := string(buf[:n]), "llo"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}
