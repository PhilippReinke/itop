package ui

import "testing"

func TestPanelTextAppliesCursorMovement(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "move left and overwrite",
			input: "hello\x1b[2D!!",
			want:  "hel!!",
		},
		{
			name:  "move up and replace line",
			input: "first\n\x1b[1A\r\x1b[2Ksecond",
			want:  "second",
		},
		{
			name:  "carriage return overwrites progress",
			input: "loading 10%\rloading 20%",
			want:  "loading 20%",
		},
		{
			name:  "erase to end of line",
			input: "12345\x1b[3D\x1b[Kx",
			want:  "12x",
		},
		{
			name:  "absolute position and erase",
			input: "one\n\x1b[1;1H\x1b[2Ktwo",
			want:  "two",
		},
		{
			name:  "styles and title are hidden",
			input: "\x1b[31mred\x1b[0m\x1b]0;title\a",
			want:  "red",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := cleanPanelText(test.input); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
