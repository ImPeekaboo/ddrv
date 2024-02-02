package lreader

import (
	"bytes"
	"io"
	"testing"
)

func TestLimitReader(t *testing.T) {
	tt := []struct {
		name          string
		input         string
		limit         int
		expected      string
		expectedError error
	}{
		{
			name:          "Exact limit",
			input:         "This is a test string",
			limit:         10,
			expected:      "This is a ",
			expectedError: io.EOF,
		},
		{
			name:          "Lower limit",
			input:         "This is a test string",
			limit:         5,
			expected:      "This ",
			expectedError: io.EOF,
		},
		{
			name:     "Higher limit",
			input:    "This is a test string",
			limit:    50,
			expected: "This is a test string",
		},
		{
			name:          "Zero limit",
			input:         "This is a test string",
			limit:         0,
			expected:      "",
			expectedError: io.EOF,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewBufferString(tc.input)
			lReader := New(reader, tc.limit)

			buffer := make([]byte, len(tc.input))
			n, err := lReader.Read(buffer)

			// trim buffer to actual read size
			buffer = buffer[:n]

			if string(buffer) != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, buffer)
			}

			if err != tc.expectedError {
				t.Errorf("expected error %v, got %v", tc.expectedError, err)
			}
		})
	}
}
