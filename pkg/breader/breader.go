// Package breader provides a thread-safe reader that ensures full reads.
package breader

import (
	"io"
	"sync"
)

// BReader is a structure that wraps an io.Reader with a mutex to ensure thread safety.
type BReader struct {
	reader io.Reader  // underlying io.Reader
	mu     sync.Mutex // mutex for ensuring thread safety
}

// New creates a new instance of a BReader and returns it as an io.Reader.
// It accepts an io.Reader which it wraps in a BReader to make it thread-safe.
func New(r io.Reader) io.Reader {
	return &BReader{
		reader: r,            // set the underlying reader
		mu:     sync.Mutex{}, // initialize the mutex
	}
}

// Read reads from the underlying reader into p until it is full or an error occurs.
// It ensures thread safety by locking around the reading operation.
func (br *BReader) Read(p []byte) (int, error) {
	br.mu.Lock()
	defer br.mu.Unlock()

	currReadIdx := 0
	// Loop until p is full
	for currReadIdx < len(p) {
		n, err := br.reader.Read(p[currReadIdx:])
		currReadIdx += n
		if err != nil {
			return currReadIdx, err
		}
	}

	return currReadIdx, nil
}
