package app

import "testing"

func TestForegroundGroupRunning(t *testing.T) {
	cases := []struct {
		name        string
		pgrp, tpgid int
		want        bool
	}{
		{"shell is foreground", 100, 100, false},
		{"a command is foreground", 100, 200, true},
		{"no foreground group", 100, 0, false},
		{"no controlling terminal", 100, -1, false},
	}
	for _, c := range cases {
		if got := foregroundGroupRunning(c.pgrp, c.tpgid); got != c.want {
			t.Errorf("%s: foregroundGroupRunning(%d, %d) = %v, want %v", c.name, c.pgrp, c.tpgid, got, c.want)
		}
	}
}
