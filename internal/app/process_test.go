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

func TestSSHDestination(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"host"}, "host"},
		{[]string{"user@host"}, "host"},
		{[]string{"user@host", "run"}, "host"},
		{[]string{"host:2222"}, "host"},
		{[]string{"-p", "2222", "host"}, "host"},
		{[]string{"-i", "key", "user@host"}, "host"},
		{[]string{"-o", "StrictHostKeyChecking=no", "host"}, "host"},
		{[]string{"-v", "host"}, "host"},
		{[]string{"--", "-host"}, "-host"},
		{[]string{}, ""},
	}
	for _, c := range cases {
		if got := sshDestination(c.args); got != c.want {
			t.Errorf("sshDestination(%q) = %q, want %q", c.args, got, c.want)
		}
	}
}
