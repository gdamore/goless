// Copyright 2026 Garrett D'Amore
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"io"
	"os"
)

// Source fills a byte store from some upstream content source.
type Source interface {
	Fill(store ByteStore) error
	Size() (int64, bool)
}

// ReaderSource adapts an io.Reader into a Source.
type ReaderSource struct {
	reader    io.Reader
	chunkSize int
}

// NewReaderSource constructs a Source for an io.Reader.
func NewReaderSource(reader io.Reader) Source {
	return &ReaderSource{
		reader:    reader,
		chunkSize: defaultChunkSize,
	}
}

// Fill appends all available bytes from the reader into the store.
func (s *ReaderSource) Fill(store ByteStore) error {
	buf := make([]byte, s.chunkSize)
	for {
		n, err := s.reader.Read(buf)
		if n > 0 {
			store.Append(buf[:n])
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// Size reports whether the source length is known.
func (s *ReaderSource) Size() (int64, bool) {
	return 0, false
}

// BytesSource adapts a byte slice into a Source.
type BytesSource struct {
	data []byte
}

// NewBytesSource constructs a Source for a byte slice.
func NewBytesSource(data []byte) Source {
	dup := append([]byte(nil), data...)
	return &BytesSource{data: dup}
}

// Fill appends all source bytes to the store.
func (s *BytesSource) Fill(store ByteStore) error {
	store.Append(s.data)
	return nil
}

// Size reports the exact byte length of the source.
func (s *BytesSource) Size() (int64, bool) {
	return int64(len(s.data)), true
}

// StringSource adapts a string into a Source.
type StringSource struct {
	data string
}

// NewStringSource constructs a Source for a string.
func NewStringSource(data string) Source {
	return &StringSource{data: data}
}

// Fill appends all source bytes to the store.
func (s *StringSource) Fill(store ByteStore) error {
	store.Append([]byte(s.data))
	return nil
}

// Size reports the exact byte length of the source.
func (s *StringSource) Size() (int64, bool) {
	return int64(len(s.data)), true
}

// FileSource adapts an os.File into a Source.
type FileSource struct {
	file *os.File
}

// NewFileSource constructs a Source for an os.File.
func NewFileSource(file *os.File) Source {
	return &FileSource{file: file}
}

// Fill appends all bytes read from the file's current position to the store.
func (s *FileSource) Fill(store ByteStore) error {
	buf := make([]byte, defaultChunkSize)
	for {
		n, err := s.file.Read(buf)
		if n > 0 {
			store.Append(buf[:n])
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// Size reports the file size when it can be determined.
func (s *FileSource) Size() (int64, bool) {
	info, err := s.file.Stat()
	if err != nil {
		return 0, false
	}
	return info.Size(), true
}
