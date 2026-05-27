package console

import "testing"

func TestGetPreferredConsoleTTY(t *testing.T) {
	tests := []struct {
		name string
		ttys []string
		want string
	}{
		{
			name: "prefer tty1 over serial",
			ttys: []string{"ttyS0", "tty1"},
			want: "tty1",
		},
		{
			name: "prefer tty1 over earlier virtual tty",
			ttys: []string{"tty2", "tty1"},
			want: "tty1",
		},
		{
			name: "skip tty0 when serial is available",
			ttys: []string{"tty0", "ttyS0"},
			want: "ttyS0",
		},
		{
			name: "prefer first usable virtual tty",
			ttys: []string{"ttyS0", "tty2"},
			want: "tty2",
		},
		{
			name: "keep ama console when it is the only usable option",
			ttys: []string{"ttyAMA0"},
			want: "ttyAMA0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPreferredConsoleTTY(tt.ttys)
			if got != tt.want {
				t.Fatalf("getPreferredConsoleTTY(%v) = %q, want %q", tt.ttys, got, tt.want)
			}
		})
	}
}
