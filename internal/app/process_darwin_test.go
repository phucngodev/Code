//go:build darwin

package app

import "testing"

func TestParseProcArgs(t *testing.T) {
	var buf []byte
	buf = append(buf, 2, 0, 0, 0)                    // argc, little-endian
	buf = append(buf, []byte("/usr/bin/ssh\x00")...) // executable path
	buf = append(buf, 0)                             // padding
	buf = append(buf, []byte("ssh\x00user@host\x00")...)

	args := parseProcArgs(buf)
	want := []string{"ssh", "user@host"}
	if len(args) != len(want) {
		t.Fatalf("args = %q, want %q", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestParseProcArgsTooShort(t *testing.T) {
	if args := parseProcArgs([]byte{1, 2}); args != nil {
		t.Fatalf("args = %q, want nil", args)
	}
}
