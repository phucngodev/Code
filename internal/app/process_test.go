package app

import "testing"

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
