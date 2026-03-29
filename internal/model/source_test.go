package model

import (
	"os"
	"strings"
	"testing"
)

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
