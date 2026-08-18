package page

import (
	"reflect"
	"testing"
)

func TestWrapText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		width int
		want  []string
	}{
		{
			name:  "short text stays on one line",
			input: "hello world",
			width: 50,
			want:  []string{"hello world"},
		},
		{
			name:  "breaks on word boundary",
			input: "gaussdb://user:****@host:8000/db lint rule",
			width: 40,
			want:  []string{"gaussdb://user:****@host:8000/db lint", "rule"},
		},
		{
			name:  "hard-breaks long token without spaces",
			input: "gaussdb://user:****@long-host.example.com:8000/testdb?sslmode=disable&target_session_attrs=primary",
			width: 40,
			want:  []string{
				"gaussdb://user:****@long-host.example.co",
				"m:8000/testdb?sslmode=disable&target_ses",
				"sion_attrs=primary",
			},
		},
		{
			name:  "empty string",
			input: "",
			width: 40,
			want:  []string{""},
		},
		{
			name:  "non-positive width returns original",
			input: "hello",
			width: 0,
			want:  []string{"hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapText(tt.input, tt.width)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("wrapText(%q, %d) = %#v, want %#v", tt.input, tt.width, got, tt.want)
			}
		})
	}
}