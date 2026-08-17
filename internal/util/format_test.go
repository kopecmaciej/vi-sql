package util

import "testing"

func TestNormalizeBooleanValue(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		isBoolean bool
		want      string
	}{
		{name: "true", value: "t", isBoolean: true, want: "true"},
		{name: "false", value: "f", isBoolean: true, want: "false"},
		{name: "already normalized", value: "true", isBoolean: true, want: "true"},
		{name: "non boolean", value: "t", isBoolean: false, want: "t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeBooleanValue(tt.value, tt.isBoolean); got != tt.want {
				t.Errorf("NormalizeBooleanValue(%q, %t) = %q, want %q", tt.value, tt.isBoolean, got, tt.want)
			}
		})
	}
}
