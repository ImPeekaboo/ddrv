// Package httprange provides a utility function for parsing HTTP Range headers.
//
// The function supports requests for a single range of bytes, which is the most common use case for range requests.
// It returns the start and end positions of the requested range, along with the corresponding Content-Range
// and Content-Length strings for use in an HTTP 206 Partial Content response.
//
// The current implementation does not fully comply with RFC 7233 in the following ways:
//
//  1. The function assumes that the unit is always 'bytes'. According to RFC 7233, a range unit identifier can be
//     bytes or any token registered in the HTTP Range Unit Registry. However, 'bytes' is the only range unit identifier
//     defined by HTTP/1.1, and it's the most commonly used in practice.
//
//  2. The function only supports a single range. RFC 7233 allows a client to request multiple ranges in a single
//     request, but this is not often used in practice and can increase the complexity of the server's response.
//
// These limitations are deemed acceptable for the typical use cases this function is designed for, and they significantly
// simplify the implementation. However, if full compliance with RFC 7233 is required, this function should not be used.
package httprange

import (
	"errors"
	"fmt"
	"strings"
)

type Range struct {
	Start  int64
	Length int64
	Header string
}

var ErrInvalid = errors.New("invalid range header format")

// Parse parses the Range header from an HTTP request.
// It supports bytes as a unit and doesn't support multiple range requests.
// The function returns a Range struct containing the start and end positions of the range, the Content-Range header and the Content-Length header.
func Parse(rangeHeader string, size int64) (*Range, error) {
	// Expect format is "bytes=start-end"
	parts := strings.SplitN(rangeHeader, "=", 2)
	if len(parts) != 2 || parts[0] != "bytes" {
		return nil, ErrInvalid
	}

	start, end := int64(-1), int64(-1)
	if strings.HasPrefix(parts[1], "-") {
		// The -suffix form "-n"
		_, err := fmt.Sscanf(parts[1], "-%d", &end)
		if err != nil {
			return nil, ErrInvalid
		}
		start = size - end
		end = size - 1
	} else if strings.HasSuffix(parts[1], "-") {
		// The start- form "n-"
		_, err := fmt.Sscanf(parts[1], "%d-", &start)
		if err != nil {
			return nil, ErrInvalid
		}
		end = size - 1
	} else {
		// "n-m" form
		_, err := fmt.Sscanf(parts[1], "%d-%d", &start, &end)
		if err != nil {
			return nil, err
		}
	}

	if start > end || start > size || end > size || start < 0 || end < 0 {
		return nil, ErrInvalid
	}

	contentRange := fmt.Sprintf("bytes %d-%d/%d", start, end, size)
	length := end - start + 1

	return &Range{start, length, contentRange}, nil
}
