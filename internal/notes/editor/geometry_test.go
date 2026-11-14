package editor

import "testing"

// TestTextWidth checks a LineWidth in (0, 1] is a share of the pane's width,
// a larger one is a maximum in points, and either fits a small pane.
func TestTextWidth(t *testing.T) {
	cases := []struct {
		name string
		line float32
		pane float32
		want float32
	}{
		{"a share scales with the pane", 0.75, 2000, 1500},
		{"a share on a small pane", 0.75, 800, 600},
		{"a share never exceeds the pane", 0.75, 400, 300},
		{"an absolute width is kept", 1200, 2000, 1200},
		{"an absolute width fits a small pane", 1200, 900, 900},
		{"an unset width falls back", 0, 2000, 760},
		{"a small absolute width is kept", 100, 2000, 100},
	}
	for _, c := range cases {
		if got := textWidth(c.line, c.pane); got != c.want {
			t.Errorf("%s: textWidth(%v, %v) = %v, want %v", c.name, c.line, c.pane, got, c.want)
		}
	}
}
