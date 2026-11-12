package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStripRejectsNonELF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lib.so")
	if err := os.WriteFile(path, []byte("not an elf file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := strip(path); err == nil {
		t.Fatal("strip should reject a file that is not ELF")
	}
}

func TestStripRejectsTruncatedELFHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lib.so")
	data := make([]byte, 0x30)
	data[0] = 0x7f
	copy(data[1:], "ELF")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := strip(path); err == nil {
		t.Fatal("strip should reject a truncated ELF header")
	}
}
