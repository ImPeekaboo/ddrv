package ns

import (
	"database/sql/driver"
	"testing"
)

func TestNullString(t *testing.T) {
	// Test cases for the Scan method
	tests := []struct {
		name    string
		input   interface{}
		want    NullString
		wantErr bool
	}{
		{
			name:  "null input",
			input: nil,
			want:  "",
		},
		{
			name:  "byte slice input",
			input: []byte("test"),
			want:  "test",
		},
		{
			name:  "string input",
			input: "test",
			want:  "test",
		},
		{
			name:    "unsupported type input",
			input:   123,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ns NullString
			err := ns.Scan(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Scan() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if ns != tt.want {
				t.Errorf("Scan() = %v, want %v", ns, tt.want)
			}
		})
	}

	// Test cases for the Value method
	valueTests := []struct {
		name string
		ns   NullString
		want driver.Value
	}{
		{
			name: "null string",
			ns:   "",
			want: nil,
		},
		{
			name: "non-null string",
			ns:   "test",
			want: "test",
		},
	}

	for _, tt := range valueTests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.ns.Value()
			if err != nil {
				t.Errorf("Value() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Value() = %v, want %v", got, tt.want)
			}
		})
	}
}
