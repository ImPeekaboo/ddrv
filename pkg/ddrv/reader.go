package ddrv

import "io"

// Reader is a structure that manages the reading of a sequence of Chunks.
// It reads chunks in order, closing each one after it's Read and moving on to the next.
type Reader struct {
	chunks []Node        // The list of chunks to be Read.
	curIdx int           // Index of the chunk that is currently being Read.
	closed bool          // Indicates whether the Reader has been closed.
	rest   *Rest         // rest object provides access to the chunks.
	reader io.ReadCloser // The reader that is reading the current chunk.
	pos    int64
}

// NewReader creates new Reader instance which implements io.ReadCloser.
func NewReader(chunks []Node, pos int64, rest *Rest) (io.ReadCloser, error) {
	r := &Reader{chunks: chunks, pos: pos, rest: rest}
	// Calculate Start and End for each part
	var offset int64
	for i := range r.chunks {
		r.chunks[i].Start = offset                             // 0
		r.chunks[i].End = offset + int64(r.chunks[i].Size) - 1 // 9
		offset = r.chunks[i].End + 1
	}
	// If pos > size means file is ended
	if r.pos > offset {
		return nil, io.EOF
	}
	// Find starting chunk and drop all the chunks that are completely before 'pos'
	var start int
	for i, chunk := range r.chunks {
		start += chunk.Size
		if start > int(r.pos) {
			// Drop extra chunks to save memory
			r.chunks = r.chunks[i:]
			break
		}
	}

	return r, nil
}

// Read reads data from the current chunk into p.
// If it reaches the end of a chunk, it moves to the next one.
// It reads until p is full or there are no more chunks to Read from.
func (r *Reader) Read(p []byte) (int, error) {
	if r.closed {
		return 0, ErrClosed
	}
	// Handle files with zero length
	if 0 == len(r.chunks) {
		return 0, io.EOF
	}
	if r.reader == nil {
		if err := r.next(); err != nil {
			return 0, err
		}
	}
	var totalRead int
	for {
		nr, err := r.reader.Read(p[totalRead:])
		totalRead += nr

		if err == io.EOF {
			r.curIdx++
			if r.curIdx >= len(r.chunks) {
				return totalRead, err
			}
			if err = r.next(); err != nil {
				return totalRead, err
			}
		}

		if err != nil && err != io.EOF {
			return totalRead, err
		}

		if totalRead >= len(p) {
			return totalRead, nil
		}
	}
}

// Close implements the Close method of io.Closer. It closes the Reader.
// If the Reader is already closed, Close returns ErrAlreadyClosed.
func (r *Reader) Close() error {
	if r.closed {
		return ErrAlreadyClosed
	}
	if r.reader != nil {
		_ = r.reader.Close()
	}
	r.closed = true
	return nil
}

// next moves to the next chunk in the chunks slice, creating a new reader for it.
func (r *Reader) next() error {
	if r.reader != nil {
		if err := r.reader.Close(); err != nil {
			return err
		}
	}
	chunk := r.chunks[r.curIdx]

	// Find start byte in range header here
	var start int
	if r.pos > chunk.Start {
		start = int(r.pos - chunk.Start)
	}

	reader, err := r.rest.ReadAttachment(&chunk, start, chunk.Size-1)
	if err != nil {
		return err
	}
	r.reader = reader

	return nil
}
