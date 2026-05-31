package blockchain

import "testing"

func TestShort(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{"nil", nil, ""},
		{"empty", []byte{}, ""},
		{"shorter than 8 hex chars", []byte{0xab}, "ab"},
		{"exactly 8 hex chars", []byte{0xde, 0xad, 0xbe, 0xef}, "deadbeef"},
		{"truncated to 8 hex chars", []byte{0xde, 0xad, 0xbe, 0xef, 0x01}, "deadbeef"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Short(tt.in); got != tt.want {
				t.Fatalf("Short(%x) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
