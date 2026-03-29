package model

import "io"

const defaultChunkSize = 32 * 1024

// ByteStore is append-only storage for the original byte stream.
type ByteStore interface {
	Append(p []byte) (start int64, end int64)
	ReadAt(p []byte, off int64) (int, error)
	Len() int64
}

// ChunkStore keeps data in fixed-size chunks to support efficient appends.
type ChunkStore struct {
	chunkSize int
	chunks    [][]byte
	length    int64
}

// NewChunkStore constructs a chunked append-only store.
func NewChunkStore(chunkSize int) *ChunkStore {
	if chunkSize <= 0 {
		chunkSize = defaultChunkSize
	}
	return &ChunkStore{chunkSize: chunkSize}
}

// Append appends data to the store and returns the byte range written.
func (s *ChunkStore) Append(p []byte) (start int64, end int64) {
	start = s.length
	for len(p) > 0 {
		if len(s.chunks) == 0 || len(s.chunks[len(s.chunks)-1]) == s.chunkSize {
			s.chunks = append(s.chunks, make([]byte, 0, s.chunkSize))
		}

		last := s.chunks[len(s.chunks)-1]
		n := s.chunkSize - len(last)
		if n > len(p) {
			n = len(p)
		}
		last = append(last, p[:n]...)
		s.chunks[len(s.chunks)-1] = last
		s.length += int64(n)
		p = p[n:]
	}
	return start, s.length
}

// Len returns the total number of bytes stored.
func (s *ChunkStore) Len() int64 {
	return s.length
}

// ReadAt reads bytes at the given offset.
func (s *ChunkStore) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, io.EOF
	}
	if off >= s.length {
		return 0, io.EOF
	}

	read := 0
	for len(p) > 0 && off < s.length {
		chunkIndex := int(off / int64(s.chunkSize))
		chunkOffset := int(off % int64(s.chunkSize))
		chunk := s.chunks[chunkIndex]
		n := len(chunk) - chunkOffset
		if n > len(p) {
			n = len(p)
		}
		copy(p[:n], chunk[chunkOffset:chunkOffset+n])
		p = p[n:]
		off += int64(n)
		read += n
	}

	if len(p) > 0 {
		return read, io.EOF
	}
	return read, nil
}
