// Command elfstrip removes a Linux shared library's debug sections and symbol
// table, which Zig keeps even with -Dstrip=true. It strips each file in place,
// the way MyGo strips its own libghostty-vt build. It is a build tool, not part
// of the app.
//
//	go run ./internal/elfstrip path/to/lib.so
package main

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: elfstrip <file.so>...")
		os.Exit(2)
	}
	for _, path := range os.Args[1:] {
		if err := strip(path); err != nil {
			log.Fatalf("elfstrip: %s: %v", path, err)
		}
	}
}

// strip rewrites the ELF file without its debug sections and symbol table,
// keeping every allocated section and the dynamic symbols a loader needs.
func strip(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	file, err := elf.NewFile(bytes.NewReader(data))
	if err != nil {
		return err
	}
	if file.Class != elf.ELFCLASS64 || file.Data != elf.ELFDATA2LSB {
		return fmt.Errorf("not a 64-bit little-endian ELF file")
	}

	// Keep the file up to the end of the last program segment.
	end := uint64(0)
	for _, segment := range file.Progs {
		end = max(end, segment.Off+segment.Filesz)
	}
	out := append([]byte(nil), data[:end]...)

	le := binary.LittleEndian
	shoff := le.Uint64(data[0x28:])
	shentsize := uint64(le.Uint16(data[0x3a:]))
	shnum := uint64(le.Uint16(data[0x3c:]))
	headers := append([]byte(nil), data[shoff:shoff+shentsize*shnum]...)

	for i, section := range file.Sections {
		header := headers[uint64(i)*shentsize:]
		// Keep allocated sections and anything already inside the segments.
		if section.Type == elf.SHT_NULL || section.Type == elf.SHT_NOBITS ||
			section.Flags&elf.SHF_ALLOC != 0 || section.Offset < end {
			continue
		}
		size := section.FileSize
		if strings.HasPrefix(section.Name, ".debug") || strings.HasPrefix(section.Name, ".zdebug") {
			size = 0
		}
		align := max(section.Addralign, 1)
		for uint64(len(out))%align != 0 {
			out = append(out, 0)
		}
		le.PutUint64(header[0x18:], uint64(len(out))) // sh_offset
		le.PutUint64(header[0x20:], size)             // sh_size
		out = append(out, data[section.Offset:section.Offset+size]...)
	}

	for len(out)%8 != 0 {
		out = append(out, 0)
	}
	le.PutUint64(out[0x28:], uint64(len(out))) // e_shoff
	out = append(out, headers...)

	tmp := path + ".stripped"
	if err := os.WriteFile(tmp, out, 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
