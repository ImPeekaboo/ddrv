package httprange

import (
	"net/http"
	"testing"
)

func TestParseRangeHeader(t *testing.T) {
	size := int64(4707476)

	tests := []struct {
		name          string
		headers       http.Header
		start         int64
		contentRange  string
		contentLength int64
		err           bool
	}{
		{
			name:          "Invalid range header",
			headers:       http.Header{"Range": []string{"invalid"}},
			start:         0,
			contentRange:  "",
			contentLength: 0,
			err:           true,
		},
		{
			name:          "Range with start and end",
			headers:       http.Header{"Range": []string{"bytes=100-500"}},
			start:         100,
			contentRange:  "bytes 100-500/4707476",
			contentLength: 401,
			err:           false,
		},
		{
			name:          "Range with start only",
			headers:       http.Header{"Range": []string{"bytes=100-"}},
			start:         100,
			contentRange:  "bytes 100-4707475/4707476",
			contentLength: 4707376,
			err:           false,
		},
		{
			name:          "Range with start only",
			headers:       http.Header{"Range": []string{"bytes=0-"}},
			start:         0,
			contentRange:  "bytes 0-4707475/4707476",
			contentLength: 4707476,
			err:           false,
		},
		{
			name:          "Range with end only",
			headers:       http.Header{"Range": []string{"bytes=-500"}},
			start:         size - 500,
			contentRange:  "bytes 4706976-4707475/4707476",
			contentLength: 500,
			err:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Header: tt.headers,
			}

			hr, err := Parse(req.Header.Get("Range"), size)

			if (err != nil) != tt.err {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.err)
				return
			}
			if err == nil {
				if hr.Start != tt.start || hr.Header != tt.contentRange || hr.Length != tt.contentLength {
					t.Errorf("Parse() = %v-%v %v, want %v-%v %v",
						hr.Start, hr.Header, hr.Length, tt.start, tt.contentRange, tt.contentLength)
				}
			}
		})
	}
}
