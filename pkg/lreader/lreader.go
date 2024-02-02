// Package lreader provides a reader that limits the amount of data read from
// an underlying io.ReadCloser to a specified limit. Once the limit is reached,
// the underlying ReadCloser is closed and further Read calls to the lreader
// will return io.EOF.
package lreader

import "io"

type lreader struct {
	r      io.ReadCloser // underlying reader
	remain int           // remaining bytes
}

// New initializes a new instance of lreader with the provided ReadCloser and limit,
// and returns it as an io.Reader. The returned Reader will read from the
// underlying ReadCloser until the specified limit is reached. At that point,
// the underlying ReadCloser is closed and further Read calls will return io.EOF.
func New(r io.ReadCloser, limit int) io.Reader {
	return &lreader{
		r:      r,
		remain: limit,
	}
}

// Read reads up to len(p) bytes into p from the underlying ReadCloser. It returns
// the number of bytes read (0 <= n <= len(p)) and any error encountered. Once
// the specified limit is reached, or if the underlying ReadCloser returns io.EOF,
// the underlying ReadCloser is closed and further Read calls will return io.EOF.
func (l *lreader) Read(p []byte) (int, error) {
	if l.remain <= 0 {
		return 0, io.EOF
	}

	if len(p) > l.remain {
		p = p[:l.remain]
	}

	n, err := l.r.Read(p)
	l.remain -= n

	if err == io.EOF {
		_ = l.r.Close()
		l.remain = 0
	}

	if err != nil {
		return n, err
	}

	if l.remain == 0 {
		err = io.EOF
	}

	return n, err
}
