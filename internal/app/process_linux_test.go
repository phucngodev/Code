//go:build linux

package app

import "testing"

func TestStatProcessGroups(t *testing.T) {
	// pid (comm with spaces) state ppid pgrp session tty_nr tpgid ...
	stat := "4321 (my shell) S 1 4321 4321 34816 5678 4194304 1 0 0 0 0 0 0 0 2 0 0 0 100 0 0"
	pgrp, tpgid, ok := statProcessGroups(stat)
	if !ok || pgrp != 4321 || tpgid != 5678 {
		t.Fatalf("statProcessGroups = %d, %d, %v; want 4321, 5678, true", pgrp, tpgid, ok)
	}
}

func TestStatProcessGroupsTooShort(t *testing.T) {
	if _, _, ok := statProcessGroups("1 (sh) S 1 1"); ok {
		t.Fatal("a short stat line should not parse")
	}
	if _, _, ok := statProcessGroups("no parens"); ok {
		t.Fatal("a stat line with no command should not parse")
	}
}
